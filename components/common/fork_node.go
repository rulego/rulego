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

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "fork",
//        "name": "并行网关"
//      }
import (
	"github.com/rulego/rulego/api/types"
)

// init 注册ForkNode组件
// init registers the ForkNode component with the default registry.
func init() {
	Registry.Add(&ForkNode{})
}

// ForkNode 将消息流分割为多个并行执行路径的并行网关节点
// ForkNode is a parallel gateway node that splits the message flow into multiple parallel execution paths.
//
// 核心算法：
// Core Algorithm:
// 1. 接收单个输入消息 - Receive single input message
// 2. 将相同消息广播到所有连接的出站关系 - Broadcast same message to all connected outbound relations
// 3. 启动所有下游节点的并行执行 - Initiate parallel execution of all downstream nodes
//
// 工作流模式 - Workflow pattern:
//   - 并行处理的扇出模式 - Fan-out pattern for parallel processing
//   - 工作流控制的网关模式 - Gateway pattern for workflow control
//   - 消息分发的广播模式 - Broadcast pattern for message distribution
//
// 使用场景 - Use cases:
//   - 并行工作流执行 - Parallel workflow execution
//   - 向多个处理器广播消息 - Message broadcasting to multiple processors
//   - 并发操作的工作流分支 - Workflow branching for concurrent operations
//
// 无需配置 - No configuration required:
//   - 行为由规则链连接确定 - Behavior determined by rule chain connections
//   - 总是成功（没有失败情况）- Always succeeds (no failure cases)
type ForkNode struct {
	// ForkNode不需要配置字段，作为简单的消息广播器运行
	// ForkNode requires no configuration fields as it operates as a simple message broadcaster
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *ForkNode) Type() string {
	return "fork"
}

// New 创建新实例
// New creates a new instance.
func (x *ForkNode) New() types.Node {
	return &ForkNode{}
}

// Init 初始化组件
// Init initializes the component.
func (x *ForkNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

// OnMsg 处理消息，将消息广播到所有连接的出站关系进行并行处理
// OnMsg processes incoming messages by broadcasting them to all connected outbound relations.
func (x *ForkNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *ForkNode) Destroy() {
}
