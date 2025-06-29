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
// Package action 为 RuleGo 规则引擎提供动作节点组件。
// 动作节点作为规则链处理的一部分执行操作、转换和业务逻辑。
//
// Registration:
// 注册：
//
// All components are automatically registered during package initialization:
// 所有组件在包初始化期间自动注册：
//
//	func init() {
//		Registry.Add(&DelayNode{})
//		Registry.Add(&ForNode{})
//		Registry.Add(&ExecNode{})
//		// ... other components
//	}
//
// Example Usage:
// 使用示例：
//
//	// Delay message processing
//	// 延迟消息处理
//	{
//		"id": "delay1",
//		"type": "delay",
//		"configuration": {
//			"periodInSeconds": 30,
//			"maxPendingMsgs": 1000
//		}
//	}
//
//	// Iterate over collection
//	// 遍历集合
//	{
//		"id": "processItems",
//		"type": "for",
//		"configuration": {
//			"range": "msg.items",
//			"do": "processItem",
//			"mode": 1
//		}
//	}
//
//	// Execute custom function
//	// 执行自定义函数
//	{
//		"id": "customLogic",
//		"type": "functions",
//		"configuration": {
//			"functionName": "calculateTotal"
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// 有关各个组件的详细文档，请参见其各自的源文件。
package action
