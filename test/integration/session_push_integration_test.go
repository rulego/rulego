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

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/external"
	netep "github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/test"
)

// TestNetSessionPush net In+Out closed-loop integrated testing:
// Device TCP connects to endpoint/net → first frame extracts deviceId(sessionKey=${msg.deviceId})
// → NetNode(server=ref://net_endpoint, target=DEV_001) addressing push → device receives "HELLO\n"
//
// Difference from net_endpoint_integration_test: which is the incoming station + synchronous response (jsTransform writeback);
// This test tested **active addressing push** (new session feature, cross-request multiplexing connection).
func TestNetSessionPush(t *testing.T) {
	config := types.NewConfig()

	// (1) endpoint/net startup (random port + sessionKey extraction)
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

	// (2) Register ep to NodePool (the addressing target of NetNode ref://)
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// (3) Device TCP connection + deviceId reported
	conn, err := net.Dial("tcp", ep.Addr())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// ④ NetNode（server=ref://<ep.Id()>，target=DEV_001）
	node := &external.NetNode{}
	if err := node.Init(config, types.Configuration{
		"server": "ref://" + ep.Id(),
		"target": "DEV_001",
	}); err != nil {
		t.Fatalf("NetNode Init: %v", err)
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
			t.Fatalf("NetNode OnMsg err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting NetNode OnMsg callback")
	}

	// (6) The device receives the push data
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("device read: %v", err)
	}
	if string(buf[:n]) != "HELLO\n" {
		t.Fatalf("device got %q, want \"HELLO\\n\"", buf[:n])
	}
}

// TestNetSessionPushBroadcast Broadcast: target=* All connections are received
func TestNetSessionPushBroadcast(t *testing.T) {
	config := types.NewConfig()

	ep := &netep.Net{}
	if err := ep.Init(config, types.Configuration{
		"protocol": "tcp", "server": ":0", "sessionKey": "${msg.deviceId}",
	}); err != nil {
		t.Fatalf("ep Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("ep Start: %v", err)
	}
	defer ep.Destroy()

	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// Two devices connected
	conn1, _ := net.Dial("tcp", ep.Addr())
	defer conn1.Close()
	conn1.Write([]byte(`{"deviceId":"DEV_A"}` + "\n"))

	conn2, _ := net.Dial("tcp", ep.Addr())
	defer conn2.Close()
	conn2.Write([]byte(`{"deviceId":"DEV_B"}` + "\n"))
	time.Sleep(300 * time.Millisecond)

	// Broadcast
	node := &external.NetNode{}
	node.Init(config, types.Configuration{"server": "ref://" + ep.Id(), "target": "*"})
	defer node.Destroy()

	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "BCAST", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
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

	// Both devices were received
	for i, c := range []net.Conn{conn1, conn2} {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 100)
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("device %d read: %v", i, err)
		}
		if string(buf[:n]) != "BCAST\n" {
			t.Fatalf("device %d got %q, want BCAST\\n", i, buf[:n])
		}
	}
}
