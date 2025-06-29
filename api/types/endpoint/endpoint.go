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

// Package endpoint provides the core definitions and interfaces for endpoints in the RuleGo framework.
// Endpoints serve as entry points for external data to flow into rule chains, abstracting different
// input sources and providing a unified interface for message processing and routing.
//
// Package endpoint 为 RuleGo 框架中的端点提供核心定义和接口。
// 端点作为外部数据流入规则链的入口点，抽象不同的输入源并提供统一的消息处理和路由接口。
//
// Core Concepts / 核心概念：
//
// • Endpoint: The main abstraction for input sources (HTTP, MQTT, WebSocket, etc.)  端点：输入源的主要抽象
// • Router: Defines how incoming messages are routed to rule chains  路由器：定义传入消息如何路由到规则链
// • Message: Abstracts incoming and outgoing message data  消息：抽象传入和传出的消息数据
// • Exchange: Contains both request and response messages  交换：包含请求和响应消息
// • Process: Middleware-style processing functions  处理：中间件风格的处理函数
//
// Architecture / 架构：
//
// The endpoint system follows a layered architecture:
// 端点系统遵循分层架构：
//
// 1. Message Layer: Abstracts incoming/outgoing data  消息层：抽象传入/传出数据
// 2. Processing Layer: Transforms and validates messages  处理层：转换和验证消息
// 3. Routing Layer: Routes messages to appropriate destinations  路由层：将消息路由到适当的目标
// 4. Execution Layer: Executes rule chains or components  执行层：执行规则链或组件
//
// Endpoint Lifecycle / 端点生命周期：
//
// 1. Creation: Endpoint is created and configured  创建：端点被创建和配置
// 2. Initialization: Resources are allocated and connections established  初始化：分配资源并建立连接
// 3. Start: Endpoint begins accepting incoming messages  启动：端点开始接受传入消息
// 4. Processing: Messages are processed through routers  处理：消息通过路由器处理
// 5. Shutdown: Resources are cleaned up gracefully  关闭：优雅地清理资源
//
// Usage Patterns / 使用模式：
//
// Static Configuration / 静态配置：
//
//	endpoint := &rest.Rest{}
//	endpoint.Init(config, restConfig)
//	router := endpoint.NewRouter().From("/api/data").To("chain:processing")
//	endpoint.POST(router)
//	endpoint.Start()
//
// Dynamic Configuration / 动态配置：
//
//	factory := endpoint.NewFactory()
//	dynamicEndpoint, err := factory.NewFromDsl(dslBytes)
//	dynamicEndpoint.Start()
//
// Message Processing Pipeline / 消息处理管道：
//
// The message processing follows this flow:
// 消息处理遵循以下流程：
//
// 1. External Message → Endpoint  外部消息 → 端点
// 2. Message → RequestMessage  消息 → 请求消息
// 3. Router Matching & Processing  路由器匹配和处理
// 4. Rule Chain/Component Execution  规则链/组件执行
// 5. Response Generation → ResponseMessage  响应生成 → 响应消息
// 6. ResponseMessage → External Response  响应消息 → 外部响应
package endpoint

import (
	"context"
	"net/http"
	"net/textproto"
	"sync"

	"github.com/rulego/rulego/api/types"
)

// Event constants define various lifecycle and operational events in the endpoint system.
// These events enable monitoring and handling of endpoint state changes and operations.
// 事件常量定义端点系统中的各种生命周期和操作事件。
// 这些事件使得监控和处理端点状态变化和操作成为可能。
const (
	// EventConnect represents a connection establishment event.
	// Triggered when a new client connection is established (e.g., WebSocket connection).
	// EventConnect 表示连接建立事件。
	// 当建立新的客户端连接时触发（例如，WebSocket 连接）。
	EventConnect = "Connect"

	// EventDisconnect represents a connection termination event.
	// Triggered when a client connection is closed or lost.
	// EventDisconnect 表示连接终止事件。
	// 当客户端连接关闭或丢失时触发。
	EventDisconnect = "Disconnect"

	// EventInitServer represents a server initialization event.
	// Triggered when the endpoint server is being initialized.
	// EventInitServer 表示服务器初始化事件。
	// 当端点服务器正在初始化时触发。
	EventInitServer = "InitServer"

	// EventCompletedServer represents a server completion event.
	// Triggered when the endpoint server has completed its operations.
	// EventCompletedServer 表示服务器完成事件。
	// 当端点服务器完成其操作时触发。
	EventCompletedServer = "completedServer"
)

// OnEvent is a callback function type for handling endpoint events.
// It provides a flexible way to respond to various endpoint lifecycle and operational events.
//
// OnEvent 是处理端点事件的回调函数类型。
// 它提供了响应各种端点生命周期和操作事件的灵活方式。
//
// Parameters / 参数：
// • eventName: The name of the event being triggered  被触发的事件名称
// • params: Variable number of parameters specific to the event type  特定于事件类型的可变数量参数
//
// Usage Examples / 使用示例：
//
//	endpoint.SetOnEvent(func(eventName string, params ...interface{}) {
//	    switch eventName {
//	    case endpoint.EventConnect:
//	        log.Printf("Client connected: %v", params[0])
//	    case endpoint.EventDisconnect:
//	        log.Printf("Client disconnected: %v", params[0])
//	    }
//	})
type OnEvent func(eventName string, params ...interface{})

// Endpoint defines the core interface for all endpoint implementations in the RuleGo framework.
// It provides the fundamental operations needed for message input, routing, and lifecycle management.
//
// Endpoint 定义 RuleGo 框架中所有端点实现的核心接口。
// 它提供消息输入、路由和生命周期管理所需的基本操作。
//
// Key Responsibilities / 主要职责：
//
// • Message Input: Accept messages from external sources  消息输入：接受来自外部源的消息
// • Router Management: Add, remove, and configure message routers  路由器管理：添加、删除和配置消息路由器
// • Lifecycle Control: Start, stop, and manage endpoint lifecycle  生命周期控制：启动、停止和管理端点生命周期
// • Event Handling: Process and notify about endpoint events  事件处理：处理和通知端点事件
// • Interceptor Support: Apply cross-cutting concerns through interceptors  拦截器支持：通过拦截器应用横切关注点
//
// Implementation Guidelines / 实现指南：
//
// All endpoint implementations should:
// 所有端点实现应该：
//
// • Be thread-safe for concurrent operations  对并发操作是线程安全的
// • Support graceful shutdown procedures  支持优雅关闭程序
// • Handle connection failures and recovery  处理连接故障和恢复
// • Provide meaningful error messages  提供有意义的错误消息
// • Support dynamic router configuration  支持动态路由器配置
type Endpoint interface {
	// Node interface provides basic component functionality including initialization,
	// type identification, and lifecycle management.
	// Node 接口提供基本的组件功能，包括初始化、类型识别和生命周期管理。
	types.Node

	// Id returns a unique identifier for the endpoint instance.
	// This ID is used for endpoint registration, lookup, and management operations.
	//
	// Id 返回端点实例的唯一标识符。
	// 此 ID 用于端点注册、查找和管理操作。
	//
	// Returns / 返回：
	// • string: Unique endpoint identifier  唯一的端点标识符
	Id() string

	// SetOnEvent registers an event listener function for the endpoint.
	// The listener will be called for various endpoint lifecycle and operational events.
	//
	// SetOnEvent 为端点注册事件监听器函数。
	// 监听器将为各种端点生命周期和操作事件被调用。
	//
	// Parameters / 参数：
	// • onEvent: Event listener function  事件监听器函数
	SetOnEvent(onEvent OnEvent)

	// Start initiates the endpoint service and begins accepting incoming messages.
	// This method should establish necessary connections, bind to ports, and prepare
	// the endpoint for message processing.
	//
	// Start 启动端点服务并开始接受传入消息。
	// 此方法应建立必要的连接、绑定到端口并准备端点进行消息处理。
	//
	// Returns / 返回：
	// • error: Error if startup fails, nil on success  启动失败时的错误，成功时为 nil
	//
	// Behavior / 行为：
	// • Should be idempotent (safe to call multiple times)  应该是幂等的（多次调用安全）
	// • Should not block the calling goroutine  不应阻塞调用的协程
	// • Should handle resource allocation and connection establishment  应处理资源分配和连接建立
	Start() error

	// AddInterceptors adds global interceptors to the endpoint processing pipeline.
	// Interceptors are executed in the order they are added for all incoming messages.
	//
	// AddInterceptors 向端点处理管道添加全局拦截器。
	// 拦截器按添加顺序对所有传入消息执行。
	//
	// Parameters / 参数：
	// • interceptors: Processing functions to add to the pipeline  要添加到管道的处理函数
	//
	// Usage / 使用：
	// Interceptors can be used for cross-cutting concerns such as:
	// 拦截器可用于横切关注点，如：
	// • Authentication and authorization  身份验证和授权
	// • Logging and monitoring  日志记录和监控
	// • Rate limiting and throttling  速率限制和节流
	// • Message transformation and validation  消息转换和验证
	AddInterceptors(interceptors ...Process)

	// AddRouter adds a message router to the endpoint with optional configuration parameters.
	// Routers define how incoming messages are matched and routed to rule chains or components.
	//
	// AddRouter 向端点添加消息路由器，带有可选的配置参数。
	// 路由器定义传入消息如何匹配并路由到规则链或组件。
	//
	// Parameters / 参数：
	// • router: Router configuration defining message routing logic  定义消息路由逻辑的路由器配置
	// • params: Protocol-specific parameters (e.g., HTTP methods)  协议特定参数（例如，HTTP 方法）
	//
	// Returns / 返回：
	// • string: Router ID for future reference and management  用于将来引用和管理的路由器 ID
	// • error: Error if router addition fails  路由器添加失败时的错误
	//
	// Note / 注意：
	// Some endpoints may return a modified router ID that should be used for future operations.
	// 某些端点可能返回修改后的路由器 ID，应用于将来的操作。
	AddRouter(router Router, params ...interface{}) (string, error)

	// RemoveRouter removes a message router from the endpoint by its ID.
	// This operation immediately stops routing messages to the specified router.
	//
	// RemoveRouter 通过 ID 从端点删除消息路由器。
	// 此操作立即停止将消息路由到指定的路由器。
	//
	// Parameters / 参数：
	// • routerId: ID of the router to remove  要删除的路由器 ID
	// • params: Protocol-specific parameters for removal  删除的协议特定参数
	//
	// Returns / 返回：
	// • error: Error if router removal fails or router not found  路由器删除失败或未找到路由器时的错误
	//
	// Behavior / 行为：
	// • Should gracefully handle in-flight messages  应优雅处理正在处理的消息
	// • Should clean up any router-specific resources  应清理任何路由器特定的资源
	RemoveRouter(routerId string, params ...interface{}) error
}

