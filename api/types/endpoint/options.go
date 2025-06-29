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

package endpoint

import (
	"context"

	"github.com/rulego/rulego/api/types"
)

// RouterOption defines a function type for configuring router components using the Options pattern.
// This pattern provides a flexible and extensible way to configure routers with various settings
// while maintaining type safety and backward compatibility.
//
// RouterOption 定义使用选项模式配置路由器组件的函数类型。
// 此模式提供灵活且可扩展的方式来配置路由器的各种设置，
// 同时保持类型安全和向后兼容性。
//
// Design Benefits / 设计优势：
//
// • Type Safety: Compile-time validation of configuration options  类型安全：配置选项的编译时验证
// • Extensibility: Easy to add new options without breaking existing code  可扩展性：易于添加新选项而不破坏现有代码
// • Fluent Interface: Chainable configuration for better readability  流畅接口：可链接配置以提高可读性
// • Default Values: Options can provide sensible defaults  默认值：选项可以提供合理的默认值
//
// Usage Pattern / 使用模式：
//
//	options := []RouterOption{
//	    RouterOptions.WithRuleGo(ruleGo),
//	    RouterOptions.WithRuleConfig(config),
//	    RouterOptions.WithContextFunc(customContextFunc),
//	}
//	router.ApplyOptions(options...)
//
// Error Handling / 错误处理：
//
// RouterOption functions return an error to indicate configuration failures.
// This allows for graceful handling of invalid configurations.
// RouterOption 函数返回错误以指示配置失败。
// 这允许优雅地处理无效配置。
type RouterOption func(OptionsSetter) error

// RouterOptions provides a global instance of router configuration options.
// This singleton instance offers convenient methods for creating router options
// without requiring explicit instantiation.
//
// RouterOptions 提供路由器配置选项的全局实例。
// 此单例实例提供创建路由器选项的便利方法，
// 无需显式实例化。
//
// Usage / 使用：
//
// The RouterOptions variable provides access to all available router configuration methods:
// RouterOptions 变量提供对所有可用路由器配置方法的访问：
//
//	option1 := RouterOptions.WithRuleGo(ruleGo)
//	option2 := RouterOptions.WithRuleConfig(config)
//	option3 := RouterOptions.WithContextFunc(contextFunc)
//
// Thread Safety / 线程安全：
//
// The RouterOptions instance is stateless and thread-safe.
// RouterOptions 实例是无状态且线程安全的。
var RouterOptions = routerOptions{}

// routerOptions is the concrete implementation of router configuration options.
// It provides methods for creating RouterOption functions that configure various
// aspects of router behavior and integration.
//
// routerOptions 是路由器配置选项的具体实现。
// 它提供创建 RouterOption 函数的方法，这些函数配置路由器行为和集成的各个方面。
type routerOptions struct {
}

// WithRuleGoFunc creates a RouterOption that sets a function to dynamically determine the rule engine pool.
// This option enables advanced scenarios such as load balancing, multi-tenancy, and context-based pool selection.
//
// WithRuleGoFunc 创建一个 RouterOption，设置动态确定规则引擎池的函数。
// 此选项启用高级场景，如负载均衡、多租户和基于上下文的池选择。
//
// Parameters / 参数：
// • f: Function that returns a rule engine pool based on the exchange context  根据交换上下文返回规则引擎池的函数
//
// Returns / 返回：
// • RouterOption: Configuration function that applies the rule engine pool function  应用规则引擎池函数的配置函数
//
// Function Behavior / 函数行为：
//
// The provided function is called for each message exchange to determine which rule engine pool
// should process the message. This enables:
// 为每个消息交换调用提供的函数以确定哪个规则引擎池应处理消息。这启用：
//
// • Dynamic Pool Selection: Choose pools based on message content or metadata  动态池选择：基于消息内容或元数据选择池
// • Load Balancing: Distribute load across multiple pools  负载均衡：在多个池之间分配负载
// • Multi-tenancy: Route messages to tenant-specific pools  多租户：将消息路由到租户特定的池
// • Failover: Implement pool failover mechanisms  故障转移：实现池故障转移机制
//
// Example Usage / 示例使用：
//
//	option := RouterOptions.WithRuleGoFunc(func(exchange *Exchange) types.RuleEnginePool {
//	    tenantId := exchange.In.GetParam("tenantId")
//	    return getTenantPool(tenantId)
//	})
func (r routerOptions) WithRuleGoFunc(f func(exchange *Exchange) types.RuleEnginePool) RouterOption {
	return func(re OptionsSetter) error {
		re.SetRuleEnginePoolFunc(f)
		return nil
	}
}

