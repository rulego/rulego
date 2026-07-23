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

//Sub-rule chain nodes, example:
//{
//        "id": "s1",
//        "type": "flow",
//        "name": "子规则链",
//        "configuration": {
//			"targetId": "sub_chain_01",
//        }
//  }
import (
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init registers the ChainNode component
// init registers the ChainNode component with the default registry.
func init() {
	Registry.Add(&ChainNode{})
}

// ChainNodeConfiguration ChainNode configuration structure
// ChainNodeConfiguration defines the configuration structure for the ChainNode component.
type ChainNodeConfiguration struct {
	// TargetId is the sub-rule chain ID to execute.
	TargetId string `json:"targetId" label:"Target Chain ID" desc:"Sub-rule chain ID to execute" required:"true"`
	// Extend: true=inherit sub-chain relations without merging, false=merge all outputs.
	Extend bool `json:"extend" label:"Extend" desc:"true=forward each sub-chain output directly, false=merge all outputs into single result"`
}

// ChainNode executes the flow control component of the sub-rule chain
// ChainNode is a flow control component that executes sub-rule chains.
//
// Core algorithm:
// Core Algorithm:
// 1. Find and execute the sub-rule chain by TargetId - Find and execute the sub-rule chain by TargetId
// 2. Choose output handling mode based on Extend configuration
// 3. Forward results to downstream nodes or merge all outputs - Forward results to downstream nodes or merge all outputs
//
// Execution modes:
//
// Extend mode (Extend=true) - Extend mode:
//   - Each output from the sub-chain forwarded directly to downstream nodes
//   - Preserves original message flow structure
//   - No merging of sub-chain relations and outputs
//
// Merge mode (Extend=false) - Merge mode:
//   - Wait for all sub-chain branches to complete
//   - Merge all results into a single output - Merge all results into single output
//   - Output format: []WrapperMsg contains all results - Output format: []WrapperMsg contains all results
//   - Merge metadata from all successful branches - Merge metadata from all successful branches
//
// Configuration example:
//
//	{
//	  "targetId": "validation_chain",
//	  "extend": false
//	}
//
// Use cases:
//   - Modular rule chain composition
//   - Sub-workflow execution
//   - Complex business logic decomposition
type ChainNode struct {
	// Config node configuration, including the target chain ID and execution mode
	// Config holds the node configuration including target chain ID and execution mode
	Config ChainNodeConfiguration
}

// Type returns the component type
// Type returns the component type identifier.
func (x *ChainNode) Type() string {
	return "flow"
}

// New creates an instance
// New creates a new instance.
func (x *ChainNode) New() types.Node {
	return &ChainNode{}
}

// Init initializes the component
// Init initializes the component.
func (x *ChainNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

// OnMsg processes messages by executing configured subrule chains to handle incoming messages
// OnMsg processes incoming messages by executing the configured sub-rule chain.
func (x *ChainNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Config.Extend {
		x.TellFlowAndNoMerge(ctx, msg)
	} else {
		x.TellFlowAndMerge(ctx, msg)
	}
}

// TellFlowAndNoMerge executes the sub-rule chain without merging results, forwarding each output separately
// TellFlowAndNoMerge executes the sub-rule chain without merging results.
func (x *ChainNode) TellFlowAndNoMerge(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellFlow(x.Config.TargetId, msg, types.WithContext(ctx.GetContext()), types.WithOnEnd(func(nodeCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
		if err != nil {
			ctx.TellFailure(onEndMsg, err)
		} else {
			ctx.TellNext(onEndMsg, relationType)
		}

	}))
}

// TellFlowAndMerge executes the sub-rule chain and consolidates all results into a single output
// TellFlowAndMerge executes the sub-rule chain and merges all results into a single output.
func (x *ChainNode) TellFlowAndMerge(ctx types.RuleContext, msg types.RuleMsg) {
	var wrapperMsg = msg.Copy()
	var msgs []types.WrapperMsg
	var targetRelationType = types.Success
	var targetErr error
	//A mutex lock is used to protect concurrent writes and metadata merging on MSGS slices
	var mu sync.Mutex
	ctx.TellFlow(x.Config.TargetId, msg, types.WithContext(ctx.GetContext()), types.WithOnEnd(func(nodeCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
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
		//Delete the metadata
		if onEndMsg.Metadata != nil {
			onEndMsg.Metadata.Clear()
		}
		msgs = append(msgs, types.WrapperMsg{
			Msg:    onEndMsg,
			Err:    errStr,
			NodeId: selfId,
		})

	}), types.WithOnAllNodeCompleted(func() {
		wrapperMsg.DataType = types.JSON
		wrapperMsg.SetData(str.ToString(msgs))
		if targetRelationType == types.Failure {
			ctx.TellFailure(wrapperMsg, targetErr)
		} else {
			ctx.TellSuccess(wrapperMsg)
		}
	}))
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *ChainNode) Destroy() {
}

// Desc returns the component description
func (x *ChainNode) Desc() string {
	return "Execute a sub-rule chain by targetId. extend=true forwards each output directly, false merges all outputs. Routes to Success/Failure"
}
