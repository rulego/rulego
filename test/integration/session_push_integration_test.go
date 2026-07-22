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

// TestNetSessionPush net 入+出闭环集成测试：
// 设备 TCP 连入 endpoint/net → 首帧提取 deviceId（sessionKey=${msg.deviceId}）
// → NetNode（server=ref://net_endpoint, target=DEV_001）寻址推送 → 设备收到 "HELLO\n"
//
// 与 net_endpoint_integration_test 的区别：那个测入站+同步响应（jsTransform 回写）；
// 本测试测**主动寻址推送**（session 新功能，跨请求复用连接）。
func TestNetSessionPush(t *testing.T) {
	config := types.NewConfig()

	// ① endpoint/net 启动（随机端口 + sessionKey 提取）
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

	// ② 注册 ep 到 NodePool（NetNode ref:// 的寻址目标）
	pool := node_pool.NewNodePool(config)
	if _, err := pool.AddNode(ep); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	config.NodePool = pool

	// ③ 设备 TCP 连入 + 上报 deviceId
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

	// ⑤ 触发推送
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

	// ⑥ 设备收到推送数据
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

// TestNetSessionPushBroadcast 广播：target=* 所有连接都收到
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

	// 两个设备连入
	conn1, _ := net.Dial("tcp", ep.Addr())
	defer conn1.Close()
	conn1.Write([]byte(`{"deviceId":"DEV_A"}` + "\n"))

	conn2, _ := net.Dial("tcp", ep.Addr())
	defer conn2.Close()
	conn2.Write([]byte(`{"deviceId":"DEV_B"}` + "\n"))
	time.Sleep(300 * time.Millisecond)

	// 广播
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

	// 两个设备都收到
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