// WithRuleGo creates a RouterOption that sets a specific rule engine pool for the router.
// This option configures the router to use a fixed rule engine pool for all message processing.
//
// WithRuleGo 创建一个 RouterOption，为路由器设置特定的规则引擎池。
// 此选项配置路由器使用固定的规则引擎池进行所有消息处理。
//
// Parameters / 参数：
// • ruleGo: Rule engine pool instance to use for message processing  用于消息处理的规则引擎池实例
//
// Returns / 返回：
// • RouterOption: Configuration function that applies the rule engine pool  应用规则引擎池的配置函数
//
// Use Cases / 使用场景：
//
// • Static Configuration: Use a predefined pool for all messages  静态配置：对所有消息使用预定义的池
// • Single-tenant Applications: Use a single shared pool  单租户应用程序：使用单个共享池
// • Development and Testing: Use a controlled pool environment  开发和测试：使用受控的池环境
//
// Default Behavior / 默认行为：
//
// If no rule engine pool is specified, the router will use `rulego.DefaultPool`.
// 如果未指定规则引擎池，路由器将使用 `rulego.DefaultPool`。
//
// Example Usage / 示例使用：
//
//	pool := rulego.NewPool("custom-pool")
//	option := RouterOptions.WithRuleGo(pool)
func (r routerOptions) WithRuleGo(ruleGo types.RuleEnginePool) RouterOption {
	return func(re OptionsSetter) error {
		re.SetRuleEnginePool(ruleGo)
		return nil
	}
}

// WithRuleConfig creates a RouterOption that sets the rule engine configuration for the router.
// This configuration affects how the router interacts with the rule engine and processes messages.
//
// WithRuleConfig 创建一个 RouterOption，为路由器设置规则引擎配置。
// 此配置影响路由器如何与规则引擎交互并处理消息。
//
// Parameters / 参数：
// • config: Rule engine configuration containing various settings  包含各种设置的规则引擎配置
//
// Returns / 返回：
// • RouterOption: Configuration function that applies the rule engine configuration  应用规则引擎配置的配置函数
//
// Configuration Aspects / 配置方面：
//
// The rule engine configuration can control:
// 规则引擎配置可以控制：
//
// • Script Execution: Timeout settings and script engine configuration  脚本执行：超时设置和脚本引擎配置
// • Component Registry: Available components and their registration  组件注册表：可用组件及其注册
// • Logging and Debugging: Debug modes and logging configurations  日志和调试：调试模式和日志配置
// • Worker Pool: Concurrent processing settings  工作池：并发处理设置
// • Global Properties: Shared configuration values  全局属性：共享配置值
//
// Example Usage / 示例使用：
//
//	config := types.NewConfig(
//	    types.WithLogger(customLogger),
//	    types.WithPool(workerPool),
//	)
//	option := RouterOptions.WithRuleConfig(config)
func (r routerOptions) WithRuleConfig(config types.Config) RouterOption {
	return func(re OptionsSetter) error {
		re.SetConfig(config)
		return nil
	}
}

