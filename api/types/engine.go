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

package types

import "github.com/rulego/rulego/api/types/metrics"

// RuleEngineOption defines a function type for configuring a RuleEngine.
// It follows the functional options pattern for flexible configuration.
//
// RuleEngineOption 定义了用于配置 RuleEngine 的函数类型。
// 它遵循函数选项模式，提供灵活的配置。
//
// Example usage:
// 使用示例：
//
//	engine, err := rulego.New("myEngine", ruleChainDef,
//		types.WithConfig(myConfig),
//		types.WithAspects(debugAspect, metricsAspect))
type RuleEngineOption func(RuleEngine) error

// WithConfig creates a RuleEngineOption to set the configuration of a RuleEngine.
// This allows customizing the engine's behavior, logging, caching, and other settings.
//
// WithConfig 创建一个 RuleEngineOption 来设置 RuleEngine 的配置。
// 这允许自定义引擎的行为、日志记录、缓存和其他设置。
func WithConfig(config Config) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetConfig(config) // Apply the provided configuration to the RuleEngine.
		return nil           // Return no error.
	}
}

// WithAspects creates a RuleEngineOption to set the aspects of a RuleEngine.
// Aspects provide AOP (Aspect-Oriented Programming) capabilities for cross-cutting concerns
// like logging, metrics, validation, and debugging.
//
// WithAspects 创建一个 RuleEngineOption 来设置 RuleEngine 的切面。
// 切面为日志记录、指标、验证和调试等横切关注点提供 AOP（面向切面编程）功能。
func WithAspects(aspects ...Aspect) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetAspects(aspects...) // Apply the provided aspects to the RuleEngine.
		return nil                // Return no error.
	}
}

// WithRuleEnginePool creates a RuleEngineOption to set the rule engine pool.
// This enables the engine to manage sub-rule chains and cross-chain communication.
//
// WithRuleEnginePool 创建一个 RuleEngineOption 来设置规则引擎池。
// 这使引擎能够管理子规则链和跨链通信。
func WithRuleEnginePool(ruleEnginePool RuleEnginePool) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetRuleEnginePool(ruleEnginePool)
		return nil
	}
}

