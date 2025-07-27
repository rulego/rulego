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

package flow

//节点复用节点，示例：
//{
//        "id": "s1",
//        "type": "ref",
//        "name": "节点复用",
//        "configuration": {
//			"targetId": "chain_01:node",
//        }
//  }
import (
	"errors"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
)

// init 注册RefNode组件
// init registers the RefNode component with the default registry.
func init() {
	Registry.Add(&RefNode{})
}

// RefNodeConfiguration RefNode配置结构
// RefNodeConfiguration defines the configuration structure for the RefNode component.
type RefNodeConfiguration struct {
	// TargetId 指定要引用的目标节点ID
	// TargetId specifies the target node ID to reference.
	// Format: [{chainId}]:{nodeId} for external chain nodes
	// Format: {nodeId} for current chain nodes
	TargetId string
}

// RefNode 引用并执行来自相同或不同规则链的节点的流控制组件
// RefNode is a flow control component that references and executes nodes from the same or different rule chains.
//
// 核心算法：
// Core Algorithm:
// 1. 解析目标ID以确定链和节点 - Parse target ID to determine chain and node
// 2. 使用当前消息执行引用的节点 - Execute referenced node with current message
// 3. 使用原始输出关系转发结果 - Forward result with original output relation
//
// 目标ID格式 - Target ID formats:
//
// 本地节点引用 - Local node reference:
//   - 格式：{nodeId} - Format: {nodeId}
//   - 示例："validatorNode" - Example: "validatorNode"
//   - 引用同一规则链内的节点 - References a node within the same rule chain
//
// 外部链节点引用 - External chain node reference:
//   - 格式：{chainId}:{nodeId} - Format: {chainId}:{nodeId}
//   - 示例："validation_chain:emailValidator" - Example: "validation_chain:emailValidator"
//   - 引用来自不同规则链的节点 - References a node from a different rule chain
//
// 配置示例 - Configuration examples:
//
// 本地节点引用 - Local node reference:
//
//	{
//	  "targetId": "dataValidator"
//	}
//
// 外部链节点引用 - External chain node reference:
//
//	{
//	  "targetId": "common_validators:emailCheck"
//	}
//
// 使用场景 - Use cases:
//   - 跨多个链的共享验证逻辑 - Shared validation logic across multiple chains
//   - 通用工具节点重用 - Common utility node reuse
//   - 模块化规则链架构 - Modular rule chain architecture
type RefNode struct {
	// Config 节点配置，包括目标节点规范
	// Config holds the node configuration including target node specification
	Config RefNodeConfiguration

	// chainId 存储外部引用的已解析链ID（本地为空）
	// chainId stores the parsed chain ID for external references (empty for local)
	chainId string

	// nodeId 存储要引用的已解析节点ID
	// nodeId stores the parsed node ID to reference
	nodeId string
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *RefNode) Type() string {
	return "ref"
}

// New 创建新实例
// New creates a new instance.
func (x *RefNode) New() types.Node {
	return &RefNode{}
}

// Init 初始化组件，解析目标ID以提取链和节点标识符
// Init initializes the component.
func (x *RefNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	if x.Config.TargetId == "" {
		return errors.New("targetId is empty")
	}

	values := strings.Split(x.Config.TargetId, ":")
	if len(values) == 1 {
		x.nodeId = strings.TrimSpace(values[0])
		if x.nodeId == "" {
			return errors.New("nodeId is empty")
		}
	} else if len(values) == 2 {
		x.chainId = strings.TrimSpace(values[0])
		x.nodeId = strings.TrimSpace(values[1])
		if x.chainId == "" || x.nodeId == "" {
			return errors.New("chainId or nodeId is empty")
		}
	} else {
		return errors.New("invalid targetId format, expected 'nodeId' or 'chainId:nodeId'")
	}
	return nil
}

// OnMsg 处理消息，通过执行引用的节点来处理传入消息
// OnMsg processes incoming messages by executing the referenced node.
func (x *RefNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellChainNode(ctx.GetContext(), x.chainId, x.nodeId, msg, true, func(newCtx types.RuleContext, newMsg types.RuleMsg, err error, relationType string) {
		if err != nil {
			ctx.TellFailure(msg, err)
		} else {
			ctx.TellNext(newMsg, relationType)
		}
	}, nil)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *RefNode) Destroy() {
}