// DynamicEndpoint extends the basic Endpoint interface with dynamic configuration capabilities.
// It allows endpoints to be created, modified, and reloaded at runtime using DSL configurations.
//
// DynamicEndpoint 扩展基本的 Endpoint 接口，具有动态配置功能。
// 它允许端点在运行时使用 DSL 配置进行创建、修改和重新加载。
//
// Key Features / 主要特性：
//
// • Runtime Configuration: Modify endpoint behavior without restart  运行时配置：无需重启即可修改端点行为
// • DSL Integration: Use declarative configuration for complex setups  DSL 集成：使用声明性配置进行复杂设置
// • Hot Reloading: Update routing rules and configurations dynamically  热重载：动态更新路由规则和配置
// • Template Support: Support for variable substitution in configurations  模板支持：配置中的变量替换支持
//
// Use Cases / 使用场景：
//
// • Configuration Management Systems  配置管理系统
// • Multi-tenant Applications  多租户应用程序
// • Dynamic API Gateways  动态 API 网关
// • Development and Testing Environments  开发和测试环境
type DynamicEndpoint interface {
	// Endpoint provides all basic endpoint functionality
	// Endpoint 提供所有基本端点功能
	Endpoint

	// SetId sets the unique identifier for the dynamic endpoint.
	// This method is typically called during endpoint initialization.
	//
	// SetId 设置动态端点的唯一标识符。
	// 此方法通常在端点初始化期间调用。
	//
	// Parameters / 参数：
	// • id: Unique identifier for the endpoint  端点的唯一标识符
	SetId(id string)

	// SetConfig sets the rule engine configuration for the dynamic endpoint.
	// This configuration affects how the endpoint interacts with rule chains.
	//
	// SetConfig 设置动态端点的规则引擎配置。
	// 此配置影响端点如何与规则链交互。
	//
	// Parameters / 参数：
	// • config: Rule engine configuration  规则引擎配置
	SetConfig(config types.Config)

	// SetRouterOptions sets default options that will be applied to all routers.
	// These options provide common configuration for router behavior.
	//
	// SetRouterOptions 设置将应用于所有路由器的默认选项。
	// 这些选项为路由器行为提供通用配置。
	//
	// Parameters / 参数：
	// • opts: Router configuration options  路由器配置选项
	SetRouterOptions(opts ...RouterOption)

	// SetRestart sets whether the endpoint should restart when configuration changes.
	// When true, the endpoint will be fully restarted; when false, only routing is updated.
	//
	// SetRestart 设置配置更改时端点是否应重启。
	// 为 true 时，端点将完全重启；为 false 时，仅更新路由。
	//
	// Parameters / 参数：
	// • restart: Whether to restart on configuration changes  配置更改时是否重启
	SetRestart(restart bool)

	// SetInterceptors sets the global interceptors for the dynamic endpoint.
	// This replaces any existing interceptors with the provided ones.
	//
	// SetInterceptors 设置动态端点的全局拦截器。
	// 这将用提供的拦截器替换任何现有的拦截器。
	//
	// Parameters / 参数：
	// • interceptors: Processing functions to set as global interceptors  要设置为全局拦截器的处理函数
	SetInterceptors(interceptors ...Process)

	// Reload reloads the dynamic endpoint with a new DSL configuration.
	// The behavior depends on the restart setting and configuration changes.
	//
	// Reload 使用新的 DSL 配置重新加载动态端点。
	// 行为取决于重启设置和配置更改。
	//
	// Parameters / 参数：
	// • dsl: JSON byte array containing the new endpoint configuration  包含新端点配置的 JSON 字节数组
	// • opts: Additional options for the reload operation  重新加载操作的附加选项
	//
	// Returns / 返回：
	// • error: Error if reload fails  重新加载失败时的错误
	//
	// Behavior / 行为：
	// • If restart=true: Full endpoint restart with new configuration  如果 restart=true：使用新配置完全重启端点
	// • If restart=false: Only update routing without service interruption  如果 restart=false：仅更新路由而不中断服务
	// • Routing conflicts may force a restart regardless of setting  路由冲突可能强制重启，无论设置如何
	Reload(dsl []byte, opts ...DynamicEndpointOption) error

	// ReloadFromDef reloads the dynamic endpoint with a new DSL definition structure.
	// This is an alternative to Reload() that accepts a structured configuration.
	//
	// ReloadFromDef 使用新的 DSL 定义结构重新加载动态端点。
	// 这是 Reload() 的替代方案，接受结构化配置。
	//
	// Parameters / 参数：
	// • def: Structured endpoint DSL definition  结构化端点 DSL 定义
	// • opts: Additional options for the reload operation  重新加载操作的附加选项
	//
	// Returns / 返回：
	// • error: Error if reload fails  重新加载失败时的错误
	ReloadFromDef(def types.EndpointDsl, opts ...DynamicEndpointOption) error

	// AddOrReloadRouter adds a new router or reloads an existing one with new configuration.
	// This method provides fine-grained control over individual router updates.
	//
	// AddOrReloadRouter 添加新路由器或使用新配置重新加载现有路由器。
	// 此方法提供对单个路由器更新的细粒度控制。
	//
	// Parameters / 参数：
	// • dsl: JSON byte array containing the router configuration  包含路由器配置的 JSON 字节数组
	// • opts: Additional options for the operation  操作的附加选项
	//
	// Returns / 返回：
	// • error: Error if operation fails  操作失败时的错误
	AddOrReloadRouter(dsl []byte, opts ...DynamicEndpointOption) error

	// Definition returns the current DSL definition of the dynamic endpoint.
	// This provides access to the endpoint's configuration structure.
	//
	// Definition 返回动态端点的当前 DSL 定义。
	// 这提供对端点配置结构的访问。
	//
	// Returns / 返回：
	// • types.EndpointDsl: Current endpoint DSL definition  当前端点 DSL 定义
	Definition() types.EndpointDsl

	// DSL returns the current DSL configuration as a JSON byte array.
	// This is useful for serialization, storage, or external processing.
	//
	// DSL 返回当前 DSL 配置作为 JSON 字节数组。
	// 这对于序列化、存储或外部处理很有用。
	//
	// Returns / 返回：
	// • []byte: JSON representation of the current configuration  当前配置的 JSON 表示
	DSL() []byte

	// Target returns the underlying concrete endpoint implementation.
	// This allows access to protocol-specific functionality when needed.
	//
	// Target 返回底层的具体端点实现。
	// 这允许在需要时访问协议特定的功能。
	//
	// Returns / 返回：
	// • Endpoint: The underlying endpoint implementation  底层端点实现
	Target() Endpoint

	// SetRuleChain stores the original rule chain DSL definition when the endpoint
	// is initialized from a rule chain configuration.
	//
	// SetRuleChain 当端点从规则链配置初始化时存储原始规则链 DSL 定义。
	//
	// Parameters / 参数：
	// • ruleChain: Original rule chain DSL definition  原始规则链 DSL 定义
	SetRuleChain(ruleChain *types.RuleChain)

	// GetRuleChain retrieves the original rule chain DSL definition that was used
	// to initialize this endpoint, if any.
	//
	// GetRuleChain 检索用于初始化此端点的原始规则链 DSL 定义（如果有）。
	//
	// Returns / 返回：
	// • *types.RuleChain: Original rule chain DSL, nil if not initialized from rule chain  原始规则链 DSL，如果不是从规则链初始化则为 nil
	GetRuleChain() *types.RuleChain
}

