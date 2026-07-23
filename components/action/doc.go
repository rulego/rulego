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

// Package action provides action node components for the RuleGo rule engine.
// Action nodes perform operations, transformations, and business logic execution as part of rule chain processing.
//
// Package action provides action node components for the RuleGo rule engine.
// Action nodes act as part of the rule chain processing to execute operations, transformations, and business logic.
//
// Registration:
// Registration:
//
// All components are automatically registered during package initialization:
// All components are automatically registered during package initialization:
//
// Example Usage:
// Example:
//
//	// Delay message processing
//	Delayed message processing
//	{
//		"id": "delay1",
//		"type": "delay",
//		"configuration": {
//			"periodInSeconds": 30,
//			"maxPendingMsgs": 1000
//		}
//	}
//
//
//	// Execute custom function
//	Execute custom functions
//	{
//		"id": "customLogic",
//		"type": "functions",
//		"configuration": {
//			"functionName": "calculateTotal"
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// For detailed documentation of each component, please refer to their respective source files.
package action
