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

// Package transform provides transformation node components for the RuleGo rule engine.
// Transform nodes modify, convert, and restructure message data as it flows through rule chains,
// enabling data processing and format conversion.
//
// Package transform 为 RuleGo 规则引擎提供转换节点组件。
// 转换节点在消息流经规则链时修改、转换和重构消息数据，启用数据处理和格式转换。
//
// Transform nodes are responsible for:
// 转换节点负责：
//
// • Modifying message content and structure  修改消息内容和结构
// • Converting between different data formats  在不同数据格式之间转换
// • Enriching messages with additional data  用附加数据丰富消息
// • Applying business rules and transformations  应用业务规则和转换
// • Processing metadata and message properties  处理元数据和消息属性
//
// Available Transform Components:
// 可用的转换组件：
//
//   - JsTransformNode: JavaScript-based data transformation with full scripting capabilities
//     基于 JavaScript 的数据转换，具有完整的脚本功能
//   - ExprTransformNode: Expression-based field transformation using expression language
//     使用表达式语言进行基于表达式的字段转换
//   - TemplateNode: Template-based message formatting and data restructuring
//     基于模板的消息格式化和数据重构
//   - MetadataTransformNode: Message metadata modification and manipulation
//     消息元数据修改和操作
//
// Component Categories by Function:
// 按功能分类的组件：
//
// Script-Based Transformation:
// 基于脚本的转换：
//   - JsTransformNode: Full JavaScript transformation with access to built-in functions
//     完整的 JavaScript 转换，可访问内置函数
//   - ExprTransformNode: Expression language for simple field transformations
//     表达式语言，用于简单的字段转换
//
// Template and Formatting:
// 模板和格式化：
//   - TemplateNode: Apply templates for structured data formatting
//     应用模板进行结构化数据格式化
//
// Metadata Processing:
// 元数据处理：
//   - MetadataTransformNode: Transform and manipulate message metadata
//     转换和操作消息元数据
//
// Transform Output Relations:
// 转换输出关系：
//
// Transform nodes typically produce two types of outputs:
// 转换节点通常产生两种类型的输出：
//   - "Success": Transformation completed successfully  转换成功完成
//   - "Failure": Error occurred during transformation  转换期间发生错误
//
// JavaScript Engine Support:
// JavaScript 引擎支持：
//
// JavaScript-based components support ECMAScript 5.1+ with partial ES6 features:
// 基于 JavaScript 的组件支持 ECMAScript 5.1+ 和部分 ES6 特性：
//   - Built-in variables: msg, metadata, msgType, dataType
//     内置变量：msg、metadata、msgType、dataType
//   - Built-in functions: $ctx.ChainCache(), $ctx.GlobalCache(), global.*, vars.*
//     内置函数：$ctx.ChainCache()、$ctx.GlobalCache()、global.*、vars.*
//   - Modern syntax: async/await, Promise, let/const, arrow functions
//     现代语法：async/await、Promise、let/const、箭头函数
//
// Registration:
// 注册：
//
// All components are automatically registered during package initialization:
// 所有组件在包初始化期间自动注册：
//
//	func init() {
//		Registry.Add(&JsTransformNode{})
//		Registry.Add(&ExprTransformNode{})
//		Registry.Add(&TemplateNode{})
//		Registry.Add(&MetadataTransformNode{})
//	}
//
// Usage Examples:
// 使用示例：
//
//	// JavaScript transformation
//	// JavaScript 转换
//	{
//		"id": "jsTransform",
//		"type": "jsTransform",
//		"configuration": {
//			"jsScript": "msg.temperature = msg.temperature * 9/5 + 32; return {msg: msg, metadata: metadata, msgType: msgType};"
//		}
//	}
//
//	// Expression transformation
//	// 表达式转换
//	{
//		"id": "exprTransform",
//		"type": "exprTransform",
//		"configuration": {
//			"expr": "upper(msg.name)"
//		}
//	}
//
//	// Template formatting
//	// 模板格式化
//	{
//		"id": "templateFormat",
//		"type": "template",
//		"configuration": {
//			"template": "Hello ${msg.name}, your order ${msg.orderId} is ready!"
//		}
//	}
//
//	// Metadata transformation
//	// 元数据转换
//	{
//		"id": "metadataTransform",
//		"type": "metadataTransform",
//		"configuration": {
//			"mapping": {
//				"userId": "${msg.user.id}",
//				"timestamp": "${msg.created_at}"
//			}
//		}
//	}
//
// Extended Component Libraries:
// 扩展组件库：
//
// RuleGo provides additional component libraries for specialized use cases:
// RuleGo 为专门用例提供额外的组件库：
//
//   - rulego-components: Additional extension components for general use
//     rulego-components：通用扩展组件
//     https://github.com/rulego/rulego-components
//
//   - rulego-components-ai: AI scenario components for machine learning integration
//     rulego-components-ai：机器学习集成的 AI 场景组件
//     https://github.com/rulego/rulego-components-ai
//
//   - rulego-components-ci: CI/CD scenario components for DevOps workflows
//     rulego-components-ci：DevOps 工作流的 CI/CD 场景组件
//     https://github.com/rulego/rulego-components-ci
//
//   - rulego-components-iot: IoT scenario components for device connectivity
//     rulego-components-iot：设备连接的 IoT 场景组件
//     https://github.com/rulego/rulego-components-iot
//
//   - rulego-components-etl: ETL scenario components for data processing
//     rulego-components-etl：数据处理的 ETL 场景组件
//     https://github.com/rulego/rulego-components-etl
//
// For detailed documentation on individual components, see their respective source files.
// 有关各个组件的详细文档，请参见其各自的源文件。
package transform
