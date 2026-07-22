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
	"net"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	netep "github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/engine"
)

// TestChainEndpointRefPush 验证 ref:// 引用「同链 endpoint」寻址推送（方案 B）。
//
// endpoint/net 定义在规则链 metadata.endpoints 内（不进 NodePool），
// NetNode ref://ep_net 应通过同链 ResourceRegistry 解析（resolveRef 同链优先），
// 再 SendToTarget 按 deviceId 寻址推送。设备连入后自动收到定时推送。
//
// 与 TestNetScheduledAutoPush 的区别：那个 endpoint 注册在共享池 NodePool（ref 走 NodePool）；
// 本测试 endpoint 在链内（ref 走同链 Resources），验证新的通用资源目录机制。
func TestChainEndpointRefPush(t *testing.T) {
	// 启用 endpoint；故意不设 NodePool → ref:// 必须走同链解析
	config := rulego.NewConfig(types.WithEndpointEnabled(true))

	chain := `{
  "ruleChain": {"id": "test_chain_ep_ref", "name": "chain endpoint ref push", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "endpoints": [
      {"id": "ep_net", "type": "endpoint/net", "configuration": {"protocol":"tcp","server":":0","sessionKey":"${msg.deviceId}","packetMode":"line"}},
      {"id": "ep_sched", "type": "endpoint/schedule", "routers": [{"params":["{\"deviceId\":\"DEV_001\"}","JSON"],"from":{"path":"*/1 * * * * *"},"to":{"path":"test_chain_ep_ref:build"}}]}
    ],
    "nodes": [
      {"id":"build","type":"jsTransform","configuration":{"jsScript":"metadata.deviceId=msg.deviceId;return {msg:'chain ref push',metadata:metadata,msgType:msgType};"}},
      {"id":"push","type":"net","configuration":{"server":"ref://ep_net","target":"${deviceId}"}}
    ],
    "connections": [{"fromId": "build", "toId": "push", "type": "Success"}]
  }
}`

	eng, err := rulego.New("test_chain_ep_ref", []byte(chain), engine.WithConfig(config))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_chain_ep_ref")

	// 从引擎 rootChainCtx 的资源目录取同链 endpoint 实例（验证 EndpointAspect 注册生效）
	ruleEng, ok := eng.(*engine.RuleEngine)
	if !ok {
		t.Fatalf("engine type %T not *engine.RuleEngine", eng)
	}
	epInst, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_net")
	if !found {
		t.Fatal("ep_net not registered in chain Resources (EndpointAspect syncResources failed)")
	}
	ep, ok := epInst.(*netep.Net)
	if !ok {
		t.Fatalf("ep_net resource type %T not *netep.Net", epInst)
	}

	// 设备连入并上报业务 deviceId（触发 sessionKey 提取，session 绑定 DEV_001）
	conn, err := net.Dial("tcp", ep.Addr())
	if err != nil {
		t.Fatalf("Dial %s: %v", ep.Addr(), err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	// 断言：设备自动收到定时推送（ref:// 同链寻址成功）
	count := 0
	for i := 0; i < 3; i++ {
		if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("chain-ref push #%d read err: %v (received %d)", i+1, err, count)
		}
		count++
		t.Logf("chain-ref push #%d received: %q", count, string(buf[:n]))
	}
	if count < 2 {
		t.Fatalf("expected >=2 chain-ref pushes, got %d", count)
	}
	t.Logf("PASS: ref:// 同链 endpoint 寻址推送 %d 次成功", count)
}

// TestChainEndpointRefReload 验证 reload 规则链后，同链 endpoint 仍注册到资源目录
// （EndpointAspect.OnReload 的 syncResources 重新注册），ref:// 同链解析不失效。
func TestChainEndpointRefReload(t *testing.T) {
	config := rulego.NewConfig(types.WithEndpointEnabled(true))
	chain := `{
  "ruleChain": {"id": "test_chain_ep_reload", "name": "chain endpoint ref reload", "root": true},
  "metadata": {
    "firstNodeIndex": 0,
    "endpoints": [
      {"id": "ep_net", "type": "endpoint/net", "configuration": {"protocol":"tcp","server":":0","packetMode":"line"}}
    ],
    "nodes": [
      {"id":"n1","type":"log","configuration":{"jsScript":"return msg;"}}
    ]
  }
}`

	eng, err := rulego.New("test_chain_ep_reload", []byte(chain), engine.WithConfig(config))
	if err != nil {
		t.Fatalf("load chain: %v", err)
	}
	defer rulego.Del("test_chain_ep_reload")
	ruleEng := eng.(*engine.RuleEngine)

	// reload 前：OnCreated 已注册 ep_net 到资源目录
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_net"); !found {
		t.Fatal("ep_net not registered before reload (OnCreated syncResources failed)")
	}

	// reload 规则链
	if err := eng.ReloadSelf([]byte(chain)); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// reload 后：OnReload 应重新注册 ep_net，ref:// 同链解析不失效
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_net"); !found {
		t.Fatal("ep_net not registered after reload (OnReload syncResources failed)")
	}
}
