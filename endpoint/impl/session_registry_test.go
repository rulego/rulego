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

package impl

import (
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

func TestDefaultSessionRegistry_AddRemoveLookup(t *testing.T) {
	r := &DefaultSessionRegistry{}
	s1 := endpoint.NewSession("192.168.1.10:1000", nil)
	s2 := endpoint.NewSession("192.168.1.10:1001", nil)
	s3 := endpoint.NewSession("DEV_001", nil)
	r.Add(s1)
	r.Add(s2)
	r.Add(s3)
	r.Add(nil) // nil 容忍

	// 精确匹配 Key
	if got := r.Lookup("DEV_001"); len(got) != 1 || got[0] != s3 {
		t.Fatalf("exact lookup DEV_001 = %v, want [s3]", got)
	}
	// 精确匹配 IP:port（不再做 host 段匹配）
	if got := r.Lookup("192.168.1.10:1000"); len(got) != 1 || got[0] != s1 {
		t.Fatalf("exact lookup 192.168.1.10:1000 = %v, want [s1]", got)
	}
	// 裸 IP 不再命中 host:port 连接（已去除 ipOf 隐式匹配）
	if got := r.Lookup("192.168.1.10"); len(got) != 0 {
		t.Fatalf("bare ip should not match host:port keys, got %d", len(got))
	}
	// 广播 *
	if got := r.Lookup("*"); len(got) != 3 {
		t.Fatalf("broadcast * = %d, want 3", len(got))
	}
	// 广播 空
	if got := r.Lookup(""); len(got) != 3 {
		t.Fatalf("broadcast empty = %d, want 3", len(got))
	}
	// 无匹配
	if got := r.Lookup("UNKNOWN"); len(got) != 0 {
		t.Fatalf("no-match = %d, want 0", len(got))
	}
	// Remove 后查不到
	r.Remove("DEV_001")
	if got := r.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after remove = %d, want 0", len(got))
	}
	// Remove 不存在的 key 不 panic
	r.Remove("NOT_EXIST")
}

