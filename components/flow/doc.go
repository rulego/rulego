/*
 * Copyright 2024 The RuleGo Authors.
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

// Package flow provides components for managing sub-rule chains and component references in the RuleGo rule engine.
// These components enable rule chain composition, modularity, and reusable processing patterns
// through nested rule chain execution and component referencing.
//
// Package flow provides components for the RuleGo rule engine to manage sub-rule chains and component references.
// These components implement rule chain combination, modularization, and reusable processing modes through nested rule chain execution and component references.
//
// Available Components:
// Available components:
//
//   - FlowNode (ChainNode): Executes a sub-rule chain within the current rule chain
//     FlowNode (ChainNode): executes sub-rule chains within the current rule chain
//   - RefNode: References and executes another component within the current rule chain
//     RefNode: References and executes other components within the current rule chain
//
// Component Functions:
// Component functions:
//
// FlowNode (Sub-Chain Execution):
// FlowNode (subchain execution):
//   - Invokes separate rule chains as nested workflows
//     Call independent rule chains as nested workflows
//   - Enables rule chain composition and modularity
//     Enable rule chain combination and modularization
//   - Supports isolated processing contexts
//     Supports isolated processing contexts
//   - Allows complex workflow orchestration
//     Allows for complex workflow orchestration
//
// RefNode (Component Reference):
// RefNode (component reference):
//   - References other components by type or ID
//     Reference other components by type or ID
//   - Promotes code reuse and maintainability
//     Promote code reuse and maintainability
//   - Enables dynamic component selection
//     Enable dynamic component selection
//   - Supports configuration sharing patterns
//     Supports configuring sharing mode
//
// Use Cases:
// Usage scenarios:
//
// Sub-Chain Processing:
// Subchain handling:
//   - Complex business logic breakdown
//     Breaking down complex business logic
//   - Reusable processing workflows
//     Reusable processing workflows
//   - Conditional workflow execution
//     Conditional workflow execution
//   - Multi-stage data processing
//     Multi-stage data processing
//
// Component Referencing:
// Component reference:
//   - Shared component logic
//     Shared component logic
//   - Configuration templates
//     Configure the template
//   - Dynamic processing paths
//     Dynamic processing path
//   - Component composition patterns
//     Component combination mode
//
// Registration:
// Registration:
//
// Components are automatically registered during package initialization:
// Components are automatically registered during package initialization:
//
//	func init() {
//		Registry.Add(&FlowNode{})
//		Registry.Add(&RefNode{})
//	}
//
// Example Usage:
// Example:
//
//	// Execute sub-rule chain
//	Execute the sub-rule chain
//	{
//		"id": "processOrder",
//		"type": "flow",
//		"configuration": {
//			"targetId": "order_processing_chain"
//		}
//	}
//
//	// Reference another component
//	Reference other components
//	{
//		"id": "validateData",
//		"type": "ref",
//		"configuration": {
//			"componentType": "validator",
//			"targetId": "schema_validator"
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// For detailed documentation of each component, please refer to their respective source files.
package flow
