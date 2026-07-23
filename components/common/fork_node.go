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

//Example of rule chain node configuration:
//{
//        "id": "s2",
//        "type": "fork",
//        "name": "并行网关"
//      }
import (
	"github.com/rulego/rulego/api/types"
)

// init registers ForkNode components
// init registers the ForkNode component with the default registry.
func init() {
	Registry.Add(&ForkNode{})
}

// ForkNode splits the message stream into multiple parallel execution paths with parallel network nodes
// ForkNode is a parallel gateway node that splits the message flow into multiple parallel execution paths.
//
// Core algorithm:
// Core Algorithm:
// 1. Receive single input message - Receive single input message
// 2. Broadcast the same message to all connected outbound relations
// 3. Initiate parallel execution of all downstream nodes
//
// Workflow pattern - Workflow pattern:
//   - Fan-out pattern for parallel processing
//   - Gateway pattern for workflow control
//   - Broadcast pattern for message distribution
//
// Use cases:
//   - Parallel workflow execution
//   - Message broadcasting to multiple processors
//   - Workflow branching for concurrent operations
//
// No configuration required:
//   - Behavior determined by rule chain connections
//   - Always succeeds (no failure cases)
type ForkNode struct {
	// ForkNode does not require configuration fields and operates as a simple message broadcaster
	// ForkNode requires no configuration fields as it operates as a simple message broadcaster
}

// Type returns the component type
// Type returns the component type identifier.
func (x *ForkNode) Type() string {
	return "fork"
}

// New creates an instance
// New creates a new instance.
func (x *ForkNode) New() types.Node {
	return &ForkNode{}
}

// Init initializes the component
// Init initializes the component.
func (x *ForkNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

// OnMsg processes messages by broadcasting them to all outbound connections for parallel processing
// OnMsg processes incoming messages by broadcasting them to all connected outbound relations.
func (x *ForkNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *ForkNode) Destroy() {
}

// Def returns the component form definition
func (x *ForkNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Parallel gateway that broadcasts message to all connected outbound relations. Use with join to collect results",
		RelationTypes: &[]string{types.Success},
	}
}
