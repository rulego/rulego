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

// Package types defines the core types and interfaces for the RuleGo aspect-oriented programming (AOP) system.
// This package provides comprehensive AOP support for rule engines, enabling cross-cutting concerns
// to be separated from business logic.
//
// types 包定义了 RuleGo 面向切面编程（AOP）系统的核心类型和接口。
// 该包为规则引擎提供全面的 AOP 支持，使横切关注点能够与业务逻辑分离。
package types

import (
	"sort"
)

// The interface provides AOP (Aspect Oriented Programming) mechanism, which is similar to interceptor or hook mechanism, but more powerful and flexible.
//
//   - It allows adding extra behavior to the rule chain execution without modifying the original logic of the rule chain or nodes.
//   - It allows separating some common behaviors (such as logging, security, rule chain execution tracking, component degradation, component retry, component caching) from the business logic.
//   - It provides fine-grained control over the execution flow of rule chains and individual nodes.
//   - It supports both synchronous and asynchronous aspect execution patterns.
//
// 该接口提供 AOP(面向切面编程，Aspect Oriented Programming)机制，它类似拦截器或者hook机制，但是功能更加强大和灵活。
//
//   - 它允许在不修改规则链或节点的原有逻辑的情况下，对规则链的执行添加额外的行为，或者直接替换原规则链或者节点逻辑。
//   - 它允许把一些公共的行为（例如：日志、安全、规则链执行跟踪、组件降级、组件重试、组件缓存）从业务逻辑中分离出来。
//   - 它提供了对规则链和个别节点执行流程的细粒度控制。
//   - 它支持同步和异步的切面执行模式。

// ============ Core Aspect Interfaces / 核心切面接口 ============

// Aspect is the base interface for all advice types in the AOP system.
// All aspect implementations must implement this interface to define their execution order and instantiation behavior.
//
// Aspect 是 AOP 系统中所有增强点类型的基础接口。
// 所有切面实现都必须实现此接口以定义其执行顺序和实例化行为。
type Aspect interface {
	// Order returns the execution order of this aspect. Lower values indicate higher priority.
	// Aspects with lower order values will be executed before those with higher values.
	// This is particularly important when multiple aspects are applied to the same join point.
	//
	// Order 返回此切面的执行顺序。较低的值表示较高的优先级。
	// 具有较低顺序值的切面将在具有较高值的切面之前执行。
	// 当多个切面应用于同一个连接点时，这一点尤其重要。
	Order() int

	// New creates a new instance of this aspect for use in a rule chain context.
	// This method is called during rule chain initialization to create isolated instances.
	// If any field values need to be inherited from the original instance, they must be handled here.
	// Each rule chain will have its own aspect instances to ensure thread safety and isolation.
	//
	// New 创建此切面的新实例，用于规则链上下文中。
	// 此方法在规则链初始化期间调用以创建隔离的实例。
	// 如果任何字段值需要从原始实例继承，必须在这里处理。
	// 每个规则链将拥有自己的切面实例以确保线程安全和隔离。
	New() Aspect
}

// ============ Node-Level Aspect Interfaces / 节点级切面接口 ============

// NodeAspect is the base interface for all node-level advice types.
// It extends the base Aspect interface with a point-cut mechanism to determine
// when the aspect should be applied to specific nodes and message flows.
//
// NodeAspect 是所有节点级增强点类型的基础接口。
// 它扩展了基础 Aspect 接口，具有切入点机制来确定
// 何时将切面应用于特定节点和消息流。
type NodeAspect interface {
	Aspect

	// PointCut declares a join point that determines whether this aspect should be executed
	// for a specific node's OnMsg method invocation. This method provides fine-grained control
	// over when aspects are applied based on context, message content, and relationship type.
	//
	// Parameters:
	//   - ctx: The rule context containing information about the current execution state
	//   - msg: The message being processed
	//   - relationType: The relationship type that led to this node (e.g., "Success", "Failure")
	//
	// Returns:
	//   - true: The aspect should be executed for this invocation
	//   - false: The aspect should be skipped for this invocation
	//
	// Examples:
	//   - return ctx.Self().Type() == "mqttClient" // Only apply to MQTT client nodes
	//   - return relationType == "Success" // Only apply on success relationships
	//   - return msg.Type == "TELEMETRY" // Only apply to telemetry messages
	//
	// PointCut 声明一个切入点，用于判断是否需要对特定节点的 OnMsg 方法调用执行此切面。
	// 此方法提供了基于上下文、消息内容和关系类型精细控制何时应用切面的机制。
	//
	// 参数：
	//   - ctx: 包含当前执行状态信息的规则上下文
	//   - msg: 正在处理的消息
	//   - relationType: 导致到达此节点的关系类型（例如："Success"、"Failure"）
	//
	// 返回值：
	//   - true: 应该为此调用执行切面
	//   - false: 应该为此调用跳过切面
	//
	// 示例：
	//   - return ctx.Self().Type() == "mqttClient" // 仅应用于 MQTT 客户端节点
	//   - return relationType == "Success" // 仅在成功关系上应用
	//   - return msg.Type == "TELEMETRY" // 仅应用于遥测消息
	PointCut(ctx RuleContext, msg RuleMsg, relationType string) bool
}

