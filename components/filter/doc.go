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

// Package filter provides message filtering and routing components for the RuleGo rule engine.
//
// These components are designed to filter or route messages based on certain conditions:
//
// - JsFilter: Filters messages using JavaScript conditions
// - JsSwitch: Routes messages to different paths based on JavaScript logic
// - MsgTypeSwitch: Routes messages to different paths based on their type
//
// Each component is registered with the Registry, allowing them to be used
// within rule chains. These components help in creating conditional logic and
// message routing within rule processing flows.
//
// To use these components, include them in your rule chain configuration and
// connect them to other nodes to create branching logic in your rules.
//
// You can use these components in your rule chain DSL file by referencing
// their Type. For example:
//
//	{
//	  "id": "node1",
//	  "type": "jsFilter",
//	  "name": "js filter",
//	  "configuration": {
//	    "jsScript": "return msg.temperature > 50;"
//	  }
//	}
package filter
