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
// Package filter 为 RuleGo 规则引擎提供过滤器节点组件。
// 过滤器节点评估条件并基于布尔逻辑路由消息，在规则链内启用条件处理。
//
// Filter nodes are essential for decision-making in rule chains, responsible for:
// 过滤器节点对规则链中的决策至关重要，负责：
//
// • Evaluating boolean conditions and expressions  评估布尔条件和表达式
// • Routing messages based on True/False outcomes  基于 True/False 结果路由消息
// • Implementing complex conditional logic  实现复杂的条件逻辑
// • Filtering data based on criteria  基于条件过滤数据
// • Grouping and coordinating multiple conditions  分组和协调多个条件
// • Performing type checking and validation  执行类型检查和验证
//
// Available Filter Components:
// 可用的过滤器组件：
//
//   - ExprFilterNode: Evaluates complex expressions using expression language
//     使用表达式语言评估复杂表达式
//   - JsFilterNode: Executes JavaScript-based filter logic
//     执行基于 JavaScript 的过滤逻辑
//   - JsSwitchNode: JavaScript-based conditional routing
//     基于 JavaScript 的条件路由
//   - GroupFilterNode: Coordinates multiple filter conditions
//     协调多个过滤条件
//   - MsgTypeSwitchNode: Routes messages based on message type
//     基于消息类型路由消息
//   - SwitchNode: General purpose conditional routing
//     通用条件路由
//   - FieldFilterNode: Filters based on specific field values
//     基于特定字段值过滤
//   - ForkNode: Parallel message processing gateway
//     并行消息处理网关
//
// Component Categories by Function:
// 按功能分类的组件：
//
// Expression Evaluation:
// 表达式评估：
//   - ExprFilterNode: Advanced expression language support
//     高级表达式语言支持
//   - JsFilterNode: JavaScript-based conditions
//     基于 JavaScript 的条件
//
// Message Routing:
// 消息路由：
//   - MsgTypeSwitchNode: Message type-based routing
//     基于消息类型的路由
//   - SwitchNode: General conditional routing
//     通用条件路由
//   - JsSwitchNode: JavaScript-based routing
//     基于 JavaScript 的路由
//
// Data Filtering:
// 数据过滤：
//   - FieldFilterNode: Field-based conditions
//     基于字段的条件
//
// Coordination:
// 协调：
//   - GroupFilterNode: Multiple condition coordination
//     多条件协调
//   - ForkNode: Parallel processing coordination
//     并行处理协调
//
// Filter Output Relations:
// 过滤器输出关系：
//
// Filter nodes typically produce three types of outputs:
// 过滤器节点通常产生三种类型的输出：
//   - "True": Condition evaluated to true  条件评估为真
//   - "False": Condition evaluated to false  条件评估为假
//   - "Failure": Error occurred during evaluation  评估期间发生错误
//
// Usage Example:
// 使用示例：
//
//	// Register filter components with the rule engine
//	// 向规则引擎注册过滤器组件
//	rulego.Registry.Register(&ExprFilterNode{})
//	rulego.Registry.Register(&JsFilterNode{})
//	rulego.Registry.Register(&MsgTypeSwitchNode{})
//
//	// Use in rule chain configuration:
//	// 在规则链配置中使用：
//	{
//		"id": "temperatureFilter",
//		"type": "exprFilter",
//		"configuration": {
//			"expr": "temperature > 25.0"
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// 有关各个组件的详细文档，请参见其各自的源文件。
package filter