// Message defines the abstraction for data received at an endpoint.
// It provides a unified interface for accessing message content, headers, parameters,
// and converting messages to the RuleGo processing format.
//
// Message 定义在端点接收的数据的抽象。
// 它提供访问消息内容、头部、参数以及将消息转换为 RuleGo 处理格式的统一接口。
//
// Key Concepts / 关键概念：
//
// • Protocol Abstraction: Provides common interface across different protocols  协议抽象：跨不同协议提供通用接口
// • Data Access: Uniform access to message body, headers, and parameters  数据访问：统一访问消息体、头部和参数
// • Format Conversion: Seamless conversion to RuleGo message format  格式转换：无缝转换为 RuleGo 消息格式
// • Error Handling: Built-in error tracking and status management  错误处理：内置错误跟踪和状态管理
//
// Implementation Notes / 实现注意事项：
//
// • Message implementations should be thread-safe where possible  消息实现应尽可能线程安全
// • Body() should support lazy loading for performance optimization  Body() 应支持延迟加载以进行性能优化
// • GetMsg() should cache converted messages to avoid repeated conversion  GetMsg() 应缓存转换后的消息以避免重复转换
type Message interface {
	// Body returns the raw message body as a byte slice.
	// Implementations may use lazy loading for performance optimization.
	//
	// Body 返回原始消息体作为字节切片。
	// 实现可能使用延迟加载进行性能优化。
	//
	// Returns / 返回：
	// • []byte: Raw message body content  原始消息体内容
	Body() []byte

	// Headers returns the message headers in a standardized format.
	// This provides access to protocol-specific metadata and properties.
	//
	// Headers 以标准化格式返回消息头。
	// 这提供对协议特定元数据和属性的访问。
	//
	// Returns / 返回：
	// • textproto.MIMEHeader: Standardized header format  标准化头部格式
	Headers() textproto.MIMEHeader

	// From returns the origin identifier of the message.
	// The format depends on the protocol (URL for HTTP, topic for MQTT, etc.).
	//
	// From 返回消息的来源标识符。
	// 格式取决于协议（HTTP 的 URL，MQTT 的主题等）。
	//
	// Returns / 返回：
	// • string: Origin identifier specific to the protocol  特定于协议的来源标识符
	From() string

	// GetParam retrieves a parameter value by key.
	// This may include query parameters, path parameters, or protocol-specific data.
	//
	// GetParam 通过键检索参数值。
	// 这可能包括查询参数、路径参数或协议特定数据。
	//
	// Parameters / 参数：
	// • key: Parameter name to retrieve  要检索的参数名称
	//
	// Returns / 返回：
	// • string: Parameter value, empty string if not found  参数值，如果未找到则为空字符串
	GetParam(key string) string

	// SetMsg sets the converted RuleMsg for this message.
	// This is typically used for caching the conversion result.
	//
	// SetMsg 为此消息设置转换后的 RuleMsg。
	// 这通常用于缓存转换结果。
	//
	// Parameters / 参数：
	// • msg: Converted RuleMsg to associate with this message  要与此消息关联的转换后的 RuleMsg
	SetMsg(msg *types.RuleMsg)

	// GetMsg converts the message to RuleGo's internal message format.
	// This method should handle data type detection and metadata population.
	//
	// GetMsg 将消息转换为 RuleGo 的内部消息格式。
	// 此方法应处理数据类型检测和元数据填充。
	//
	// Returns / 返回：
	// • *types.RuleMsg: Converted message ready for rule chain processing  转换后的消息，可供规则链处理
	//
	// Conversion Behavior / 转换行为：
	// • Should detect appropriate data type (JSON, TEXT, BINARY)  应检测适当的数据类型
	// • Should populate metadata with relevant protocol information  应使用相关协议信息填充元数据
	// • Should cache the result for subsequent calls  应为后续调用缓存结果
	GetMsg() *types.RuleMsg

	// SetStatusCode sets the response status code for protocols that support it.
	// For protocols without status codes (e.g., MQTT), this may be a no-op.
	//
	// SetStatusCode 为支持它的协议设置响应状态码。
	// 对于没有状态码的协议（例如，MQTT），这可能是无操作。
	//
	// Parameters / 参数：
	// • statusCode: Protocol-specific status code  协议特定的状态码
	SetStatusCode(statusCode int)

	// SetBody sets or modifies the message body content.
	// This is typically used for response messages or message transformation.
	//
	// SetBody 设置或修改消息体内容。
	// 这通常用于响应消息或消息转换。
	//
	// Parameters / 参数：
	// • body: New message body content  新的消息体内容
	SetBody(body []byte)

	// SetError associates an error with this message.
	// This is used for error tracking and debugging purposes.
	//
	// SetError 将错误与此消息关联。
	// 这用于错误跟踪和调试目的。
	//
	// Parameters / 参数：
	// • err: Error to associate with the message  要与消息关联的错误
	SetError(err error)

	// GetError retrieves any error associated with this message.
	// This is useful for error handling and debugging.
	//
	// GetError 检索与此消息关联的任何错误。
	// 这对于错误处理和调试很有用。
	//
	// Returns / 返回：
	// • error: Associated error, nil if no error  关联的错误，如果没有错误则为 nil
	GetError() error
}

// Exchange represents a complete message exchange containing both request and response.
// It provides the context for processing a single message interaction through the endpoint system.
//
// Exchange 表示包含请求和响应的完整消息交换。
// 它为通过端点系统处理单个消息交互提供上下文。
//
// Architecture / 架构：
//
// Exchange follows the request-response pattern common in many protocols:
// Exchange 遵循许多协议中常见的请求-响应模式：
//
// 1. Request Processing: In message is processed through the pipeline  请求处理：In 消息通过管道处理
// 2. Business Logic: Rule chains or components execute business logic  业务逻辑：规则链或组件执行业务逻辑
// 3. Response Generation: Results are populated in the Out message  响应生成：结果填充到 Out 消息中
// 4. Protocol Handling: Protocol-specific response is sent back  协议处理：发送回协议特定的响应
//
// Thread Safety / 线程安全：
//
// Exchange includes RWMutex for protecting concurrent access to its fields.
// Exchange 包含 RWMutex 以保护对其字段的并发访问。
//
// Usage Pattern / 使用模式：
//
//	exchange := &endpoint.Exchange{
//	    In:  requestMessage,
//	    Out: responseMessage,
//	    Context: context.Background(),
//	}
//	// Process through pipeline
//	router.Execute(exchange)
type Exchange struct {
	// In represents the incoming request message.
	// This contains the original data received from the external source.
	// In 表示传入的请求消息。
	// 这包含从外部源接收的原始数据。
	In Message

	// Out represents the outgoing response message.
	// This will be populated with the processing results and sent back.
	// Out 表示传出的响应消息。
	// 这将填充处理结果并发送回去。
	Out Message

	// Context provides additional context for the exchange operation.
	// This can include timeouts, cancellation, and request-scoped values.
	// Context 为交换操作提供额外的上下文。
	// 这可以包括超时、取消和请求范围的值。
	Context context.Context

	// RWMutex protects concurrent access to Exchange fields.
	// This ensures thread-safe operations when multiple goroutines access the exchange.
	// RWMutex 保护对 Exchange 字段的并发访问。
	// 这确保多个协程访问交换时的线程安全操作。
	sync.RWMutex
}

// From defines the interface for message source configuration in routing operations.
// It represents the input side of a routing rule, defining where messages originate
// and how they should be processed before being sent to their destination.
//
// From 定义路由操作中消息源配置的接口。
// 它表示路由规则的输入端，定义消息的来源以及在发送到目标之前应如何处理。
//
// Key Features / 主要特性：
//
// • Source Definition: Specifies the origin pattern for incoming messages  源定义：指定传入消息的来源模式
// • Processing Pipeline: Supports transformation and processing functions  处理管道：支持转换和处理函数
// • Flexible Routing: Supports various destination types and configurations  灵活路由：支持各种目标类型和配置
// • Fluent API: Provides a fluent interface for configuration chaining  流畅 API：提供用于配置链接的流畅接口
//
// Processing Pipeline / 处理管道：
//
// The From interface supports a processing pipeline that can:
// From 接口支持一个处理管道，可以：
//
// 1. Transform message content and format  转换消息内容和格式
// 2. Validate message data and structure  验证消息数据和结构
// 3. Apply security and authentication checks  应用安全和身份验证检查
// 4. Route to appropriate destinations  路由到适当的目标
type From interface {
	// ToString returns a string representation of the source configuration.
	// This is typically the pattern or path used to match incoming messages.
	//
	// ToString 返回源配置的字符串表示。
	// 这通常是用于匹配传入消息的模式或路径。
	//
	// Returns / 返回：
	// • string: Source pattern or path  源模式或路径
	ToString() string

	// Transform adds a transformation process to the source processing pipeline.
	// Transformations modify message content, format, or structure.
	//
	// Transform 向源处理管道添加转换过程。
	// 转换修改消息内容、格式或结构。
	//
	// Parameters / 参数：
	// • transform: Processing function that modifies the message  修改消息的处理函数
	//
	// Returns / 返回：
	// • From: The same From instance for method chaining  用于方法链接的相同 From 实例
	//
	// Usage / 使用：
	//	from.Transform(func(router Router, exchange *Exchange) bool {
	//	    // Modify exchange.In or exchange.Out
	//	    return true // Continue processing
	//	})
	Transform(transform Process) From

	// Process adds a general processing function to the source pipeline.
	// Processors can perform validation, filtering, or other operations.
	//
	// Process 向源管道添加通用处理函数。
	// 处理器可以执行验证、过滤或其他操作。
	//
	// Parameters / 参数：
	// • process: Processing function to execute  要执行的处理函数
	//
	// Returns / 返回：
	// • From: The same From instance for method chaining  用于方法链接的相同 From 实例
	//
	// Processing Control / 处理控制：
	// If the process function returns false, subsequent processing is interrupted.
	// 如果处理函数返回 false，后续处理将被中断。
	Process(process Process) From

	// GetProcessList returns all processing functions configured for this source.
	// This is useful for introspection and debugging.
	//
	// GetProcessList 返回为此源配置的所有处理函数。
	// 这对于内省和调试很有用。
	//
	// Returns / 返回：
	// • []Process: List of configured processing functions  配置的处理函数列表
	GetProcessList() []Process

	// ExecuteProcess executes all configured processing functions in order.
	// This is called by the endpoint when a matching message is received.
	//
	// ExecuteProcess 按顺序执行所有配置的处理函数。
	// 当接收到匹配的消息时，端点会调用此方法。
	//
	// Parameters / 参数：
	// • router: The router containing this source configuration  包含此源配置的路由器
	// • exchange: The message exchange to process  要处理的消息交换
	//
	// Returns / 返回：
	// • bool: true to continue processing, false to interrupt  true 继续处理，false 中断
	//
	// Execution Flow / 执行流程：
	// If any processor returns false, execution stops and subsequent operations are interrupted.
	// 如果任何处理器返回 false，执行停止，后续操作被中断。
	ExecuteProcess(router Router, exchange *Exchange) bool

	// To defines the destination for messages from this source.
	// This creates a routing rule that connects the source to a destination.
	//
	// To 定义来自此源的消息的目标。
	// 这创建一个将源连接到目标的路由规则。
	//
	// Parameters / 参数：
	// • to: Destination path (e.g., "chain:chainId", "component:nodeType")  目标路径
	// • configs: Optional configuration for the destination  目标的可选配置
	//
	// Returns / 返回：
	// • To: Destination configuration interface  目标配置接口
	//
	// Destination Formats / 目标格式：
	// • "chainId" - Route to a specific rule chain (chain: prefix optional, default processor)  路由到特定规则链（chain: 前缀可选，默认处理器）
	// • "chain:chainId" - Route to a specific rule chain (explicit prefix)  路由到特定规则链（显式前缀）
	// • "chainId:nodeId" - Route to a specific node within a rule chain  路由到规则链中的特定节点
	// • "chain:chainId:nodeId" - Route to a specific node within a rule chain (explicit prefix)  路由到规则链中的特定节点（显式前缀）
	// • "component:nodeType" - Route to a component type  路由到组件类型
	// • Variable substitution is supported using ${variable} syntax  支持使用 ${variable} 语法的变量替换
	//
	// Examples / 示例：
	// • To("userProcessing") - Process through entire user processing chain (default)  通过整个用户处理链处理（默认）
	// • To("chain:userProcessing") - Process through entire user processing chain (explicit)  通过整个用户处理链处理（显式）
	// • To("userProcessing:validateInput") - Start from validateInput node  从 validateInput 节点开始
	// • To("chain:userProcessing:validateInput") - Start from validateInput node (explicit)  从 validateInput 节点开始（显式）
	// • To("component:jsTransform") - Execute JavaScript transform component  执行 JavaScript 转换组件
	To(to string, configs ...types.Configuration) To

	// GetTo retrieves the current destination configuration.
	// Returns nil if no destination has been configured.
	//
	// GetTo 检索当前目标配置。
	// 如果没有配置目标，则返回 nil。
	//
	// Returns / 返回：
	// • To: Current destination configuration, nil if not set  当前目标配置，如果未设置则为 nil
	GetTo() To

	// ToComponent sets the destination to be executed by a specific component instance.
	// This allows direct routing to a pre-configured component.
	//
	// ToComponent 设置目标由特定组件实例执行。
	// 这允许直接路由到预配置的组件。
	//
	// Parameters / 参数：
	// • node: Component instance to execute  要执行的组件实例
	//
	// Returns / 返回：
	// • To: Destination configuration interface  目标配置接口
	ToComponent(node types.Node) To

	// End finalizes the source configuration and returns the complete router.
	// This method completes the fluent configuration chain.
	//
	// End 完成源配置并返回完整的路由器。
	// 此方法完成流畅的配置链。
	//
	// Returns / 返回：
	// • Router: Complete router configuration  完整的路由器配置
	End() Router
}

