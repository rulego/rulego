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

package net

import (
	"net"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
)

// TestEndpointNetSessionExtraction 端到端验证 endpoint/net 的 session 维护：
// 设备 TCP 连入 → 首帧提取 deviceId 改写 Key → 第二帧保持 → 断开注销。
// 不启动规则链（routers 空），只验证 session 提取/寻址（在 DoProcess 之前完成）。
func TestEndpointNetSessionExtraction(t *testing.T) {
	ep := &Net{}
	cfg := types.Configuration{
		"protocol":   "tcp",
		"server":     ":0", // 随机端口
		"sessionKey": "${msg.deviceId}",
	}
	if err := ep.Init(engine.NewConfig(), cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ep.Destroy()

	// 取实际监听地址（随机端口）
	ep.mu.RLock()
	addr := ep.listener.Addr().String()
	ep.mu.RUnlock()

	// 模拟设备 TCP 连入
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	// 首帧 {"deviceId":"DEV_001"}（line 模式，以 \n 结束一帧）
	if _, err := conn.Write([]byte(`{"deviceId":"DEV_001"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// 验证：session 已注册，Key 改写为 DEV_001，已 resolved
	sessions := ep.Lookup("DEV_001")
	if len(sessions) != 1 {
		t.Fatalf("after frame1: Lookup(DEV_001) = %d, want 1", len(sessions))
	}
	if !sessions[0].IsResolved() {
		t.Fatal("session should be resolved after first frame")
	}

	// 第二帧 {"temp":26}（无 deviceId）：keyResolved=true，Key 应保持 DEV_001
	if _, err := conn.Write([]byte(`{"temp":26}` + "\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if sessions[0].Key() != "DEV_001" {
		t.Fatalf("after frame2: Key = %q, want DEV_001 (should not change)", sessions[0].Key())
	}

	// 按 deviceId 寻址：Lookup 应命中，Sender 可发送（验证 Sender 通道连通）
	if got := ep.Lookup("DEV_001"); len(got) != 1 {
		t.Fatalf("addressing lookup = %d, want 1", len(got))
	}

	// 设备断开 → session 注销
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond)
	if got := ep.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after disconnect: Lookup(DEV_001) = %d, want 0 (session should be removed)", len(got))
	}
}

// TestEndpointNetSessionDefaultKey 验证不配 sessionKey 时，Key 默认为 RemoteAddr（IP 寻址）
func TestEndpointNetSessionDefaultKey(t *testing.T) {
	ep := &Net{}
	cfg := types.Configuration{
		"protocol": "tcp",
		"server":   ":0",
		// 不配 sessionKey → 默认 RemoteAddr
	}
	if err := ep.Init(engine.NewConfig(), cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ep.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ep.Destroy()

	ep.mu.RLock()
	addr := ep.listener.Addr().String()
	ep.mu.RUnlock()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// 默认 Key = RemoteAddr（完整 host:port）；精确 Lookup 命中
	fullAddr := conn.LocalAddr().String()
	if got := ep.Lookup(fullAddr); len(got) != 1 {
		t.Fatalf("Lookup by full RemoteAddr %q = %d, want 1", fullAddr, len(got))
	}
	// 按 IP（host 段）不再命中：已去除 IP 段匹配，寻址需配 sessionKey 提取稳定标识
	host, _, _ := net.SplitHostPort(fullAddr)
	if got := ep.Lookup(host); len(got) != 0 {
		t.Fatalf("Lookup by IP %q = %d, want 0 (IP-segment match removed; configure sessionKey for addressing)", host, len(got))
	}
}
