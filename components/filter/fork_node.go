/*
 * Copyright 2023 The RuleGo Authors.
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

package filter

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "fork",
//        "name": "并行网关"
//      }
import (
	"github.com/rulego/rulego/api/types"
)

func init() {
	Registry.Add(&ForkNode{})
}

// ForkNode 并行网关节点，把流分成多个并行执行的路径
type ForkNode struct {
}

// Type 组件类型
func (x *ForkNode) Type() string {
	return "fork"
}

func (x *ForkNode) New() types.Node {
	return &ForkNode{}
}

// Init 初始化
func (x *ForkNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

// OnMsg 处理消息
func (x *ForkNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy 销毁
func (x *ForkNode) Destroy() {
}