// WithContextFunc creates a RouterOption that sets a context modification function for routing operations.
// This function can enhance or modify the context for each message exchange, enabling request-specific
// configurations and cross-cutting concerns.
//
// WithContextFunc 创建一个 RouterOption，为路由操作设置上下文修改函数。
// 此函数可以为每个消息交换增强或修改上下文，启用请求特定的配置和横切关注点。
//
// Parameters / 参数：
// • f: Function that takes the current context and exchange and returns a modified context  接受当前上下文和交换并返回修改后上下文的函数
//
// Returns / 返回：
// • RouterOption: Configuration function that applies the context modification function  应用上下文修改函数的配置函数
//
// Context Enhancement Capabilities / 上下文增强功能：
//
// The context function can:
// 上下文函数可以：
//
// • Add Request-specific Values: Inject request metadata, user information, etc.  添加请求特定值：注入请求元数据、用户信息等
// • Set Custom Timeouts: Apply different timeouts based on request type  设置自定义超时：根据请求类型应用不同的超时
// • Apply Security Context: Add authentication and authorization data  应用安全上下文：添加身份验证和授权数据
// • Enable Tracing: Add distributed tracing information  启用跟踪：添加分布式跟踪信息
// • Inject Dependencies: Provide access to external services or resources  注入依赖项：提供对外部服务或资源的访问
//
// Function Execution / 函数执行：
//
// The context function is called for each message exchange before rule chain execution.
// 在规则链执行之前，为每个消息交换调用上下文函数。
//
// Example Usage / 示例使用：
//
//	option := RouterOptions.WithContextFunc(func(ctx context.Context, exchange *Exchange) context.Context {
//	    // Add request ID for tracing
//	    requestId := exchange.In.Headers().Get("X-Request-ID")
//	    if requestId != "" {
//	        ctx = context.WithValue(ctx, "requestId", requestId)
//	    }
//
//	    // Set custom timeout
//	    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
//
//	    return ctx
//	})
func (r routerOptions) WithContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context) RouterOption {
	return func(re OptionsSetter) error {
		re.SetContextFunc(f)
		return nil
	}
}

// WithDefinition creates a RouterOption that sets the DSL definition for the router.
// This option provides access to the original DSL configuration for introspection,
// debugging, and advanced configuration scenarios.
//
// WithDefinition 创建一个 RouterOption，为路由器设置 DSL 定义。
// 此选项提供对原始 DSL 配置的访问，用于内省、调试和高级配置场景。
//
// Parameters / 参数：
// • def: Router DSL definition containing the original configuration structure  包含原始配置结构的路由器 DSL 定义
//
// Returns / 返回：
// • RouterOption: Configuration function that applies the DSL definition  应用 DSL 定义的配置函数
//
// DSL Definition Benefits / DSL 定义优势：
//
// • Configuration Introspection: Access to the complete original configuration  配置内省：访问完整的原始配置
// • Debugging Support: Better error messages and debugging information  调试支持：更好的错误消息和调试信息
// • Dynamic Reconfiguration: Support for runtime configuration updates  动态重新配置：支持运行时配置更新
// • Serialization: Ability to serialize and persist configuration  序列化：序列化和持久化配置的能力
//
// Use Cases / 使用场景：
//
// • Configuration Management: Store and manage router configurations  配置管理：存储和管理路由器配置
// • Hot Reloading: Update router configuration without restart  热重载：无需重启即可更新路由器配置
// • Audit and Compliance: Track configuration changes and compliance  审计和合规：跟踪配置更改和合规性
// • Template Processing: Support for configuration templates  模板处理：支持配置模板
//
// Example Usage / 示例使用：
//
//	dslDef := &types.RouterDsl{
//	    Id: "api-router",
//	    From: types.FromDsl{Path: "/api/*"},
//	    To: types.ToDsl{Path: "chain:api-handler"},
//	}
//	option := RouterOptions.WithDefinition(dslDef)
func (r routerOptions) WithDefinition(def *types.RouterDsl) RouterOption {
	return func(re OptionsSetter) error {
		re.SetDefinition(def)
		return nil
	}
}

// DynamicEndpointOption defines a function type for configuring dynamic endpoint instances using the Options pattern.
// This pattern provides a flexible and type-safe way to configure dynamic endpoints with various settings
// while maintaining backward compatibility and extensibility.
//
// DynamicEndpointOption 定义使用选项模式配置动态端点实例的函数类型。
// 此模式提供灵活且类型安全的方式来配置动态端点的各种设置，
// 同时保持向后兼容性和可扩展性。
//
// Key Features / 主要特性：
//
// • Runtime Configuration: Modify endpoint behavior at runtime  运行时配置：在运行时修改端点行为
// • Hot Reloading: Support for configuration updates without restart  热重载：支持无需重启的配置更新
// • Type Safety: Compile-time validation of configuration options  类型安全：配置选项的编译时验证
// • Composable Options: Combine multiple options for complex configurations  可组合选项：组合多个选项进行复杂配置
//
// Configuration Scope / 配置范围：
//
// DynamicEndpointOption can configure:
// DynamicEndpointOption 可以配置：
//
// • Endpoint Identity: ID and naming configuration  端点标识：ID 和命名配置
// • Rule Engine Integration: Rule engine pools and configurations  规则引擎集成：规则引擎池和配置
// • Router Behavior: Default router options and settings  路由器行为：默认路由器选项和设置
// • Event Handling: Event listeners and callbacks  事件处理：事件监听器和回调
// • Lifecycle Management: Restart policies and resource management  生命周期管理：重启策略和资源管理
//
// Usage Pattern / 使用模式：
//
//	options := []DynamicEndpointOption{
//	    DynamicEndpointOptions.WithId("custom-endpoint"),
//	    DynamicEndpointOptions.WithConfig(config),
//	    DynamicEndpointOptions.WithRestart(true),
//	}
//	endpoint.Reload(dslBytes, options...)
type DynamicEndpointOption func(DynamicEndpoint) error

