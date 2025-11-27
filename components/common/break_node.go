/*
 * Copyright 2025 The RuleGo Authors.
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

package common

import (
	"github.com/rulego/rulego/api/types"
)

func init() {
	Registry.Add(&BreakNode{})
}

// MdKeyBreak 循环结束标记key
const MdKeyBreak = "_break"

// MdValueBreak 循环结束标记value
const MdValueBreak = "1"

// BreakNodeConfiguration BreakNode配置
type BreakNodeConfiguration struct {
}

// BreakNode 中断组件，用于中断 for 循环节点
type BreakNode struct {
}

// Type 返回组件类型
func (x *BreakNode) Type() string {
	return "break"
}

func (x *BreakNode) New() types.Node {
	return &BreakNode{}
}

func (x *BreakNode) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

func (x *BreakNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.GetMetadata().PutValue(MdKeyBreak, MdValueBreak)
	ctx.TellSuccess(msg)
}

func (x *BreakNode) Destroy() {
}
