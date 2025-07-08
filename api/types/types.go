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

// Package types defines the core interfaces, data structures, and contracts for the RuleGo rule engine framework.
// 包 types 定义了 RuleGo 规则引擎框架的核心接口、数据结构和契约。
//
// This package serves as the foundation for the entire RuleGo ecosystem, providing:
// 该包是整个 RuleGo 生态系统的基础，提供：
//
//   - Core interfaces for components, nodes, and rule engines
//     组件、节点和规则引擎的核心接口
//   - Message structures for data flow between nodes
//     节点间数据流转的消息结构
//   - Configuration and context types for rule execution
//     规则执行的配置和上下文类型
//   - Aspect-oriented programming (AOP) support
//     面向切面编程（AOP）支持
//   - Plugin and component registry mechanisms
//     插件和组件注册机制
//
// # Extension Component Libraries
// # 扩展组件库生态
//
// RuleGo provides a complete ecosystem of extension component libraries:
// RuleGo 提供完整的扩展组件库生态系统：
//
//  1. rulego-components (https://github.com/rulego/rulego-components)
//     Core extension components including Kafka, Redis, RabbitMQ, NATS, gRPC, FastHTTP
//     核心扩展组件，包含 Kafka、Redis、RabbitMQ、NATS、gRPC、FastHTTP 等通用端点和处理组件
//
//  2. rulego-components-ai (https://github.com/rulego/rulego-components-ai)
//     AI scenario components for intelligent inference, model invocation, data preprocessing
//     AI 场景组件库，包含智能推理、模型调用、数据预处理等 AI 相关端点和组件
//
//  3. rulego-components-ci (https://github.com/rulego/rulego-components-ci)
//     CI/CD scenario components for code repositories, build tools, deployment platforms
//     CI/CD 场景组件库，包含代码仓库、构建工具、部署平台集成等 DevOps 相关组件
//
//  4. rulego-components-iot (https://github.com/rulego/rulego-components-iot)
//     IoT scenario components for device connectivity, protocol conversion, data acquisition
//     IoT 场景组件库，包含设备连接、协议转换、数据采集等物联网相关组件
//
//  5. rulego-components-etl (https://github.com/rulego/rulego-components-etl)
//     ETL scenario components for database connections, file processing, data cleansing
//     ETL 场景组件库，包含数据库连接、文件处理、数据清洗等数据处理组件
//
// These extension libraries provide modular architecture, specialized solutions, unified API interfaces,
// and support on-demand selection and seamless integration.
// 这些扩展库提供模块化架构、专用解决方案、统一 API 接口，支持按需选择和无缝集成。
//
// # Key Components
// # 关键组件
//
//   - Node: Interface for implementing rule engine components
//     Node：实现规则引擎组件的接口
//   - RuleMsg: Core message structure for data flow
//     RuleMsg：数据流转的核心消息结构
//   - RuleContext: Execution context for message processing
//     RuleContext：消息处理的执行上下文
//   - RuleEngine: Main engine interface for rule execution
//     RuleEngine：规则执行的主引擎接口
//   - ComponentRegistry: Component registration and management
//     ComponentRegistry：组件注册和管理
//
// # Architecture Overview
// # 架构概览
//
// The RuleGo framework follows a modular, component-based architecture:
// RuleGo 框架遵循模块化、基于组件的架构：
//
//  1. Messages flow through a chain of interconnected nodes
//     消息通过互连节点链进行流转
//  2. Each node implements specific business logic or transformation
//     每个节点实现特定的业务逻辑或转换
//  3. Relationships between nodes define the message routing
//     节点间的关系定义消息路由
//  4. AOP aspects provide cross-cutting concerns like monitoring
//     AOP 切面提供监控等横切关注点
//
// # Example Usage
// # 使用示例
//
//	// Implement a custom node component
//	// 实现自定义节点组件
//	type MyNode struct{}
//
//	func (n *MyNode) Type() string { return "myNode" }
//	func (n *MyNode) New() types.Node { return &MyNode{} }
//	func (n *MyNode) Init(config types.Config, configuration types.Configuration) error { return nil }
//	func (n *MyNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
//		// Process message and forward to next nodes
//		// 处理消息并转发到下一个节点
//		ctx.TellSuccess(msg)
//	}
//	func (n *MyNode) Destroy() {}
//
//	// Register the component
//	// 注册组件
//	registry.Register(&MyNode{})
//
//	// Use in rule chain DSL
//	// 在规则链 DSL 中使用
//	{
//		"ruleChain": {
//			"nodes": [
//				{
//					"id": "s1",
//					"type": "myNode",
//					"configuration": {}
//				}
//			]
//		}
//	}
//
// For detailed usage examples and documentation, see the RuleGo main package and extension libraries.
// 详细的使用示例和文档，请参见 RuleGo 主包和扩展库。
package types

import (
	"context"
)

// Relation types define the connections between nodes. These are common relations that can be customized.
// 关系类型定义节点之间的连接。这些是可以自定义的常见关系。
//
// These constants represent the standard relationship types used to route messages between nodes:
// 这些常量表示用于在节点间路由消息的标准关系类型：
//   - Success: Message processed successfully, continue to success path
//     Success：消息处理成功，继续成功路径
//   - Failure: Message processing failed, route to error handling
//     Failure：消息处理失败，路由到错误处理
//   - True/False: Boolean logic routing for filter and condition nodes
//     True/False：用于过滤器和条件节点的布尔逻辑路由
const (
	Success = "Success"
	Failure = "Failure"
	True    = "True"
	False   = "False"
)

// Flow direction types indicate the direction of message flow into and out of nodes.
// 流向类型表示消息流入和流出节点的方向。
//
// These constants are used for debugging, monitoring, and AOP aspects to track message flow:
// 这些常量用于调试、监控和 AOP 切面来跟踪消息流：
const (
	In  = "IN"  // Represents a message flowing into a node. 表示消息流入节点
	Out = "OUT" // Represents a message flowing out of a node. 表示消息流出节点
	Log = "Log" // Used for logging purposes. 用于日志记录目的
)

// Script types define the scripting languages supported for script execution within nodes.
// 脚本类型定义节点内脚本执行支持的脚本语言。
//
// These constants are used by script-enabled components to specify the execution engine:
// 这些常量由支持脚本的组件用来指定执行引擎：
const (
	AllScript = ""       // All script match. 匹配所有脚本类型
	Js        = "Js"     // Represents JavaScript scripting language. 表示 JavaScript 脚本语言
	Lua       = "Lua"    // Represents Lua scripting language. 表示 Lua 脚本语言
	Python    = "Python" // Represents Python scripting language. 表示 Python 脚本语言
)

// OnEndFunc is a callback function type that is executed when a branch of the rule chain completes.
// OnEndFunc 是规则链分支完成时执行的回调函数类型。
//
// This callback provides detailed information about the execution result:
// 此回调提供执行结果的详细信息：
//   - ctx: The rule context containing execution state
//     ctx：包含执行状态的规则上下文
//   - msg: The final message after processing
//     msg：处理后的最终消息
//   - err: Any error that occurred during processing
//     err：处理过程中发生的任何错误
//   - relationType: The relationship type that led to this endpoint
//     relationType：导致此端点的关系类型
type OnEndFunc = func(ctx RuleContext, msg RuleMsg, err error, relationType string)

