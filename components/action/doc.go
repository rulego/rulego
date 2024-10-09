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

// Package action provides a collection of action components for the RuleGo rule engine.
//
// These components are designed to perform various actions within a rule chain, including:
//
// - DelayNode: Introduces a time delay in rule execution
// - ExecCommandNode: Executes system commands
// - ForNode: Implements loop functionality for iterating over data
// - FunctionsNode: Allows calling custom-defined functions
// - GroupActionNode: Groups multiple nodes and executes them asynchronously
// - IteratorNode: Iterates over data (deprecated, use ForNode instead)
// - JoinNode: Merges results from multiple asynchronous nodes
// - JsLogNode: Logs messages using JavaScript
//
// Each component is registered with the Registry, allowing them to be used
// within rule chains. These components can be configured and connected to create
// complex rule processing flows.
//
// To use these components, include them in your rule chain configuration and
// ensure they are properly connected to other nodes in the chain.
//
// You can use these components in your rule chain DSL file by referencing
// their Type. For example:
//
//	  {
//	    "id": "node1",
//	    "type": "for",
//	    "name": "for",
//	    "configuration": {
//				"range": "msg.items",
//				"do":        "s3"
//	    }
//	  }
package action
