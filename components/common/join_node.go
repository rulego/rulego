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

// Example of rule chain node configuration:
//{
//	"id": "s1",
//	"type": "join",
//	"name": "join",
//	"configuration": {
//	 }
//	}
//}
import (
	"context"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init registers the JoinNode component
// init registers the JoinNode component with the default registry.
func init() {
	Registry.Add(&JoinNode{})
}

// JoinNodeConfiguration JoinNode configuration structure
type JoinNodeConfiguration struct {
	// Timeout is the execution timeout in seconds. 0=no limit.
	Timeout int `json:"timeout" label:"Timeout" desc:"Timeout waiting for all branches in seconds, 0=no limit"`
	// MergeToMap merges all branch outputs into a {branchName: result} map if true.
	MergeToMap bool `json:"mergeToMap" label:"Merge to Map" desc:"true=merge all branch outputs into {branchName: result} map, false=use last message"`
}

// JoinNode is an action component that merges multiple asynchronous nodes to execute results
// JoinNode is an action component that merges results from multiple asynchronous node executions.
//
// Core algorithm:
// Core Algorithm:
// 1. Wait for all parallel branches to complete execution - Wait for all parallel branches to complete
// 2. Collect messages from all branches
// 3. Merge metadata from all branches
// 4. Merge collected results into a JSON array - Combine collected results into a JSON array
// 5. Send merged results via Success relation
//
// Workflow pattern - Workflow pattern:
//   - Fork -> [BranchA, BranchB, BranchC] -> Join -> Continue - Fork -> [Parallel Processing] -> Join -> Continue
//
// Timeout handling:
//   - Configurable timeout prevents indefinite waiting
//   - Route via Failure relation on timeout - Route via Failure relation on timeout
type JoinNode struct {
	// Config defines the node configuration
	// Config holds the node configuration including timeout settings
	Config JoinNodeConfiguration
}

// Type returns the component type
// Type returns the component type identifier.
func (x *JoinNode) Type() string {
	return "join"
}

// New creates an instance
// New creates a new instance.
func (x *JoinNode) New() types.Node {
	return &JoinNode{Config: JoinNodeConfiguration{}}
}

// Init initializes the component
// Init initializes the component.
func (x *JoinNode) Init(_ types.Config, configuration types.Configuration) error {
	return maps.Map2Struct(configuration, &x.Config)
}

// OnMsg processes messages, collects and merges results from parallel branches
// OnMsg processes incoming messages by collecting results from parallel branches and merging them.
func (x *JoinNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	c := make(chan struct{}, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}
	defer cancel()

	var wrapperMsg = msg.Copy()

	var err error
	ok := ctx.TellCollect(msg, func(msgList []types.WrapperMsg) {
		// Check if the context has been canceled
		select {
		case <-chanCtx.Done():
			return
		default:
		}

		wrapperMsg.SetDataType(types.JSON)
		err = mergeMetadata(msgList, &wrapperMsg)
		if x.Config.MergeToMap {
			mergedMap := make(map[string]interface{})
			for _, val := range msgList {
				if val.NodeId != "" {
					switch val.Msg.DataType {
					case types.JSON:
						if dataMap, err := val.Msg.GetJsonData(); err == nil {
							if m, ok := dataMap.(map[string]interface{}); ok {
								for k, v := range m {
									mergedMap[k] = v
								}
							} else {
								mergedMap[val.NodeId] = dataMap
							}
						} else {
							mergedMap[val.NodeId] = val.Msg.GetData()
						}
					default:
						mergedMap[val.NodeId] = val.Msg.GetData()
					}
				}
			}
			wrapperMsg.SetData(str.ToString(mergedMap))
		} else {
			wrapperMsg.SetData(str.ToString(filterEmptyAndRemoveMeta(msgList)))
		}
		select {
		case c <- struct{}{}:
		default: // Prevents blockages
		}
	})
	if ok {
		select {
		case <-chanCtx.Done():
			ctx.TellFailure(wrapperMsg, chanCtx.Err())
		case <-c:
			if err != nil {
				ctx.TellFailure(wrapperMsg, err)
			} else {
				ctx.TellSuccess(wrapperMsg)
			}
		}
	}
}

// Desc returns the component description
func (x *JoinNode) Desc() string {
	return "Wait for all fork branches to complete and merge results. mergeToMap=true creates {branchName: result} map. Routes to Success/Failure"
}

// Destroy to clean up resources
func (x *JoinNode) Destroy() {
}