// DynamicEndpointOptions provides a global instance of dynamic endpoint configuration options.
// This singleton instance offers convenient methods for creating dynamic endpoint options
// without requiring explicit instantiation.
//
// DynamicEndpointOptions 提供动态端点配置选项的全局实例。
// 此单例实例提供创建动态端点选项的便利方法，
// 无需显式实例化。
//
// Global Access / 全局访问：
//
// The DynamicEndpointOptions variable provides centralized access to all configuration methods:
// DynamicEndpointOptions 变量提供对所有配置方法的集中访问：
//
//	option1 := DynamicEndpointOptions.WithId("endpoint-1")
//	option2 := DynamicEndpointOptions.WithConfig(config)
//	option3 := DynamicEndpointOptions.WithRestart(true)
//
// Thread Safety / 线程安全：
//
// The DynamicEndpointOptions instance is stateless and thread-safe.
// DynamicEndpointOptions 实例是无状态且线程安全的。
var DynamicEndpointOptions = dynamicEndpointOptions{}

// dynamicEndpointOptions is the concrete implementation of dynamic endpoint configuration options.
// It provides methods for creating DynamicEndpointOption functions that configure various
// aspects of dynamic endpoint behavior and lifecycle.
//
// dynamicEndpointOptions 是动态端点配置选项的具体实现。
// 它提供创建 DynamicEndpointOption 函数的方法，这些函数配置动态端点行为和生命周期的各个方面。
type dynamicEndpointOptions struct {
}

// WithId creates a DynamicEndpointOption that sets the unique identifier for the dynamic endpoint.
// This option is essential for endpoint identification, management, and reference within the system.
//
// WithId 创建一个 DynamicEndpointOption，设置动态端点的唯一标识符。
// 此选项对于系统内的端点识别、管理和引用至关重要。
//
// Parameters / 参数：
// • id: Unique identifier string for the endpoint  端点的唯一标识符字符串
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the endpoint ID  应用端点 ID 的配置函数
//
// ID Requirements / ID 要求：
//
// • Uniqueness: Must be unique within the endpoint pool or system  唯一性：在端点池或系统内必须唯一
// • Persistence: Should remain stable across reloads and restarts  持久性：在重新加载和重启过程中应保持稳定
// • Readability: Should be human-readable for debugging and management  可读性：应易于人类阅读以便调试和管理
//
// Use Cases / 使用场景：
//
// • Endpoint Lookup: Find specific endpoints in pools or registries  端点查找：在池或注册表中查找特定端点
// • Configuration Management: Associate configurations with specific endpoints  配置管理：将配置与特定端点关联
// • Monitoring and Logging: Track endpoint-specific metrics and logs  监控和日志记录：跟踪端点特定的指标和日志
// • Hot Reloading: Update specific endpoints without affecting others  热重载：更新特定端点而不影响其他端点
//
// Example Usage / 示例使用：
//
//	option := DynamicEndpointOptions.WithId("user-api-endpoint")
func (d dynamicEndpointOptions) WithId(id string) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetId(id)
		return nil
	}
}

