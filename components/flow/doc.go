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
// Package flow 为 RuleGo 规则引擎提供管理子规则链和组件引用的组件。
// 这些组件通过嵌套规则链执行和组件引用实现规则链组合、模块化和可重用处理模式。
//
// Available Components:
// 可用组件：
//
//   - FlowNode (ChainNode): Executes a sub-rule chain within the current rule chain
//     FlowNode（ChainNode）：在当前规则链内执行子规则链
//   - RefNode: References and executes another component within the current rule chain
//     RefNode：在当前规则链内引用和执行其他组件
//
// Component Functions:
// 组件功能：
//
// FlowNode (Sub-Chain Execution):
// FlowNode（子链执行）：
//   - Invokes separate rule chains as nested workflows
//     调用独立规则链作为嵌套工作流
//   - Enables rule chain composition and modularity
//     启用规则链组合和模块化
//   - Supports isolated processing contexts
//     支持隔离的处理上下文
//   - Allows complex workflow orchestration
//     允许复杂的工作流编排
//
// RefNode (Component Reference):
// RefNode（组件引用）：
//   - References other components by type or ID
//     按类型或ID引用其他组件
//   - Promotes code reuse and maintainability
//     促进代码重用和可维护性
//   - Enables dynamic component selection
//     启用动态组件选择
//   - Supports configuration sharing patterns
//     支持配置共享模式
//
// Use Cases:
// 使用场景：
//
// Sub-Chain Processing:
// 子链处理：
//   - Complex business logic breakdown
//     复杂业务逻辑分解
//   - Reusable processing workflows
//     可重用处理工作流
//   - Conditional workflow execution
//     条件工作流执行
//   - Multi-stage data processing
//     多阶段数据处理
//
// Component Referencing:
// 组件引用：
//   - Shared component logic
//     共享组件逻辑
//   - Configuration templates
//     配置模板
//   - Dynamic processing paths
//     动态处理路径
//   - Component composition patterns
//     组件组合模式
//
// Registration:
// 注册：
//
// Components are automatically registered during package initialization:
// 组件在包初始化期间自动注册：
//
//	func init() {
//		Registry.Add(&FlowNode{})
//		Registry.Add(&RefNode{})
//	}
//
// Example Usage:
// 使用示例：
//
//	// Execute sub-rule chain
//	// 执行子规则链
//	{
//		"id": "processOrder",
//		"type": "flow",
//		"configuration": {
//			"targetId": "order_processing_chain"
//		}
//	}
//
//	// Reference another component
//	// 引用其他组件
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
// 有关各个组件的详细文档，请参见其各自的源文件。
package flow