// To defines the interface for message destination configuration in routing operations.
// It represents the output side of a routing rule, defining where processed messages
// are sent and how they should be handled during execution.
//
// To 定义路由操作中消息目标配置的接口。
// 它表示路由规则的输出端，定义处理后的消息发送到哪里以及在执行期间应如何处理。
//
// Key Features / 主要特性：
//
// • Destination Execution: Controls how and where messages are processed  目标执行：控制消息如何以及在哪里处理
// • Processing Pipeline: Supports post-processing and transformation  处理管道：支持后处理和转换
// • Execution Control: Supports synchronous and asynchronous execution modes  执行控制：支持同步和异步执行模式
// • Variable Substitution: Supports dynamic destination paths with variables  变量替换：支持带变量的动态目标路径
//
// Execution Modes / 执行模式：
//
// • Asynchronous (default): Messages are processed in background goroutines  异步（默认）：消息在后台协程中处理
// • Synchronous (with Wait()): Execution waits for completion before returning  同步（使用 Wait()）：执行等待完成后返回
//
// Destination Types / 目标类型：
//
// • Rule Chains: "chainId" or "chain:chainId" - Execute a rule chain (chain: prefix optional)  规则链：执行规则链（chain: 前缀可选）
// • Chain Nodes: "chainId:nodeId" or "chain:chainId:nodeId" - Execute from a specific node  链节点：从特定节点开始执行
// • Components: "component:nodeType" - Execute a component type  组件：执行组件类型
// • Direct Components: Pre-configured component instances  直接组件：预配置的组件实例
//
// Chain Node Routing Examples / 链节点路由示例：
// • "deviceProcessing" - Execute entire deviceProcessing chain (default)  执行整个 deviceProcessing 链（默认）
// • "chain:deviceProcessing" - Execute entire deviceProcessing chain (explicit)  执行整个 deviceProcessing 链（显式）
// • "deviceProcessing:filterNode" - Start from filterNode in deviceProcessing chain  从 deviceProcessing 链的 filterNode 开始
// • "chain:deviceProcessing:filterNode" - Start from filterNode (explicit prefix)  从 filterNode 开始（显式前缀）
// • "${tenant}:validateData" - Dynamic chain with specific node  动态链的特定节点
// • "alertChain:notificationNode" - Route to notification node in alert chain  路由到告警链的通知节点
type To interface {
	// ToString returns a string representation of the destination configuration.
	// This typically includes the destination path and any configured parameters.
	//
	// ToString 返回目标配置的字符串表示。
	// 这通常包括目标路径和任何配置的参数。
	//
	// Returns / 返回：
	// • string: Destination configuration string  目标配置字符串
	ToString() string

	// Execute performs the actual routing operation for the given exchange.
	// This is the core method that processes messages according to the destination configuration.
	//
	// Execute 对给定的交换执行实际的路由操作。
	// 这是根据目标配置处理消息的核心方法。
	//
	// Parameters / 参数：
	// • ctx: Context for the execution operation  执行操作的上下文
	// • exchange: Message exchange containing request and response  包含请求和响应的消息交换
	//
	// Execution Behavior / 执行行为：
	// • Synchronous execution if Wait() was called  如果调用了 Wait() 则同步执行
	// • Asynchronous execution by default  默认异步执行
	// • Processing pipeline is applied before destination execution  在目标执行前应用处理管道
	Execute(ctx context.Context, exchange *Exchange)

	// Transform adds a transformation process to the destination processing pipeline.
	// Transformations are applied after the destination execution completes.
	//
	// Transform 向目标处理管道添加转换过程。
	// 转换在目标执行完成后应用。
	//
	// Parameters / 参数：
	// • transform: Processing function that modifies the response  修改响应的处理函数
	//
	// Returns / 返回：
	// • To: The same To instance for method chaining  用于方法链接的相同 To 实例
	//
	// Usage / 使用：
	//	to.Transform(func(router Router, exchange *Exchange) bool {
	//	    // Modify exchange.Out after rule chain execution
	//	    return true
	//	})
	Transform(transform Process) To

	// Process adds a general processing function to the destination pipeline.
	// Processors can perform validation, logging, or other operations.
	//
	// Process 向目标管道添加通用处理函数。
	// 处理器可以执行验证、日志记录或其他操作。
	//
	// Parameters / 参数：
	// • process: Processing function to execute  要执行的处理函数
	//
	// Returns / 返回：
	// • To: The same To instance for method chaining  用于方法链接的相同 To 实例
	//
	// Processing Order / 处理顺序：
	// Processors are executed in the order they are added.
	// 处理器按添加顺序执行。
	Process(process Process) To

	// Wait configures the destination to use synchronous execution mode.
	// When Wait() is called, Execute() will block until processing is complete.
	//
	// Wait 配置目标使用同步执行模式。
	// 当调用 Wait() 时，Execute() 将阻塞直到处理完成。
	//
	// Returns / 返回：
	// • To: The same To instance for method chaining  用于方法链接的相同 To 实例
	//
	// Use Cases / 使用场景：
	// • Request-response patterns where response is needed immediately  需要立即响应的请求-响应模式
	// • Validation scenarios where errors must be caught  必须捕获错误的验证场景
	// • Testing and debugging scenarios  测试和调试场景
	Wait() To

	// IsWait checks if the destination is configured for synchronous execution.
	// This can be used to determine the execution mode before calling Execute().
	//
	// IsWait 检查目标是否配置为同步执行。
	// 这可用于在调用 Execute() 之前确定执行模式。
	//
	// Returns / 返回：
	// • bool: true if synchronous mode, false if asynchronous  true 表示同步模式，false 表示异步
	IsWait() bool

	// SetOpts applies rule context options that will be used during execution.
	// These options control various aspects of rule chain execution.
	//
	// SetOpts 应用在执行期间使用的规则上下文选项。
	// 这些选项控制规则链执行的各个方面。
	//
	// Parameters / 参数：
	// • opts: Rule context options to apply  要应用的规则上下文选项
	//
	// Returns / 返回：
	// • To: The same To instance for method chaining  用于方法链接的相同 To 实例
	//
	// Available Options / 可用选项：
	// • Timeout settings  超时设置
	// • Debug configurations  调试配置
	// • Custom metadata  自定义元数据
	SetOpts(opts ...types.RuleContextOption) To

	// GetOpts returns the currently configured rule context options.
	// This is useful for introspection and debugging.
	//
	// GetOpts 返回当前配置的规则上下文选项。
	// 这对于内省和调试很有用。
	//
	// Returns / 返回：
	// • []types.RuleContextOption: List of configured options  配置的选项列表
	GetOpts() []types.RuleContextOption

	// GetProcessList returns all processing functions configured for this destination.
	// This includes both Transform and Process functions in order.
	//
	// GetProcessList 返回为此目标配置的所有处理函数。
	// 这包括按顺序的 Transform 和 Process 函数。
	//
	// Returns / 返回：
	// • []Process: List of configured processing functions  配置的处理函数列表
	GetProcessList() []Process

	// ToStringByDict returns a string representation with variable substitution.
	// Variables in the destination path are replaced with values from the dictionary.
	//
	// ToStringByDict 返回带变量替换的字符串表示。
	// 目标路径中的变量被字典中的值替换。
	//
	// Parameters / 参数：
	// • dict: Dictionary for variable substitution  用于变量替换的字典
	//
	// Returns / 返回：
	// • string: Destination path with variables substituted  替换变量后的目标路径
	//
	// Variable Format / 变量格式：
	// Variables are specified using ${variableName} syntax.
	// 变量使用 ${variableName} 语法指定。
	ToStringByDict(dict map[string]string) string

	// End finalizes the destination configuration and returns the complete router.
	// This method completes the fluent configuration chain.
	//
	// End 完成目标配置并返回完整的路由器。
	// 此方法完成流畅的配置链。
	//
	// Returns / 返回：
	// • Router: Complete router configuration  完整的路由器配置
	End() Router
}

