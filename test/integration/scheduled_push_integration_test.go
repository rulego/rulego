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

// TestNetScheduledAutoPush Rules chain automatically and schedules integrated testing.
//
// Verifying "Rule chain automatically sends data to the client": Load a root rule chain with embedded endpoint/schedule (triggered every 1 second)
// + jsTransform (transparent service deviceId) + NetNode (ref:// push addressed by ${deviceId} service ID).
// After the device TCP connects to endpoint/net and reports the deviceId, it does not manually trigger any nodes throughout the process,
// Asserts that the device automatically receives multiple scheduled pushes.
//
// Difference from session_push_integration_test: The one uses test.NodeOnMsg manually tunes NetNode.OnMsg (node-level);
// This test is driven by endpoint/scheduled, timed rule chain automatic circulation, covering the "scheduled proactive push" end-to-end link (rule chain level).
func TestNetScheduledAutoPush(t *testing.T) {
	config := rulego.NewConfig()

	// (1) endpoint/net: Receives device connection, sessionKey=${msg.deviceId} extracts the business deviceId
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

	// (2) Register in an independent NodePool as the addressing target for NetNode ref://
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// (3) Load root rule chain: schedule every 1 second → jsTransform transparent-transmitting service deviceId → NetNode ref://${deviceId}
	//    endpoint/schedule is embedded in the root chain and automatically started by EndpointAspect (the rulego package init registers the aspect)
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

	// (4) The device connects and reports the business deviceId (triggers sessionKey extraction, binding the session to DEV_001)
	conn, err := net.Dial("tcp", ep.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	// (5) Assertion: No nodes are manually triggered; the device automatically receives multiple scheduled push notifications
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
