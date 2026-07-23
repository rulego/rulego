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

// stubCtx only overrides GetEnv's RuleContext stub: TargetResolver.Resolve only calls ctx.GetEnv,
// The remaining methods go through embedded interfaces (not reached by testing). Avoid using the base package to import the engine backward for CTX purposes.
type stubCtx struct {
	types.RuleContext
	env map[string]interface{}
}

func (s *stubCtx) GetEnv(_ types.RuleMsg, _ bool) map[string]interface{} { return s.env }

// TestTargetResolver_Empty Null configuration: IsEmpty and Resolve returns an empty string (literal branches do not reach ctx).
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

// TestTargetResolver_Literal Returns non-expression literals as they are.
func TestTargetResolver_Literal(t *testing.T) {
	r := NewTargetResolver("192.168.1.100")
	if r.IsEmpty() {
		t.Fatal("IsEmpty should be false for literal")
	}
	if r.Literal() != "192.168.1.100" {
		t.Fatalf("Literal=%q", r.Literal())
	}
	// literal values are also compiled into templates by el, so Resolve also uses ctx.GetEnv branch, requires a valid ctx
	if got := r.Resolve(&stubCtx{env: map[string]interface{}{}}, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "192.168.1.100" {
		t.Fatalf("literal Resolve got %q want 192.168.1.100", got)
	}
}

// TestTargetResolver_Star Return the broadcast mark "*" as is.
func TestTargetResolver_Star(t *testing.T) {
	r := NewTargetResolver("*")
	if got := r.Resolve(&stubCtx{env: map[string]interface{}{}}, types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "")); got != "*" {
		t.Fatalf("star Resolve got %q want *", got)
	}
}

// TestTargetResolver_MsgExpression ${msg.deviceId} from ctx.GetEnv environment analysis.
func TestTargetResolver_MsgExpression(t *testing.T) {
	r := NewTargetResolver("${msg.deviceId}")
	ctx := &stubCtx{env: map[string]interface{}{
		"msg": map[string]interface{}{"deviceId": "DEV_42"},
	}}
	if got := r.Resolve(ctx, types.NewMsg(0, "", types.JSON, types.NewMetadata(), "")); got != "DEV_42" {
		t.Fatalf("msg expr Resolve got %q want DEV_42", got)
	}
}

// TestTargetResolver_MetadataExpression ${metadata.host} and tiling ${host} can both be parsed
// (GetEvnAndMetadata standard environment provides both, while TargetResolver does not build its own environment.)
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

// TestTargetResolver_NestedMsgExpression ${msg.header.sn} Nested field parsing.
func TestTargetResolver_NestedMsgExpression(t *testing.T) {
	r := NewTargetResolver("${msg.header.sn}")
	ctx := &stubCtx{env: map[string]interface{}{
		"msg": map[string]interface{}{"header": map[string]interface{}{"sn": "SN-99"}},
	}}
	if got := r.Resolve(ctx, types.NewMsg(0, "", types.JSON, types.NewMetadata(), "")); got != "SN-99" {
		t.Fatalf("nested msg expr Resolve got %q want SN-99", got)
	}
}