// Configuration is a type for component configurations, represented as a map with string keys and interface{} values.
// Configuration 是组件配置的类型，表示为具有字符串键和 interface{} 值的映射。
//
// This flexible configuration format allows components to define their own configuration schema
// while providing type safety through validation during component initialization.
// 这种灵活的配置格式允许组件定义自己的配置模式，同时通过组件初始化期间的验证提供类型安全性。
//
// Example:
// 示例：
//
//	config := Configuration{
//	    "timeout": 30,
//	    "host": "localhost",
//	    "port": 8080,
//	    "enabled": true,
//	}
type Configuration map[string]interface{}

// Copy creates a shallow copy of the Configuration.
// Copy 创建 Configuration 的浅拷贝。
//
// This method creates a new Configuration map and copies all key-value pairs from the original.
// Note that this is a shallow copy - if values are pointers or reference types,
// they will still reference the same underlying data.
// 此方法创建一个新的 Configuration 映射并从原始映射复制所有键值对。
// 注意这是浅拷贝 - 如果值是指针或引用类型，它们仍将引用相同的底层数据。
//
// Returns:
// 返回值：
//   - Configuration: A new Configuration containing copies of all key-value pairs
//     Configuration：包含所有键值对副本的新 Configuration
func (c Configuration) Copy() Configuration {
	if c == nil {
		return nil
	}
	copy := make(Configuration, len(c))
	for key, value := range c {
		copy[key] = value
	}
	return copy
}

// ComponentType is an enum for component types: rule nodes or sub-rule chains.
// ComponentType 是组件类型的枚举：规则节点或子规则链。
//
// This type distinguishes between different kinds of components in the rule chain:
// 此类型区分规则链中不同种类的组件：
type ComponentType int

const (
	NODE     ComponentType = iota // NODE represents a rule node component. NODE 表示规则节点组件
	CHAIN                         // CHAIN represents a sub-rule chain component. CHAIN 表示子规则链组件
	ENDPOINT                      // ENDPOINT represents an endpoint component. ENDPOINT 表示端点组件
)

// PluginRegistry is an interface for providing node components via Go plugins.
// PluginRegistry 是通过 Go 插件提供节点组件的接口。
//
// This interface enables dynamic loading of components at runtime, allowing for modular architecture
// and third-party component distribution. Plugins are compiled as .so files and loaded dynamically.
// 此接口支持在运行时动态加载组件，允许模块化架构和第三方组件分发。插件编译为 .so 文件并动态加载。
//
// Implementation Guidelines:
// 实现指南：
//  1. Plugin must export a variable named "Plugins" implementing this interface
//     插件必须导出名为 "Plugins" 的变量实现此接口
//  2. Init() should handle plugin initialization and resource setup
//     Init() 应处理插件初始化和资源设置
//  3. Components() should return all components provided by the plugin
//     Components() 应返回插件提供的所有组件
//
// Example:
// 示例：
//
//	package main
//	var Plugins MyPlugins // Plugin entry point
//	type MyPlugins struct{}
//
//	func (p *MyPlugins) Init() error {
//		return nil // Initialization logic for the plugin
//	}
//
//	func (p *MyPlugins) Components() []types.Node {
//		return []types.Node{&UpperNode{}, &TimeNode{}, &FilterNode{}} // A plugin can provide multiple components
//	}
//
//	// Build command:
//	// go build -buildmode=plugin -o plugin.so plugin.go # Compile the plugin to generate a plugin.so file
//	// Registration:
//	// rulego.Registry.RegisterPlugin("test", "./plugin.so") // Register the plugin with the default RuleGo registry
type PluginRegistry interface {
	// Init initializes the plugin.
	// Init 初始化插件。
	Init() error
	// Components returns a list of components provided by the plugin.
	// Components 返回插件提供的组件列表。
	Components() []Node
}

