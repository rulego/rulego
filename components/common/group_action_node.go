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
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init registers the GroupActionNode component
// init registers the GroupActionNode component with the default registry.
func init() {
	Registry.Add(&GroupActionNode{})
}

// GroupActionNodeConfiguration GroupActionNode configuration structure
// GroupActionNodeConfiguration defines the configuration structure for the GroupActionNode component.
type GroupActionNodeConfiguration struct {
	// MatchRelationType is the relation type to match within the group.
	MatchRelationType string `json:"matchRelationType" label:"Match Relation" desc:"Relation type to match: Success, Failure, True, False, or custom. Default: Success"`
	// MatchNum is the number of nodes that must match. 0=all must match.
	MatchNum int `json:"matchNum" label:"Match Count" desc:"Nodes that must match relation type. 0=all must match for Success"`
	// NodeIds is the list of node IDs in the group.
	NodeIds interface{} `json:"nodeIds" label:"Node IDs" desc:"Comma-separated node IDs or string array" required:"true"`
	// Timeout is the execution timeout in seconds. 0=no limit.
	Timeout int `json:"timeout" label:"Timeout" desc:"Execution timeout in seconds, 0=no limit"`
	// MergeToMap merges all node outputs into a single JSON map if true.
	MergeToMap bool `json:"mergeToMap" label:"Merge to Map" desc:"true=merge all outputs into {nodeId: result} map"`
}

// GroupActionNode is an action component that groups multiple nodes and executes them asynchronously
// GroupActionNode is an action component that groups multiple nodes and executes them asynchronously.
//
// Core algorithm:
// Core Algorithm:
// 1. Execute all nodes in the group concurrently
// 2. Collect results and count matching relation types
// 3. Determine success based on MatchNum criteria
// 4. Merge results and route to Success or Failure chains - Merge results and route to Success or Failure
//
// Matching logic:
//   - MatchNum=0: All nodes must match
//   - MatchNum>0: At least MatchNum nodes must match
//
// Timeout protection:
//   - Configured timeout prevents indefinite waiting
//   - Early termination when match criteria are satisfied
type GroupActionNode struct {
	// Config defines the node configuration
	// Config holds the node configuration including matching criteria and timeout
	Config GroupActionNodeConfiguration

	// NodeIdList is the list of node IDs to be executed
	// NodeIdList contains the parsed list of node IDs to execute
	NodeIdList []string

	// Number of nodes in the Length group
	// Length stores the number of nodes in the group for efficient access
	Length int32
}

// Type returns the component type
// Type returns the component type identifier.
func (x *GroupActionNode) Type() string {
	return "groupAction"
}

// New creates an instance
// New creates a new instance.
func (x *GroupActionNode) New() types.Node {
	return &GroupActionNode{Config: GroupActionNodeConfiguration{MatchRelationType: types.Success, MatchNum: 0}}
}

// Init initializes the component
// Init initializes the component.
func (x *GroupActionNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	var nodeIds []string
	if v, ok := x.Config.NodeIds.(string); ok {
		nodeIds = strings.Split(v, ",")
	} else if v, ok := x.Config.NodeIds.([]string); ok {
		nodeIds = v
	} else if v, ok := x.Config.NodeIds.([]interface{}); ok {
		for _, item := range v {
			nodeIds = append(nodeIds, str.ToString(item))
		}
	}
	for _, nodeId := range nodeIds {
		if v := strings.TrimSpace(nodeId); v != "" {
			x.NodeIdList = append(x.NodeIdList, v)
		}
	}
	x.Config.MatchRelationType = strings.TrimSpace(x.Config.MatchRelationType)

	if x.Config.MatchRelationType == "" {
		x.Config.MatchRelationType = types.Success
	}
	if x.Config.MatchNum <= 0 || x.Config.MatchNum > len(x.NodeIdList) {
		x.Config.MatchNum = len(x.NodeIdList)
	}
	x.Length = int32(len(x.NodeIdList))
	return err
}

