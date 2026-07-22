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

// TestNetOut net 出站集成测试：
// NetNode 作为 TCP 客户端拨号连远端 server → 发送数据 → server 收到。
// 与 session_push_integration_test（入站寻址推送）对应，本测试验证出站直发主路径。
func TestNetOut(t *testing.T) {
	config := types.NewConfig()

	// ① 起一个 TCP server，读到数据塞 chan
	received := make(chan []byte, 4)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go serveTCPAccept(ln, received)

	// ② NetNode dial 该 server
	node := &external.NetNode{}
	if err := node.Init(config, types.Configuration{
		"protocol":          "tcp",
		"server":            ln.Addr().String(),
		"connectTimeout":    5,
		"heartbeatInterval": 0, // 禁用心跳，专注验证出站发送
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer node.Destroy()

	// ③ 发送文本（NetNode 非 binary 会自动追加 \n 结束符）
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

	// ④ server 收到 "PING\n"
	select {
	case got := <-received:
		if string(got) != "PING\n" {
			t.Fatalf("server got %q, want \"PING\\n\"", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting server receive")
	}
}

// serveTCPAccept 接受连接，每条连接读到的数据塞 received
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
