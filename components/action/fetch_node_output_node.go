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

package action

//规则链节点配置示例：
//{
//        "id": "s1",
//        "type": "nodeOutput",
//        "name": "节点输出获取",
//        "debugMode": false,
//        "configuration": {
//          "nodeId": "targetNodeId",
//          "fallbackToCurrentMsg": true
//        }
//  }
import (
	"errors"
	"fmt"

	"github.com/rulego/rulego/components/base"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
)

// 注册节点
func init() {
	Registry.Add(&FetchNodeOutputNode{})
}

// NodeOutputNodeConfiguration 节点配置
type NodeOutputNodeConfiguration struct {
	// NodeId 目标节点ID，获取该节点的输出消息
	NodeId string
}

// FetchNodeOutputNode 获取指定节点输出的组件
// FetchNodeOutputNode retrieves the output of a specified node and passes it to the next node.
//
// 核心功能：
// Core functionality:
// 1. 通过nodeId获取目标节点的输出消息 - Retrieve target node's output message by nodeId
// 2. 将获取到的消息传递给下一个节点 - Pass the retrieved message to the next node
// 3. 自动建立节点依赖关系以启用输出缓存 - Automatically establish node dependency to enable output caching
//
// 依赖关系机制：
// Dependency mechanism:
// - 在Init()阶段自动调用chainCtx.AddNodeDependency()建立依赖关系
// - 只有建立依赖关系的节点才会缓存输出数据
// - 确保目标节点的输出能够被GetNodeRuleMsg()访问
// - Automatically calls chainCtx.AddNodeDependency() during Init() to establish dependency
// - Only nodes with established dependencies will cache output data
// - Ensures target node output can be accessed via GetNodeRuleMsg()
//
// 使用场景：
// Use cases:
// - 跨节点数据传递 - Cross-node data passing
// - 节点输出复用 - Node output reuse
// - 条件分支合并 - Conditional branch merging
type FetchNodeOutputNode struct {
	// 节点配置
	Config NodeOutputNodeConfiguration
}

// Type 组件类型
func (x *FetchNodeOutputNode) Type() string {
	return "fetchNodeOutput"
}

// New 创建新实例
func (x *FetchNodeOutputNode) New() types.Node {
	return &FetchNodeOutputNode{
		Config: NodeOutputNodeConfiguration{},
	}
}

// Init initializes the node
// Establishes node dependency during initialization to ensure target node output is cached
func (x *FetchNodeOutputNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}
	chainCtx := base.NodeUtils.GetChainCtx(configuration)
	if chainCtx == nil {
		return errors.New("chain ctx is nil")
	}
	self := base.NodeUtils.GetSelfDefinition(configuration)
	// Establish node dependency to enable target node output caching and access
	chainCtx.AddNodeDependency(self.Id, x.Config.NodeId)
	return err
}

// OnMsg processes the message
// Retrieves target node's cached output via GetNodeRuleMsg, sends to failure chain if not found
func (x *FetchNodeOutputNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if targetMsg, exists := ctx.GetNodeRuleMsg(x.Config.NodeId); exists {
		ctx.TellSuccess(targetMsg)
	} else {
		// Target node has no output or dependency not established, send to failure chain
		ctx.TellFailure(msg, fmt.Errorf("node %s output not found", x.Config.NodeId))
	}
}

// Destroy 销毁节点
func (x *FetchNodeOutputNode) Destroy() {
}
