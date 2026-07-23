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
	"github.com/rulego/rulego/api/types"
)

// init registers the EndNode component with the default registry.
func init() {
	Registry.Add(&EndNode{})
}

// EndNode is the termination node component, used to trigger the end callback of the rule chain. If the rule chain has an end node component, it replaces the default branch end behavior, and only triggers the end callback when the node is terminated
// EndNode is an end node component that triggers the end callback of the rule chain. If the rule chain has an end node component set, it will replace the default branch ending behavior.
//
// Function Description:
// Function Description:
// 1. Receive messages and trigger DoOnEnd callbacks - Receives messages and triggers DoOnEnd callbacks
// 2. Use the relation type passed from the previous node - Uses the relation type passed from the previous node
// 3. Does not continue passing messages to the next node - Does not continue passing messages to the next nodes
//
// Usage scenarios:
// Use Cases:
// - Explicit end point of rule chains
// - Trigger specific end processing logic
// - Replace default branch ending behavior - Replace default branch ending behavior
type EndNode struct {
}

// Type returns the component type
// Type returns the component type identifier.
func (x *EndNode) Type() string {
	return types.NodeTypeEnd
}

// New creates a new instance.
func (x *EndNode) New() types.Node {
	return &EndNode{}
}

// Init initializes the component.
func (x *EndNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// No configuration needed
	return nil
}

// OnMsg processes the incoming message and triggers the end callback.
func (x *EndNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	relationType := ""
	if relationTypes := ctx.GetRelationTypes(); len(relationTypes) > 0 {
		relationType = relationTypes[0]
	}
	ctx.DoOnEnd(msg, ctx.GetErr(), relationType)
}

func (x *EndNode) Destroy() {
}

// Def returns the component form definition
func (x *EndNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "End node that triggers the rule chain end callback. Replaces default branch ending behavior",
		RelationTypes: &[]string{},
	}
}
