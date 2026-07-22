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

// TestWsSessionPush ws 入+出闭环集成测试：
// 设备 WS 连入 endpoint/ws → 首帧提取 deviceId → WsNode ref://寻址推送 → 设备收到 "HELLO"
func TestWsSessionPush(t *testing.T) {
	config := types.NewConfig()
	serverPort := "localhost:9211"

	// ① endpoint/ws 启动（sessionKey=${msg.deviceId}）
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

	// ② 注册到 NodePool
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// ③ 设备 WS 连入 + 上报 deviceId
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

	// ⑤ 触发推送
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

	// ⑥ 设备收到推送
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
