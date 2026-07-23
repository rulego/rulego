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
	"github.com/rulego/rulego/test"
)

// TestNetOut net outbound integration testing:
// NetNode acts as a TCP client dial-up to connect to the remote server→ sending data → server receives.
// Corresponding to session_push_integration_test (Inbound Addressing Push), this test verifies the main outbound direct dispatch path.
func TestNetOut(t *testing.T) {
	config := types.NewConfig()

	// (1) Start a TCP server and read data and plug the chan
	received := make(chan []byte, 4)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go serveTCPAccept(ln, received)

	// (2) NetNode dial for this server
	node := &external.NetNode{}
	if err := node.Init(config, types.Configuration{
		"protocol":          "tcp",
		"server":            ln.Addr().String(),
		"connectTimeout":    5,
		"heartbeatInterval": 0, // Disable heartbeat, focus on verifying outbound sending
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// (3) Send text (NetNode will automatically add a \n terminator if not binary)
	done := make(chan error, 1)
	test.NodeOnMsg(t, node, []test.Msg{{Data: "PING", DataType: types.TEXT}}, func(m types.RuleMsg, rel string, err error) {
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

	// (4) server receives "PING\n"
	select {
	case got := <-received:
		if string(got) != "PING\n" {
			t.Fatalf("server got %q, want \"PING\\n\"", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting server receive")
	}
}

// serveTCPAccept accepts connections, and each connection receives data from the data block
func serveTCPAccept(ln net.Listener, received chan<- []byte) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 1024)
			for {
				n, err := c.Read(buf)
				if n > 0 {
					cp := make([]byte, n)
					copy(cp, buf[:n])
					select {
					case received <- cp:
					default:
					}
				}
				if err != nil {
					return
				}
			}
		}(conn)
	}
}
