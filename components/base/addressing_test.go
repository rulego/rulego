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
	"testing"

	"github.com/rulego/rulego/api/types"
)

// stubCtx 仅覆盖 GetEnv 的 RuleContext 桩：TargetResolver.Resolve 只调 ctx.GetEnv，
// 其余方法经嵌入的接口（测试不触达）。避免 base 包为造 ctx 而反向 import engine。
type stubCtx struct {
	types.RuleContext
	env map[string]interface{}
}

func (s *stubCtx) GetEnv(_ types.RuleMsg, _ bool) map[string]interface{} { return s.env }

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
