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

// Package filter provides filter node components for the RuleGo rule engine.
// Filter nodes evaluate conditions and route messages based on boolean logic,
// enabling conditional processing within rule chains.
//
// Package filter provides filter node components for the RuleGo rule engine.
// Filter nodes evaluate conditions and, based on Boolean logical routing messages, enable conditional handling within the rule chain.
//
// Filter nodes are essential for decision-making in rule chains, responsible for:
// Filter nodes are crucial for decision-making in the rule chain and are responsible for:
//
// • Evaluating boolean conditions and expressions
// • Routing messages based on True/False outcomes
// • Implementing complex conditional logic
// • Filtering data based on criteria
// • Grouping and coordinating multiple conditions
// • Performing type checking and validation
//
// Available Filter Components:
// Available filter components:
//
//   - ExprFilterNode: Evaluates complex expressions using expression language
//     Evaluate complex expressions using an expression language
//   - JsFilterNode: Executes JavaScript-based filter logic
//     Execute JavaScript-based filtering logic
//   - JsSwitchNode: JavaScript-based conditional routing
//     JavaScript-based conditional routing
//   - GroupFilterNode: Coordinates multiple filter conditions
//     Coordinate multiple filtering conditions
//   - MsgTypeSwitchNode: Routes messages based on message type
//     Routing messages based on message type
//   - SwitchNode: General purpose conditional routing
//     Universal conditional routing
//   - FieldFilterNode: Filters based on specific field values
//     Filtering based on specific field values
//   - ForkNode: Parallel message processing gateway
//     Parallel message processing gateway
//
// Component Categories by Function:
// Components classified by function:
//
// Expression Evaluation:
// Expression Evaluation:
//   - ExprFilterNode: Advanced expression language support
//     High-level expression language support
//   - JsFilterNode: JavaScript-based conditions
//     JavaScript-based conditions
//
// Message Routing:
// Message routing:
//   - MsgTypeSwitchNode: Message type-based routing
//     Routing based on message type
//   - SwitchNode: General conditional routing
//     Universal conditional routing
//   - JsSwitchNode: JavaScript-based routing
//     JavaScript-based routing
//
// Data Filtering:
// Data Filtering:
//   - FieldFilterNode: Field-based conditions
//     Field-based conditions
//
// Coordination:
// Coordination:
//   - GroupFilterNode: Multiple condition coordination
//     Multi-condition coordination
//   - ForkNode: Parallel processing coordination
//     Coordinate and handle in parallel
//
// Filter Output Relations:
// Filter output relationships:
//
// Filter nodes typically produce three types of outputs:
// Filter nodes typically produce three types of outputs:
//   - "True": Condition evaluated to true
//   - "False": Condition evaluated to false
//   - "Failure": Error occurred during evaluation
//
// Usage Example:
// Example:
//
//	// Register filter components with the rule engine
//	Register the filter component with the rule engine
//	rulego.Registry.Register(&ExprFilterNode{})
//	rulego.Registry.Register(&JsFilterNode{})
//	rulego.Registry.Register(&MsgTypeSwitchNode{})
//
//	// Use in rule chain configuration:
//	Used in the rule chain configuration:
//	{
//		"id": "temperatureFilter",
//		"type": "exprFilter",
//		"configuration": {
//			"expr": "temperature > 25.0"
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// For detailed documentation of each component, please refer to their respective source files.
package filter