// BeforeAspect provides pre-execution advice for node OnMsg methods.
// This aspect type is executed before the target node's OnMsg method is called,
// allowing for message preprocessing, validation, or logging.
//
// BeforeAspect 为节点 OnMsg 方法提供前置执行增强。
// 此切面类型在目标节点的 OnMsg 方法被调用之前执行，
// 允许进行消息预处理、验证或日志记录。
type BeforeAspect interface {
	NodeAspect

	// Before is executed before the node's OnMsg method. The returned message will be used
	// as input for subsequent aspects and the node's OnMsg method.
	// This allows for message transformation, enrichment, or validation before processing.
	//
	// Use cases:
	//   - Message validation and sanitization
	//   - Adding metadata or context information
	//   - Logging incoming messages
	//   - Rate limiting or throttling
	//   - Security checks and authorization
	//
	// Before 在节点的 OnMsg 方法之前执行。返回的消息将用作
	// 后续切面和节点 OnMsg 方法的输入。
	// 这允许在处理之前进行消息转换、丰富或验证。
	//
	// 使用场景：
	//   - 消息验证和清理
	//   - 添加元数据或上下文信息
	//   - 记录传入消息
	//   - 速率限制或节流
	//   - 安全检查和授权
	Before(ctx RuleContext, msg RuleMsg, relationType string) RuleMsg
}