// Router defines the interface for complete routing configurations.
// A router combines source (From) and destination (To) configurations to create
// a complete message routing rule within an endpoint.
//
// Router 定义完整路由配置的接口。
// 路由器结合源（From）和目标（To）配置，在端点内创建完整的消息路由规则。
//
// Key Responsibilities / 主要职责：
//
// • Configuration Management: Maintain complete routing configuration  配置管理：维护完整的路由配置
// • Pattern Matching: Determine if incoming messages match the router  模式匹配：确定传入消息是否匹配路由器
// • Execution Context: Provide context for message processing  执行上下文：为消息处理提供上下文
// • State Management: Track router state and availability  状态管理：跟踪路由器状态和可用性
//
// Router Lifecycle / 路由器生命周期：
//
// 1. Creation: Router is created with source and destination configuration  创建：使用源和目标配置创建路由器
// 2. Registration: Router is registered with an endpoint  注册：路由器向端点注册
// 3. Activation: Router becomes active and begins processing messages  激活：路由器变为活动状态并开始处理消息
// 4. Processing: Messages are routed according to configuration  处理：根据配置路由消息
// 5. Deactivation: Router can be disabled or removed  停用：路由器可以被禁用或移除
//
// Error Handling / 错误处理：
//
// Routers track configuration and runtime errors through the Err() method.
// 路由器通过 Err() 方法跟踪配置和运行时错误。
type Router interface {
	// SetId sets the unique identifier for the router.
	// This ID is used for router management and reference within the endpoint.
	//
	// SetId 设置路由器的唯一标识符。
	// 此 ID 用于端点内的路由器管理和引用。
	//
	// Parameters / 参数：
	// • id: Unique identifier for the router  路由器的唯一标识符
	//
	// Returns / 返回：
	// • Router: The same Router instance for method chaining  用于方法链接的相同 Router 实例
	SetId(id string) Router

	// GetId retrieves the unique identifier of the router.
	// Returns an empty string if no ID has been set.
	//
	// GetId 检索路由器的唯一标识符。
	// 如果没有设置 ID，则返回空字符串。
	//
	// Returns / 返回：
	// • string: Router identifier, empty if not set  路由器标识符，如果未设置则为空
	GetId() string

	// FromToString returns a string representation of the source configuration.
	// This is typically the pattern or path used to match incoming messages.
	//
	// FromToString 返回源配置的字符串表示。
	// 这通常是用于匹配传入消息的模式或路径。
	//
	// Returns / 返回：
	// • string: Source configuration string  源配置字符串
	FromToString() string

	// From defines the source configuration for the routing operation.
	// This specifies where messages originate and how they should be matched.
	//
	// From 定义路由操作的源配置。
	// 这指定消息的来源以及应如何匹配。
	//
	// Parameters / 参数：
	// • from: Source pattern or path (e.g., "/api/*", "topic/+")  源模式或路径
	// • configs: Optional configuration for the source  源的可选配置
	//
	// Returns / 返回：
	// • From: Source configuration interface  源配置接口
	//
	// Pattern Formats / 模式格式：
	// • HTTP: URL patterns with wildcards "/api/*"  HTTP：带通配符的 URL 模式
	// • MQTT: Topic patterns with wildcards "device/+/data"  MQTT：带通配符的主题模式
	// • Custom: Protocol-specific patterns  自定义：协议特定模式
	From(from string, configs ...types.Configuration) From

	// GetFrom retrieves the current source configuration.
	// Returns nil if no source has been configured.
	//
	// GetFrom 检索当前源配置。
	// 如果没有配置源，则返回 nil。
	//
	// Returns / 返回：
	// • From: Current source configuration, nil if not set  当前源配置，如果未设置则为 nil
	GetFrom() From

	// GetRuleGo retrieves the rule engine pool associated with this router.
	// The pool is determined based on the exchange context and router configuration.
	//
	// GetRuleGo 检索与此路由器关联的规则引擎池。
	// 池根据交换上下文和路由器配置确定。
	//
	// Parameters / 参数：
	// • exchange: Message exchange providing context  提供上下文的消息交换
	//
	// Returns / 返回：
	// • types.RuleEnginePool: Rule engine pool for processing  用于处理的规则引擎池
	//
	// Pool Selection / 池选择：
	// • May return different pools based on exchange properties  可能根据交换属性返回不同的池
	// • Supports multi-tenant scenarios with pool isolation  支持池隔离的多租户场景
	GetRuleGo(exchange *Exchange) types.RuleEnginePool

	// GetContextFunc retrieves the context function for exchange processing.
	// This function can modify or enhance the context for each exchange.
	//
	// GetContextFunc 检索用于交换处理的上下文函数。
	// 此函数可以为每个交换修改或增强上下文。
	//
	// Returns / 返回：
	// • func: Context modification function, nil if not set  上下文修改函数，如果未设置则为 nil
	//
	// Context Enhancement / 上下文增强：
	// The function can add request-specific values, timeouts, or cancellation.
	// 函数可以添加请求特定的值、超时或取消。
	GetContextFunc() func(ctx context.Context, exchange *Exchange) context.Context

	// Disable sets the availability state of the router.
	// Disabled routers will not process incoming messages.
	//
	// Disable 设置路由器的可用状态。
	// 禁用的路由器不会处理传入消息。
	//
	// Parameters / 参数：
	// • disable: true to disable, false to enable  true 表示禁用，false 表示启用
	//
	// Returns / 返回：
	// • Router: The same Router instance for method chaining  用于方法链接的相同 Router 实例
	//
	// Use Cases / 使用场景：
	// • Maintenance mode  维护模式
	// • A/B testing scenarios  A/B 测试场景
	// • Gradual rollout of new routes  新路由的逐步推出
	Disable(disable bool) Router

	// IsDisable checks the availability state of the router.
	// This can be used to determine if the router will process messages.
	//
	// IsDisable 检查路由器的可用状态。
	// 这可用于确定路由器是否会处理消息。
	//
	// Returns / 返回：
	// • bool: true if disabled, false if enabled  true 表示禁用，false 表示启用
	IsDisable() bool

	// Definition returns the DSL definition of the router if available.
	// This provides access to the original configuration structure.
	//
	// Definition 返回路由器的 DSL 定义（如果可用）。
	// 这提供对原始配置结构的访问。
	//
	// Returns / 返回：
	// • *types.RouterDsl: Router DSL definition, nil if not set  路由器 DSL 定义，如果未设置则为 nil
	//
	// Usage / 使用：
	// Useful for configuration introspection, debugging, and serialization.
	// 对于配置内省、调试和序列化很有用。
	Definition() *types.RouterDsl

	// SetParams sets protocol-specific parameters for the router.
	// These parameters control protocol-specific behavior and matching.
	//
	// SetParams 为路由器设置协议特定参数。
	// 这些参数控制协议特定的行为和匹配。
	//
	// Parameters / 参数：
	// • args: Variable number of protocol-specific arguments  可变数量的协议特定参数
	//
	// Parameter Examples / 参数示例：
	// • HTTP: HTTP methods ["GET", "POST"]  HTTP：HTTP 方法
	// • MQTT: QoS levels, retain flags  MQTT：QoS 级别、保留标志
	// • WebSocket: Sub-protocols  WebSocket：子协议
	SetParams(args ...interface{})

	// GetParams retrieves the protocol-specific parameters.
	// Returns the parameters set by SetParams().
	//
	// GetParams 检索协议特定参数。
	// 返回由 SetParams() 设置的参数。
	//
	// Returns / 返回：
	// • []interface{}: List of protocol-specific parameters  协议特定参数列表
	GetParams() []interface{}

	// Err returns any error associated with the router configuration or operation.
	// This includes initialization errors, configuration errors, and runtime errors.
	//
	// Err 返回与路由器配置或操作关联的任何错误。
	// 这包括初始化错误、配置错误和运行时错误。
	//
	// Returns / 返回：
	// • error: Associated error, nil if no error  关联的错误，如果没有错误则为 nil
	//
	// Error Types / 错误类型：
	// • Configuration errors during router setup  路由器设置期间的配置错误
	// • Pattern compilation errors  模式编译错误
	// • Runtime processing errors  运行时处理错误
	Err() error
}

