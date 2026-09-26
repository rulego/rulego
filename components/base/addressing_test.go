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

package base

import (
	"errors"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// stubCtx 仅覆盖 GetEnv/RuleChain 的 RuleContext 桩：TargetResolver.Resolve 只调 ctx.GetEnv，
// ref:// 解析只调 ctx.RuleChain()，其余方法经嵌入的接口（测试不触达）。
// 避免 base 包为造 ctx 而反向 import engine。
type stubCtx struct {
	types.RuleContext
	env     map[string]interface{}
	useMeta []bool        // records the useMetadata flag of each GetEnv call
	chain   types.NodeCtx // returned by RuleChain; nil means no chain
}

func (s *stubCtx) GetEnv(_ types.RuleMsg, useMetadata bool) map[string]interface{} {
	s.useMeta = append(s.useMeta, useMetadata)
	return s.env
}

func (s *stubCtx) RuleChain() types.NodeCtx { return s.chain }

// TestTargetResolver_Empty 空配置：IsEmpty 且 Resolve 返回空串（字面量分支不触达 ctx）。
func TestTargetResolver_Empty(t *testing.T) {
	r := NewTargetResolver("")
	if !r.IsEmpty() {
		t.Fatal("IsEmpty should be true for empty config")
	}
	if r.Literal() != "" {
		t.Fatalf("Literal=%q want empty", r.Literal())
	}
	if got := r.Resolve(nil, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "" {
		t.Fatalf("empty Resolve got %q want empty", got)
	}
}

// TestTargetResolver_Literal 非表达式字面量原样返回。
func TestTargetResolver_Literal(t *testing.T) {
	r := NewTargetResolver("192.168.1.100")
	if r.IsEmpty() {
		t.Fatal("IsEmpty should be false for literal")
	}
	if r.Literal() != "192.168.1.100" {
		t.Fatalf("Literal=%q", r.Literal())
	}
	// 字面量也会被 el 编译成模板，故 Resolve 同样走 ctx.GetEnv 分支，需有效 ctx
	if got := r.Resolve(&stubCtx{env: map[string]interface{}{}}, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "192.168.1.100" {
		t.Fatalf("literal Resolve got %q want 192.168.1.100", got)
	}
}

// TestTargetResolver_Star 广播标记 "*" 原样返回。
func TestTargetResolver_Star(t *testing.T) {
	r := NewTargetResolver("*")
	if got := r.Resolve(&stubCtx{env: map[string]interface{}{}}, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "*" {
		t.Fatalf("star Resolve got %q want *", got)
	}
}

// TestTargetResolver_MsgExpression ${msg.deviceId} 从 ctx.GetEnv 环境解析。
func TestTargetResolver_MsgExpression(t *testing.T) {
	r := NewTargetResolver("${msg.deviceId}")
	ctx := &stubCtx{env: map[string]interface{}{
		"msg": map[string]interface{}{"deviceId": "DEV_42"},
	}}
	if got := r.Resolve(ctx, types.NewMsg(0, "", types.JSON, types.NewMetadata(), "")); got != "DEV_42" {
		t.Fatalf("msg expr Resolve got %q want DEV_42", got)
	}
}

// TestTargetResolver_MetadataExpression ${metadata.host} 与平铺 ${host} 均可解析
// （GetEvnAndMetadata 标准环境同时提供两者，TargetResolver 不自建环境）。
func TestTargetResolver_MetadataExpression(t *testing.T) {
	r := NewTargetResolver("${metadata.host}")
	ctx := &stubCtx{env: map[string]interface{}{
		"metadata": map[string]interface{}{"host": "10.0.0.1"},
		"host":     "10.0.0.1",
	}}
	if got := r.Resolve(ctx, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "10.0.0.1" {
		t.Fatalf("metadata expr Resolve got %q want 10.0.0.1", got)
	}
}

// TestTargetResolver_NestedMsgExpression ${msg.header.sn} 嵌套字段解析。
func TestTargetResolver_NestedMsgExpression(t *testing.T) {
	r := NewTargetResolver("${msg.header.sn}")
	ctx := &stubCtx{env: map[string]interface{}{
		"msg": map[string]interface{}{"header": map[string]interface{}{"sn": "SN-99"}},
	}}
	if got := r.Resolve(ctx, types.NewMsg(0, "", types.JSON, types.NewMetadata(), "")); got != "SN-99" {
		t.Fatalf("nested msg expr Resolve got %q want SN-99", got)
	}
}

// fakeSender records the addressing call and returns canned results.
type fakeSender struct {
	target string
	data   []byte
	sent   int
	failed int
	err    error
}

func (f *fakeSender) SendToTarget(target string, data []byte) (int, int, error) {
	f.target, f.data = target, data
	return f.sent, f.failed, f.err
}

// TestResolveResourcePoolFallback pool 回退：reg miss 后查 NodePool.Lookup。
func TestResolveResourcePoolFallback(t *testing.T) {
	reg := &fakeLookup{items: map[string]any{}}
	pool := &stubNodePool{items: map[string]any{"p": "pool-v"}}
	if v, ok := ResolveResource(reg, pool, "p"); !ok || v != "pool-v" {
		t.Fatalf("pool fallback: got %v ok %v", v, ok)
	}
	if _, ok := ResolveResource(reg, pool, "missing"); ok {
		t.Fatal("expected not found when both reg and pool miss")
	}
}

// TestResolveResourceFromCtx 解析跟随消息链：ctx.RuleChain() 为 ChainCtx 时用链目录，
// 否则只查 pool。
func TestResolveResourceFromCtx(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	chain.reg.Register("k", "chain-v")
	pool := &stubNodePool{items: map[string]any{"k": "pool-v", "p": "pool-p"}}

	// chain directory wins over the pool entry with the same id
	if v, ok := ResolveResourceFromCtx(&stubCtx{chain: chain}, pool, "k"); !ok || v != "chain-v" {
		t.Fatalf("chain hit: got %v ok %v", v, ok)
	}
	// chain miss falls back to the pool
	if v, ok := ResolveResourceFromCtx(&stubCtx{chain: chain}, pool, "p"); !ok || v != "pool-p" {
		t.Fatalf("pool fallback: got %v ok %v", v, ok)
	}
	// RuleChain() returning a plain NodeCtx (not ChainCtx) skips the directory
	plain := struct{ types.NodeCtx }{}
	if v, ok := ResolveResourceFromCtx(&stubCtx{chain: plain}, pool, "p"); !ok || v != "pool-p" {
		t.Fatalf("non-chain ctx: got %v ok %v", v, ok)
	}
	// nil RuleChain: only the pool is consulted
	if v, ok := ResolveResourceFromCtx(&stubCtx{}, pool, "k"); !ok || v != "pool-v" {
		t.Fatalf("nil chain should fall back to pool: got %v ok %v", v, ok)
	}
	if _, ok := ResolveResourceFromCtx(&stubCtx{}, nil, "p"); ok {
		t.Fatal("expected not found when chain nil and pool nil")
	}
}

// TestLoadConnFromCtx 连接借用跟随消息链：链目录命中 holder；非链 ctx 回退 pool。
func TestLoadConnFromCtx(t *testing.T) {
	a := 1
	holder := &connHolder[*int]{}
	holder.store(&a)
	chain := &stubChainCtx{reg: newStubRegistry()}
	chain.reg.Register("h", holder)
	pool := &stubNodePool{items: map[string]any{"p": holder}}

	if v, ok := LoadConnFromCtx[*int](&stubCtx{chain: chain}, nil, "h"); !ok || v != &a {
		t.Fatalf("chain holder: got %v ok %v", v, ok)
	}
	if v, ok := LoadConnFromCtx[*int](&stubCtx{}, pool, "p"); !ok || v != &a {
		t.Fatalf("pool holder: got %v ok %v", v, ok)
	}
	if _, ok := LoadConnFromCtx[*int](&stubCtx{}, pool, "missing"); ok {
		t.Fatal("expected miss for unknown id")
	}
	chain.reg.Register("s", "not-a-holder")
	if _, ok := LoadConnFromCtx[*int](&stubCtx{chain: chain}, nil, "s"); ok {
		t.Fatal("expected false for non-holder chain entry")
	}
}

// TestSendToRefTarget 按 target 寻址推送：未找到 / 非 TargetSender / 正常投递 / 投递错误。
func TestSendToRefTarget(t *testing.T) {
	chain := &stubChainCtx{reg: newStubRegistry()}
	ctx := &stubCtx{chain: chain}

	// not found in chain or pool
	_, _, err := SendToRefTarget(ctx, nil, "nope", "t1", []byte("d"))
	if !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("miss: err=%v, want ErrResourceNotFound", err)
	}

	// found but not addressable
	chain.reg.Register("raw", 123)
	_, _, err = SendToRefTarget(ctx, nil, "raw", "t1", []byte("d"))
	if !errors.Is(err, ErrNotTargetSender) {
		t.Fatalf("non-sender: err=%v, want ErrNotTargetSender", err)
	}

	// delivery results pass through unchanged
	fs := &fakeSender{sent: 2, failed: 1}
	chain.reg.Register("sender", fs)
	sent, failed, err := SendToRefTarget(ctx, nil, "sender", "dev-1", []byte("payload"))
	if err != nil {
		t.Fatalf("send: err=%v", err)
	}
	if sent != 2 || failed != 1 {
		t.Fatalf("send counts: sent=%d failed=%d, want 2/1", sent, failed)
	}
	if fs.target != "dev-1" || string(fs.data) != "payload" {
		t.Fatalf("sender args: target=%q data=%q", fs.target, fs.data)
	}

	// the sender's first error is returned
	sendErr := errors.New("peer reset")
	failing := &fakeSender{err: sendErr}
	chain.reg.Register("failing", failing)
	_, _, err = SendToRefTarget(ctx, nil, "failing", "t1", nil)
	if !errors.Is(err, sendErr) {
		t.Fatalf("sender error: err=%v, want %v", err, sendErr)
	}
}

// TestSendToRefTargetPoolResolution 目标在 NodePool（非链目录）也可寻址。
func TestSendToRefTargetPoolResolution(t *testing.T) {
	fs := &fakeSender{sent: 1}
	pool := &stubNodePool{items: map[string]any{"p1": fs}}
	sent, _, err := SendToRefTarget(&stubCtx{}, pool, "p1", "*", []byte("b"))
	assert.Nil(t, err)
	assert.Equal(t, 1, sent)
}
