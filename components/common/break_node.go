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

// MdKeyBreak marks the key to end the loop
const MdKeyBreak = "_break"

// MdValueBreak Loop ends marking value
const MdValueBreak = "1"

// BreakNodeConfiguration BreakNode configuration
type BreakNodeConfiguration struct {
}

// BreakNode interrupt component, used to interrupt for loop nodes
type BreakNode struct {
}

// Type returns the component type
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

// Def returns the component form definition
func (x *BreakNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Break out of a for loop. Sets _break flag in metadata to signal the for node to stop iterating",
		RelationTypes: &[]string{types.Success},
	}
}