// Process defines a processing function type for the endpoint pipeline system.
// Process functions implement middleware-style processing that can transform,
// validate, filter, or otherwise modify messages during routing operations.
//
// Process 定义端点管道系统的处理函数类型。
// Process 函数实现中间件风格的处理，可以在路由操作期间转换、验证、过滤或以其他方式修改消息。
//
// Function Signature / 函数签名：
//
// The function receives a Router and Exchange and returns a boolean indicating
// whether processing should continue.
// 函数接收 Router 和 Exchange 并返回一个布尔值，指示处理是否应继续。
//
// Return Value Behavior / 返回值行为：
//
// • true: Continue processing to the next processor or destination  true：继续处理到下一个处理器或目标
// • false: Stop processing pipeline immediately  false：立即停止处理管道
//
// Common Use Cases / 常见用例：
//
// • Authentication and Authorization  身份验证和授权
// • Message Validation and Transformation  消息验证和转换
// • Logging and Monitoring  日志记录和监控
// • Rate Limiting and Throttling  速率限制和节流
// • Error Handling and Recovery  错误处理和恢复
//
// Processing Context / 处理上下文：
//
// • Router: Provides access to routing configuration and context  路由器：提供对路由配置和上下文的访问
// • Exchange: Contains request/response messages and processing context  交换：包含请求/响应消息和处理上下文
//
// Example Implementations / 示例实现：
//
//	// Authentication processor
//	func authProcessor(router Router, exchange *Exchange) bool {
//	    token := exchange.In.Headers().Get("Authorization")
//	    if !isValidToken(token) {
//	        exchange.Out.SetStatusCode(401)
//	        return false // Stop processing
//	    }
//	    return true // Continue processing
//	}
//
//	// Logging processor
//	func logProcessor(router Router, exchange *Exchange) bool {
//	    log.Printf("Processing request from %s", exchange.In.From())
//	    return true // Always continue
//	}
//
//	// Message transformation processor
//	func transformProcessor(router Router, exchange *Exchange) bool {
//	    body := exchange.In.Body()
//	    transformed := transform(body)
//	    exchange.In.SetBody(transformed)
//	    return true
//	}
type Process func(router Router, exchange *Exchange) bool

// OptionsSetter defines the interface for configuring routing components with various options.
// This interface is used by RouterOption functions to apply configuration settings
// to routers, endpoints, and other routing components.
//
// OptionsSetter 定义为路由组件配置各种选项的接口。
// 此接口由 RouterOption 函数用于将配置设置应用于路由器、端点和其他路由组件。
//
// Design Pattern / 设计模式：
//
// OptionsSetter follows the Options pattern for flexible component configuration:
// OptionsSetter 遵循选项模式以实现灵活的组件配置：
//
// 1. Centralized Configuration: All configuration options are applied through a single interface  集中配置：所有配置选项通过单一接口应用
// 2. Type Safety: Strongly typed configuration methods  类型安全：强类型配置方法
// 3. Extensibility: Easy to add new configuration options  可扩展性：易于添加新的配置选项
// 4. Consistency: Uniform configuration approach across components  一致性：跨组件的统一配置方法
//
// Usage Pattern / 使用模式：
//
//	func WithCustomOption(value string) RouterOption {
//	    return func(setter OptionsSetter) error {
//	        // Apply configuration
//	        setter.SetConfig(customConfig)
//	        return nil
//	    }
//	}
type OptionsSetter interface {
	// SetConfig sets the rule engine configuration for the component.
	// This configuration affects how the component interacts with the rule engine.
	//
	// SetConfig 为组件设置规则引擎配置。
	// 此配置影响组件如何与规则引擎交互。
	//
	// Parameters / 参数：
	// • config: Rule engine configuration to apply  要应用的规则引擎配置
	SetConfig(config types.Config)

	// SetRuleEnginePool sets a specific rule engine pool for the component.
	// This allows for custom pool selection and multi-tenancy support.
	//
	// SetRuleEnginePool 为组件设置特定的规则引擎池。
	// 这允许自定义池选择和多租户支持。
	//
	// Parameters / 参数：
	// • pool: Rule engine pool to use  要使用的规则引擎池
	SetRuleEnginePool(pool types.RuleEnginePool)

	// SetRuleEnginePoolFunc sets a function to dynamically determine the rule engine pool.
	// This enables advanced scenarios like load balancing and context-based pool selection.
	//
	// SetRuleEnginePoolFunc 设置动态确定规则引擎池的函数。
	// 这启用了负载均衡和基于上下文的池选择等高级场景。
	//
	// Parameters / 参数：
	// • f: Function that returns a rule engine pool based on the exchange  根据交换返回规则引擎池的函数
	//
	// Function Behavior / 函数行为：
	// The function is called for each message exchange to determine the appropriate pool.
	// 函数为每个消息交换调用以确定适当的池。
	SetRuleEnginePoolFunc(f func(exchange *Exchange) types.RuleEnginePool)

	// SetContextFunc sets a function to modify or enhance the context for each exchange.
	// This allows for request-specific context modification and enrichment.
	//
	// SetContextFunc 设置为每个交换修改或增强上下文的函数。
	// 这允许请求特定的上下文修改和丰富。
	//
	// Parameters / 参数：
	// • f: Function that modifies the context  修改上下文的函数
	//
	// Context Enhancement / 上下文增强：
	// • Add request-specific values  添加请求特定的值
	// • Set custom timeouts  设置自定义超时
	// • Apply security context  应用安全上下文
	// • Inject dependencies  注入依赖项
	SetContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context)

	// SetDefinition sets the DSL configuration for the component.
	// This provides access to the original DSL definition for introspection.
	//
	// SetDefinition 为组件设置 DSL 配置。
	// 这提供对原始 DSL 定义的访问以进行内省。
	//
	// Parameters / 参数：
	// • dsl: Router DSL definition  路由器 DSL 定义
	SetDefinition(dsl *types.RouterDsl)
}

// Executor defines the interface for executing the destination side of routing operations.
// Executors are responsible for processing messages according to specific destination types
// and configurations, providing a pluggable execution model for different target systems.
//
// Executor 定义执行路由操作目标端的接口。
// 执行器负责根据特定的目标类型和配置处理消息，为不同的目标系统提供可插拔的执行模型。
//
// Key Concepts / 关键概念：
//
// • Pluggable Execution: Different executors for different destination types  可插拔执行：不同目标类型的不同执行器
// • Configuration-Driven: Behavior controlled by configuration parameters  配置驱动：行为由配置参数控制
// • Variable Support: Dynamic path resolution with variable substitution  变量支持：带变量替换的动态路径解析
// • Stateless Design: Executors should be stateless and reusable  无状态设计：执行器应该是无状态和可重用的
//
// Executor Types / 执行器类型：
//
// • Chain Executor: Executes rule chains  链执行器：执行规则链
// • Component Executor: Executes individual components  组件执行器：执行单个组件
// • Script Executor: Executes script-based logic  脚本执行器：执行基于脚本的逻辑
// • Custom Executors: User-defined execution logic  自定义执行器：用户定义的执行逻辑
//
// Performance Considerations / 性能考虑：
//
// • Executors should be lightweight and fast to create  执行器应该是轻量级且快速创建的
// • Heavy initialization should be done in Init()  繁重的初始化应在 Init() 中完成
// • Execute() should be optimized for high throughput  Execute() 应针对高吞吐量优化
type Executor interface {
	// New creates a new instance of the executor.
	// This method should return a clean, initialized executor instance.
	//
	// New 创建执行器的新实例。
	// 此方法应返回一个干净的、已初始化的执行器实例。
	//
	// Returns / 返回：
	// • Executor: New executor instance  新的执行器实例
	//
	// Implementation Notes / 实现注意事项：
	// • Should be lightweight and fast  应该是轻量级且快速的
	// • Should not share state between instances  不应在实例之间共享状态
	// • Should copy any necessary configuration  应复制任何必要的配置
	New() Executor

	// IsPathSupportVar indicates whether the executor supports variable substitution in paths.
	// This determines if the destination path can contain dynamic variables.
	//
	// IsPathSupportVar 指示执行器是否支持路径中的变量替换。
	// 这确定目标路径是否可以包含动态变量。
	//
	// Returns / 返回：
	// • bool: true if variable substitution is supported  true 表示支持变量替换
	//
	// Variable Format / 变量格式：
	// Variables are typically specified using ${variableName} syntax.
	// 变量通常使用 ${variableName} 语法指定。
	IsPathSupportVar() bool

	// Init initializes the executor with configuration and destination-specific settings.
	// This method is called once during executor setup and should perform any heavy initialization.
	//
	// Init 使用配置和目标特定设置初始化执行器。
	// 此方法在执行器设置期间调用一次，应执行任何繁重的初始化。
	//
	// Parameters / 参数：
	// • config: Rule engine configuration  规则引擎配置
	// • configuration: Destination-specific configuration  目标特定配置
	//
	// Returns / 返回：
	// • error: Initialization error, nil on success  初始化错误，成功时为 nil
	//
	// Initialization Tasks / 初始化任务：
	// • Parse and validate configuration  解析和验证配置
	// • Establish connections if needed  如果需要建立连接
	// • Compile patterns or scripts  编译模式或脚本
	// • Allocate resources  分配资源
	Init(config types.Config, configuration types.Configuration) error

	// Execute performs the actual execution of the destination logic.
	// This is the core method that processes messages according to the executor's purpose.
	//
	// Execute 执行目标逻辑的实际执行。
	// 这是根据执行器目的处理消息的核心方法。
	//
	// Parameters / 参数：
	// • ctx: Context for the execution operation  执行操作的上下文
	// • router: Router providing execution context  提供执行上下文的路由器
	// • exchange: Message exchange to process  要处理的消息交换
	//
	// Execution Flow / 执行流程：
	// 1. Extract message and metadata from exchange  从交换中提取消息和元数据
	// 2. Apply any variable substitution if supported  如果支持则应用任何变量替换
	// 3. Execute the destination logic (rule chain, component, etc.)  执行目标逻辑（规则链、组件等）
	// 4. Populate response in exchange.Out if needed  如果需要，在 exchange.Out 中填充响应
	//
	// Error Handling / 错误处理：
	// Errors should be handled gracefully and may be set in the exchange.
	// 错误应优雅处理，可能在交换中设置。
	Execute(ctx context.Context, router Router, exchange *Exchange)
}