// WithConfig creates a DynamicEndpointOption that sets the rule engine configuration for the dynamic endpoint.
// This configuration affects how the endpoint interacts with rule engines and processes messages.
//
// WithConfig 创建一个 DynamicEndpointOption，为动态端点设置规则引擎配置。
// 此配置影响端点如何与规则引擎交互并处理消息。
//
// Parameters / 参数：
// • config: Rule engine configuration containing various settings and options  包含各种设置和选项的规则引擎配置
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the rule engine configuration  应用规则引擎配置的配置函数
//
// Configuration Impact / 配置影响：
//
// The rule engine configuration affects:
// 规则引擎配置影响：
//
// • Message Processing: How messages are parsed, validated, and transformed  消息处理：如何解析、验证和转换消息
// • Component Behavior: Available components and their configurations  组件行为：可用组件及其配置
// • Performance Settings: Worker pools, timeouts, and resource limits  性能设置：工作池、超时和资源限制
// • Debugging and Logging: Debug modes and logging configurations  调试和日志记录：调试模式和日志配置
// • Security Settings: Authentication, authorization, and encryption  安全设置：身份验证、授权和加密
//
// Configuration Inheritance / 配置继承：
//
// Endpoint-specific configurations override global defaults while preserving unspecified settings.
// 端点特定配置覆盖全局默认值，同时保留未指定的设置。
//
// Example Usage / 示例使用：
//
//	config := types.NewConfig(
//	    types.WithLogger(endpointLogger),
//	    types.WithPool(customPool),
//	    types.WithDebug(true),
//	)
//	option := DynamicEndpointOptions.WithConfig(config)
func (d dynamicEndpointOptions) WithConfig(config types.Config) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetConfig(config)
		return nil
	}
}

// WithRouterOpts creates a DynamicEndpointOption that sets default router options for the dynamic endpoint.
// These options are applied to all routers created within the endpoint, providing consistent configuration.
//
// WithRouterOpts 创建一个 DynamicEndpointOption，为动态端点设置默认路由器选项。
// 这些选项应用于端点内创建的所有路由器，提供一致的配置。
//
// Parameters / 参数：
// • opts: Variable number of router options to apply as defaults  作为默认值应用的可变数量路由器选项
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the default router options  应用默认路由器选项的配置函数
//
// Default Option Behavior / 默认选项行为：
//
// • Global Application: Applied to all routers within the endpoint  全局应用：应用于端点内的所有路由器
// • Override Support: Individual routers can override these defaults  覆盖支持：单个路由器可以覆盖这些默认值
// • Consistency: Ensures consistent behavior across all endpoint routers  一致性：确保所有端点路由器的一致行为
//
// Common Default Options / 常见默认选项：
//
// • Rule Engine Configuration: Default rule engine pools and configurations  规则引擎配置：默认规则引擎池和配置
// • Context Functions: Default context enhancement functions  上下文函数：默认上下文增强函数
// • Timeout Settings: Default timeout and retry configurations  超时设置：默认超时和重试配置
// • Security Policies: Default authentication and authorization settings  安全策略：默认身份验证和授权设置
//
// Example Usage / 示例使用：
//
//	defaultOpts := []RouterOption{
//	    RouterOptions.WithRuleGo(defaultPool),
//	    RouterOptions.WithContextFunc(defaultContextFunc),
//	}
//	option := DynamicEndpointOptions.WithRouterOpts(defaultOpts...)
func (d dynamicEndpointOptions) WithRouterOpts(opts ...RouterOption) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetRouterOptions(opts...)
		return nil
	}
}

// WithOnEvent creates a DynamicEndpointOption that sets the event handler for the dynamic endpoint.
// This handler receives notifications about endpoint lifecycle events and operational status changes.
//
// WithOnEvent 创建一个 DynamicEndpointOption，为动态端点设置事件处理器。
// 此处理器接收有关端点生命周期事件和操作状态更改的通知。
//
// Parameters / 参数：
// • onEvent: Event handler function that processes endpoint events  处理端点事件的事件处理器函数
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the event handler  应用事件处理器的配置函数
//
// Event Types / 事件类型：
//
// The event handler can receive various types of events:
// 事件处理器可以接收各种类型的事件：
//
// • Lifecycle Events: Start, stop, reload, and destruction events  生命周期事件：启动、停止、重新加载和销毁事件
// • Connection Events: Client connections and disconnections  连接事件：客户端连接和断开连接
// • Error Events: Configuration errors and runtime failures  错误事件：配置错误和运行时故障
// • Performance Events: Throughput metrics and performance indicators  性能事件：吞吐量指标和性能指标
//
// Event Handler Use Cases / 事件处理器使用场景：
//
// • Monitoring and Alerting: Track endpoint health and performance  监控和警报：跟踪端点健康状况和性能
// • Logging and Auditing: Record endpoint activities and changes  日志记录和审计：记录端点活动和更改
// • Auto-scaling: Trigger scaling based on load and performance  自动扩缩：基于负载和性能触发扩缩
// • Integration: Notify external systems about endpoint status  集成：通知外部系统端点状态
//
// Example Usage / 示例使用：
//
//	eventHandler := func(eventName string, params ...interface{}) {
//	    switch eventName {
//	    case endpoint.EventConnect:
//	        log.Printf("Client connected: %v", params)
//	    case endpoint.EventDisconnect:
//	        log.Printf("Client disconnected: %v", params)
//	    }
//	}
//	option := DynamicEndpointOptions.WithOnEvent(eventHandler)
func (d dynamicEndpointOptions) WithOnEvent(onEvent OnEvent) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetOnEvent(onEvent)
		return nil
	}
}

