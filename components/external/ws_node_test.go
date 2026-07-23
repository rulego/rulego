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

// Reuse net_node_push_test.go's miniPool / fakeRegistry / fakeSender

// TestWsNodeAddressing ref:// Addressing Push (WsNode addressing branch = original wsSend logic)
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

// TestWsNodeBroadcast ref:// target=* Broadcast
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

// TestWsNodeNoMatch ref:// Missed → TellFailure
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

// TestWsNodeDialFailure Outbound mode (server not ref://):d ial Invalid address failure → TellFailure
func TestWsNodeDialFailure(t *testing.T) {
	cfg := types.NewConfig()
	node := &WsNode{}
	// ws://127.0.0.1:1 ports are usually denied connections (fast failure). NodeClientInitNow defaults to false, and Init does not dial immediately
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

// newWsTestServer creates a real WS server and inserts the received message into the received server
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

// TestWsNodeDialSend Outbound dial-up real ws server: dial + send text/binary → server received
func TestWsNodeDialSend(t *testing.T) {
	received := make(chan []byte, 4)
	srv := newWsTestServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cfg := types.NewConfig()
	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 0, // Disable heartbeats and focus on verifying the main transmission path
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// text frame
	wsSendOk(t, node, "HELLO", types.TEXT)
	if got := wsMustRecv(t, received); string(got) != "HELLO" {
		t.Fatalf("text: server got %q, want HELLO", got)
	}

	// binary frame (DataType=BINARY triggers BinaryMessage)
	wsSendOk(t, node, "BIN", types.BINARY)
	if got := wsMustRecv(t, received); string(got) != "BIN" {
		t.Fatalf("binary: server got %q, want BIN", got)
	}
}

// TestWsNodeDialHandshake verifies outbound dial-up with headers and subprotocol to reach the server side
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
		_, _, _ = c.ReadMessage() // Wait for WsNode to send one frame and then close it
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

// TestWsNodeHeartbeat enables heartbeat: multiple transmissions spanning multiple heartbeat cycles to verify that the heartbeat timer and business transmission are concurrent without race or message loss.
// Used to check timerMu's protection against reading and writing heartbeatTimer fields under -race.
func TestWsNodeHeartbeat(t *testing.T) {
	received := make(chan []byte, 10)
	srv := newWsTestServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	cfg := types.NewConfig()
	node := &WsNode{}
	if err := node.Init(cfg, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 1, // 1 second of heartbeat
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// Send 3 messages at 0.5-second intervals, spanning at least one heartbeat cycle, allowing onPing to send concurrent messages with the service
	for i := 0; i < 3; i++ {
		wsSendOk(t, node, fmt.Sprintf("m%d", i), types.TEXT)
		time.Sleep(500 * time.Millisecond)
	}
	time.Sleep(2 * time.Second) // Let your heart race a few more times

	for i := 0; i < 3; i++ {
		got := wsMustRecv(t, received)
		if string(got) != fmt.Sprintf("m%d", i) {
			t.Fatalf("msg %d: got %q", i, got)
		}
	}
}
