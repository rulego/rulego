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

// Package flow provides components for managing sub-rule chains and referencing components in the RuleGo rule engine.
//
// These components are designed to enhance the flexibility and modularity of rule chains:
//
// - ChainNode: Executes a sub-rule chain as part of the current rule chain
// - RefNode: References and executes another component within the current rule chain
//
// Each component is registered with the Registry, allowing them to be used
// within rule chains. These components enable the creation of modular and
// reusable rule structures.
//
// The ChainNode allows for nested rule execution by invoking separate rule chains
// as part of a larger workflow. The RefNode provides the ability to reference and
// execute other components, promoting code reuse and maintainability.
//
// To use these components, include them in your rule chain configuration and
// configure them to manage the flow and structure of your rule processing logic.
//
// You can use these components in your rule chain DSL file by referencing
// their Type. For example:
//
//	{
//	  "id": "node1",
//	  "type": "flow",
//	  "name": "sub rule chain",
//	  "configuration": {
//	    "targetId": "sub_chain_01"
//	  }
//	}
package flow