// RuleEngine is the core interface for a rule engine instance.
// Each RuleEngine manages a single root rule chain and provides methods for
// message processing, configuration updates, and lifecycle management.
//
// RuleEngine 是规则引擎实例的核心接口。
// 每个 RuleEngine 管理一个根规则链，并提供消息处理、配置更新和生命周期管理的方法。
//
// Key Features:
// 主要特性：
//   - Rule chain execution and management  规则链执行和管理
//   - Dynamic configuration reloading  动态配置重载
//   - Aspect-oriented programming support  面向切面编程支持
//   - Performance metrics collection  性能指标收集
//   - Concurrent message processing  并发消息处理
//
// Lifecycle:
// 生命周期：
//  1. Create engine with New() or Load()  使用 New() 或 Load() 创建引擎
//  2. Process messages with OnMsg()  使用 OnMsg() 处理消息
//  3. Update configuration with ReloadSelf()  使用 ReloadSelf() 更新配置
//  4. Clean up resources with Stop()  使用 Stop() 清理资源
type RuleEngine interface {
	// Id returns the unique identifier of the RuleEngine.
	// This ID is used for engine lookup and management within pools.
	// Id 返回 RuleEngine 的唯一标识符。
	// 此 ID 用于池中的引擎查找和管理。
	Id() string

	// SetConfig sets the configuration for the RuleEngine.
	// This affects logging, caching, component registry, and other engine behaviors.
	// SetConfig 设置 RuleEngine 的配置。
	// 这影响日志记录、缓存、组件注册表和其他引擎行为。
	SetConfig(config Config)

	// SetAspects sets the aspects for the RuleEngine.
	// Aspects provide cross-cutting functionality like metrics, debugging, and validation.
	// SetAspects 设置 RuleEngine 的切面。
	// 切面提供如指标、调试和验证等横切功能。
	SetAspects(aspects ...Aspect)

	// SetRuleEnginePool sets the rule engine pool for the RuleEngine.
	// This enables sub-rule chain execution and cross-chain communication.
	// SetRuleEnginePool 设置 RuleEngine 的规则引擎池。
	// 这启用子规则链执行和跨链通信。
	SetRuleEnginePool(ruleEnginePool RuleEnginePool)

	// Reload reloads the RuleEngine with the given options.
	// This refreshes the current rule chain configuration while applying new options.
	// Reload 使用给定选项重新加载 RuleEngine。
	// 这在应用新选项的同时刷新当前规则链配置。
	Reload(opts ...RuleEngineOption) error

	// ReloadSelf reloads the RuleEngine itself with the given definition and options.
	// This completely replaces the current rule chain with a new configuration.
	// ReloadSelf 使用给定定义和选项重新加载 RuleEngine 本身。
	// 这完全用新配置替换当前规则链。
	ReloadSelf(def []byte, opts ...RuleEngineOption) error

	// ReloadChild reloads a specific child node within the RuleEngine.
	// This allows partial updates without affecting the entire rule chain.
	// ReloadChild 重新加载 RuleEngine 中的特定子节点。
	// 这允许部分更新而不影响整个规则链。
	ReloadChild(ruleNodeId string, dsl []byte) error

	// DSL returns the DSL (Domain Specific Language) representation of the RuleEngine.
	// This provides the complete rule chain configuration in serialized format.
	// DSL 返回 RuleEngine 的 DSL（领域特定语言）表示。
	// 这以序列化格式提供完整的规则链配置。
	DSL() []byte

	// Definition returns the structured definition of the rule chain.
	// This provides programmatic access to the rule chain structure.
	// Definition 返回规则链的结构化定义。
	// 这提供对规则链结构的编程访问。
	Definition() RuleChain

	// RootRuleChainCtx returns the context of the root rule chain.
	// This provides access to the chain's execution context and management methods.
	// RootRuleChainCtx 返回根规则链的上下文。
	// 这提供对链的执行上下文和管理方法的访问。
	RootRuleChainCtx() ChainCtx

	// NodeDSL returns the DSL of a specific node within the rule chain.
	// This enables inspection and management of individual nodes.
	// NodeDSL 返回规则链中特定节点的 DSL。
	// 这启用对单个节点的检查和管理。
	NodeDSL(chainId RuleNodeId, childNodeId RuleNodeId) []byte

	// Initialized checks if the RuleEngine is properly initialized and ready for use.
	// Returns true if the engine has a valid rule chain configuration.
	// Initialized 检查 RuleEngine 是否已正确初始化并准备好使用。
	// 如果引擎具有有效的规则链配置，则返回 true。
	Initialized() bool

	// Stop gracefully shuts down the RuleEngine and releases all resources.
	// This should be called when the engine is no longer needed.
	// Stop 优雅地关闭 RuleEngine 并释放所有资源。
	// 当不再需要引擎时应调用此方法。
	Stop()

	// OnMsg processes a message asynchronously with the given context options.
	// This is the primary method for feeding data into the rule engine.
	// OnMsg 使用给定上下文选项异步处理消息。
	// 这是向规则引擎输入数据的主要方法。
	OnMsg(msg RuleMsg, opts ...RuleContextOption)

	// OnMsgAndWait processes a message synchronously and waits for completion.
	// This blocks until all rule chain execution is complete.
	// OnMsgAndWait 同步处理消息并等待完成。
	// 这会阻塞直到所有规则链执行完成。
	OnMsgAndWait(msg RuleMsg, opts ...RuleContextOption)

	// RootRuleContext returns the root rule context for advanced operations.
	// This provides access to the execution context of the root rule chain.
	// RootRuleContext 返回用于高级操作的根规则上下文。
	// 这提供对根规则链执行上下文的访问。
	RootRuleContext() RuleContext

	// GetMetrics returns performance and execution metrics of the RuleEngine.
	// This is useful for monitoring, debugging, and performance optimization.
	// GetMetrics 返回 RuleEngine 的性能和执行指标。
	// 这对监控、调试和性能优化很有用。
	GetMetrics() *metrics.EngineMetrics
}

