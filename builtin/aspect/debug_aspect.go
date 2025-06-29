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

package aspect

import (
	"github.com/rulego/rulego/api/types"
)

var (
	// Compile-time check Debug implements types.BeforeAspect.
	_ types.BeforeAspect = (*Debug)(nil)
	// Compile-time check Debug implements types.AfterAspect.
	_ types.AfterAspect = (*Debug)(nil)
)

// Debug is a debug logging aspect that provides comprehensive debug information
// for rule node execution. It logs both input and output messages along with
// execution context, making it essential for debugging rule chains.
//
// Debug 是一个调试日志切面，为规则节点执行提供全面的调试信息。
// 它记录输入和输出消息以及执行上下文，这对于调试规则链至关重要。
//
// Features:
// 功能特性：
//   - Logs message flow into nodes (In flow)  记录消息流入节点（In 流）
//   - Logs message flow out of nodes (Out flow)  记录消息流出节点（Out 流）
//   - Captures rule chain and node IDs  捕获规则链和节点 ID
//   - Records relation types and error information  记录关系类型和错误信息
//   - Asynchronous logging for minimal performance impact  异步日志记录，最小化性能影响
//
// Usage:
// 使用方法：
//
//	// Apply to all nodes in rule engine
//	// 应用到规则引擎的所有节点
//	config := types.NewConfig().WithAspects(&Debug{})
//	engine := rulego.NewRuleEngine(config)
//
// Debug logs are generated through the OnDebug callback configured in the rule context.
// 调试日志通过规则上下文中配置的 OnDebug 回调生成。
type Debug struct {
}

// Order returns the execution order of this aspect. Higher values execute later.
// Debug aspect executes with order 900, making it one of the last aspects to run.
//
// Order 返回此切面的执行顺序。值越高，执行越晚。
// Debug 切面的执行顺序为 900，使其成为最后执行的切面之一。
func (aspect *Debug) Order() int {
	return 900
}

// New creates a new instance of the Debug aspect.
// Each rule chain gets its own Debug aspect instance.
//
// New 创建 Debug 切面的新实例。
// 每个规则链都会获得自己的 Debug 切面实例。
func (aspect *Debug) New() types.Aspect {
	return &Debug{}
}

// Type returns the unique identifier for this aspect type.
//
// Type 返回此切面类型的唯一标识符。
func (aspect *Debug) Type() string {
	return "debug"
}

// PointCut determines which nodes this aspect applies to.
// The Debug aspect applies to all nodes unconditionally.
//
// PointCut 确定此切面应用于哪些节点。
// Debug 切面无条件地应用于所有节点。
func (aspect *Debug) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Before is executed before node processing. It logs the incoming message
// and context information asynchronously to avoid blocking execution.
//
// Before 在节点处理之前执行。它异步记录传入消息和上下文信息，避免阻塞执行。
func (aspect *Debug) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	//异步记录In日志
	aspect.onDebug(ctx, types.In, msg, relationType, nil)
	return msg
}

// After is executed after node processing. It logs the outgoing message
// and any error that occurred during processing.
//
// After 在节点处理之后执行。它记录传出消息和处理过程中发生的任何错误。
func (aspect *Debug) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	//异步记录Out日志
	aspect.onDebug(ctx, types.Out, msg, relationType, err)
	return msg
}

// onDebug handles the actual debug logging by calling the OnDebug callback
// configured in the rule context. It captures comprehensive information including
// chain ID, flow direction, node ID, message content, relation type, and errors.
//
// onDebug 通过调用规则上下文中配置的 OnDebug 回调来处理实际的调试日志记录。
// 它捕获全面的信息，包括链 ID、流向、节点 ID、消息内容、关系类型和错误。
func (aspect *Debug) onDebug(ctx types.RuleContext, flowType string, msg types.RuleMsg, relationType string, err error) {
	var chainId = ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	ctx.OnDebug(chainId, flowType, ctx.Self().GetNodeId().Id, msg, relationType, err)
}