// ComponentRegistry is an interface for registering node components.
// ComponentRegistry 是注册节点组件的接口。
//
// This registry manages the lifecycle of components and provides factory methods for component creation.
// It supports both static registration (compile-time) and dynamic registration (runtime via plugins).
// 此注册表管理组件的生命周期并提供组件创建的工厂方法。
// 它支持静态注册（编译时）和动态注册（通过插件的运行时）。
//
// Thread Safety:
// 线程安全性：
// Implementations should be thread-safe to support concurrent registration and component creation
// in multi-threaded environments.
// 实现应该是线程安全的，以支持多线程环境中的并发注册和组件创建。
//
// Usage Pattern:
// 使用模式：
//  1. Register components during application startup
//     在应用程序启动期间注册组件
//  2. Use NewNode() to create component instances for rule chains
//     使用 NewNode() 为规则链创建组件实例
//  3. Retrieve component metadata for UI configuration
//
// ComponentRegistry is the interface for managing rule engine components with isolation and discovery capabilities.
// ComponentRegistry 是管理规则引擎组件的接口，具备隔离和发现功能。
//
// 核心职责 - Core Responsibilities:
// 1. 组件生命周期管理 - Component lifecycle management
// 2. 命名空间隔离 - Namespace isolation
// 3. 动态加载与卸载 - Dynamic loading and unloading
// 4. 可视化配置支持 - Visual configuration support
//
// 隔离特性 - Isolation Features:
//   - 独立组件空间：每个注册表维护独立的组件集合 - Independent component space: each registry maintains separate component collections
//   - 类型命名空间：支持"domain/type"格式防止冲突 - Type namespaces: supports "domain/type" format to prevent conflicts
//   - 多租户支持：不同业务域使用隔离的组件注册表 - Multi-tenant support: different business domains use isolated component registries
//   - 版本管理：同一组件类型的多版本并存 - Version management: multiple versions of the same component type can coexist
//
// 组件发现 - Component Discovery:
//   - GetComponents(): 获取所有可用组件列表 - Get list of all available components
//   - GetComponentForms(): 获取组件配置表单，支持UI工具 - Get component configuration forms for UI tools
//   - NewNode(): 通过类型名称实例化组件 - Instantiate components by type name
//   - 自动组件分类和元数据提取 - Automatic component categorization and metadata extraction
//
// 使用模式 - Usage Patterns:
//
//	// 基础注册 - Basic registration
//	registry.Register(&MyCustomNode{})
//
//	// 命名空间注册 - Namespace registration
//	registry.Register(&MyNode{}) // Type() returns "mycompany/processor"
//
//	// 插件动态加载 - Plugin dynamic loading
//	registry.RegisterPlugin("businessPlugin", "./plugins/business.so")
//
//	// 获取可用组件 - Get available components
//	components := registry.GetComponents()
//	for typeName, node := range components {
//		fmt.Printf("Available: %s\n", typeName)
//	}
type ComponentRegistry interface {
	// Register adds a new component. If `node.Type()` already exists, it returns an 'already exists' error.
	// Register 添加新组件。如果 `node.Type()` 已存在，返回"已存在"错误。
	Register(node Node) error
	// RegisterPlugin loads and registers a component from an external .so file using the plugin mechanism.
	// If `name` already exists or the component list provided by the plugin `node.Type()` exists, it returns an 'already exists' error.
	// RegisterPlugin 使用插件机制从外部 .so 文件加载并注册组件。
	// 如果 `name` 已存在或插件提供的组件列表 `node.Type()` 存在，返回"已存在"错误。
	RegisterPlugin(name string, file string) error
	// Unregister removes a component or a batch of components by plugin name.
	// Unregister 通过插件名称删除组件或批量组件。
	Unregister(componentType string) error
	// NewNode creates a new instance of a node by nodeType.
	// NewNode 通过 nodeType 创建节点的新实例。
	NewNode(nodeType string) (Node, error)
	// GetComponents retrieves a complete list of all registered components in this registry instance.
	// GetComponents 检索此注册表实例中所有已注册组件的完整列表。
	//
	// This method provides component discovery capabilities for:
	// 此方法为以下场景提供组件发现功能：
	//   - Runtime component enumeration and validation  运行时组件枚举和验证
	//   - UI tools displaying available component types  UI工具显示可用的组件类型
	//   - Dynamic rule chain composition and validation  动态规则链组合和验证
	//   - Component inventory management and auditing  组件清单管理和审计
	//
	// Returns:
	// 返回：
	//   - map[string]Node: Map of component type names to component instances
	//     map[string]Node：组件类型名称到组件实例的映射
	//
	// The returned map contains:
	// 返回的映射包含：
	//   - Key: Component type identifier (e.g., "jsTransform", "mycompany/processor")
	//     Key：组件类型标识符（例如，"jsTransform"、"mycompany/processor"）
	//   - Value: Component prototype instance for metadata access
	//     Value：用于元数据访问的组件原型实例
	//
	// Note: The returned instances are prototypes for metadata only.
	// Use NewNode() to create working instances for rule chains.
	// 注意：返回的实例是仅用于元数据的原型。
	// 使用 NewNode() 为规则链创建工作实例。
	GetComponents() map[string]Node

	// GetComponentForms retrieves configuration forms for all registered components, enabling visual configuration tools.
	// GetComponentForms 检索所有已注册组件的配置表单，支持可视化配置工具。
	//
	// This method supports visual rule chain builders by providing:
	// 此方法通过提供以下内容支持可视化规则链构建器：
	//   - Component configuration schemas and form definitions  组件配置架构和表单定义
	//   - Input validation rules and constraints  输入验证规则和约束
	//   - UI rendering hints and component categorization  UI渲染提示和组件分类
	//   - Documentation and help text for each component  每个组件的文档和帮助文本
	//
	// Returns:
	// 返回：
	//   - ComponentFormList: Structured metadata for UI configuration tools
	//     ComponentFormList：UI配置工具的结构化元数据
	//
	// The returned forms enable:
	// 返回的表单支持：
	//   - Drag-and-drop rule chain editors  拖放式规则链编辑器
	//   - Dynamic configuration forms generation  动态配置表单生成
	//   - Real-time configuration validation  实时配置验证
	//   - Component documentation integration  组件文档集成
	GetComponentForms() ComponentFormList
}

