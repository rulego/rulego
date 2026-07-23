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
// Package transform provides conversion node components for the RuleGo rule engine.
// Transformation nodes modify, transform, and reconstruct message data as messages flow through the rule chain, enabling data processing and format conversion.
//
// Transform nodes are responsible for:
// Transition nodes are responsible for:
//
// • Modifying message content and structure
// • Converting between different data formats
// • Enriching messages with additional data
// • Applying business rules and transformations
// • Processing metadata and message properties
//
// Available Transform Components:
// Available conversion components:
//
//   - JsTransformNode: JavaScript-based data transformation with full scripting capabilities
//     JavaScript-based data conversion with full scripting capabilities
//   - ExprTransformNode: Expression-based field transformation using expression language
//     Use expression language for expression-based field conversion
//   - TemplateNode: Template-based message formatting and data restructuring
//     Template-based message formatting and data refactoring
//   - MetadataTransformNode: Message metadata modification and manipulation
//     Message metadata modification and operation
//
// Component Categories by Function:
// Components classified by function:
//
// Script-Based Transformation:
// Script-based conversion:
//   - JsTransformNode: Full JavaScript transformation with access to built-in functions
//     Complete JavaScript conversion with access to built-in functions
//   - ExprTransformNode: Expression language for simple field transformations
//     Expression language, used for simple field conversions
//
// Template and Formatting:
// Templates and formatting:
//   - TemplateNode: Apply templates for structured data formatting
//     Templates are used for structured data formatting
//
// Metadata Processing:
// Metadata Processing:
//   - MetadataTransformNode: Transform and manipulate message metadata
//     Transform and manipulate message metadata
//
// Transform Output Relations:
// Conversion output relationships:
//
// Transform nodes typically produce two types of outputs:
// Conversion nodes typically produce two types of outputs:
//   - "Success": Transformation completed successfully
//   - "Failure": Error occurred during transformation
//
// JavaScript Engine Support:
// JavaScript engine supports:
//
// JavaScript-based components support ECMAScript 5.1+ with partial ES6 features:
// JavaScript-based components support ECMAScript 5.1+ and some ES6 features:
//   - Built-in variables: msg, metadata, msgType, dataType
//     Built-in variables: msg, metadata, msgType, dataType
//   - Built-in functions: $ctx.ChainCache(), $ctx.GlobalCache(), global.*, vars.*
//     Built-in function: $ctx.ChainCache(), $ctx.GlobalCache(), global.*, vars.*
//   - Modern syntax: async/await, Promise, let/const, arrow functions
//     Modern syntax: async/await, Promise, let/const, arrow function
//
// Registration:
// Registration:
//
// All components are automatically registered during package initialization:
// All components are automatically registered during package initialization:
//
//	func init() {
//		Registry.Add(&JsTransformNode{})
//		Registry.Add(&ExprTransformNode{})
//		Registry.Add(&TemplateNode{})
//		Registry.Add(&MetadataTransformNode{})
//	}
//
// Usage Examples:
// Example:
//
//	// JavaScript transformation
//	JavaScript conversion
//	{
//		"id": "jsTransform",
//		"type": "jsTransform",
//		"configuration": {
//			"jsScript": "msg.temperature = msg.temperature * 9/5 + 32; return {msg: msg, metadata: metadata, msgType: msgType};"
//		}
//	}
//
//	// Expression transformation
//	Expression conversion
//	{
//		"id": "exprTransform",
//		"type": "exprTransform",
//		"configuration": {
//			"expr": "upper(msg.name)"
//		}
//	}
//
//	// Template formatting
//	Template formatting
//	{
//		"id": "templateFormat",
//		"type": "template",
//		"configuration": {
//			"template": "Hello ${msg.name}, your order ${msg.orderId} is ready!"
//		}
//	}
//
//	// Metadata transformation
//	Metadata transformation
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
// Extended component library:
//
// RuleGo provides additional component libraries for specialized use cases:
// RuleGo provides additional component libraries for specialized use cases:
//
//   - rulego-components: Additional extension components for general use
//     rulego-components: General-purpose extension components
//     https://github.com/rulego/rulego-components
//
//   - rulego-components-ai: AI scenario components for machine learning integration
//     rulego-components-ai: AI scenario components integrated with machine learning
//     https://github.com/rulego/rulego-components-ai
//
//   - rulego-components-ci: CI/CD scenario components for DevOps workflows
//     rulego-components-ci: CI/CD scene component for DevOps workflows
//     https://github.com/rulego/rulego-components-ci
//
//   - rulego-components-iot: IoT scenario components for device connectivity
//     rulego-components-iot: IoT scenario components connected to devices
//     https://github.com/rulego/rulego-components-iot
//
//   - rulego-components-etl: ETL scenario components for data processing
//     rulego-components-etl: ETL scenario component for data processing
//     https://github.com/rulego/rulego-components-etl
//
// For detailed documentation on individual components, see their respective source files.
// For detailed documentation of each component, please refer to their respective source files.
package transform
