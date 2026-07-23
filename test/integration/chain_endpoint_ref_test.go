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

// TestChainEndpointRefPush verifies ref:// refers to the "same-chain endpoint" for addressing push (Solution B).
//
// endpoint/net is defined within the rule chain metadata.endpoints (not entered into NodePool),
// NetNode ref://ep_net should be parsed via the same-chain ResourceRegistry (resolveRef takes precedence over on-chain),
// Then SendToTarget presses deviceId to address and push the target. Once the device is connected, it automatically receives scheduled push notifications.
//
// Difference from TestNetScheduledAutoPush: The endpoint is registered in the shared pool NodePool (ref goes to NodePool);
// This test uses endpoints on-chain (ref runs on the same chain as Resources), verifying the new general resource directory mechanism.
func TestChainEndpointRefPush(t *testing.T) {
	// Enable endpoints; NodePool → ref:// deliberately omitted requires same-chain resolution
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

	// Fetch the on-chain endpoint instance from the engine rootChainCtx's resource directory (verify that EndpointAspect registration is valid)
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

	// Device connects and reports business deviceId (triggers sessionKey extraction, session binds DEV_001)
	conn, err := net.Dial("tcp", ep.Addr())
	if err != nil {
		t.Fatalf("Dial %s: %v", ep.Addr(), err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	// Assertion: Device automatically receives scheduled push notifications (ref:// Same-chain addressing successful)
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
	t.Logf("PASS: ref:// Same-chain endpoint addressed push %d successfully", count)
}

// TestChainEndpointRefReload After verifying the reload rule chain, the same-chain endpoint is still registered in the resource directory
// (Re-register syncResources in EndpointAspect.OnReload), ref:// same-chain resolution remains intact.
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

	// Before reload: OnCreated has registered ep_net to the resource directory
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_net"); !found {
		t.Fatal("ep_net not registered before reload (OnCreated syncResources failed)")
	}

	// reload the rule chain
	if err := eng.ReloadSelf([]byte(chain)); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// After reload: OnReload should re-register the ep_net, ref:// same-chain resolution will not be disabled
	if _, found := ruleEng.RootRuleChainCtx().Resources().Lookup("ep_net"); !found {
		t.Fatal("ep_net not registered after reload (OnReload syncResources failed)")
	}
}