// Node is the core interface for rule engine node components.
// It defines the fundamental contract for all components in the RuleGo ecosystem,
// encapsulating business logic or common functionality that can be invoked through rule chain configurations.
//
// Node 是规则引擎节点组件的核心接口。
// 它定义了 RuleGo 生态系统中所有组件的基本契约，
// 封装可通过规则链配置调用的业务逻辑或通用功能。
//
// Architecture Overview:
// 架构概述：
//
//	The Node interface represents the atomic unit of computation in RuleGo rule chains.
//	Each component encapsulates specific functionality and can be connected to other
//	components to form complex processing workflows. Components are stateless by design,
//	with each rule chain instance receiving its own component instance for data isolation.
//
//	Node 接口表示 RuleGo 规则链中的原子计算单元。每个组件封装特定功能，
//	可以连接到其他组件以形成复杂的处理工作流。组件在设计上是无状态的，
//	每个规则链实例都会收到自己的组件实例以实现数据隔离。
//
// Component Categories:
// 组件类别：
//   - Filter components: Data filtering and routing based on conditions
//     过滤器组件：基于条件的数据过滤和路由
//   - Transform components: Data transformation and enrichment
//     转换器组件：数据转换和丰富
//   - Action components: Business logic execution and external service integration
//     动作组件：业务逻辑执行和外部服务集成
//   - Flow components: Control flow and rule chain orchestration
//     流程组件：控制流和规则链编排
//   - External components: Integration with external systems and protocols
//     外部组件：与外部系统和协议的集成
//
// Lifecycle Management:
// 生命周期管理：
//
//  1. Registration: Components are registered with the ComponentRegistry
//     注册：组件通过 ComponentRegistry 注册
//  2. Instantiation: New() creates isolated instances for each rule chain
//     实例化：New() 为每个规则链创建隔离的实例
//  3. Initialization: Init() configures the component with specific parameters
//     初始化：Init() 使用特定参数配置组件
//  4. Execution: OnMsg() processes incoming messages
//     执行：OnMsg() 处理传入消息
//  5. Cleanup: Destroy() releases resources when no longer needed
//     清理：Destroy() 在不再需要时释放资源
//
// Optional Interface Extensions:
// 可选接口扩展：
//
//	Components can implement additional interfaces for enhanced functionality:
//	组件可以实现额外接口以增强功能：
//	- ComponentDefGetter: Provides metadata for visual configuration tools
//	  ComponentDefGetter：为可视化配置工具提供元数据
//	- CategoryGetter: Defines component categorization for UI organization
//	  CategoryGetter：定义组件分类以便 UI 组织
//	- DescGetter: Supplies component descriptions and documentation
//	  DescGetter：提供组件描述和文档
//
// Thread Safety Considerations:
// 线程安全考虑：
//
//   - Each rule chain receives its own component instance (data isolation)
//     每个规则链都会收到自己的组件实例（数据隔离）
//   - OnMsg() may be called concurrently from multiple goroutines
//     OnMsg() 可能从多个 goroutine 并发调用
//   - Components should avoid shared mutable state without proper synchronization
//     组件应避免在没有适当同步的情况下共享可变状态
//   - Use NodePool for expensive resource sharing across multiple instances
//     使用 NodePool 在多个实例间共享昂贵资源
//
// Best Practices:
// 最佳实践：
//   - Keep components stateless for better scalability
//     保持组件无状态以获得更好的可扩展性
//   - Use meaningful type names with namespace prefixes (e.g., "myCompany/dataProcessor")
//     使用有意义的类型名称和命名空间前缀（例如，"myCompany/dataProcessor"）
//   - Implement proper error handling and resource cleanup
//     实现适当的错误处理和资源清理
//   - Consider implementing optional interfaces for better tooling support
//     考虑实现可选接口以获得更好的工具支持
//   - Use configuration validation in Init() to catch errors early
//     在 Init() 中使用配置验证以尽早捕获错误
//
// Registration Example:
// 注册示例：
//
//	// Register a custom component
//	// 注册自定义组件
//	rulego.Registry.Register(&MyCustomNode{})
//
//	// Register from plugin
//	// 从插件注册
//	rulego.Registry.RegisterPlugin("myPlugin", "./plugin.so")
//
// Implementation Reference:
// 实现参考：
//
//	Standard implementations can be found in the `components` package.
//	Extension components are available in separate repositories:
//	标准实现可在 `components` 包中找到。
//	扩展组件可在单独的仓库中获得：
//	- github.com/rulego/rulego-components
//	- github.com/rulego/rulego-components-ai
//	- github.com/rulego/rulego-components-iot
//	- github.com/rulego/rulego-components-ci
//	- github.com/rulego/rulego-components-etl
type Node interface {
	// New creates a new instance of the component for each rule chain.
	// This method ensures data isolation between different rule chain instances,
	// preventing state sharing and potential race conditions.
	//
	// New 为每个规则链创建组件的新实例。
	// 此方法确保不同规则链实例之间的数据隔离，
	// 防止状态共享和潜在的竞态条件。
	//
	// Design Pattern:
	// 设计模式：
	//	This follows the Prototype pattern, where the registered component
	//	serves as a template for creating new instances. Each instance
	//	maintains its own state and configuration.
	//
	//	这遵循原型模式，注册的组件作为创建新实例的模板。
	//	每个实例维护自己的状态和配置。
	//
	// Returns:
	// 返回：
	//   - Node: A new component instance ready for initialization
	//     Node：准备好初始化的新组件实例
	//
	// Implementation Notes:
	// 实现注意事项：
	//   - Return a new instance of the same type, not a copy of existing data
	//     返回相同类型的新实例，而不是现有数据的副本
	//   - Initialize only default values, detailed configuration happens in Init()
	//     仅初始化默认值，详细配置在 Init() 中进行
	//   - Avoid expensive operations that should be deferred to Init()
	//     避免应该延迟到 Init() 的昂贵操作
	New() Node

	// Type returns the unique component type identifier.
	// This identifier is used for component lookup, registration, and rule chain configuration.
	//
	// Type 返回唯一的组件类型标识符。
	// 此标识符用于组件查找、注册和规则链配置。
	//
	// Naming Convention:
	// 命名约定：
	//	It is recommended to use forward slashes (/) to distinguish namespaces
	//	and prevent type name conflicts between different component libraries.
	//
	//	建议使用正斜杠 (/) 来区分命名空间，防止不同组件库之间的类型名称冲突。
	//
	// Examples:
	// 示例：
	//   - Standard components: "jsTransform", "httpClient", "delay"
	//     标准组件："jsTransform"、"httpClient"、"delay"
	//   - Company-specific: "myCompany/dataProcessor", "acme/validator"
	//     公司特定："myCompany/dataProcessor"、"acme/validator"
	//   - Protocol-specific: "mqtt/publish", "kafka/consumer"
	//     协议特定："mqtt/publish"、"kafka/consumer"
	//
	// Returns:
	// 返回：
	//   - string: Unique component type identifier
	//     string：唯一的组件类型标识符
	//
	// Requirements:
	// 要求：
	//   - Must be unique across all registered components
	//     在所有已注册组件中必须唯一
	//   - Should be descriptive and self-explanatory
	//     应该是描述性和自解释的
	//   - Should remain stable across component versions
	//     应该在组件版本间保持稳定
	Type() string

	// Init initializes the component with configuration parameters and rule engine context.
	// This method is called once during rule chain initialization and should perform
	// all necessary setup operations including parameter validation and resource allocation.
	//
	// Init 使用配置参数和规则引擎上下文初始化组件。
	// 此方法在规则链初始化期间调用一次，应执行所有必要的设置操作，
	// 包括参数验证和资源分配。
	//
	// Initialization Responsibilities:
	// 初始化职责：
	//   - Parse and validate component configuration
	//     解析和验证组件配置
	//   - Initialize external clients (HTTP, database, message queue)
	//     初始化外部客户端（HTTP、数据库、消息队列）
	//   - Set up internal state and caches
	//     设置内部状态和缓存
	//   - Validate required dependencies and resources
	//     验证所需的依赖项和资源
	//   - Register with external services if needed
	//     如需要，向外部服务注册
	//
	// Configuration Processing:
	// 配置处理：
	//	The configuration parameter contains the component-specific settings
	//	extracted from the rule chain DSL. Use the maps.Map2Struct utility
	//	to convert the configuration map to your component's configuration struct.
	//
	//	配置参数包含从规则链 DSL 中提取的组件特定设置。
	//	使用 maps.Map2Struct 工具将配置映射转换为组件的配置结构体。
	//
	// Error Handling:
	// 错误处理：
	//	Return an error if initialization fails. This will prevent the rule chain
	//	from starting and provide early feedback about configuration issues.
	//
	//	如果初始化失败，返回错误。这将阻止规则链启动并提供关于配置问题的早期反馈。
	//
	// Parameters:
	// 参数：
	//   - ruleConfig: Global rule engine configuration and shared resources
	//     ruleConfig：全局规则引擎配置和共享资源
	//   - configuration: Component-specific configuration from the rule chain DSL
	//     configuration：来自规则链 DSL 的组件特定配置
	//
	// Returns:
	// 返回：
	//   - error: Initialization error, or nil if successful
	//     error：初始化错误，成功时为 nil
	Init(ruleConfig Config, configuration Configuration) error

	// OnMsg processes incoming messages and implements the component's core functionality.
	// This method is the heart of the component and will be called for each message
	// that flows through this node in the rule chain.
	//
	// OnMsg 处理传入消息并实现组件的核心功能。
	// 此方法是组件的核心，将为流经规则链中此节点的每条消息调用。
	//
	// Message Processing Contract:
	// 消息处理契约：
	//
	//	After processing the message, the component MUST call one of the following
	//	methods to continue the rule chain execution, otherwise the chain will hang:
	//
	//	处理消息后，组件必须调用以下方法之一来继续规则链执行，否则链将挂起：
	//	- ctx.TellSuccess(msg): Forward message via "Success" relationship
	//	  ctx.TellSuccess(msg)：通过"Success"关系转发消息
	//	- ctx.TellFailure(msg, err): Forward message via "Failure" relationship
	//	  ctx.TellFailure(msg, err)：通过"Failure"关系转发消息
	//	- ctx.TellNext(msg, relationTypes...): Forward via specific relationship types
	//	  ctx.TellNext(msg, relationTypes...)：通过特定关系类型转发
	//	- ctx.DoOnEnd(msg, err, relationType): End this chain branch
	//	  ctx.DoOnEnd(msg, err, relationType)：结束此链分支
	//
	// Message Modification:
	// 消息修改：
	//	Components can modify message content, metadata, or type before forwarding.
	//	Use message copy methods when modifications might affect parallel processing branches.
	//
	//	组件可以在转发前修改消息内容、元数据或类型。
	//	当修改可能影响并行处理分支时，使用消息复制方法。
	//
	// Asynchronous Processing:
	// 异步处理：
	//	For long-running operations, use ctx.SubmitTask() to execute work in background
	//	goroutines while ensuring proper chain continuation.
	//
	//	对于长时间运行的操作，使用 ctx.SubmitTask() 在后台 goroutine 中执行工作，
	//	同时确保适当的链继续。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule context providing message routing and utility functions
	//     ctx：提供消息路由和工具函数的规则上下文
	//   - msg: The message to be processed by this component
	//     msg：此组件要处理的消息
	OnMsg(ctx RuleContext, msg RuleMsg)

	// Destroy releases any resources held by the component when it's no longer needed.
	// This method is called during rule chain shutdown, component updates, or engine destruction.
	//
	// Destroy 在不再需要组件时释放组件持有的任何资源。
	// 此方法在规则链关闭、组件更新或引擎销毁期间调用。
	//
	// Cleanup Responsibilities:
	// 清理职责：
	//   - Close external connections (HTTP clients, database connections)
	//     关闭外部连接（HTTP 客户端、数据库连接）
	//   - Release file handles and network resources
	//     释放文件句柄和网络资源
	//   - Cancel background goroutines and timers
	//     取消后台 goroutine 和定时器
	//   - Clear internal caches and temporary data
	//     清除内部缓存和临时数据
	//   - Unregister from external services
	//     从外部服务注销
	//
	// Graceful Shutdown:
	// 优雅关闭：
	//	The rule engine ensures that no new messages are sent to OnMsg()
	//	when Destroy() is called. Components can safely clean up resources
	//	without worrying about concurrent access from OnMsg().
	//
	//	规则引擎确保在调用 Destroy() 时不会向 OnMsg() 发送新消息。
	//	组件可以安全地清理资源，而无需担心来自 OnMsg() 的并发访问。
	//
	// Error Handling:
	// 错误处理：
	//	This method should not panic. Log any cleanup errors but don't fail
	//	the entire shutdown process for individual component cleanup failures.
	//
	//	此方法不应崩溃。记录任何清理错误，但不要因个别组件清理失败而导致整个关闭过程失败。
	//
	// Implementation Notes:
	// 实现注意事项：
	//   - This method may be called multiple times, implement idempotent cleanup
	//     此方法可能被多次调用，实现幂等清理
	//   - Use timeout contexts for cleanup operations to prevent hanging
	//     为清理操作使用超时上下文以防止挂起
	//   - Consider implementing a cleanup timeout to avoid blocking shutdown
	//     考虑实现清理超时以避免阻塞关闭
	Destroy()
}