// Factory defines the interface for creating endpoint instances.
// Factories provide a centralized mechanism for creating and configuring endpoints
// from various sources including DSL configurations and type specifications.
//
// Factory 定义创建端点实例的接口。
// 工厂为从各种源（包括 DSL 配置和类型规范）创建和配置端点提供集中机制。
//
// Factory Pattern Benefits / 工厂模式优势：
//
// • Centralized Creation: Single point for endpoint instantiation  集中创建：端点实例化的单一点
// • Configuration Management: Consistent configuration application  配置管理：一致的配置应用
// • Type Safety: Strongly typed endpoint creation  类型安全：强类型端点创建
// • Extensibility: Easy to add new endpoint types  可扩展性：易于添加新的端点类型
//
// Supported Creation Methods / 支持的创建方法：
//
// • DSL-based: Create from JSON/YAML DSL definitions  基于 DSL：从 JSON/YAML DSL 定义创建
// • Type-based: Create from component type and configuration  基于类型：从组件类型和配置创建
// • Definition-based: Create from structured definitions  基于定义：从结构化定义创建
type Factory interface {
	// NewFromDsl creates a new DynamicEndpoint instance from a DSL byte array.
	// This method parses the DSL and creates a fully configured dynamic endpoint.
	//
	// NewFromDsl 从 DSL 字节数组创建新的 DynamicEndpoint 实例。
	// 此方法解析 DSL 并创建完全配置的动态端点。
	//
	// Parameters / 参数：
	// • dsl: JSON byte array containing endpoint configuration  包含端点配置的 JSON 字节数组
	// • opts: Additional options for endpoint creation  端点创建的附加选项
	//
	// Returns / 返回：
	// • DynamicEndpoint: Created dynamic endpoint instance  创建的动态端点实例
	// • error: Creation error, nil on success  创建错误，成功时为 nil
	//
	// DSL Format / DSL 格式：
	// The DSL should follow the types.EndpointDsl structure format.
	// DSL 应遵循 types.EndpointDsl 结构格式。
	NewFromDsl(dsl []byte, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// NewFromDef creates a new DynamicEndpoint instance from a structured definition.
	// This method accepts a pre-parsed endpoint definition structure.
	//
	// NewFromDef 从结构化定义创建新的 DynamicEndpoint 实例。
	// 此方法接受预解析的端点定义结构。
	//
	// Parameters / 参数：
	// • def: Structured endpoint DSL definition  结构化端点 DSL 定义
	// • opts: Additional options for endpoint creation  端点创建的附加选项
	//
	// Returns / 返回：
	// • DynamicEndpoint: Created dynamic endpoint instance  创建的动态端点实例
	// • error: Creation error, nil on success  创建错误，成功时为 nil
	//
	// Advantages / 优势：
	// • No JSON parsing overhead  无 JSON 解析开销
	// • Type safety at compile time  编译时类型安全
	// • Easier to programmatically generate  更容易以编程方式生成
	NewFromDef(def types.EndpointDsl, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// NewFromType creates a new Endpoint instance from a component type and configuration.
	// This method creates a static endpoint for a specific protocol or component type.
	//
	// NewFromType 从组件类型和配置创建新的 Endpoint 实例。
	// 此方法为特定协议或组件类型创建静态端点。
	//
	// Parameters / 参数：
	// • componentType: Type identifier for the endpoint (e.g., "rest", "mqtt")  端点的类型标识符
	// • ruleConfig: Rule engine configuration  规则引擎配置
	// • configuration: Component-specific configuration  组件特定配置
	//
	// Returns / 返回：
	// • Endpoint: Created endpoint instance  创建的端点实例
	// • error: Creation error, nil on success  创建错误，成功时为 nil
	//
	// Supported Types / 支持的类型：
	// • "rest" - REST/HTTP endpoints  REST/HTTP 端点
	// • "mqtt" - MQTT pub/sub endpoints  MQTT 发布/订阅端点
	// • "websocket" - WebSocket endpoints  WebSocket 端点
	// • Custom types registered with the component registry  注册到组件注册表的自定义类型
	NewFromType(componentType string, ruleConfig types.Config, configuration interface{}) (Endpoint, error)
}

// Pool defines the interface for managing a collection of dynamic endpoints.
// The pool provides centralized lifecycle management, including creation, retrieval,
// and cleanup of endpoint instances with support for hot reloading and configuration updates.
//
// Pool 定义管理动态端点集合的接口。
// 池提供集中的生命周期管理，包括端点实例的创建、检索和清理，支持热重载和配置更新。
//
// Pool Architecture / 池架构：
//
// • Centralized Management: Single point for endpoint lifecycle  集中管理：端点生命周期的单一点
// • Hot Reloading: Support for runtime configuration updates  热重载：支持运行时配置更新
// • Resource Cleanup: Automatic cleanup of unused endpoints  资源清理：自动清理未使用的端点
// • Thread Safety: Safe for concurrent access and operations  线程安全：并发访问和操作安全
//
// Use Cases / 使用场景：
//
// • Multi-tenant Applications: Separate endpoints per tenant  多租户应用程序：每个租户单独的端点
// • Dynamic API Gateways: Runtime endpoint configuration  动态 API 网关：运行时端点配置
// • Microservice Orchestration: Dynamic service endpoint management  微服务编排：动态服务端点管理
// • Development and Testing: Easy endpoint switching and testing  开发和测试：易于端点切换和测试
type Pool interface {
	// New creates a new dynamic endpoint with the specified ID and configuration.
	// The endpoint is automatically added to the pool for management.
	//
	// New 使用指定的 ID 和配置创建新的动态端点。
	// 端点自动添加到池中进行管理。
	//
	// Parameters / 参数：
	// • id: Unique identifier for the endpoint  端点的唯一标识符
	// • del: DSL configuration as byte array  作为字节数组的 DSL 配置
	// • opts: Additional options for endpoint creation  端点创建的附加选项
	//
	// Returns / 返回：
	// • DynamicEndpoint: Created dynamic endpoint instance  创建的动态端点实例
	// • error: Creation error, nil on success  创建错误，成功时为 nil
	//
	// Behavior / 行为：
	// • If an endpoint with the same ID exists, it may be replaced or return an error  如果存在相同 ID 的端点，可能被替换或返回错误
	// • The endpoint is automatically started if the configuration allows  如果配置允许，端点自动启动
	New(id string, del []byte, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// Get retrieves a dynamic endpoint by its unique identifier.
	// This method provides access to existing endpoints in the pool.
	//
	// Get 通过唯一标识符检索动态端点。
	// 此方法提供对池中现有端点的访问。
	//
	// Parameters / 参数：
	// • id: Unique identifier of the endpoint to retrieve  要检索的端点的唯一标识符
	//
	// Returns / 返回：
	// • DynamicEndpoint: Retrieved endpoint instance  检索到的端点实例
	// • bool: true if found, false if not found  true 表示找到，false 表示未找到
	Get(id string) (DynamicEndpoint, bool)

	// Del removes a dynamic endpoint from the pool by its ID.
	// This method performs cleanup and stops the endpoint before removal.
	//
	// Del 通过 ID 从池中删除动态端点。
	// 此方法在删除前执行清理并停止端点。
	//
	// Parameters / 参数：
	// • id: Unique identifier of the endpoint to remove  要删除的端点的唯一标识符
	//
	// Cleanup Behavior / 清理行为：
	// • Stops the endpoint gracefully  优雅地停止端点
	// • Releases any held resources  释放任何持有的资源
	// • Removes from internal storage  从内部存储中删除
	Del(id string)

	// Stop gracefully shuts down all dynamic endpoints in the pool.
	// This method should be called during application shutdown.
	//
	// Stop 优雅地关闭池中的所有动态端点。
	// 此方法应在应用程序关闭期间调用。
	//
	// Shutdown Process / 关闭过程：
	// 1. Stop accepting new requests  停止接受新请求
	// 2. Complete processing of in-flight requests  完成正在处理的请求
	// 3. Release resources and cleanup  释放资源并清理
	// 4. Clear the pool  清空池
	Stop()

	// Reload reloads all dynamic endpoints with new configuration options.
	// This allows for global configuration updates across all endpoints.
	//
	// Reload 使用新的配置选项重新加载所有动态端点。
	// 这允许对所有端点进行全局配置更新。
	//
	// Parameters / 参数：
	// • opts: New configuration options to apply  要应用的新配置选项
	//
	// Reload Behavior / 重载行为：
	// • Updates configuration for all endpoints  为所有端点更新配置
	// • May restart endpoints if required  如果需要可能重启端点
	// • Preserves endpoint state where possible  尽可能保持端点状态
	Reload(opts ...DynamicEndpointOption)

	// Range iterates over all dynamic endpoints in the pool.
	// This method provides a way to inspect and operate on all managed endpoints.
	//
	// Range 遍历池中的所有动态端点。
	// 此方法提供检查和操作所有托管端点的方式。
	//
	// Parameters / 参数：
	// • f: Function to call for each endpoint  为每个端点调用的函数
	//
	// Function Signature / 函数签名：
	// • key: Endpoint ID  端点 ID
	// • value: Endpoint instance  端点实例
	// • Return bool: true to continue iteration, false to stop  返回 bool：true 继续迭代，false 停止
	//
	// Usage / 使用：
	//	pool.Range(func(key, value any) bool {
	//	    id := key.(string)
	//	    endpoint := value.(DynamicEndpoint)
	//	    // Process endpoint
	//	    return true
	//	})
	Range(f func(key, value any) bool)

	// Factory returns the factory instance used to create endpoints.
	// This provides access to the underlying factory for direct endpoint creation.
	//
	// Factory 返回用于创建端点的工厂实例。
	// 这提供对底层工厂的访问以直接创建端点。
	//
	// Returns / 返回：
	// • Factory: Factory instance used by the pool  池使用的工厂实例
	Factory() Factory
}

// HeaderModifier defines the interface for modifying headers in endpoint messages.
// This interface provides a standardized way to manipulate message headers and metadata
// across different protocols and message types.
//
// HeaderModifier 定义修改端点消息中头部的接口。
// 此接口提供跨不同协议和消息类型操作消息头部和元数据的标准化方式。
//
// Key Features / 主要特性：
//
// • Protocol Abstraction: Unified header manipulation across protocols  协议抽象：跨协议的统一头部操作
// • Metadata Integration: Direct access to RuleGo metadata  元数据集成：直接访问 RuleGo 元数据
// • Standard Operations: Add, set, and delete header operations  标准操作：添加、设置和删除头部操作
// • Type Safety: Strongly typed header operations  类型安全：强类型头部操作
//
// Use Cases / 使用场景：
//
// • Response Header Management: Set response headers for HTTP  响应头管理：为 HTTP 设置响应头
// • Metadata Propagation: Forward metadata between systems  元数据传播：在系统间转发元数据
// • Protocol Translation: Convert headers between protocols  协议转换：在协议间转换头部
// • Security Headers: Add security-related headers  安全头：添加安全相关头部
type HeaderModifier interface {
	// AddHeader adds a header value to the message.
	// If the header already exists, the value is appended.
	//
	// AddHeader 向消息添加头部值。
	// 如果头部已存在，值将被追加。
	//
	// Parameters / 参数：
	// • key: Header name  头部名称
	// • value: Header value to add  要添加的头部值
	//
	// Behavior / 行为：
	// • Multiple values for the same key are supported  支持同一键的多个值
	// • Case-insensitive key handling (protocol dependent)  不区分大小写的键处理（取决于协议）
	AddHeader(key, value string)

	// SetHeader sets a header value, replacing any existing value.
	// This operation overwrites any existing values for the specified key.
	//
	// SetHeader 设置头部值，替换任何现有值。
	// 此操作覆盖指定键的任何现有值。
	//
	// Parameters / 参数：
	// • key: Header name  头部名称
	// • value: Header value to set  要设置的头部值
	//
	// Behavior / 行为：
	// • Overwrites existing values  覆盖现有值
	// • Creates the header if it doesn't exist  如果不存在则创建头部
	SetHeader(key, value string)

	// DelHeader removes a header from the message.
	// This operation removes all values associated with the specified key.
	//
	// DelHeader 从消息中删除头部。
	// 此操作删除与指定键关联的所有值。
	//
	// Parameters / 参数：
	// • key: Header name to remove  要删除的头部名称
	//
	// Behavior / 行为：
	// • Removes all values for the key  删除键的所有值
	// • No-op if the header doesn't exist  如果头部不存在则无操作
	DelHeader(key string)

	// GetMetadata returns the RuleGo metadata associated with the message.
	// This provides access to the internal metadata structure for advanced operations.
	//
	// GetMetadata 返回与消息关联的 RuleGo 元数据。
	// 这提供对内部元数据结构的访问以进行高级操作。
	//
	// Returns / 返回：
	// • *types.Metadata: Message metadata structure  消息元数据结构
	//
	// Metadata Usage / 元数据使用：
	// • Access to internal message properties  访问内部消息属性
	// • Cross-component data sharing  跨组件数据共享
	// • Protocol-specific information  协议特定信息
	GetMetadata() *types.Metadata
}

// HttpEndpoint defines the interface for HTTP-specific endpoint functionality.
// This interface extends the basic Endpoint interface with HTTP-specific methods
// for handling different HTTP methods and static file serving.
//
// HttpEndpoint 定义 HTTP 特定端点功能的接口。
// 此接口扩展基本的 Endpoint 接口，添加处理不同 HTTP 方法和静态文件服务的 HTTP 特定方法。
//
// Key Features / 主要特性：
//
// • HTTP Method Support: Dedicated methods for each HTTP verb  HTTP 方法支持：每个 HTTP 动词的专用方法
// • Static File Serving: Built-in static file hosting capabilities  静态文件服务：内置静态文件托管功能
// • Global Options: Support for global OPTIONS handler  全局选项：支持全局 OPTIONS 处理器
// • Fluent API: Chainable method calls for configuration  流畅 API：可链接的方法调用进行配置
//
// HTTP Method Mapping / HTTP 方法映射：
//
// Each HTTP method has a corresponding configuration method:
// 每个 HTTP 方法都有对应的配置方法：
//
// • GET: Retrieve resources  GET：检索资源
// • POST: Create new resources  POST：创建新资源
// • PUT: Update existing resources  PUT：更新现有资源
// • DELETE: Remove resources  DELETE：删除资源
// • PATCH: Partial updates  PATCH：部分更新
// • HEAD: Retrieve headers only  HEAD：仅检索头部
// • OPTIONS: CORS and capability discovery  OPTIONS：CORS 和功能发现
//
// Usage Pattern / 使用模式：
//
//	httpEndpoint.
//	    GET(router1, router2).
//	    POST(router3).
//	    PUT(router4).
//	    RegisterStaticFiles("/static/*").
//	    Start()
type HttpEndpoint interface {
	// Endpoint provides all basic endpoint functionality
	// Endpoint 提供所有基本端点功能
	Endpoint

	// GET configures routers for HTTP GET requests.
	// GET requests are typically used for retrieving resources without side effects.
	//
	// GET 为 HTTP GET 请求配置路由器。
	// GET 请求通常用于检索资源而不产生副作用。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	GET(routers ...Router) HttpEndpoint

	// HEAD configures routers for HTTP HEAD requests.
	// HEAD requests return the same headers as GET but without the response body.
	//
	// HEAD 为 HTTP HEAD 请求配置路由器。
	// HEAD 请求返回与 GET 相同的头部但没有响应体。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	HEAD(routers ...Router) HttpEndpoint

	// OPTIONS configures routers for HTTP OPTIONS requests.
	// OPTIONS requests are used for CORS preflight checks and capability discovery.
	//
	// OPTIONS 为 HTTP OPTIONS 请求配置路由器。
	// OPTIONS 请求用于 CORS 预检检查和功能发现。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	OPTIONS(routers ...Router) HttpEndpoint

	// POST configures routers for HTTP POST requests.
	// POST requests are typically used for creating new resources.
	//
	// POST 为 HTTP POST 请求配置路由器。
	// POST 请求通常用于创建新资源。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	POST(routers ...Router) HttpEndpoint

	// PUT configures routers for HTTP PUT requests.
	// PUT requests are typically used for updating or creating resources with known IDs.
	//
	// PUT 为 HTTP PUT 请求配置路由器。
	// PUT 请求通常用于更新或创建具有已知 ID 的资源。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	PUT(routers ...Router) HttpEndpoint

	// PATCH configures routers for HTTP PATCH requests.
	// PATCH requests are used for partial updates to existing resources.
	//
	// PATCH 为 HTTP PATCH 请求配置路由器。
	// PATCH 请求用于对现有资源进行部分更新。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	PATCH(routers ...Router) HttpEndpoint

	// DELETE configures routers for HTTP DELETE requests.
	// DELETE requests are used for removing resources.
	//
	// DELETE 为 HTTP DELETE 请求配置路由器。
	// DELETE 请求用于删除资源。
	//
	// Parameters / 参数：
	// • routers: Variable number of router configurations  可变数量的路由器配置
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	DELETE(routers ...Router) HttpEndpoint

	// GlobalOPTIONS sets a global handler for all OPTIONS requests.
	// This is typically used for CORS configuration and global OPTIONS responses.
	//
	// GlobalOPTIONS 为所有 OPTIONS 请求设置全局处理器。
	// 这通常用于 CORS 配置和全局 OPTIONS 响应。
	//
	// Parameters / 参数：
	// • handler: HTTP handler for OPTIONS requests  OPTIONS 请求的 HTTP 处理器
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	//
	// Use Cases / 使用场景：
	// • CORS preflight handling  CORS 预检处理
	// • API capability advertisement  API 功能广告
	// • Global OPTIONS responses  全局 OPTIONS 响应
	GlobalOPTIONS(handler http.Handler) HttpEndpoint

	// RegisterStaticFiles enables static file serving for the specified path pattern.
	// This allows the endpoint to serve static files like HTML, CSS, JS, and images.
	//
	// RegisterStaticFiles 为指定的路径模式启用静态文件服务。
	// 这允许端点服务静态文件，如 HTML、CSS、JS 和图像。
	//
	// Parameters / 参数：
	// • resourceMapping: Path pattern for static file mapping  静态文件映射的路径模式
	//
	// Returns / 返回：
	// • HttpEndpoint: The same endpoint instance for method chaining  用于方法链接的相同端点实例
	//
	// Resource Mapping Examples / 资源映射示例：
	// • "/ui/=/home/demo/dist" - Map /ui/ path to /home/demo/dist directory  将 /ui/ 路径映射到 /home/demo/dist 目录
	// • "/editor/=./editor,/images/=./editor/images" - Multiple mappings  多个映射
	// • "/static/=./public" - Map /static/ to local public directory  将 /static/ 映射到本地 public 目录
	//
	// File Serving Features / 文件服务功能：
	// • MIME type detection  MIME 类型检测
	// • Caching headers  缓存头部
	// • Range request support  范围请求支持
	// • Directory listing (configurable)  目录列表（可配置）
	RegisterStaticFiles(resourceMapping string) HttpEndpoint
}