// RuleEnginePool is an interface for managing a collection of rule engines.
// It provides centralized management, loading, and coordination of multiple rule engines.
//
// RuleEnginePool 是管理规则引擎集合的接口。
// 它提供多个规则引擎的集中管理、加载和协调。
//
// Key Features:
// 主要特性：
//   - Centralized rule engine management  集中的规则引擎管理
//   - Dynamic loading from file system  从文件系统动态加载
//   - Cross-engine message broadcasting  跨引擎消息广播
//   - Lifecycle management for all engines  所有引擎的生命周期管理
//
// Usage Example:
// 使用示例：
//
//	// Load all rule chains from a directory
//	// 从目录加载所有规则链
//	err := pool.Load("./rules")
//
//	// Get a specific engine
//	// 获取特定引擎
//	engine, ok := pool.Get("engineId")
//
//	// Broadcast message to all engines
//	// 向所有引擎广播消息
//	pool.OnMsg(message)
type RuleEnginePool interface {
	// Load loads all rule chain configurations from a specified folder and its subfolders
	// into the rule engine instance pool. The rule chain ID is taken from the ruleChain.id
	// specified in the rule chain file.
	// Load 从指定文件夹及其子文件夹加载所有规则链配置到规则引擎实例池中。
	// 规则链 ID 取自规则链文件中指定的 ruleChain.id。
	Load(folderPath string, opts ...RuleEngineOption) error

	// New creates a new RuleEngine and stores it in the rule engine pool.
	// If the specified id is empty, the ruleChain.id from the rule chain file is used.
	// New 创建新的 RuleEngine 并将其存储在规则引擎池中。
	// 如果指定的 id 为空，则使用规则链文件中的 ruleChain.id。
	New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (RuleEngine, error)

	// Get retrieves a RuleEngine by its unique identifier.
	// Returns the engine and a boolean indicating whether it was found.
	// Get 通过唯一标识符检索 RuleEngine。
	// 返回引擎和表示是否找到的布尔值。
	Get(id string) (RuleEngine, bool)

	// Del removes and stops a RuleEngine instance by its ID.
	// This gracefully shuts down the engine and releases its resources.
	// Del 通过 ID 删除并停止 RuleEngine 实例。
	// 这会优雅地关闭引擎并释放其资源。
	Del(id string)

	// Stop gracefully shuts down and releases all RuleEngine instances in the pool.
	// This should be called during application shutdown.
	// Stop 优雅地关闭并释放池中的所有 RuleEngine 实例。
	// 这应该在应用程序关闭期间调用。
	Stop()

	// OnMsg broadcasts a message to all RuleEngine instances in the pool.
	// Each engine will attempt to process the message according to its rule chain.
	// OnMsg 向池中的所有 RuleEngine 实例广播消息。
	// 每个引擎将尝试根据其规则链处理消息。
	OnMsg(msg RuleMsg)

	// Reload reloads all RuleEngine instances in the pool with the given options.
	// This applies configuration changes to all engines simultaneously.
	// Reload 使用给定选项重新加载池中的所有 RuleEngine 实例。
	// 这同时将配置更改应用到所有引擎。
	Reload(opts ...RuleEngineOption)

	// Range iterates over all RuleEngine instances in the pool.
	// The function should return false to stop iteration.
	// Range 遍历池中的所有 RuleEngine 实例。
	// 函数应返回 false 以停止迭代。
	Range(f func(key, value any) bool)
}