// NodeCtx is the context for instantiating rule nodes.
// NodeCtx 是实例化规则节点的上下文。
//
// NodeCtx extends the basic Node interface with additional context-aware functionality,
// providing access to configuration, debug information, and node management capabilities.
// NodeCtx 扩展了基本的 Node 接口，增加了上下文感知功能，
// 提供对配置、调试信息和节点管理功能的访问。
//
// This interface serves as a wrapper around Node instances within the rule engine,
// enabling advanced features like hot reloading, debugging, and hierarchical node access.
// 此接口在规则引擎内充当 Node 实例的包装器，
// 启用热重载、调试和分层节点访问等高级功能。
//
// Key Features:
// 关键特性：
//   - Configuration management and hot reloading
//     配置管理和热重载
//   - Debug mode control for development and monitoring
//     开发和监控的调试模式控制
//   - Node identification and metadata access
//     节点标识和元数据访问
//   - DSL (Domain Specific Language) configuration access
//     DSL（领域特定语言）配置访问
type NodeCtx interface {
	Node
	Config() Config
	// IsDebugMode checks if the node is in debug mode.
	// True: When messages flow in and out of the node, the config.OnDebug callback function is called; otherwise, it is not.
	// IsDebugMode 检查节点是否处于调试模式。
	// True：当消息流入和流出节点时，调用 config.OnDebug 回调函数；否则不调用。
	IsDebugMode() bool
	// GetNodeId retrieves the component ID.
	// GetNodeId 检索组件 ID。
	GetNodeId() RuleNodeId
	// ReloadSelf refreshes the configuration of the component.
	// ReloadSelf 刷新组件的配置。
	//
	// This method enables hot reloading of component configuration without restarting the entire rule chain.
	// The def parameter should contain the new configuration in the same format as the original DSL.
	// 此方法启用组件配置的热重载，无需重启整个规则链。
	// def 参数应包含与原始 DSL 相同格式的新配置。
	ReloadSelf(def []byte) error
	// GetNodeById retrieves the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	// GetNodeById 检索子规则链中指定 ID 组件的配置。
	// 如果是节点类型，则不支持此方法。
	GetNodeById(nodeId RuleNodeId) (NodeCtx, bool)
	// DSL returns the configuration DSL of the node.
	// DSL 返回节点的配置 DSL。
	//
	// The returned byte slice contains the Domain Specific Language definition
	// used to configure this node, typically in JSON format.
	// 返回的字节片段包含用于配置此节点的领域特定语言定义，通常为 JSON 格式。
	DSL() []byte
}

// ChainCtx represents the context for rule chain management and execution.
// ChainCtx 表示规则链管理和执行的上下文。
//
// ChainCtx extends NodeCtx with capabilities specific to managing entire rule chains,
// including child node management, rule chain definitions, and engine pool access.
// ChainCtx 扩展了 NodeCtx，增加了管理整个规则链的特定功能，
// 包括子节点管理、规则链定义和引擎池访问。
//
// This interface is used for rule chain instances that contain multiple interconnected nodes,
// providing hierarchical management and advanced configuration capabilities.
// 此接口用于包含多个互连节点的规则链实例，
// 提供分层管理和高级配置功能。
//
// Key Responsibilities:
// 主要职责：
//   - Child node lifecycle management
//     子节点生命周期管理
//   - Rule chain definition and metadata access
//     规则链定义和元数据访问
//   - Engine pool integration for resource management
//     引擎池集成用于资源管理
//   - Hierarchical configuration updates
//     分层配置更新
type ChainCtx interface {
	NodeCtx
	// ReloadChild refreshes the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	// ReloadChild 刷新子规则链中指定 ID 组件的配置。
	// 如果是节点类型，则不支持此方法。
	//
	// This method enables fine-grained hot reloading of individual nodes within a rule chain
	// without affecting other nodes or the overall chain structure.
	// 此方法启用规则链内单个节点的细粒度热重载，
	// 不影响其他节点或整体链结构。
	ReloadChild(nodeId RuleNodeId, def []byte) error
	// Definition returns the definition of the rule chain.
	// Definition 返回规则链的定义。
	//
	// The returned RuleChain contains the complete structural definition,
	// including all nodes, connections, and metadata.
	// 返回的 RuleChain 包含完整的结构定义，
	// 包括所有节点、连接和元数据。
	Definition() *RuleChain
	// GetRuleEnginePool retrieves the rule engine pool.
	// GetRuleEnginePool 检索规则引擎池。
	//
	// The engine pool manages multiple rule engine instances for load balancing
	// and resource optimization in high-concurrency scenarios.
	// 引擎池管理多个规则引擎实例，用于高并发场景中的负载均衡
	// 和资源优化。
	GetRuleEnginePool() RuleEnginePool
}

