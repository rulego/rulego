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

//Example of rule chain node configuration:
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

// Register the node
func init() {
	Registry.Add(&FetchNodeOutputNode{})
}

// NodeOutputNodeConfiguration
type NodeOutputNodeConfiguration struct {
	// NodeId is the target node ID whose output message will be retrieved.
	NodeId string `json:"nodeId" label:"Node ID" desc:"Target node ID whose output will be retrieved" required:"true"`
}

// FetchNodeOutputNode Retrieves the component output by the specified node
// FetchNodeOutputNode retrieves the output of a specified node and passes it to the next node.
//
// Core Features:
// Core functionality:
// 1. Retrieve the target node's output message by nodeId - Retrieve the target node's output message by nodeId
// 2. Pass the retrieved message to the next node - Pass the retrieved message to the next node
// 3. Automatically establish node dependency to enable output caching - Automatically establish node dependency to enable output caching
//
// Dependency Mechanism:
// Dependency mechanism:
// - Automatically calls chainCtx.AddNodeDependency() to establish dependencies at the Init() phase
// - Only nodes that establish dependencies cache output data
// - Ensure the output of the target node can be accessed by GetNodeRuleMsg().
// - Automatically calls chainCtx.AddNodeDependency() during Init() to establish dependency
// - Only nodes with established dependencies will cache output data
// - Ensures target node output can be accessed via GetNodeRuleMsg()
//
// Usage scenarios:
// Use cases:
// - Cross-node data passing
// - Node output reuse
// - Conditional branch merging
type FetchNodeOutputNode struct {
	// Node configuration
	Config NodeOutputNodeConfiguration
}

// Type returns the component type
func (x *FetchNodeOutputNode) Type() string {
	return "fetchNodeOutput"
}

// New creates an instance
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

// Destroy the node
func (x *FetchNodeOutputNode) Destroy() {
}

// Desc returns the component description
func (x *FetchNodeOutputNode) Desc() string {
	return "Retrieve cached output of a specified node by nodeId. Auto-establishes node dependency for output caching. Routes to Success/Failure"
}
