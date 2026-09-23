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

//子规则链节点，示例：
//{
//        "id": "s1",
//        "type": "flow",
//        "name": "子规则链",
//        "configuration": {
//			"targetId": "sub_chain_01",
//        }
//  }
//targetId 支持 {chainId}:{nodeId} 格式，从子链指定节点开始执行，例如 "sub_chain_01:node_2"
import (
	"errors"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init 注册ChainNode组件
// init registers the ChainNode component with the default registry.
func init() {
	Registry.Add(&ChainNode{})
}

// ChainNodeConfiguration ChainNode配置结构
// ChainNodeConfiguration defines the configuration structure for the ChainNode component.
type ChainNodeConfiguration struct {
	// TargetId is the sub-rule chain ID to execute.
	// Format: {chainId} executes the whole chain, {chainId}:{nodeId} starts from the given node.
	TargetId string `json:"targetId" label:"Target Chain ID" desc:"Sub-rule chain ID to execute. Format: {chainId} or {chainId}:{nodeId} to start from a specific node" required:"true" component:"{\"type\":\"RuleChainSelector\",\"nodePicker\":true}"`
	// Extend: true=inherit sub-chain relations without merging, false=merge all outputs.
	Extend bool `json:"extend" label:"Extend" desc:"true=forward each sub-chain output directly, false=merge all outputs into single result"`
}

// ChainNode 执行子规则链的流控制组件
// ChainNode is a flow control component that executes sub-rule chains.
//
// 核心算法：
// Core Algorithm:
// 1. 根据TargetId查找并执行子规则链 - Find and execute sub-rule chain by TargetId
// 2. 根据Extend配置选择输出处理模式 - Choose output handling mode based on Extend configuration
// 3. 转发结果到下游节点或合并所有输出 - Forward results to downstream nodes or merge all outputs
//
// 执行模式 - Execution modes:
//
// 扩展模式（Extend=true）- Extend mode:
//   - 子链的每个输出直接转发到下游节点 - Each output from sub-chain forwarded directly to downstream nodes
//   - 保留原始消息流结构 - Preserves original message flow structure
//   - 不合并子链的关系和输出 - No merging of sub-chain relations and outputs
//
// 合并模式（Extend=false）- Merge mode:
//   - 等待所有子链分支执行完成 - Wait for all sub-chain branches to complete
//   - 将所有结果合并为单个输出 - Merge all results into single output
//   - 输出格式：[]WrapperMsg包含所有结果 - Output format: []WrapperMsg containing all results
//   - 从所有成功分支合并元数据 - Merge metadata from all successful branches
//
// 配置示例 - Configuration example:
//
//	{
//	  "targetId": "validation_chain",
//	  "extend": false
//	}
//
// 使用场景 - Use cases:
//   - 模块化规则链组合 - Modular rule chain composition
//   - 子工作流执行 - Sub-workflow execution
//   - 复杂业务逻辑分解 - Complex business logic decomposition
type ChainNode struct {
	// Config 节点配置，包括目标链ID和执行模式
	// Config holds the node configuration including target chain ID and execution mode
	Config ChainNodeConfiguration

	// chainId 目标链 ID（targetId 去掉起点部分）
	chainId string
	// startNodeId 起始节点 ID，targetId 不带起点时为空
	startNodeId string
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *ChainNode) Type() string {
	return "flow"
}

// New 创建新实例
// New creates a new instance.
func (x *ChainNode) New() types.Node {
	return &ChainNode{}
}

// Init 初始化组件
// Init initializes the component.
func (x *ChainNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.chainId = ""
	x.startNodeId = ""
	targetId := strings.TrimSpace(x.Config.TargetId)
	//空 targetId 允许初始化（画布先放置后配置），执行时按链不存在走 Failure
	if targetId == "" {
		return nil
	}
	values := strings.Split(targetId, ":")
	switch len(values) {
	case 1:
		x.chainId = values[0]
	case 2:
		x.chainId = strings.TrimSpace(values[0])
		x.startNodeId = strings.TrimSpace(values[1])
		if x.chainId == "" || x.startNodeId == "" {
			return errors.New("invalid targetId format, expected 'chainId' or 'chainId:nodeId'")
		}
	default:
		return errors.New("invalid targetId format, expected 'chainId' or 'chainId:nodeId'")
	}
	return nil
}

// OnMsg 处理消息，通过执行配置的子规则链来处理传入消息
// OnMsg processes incoming messages by executing the configured sub-rule chain.
func (x *ChainNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	opts := make([]types.RuleContextOption, 0, 3)
	opts = append(opts, types.WithContext(ctx.GetContext()))
	if x.startNodeId != "" {
		opts = append(opts, types.WithStartNode(x.startNodeId))
	}
	if x.Config.Extend {
		x.TellFlowAndNoMerge(ctx, msg, opts...)
	} else {
		x.TellFlowAndMerge(ctx, msg, opts...)
	}
}

// TellFlowAndNoMerge 执行子规则链而不合并结果，每个输出单独转发
// TellFlowAndNoMerge executes the sub-rule chain without merging results.
func (x *ChainNode) TellFlowAndNoMerge(ctx types.RuleContext, msg types.RuleMsg, opts ...types.RuleContextOption) {
	opts = append(opts, types.WithOnEnd(func(nodeCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
		if err != nil {
			ctx.TellFailure(onEndMsg, err)
		} else {
			ctx.TellNext(onEndMsg, relationType)
		}

	}))
	ctx.TellFlow(x.chainId, msg, opts...)
}

// TellFlowAndMerge 执行子规则链并将所有结果合并为单个输出
// TellFlowAndMerge executes the sub-rule chain and merges all results into a single output.
func (x *ChainNode) TellFlowAndMerge(ctx types.RuleContext, msg types.RuleMsg, opts ...types.RuleContextOption) {
	var wrapperMsg = msg.Copy()
	var msgs []types.WrapperMsg
	var targetRelationType = types.Success
	var targetErr error
	//使用一个互斥锁来保护对msgs切片的并发写入和metadata合并
	var mu sync.Mutex
	opts = append(opts, types.WithOnEnd(func(nodeCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
		mu.Lock()
		defer mu.Unlock()
		errStr := ""
		if err == nil {
			// use zero-copy ForEach for better metadata merging performance
			onEndMsg.Metadata.ForEach(func(k, v string) bool {
				wrapperMsg.Metadata.PutValue(k, v)
				return true // continue iteration
			})
		} else {
			errStr = err.Error()
		}
		selfId := nodeCtx.GetSelfId()

		if relationType == types.Failure {
			targetRelationType = relationType
			targetErr = err
		}
		//删除掉元数据
		if onEndMsg.Metadata != nil {
			onEndMsg.Metadata.Clear()
		}
		msgs = append(msgs, types.WrapperMsg{
			Msg:    onEndMsg,
			Err:    errStr,
			NodeId: selfId,
		})

	}), types.WithOnAllNodeCompleted(func() {
		mu.Lock()
		wrapperMsg.DataType = types.JSON
		wrapperMsg.SetData(str.ToString(msgs))
		relationType := targetRelationType
		err := targetErr
		mu.Unlock()
		if relationType == types.Failure {
			ctx.TellFailure(wrapperMsg, err)
		} else {
			ctx.TellSuccess(wrapperMsg)
		}
	}))
	ctx.TellFlow(x.chainId, msg, opts...)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *ChainNode) Destroy() {
}

// Desc returns the component description
func (x *ChainNode) Desc() string {
	return "Execute a sub-rule chain by targetId. Format: {chainId} or {chainId}:{nodeId} to start from a specific node. extend=true forwards each output directly, false merges all outputs. Routes to Success/Failure"
}