// RuleContext is the interface for message processing context within the rule engine.
// It handles the transfer of messages to the next or multiple nodes and triggers their business logic.
// It also controls and orchestrates the node flow of the current execution instance.
type RuleContext interface {
	// TellSuccess notifies the rule engine that the current message has been successfully processed and sends the message to the next node via the 'Success' relationship.
	TellSuccess(msg RuleMsg)
	// TellFailure notifies the rule engine that the current message has failed to process and sends the message to the next node via the 'Failure' relationship.
	TellFailure(msg RuleMsg, err error)
	// TellNext sends the message to the next node using the specified relationTypes.
	TellNext(msg RuleMsg, relationTypes ...string)
	// TellSelf sends a message to the current node after a specified delay (in milliseconds).
	TellSelf(msg RuleMsg, delayMs int64)
	// TellNextOrElse sends the message to the next node using the specified relationTypes. If the corresponding relationType does not find the next node, it uses defaultRelationType to search.
	TellNextOrElse(msg RuleMsg, defaultRelationType string, relationTypes ...string)
	// TellFlow executes a sub-rule chain.
	// ruleChainId: The ID of the rule chain.
	// onEndFunc: Callback for when a branch of the sub-rule chain completes, returning the result of that chain. If multiple branches are triggered, it will be called multiple times.
	// onAllNodeCompleted: Callback for when all nodes have completed, with no result returned.
	// If the rule chain is not found, the message is sent to the next node via the 'Failure' relationship.
	TellFlow(ctx context.Context, ruleChainId string, msg RuleMsg, endFunc OnEndFunc, onAllNodeCompleted func())
	// TellNode starts execution from a specified node. If skipTellNext=true, only the current node is executed without notifying the next node.
	// onEnd is used to view the final execution result.
	TellNode(ctx context.Context, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// TellChainNode executes the specified node in the specified rule chain.
	// If skipTellNext=true, only the current node is executed, and no message is sent to the next node.
	TellChainNode(ctx context.Context, ruleChainId, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// NewMsg creates a new message instance.
	NewMsg(msgType string, metaData *Metadata, data string) RuleMsg
	// GetSelfId retrieves the current node ID.
	GetSelfId() string
	// Self retrieves the current node instance.
	Self() NodeCtx
	// From retrieves the node instance from which the message entered the current node.
	From() NodeCtx
	// RuleChain retrieves the rule chain instance where the current node resides.
	RuleChain() NodeCtx
	// Config retrieves the configuration of the rule engine.
	Config() Config
	// SubmitTack submits an asynchronous task for execution.
	//Deprecated: Use Flow SubmitTask instead.
	SubmitTack(task func())
	// SubmitTask submits an asynchronous task for execution.
	SubmitTask(task func())
	// SetEndFunc sets the callback function for when the current message processing ends.
	SetEndFunc(f OnEndFunc) RuleContext
	// GetEndFunc retrieves the callback function for when the current message processing ends.
	GetEndFunc() OnEndFunc
	// SetContext sets a context for sharing semaphores or data across different component instances.
	SetContext(c context.Context) RuleContext
	// GetContext retrieves the context for sharing semaphores or data across different component instances.
	GetContext() context.Context
	// SetOnAllNodeCompleted sets the callback for when all nodes have completed execution.
	SetOnAllNodeCompleted(onAllNodeCompleted func())
	// DoOnEnd triggers the OnEnd callback function.
	DoOnEnd(msg RuleMsg, err error, relationType string)
	// SetCallbackFunc sets a callback function.
	SetCallbackFunc(functionName string, f interface{})
	// GetCallbackFunc retrieves a callback function.
	GetCallbackFunc(functionName string) interface{}
	// OnDebug calls the configured OnDebug callback function.
	OnDebug(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// SetExecuteNode sets the current node.
	// If relationTypes is empty, execute the current node; otherwise,
	// find and execute the child nodes of the current node.
	SetExecuteNode(nodeId string, relationTypes ...string)
	// TellCollect gathers the execution results from multiple nodes and registers a callback function to collect the result list.
	// If it is the first time to register, it returns true; otherwise, it returns false.
	TellCollect(msg RuleMsg, callback func(msgList []WrapperMsg)) bool
	// GetOut retrieves the OUT message.
	GetOut() RuleMsg
	// GetErr retrieves the IN or OUT error.
	GetErr() error
	// GlobalCache returns a Cache instance for global cache operations
	// The cache items will persist until manually deleted or expired
	GlobalCache() Cache
	// ChainCache returns a Cache instance for rule chain cache operations
	// This is a namespaced version of GlobalCache that automatically prefixes all keys with the rule chain ID
	// The cache items with rule chain prefix will be automatically cleared when the rule chain is destroyed
	ChainCache() Cache
	// GetEnv gets environment variables and metadata from message
	// useMetadata: whether to include metadata in the result
	GetEnv(msg RuleMsg, useMetadata bool) map[string]interface{}
}

// RuleContextOption is a function type for modifying RuleContext options.
type RuleContextOption func(RuleContext)

// WithEndFunc is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use `types.WithOnEnd` instead.
func WithEndFunc(endFunc func(ctx RuleContext, msg RuleMsg, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(func(ctx RuleContext, msg RuleMsg, err error, relationType string) {
			endFunc(ctx, msg, err)
		})
	}
}

// WithOnEnd is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
func WithOnEnd(endFunc func(ctx RuleContext, msg RuleMsg, err error, relationType string)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(endFunc)
	}
}

// WithContext sets a context for sharing data or semaphores between different component instances.
// It is also used for timeout cancellation.
func WithContext(c context.Context) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetContext(c)
	}
}

// WithOnAllNodeCompleted is a callback function for when the rule chain execution completes.
func WithOnAllNodeCompleted(onAllNodeCompleted func()) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetOnAllNodeCompleted(onAllNodeCompleted)
	}
}

// WithOnRuleChainCompleted is a callback function for when the rule chain execution completes and collects the runtime logs of each node.
func WithOnRuleChainCompleted(onCallback func(ctx RuleContext, snapshot RuleChainRunSnapshot)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnRuleChainCompleted, onCallback)
	}
}

// WithOnNodeCompleted is a callback function for when a node execution completes and collects the node's runtime log.
func WithOnNodeCompleted(onCallback func(ctx RuleContext, nodeRunLog RuleNodeRunLog)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnNodeCompleted, onCallback)
	}
}

