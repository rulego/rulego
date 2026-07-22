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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	netep "github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
)

// TestNetScheduledAutoPush 规则链自动定时推送集成测试。
//
// 验证「规则链自动发送数据到客户端」：加载一条 root 规则链，内嵌 endpoint/schedule（每 1 秒触发）
// + jsTransform（透传业务 deviceId）+ NetNode（ref:// 按 ${deviceId} 业务ID寻址推送）。
// 设备 TCP 连入 endpoint/net 上报 deviceId 后，全程**不手动触发任何节点**，
// 断言设备自动收到多次定时推送。
//
// 与 session_push_integration_test 的区别：那个用 test.NodeOnMsg 手动调 NetNode.OnMsg（节点级）；
// 本测试由 endpoint/schedule 定时驱动规则链自动流转，覆盖「定时主动推送」端到端链路（规则链级）。
func TestNetScheduledAutoPush(t *testing.T) {
	config := rulego.NewConfig()

	// ① endpoint/net：接收设备连接，sessionKey=${msg.deviceId} 提取业务 deviceId
	ep := &netep.Net{}
	if err := ep.Init(config, types.Configuration{
		"protocol":   "tcp",
		"server":     ":0",
		"sessionKey": "${msg.deviceId}",
	}); err != nil {
		t.Fatalf("ep Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("ep Start: %v", err)
	}
	defer ep.Destroy()

	// ② 注册到独立 NodePool，作为 NetNode ref:// 的寻址目标
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// ③ 加载 root 规则链：schedule 每 1 秒 → jsTransform 透传业务 deviceId → NetNode ref://${deviceId}
	//    endpoint/schedule 内嵌在 root 链，由 EndpointAspect 自动 Start（rulego 包 init 注册该 aspect）
	chain := fmt.Sprintf(`{
  "ruleChain": {"id": "test_sched_push", "name": "scheduled auto push", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "endpoints": [{
      "id": "ep_sched", "type": "endpoint/schedule",
      "routers": [{
        "params": ["{\"deviceId\":\"DEV_001\"}", "JSON"],
        "from": {"path": "*/1 * * * * *"},
        "to": {"path": "test_sched_push:build"}
      }]
    }],
    "nodes": [
      {"id": "build", "type": "jsTransform", "configuration": {"jsScript": "metadata.deviceId=msg.deviceId;return {msg:'auto push',metadata:metadata,msgType:msgType};"}},
      {"id": "push", "type": "net", "configuration": {"server": "ref://%s", "target": "${deviceId}"}}
    ],
    "connections": [{"fromId": "build", "toId": "push", "type": "Success"}]
  }
}`, ep.Id())

	if _, err := rulego.New("test_sched_push", []byte(chain), engine.WithConfig(config)); err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_sched_push")

	// ④ 设备连入并上报业务 deviceId（触发 sessionKey 提取，session 绑定到 DEV_001）
	conn, err := net.Dial("tcp", ep.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	// ⑤ 断言：不手动触发任何节点，设备自动收到多次定时推送
	count := 0
	for i := 0; i < 3; i++ {
		if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("auto push #%d read err: %v (received %d so far)", i+1, err, count)
		}
		count++
		t.Logf("auto push #%d received: %q", count, string(buf[:n]))
	}
	if count < 2 {
		t.Fatalf("expected >=2 auto pushes, got %d", count)
	}
	t.Logf("PASS: received %d scheduled auto pushes to client without any manual trigger", count)
}
