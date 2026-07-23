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

// TestWsOut ws Outbound Integration Testing:
// WsNode acts as a client dial-up, connects to the remote WS server → sends data → server receives.
// Corresponding to ws_session_push_integration_test (Inbound Addressing Push), this test verifies the main outbound direct dispatch path.
func TestWsOut(t *testing.T) {
	config := types.NewConfig()

	// (1) Set up a WS server and send messages to Chan
	received := make(chan []byte, 4)
	srv := newWsOutServer(received)
	defer srv.Close()
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://")

	// (2) WsNode dial for this server
	node := &external.WsNode{}
	if err := node.Init(config, types.Configuration{
		"server":            wsURL,
		"heartbeatInterval": 0, // Disable heartbeat, focus on verifying outbound sending
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// (3) Send text
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

	// (4) Server receives "HELLO"
	select {
	case got := <-received:
		if string(got) != "HELLO" {
			t.Fatalf("server got %q, want HELLO", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting server receive")
	}
}

// newWsOutServer creates a real WS server and inserts the received message into the received server
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
