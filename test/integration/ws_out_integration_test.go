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

package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/external"
	"github.com/rulego/rulego/test"
)

// TestWsOut ws 出站集成测试：
// WsNode 作为客户端拨号连远端 ws server → 发送数据 → server 收到。
// 与 ws_session_push_integration_test（入站寻址推送）对应，本测试验证出站直发主路径。
func TestWsOut(t *testing.T) {
	config := types.NewConfig()

	// ① 起一个 ws server，收到消息塞 chan
	received := make(chan []byte, 4)
	srv := newWsOutServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	// ② WsNode dial 该 server
	node := &external.WsNode{}
	if err := node.Init(config, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 0, // 禁用心跳，专注验证出站发送
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// ③ 发送文本
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
		t.Fatal("timeout OnMsg")
	}

	// ④ server 收到 "HELLO"
	select {
	case got := <-received:
		if string(got) != "HELLO" {
			t.Fatalf("server got %q, want HELLO", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting server receive")
	}
}

// newWsOutServer 起一个真实 ws server，把收到的消息塞进 received
func newWsOutServer(received chan<- []byte) *httptest.Server {
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
