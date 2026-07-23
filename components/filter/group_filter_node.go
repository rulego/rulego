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

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init registers the GroupFilterNode component
// init registers the GroupFilterNode component with the default registry.
func init() {
	Registry.Add(&GroupFilterNode{})
}

// GroupFilterNodeConfiguration GroupFilterNode configuration structure
// GroupFilterNodeConfiguration defines the configuration structure for the GroupFilterNode component.
type GroupFilterNodeConfiguration struct {
	// AllMatches determines the group's evaluation logic
	// AllMatches determines the group evaluation logic:
	//   - true: All nodes must return True for message to route to "True" chain
	//   - false: Any node returning True will route message to "True" chain
	AllMatches bool `json:"allMatches" label:"All Matches" desc:"true=AND logic (all must pass), false=OR logic (any passes)"`

	// NodeIds specifies the list of filter nodes to include in the group.
	// Can be provided as comma-separated string, []string, or []interface{}.
	NodeIds interface{} `json:"nodeIds" label:"Node IDs" desc:"Comma-separated filter node IDs or string array" required:"true"`

	// Timeout specifies the execution timeout in seconds. Default 0 means no timeout.
	Timeout int `json:"timeout" label:"Timeout" desc:"Execution timeout in seconds, 0=no limit"`
}

// GroupFilterNode is a filter component that groups multiple filter nodes and collectively evaluates them
// GroupFilterNode groups multiple filter nodes and evaluates them collectively.
//
// Core algorithm:
// Core Algorithm:
// 1. Execute all configured filter nodes concurrently
// 2. Aggregating True/False results using atomic operations - Aggregating True/False results using atomic operations
// 3. Apply AND/OR logic based on AllMatches configuration
// 4. Implement early termination optimization to reduce unnecessary computation
//
// Evaluation logic:
//   - AllMatches=true (AND logic): All nodes must return True - All nodes must return True
//   - AllMatches=false (OR logic): Any node returning True is success
//
// Timeout handling:
//   - Configurable timeout prevents indefinite waiting
//   - Route to Failure relation on timeout - Route to Failure relation on timeout
type GroupFilterNode struct {
	// Config group filter configuration
	// Config holds the group filter configuration
	Config GroupFilterNodeConfiguration

	// NodeIdList is the list of node IDs to be executed
	// NodeIdList contains the parsed list of node IDs to execute
	NodeIdList []string

	// Length group: Total number of nodes
	// Length is the total number of nodes in the group
	Length int32
}

// Type returns the component type
// Type returns the component type identifier.
func (x *GroupFilterNode) Type() string {
	return "groupFilter"
}

// New creates an instance
// New creates a new instance.
func (x *GroupFilterNode) New() types.Node {
	return &GroupFilterNode{Config: GroupFilterNodeConfiguration{AllMatches: false}}
}

// Init initializes the component
// Init initializes the component.
func (x *GroupFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
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
		if v := strings.Trim(nodeId, ""); v != "" {
			x.NodeIdList = append(x.NodeIdList, v)
		}
	}
	x.Length = int32(len(x.NodeIdList))
	return err
}

// OnMsg processes messages, executes all configured filter nodes concurrently, and aggregates results based on the configured logic
// OnMsg processes incoming messages by executing all configured filter nodes concurrently.
func (x *GroupFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.Length == 0 {
		ctx.TellFailure(msg, errors.New("nodeIds is empty"))
		return
	}
	var endCount int32
	var trueCount int32 // New: Track the number of True results
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

	//Execute node list logic
	for _, nodeId := range x.NodeIdList {
		ctx.TellNode(chanCtx, nodeId, msg, true, func(callbackCtx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// Check if the context has been canceled to avoid meaningless calculations
			select {
			case <-chanCtx.Done():
				return // Exit early to avoid wasting resources
			default:
			}

			// Directly use atomic operations to obtain the current count and avoid race window entries
			currentEndCount := atomic.AddInt32(&endCount, 1)
			var currentTrueCount int32
			if relationType == types.True {
				currentTrueCount = atomic.AddInt32(&trueCount, 1)
			} else {
				currentTrueCount = atomic.LoadInt32(&trueCount)
			}

			// Decide whether to end and send the results
			var shouldComplete bool
			var result bool

			if x.Config.AllMatches {
				// AllMatches=true: Returns False immediately if any False is found; only returns True if all are True
				if relationType != types.True {
					shouldComplete = true
					result = false
				} else if currentEndCount >= x.Length && currentTrueCount >= x.Length {
					shouldComplete = true
					result = true
				}
			} else {
				// AllMatches=false: Returns True immediately if there are any Trues; returns False only when all are complete and none are True
				if relationType == types.True {
					shouldComplete = true
					result = true
				} else if currentEndCount >= x.Length && currentTrueCount == 0 {
					shouldComplete = true
					result = false
				}
			}

			// Use CAS to ensure that only one goroutine can send results
			if shouldComplete && atomic.CompareAndSwapInt32(&completed, 0, 1) {
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
		ctx.TellFailure(msg, chanCtx.Err())
	case r := <-c:
		if r {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Desc returns the component description
func (x *GroupFilterNode) Desc() string {
	return "Group multiple filter nodes and evaluate collectively. allMatches=true requires all to pass (AND), false requires any to pass (OR). Routes to True/False"
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *GroupFilterNode) Destroy() {
}