// WithRestart creates a DynamicEndpointOption that sets the restart behavior for the dynamic endpoint.
// This option controls whether configuration changes trigger a full endpoint restart or just update routing.
//
// WithRestart 创建一个 DynamicEndpointOption，设置动态端点的重启行为。
// 此选项控制配置更改是否触发完整的端点重启或仅更新路由。
//
// Parameters / 参数：
// • restart: Boolean flag indicating whether to restart on configuration changes  指示配置更改时是否重启的布尔标志
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the restart behavior  应用重启行为的配置函数
//
// Restart Behavior / 重启行为：
//
// • restart = true: Full endpoint restart with complete reinitialization  restart = true：完整的端点重启和完全重新初始化
//   - Closes all existing connections  关闭所有现有连接
//   - Releases all resources  释放所有资源
//   - Recreates the endpoint with new configuration  使用新配置重新创建端点
//   - Establishes new connections and resources  建立新连接和资源
//
// • restart = false: Hot reload with minimal disruption  restart = false：最小干扰的热重载
//   - Updates routing rules without stopping the endpoint  更新路由规则而不停止端点
//   - Preserves existing connections where possible  尽可能保持现有连接
//   - Applies configuration changes gradually  逐步应用配置更改
//   - May force restart if routing conflicts occur  如果发生路由冲突可能强制重启
//
// Decision Factors / 决策因素：
//
// Choose restart = true for:
// 选择 restart = true 适用于：
// • Major configuration changes (port, protocol, authentication)  重大配置更改（端口、协议、身份验证）
// • Resource allocation changes  资源分配更改
// • Breaking changes in routing structure  路由结构的破坏性更改
//
// Choose restart = false for:
// 选择 restart = false 适用于：
// • Minor routing updates  次要路由更新
// • Rule chain modifications  规则链修改
// • Non-breaking configuration adjustments  非破坏性配置调整
//
// Example Usage / 示例使用：
//
//	// For production environments - minimize disruption
//	option1 := DynamicEndpointOptions.WithRestart(false)
//
//	// For development environments - ensure clean state
//	option2 := DynamicEndpointOptions.WithRestart(true)
func (d dynamicEndpointOptions) WithRestart(restart bool) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetRestart(restart)
		return nil
	}
}