// OnMsg processes messages, executes node groups concurrently, and determines success based on matching conditions
// OnMsg processes incoming messages by executing the configured group of nodes in parallel.
func (x *GroupActionNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Length == 0 {
		ctx.TellFailure(msg, errors.New("nodeIds is empty"))
		return
	}
	//The number of completed execution nodes
	var endCount int32
	//Match the number of nodes
	var currentMatchedCount int32
	//Whether it has been completed
	var completed int32
	c := make(chan bool, 1)
	var chanCtx context.Context
	var cancel context.CancelFunc
	if x.Config.Timeout > 0 {
		chanCtx, cancel = context.WithTimeout(ctx.GetContext(), time.Duration(x.Config.Timeout)*time.Second)
	} else {
		chanCtx, cancel = context.WithCancel(ctx.GetContext())
	}

	defer cancel()

	var wrapperMsg = msg.Copy()
	//Each node executes the result list
	var msgs = make([]types.WrapperMsg, len(x.NodeIdList))
	//Protect the mutex lock of the msgs array
	var msgsMutex sync.Mutex

	//Execute node list logic
	for i, nodeId := range x.NodeIdList {
		index := i
		ctx.TellNode(chanCtx, nodeId, msg.Copy(), true, func(callbackCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			// Check if the context has been canceled to avoid meaningless calculations
			select {
			case <-chanCtx.Done():
				return // Exit early to avoid wasting resources
			default:
			}

			// Safely write to the msgs array
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			selfId := callbackCtx.GetSelfId()

			msgsMutex.Lock()
			msgs[index] = types.WrapperMsg{
				Msg:    onEndMsg,
				Err:    errStr,
				NodeId: selfId,
			}
			msgsMutex.Unlock()

			// Directly use atomic operations to obtain the current count and avoid race window entries
			currentEndCount := atomic.AddInt32(&endCount, 1)
			var currentMatchCount int32
			if x.Config.MatchRelationType == relationType {
				currentMatchCount = atomic.AddInt32(&currentMatchedCount, 1)
			} else {
				currentMatchCount = atomic.LoadInt32(&currentMatchedCount)
			}

			// Decide whether to end and send the results
			var shouldComplete bool
			var result bool

			// If the match quantity is reached, it immediately returns to success
			if currentMatchCount >= int32(x.Config.MatchNum) {
				shouldComplete = true
				result = true
			} else if currentEndCount >= x.Length {
				// All nodes complete but fail to match the number of nodes, resulting in failure
				shouldComplete = true
				result = false
			}

			// Use CAS to ensure that only one goroutine can send results
			if shouldComplete && atomic.CompareAndSwapInt32(&completed, 0, 1) {
				// Safely read the msgs array for processing
				msgsMutex.Lock()
				msgsCopy := make([]types.WrapperMsg, len(msgs))
				copy(msgsCopy, msgs)
				msgsMutex.Unlock()

				if x.Config.MergeToMap {
					wrapperMsg.SetDataType(types.JSON)
					mergedMap := make(map[string]interface{})
					for _, val := range msgsCopy {
						if val.NodeId != "" {
							// Different processing is performed depending on the data type
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
					wrapperMsg.SetData(str.ToString(filterEmptyAndRemoveMeta(msgsCopy)))
				}
				_ = mergeMetadata(msgsCopy, &wrapperMsg)

				// Uses non-blocking sending to prevent channel blocking during timeouts
				select {
				case c <- result:
					// Sent successfully
				default:
					// The channel is full or has no recipients (possibly the main function has timed out), so the transmission is abandoned
				}
			}
		}, nil)
	}

	// Waiting for execution to finish or timeout
	select {
	case <-chanCtx.Done():
		ctx.TellFailure(wrapperMsg, chanCtx.Err())
	case r := <-c:
		if r {
			ctx.TellSuccess(wrapperMsg)
		} else {
			ctx.TellNext(wrapperMsg, types.Failure)
		}
	}
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *GroupActionNode) Destroy() {
	// No resources to clean
	// No resources to clean up
}

// filterEmptyAndRemoveMeta filters out empty messages and clears metadata
// filterEmptyAndRemoveMeta filters out empty messages and removes metadata for cleaner output.
func filterEmptyAndRemoveMeta(msgs []types.WrapperMsg) []types.WrapperMsg {
	var result []types.WrapperMsg
	for _, msg := range msgs {
		if msg.NodeId != "" {
			if msg.Msg.Metadata != nil {
				msg.Msg.Metadata.Clear()
			}
			result = append(result, msg)
		}
	}
	return result
}

// mergeMetadata merges successfully executed metadata into wrapper messages
// mergeMetadata merges metadata from successful group executions into the wrapper message.
func mergeMetadata(msgs []types.WrapperMsg, wrapperMsg *types.RuleMsg) error {
	var errStr string
	for _, msg := range msgs {
		if msg.NodeId != "" && msg.Err == "" {
			msg.Msg.Metadata.ForEach(func(k, v string) bool {
				wrapperMsg.Metadata.PutValue(k, v)
				return true // continue iteration
			})
		} else if msg.Err != "" {
			errStr += fmt.Sprintf("NodeId=%s,Err=%s ", msg.NodeId, msg.Err)
		}
	}
	if errStr != "" {
		return errors.New(errStr)
	} else {
		return nil
	}
}

// Desc returns the component description
func (x *GroupActionNode) Desc() string {
	return "Group multiple nodes and execute asynchronously. Route based on matchRelationType and matchNum conditions. Supports timeout and output merging. Routes to Success/Failure"
}