// WithOnNodeDebug is a callback function for node debug logs, called in real-time asynchronously. It is triggered only if the node is configured with debugMode.
func WithOnNodeDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncDebug, onDebug)
	}
}

// WithStartNode 设置第一个开始执行节点
// WithStartNode sets the first node to start execution.
func WithStartNode(nodeId string) RuleContextOption {
	return func(rc RuleContext) {
		if nodeId == "" {
			return
		}
		rc.SetExecuteNode(nodeId)
	}
}

// WithTellNext 设置通过指定节点Id，查找下一个或者多个执行节点。用于恢复规则链执行链路
// WithTellNext sets the next or multiple execution nodes by specifying the node ID.
// It is used to restore the execution path of the rule chain.
func WithTellNext(fromNodeId string, relationTypes ...string) RuleContextOption {
	return func(rc RuleContext) {
		if fromNodeId == "" {
			return
		}
		rc.SetExecuteNode(fromNodeId, relationTypes...)
	}
}

// JsEngine is a JavaScript script engine interface.
// JsEngine 是 JavaScript 脚本引擎接口。
//
// This interface provides an abstraction layer for JavaScript execution within RuleGo components,
// enabling dynamic script execution for data transformation, filtering, and business logic.
// 此接口为 RuleGo 组件内的 JavaScript 执行提供抽象层，
// 启用数据转换、过滤和业务逻辑的动态脚本执行。
//
// The JavaScript engine supports:
// JavaScript 引擎支持：
//   - Function execution with parameter passing
//     带参数传递的函数执行
//   - Access to RuleContext for message processing
//     访问 RuleContext 进行消息处理
//   - Resource management and cleanup
//     资源管理和清理
type JsEngine interface {
	// Execute runs a specified function in the JS script, which is initialized when the JsEngine instance is created.
	// ctx is the message chain context.
	// functionName is the name of the function to execute.
	// argumentList is the list of arguments for the function.
	// Execute 运行 JS 脚本中的指定函数，该脚本在创建 JsEngine 实例时初始化。
	// ctx 是消息链上下文。
	// functionName 是要执行的函数名称。
	// argumentList 是函数的参数列表。
	Execute(ctx RuleContext, functionName string, argumentList ...interface{}) (interface{}, error)
	// Stop releases the resources of the JS engine.
	// Stop 释放 JS 引擎的资源。
	//
	// This method should be called when the engine is no longer needed to prevent memory leaks
	// and ensure proper cleanup of JavaScript contexts and associated resources.
	// 当不再需要引擎时应调用此方法，以防止内存泄漏并确保正确清理 JavaScript 上下文和相关资源。
	Stop()
}

// Parser is an interface for parsing rule chain definition files (DSL).
// The default implementation uses JSON. If other formats are used to define rule chains, this interface can be implemented.
// Then register it with the rule engine like this: `rulego.NewConfig(WithParser(&MyParser{})`
// Parser 是解析规则链定义文件（DSL）的接口。
// 默认实现使用 JSON。如果使用其他格式定义规则链，可以实现此接口。
// 然后像这样将其注册到规则引擎：`rulego.NewConfig(WithParser(&MyParser{})`
//
// This interface enables support for multiple DSL formats, allowing users to define rule chains
// using their preferred configuration language (JSON, YAML, XML, etc.).
// 此接口启用对多种 DSL 格式的支持，允许用户使用他们首选的配置语言（JSON、YAML、XML 等）定义规则链。
type Parser interface {
	// DecodeRuleChain parses a rule chain structure from a description file.
	// DecodeRuleChain 从描述文件解析规则链结构。
	DecodeRuleChain(rootRuleChain []byte) (RuleChain, error)
	// DecodeRuleNode parses a rule node structure from a description file.
	// DecodeRuleNode 从描述文件解析规则节点结构。
	DecodeRuleNode(rootRuleChain []byte) (RuleNode, error)
	// EncodeRuleChain converts a rule chain structure into a description file.
	// EncodeRuleChain 将规则链结构转换为描述文件。
	EncodeRuleChain(def interface{}) ([]byte, error)
	// EncodeRuleNode converts a rule node structure into a description file.
	// EncodeRuleNode 将规则节点结构转换为描述文件。
	EncodeRuleNode(def interface{}) ([]byte, error)
}

// Pool is an interface for a coroutine pool.
// Pool 是协程池的接口。
//
// This interface provides an abstraction for managing goroutine pools to control concurrency
// and resource usage in high-throughput scenarios. It enables efficient task scheduling
// and prevents resource exhaustion in concurrent message processing.
// 此接口提供管理协程池的抽象，以控制高吞吐量场景中的并发性和资源使用。
// 它启用高效的任务调度并防止并发消息处理中的资源耗尽。
//
// Implementation Characteristics:
// 实现特性：
//   - Fixed or dynamic pool sizing based on load
//     基于负载的固定或动态池大小
//   - Task queue management for pending operations
//     待处理操作的任务队列管理
//   - Graceful shutdown and resource cleanup
//     优雅关闭和资源清理
//   - Load balancing across available workers
//     在可用工作器间进行负载均衡
//
// Usage Pattern:
// 使用模式：
//
//	pool := NewWorkerPool(maxWorkers)
//	defer pool.Release()
//
//	if err := pool.Submit(func() {
//	    // Task execution
//	}); err != nil {
//	    // Handle pool full or error
//	}
type Pool interface {
	// Submit submits a task to the coroutine pool.
	// Returns an error if the coroutine pool is full.
	// Submit 向协程池提交任务。
	// 如果协程池已满，返回错误。
	Submit(task func()) error
	// Release releases the resources of the pool.
	// Release 释放池的资源。
	//
	// This method should be called during application shutdown to ensure
	// all pending tasks are completed and resources are properly cleaned up.
	// 此方法应在应用程序关闭期间调用，以确保所有待处理任务完成并正确清理资源。
	Release()
}

// EmptyRuleNodeId is an empty node ID.
// EmptyRuleNodeId 是空的节点 ID。
//
// This constant represents an uninitialized or invalid node identifier,
// commonly used for comparison and validation purposes.
// 此常量表示未初始化或无效的节点标识符，
// 通常用于比较和验证目的。
var EmptyRuleNodeId = RuleNodeId{}

// RuleNodeId is a type definition for component IDs.
// RuleNodeId 是组件 ID 的类型定义。
//
// This structure uniquely identifies components within the RuleGo framework,
// combining both identification and type information for proper routing and management.
// 此结构在 RuleGo 框架内唯一标识组件，
// 结合标识和类型信息以实现正确的路由和管理。
//
// The combination of Id and Type allows the framework to:
// Id 和 Type 的组合允许框架：
//   - Distinguish between different component categories
//     区分不同的组件类别
//   - Route messages to appropriate handlers
//     将消息路由到适当的处理程序
//   - Manage component lifecycles effectively
//     有效管理组件生命周期
//   - Support hierarchical node structures
//     支持分层节点结构
type RuleNodeId struct {
	// Id is the node ID.
	// Id 是节点 ID。
	//
	// This should be unique within the scope of a rule chain or engine instance.
	// 这在规则链或引擎实例的范围内应该是唯一的。
	Id string
	// Type is the component type, either a node or a sub-rule chain.
	// Type 是组件类型，可以是节点或子规则链。
	//
	// This field determines how the component is processed and managed by the engine.
	// 此字段决定组件如何被引擎处理和管理。
	Type ComponentType
}