// WithInterceptors creates a DynamicEndpointOption that sets global interceptors for the dynamic endpoint.
// These interceptors are applied to all incoming messages, providing cross-cutting functionality
// such as authentication, logging, and message transformation.
//
// WithInterceptors 创建一个 DynamicEndpointOption，为动态端点设置全局拦截器。
// 这些拦截器应用于所有传入消息，提供横切功能，
// 如身份验证、日志记录和消息转换。
//
// Parameters / 参数：
// • interceptors: Variable number of processing functions to use as global interceptors  作为全局拦截器使用的可变数量处理函数
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that applies the interceptors  应用拦截器的配置函数
//
// Interceptor Processing / 拦截器处理：
//
// • Execution Order: Interceptors are executed in the order they are provided  执行顺序：拦截器按提供的顺序执行
// • Pipeline Behavior: If any interceptor returns false, processing stops  管道行为：如果任何拦截器返回 false，处理停止
// • Global Scope: Applied to all routers and messages within the endpoint  全局范围：应用于端点内的所有路由器和消息
//
// Common Interceptor Types / 常见拦截器类型：
//
// • Authentication: Verify user credentials and tokens  身份验证：验证用户凭据和令牌
// • Authorization: Check permissions and access rights  授权：检查权限和访问权限
// • Logging: Record request details and processing information  日志记录：记录请求详细信息和处理信息
// • Rate Limiting: Control request frequency and throttling  速率限制：控制请求频率和节流
// • Validation: Validate message format and content  验证：验证消息格式和内容
// • Transformation: Modify message content or headers  转换：修改消息内容或头部
// • Monitoring: Collect metrics and performance data  监控：收集指标和性能数据
//
// Implementation Guidelines / 实现指南：
//
// • Performance: Keep interceptors lightweight and fast  性能：保持拦截器轻量级和快速
// • Error Handling: Handle errors gracefully and provide meaningful messages  错误处理：优雅地处理错误并提供有意义的消息
// • State Management: Avoid shared state between interceptor invocations  状态管理：避免拦截器调用之间的共享状态
// • Idempotency: Ensure interceptors can be safely executed multiple times  幂等性：确保拦截器可以安全地多次执行
//
// Example Usage / 示例使用：
//
//	authInterceptor := func(router Router, exchange *Exchange) bool {
//	    token := exchange.In.Headers().Get("Authorization")
//	    return validateToken(token)
//	}
//
//	logInterceptor := func(router Router, exchange *Exchange) bool {
//	    log.Printf("Processing request from %s", exchange.In.From())
//	    return true
//	}
//
//	option := DynamicEndpointOptions.WithInterceptors(authInterceptor, logInterceptor)
func (d dynamicEndpointOptions) WithInterceptors(interceptors ...Process) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetInterceptors(interceptors...)
		return nil
	}
}

// WithRuleChain creates a DynamicEndpointOption that sets the original rule chain DSL definition.
// This option is used when the endpoint is initialized from a rule chain configuration,
// preserving the original DSL for reference and potential restoration.
//
// WithRuleChain 创建一个 DynamicEndpointOption，设置原始规则链 DSL 定义。
// 此选项在端点从规则链配置初始化时使用，
// 保留原始 DSL 以供参考和潜在恢复。
//
// Parameters / 参数：
// • ruleChain: Original rule chain DSL definition that created this endpoint  创建此端点的原始规则链 DSL 定义
//
// Returns / 返回：
// • DynamicEndpointOption: Configuration function that stores the rule chain definition  存储规则链定义的配置函数
//
// Purpose and Benefits / 目的和优势：
//
// • Historical Reference: Maintain link to the original configuration source  历史参考：维护与原始配置源的链接
// • Configuration Restoration: Enable restoration to original configuration  配置恢复：启用恢复到原始配置
// • Audit Trail: Track the origin and evolution of endpoint configuration  审计跟踪：跟踪端点配置的来源和演变
// • Template Processing: Support for configuration templates and inheritance  模板处理：支持配置模板和继承
//
// Use Cases / 使用场景：
//
// • Rule Chain Integration: When endpoints are created from rule chain DSL  规则链集成：当端点从规则链 DSL 创建时
// • Configuration Management: Track configuration sources and dependencies  配置管理：跟踪配置源和依赖关系
// • Rollback Support: Enable rollback to previous configurations  回滚支持：启用回滚到以前的配置
// • Documentation: Provide context for endpoint configuration decisions  文档：为端点配置决策提供上下文
//
// Storage and Retrieval / 存储和检索：
//
// The stored rule chain can be retrieved using endpoint.GetRuleChain() for:
// 可以使用 endpoint.GetRuleChain() 检索存储的规则链用于：
// • Configuration introspection  配置内省
// • Template processing  模板处理
// • Debugging and troubleshooting  调试和故障排除
//
// Example Usage / 示例使用：
//
//	originalRuleChain := &types.RuleChain{
//	    RuleChain: types.RuleChainBaseInfo{
//	        ID: "main-chain",
//	        Name: "Main Processing Chain",
//	    },
//	    Metadata: types.RuleMetadata{
//	        Endpoints: []*types.EndpointDsl{endpointDsl},
//	    },
//	}
//	option := DynamicEndpointOptions.WithRuleChain(originalRuleChain)
func (d dynamicEndpointOptions) WithRuleChain(ruleChain *types.RuleChain) DynamicEndpointOption {
	return func(re DynamicEndpoint) error {
		re.SetRuleChain(ruleChain)
		return nil
	}
}