func TestSessionSetKey(t *testing.T) {
	s := endpoint.NewSession("OLD", nil)
	if s.Key() != "OLD" || s.IsResolved() {
		t.Fatal("initial state wrong")
	}
	s.SetKey("NEW")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("after SetKey: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
	// 空 key 被忽略（不清空、不误标记）
	s.SetKey("")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("empty key ignored: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
}

func TestRekey(t *testing.T) {
	r := &DefaultSessionRegistry{}
	s := endpoint.NewSession("OLD", nil)
	r.Add(s)

	r.Rekey(s, "NEW")
	if s.Key() != "NEW" || !s.IsResolved() {
		t.Fatalf("after Rekey: Key=%q resolved=%v", s.Key(), s.IsResolved())
	}
	if got := r.Lookup("NEW"); len(got) != 1 || got[0] != s {
		t.Fatalf("lookup NEW = %v, want [s]", got)
	}
	if got := r.Lookup("OLD"); len(got) != 0 {
		t.Fatalf("old key should be deregistered, got %d", len(got))
	}
	// Rekey 空 key 被忽略
	r.Rekey(s, "")
	if s.Key() != "NEW" {
		t.Fatalf("Rekey empty should be ignored, got %q", s.Key())
	}
	// Rekey nil session 不 panic
	r.Rekey(nil, "X")
}
// closerSender 测试用 Sender，记录 Close 调用次数。
type closerSender struct {
	closed int
}

func (s *closerSender) Send(data []byte) error { return nil }
func (s *closerSender) Close() error           { s.closed++; return nil }

// nonCloserSender 只实现 Send（非 io.Closer），验证 TTL sweep 的 graceful 降级。
type nonCloserSender struct{}

func (s *nonCloserSender) Send(data []byte) error { return nil }

func TestSweep_EvictsExpired(t *testing.T) {
	r := &DefaultSessionRegistry{}
	cs := &closerSender{}
	r.Add(endpoint.NewSession("k1", cs))
	evicted := r.sweep(time.Now().Add(time.Hour), time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1", evicted)
	}
	if len(r.Lookup("*")) != 0 {
		t.Fatal("registry should be empty after sweep")
	}
	if cs.closed != 1 {
		t.Fatalf("Close not called once, got %d", cs.closed)
	}
}

func TestSweep_KeepsActive(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.Add(endpoint.NewSession("k1", &closerSender{}))
	evicted := r.sweep(time.Now(), time.Hour)
	if evicted != 0 {
		t.Fatalf("evicted=%d, want 0", evicted)
	}
	if len(r.Lookup("*")) != 1 {
		t.Fatal("active session should remain")
	}
}

func TestSweep_MixedAges(t *testing.T) {
	r := &DefaultSessionRegistry{}
	old := endpoint.NewSession("old", &closerSender{})
	old.TouchAt(time.Now().Add(-time.Hour))
	r.Add(old)
	r.Add(endpoint.NewSession("fresh", &closerSender{}))
	evicted := r.sweep(time.Now(), 30*time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1 (only old)", evicted)
	}
	all := r.Lookup("*")
	if len(all) != 1 || all[0].Key() != "fresh" {
		t.Fatalf("should keep only fresh, got %v", all)
	}
}

func TestSweep_NonCloserSender(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.Add(endpoint.NewSession("k1", &nonCloserSender{}))
	evicted := r.sweep(time.Now().Add(time.Hour), time.Second)
	if evicted != 1 {
		t.Fatalf("evicted=%d, want 1 (Remove without Close)", evicted)
	}
	if len(r.Lookup("*")) != 0 {
		t.Fatal("registry should be empty")
	}
}

func TestStartSweeping_StopsCleanly(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.StartSweeping(10*time.Millisecond, 5*time.Millisecond)
	r.StartSweeping(10*time.Millisecond, 5*time.Millisecond) // 重复 no-op
	r.StopSweeping()
	r.StopSweeping() // 幂等
}

func TestStartSweeping_Disabled(t *testing.T) {
	r := &DefaultSessionRegistry{}
	r.StartSweeping(0, 0) // ttl<=0 no-op
	r.StopSweeping()
}

func TestSession_TouchUpdatesLastSeen(t *testing.T) {
	s := endpoint.NewSession("k1", &closerSender{})
	old := s.LastSeen()
	time.Sleep(2 * time.Millisecond)
	s.Touch()
	if s.LastSeen() <= old {
		t.Fatal("Touch should advance lastSeen")
	}
}
// fakeSender 记录收到的数据帧，用于验证寻址推送
type fakeSender struct {
	received [][]byte
}

func (f *fakeSender) Send(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	f.received = append(f.received, cp)
	return nil
}

// TestSessionKeyExtractionFlow 模拟端到端 sessionKey 提取与寻址推送流程，
// 不启动真实 TCP，验证 DefaultSessionRegistry + SessionKeyResolver + Session 三者协作。
//
// 流程：
//  1. 连接建立：session 初始 Key=RemoteAddr，未 resolved
//  2. 首帧 {"deviceId":"DEV_001"}：提取 key → 注销旧 Key → SetKey → 注册新 Key
//  3. 第二帧 {"temp":26}（无 deviceId）：IsResolved=true 跳过提取，Key 保持
//  4. 业务按 deviceId 寻址推送：Lookup("DEV_001") → Send
//  5. 广播：Lookup("*") → Send
func TestSessionKeyExtractionFlow(t *testing.T) {
	registry := &DefaultSessionRegistry{}
	resolver := NewSessionKeyResolver("${msg.deviceId}")

	// ① 连接建立
	sender := &fakeSender{}
	session := endpoint.NewSession("192.168.1.10:5000", sender)
	registry.Add(session)
	if session.IsResolved() {
		t.Fatal("new session should not be resolved")
	}

	// ② 首帧提取
	firstFrame := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"deviceId":"DEV_001"}`)
	if !session.IsResolved() {
		key := resolver.Resolve(firstFrame, nil)
		if key != "DEV_001" {
			t.Fatalf("resolve got %q, want DEV_001", key)
		}
		registry.Rekey(session, key) // 原子改键：注销旧 Key + SetKey + 注册新 Key
	}

	if !session.IsResolved() {
		t.Fatal("should be resolved after first frame")
	}
	if session.Key() != "DEV_001" {
		t.Fatalf("session.Key = %q, want DEV_001", session.Key())
	}
	if got := registry.Lookup("DEV_001"); len(got) != 1 || got[0] != session {
		t.Fatalf("lookup DEV_001 = %v, want [session]", got)
	}
	if got := registry.Lookup("192.168.1.10:5000"); len(got) != 0 {
		t.Fatalf("old key should be deregistered, got %d", len(got))
	}

	// ③ 第二帧：IsResolved=true，handler 应跳过提取（这里不调用 Resolve）
	if !session.IsResolved() {
		t.Fatal("expected to skip extraction when resolved")
	}

	// ④ 按 deviceId 寻址推送
	targets := registry.Lookup("DEV_001")
	if len(targets) != 1 {
		t.Fatalf("lookup for push = %d, want 1", len(targets))
	}
	if err := targets[0].Sender.Send([]byte("HELLO_DEVICE")); err != nil {
		t.Fatalf("send err: %v", err)
	}
	if len(sender.received) != 1 || string(sender.received[0]) != "HELLO_DEVICE" {
		t.Fatalf("sender received %v, want [HELLO_DEVICE]", sender.received)
	}

	// ⑤ 广播
	for _, s := range registry.Lookup("*") {
		if err := s.Sender.Send([]byte("BROADCAST")); err != nil {
			t.Fatalf("broadcast send err: %v", err)
		}
	}
	if len(sender.received) != 2 || string(sender.received[1]) != "BROADCAST" {
		t.Fatalf("after broadcast received %v, want 2 frames", sender.received)
	}

	// ⑥ 设备断开：注销 session
	registry.Remove(session.Key())
	if got := registry.Lookup("DEV_001"); len(got) != 0 {
		t.Fatalf("after disconnect should be removed, got %d", len(got))
	}
}

// TestSessionKeyExtractionMultiCandidateFlow 验证多候选场景的提取流程
// （私有 hex 协议设备：JSON 无字段时从字节提取）
func TestSessionKeyExtractionMultiCandidateFlow(t *testing.T) {
	registry := &DefaultSessionRegistry{}
	resolver := NewSessionKeyResolver([]string{"${msg.deviceId}", "${data[0:6]}"})

	session := endpoint.NewSession("10.0.0.1:9", &fakeSender{})
	registry.Add(session)

	// 私有协议帧：JSON 解析失败 / 无 deviceId → 回退 offset 取前 6 字节
	data := []byte("SN0001") // offset:0,len:6
	key := resolver.Resolve(jsonMsg("{}"), data)
	if key != "SN0001" {
		t.Fatalf("got %q, want SN0001", key)
	}
	registry.Rekey(session, key)

	if got := registry.Lookup("SN0001"); len(got) != 1 {
		t.Fatalf("lookup SN0001 = %d, want 1", len(got))
	}
}
