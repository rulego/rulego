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
// Debug is a debug log section that provides comprehensive debugging information for rule node execution.
// It records input and output messages as well as execution context, which is crucial for debugging the rule chain.
//
// Features:
// Features:
//   - Logs message flow into nodes (In flow)
//   - Logs message flow out of nodes (Out flow)
//   - Captures rule chain and node IDs
//   - Records relation types and error information
//   - Asynchronous logging for minimal performance impact
//
// Usage:
// How to use:
//
//	// Apply to all nodes in rule engine
//	Applied to all nodes of the rule engine
//	config := types.NewConfig().WithAspects(&Debug{})
//	engine := rulego.NewRuleEngine(config)
//
// Debug logs are generated through the OnDebug callback configured in the rule context.
// Debug logs are generated through OnDebug callbacks configured in the rule context.
type Debug struct {
}

// Order returns the execution order of this aspect. Higher values execute later.
// Debug aspect executes with order 900, making it one of the last aspects to run.
//
// Order returns the execution order of this aspect. The higher the value, the later it is executed.
// The execution order of the Debug facet is 900, making it one of the last faces to be executed.
func (aspect *Debug) Order() int {
	return 900
}

// New creates a new instance of the Debug aspect.
// Each rule chain gets its own Debug aspect instance.
//
// New: Create a new instance of the Debug face.
// Each rule chain receives its own Debug Facet instance.
func (aspect *Debug) New() types.Aspect {
	return &Debug{}
}

// Type returns the unique identifier for this aspect type.
//
// Type returns a unique identifier for this facet type.
func (aspect *Debug) Type() string {
	return "debug"
}

// PointCut determines which nodes this aspect applies to.
// The Debug aspect applies to all nodes unconditionally.
//
// PointCut determines which nodes this section is applied to.
// The Debug Face is applied unconditionally to all nodes.
func (aspect *Debug) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// Before is executed before node processing. It logs the incoming message
// and context information asynchronously to avoid blocking execution.
//
// Before executing before the node processes it. It records incoming messages and contextual information asynchronously to avoid blocking execution.
func (aspect *Debug) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	//Asynchronously log in
	aspect.onDebug(ctx, types.In, msg, relationType, nil)
	return msg
}

// After is executed after node processing. It logs the outgoing message
// and any error that occurred during processing.
//
// After is executed after the node processes it. It records any errors that occur during outgoing messages and processing.
func (aspect *Debug) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	//Asynchronously records the Out log
	aspect.onDebug(ctx, types.Out, msg, relationType, err)
	return msg
}

// onDebug handles the actual debug logging by calling the OnDebug callback
// configured in the rule context. It captures comprehensive information including
// chain ID, flow direction, node ID, message content, relation type, and errors.
//
// onDebug handles the actual debug log by calling the OnDebug callback configured in the rule context.
// It captures comprehensive information, including chain ID, flow, node ID, message content, relationship types, and errors.
func (aspect *Debug) onDebug(ctx types.RuleContext, flowType string, msg types.RuleMsg, relationType string, err error) {
	var chainId = ""
	if ctx.RuleChain() != nil {
		chainId = ctx.RuleChain().GetNodeId().Id
	}
	ctx.OnDebug(chainId, flowType, ctx.Self().GetNodeId().Id, msg, relationType, err)
}
