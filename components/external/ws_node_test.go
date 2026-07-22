/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package external

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/test"
)

// 复用 net_node_push_test.go 的 miniPool / fakeRegistry / fakeSender

// TestWsNodeAddressing ref:// 寻址推送（WsNode 寻址分支 = 原 wsSend 逻辑）
func TestWsNodeAddressing(t *testing.T) {
	sender := &fakeSender{}
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_001", sender))

	pool := &miniPool{instances: map[string]interface{}{"ws": reg}}
	cfg := types.NewConfig()
	cfg.NodePool = pool

	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{"server": "ref://ws", "target": "DEV_001"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "HELLO", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.received) != 1 || string(sender.received[0]) != "HELLO" {
		t.Fatalf("received %v, want [HELLO]", sender.received)
	}
}

// TestWsNodeBroadcast ref:// target=* 广播
func TestWsNodeBroadcast(t *testing.T) {
	s1, s2 := &fakeSender{}, &fakeSender{}
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_A", s1))
	reg.Add(endpoint.NewSession("DEV_B", s2))

	pool := &miniPool{instances: map[string]interface{}{"ws": reg}}
	cfg := types.NewConfig()
	cfg.NodePool = pool

	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{"server": "ref://ws", "target": "*"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "BCAST", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if len(s1.received) != 1 || len(s2.received) != 1 {
		t.Fatalf("both should receive: s1=%d s2=%d", len(s1.received), len(s2.received))
	}
}

// TestWsNodeNoMatch ref:// 未命中 → TellFailure
func TestWsNodeNoMatch(t *testing.T) {
	reg := &fakeRegistry{sessions: map[string]*endpoint.Session{}}
	reg.Add(endpoint.NewSession("DEV_001", &fakeSender{}))

	pool := &miniPool{instances: map[string]interface{}{"ws": reg}}
	cfg := types.NewConfig()
	cfg.NodePool = pool

	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{"server": "ref://ws", "target": "GHOST"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "X", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expect TellFailure for no-match target")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

// TestWsNodeDialFailure 出站模式（server 非 ref://）：dial 无效地址失败 → TellFailure
func TestWsNodeDialFailure(t *testing.T) {
	cfg := types.NewConfig()
	node := &WsNode{}
	// ws://127.0.0.1:1 端口通常连接被拒（快速失败）。NodeClientInitNow 默认 false，Init 不立即 dial
	if err := node.Init(cfg, types.Configuration{"server": "ws://127.0.0.1:1"}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "X", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expect dial failure TellFailure for unreachable ws server")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout (dial should fail fast on refused port)")
	}
}

// newWsTestServer 起一个真实 ws server，把收到的消息塞进 received
func newWsTestServer(received chan<- []byte) *httptest.Server {
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			select {
			case received <- msg:
			default:
			}
		}
	}))
}

func wsSendOk(t *testing.T, node types.Node, data string, dt types.DataType) {
	t.Helper()
	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: data, DataType: dt}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout OnMsg")
	}
}

func wsMustRecv(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case b := <-ch:
		return b
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting server receive")
		return nil
	}
}

// TestWsNodeDialSend 出站拨号真实 ws server：dial + 发送 text/binary → server 收到
func TestWsNodeDialSend(t *testing.T) {
	received := make(chan []byte, 4)
	srv := newWsTestServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cfg := types.NewConfig()
	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 0, // 禁用心跳，专注验证发送主路径
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// text 帧
	wsSendOk(t, node, "HELLO", types.TEXT)
	if got := wsMustRecv(t, received); string(got) != "HELLO" {
		t.Fatalf("text: server got %q, want HELLO", got)
	}

	// binary 帧（DataType=BINARY 触发 BinaryMessage）
	wsSendOk(t, node, "BIN", types.BINARY)
	if got := wsMustRecv(t, received); string(got) != "BIN" {
		t.Fatalf("binary: server got %q, want BIN", got)
	}
}

// TestWsNodeDialHandshake 验证出站拨号带上 Headers 与 Subprotocol 到达 server 端
func TestWsNodeDialHandshake(t *testing.T) {
	gotAuth := make(chan string, 1)
	gotSub := make(chan string, 1)
	upgrader := websocket.Upgrader{Subprotocols: []string{"mqtt"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		gotAuth <- r.Header.Get("Authorization")
		gotSub <- c.Subprotocol()
		_, _, _ = c.ReadMessage() // 等 WsNode 发一帧后关闭
		_ = c.Close()
	}))
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cfg := types.NewConfig()
	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{
		"server":            wsURL,
		"headers":           map[string]string{"Authorization": "Bearer abc123"},
		"subprotocol":       "mqtt",
		"heartbeatInterval": 0,
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	wsSendOk(t, node, "HI", types.TEXT)

	select {
	case h := <-gotAuth:
		if h != "Bearer abc123" {
			t.Fatalf("Authorization header got %q", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout header")
	}
	select {
	case s := <-gotSub:
		if s != "mqtt" {
			t.Fatalf("subprotocol got %q, want mqtt", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout subprotocol")
	}
}

// TestWsNodeHeartbeat 开启心跳：多次发送跨越多个心跳周期，验证心跳定时器与业务发送并发无 race、不丢消息。
// 用于 -race 下检验 timerMu 对 heartbeatTimer 字段读写的保护。
func TestWsNodeHeartbeat(t *testing.T) {
	received := make(chan []byte, 10)
	srv := newWsTestServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cfg := types.NewConfig()
	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 1, // 1秒心跳
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// 发 3 条，间隔 0.5s，跨越至少 1 个心跳周期，让 onPing 与业务发送并发
	for i := 0; i < 3; i++ {
		wsSendOk(t, node, fmt.Sprintf("m%d", i), types.TEXT)
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(2 * time.Second) // 再让心跳跑几轮

	for i := 0; i < 3; i++ {
		got := wsMustRecv(t, received)
		if string(got) != fmt.Sprintf("m%d", i) {
			t.Fatalf("msg %d: got %q", i, got)
		}
	}
}
