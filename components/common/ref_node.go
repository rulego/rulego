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

//Node reuse nodes, example:
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

// init registers the RefNode component
// init registers the RefNode component with the default registry.
func init() {
	Registry.Add(&RefNode{})
}

// RefNodeConfiguration RefNode configuration structure
// RefNodeConfiguration defines the configuration structure for the RefNode component.
type RefNodeConfiguration struct {
	// TargetId is the target node ID to reference.
	// Format: {nodeId} for local nodes, {chainId}:{nodeId} for external chain nodes.
	TargetId string `json:"targetId" label:"Target ID" desc:"Target node ID. Format: {nodeId} or {chainId}:{nodeId}" required:"true"`
	// TellChain: true executes the entire chain from TargetId, false executes only the target node.
	TellChain bool `json:"tellChain" label:"Tell Chain" desc:"true=execute entire chain from target, false=execute target node only"`
}

// RefNode refers to and executes the flow control component from nodes in the same or different rule chains
// RefNode is a flow control component that references and executes nodes from the same or different rule chains.
//
// Core algorithm:
// Core Algorithm:
// 1. Parse target ID to determine chain and node - Parse target ID to determine chain and node
// 2. Execute referenced node with current message
// 3. Forward result with original output relation - Forward result with original output relation
//
// Target ID formats:
//
// Local node reference:
//   - Format: {nodeId} - Format: {nodeId}
//   - Example: "validatorNode" - Example: "validatorNode"
//   - References a node within the same rule chain
//
// External chain node reference:
//   - Format: {chainId}:{nodeId} - Format: {chainId}:{nodeId}
//   - Example: "validation_chain:emailValidator" - Example: "validation_chain:emailValidator"
//   - References a node from a different rule chain
//
// Configuration examples:
//
// Local node reference:
//
//	{
//	  "targetId": "dataValidator"
//	}
//
// External chain node reference:
//
//	{
//	  "targetId": "common_validators:emailCheck"
//	}
//
// Use cases:
//   - Shared validation logic across multiple chains
//   - Common utility node reuse
//   - Modular rule chain architecture
type RefNode struct {
	// Config node configuration, including the target node specification
	// Config holds the node configuration including target node specification
	Config RefNodeConfiguration

	// chainId stores the externally referenced resolved chain ID (natively empty)
	// chainId stores the parsed chain ID for external references (empty for local)
	chainId string

	// nodeId stores the resolved node ID to be referenced
	// nodeId stores the parsed node ID to reference
	nodeId string
}

// Type returns the component type
// Type returns the component type identifier.
func (x *RefNode) Type() string {
	return "ref"
}

// New creates an instance
// New creates a new instance.
func (x *RefNode) New() types.Node {
	return &RefNode{}
}

// Init initializes components and parses target IDs to extract chain and node identifiers
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

// OnMsg processes messages by executing the referenced nodes to handle incoming messages
// OnMsg processes incoming messages by executing the referenced node.
func (x *RefNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellChainNode(ctx.GetContext(), x.chainId, x.nodeId, msg, !x.Config.TellChain, func(newCtx types.RuleContext, newMsg types.RuleMsg, err error, relationType string) {
		if err != nil {
			ctx.TellFailure(msg, err)
		} else {
			ctx.TellNext(newMsg, relationType)
		}
	}, nil)
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *RefNode) Destroy() {
}

// Def returns the component form definition
func (x *RefNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Reference and execute a node from same or different rule chain. Format: {nodeId} or {chainId}:{nodeId}. tellChain=true executes entire sub-chain",
		RelationTypes: &[]string{types.Success, types.Failure, types.True, types.False},
	}
}
