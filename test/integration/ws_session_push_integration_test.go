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
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/external"
	"github.com/rulego/rulego/endpoint/impl"
	wsep "github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/test"
)

// TestWsSessionPush ws input+out/exit closed-loop integration test:
// Device WS connects to endpoint/ws → first frame extraction deviceId → WsNode ref:// addressing push → device receives "HELLO"
func TestWsSessionPush(t *testing.T) {
	config := types.NewConfig()
	serverPort := "localhost:9211"

	// (1) endpoint/ws start (sessionKey=${msg.deviceId})
	ep := &wsep.Websocket{}
	if err := ep.Init(config, types.Configuration{
		"server":     ":9211",
		"allowCors":  true,
		"sessionKey": "${msg.deviceId}",
	}); err != nil {
		t.Fatalf("ep Init: %v", err)
	}
	// ws upgrade path = /ws
	router := impl.NewRouter().From("/ws").End()
	if _, err := ep.AddRouter(router); err != nil {
		t.Fatalf("AddRouter: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("ep Start: %v", err)
	}
	defer ep.Destroy()
	time.Sleep(300 * time.Millisecond)

	// (2) Register in NodePool
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// (3) Device WS connects + reports deviceId
	c, _, err := websocket.DefaultDialer.Dial("ws://"+serverPort+"/ws", nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	if err := c.WriteMessage(websocket.TextMessage, []byte(`{"deviceId":"DEV_001"}`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// ④ WsNode ref:// + target
	node := &external.WsNode{}
	if err := node.Init(config, types.Configuration{
		"server": "ref://" + ep.Id(),
		"target": "DEV_001",
	}); err != nil {
		t.Fatalf("WsNode Init: %v", err)
	}
	defer node.Destroy()

	// (5) Trigger push notification
	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "HELLO", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
		done <- err
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WsNode OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting WsNode OnMsg callback")
	}

	// (6) The device receives a push notification
	if err := c.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("device read: %v", err)
	}
	if string(data) != "HELLO" {
		t.Fatalf("device got %q, want HELLO", data)
	}
}
