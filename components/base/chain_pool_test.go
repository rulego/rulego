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
)

// fakeLookup 实现 types.ResourceLookup，用于测试。
type fakeLookup struct {
	items map[string]any
}

func (f *fakeLookup) Lookup(id string) (any, bool) {
	v, ok := f.items[id]
	return v, ok
}

// TestConnHolder 验证稳定间接层的存取语义：重连只更新内部值，目录条目（holder 指针）不变。
func TestConnHolder(t *testing.T) {
	var h connHolder[*int]
	a, b := 1, 2
	h.store(&a)
	if got := h.load(); got != &a || *got != 1 {
		t.Fatalf("load after store a: got %v", got)
	}
	h.store(&b) // 模拟重连更新
	if got := h.load(); got != &b || *got != 2 {
		t.Fatalf("load after store b: got %v", got)
	}
}

// TestResolveResource 验证解析顺序：reg 目录优先；reg miss 且 pool=nil 时未命中。
// pool 回退路径（pool.Lookup）由集成测试覆盖（需完整 NodePool）。
func TestResolveResource(t *testing.T) {
	reg := &fakeLookup{items: map[string]any{"k": "v"}}
	if v, ok := ResolveResource(reg, nil, "k"); !ok || v != "v" {
		t.Fatalf("reg hit: got %v ok %v", v, ok)
	}
	if _, ok := ResolveResource(reg, nil, "missing"); ok {
		t.Fatal("expected not found when reg miss and pool nil")
	}
	if _, ok := ResolveResource(nil, nil, "k"); ok {
		t.Fatal("expected not found when reg nil and pool nil")
	}
}

// TestLoadConn 验证连接借用解包：命中 holder 取最新；跨类型 / 未命中 / nil 连接返回 false。
func TestLoadConn(t *testing.T) {
	a := 1
	holder := &connHolder[*int]{}
	holder.store(&a)
	reg := &fakeLookup{items: map[string]any{"k": holder}}
	if v, ok := LoadConn[*int](reg, nil, "k"); !ok || v != &a {
		t.Fatalf("LoadConn hit: got %v ok %v", v, ok)
	}
	// 命中但非 holder（跨类型 ref）→ false
	regWrong := &fakeLookup{items: map[string]any{"k": "not-a-holder"}}
	if _, ok := LoadConn[*int](regWrong, nil, "k"); ok {
		t.Fatal("expected false for non-holder type")
	}
	// 未命中 → false
	if _, ok := LoadConn[*int](reg, nil, "missing"); ok {
		t.Fatal("expected false for miss")
	}
	// holder 存在但连接为 nil（零值）→ false
	nilHolder := &connHolder[*int]{}
	regNil := &fakeLookup{items: map[string]any{"k": nilHolder}}
	if _, ok := LoadConn[*int](regNil, nil, "k"); ok {
		t.Fatal("expected false for nil connection in holder")
	}
}

// TestErrSentinels 确保错误哨兵可被 errors.Is 识别（net_node 兜底分支依赖此）。
func TestErrSentinels(t *testing.T) {
	if !errors.Is(ErrNotTargetSender, ErrNotTargetSender) {
		t.Fatal("ErrNotTargetSender not identifiable")
	}
	if !errors.Is(ErrResourceNotFound, ErrResourceNotFound) {
		t.Fatal("ErrResourceNotFound not identifiable")
	}
}