// AfterAspect provides post-execution advice for node OnMsg methods.
// This aspect type is executed after the target node's OnMsg method completes,
// allowing for result processing, cleanup, or error handling.
//
// AfterAspect 为节点 OnMsg 方法提供后置执行增强。
// 此切面类型在目标节点的 OnMsg 方法完成后执行，
// 允许进行结果处理、清理或错误处理。
type AfterAspect interface {
	NodeAspect

	// After is executed after the node's OnMsg method completes. The returned message
	// will be used as input for subsequent aspects and the next node's OnMsg method.
	// The err parameter contains any error that occurred during node execution.
	//
	// Use cases:
	//   - Result transformation or enrichment
	//   - Error handling and recovery
	//   - Logging execution results
	//   - Performance monitoring
	//   - Cleanup operations
	//   - Audit trail generation
	//
	// After 在节点的 OnMsg 方法完成后执行。返回的消息
	// 将用作后续切面和下一个节点 OnMsg 方法的输入。
	// err 参数包含节点执行期间发生的任何错误。
	//
	// 使用场景：
	//   - 结果转换或丰富
	//   - 错误处理和恢复
	//   - 记录执行结果
	//   - 性能监控
	//   - 清理操作
	//   - 审计跟踪生成
	After(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// AroundAspect provides around-execution advice for node OnMsg methods.
// This is the most powerful aspect type, allowing complete control over whether
// and how the target node's OnMsg method is executed.
//
// AroundAspect 为节点 OnMsg 方法提供环绕执行增强。
// 这是最强大的切面类型，允许完全控制是否以及如何执行目标节点的 OnMsg 方法。
type AroundAspect interface {
	NodeAspect

	// Around wraps the execution of the node's OnMsg method, providing complete control
	// over the execution flow. The returned boolean determines whether the engine should
	// proceed with calling the node's OnMsg method.
	//
	// Return Values:
	//   - msg: The message to be passed to subsequent processing (may be modified)
	//   - bool: true = engine will call the node's OnMsg method
	//           false = engine will NOT call the node's OnMsg method (aspect takes full control)
	//
	// When returning false, the aspect is responsible for:
	//   - Manually calling ctx.Self().OnMsg(ctx, msg) if needed
	//   - Controlling flow via ctx.TellNext(msg, relationType)
	//   - Ensuring proper execution flow continuation
	//
	// Use cases:
	//   - Circuit breaker patterns
	//   - Caching (skip execution if cached result exists)
	//   - Conditional execution based on complex logic
	//   - Load balancing and failover
	//   - Performance optimization through execution bypassing
	//
	// Around 包装节点 OnMsg 方法的执行，提供对执行流程的完全控制。
	// 返回的布尔值决定引擎是否应该继续调用节点的 OnMsg 方法。
	//
	// 返回值：
	//   - msg: 要传递给后续处理的消息（可能已修改）
	//   - bool: true = 引擎将调用节点的 OnMsg 方法
	//           false = 引擎不会调用节点的 OnMsg 方法（切面完全控制）
	//
	// 当返回 false 时，切面负责：
	//   - 如果需要，手动调用 ctx.Self().OnMsg(ctx, msg)
	//   - 通过 ctx.TellNext(msg, relationType) 控制流程
	//   - 确保正确的执行流程继续
	//
	// 使用场景：
	//   - 断路器模式
	//   - 缓存（如果存在缓存结果则跳过执行）
	//   - 基于复杂逻辑的条件执行
	//   - 负载均衡和故障转移
	//   - 通过跳过执行进行性能优化
	Around(ctx RuleContext, msg RuleMsg, relationType string) (RuleMsg, bool)
}

// ============ Engine-Level Aspect Interfaces / 引擎级切面接口 ============

// StartAspect provides pre-execution advice for the entire rule engine.
// This aspect is executed before any nodes in the rule chain are processed,
// making it ideal for global preprocessing and validation.
//
// StartAspect 为整个规则引擎提供前置执行增强。
// 此切面在规则链中的任何节点被处理之前执行，
// 使其非常适合全局预处理和验证。
type StartAspect interface {
	NodeAspect

	// Start is executed before the rule engine begins processing a message.
	// The returned message will be used as input for subsequent aspects and nodes.
	// If an error is returned, the entire rule chain execution will be terminated.
	//
	// Use cases:
	//   - Global message validation
	//   - Security and authentication checks
	//   - Request preprocessing and enrichment
	//   - Rate limiting at the engine level
	//   - Logging and monitoring setup
	//
	// Start 在规则引擎开始处理消息之前执行。
	// 返回的消息将用作后续切面和节点的输入。
	// 如果返回错误，整个规则链执行将被终止。
	//
	// 使用场景：
	//   - 全局消息验证
	//   - 安全和身份验证检查
	//   - 请求预处理和丰富
	//   - 引擎级别的速率限制
	//   - 日志记录和监控设置
	Start(ctx RuleContext, msg RuleMsg) (RuleMsg, error)
}

// EndAspect provides post-execution advice for individual rule chain branches.
// This aspect is executed when a branch of the rule chain completes execution,
// allowing for branch-specific result processing.
//
// EndAspect 为各个规则链分支提供后置执行增强。
// 此切面在规则链的一个分支完成执行时执行，
// 允许进行特定于分支的结果处理。
type EndAspect interface {
	NodeAspect

	// End is executed when a branch of the rule chain completes execution.
	// This may be called multiple times if the rule chain has multiple terminal nodes.
	// The returned message will be used as input for subsequent aspects.
	//
	// Use cases:
	//   - Branch-specific result processing
	//   - Error handling and logging for individual branches
	//   - Metrics collection per branch
	//   - Conditional result transformation
	//
	// End 在规则链的一个分支完成执行时执行。
	// 如果规则链有多个终端节点，这可能会被多次调用。
	// 返回的消息将用作后续切面的输入。
	//
	// 使用场景：
	//   - 特定于分支的结果处理
	//   - 单个分支的错误处理和日志记录
	//   - 每个分支的指标收集
	//   - 条件结果转换
	End(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// CompletedAspect provides post-execution advice for the entire rule chain.
// This aspect is executed only once, after all branches of the rule chain have completed,
// making it ideal for final processing and cleanup.
//
// CompletedAspect 为整个规则链提供后置执行增强。
// 此切面仅执行一次，在规则链的所有分支都完成后，
// 使其非常适合最终处理和清理。
type CompletedAspect interface {
	NodeAspect

	// Completed is executed after all branches of the rule chain have finished execution.
	// This is called exactly once per rule chain execution, regardless of the number of branches.
	// The returned message will be used as input for subsequent aspects.
	//
	// Use cases:
	//   - Final result aggregation and processing
	//   - Cleanup operations
	//   - Global metrics and monitoring
	//   - Resource cleanup and connection management
	//   - Final logging and audit trail completion
	//
	// Completed 在规则链的所有分支都完成执行后执行。
	// 无论分支数量如何，每次规则链执行都只调用一次。
	// 返回的消息将用作后续切面的输入。
	//
	// 使用场景：
	//   - 最终结果聚合和处理
	//   - 清理操作
	//   - 全局指标和监控
	//   - 资源清理和连接管理
	//   - 最终日志记录和审计跟踪完成
	Completed(ctx RuleContext, msg RuleMsg) RuleMsg
}

// ============ Lifecycle Aspect Interfaces / 生命周期切面接口 ============

// RuleContextInitAspect provides execution-level aspect instance creation capability.
// This interface allows aspects to create new instances for each rule execution, ensuring
// complete state isolation between different executions.
//
// Key differences between initialization levels:
//   - Aspect.New(): Engine-level, called once during rule engine initialization, creates engine-wide instances
//   - InitWithContext(): Execution-level, called for each rule execution, creates execution-specific instances
//
// RuleContextInitAspect 提供执行级别的切面实例创建能力。
// 此接口允许切面为每次规则执行创建新实例，确保不同执行之间的完全状态隔离。
//
// 初始化级别之间的主要区别：
//   - Aspect.New(): 引擎级别，在规则引擎初始化时调用一次，创建引擎范围的实例
//   - InitWithContext(): 执行级别，为每次规则执行调用，创建执行特定的实例
type RuleContextInitAspect interface {
	// InitWithContext is called for each rule execution to create a new aspect instance.
	// This method should return a new aspect instance with execution-specific initialization.
	// The returned instance will be used only for this specific execution, ensuring state isolation.
	//
	// Execution order: Engine.New() → Aspect.New() → [For each execution] → InitWithContext()
	//
	// Benefits over Aspect.New():
	//   - Complete state isolation between executions
	//   - Access to execution-specific RuleContext and configuration
	//   - Ability to initialize based on current execution context
	//   - Per-execution monitoring and tracking
	//
	// Use cases:
	//   - Create execution-specific state containers (logs, metrics, etc.)
	//   - Initialize per-execution monitoring with unique identifiers
	//   - Set up execution-scoped callbacks and handlers
	//   - Extract execution context for specialized tracking
	//
	// Note: Aspects implementing both Aspect.New() and InitWithContext():
	//   - New() creates engine-wide template instances
	//   - InitWithContext() creates execution-specific working instances
	//
	// InitWithContext 为每次规则执行调用以创建新的切面实例。
	// 此方法应返回具有执行特定初始化的新切面实例。
	// 返回的实例仅用于此特定执行，确保状态隔离。
	//
	// 执行顺序：Engine.New() → Aspect.New() → [每次执行] → InitWithContext()
	//
	// 相比 Aspect.New() 的优势：
	//   - 执行之间的完全状态隔离
	//   - 访问执行特定的 RuleContext 和配置
	//   - 能够基于当前执行上下文进行初始化
	//   - 每次执行的监控和跟踪
	//
	// 使用场景：
	//   - 创建执行特定的状态容器（日志、指标等）
	//   - 使用唯一标识符初始化每次执行的监控
	//   - 设置执行范围的回调和处理器
	//   - 为专门跟踪提取执行上下文
	//
	// 注意：同时实现 Aspect.New() 和 InitWithContext() 的切面：
	//   - New() 创建引擎范围的模板实例
	//   - InitWithContext() 创建执行特定的工作实例
	InitWithContext(ctx RuleContext) Aspect
}

// OnChainBeforeInitAspect provides advice before rule engine initialization.
// This aspect can prevent engine creation by returning an error.
//
// OnChainBeforeInitAspect 在规则引擎初始化之前提供增强。
// 此切面可以通过返回错误来阻止引擎创建。
type OnChainBeforeInitAspect interface {
	Aspect

	// OnChainBeforeInit is executed before the rule engine is initialized.
	// If an error is returned, the engine creation will fail.
	//
	// Use cases:
	//   - Configuration validation
	//   - Resource availability checks
	//   - Security policy enforcement
	//   - Environment prerequisites validation
	//
	// OnChainBeforeInit 在规则引擎初始化之前执行。
	// 如果返回错误，引擎创建将失败。
	//
	// 使用场景：
	//   - 配置验证
	//   - 资源可用性检查
	//   - 安全策略执行
	//   - 环境先决条件验证
	OnChainBeforeInit(config Config, def *RuleChain) error
}

// OnNodeBeforeInitAspect provides advice before rule node initialization.
// This aspect can prevent node creation by returning an error.
//
// OnNodeBeforeInitAspect 在规则节点初始化之前提供增强。
// 此切面可以通过返回错误来阻止节点创建。
type OnNodeBeforeInitAspect interface {
	Aspect

	// OnNodeBeforeInit is executed before a rule node is initialized.
	// If an error is returned, the node creation will fail.
	//
	// Use cases:
	//   - Node configuration validation
	//   - Component availability checks
	//   - License and feature validation
	//   - Resource allocation verification
	//
	// OnNodeBeforeInit 在规则节点初始化之前执行。
	// 如果返回错误，节点创建将失败。
	//
	// 使用场景：
	//   - 节点配置验证
	//   - 组件可用性检查
	//   - 许可证和功能验证
	//   - 资源分配验证
	OnNodeBeforeInit(config Config, def *RuleNode) error
}

// OnCreatedAspect provides advice after successful rule engine creation.
// This aspect is called once the engine has been successfully initialized.
//
// OnCreatedAspect 在规则引擎成功创建后提供增强。
// 此切面在引擎成功初始化后调用。
type OnCreatedAspect interface {
	Aspect

	// OnCreated is executed after the rule engine is successfully created and initialized.
	// This provides an opportunity to perform post-initialization setup.
	//
	// Use cases:
	//   - Monitoring and metrics setup
	//   - Resource registration
	//   - Event notifications
	//   - Integration with external systems
	//
	// OnCreated 在规则引擎成功创建和初始化后执行。
	// 这提供了执行初始化后设置的机会。
	//
	// 使用场景：
	//   - 监控和指标设置
	//   - 资源注册
	//   - 事件通知
	//   - 与外部系统集成
	OnCreated(chainCtx NodeCtx) error
}

// OnReloadAspect provides advice after rule engine reload operations.
// This aspect is called whenever the rule chain or child nodes are reloaded.
//
// OnReloadAspect 在规则引擎重载操作后提供增强。
// 此切面在规则链或子节点重载时调用。
type OnReloadAspect interface {
	Aspect

	// OnReload is executed after the rule engine reloads the rule chain or child node configuration.
	// Note: Rule chain updates trigger OnDestroy, OnBeforeReload, and OnReload in sequence.
	// If the entire rule chain is updated, then chainCtx equals ctx.
	//
	// Parameters:
	//   - chainCtx: The context of the rule chain being reloaded
	//   - ctx: The specific context being reloaded (may be a node or the entire chain)
	//
	// Use cases:
	//   - Cache invalidation after configuration changes
	//   - Metric reset and reinitialization
	//   - Event notifications for configuration updates
	//   - Resource reallocation
	//
	// OnReload 在规则引擎重新加载规则链或子节点配置后执行。
	// 注意：规则链更新会依次触发 OnDestroy、OnBeforeReload 和 OnReload。
	// 如果更新整个规则链，则 chainCtx 等于 ctx。
	//
	// 参数：
	//   - chainCtx: 正在重载的规则链的上下文
	//   - ctx: 正在重载的特定上下文（可能是节点或整个链）
	//
	// 使用场景：
	//   - 配置更改后的缓存失效
	//   - 指标重置和重新初始化
	//   - 配置更新的事件通知
	//   - 资源重新分配
	OnReload(chainCtx NodeCtx, ctx NodeCtx) error
}

// OnDestroyAspect provides advice after rule engine instance destruction.
// This aspect is called when the engine instance is being destroyed.
//
// OnDestroyAspect 在规则引擎实例销毁后提供增强。
// 此切面在引擎实例被销毁时调用。
type OnDestroyAspect interface {
	Aspect

	// OnDestroy is executed after the rule engine instance is destroyed.
	// This provides an opportunity to perform cleanup operations.
	//
	// Use cases:
	//   - Resource cleanup and deallocation
	//   - Connection closure
	//   - Cache cleanup
	//   - Metric finalization
	//   - Event notifications
	//
	// OnDestroy 在规则引擎实例销毁后执行。
	// 这提供了执行清理操作的机会。
	//
	// 使用场景：
	//   - 资源清理和释放
	//   - 连接关闭
	//   - 缓存清理
	//   - 指标最终化
	//   - 事件通知
	OnDestroy(chainCtx NodeCtx)
}

// ============ Callback Listener Interfaces / 回调监听器接口 ============

// RuleChainCompletedListener defines the interface for aspects that want to receive
// rule chain completion callbacks. Aspects implementing this interface can be notified
// when a rule chain execution completes with a detailed execution snapshot.
//
// RuleChainCompletedListener 定义了希望接收规则链完成回调的切面接口。
// 实现此接口的切面可以在规则链执行完成时收到详细的执行快照通知。
type RuleChainCompletedListener interface {
	// SetOnRuleChainCompleted sets the callback function to be invoked when a rule chain
	// execution completes. The callback receives the execution context and a detailed
	// snapshot of the entire rule chain execution.
	//
	// Parameters:
	//   - onCallback: Function to be called with execution context and snapshot
	//
	// The snapshot contains:
	//   - Execution logs for each node
	//   - Timing information
	//   - Relationship types used
	//   - Error information if any
	//
	// SetOnRuleChainCompleted 设置在规则链执行完成时要调用的回调函数。
	// 回调接收执行上下文和整个规则链执行的详细快照。
	//
	// 参数：
	//   - onCallback: 要用执行上下文和快照调用的函数
	//
	// 快照包含：
	//   - 每个节点的执行日志
	//   - 时间信息
	//   - 使用的关系类型
	//   - 错误信息（如果有）
	SetOnRuleChainCompleted(onCallback func(ctx RuleContext, snapshot RuleChainRunSnapshot))
}

// NodeCompletedListener defines the interface for aspects that want to receive
// individual node completion callbacks. This provides more granular monitoring
// than rule chain completion callbacks.
//
// NodeCompletedListener 定义了希望接收单个节点完成回调的切面接口。
// 这提供了比规则链完成回调更细粒度的监控。
type NodeCompletedListener interface {
	// SetOnNodeCompleted sets the callback function to be invoked when an individual
	// node completes execution. This allows for fine-grained monitoring of node performance
	// and behavior.
	//
	// Parameters:
	//   - onCallback: Function to be called with execution context and node run log
	//
	// The node run log contains:
	//   - Node ID and type
	//   - Execution timing
	//   - Input/output relationship types
	//   - Error information if any
	//
	// SetOnNodeCompleted 设置在单个节点完成执行时要调用的回调函数。
	// 这允许对节点性能和行为进行细粒度监控。
	//
	// 参数：
	//   - onCallback: 要用执行上下文和节点运行日志调用的函数
	//
	// 节点运行日志包含：
	//   - 节点ID和类型
	//   - 执行时间
	//   - 输入/输出关系类型
	//   - 错误信息（如果有）
	SetOnNodeCompleted(onCallback func(ctx RuleContext, nodeRunLog RuleNodeRunLog))
}

// DebugListener defines the interface for aspects that want to receive
// debug callbacks. This is useful for real-time debugging and monitoring
// of rule chain execution.
//
// DebugListener 定义了希望接收调试回调的切面接口。
// 这对于规则链执行的实时调试和监控很有用。
type DebugListener interface {
	// SetOnDebug sets the callback function to be invoked for debug events.
	// Debug events are generated in real-time during rule chain execution
	// and are only triggered if debug mode is enabled for the relevant nodes.
	//
	// Parameters:
	//   - onDebug: Function to be called with debug information
	//
	// Debug information includes:
	//   - Rule chain ID
	//   - Flow type (IN/OUT)
	//   - Node ID
	//   - Message content
	//   - Relationship type
	//   - Error information if any
	//
	// SetOnDebug 设置用于调试事件的回调函数。
	// 调试事件在规则链执行期间实时生成，
	// 仅在相关节点启用调试模式时才触发。
	//
	// 参数：
	//   - onDebug: 要用调试信息调用的函数
	//
	// 调试信息包括：
	//   - 规则链ID
	//   - 流类型（IN/OUT）
	//   - 节点ID
	//   - 消息内容
	//   - 关系类型
	//   - 错误信息（如果有）
	SetOnDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error))
}

// ============ Aspect List Management / 切面列表管理 ============

// AspectList represents a read-only collection of aspects with utility methods for categorization
// and management. This type provides efficient sorting and filtering of aspects based
// on their types and capabilities, with pre-initialized listener caches for performance optimization.
//
// AspectList 表示具有分类和管理实用方法的只读切面集合。
// 此类型提供基于切面类型和功能的高效排序和过滤，预初始化监听器缓存以优化性能。
type AspectList struct {
	aspects []Aspect // 切面列表（只读）

	// 预初始化的监听器缓存（优化性能，避免重复遍历）
	ruleChainListeners []RuleChainCompletedListener // 规则链完成监听器缓存
	nodeListeners      []NodeCompletedListener      // 节点完成监听器缓存
	debugListeners     []DebugListener              // 调试监听器缓存
}

// NewAspectList creates a new read-only AspectList from a slice of aspects.
// The listener caches are pre-initialized for optimal performance.
// NewAspectList 从切面切片创建新的只读 AspectList。
// 监听器缓存被预初始化以获得最佳性能。
func NewAspectList(aspects []Aspect) AspectList {
	aspectList := AspectList{
		aspects: aspects,
	}

	// 预初始化所有监听器缓存
	if aspects != nil {
		for _, aspect := range aspects {
			if listener, ok := aspect.(RuleChainCompletedListener); ok {
				aspectList.ruleChainListeners = append(aspectList.ruleChainListeners, listener)
			}
			if listener, ok := aspect.(NodeCompletedListener); ok {
				aspectList.nodeListeners = append(aspectList.nodeListeners, listener)
			}
			if listener, ok := aspect.(DebugListener); ok {
				aspectList.debugListeners = append(aspectList.debugListeners, listener)
			}
		}
	}

	return aspectList
}

// Len returns the number of aspects in the list.
// Len 返回列表中切面的数量。
func (list AspectList) Len() int {
	return len(list.aspects)
}

// Aspects returns the underlying slice of aspects for compatibility.
// Aspects 返回底层的切面切片以保持兼容性。
func (list AspectList) Aspects() []Aspect {
	return list.aspects
}

// GetNodeAspects categorizes and returns node-level aspects sorted by execution order.
// This method extracts all node-level aspects (Around, Before, After) from the list
// and returns them in separate, ordered collections.
//
// Returns:
//   - []AroundAspect: Around aspects sorted by order (lowest to highest)
//   - []BeforeAspect: Before aspects sorted by order (lowest to highest)
//   - []AfterAspect: After aspects sorted by order (lowest to highest)
//
// The sorting ensures that aspects with lower Order() values are executed first.
//
// GetNodeAspects 按执行顺序分类并返回节点级切面。
// 此方法从列表中提取所有节点级切面（Around、Before、After）
// 并将它们返回为单独的有序集合。
//
// 返回值：
//   - []AroundAspect: 按顺序排序的环绕切面（从低到高）
//   - []BeforeAspect: 按顺序排序的前置切面（从低到高）
//   - []AfterAspect: 按顺序排序的后置切面（从低到高）
//
// 排序确保具有较低 Order() 值的切面首先执行。
func (list AspectList) GetNodeAspects() ([]AroundAspect, []BeforeAspect, []AfterAspect) {

	//从小到大排序
	sort.Slice(list.aspects, func(i, j int) bool {
		return list.aspects[i].Order() < list.aspects[j].Order()
	})

	var aroundAspects []AroundAspect
	var beforeAspects []BeforeAspect
	var afterAspects []AfterAspect

	for _, item := range list.aspects {
		if a, ok := item.(AroundAspect); ok {
			aroundAspects = append(aroundAspects, a)
		}
		if a, ok := item.(BeforeAspect); ok {
			beforeAspects = append(beforeAspects, a)
		}
		if a, ok := item.(AfterAspect); ok {
			afterAspects = append(afterAspects, a)
		}
	}

	return aroundAspects, beforeAspects, afterAspects
}

// GetChainAspects categorizes and returns rule chain-level aspects sorted by execution order.
// This method extracts all chain-level aspects (Start, End, Completed) from the list
// and returns them in separate, ordered collections.
//
// Returns:
//   - []StartAspect: Start aspects sorted by order (lowest to highest)
//   - []EndAspect: End aspects sorted by order (lowest to highest)
//   - []CompletedAspect: Completed aspects sorted by order (lowest to highest)
//
// The sorting ensures that aspects with lower Order() values are executed first.
//
// GetChainAspects 按执行顺序分类并返回规则链级切面。
// 此方法从列表中提取所有链级切面（Start、End、Completed）
// 并将它们返回为单独的有序集合。
//
// 返回值：
//   - []StartAspect: 按顺序排序的开始切面（从低到高）
//   - []EndAspect: 按顺序排序的结束切面（从低到高）
//   - []CompletedAspect: 按顺序排序的完成切面（从低到高）
//
// 排序确保具有较低 Order() 值的切面首先执行。
func (list AspectList) GetChainAspects() ([]StartAspect, []EndAspect, []CompletedAspect) {

	//从小到大排序
	sort.Slice(list.aspects, func(i, j int) bool {
		return list.aspects[i].Order() < list.aspects[j].Order()
	})

	var startAspects []StartAspect
	var endAspects []EndAspect
	var completedAspects []CompletedAspect
	for _, item := range list.aspects {
		if a, ok := item.(StartAspect); ok {
			startAspects = append(startAspects, a)
		}
		if a, ok := item.(EndAspect); ok {
			endAspects = append(endAspects, a)
		}
		if a, ok := item.(CompletedAspect); ok {
			completedAspects = append(completedAspects, a)
		}
	}

	return startAspects, endAspects, completedAspects
}

// GetEngineAspects categorizes and returns engine lifecycle aspects sorted by execution order.
// This method extracts all engine lifecycle aspects from the list and returns them
// in separate, ordered collections for different lifecycle events.
//
// Returns:
//   - []OnChainBeforeInitAspect: Chain initialization aspects
//   - []OnNodeBeforeInitAspect: Node initialization aspects
//   - []OnCreatedAspect: Creation completion aspects
//   - []OnReloadAspect: Reload event aspects
//   - []OnDestroyAspect: Destruction event aspects
//
// All returned collections are sorted by execution order (lowest to highest).
//
// GetEngineAspects 按执行顺序分类并返回引擎生命周期切面。
// 此方法从列表中提取所有引擎生命周期切面，并将它们
// 返回为不同生命周期事件的单独有序集合。
//
// 返回值：
//   - []OnChainBeforeInitAspect: 链初始化切面
//   - []OnNodeBeforeInitAspect: 节点初始化切面
//   - []OnCreatedAspect: 创建完成切面
//   - []OnReloadAspect: 重载事件切面
//   - []OnDestroyAspect: 销毁事件切面
//
// 所有返回的集合都按执行顺序排序（从低到高）。
func (list AspectList) GetEngineAspects() ([]OnChainBeforeInitAspect, []OnNodeBeforeInitAspect, []OnCreatedAspect, []OnReloadAspect, []OnDestroyAspect) {

	//从小到大排序
	sort.Slice(list.aspects, func(i, j int) bool {
		return list.aspects[i].Order() < list.aspects[j].Order()
	})

	var chainBeforeInitAspects []OnChainBeforeInitAspect
	var nodeBeforeInitAspects []OnNodeBeforeInitAspect
	var createdAspects []OnCreatedAspect
	var afterReloadAspects []OnReloadAspect
	var destroyAspects []OnDestroyAspect

	for _, item := range list.aspects {
		if a, ok := item.(OnChainBeforeInitAspect); ok {
			chainBeforeInitAspects = append(chainBeforeInitAspects, a)
		}
		if a, ok := item.(OnNodeBeforeInitAspect); ok {
			nodeBeforeInitAspects = append(nodeBeforeInitAspects, a)
		}
		if a, ok := item.(OnCreatedAspect); ok {
			createdAspects = append(createdAspects, a)
		}
		if a, ok := item.(OnReloadAspect); ok {
			afterReloadAspects = append(afterReloadAspects, a)
		}
		if a, ok := item.(OnDestroyAspect); ok {
			destroyAspects = append(destroyAspects, a)
		}
	}

	return chainBeforeInitAspects, nodeBeforeInitAspects, createdAspects, afterReloadAspects, destroyAspects
}

// ============ Callback Listener Extraction Methods / 回调监听器提取方法 ============

// GetRuleChainCompletedListeners extracts all aspects that implement the RuleChainCompletedListener interface.
// This method returns pre-initialized cached listeners for optimal performance.
//
// Returns:
//   - []RuleChainCompletedListener: All aspects that can receive rule chain completion callbacks
//
// GetRuleChainCompletedListeners 提取实现 RuleChainCompletedListener 接口的所有切面。
// 此方法返回预初始化的缓存监听器以获得最佳性能。
//
// 返回值：
//   - []RuleChainCompletedListener: 可以接收规则链完成回调的所有切面
func (list AspectList) GetRuleChainCompletedListeners() []RuleChainCompletedListener {
	return list.ruleChainListeners
}

// GetNodeCompletedListeners extracts all aspects that implement the NodeCompletedListener interface.
// This method returns pre-initialized cached listeners for optimal performance.
//
// Returns:
//   - []NodeCompletedListener: All aspects that can receive node completion callbacks
//
// GetNodeCompletedListeners 提取实现 NodeCompletedListener 接口的所有切面。
// 此方法返回预初始化的缓存监听器以获得最佳性能。
//
// 返回值：
//   - []NodeCompletedListener: 可以接收节点完成回调的所有切面
func (list AspectList) GetNodeCompletedListeners() []NodeCompletedListener {
	return list.nodeListeners
}

// GetDebugListeners extracts all aspects that implement the DebugListener interface.
// This method returns pre-initialized cached listeners for optimal performance.
//
// Returns:
//   - []DebugListener: All aspects that can receive debug callbacks
//
// GetDebugListeners 提取实现 DebugListener 接口的所有切面。
// 此方法返回预初始化的缓存监听器以获得最佳性能。
//
// 返回值：
//   - []DebugListener: 可以接收调试回调的所有切面
func (list AspectList) GetDebugListeners() []DebugListener {
	return list.debugListeners
}