// RuleNodeRelation defines the relationship between nodes.
// RuleNodeRelation 定义节点间的关系。
//
// This structure represents the directed connections between components in a rule chain,
// enabling message flow and execution path determination. Relations form the backbone
// of rule chain topology and determine how messages are routed through the system.
// 此结构表示规则链中组件间的有向连接，
// 启用消息流和执行路径确定。关系构成规则链拓扑的骨干，
// 并决定消息如何通过系统路由。
//
// Key Characteristics:
// 关键特性：
//   - Directed relationships (from InId to OutId)
//     有向关系（从 InId 到 OutId）
//   - Conditional routing based on RelationType
//     基于 RelationType 的条件路由
//   - Support for multiple output paths per node
//     支持每个节点的多个输出路径
//   - Dynamic relationship evaluation during runtime
//     运行时的动态关系评估
type RuleNodeRelation struct {
	// InId is the incoming component ID.
	// InId 是传入组件 ID。
	//
	// This represents the source node from which messages originate.
	// 这表示消息来源的源节点。
	InId RuleNodeId
	// OutId is the outgoing component ID.
	// OutId 是传出组件 ID。
	//
	// This represents the destination node to which messages are routed.
	// 这表示消息路由到的目标节点。
	OutId RuleNodeId
	// RelationType is the type of relationship, such as True, False, Success, Failure, or other custom types.
	// RelationType 是关系类型，如 True、False、Success、Failure 或其他自定义类型。
	//
	// This field determines the condition under which messages flow from InId to OutId.
	// Custom relationship types enable domain-specific routing logic.
	// 此字段决定消息从 InId 流向 OutId 的条件。
	// 自定义关系类型启用领域特定的路由逻辑。
	RelationType string
}

// ScriptFuncSeparator is the delimiter for script function names.
// ScriptFuncSeparator 是脚本函数名称的分隔符。
//
// This constant is used to separate script type from function name in composite identifiers,
// enabling support for multiple script engines and function namespacing.
// 此常量用于在复合标识符中分离脚本类型和函数名称，
// 启用对多个脚本引擎和函数命名空间的支持。
//
// Usage pattern: "scriptType#functionName"
// 使用模式："scriptType#functionName"
// Example: "Js#processData" or "Lua#filterMessage"
// 示例："Js#processData" 或 "Lua#filterMessage"
const ScriptFuncSeparator = "#"

// Script is used to register native functions or custom functions defined in Go.
// Script 用于注册在 Go 中定义的原生函数或自定义函数。
//
// This structure provides a flexible mechanism for extending RuleGo with custom logic,
// supporting both traditional scripting languages and native Go functions.
// 此结构提供了使用自定义逻辑扩展 RuleGo 的灵活机制，
// 支持传统脚本语言和原生 Go 函数。
//
// Script Registration Patterns:
// 脚本注册模式：
//  1. JavaScript/Lua script content as string
//     JavaScript/Lua 脚本内容作为字符串
//  2. Go function references for direct execution
//     Go 函数引用用于直接执行
//  3. Plugin-based script loading for dynamic functionality
//     基于插件的脚本加载用于动态功能
//
// Type-Content Mapping:
// 类型-内容映射：
//   - "Js": JavaScript source code (string)
//     "Js"：JavaScript 源代码（字符串）
//   - "Lua": Lua source code (string)
//     "Lua"：Lua 源代码（字符串）
//   - "Go": Go function reference (func interface{})
//     "Go"：Go 函数引用（func interface{}）
type Script struct {
	// Type is the script type, default is Js.
	// Type 是脚本类型，默认为 Js。
	//
	// Supported types include predefined constants (Js, Lua, Python) and custom types.
	// 支持的类型包括预定义常量（Js、Lua、Python）和自定义类型。
	Type string
	// Content is the script content or custom function.
	// Content 是脚本内容或自定义函数。
	//
	// The content type varies based on the script Type:
	// 内容类型根据脚本 Type 而变化：
	//   - String: Script source code for interpreted languages
	//     String：解释语言的脚本源代码
	//   - Function: Go function reference for native execution
	//     Function：原生执行的 Go 函数引用
	//   - []byte: Compiled bytecode for optimized execution
	//     []byte：优化执行的编译字节码
	Content interface{}
}

// Callbacks is a set of callback functions for pool events.
// Callbacks 是池事件的回调函数集。
//
// This structure provides event-driven notifications for rule chain and component lifecycle events,
// enabling monitoring, logging, and integration with external systems.
// 此结构为规则链和组件生命周期事件提供事件驱动的通知，
// 启用监控、日志记录和与外部系统的集成。
//
// Event Lifecycle:
// 事件生命周期：
//  1. OnNew: Triggered when new rule chains are created
//     OnNew：创建新规则链时触发
//  2. OnUpdated: Triggered when existing components are modified
//     OnUpdated：修改现有组件时触发
//  3. OnDeleted: Triggered when components are removed
//     OnDeleted：删除组件时触发
//
// Use Cases:
// 使用案例：
//   - Audit logging for configuration changes
//     配置更改的审计日志
//   - Cache invalidation for updated components
//     更新组件的缓存失效
//   - Metrics collection for monitoring systems
//     监控系统的指标收集
//   - External system synchronization
//     外部系统同步
type Callbacks struct {
	// OnNew is called when a new rule chain is created.
	// OnNew 在创建新规则链时调用。
	//
	// Parameters:
	// 参数：
	//   - chainId: Unique identifier of the new rule chain
	//     chainId：新规则链的唯一标识符
	//   - dsl: Complete DSL definition of the rule chain
	//     dsl：规则链的完整 DSL 定义
	OnNew func(chainId string, dsl []byte)

	// OnUpdated is called when an existing component is updated.
	// OnUpdated 在更新现有组件时调用。
	//
	// Parameters:
	// 参数：
	//   - chainId: Identifier of the parent rule chain
	//     chainId：父规则链的标识符
	//   - nodeId: Identifier of the updated component
	//     nodeId：更新组件的标识符
	//   - dsl: Updated DSL definition of the component
	//     dsl：组件的更新 DSL 定义
	OnUpdated func(chainId, nodeId string, dsl []byte)

	// OnDeleted is called when a component or rule chain is deleted.
	// OnDeleted 在删除组件或规则链时调用。
	//
	// Parameters:
	// 参数：
	//   - id: Identifier of the deleted entity (chain or node)
	//     id：被删除实体的标识符（链或节点）
	OnDeleted func(id string)
}
