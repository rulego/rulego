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

// Package engine provides the core functionality for the RuleGo rule engine.
// It includes implementations for rule contexts, rule engines, and related components
// that enable the execution and management of rule chains.
//
// Package engine 提供 RuleGo 规则引擎的核心功能。
// 它包括规则上下文、规则引擎和相关组件的实现，
// 这些组件支持规则链的执行和管理。
//
// The engine package is responsible for:
// engine 包负责：
//   - Defining and managing rule contexts (DefaultRuleContext)
//     定义和管理规则上下文（DefaultRuleContext）
//   - Implementing the main rule engine (RuleEngine)
//     实现主要的规则引擎（RuleEngine）
//   - Handling rule chain execution and flow control
//     处理规则链执行和流程控制
//   - Managing built-in aspects and extensions
//     管理内置切面和扩展
//   - Providing utilities for rule processing and message handling
//     提供规则处理和消息处理的工具
//
// Key Components:
// 关键组件：
//   - RuleEngine: Main engine instance managing rule chain execution
//     RuleEngine：管理规则链执行的主引擎实例
//   - RuleChainCtx: Context for individual rule chains
//     RuleChainCtx：单个规则链的上下文
//   - DefaultRuleContext: Context for message processing within rule chains
//     DefaultRuleContext：规则链内消息处理的上下文
//   - RuleNodeCtx: Context wrapper for individual node components
//     RuleNodeCtx：单个节点组件的上下文包装器
//
// Architecture Overview:
// 架构概述：
//
//	The engine follows a hierarchical structure where a RuleEngine contains
//	one root RuleChainCtx, which manages multiple RuleNodeCtx instances.
//	Message processing flows through DefaultRuleContext instances that
//	coordinate between nodes and handle aspect-oriented programming features.
//
//	引擎遵循分层结构，其中 RuleEngine 包含一个根 RuleChainCtx，
//	该上下文管理多个 RuleNodeCtx 实例。消息处理通过 DefaultRuleContext
//	实例流转，这些实例在节点之间协调并处理面向切面编程功能。
//
// This package is central to the RuleGo framework, offering the primary mechanisms
// for rule-based processing and decision making in various applications.
//
// 此包是 RuleGo 框架的核心，为各种应用程序中基于规则的处理和决策制定
// 提供主要机制。
package engine

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/rulego/rulego/api/types/metrics"
	"github.com/rulego/rulego/utils/cache"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/builtin/funcs"
	"github.com/rulego/rulego/components/base"
)

// Ensuring RuleEngine implements types.RuleEngine interface.
var _ types.RuleEngine = (*RuleEngine)(nil)

// BuiltinsAspects holds a list of built-in aspects for the rule engine.
// These aspects provide essential cross-cutting functionality and are automatically
// integrated into every rule engine instance to ensure consistent behavior.
//
// BuiltinsAspects 保存规则引擎的内置切面列表。
// 这些切面提供基本的横切功能，并自动集成到每个规则引擎实例中以确保一致的行为。
//
// Built-in Aspects:
// 内置切面：
//
//   - Validator: Validates node configurations and rule chain definitions
//     before execution to prevent runtime errors.
//     验证器：在执行前验证节点配置和规则链定义，以防止运行时错误。
//
//   - Debug: Provides debugging capabilities including execution tracing,
//     state inspection, and development-time diagnostics.
//     调试器：提供调试功能，包括执行跟踪、状态检查和开发时诊断。
//
//   - MetricsAspect: Collects performance metrics, execution statistics,
//     and operational data for monitoring and observability.
//     指标切面：收集性能指标、执行统计和运营数据，用于监控和可观察性。
//
// Automatic Integration:
// 自动集成：
//
//	These aspects are automatically added to the rule engine during initialization
//	via the initBuiltinsAspects() method. If custom aspects are provided, the
//	built-in aspects are still included unless an aspect of the same type already
//	exists in the custom list. This ensures that essential functionality is always
//	available without requiring explicit configuration.
//
//	这些切面在初始化期间通过 initBuiltinsAspects() 方法自动添加到规则引擎中。
//	如果提供了自定义切面，除非自定义列表中已存在相同类型的切面，否则仍会包含
//	内置切面。这确保基本功能始终可用，无需显式配置。
var BuiltinsAspects = []types.Aspect{&aspect.Validator{}, &aspect.Debug{}, &aspect.MetricsAspect{}}

// aspectsHolder holds the aspects for atomic access to improve performance
// by avoiding lock contention during high-frequency aspect operations.
// aspectsHolder 保存切面以提供原子访问，通过避免高频切面操作期间的锁竞争来提高性能。
type aspectsHolder struct {
	// startAspects are executed before rule chain processing begins
	// startAspects 在规则链处理开始前执行
	startAspects []types.StartAspect
	// endAspects are executed when a rule chain branch ends
	// endAspects 在规则链分支结束时执行
	endAspects []types.EndAspect
	// completedAspects are executed when all rule chain branches complete
	// completedAspects 在所有规则链分支完成时执行
	completedAspects []types.CompletedAspect
}

// RuleEngine is the core structure for a rule engine instance.
// Each RuleEngine instance manages exactly one root rule chain and provides
// the primary interface for message processing and rule execution.
//
// RuleEngine 是规则引擎实例的核心结构。
// 每个 RuleEngine 实例管理恰好一个根规则链，并为消息处理和规则执行提供主要接口。
//
// Architecture & Features:
// 架构和特性：
//   - Single root rule chain management with hot reloading capability
//     单根规则链管理，支持热重载功能
//   - Aspect-oriented programming support for cross-cutting concerns
//     面向切面编程支持，用于横切关注点
//   - Two-phase graceful shutdown for safe resource cleanup
//     两阶段优雅停机，确保安全的资源清理
//   - Deadlock-free reload mechanism with message queuing
//     无死锁重载机制，支持消息队列
//   - Concurrent message processing with atomic operations
//     并发消息处理，使用原子操作
//   - Context-aware processing with shutdown signal integration
//     上下文感知处理，集成停机信号
//   - Sub-rule chain pool integration for nested execution
//     子规则链池集成，支持嵌套执行
//   - Comprehensive metrics and debugging capabilities
//     全面的指标和调试功能
//   - Backpressure control during reload to prevent memory overflow
//     重载期间的背压控制，防止内存溢出
//
// Lifecycle Management:
// 生命周期管理：
//  1. Creation with NewRuleEngine() and rule chain definition
//     使用 NewRuleEngine() 和规则链定义创建
//  2. Message processing via OnMsg() with concurrent safety
//     通过 OnMsg() 进行消息处理，具有并发安全性
//  3. Optional hot reloading with ReloadSelf() without downtime
//     使用 ReloadSelf() 进行可选的热重载，无需停机
//  4. Graceful cleanup with Stop() and proper resource release
//     使用 Stop() 进行优雅清理和适当的资源释放
//
// Thread Safety & Concurrency:
// 线程安全和并发：
//
//	RuleEngine is designed for high-concurrency scenarios with:
//	RuleEngine 设计用于高并发场景，具有：
//	- Lock-free message processing using atomic operations
//	  使用原子操作的无锁消息处理
//	- Safe concurrent access to rule chain definitions
//	  对规则链定义的安全并发访问
//	- Coordinated reload operations without blocking message flow
//	  协调的重载操作，不阻塞消息流
//	- Graceful shutdown handling for all concurrent operations
//	  对所有并发操作的优雅停机处理
//	- Backpressure control to prevent resource exhaustion
//	  背压控制以防止资源耗尽
//
// Memory Safety During Reload:
// 重载期间的内存安全：
//
//	The engine implements sophisticated backpressure mechanisms to prevent
//	memory overflow during reload operations:
//	引擎实现了复杂的背压机制，在重载操作期间防止内存溢出：
//	- Limited concurrent goroutines waiting for reload completion
//	  限制并发等待重载完成的goroutine数量
//	- Fast-fail strategy for excessive reload wait requests
//	  对过量重载等待请求的快速失败策略
//	- Configurable memory protection thresholds
//	  可配置的内存保护阈值
//	- Automatic degradation to reject mode under high load
//	  高负载下自动降级到拒绝模式
//
// This design ensures reliable, high-performance rule processing in production environments.
// 此设计确保在生产环境中可靠的高性能规则处理。
type RuleEngine struct {
	// Embed graceful shutdown functionality
	// 嵌入优雅停机功能
	base.GracefulShutdown

	// Config is the configuration for the rule engine containing
	// global settings, component registry, and execution parameters
	// Config 是规则引擎的配置，包含全局设置、组件注册表和执行参数
	Config types.Config

	// ruleChainPool is a pool of sub-rule engines for handling nested rule chains
	// ruleChainPool 是处理嵌套规则链的子规则引擎池
	ruleChainPool types.RuleEnginePool

	// id is the unique identifier for the rule engine instance
	// id 是规则引擎实例的唯一标识符
	id string

	// rootRuleChainCtx is the context of the root rule chain containing
	// all nodes and their relationships
	// rootRuleChainCtx 是根规则链的上下文，包含所有节点及其关系
	rootRuleChainCtx *RuleChainCtx

	// aspectsPtr provides high-performance atomic access to aspects
	// to avoid lock contention during message processing
	// aspectsPtr 提供对切面的高性能原子访问，以避免消息处理期间的锁竞争
	aspectsPtr unsafe.Pointer

	// initialized indicates whether the rule engine has been properly initialized
	// Use atomic operations to prevent data races during concurrent access
	// initialized 指示规则引擎是否已正确初始化
	// 使用原子操作防止并发访问时的数据竞态
	initialized int32

	// Aspects is a list of AOP (Aspect-Oriented Programming) aspects
	// that provide cross-cutting concerns like logging, validation, and metrics
	// Aspects 是面向切面编程（AOP）切面列表，提供如日志、验证和指标等横切关注点
	Aspects types.AspectList

	// OnUpdated is a callback function triggered when the rule chain is updated
	// OnUpdated 是规则链更新时触发的回调函数
	OnUpdated func(chainId, nodeId string, dsl []byte)

	// Backpressure control fields for memory safety during reload
	// 重载期间内存安全的背压控制字段

	// maxConcurrentReloadWaiters limits the number of goroutines that can wait for reload completion
	// to prevent memory overflow during high-traffic reload operations
	// maxConcurrentReloadWaiters 限制可以等待重载完成的goroutine数量，
	// 以防止高流量重载操作期间的内存溢出
	maxConcurrentReloadWaiters int64

	// currentReloadWaiters tracks the current number of goroutines waiting for reload
	// currentReloadWaiters 跟踪当前等待重载的goroutine数量
	currentReloadWaiters int64

	// reloadBackpressureEnabled enables/disables backpressure control
	// reloadBackpressureEnabled 启用/禁用背压控制
	reloadBackpressureEnabled bool
	reloadLock                sync.Mutex
}

// NewRuleEngine creates a new RuleEngine instance with the given ID and definition.
// It applies the provided RuleEngineOptions during the creation process.
//
// NewRuleEngine 使用给定的 ID 和定义创建新的 RuleEngine 实例。
// 它在创建过程中应用提供的 RuleEngineOptions。
//
// Parameters:
// 参数：
//   - id: Unique identifier for the rule engine (can be empty to use chain ID)
//     规则引擎的唯一标识符（可以为空以使用链 ID）
//   - def: Rule chain definition in JSON or other supported format
//     JSON 或其他支持格式的规则链定义
//   - opts: Optional configuration functions to customize the engine
//     可选的配置函数来自定义引擎
//
// Returns:
// 返回：
//   - *RuleEngine: Initialized rule engine instance  已初始化的规则引擎实例
//   - error: Initialization error if any  如果有的话，初始化错误
//
// The creation process involves:
// 创建过程包括：
//  1. Parsing the rule chain definition  解析规则链定义
//  2. Initializing all components and their relationships  初始化所有组件及其关系
//  3. Setting up aspects and callback functions  设置切面和回调函数
//  4. Validating the configuration  验证配置
//  5. Configuring backpressure control for memory safety  配置背压控制以确保内存安全
func NewRuleEngine(id string, def []byte, opts ...types.RuleEngineOption) (*RuleEngine, error) {
	if len(def) == 0 {
		return nil, errors.New("def can not nil")
	}

	// Create a new RuleEngine with the Id
	// 使用 ID 创建新的 RuleEngine
	ruleEngine := &RuleEngine{
		id:            id,
		Config:        NewConfig(),
		ruleChainPool: DefaultPool,
		// Initialize backpressure control with default values
		// 使用默认值初始化背压控制
		maxConcurrentReloadWaiters: 1000, // Default: allow max 1000 concurrent waiters
		reloadBackpressureEnabled:  true, // Enable backpressure by default
	}

	// Initialize graceful shutdown functionality
	// 初始化优雅停机功能
	if ruleEngine.Config.Logger == nil {
		ruleEngine.Config.Logger = types.DefaultLogger()
	}
	ruleEngine.InitGracefulShutdown(ruleEngine.Config.Logger, 10*time.Second)

	err := ruleEngine.ReloadSelf(def, opts...)
	if err == nil && ruleEngine.rootRuleChainCtx != nil {
		if id != "" {
			ruleEngine.rootRuleChainCtx.Id = types.RuleNodeId{Id: id, Type: types.CHAIN}
		} else {
			// Use the rule chain ID if no ID is provided.
			// 如果没有提供 ID，则使用规则链 ID。
			ruleEngine.id = ruleEngine.rootRuleChainCtx.Id.Id
		}

	}

	return ruleEngine, err
}

// Id returns the unique identifier of the rule engine instance.
// Id 返回规则引擎实例的唯一标识符。
func (e *RuleEngine) Id() string {
	return e.id
}

// SetConfig updates the configuration of the rule engine.
// This should be called before initialization for best results.
// SetConfig 更新规则引擎的配置。
// 为了获得最佳效果，应在初始化前调用。
func (e *RuleEngine) SetConfig(config types.Config) {
	e.Config = config
}

// SetAspects updates the list of aspects used by the rule engine.
// Aspects provide cross-cutting functionality like logging and validation.
// SetAspects 更新规则引擎使用的切面列表。
// 切面提供如日志和验证等横切功能。
func (e *RuleEngine) SetAspects(aspects ...types.Aspect) {
	e.Aspects = aspects
}

// SetRuleEnginePool sets the pool used for managing sub-rule chains.
// This allows for nested rule chain execution and resource sharing.
// SetRuleEnginePool 设置用于管理子规则链的池。
// 这允许嵌套规则链执行和资源共享。
func (e *RuleEngine) SetRuleEnginePool(ruleChainPool types.RuleEnginePool) {
	e.ruleChainPool = ruleChainPool
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.SetRuleEnginePool(ruleChainPool)
	}
}

// GetAspects returns a copy of the current aspects list to avoid data races.
// GetAspects 返回当前切面列表的副本以避免数据竞争。
func (e *RuleEngine) GetAspects() types.AspectList {
	// 返回一个副本以避免数据竞争
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.GetAspects()
	}
	return e.Aspects
}

// Reload reloads the current rule chain with optional new configuration.
// This is a convenience method that uses the current DSL definition.
// Reload 使用可选的新配置重载当前规则链。
// 这是使用当前 DSL 定义的便捷方法。
func (e *RuleEngine) Reload(opts ...types.RuleEngineOption) error {
	return e.ReloadSelf(e.DSL(), opts...)
}

// initBuiltinsAspects initializes the built-in aspects if no custom aspects are provided.
// It ensures that essential aspects like validation and debugging are always available.
// initBuiltinsAspects 如果没有提供自定义切面，则初始化内置切面。
// 它确保验证和调试等基本切面始终可用。
func (e *RuleEngine) initBuiltinsAspects() {
	var newAspects types.AspectList
	//初始化内置切面
	if len(e.Aspects) == 0 {
		for _, builtinsAspect := range BuiltinsAspects {
			newAspects = append(newAspects, builtinsAspect.New())
		}
	} else {
		for _, item := range e.Aspects {
			newAspects = append(newAspects, item.New())
		}

		for _, builtinsAspect := range BuiltinsAspects {
			found := false
			for _, item := range newAspects {
				//判断是否是相同类型
				if reflect.TypeOf(item) == reflect.TypeOf(builtinsAspect) {
					found = true
					break
				}
			}
			if !found {
				newAspects = append(newAspects, builtinsAspect.New())
			}
		}
	}
	e.Aspects = newAspects
}

// initChain initializes the rule chain with the provided definition.
// It sets up all nodes, relationships, and executes creation aspects.
// initChain 使用提供的定义初始化规则链。
// 它设置所有节点、关系并执行创建切面。
func (e *RuleEngine) initChain(def types.RuleChain) error {
	if def.RuleChain.Disabled {
		return types.ErrEngineDisabled
	}
	if ctx, err := InitRuleChainCtx(e.Config, e.Aspects, &def, e.ruleChainPool); err == nil {
		if e.rootRuleChainCtx != nil {
			ctx.Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx
		//执行创建切面逻辑
		_, _, createdAspects, _, _ := e.Aspects.GetEngineAspects()
		for _, aop := range createdAspects {
			if err := aop.OnCreated(e.rootRuleChainCtx); err != nil {
				return err
			}
		}
		atomic.StoreInt32(&e.initialized, 1)
		return nil
	} else {
		return err
	}
}

// ReloadSelf reloads the rule chain with new definition and options.
// This method supports hot reloading of rule configurations without stopping the engine.
// It implements a two-phase graceful reload process:
//
// Phase 1: Preparation (设置阶段)
// - Apply configuration options  应用配置选项
// - Wait for any ongoing reload to complete  等待任何正在进行的重载完成
// - Set reloading state to block new messages  设置重载状态以阻塞新消息
// - Wait for active messages to complete  等待活跃消息完成
//
// Phase 2: Reload (重载阶段)
// - Parse new rule chain definition  解析新的规则链定义
// - Update or create rule chain context  更新或创建规则链上下文
// - Update atomic aspect pointers  更新原子切面指针
// - Resume normal operation  恢复正常运行
//
// ReloadSelf 使用新定义和选项重新加载规则链。
// 此方法支持在不停止引擎的情况下热重载规则配置。
// 它实现了两阶段优雅重载过程：
//
// Parameters:
// 参数：
//   - dsl: Rule chain definition in byte format  字节格式的规则链定义
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - error: Reload error if any  如果有的话，重载错误
func (e *RuleEngine) ReloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	e.reloadLock.Lock()
	defer e.reloadLock.Unlock()
	return e.reloadSelf(dsl, opts...)
}

func (e *RuleEngine) reloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	// 将选项应用于 RuleEngine。
	for _, opt := range opts {
		_ = opt(e)
	}

	// Check if engine is shutting down, if so, reject reload operation
	// 检查引擎是否正在停机，如果是，拒绝重载操作
	if e.IsShuttingDown() {
		return types.ErrEngineShuttingDown
	}

	// Set reloading state to block new messages during reload
	// 设置重载状态以在重载期间阻塞新消息
	if e.Initialized() {
		e.SetReloading(true)
		defer e.SetReloading(false)

		// Wait for active messages to complete before reloading
		// 在重载前等待活跃消息完成
		waitTimeout := 10 * time.Second
		e.WaitForActiveOperations(waitTimeout)
	}

	var err error
	if e.Initialized() {
		//初始化内置切面
		if len(e.Aspects) == 0 {
			e.initBuiltinsAspects()
		}
		e.rootRuleChainCtx.config = e.Config
		e.rootRuleChainCtx.SetAspects(e.Aspects)
		//更新规则链
		err = e.rootRuleChainCtx.ReloadSelf(dsl)
		////设置子规则链池
		//e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, e.id, dsl)
		}
	} else {
		//初始化内置切面
		e.initBuiltinsAspects()
		var rootRuleChainDef types.RuleChain
		//初始化
		if rootRuleChainDef, err = e.Config.Parser.DecodeRuleChain(dsl); err == nil {
			err = e.initChain(rootRuleChainDef)
		} else {
			return err
		}
	}

	// Set the aspect lists.
	// 设置切面列表。
	startAspects, endAspects, completedAspects := e.Aspects.GetChainAspects()
	holder := &aspectsHolder{startAspects: startAspects, endAspects: endAspects, completedAspects: completedAspects}
	atomic.StorePointer(&e.aspectsPtr, unsafe.Pointer(holder))
	return err
}

// waitForReloadComplete waits for any ongoing reload to complete before starting a new one.
// waitForReloadComplete 在开始新的重载前等待任何正在进行的重载完成。
func (e *RuleEngine) waitForReloadComplete() error {
	if e.IsReloading() {
		timeout := 10 * time.Second
		if !e.WaitForReloadComplete(timeout) {
			return types.ErrEngineReloadTimeout
		}
	}
	return nil
}

// ReloadChild updates a specific node within the root rule chain.
// If ruleNodeId is empty, it updates the entire root rule chain.
// It gracefully stops accepting new messages, waits for active messages to complete,
// performs the reload, and then resumes normal operation.
//
// ReloadChild 更新根规则链中的特定节点。
// 如果 ruleNodeId 为空，则更新整个根规则链。
// 它优雅地停止接收新消息，等待活跃消息完成，执行重载，然后恢复正常运行。
//
// Parameters:
// 参数：
//   - ruleNodeId: ID of the node to update (empty for root chain)
//     要更新的节点 ID（根链为空）
//   - dsl: New configuration for the node/chain  节点/链的新配置
//
// Returns:
// 返回：
//   - error: Update error if any  如果有的话，更新错误
func (e *RuleEngine) ReloadChild(ruleNodeId string, dsl []byte) error {
	e.reloadLock.Lock()
	defer e.reloadLock.Unlock()

	if len(dsl) == 0 {
		return types.ErrEngineDslEmpty
	} else if e.rootRuleChainCtx == nil {
		return types.ErrEngineNotInitialized
	} else if e.IsShuttingDown() {
		return types.ErrEngineShuttingDown
	} else if ruleNodeId == "" {
		//更新根规则链
		return e.reloadSelf(dsl)
	} else {
		// Set reloading state to block new messages during reload
		// 设置重载状态以在重载期间阻塞新消息
		e.SetReloading(true)
		defer e.SetReloading(false)

		// Wait for active messages to complete before reloading child node
		// 在重载子节点前等待活跃消息完成
		waitTimeout := 10 * time.Second
		e.WaitForActiveOperations(waitTimeout)

		//更新根规则链子节点
		err := e.rootRuleChainCtx.ReloadChild(types.RuleNodeId{Id: ruleNodeId}, dsl)

		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, ruleNodeId, e.DSL())
		}
		return err
	}
}

// DSL returns the current rule chain configuration in its original format.
// DSL 返回原始格式的当前规则链配置。
func (e *RuleEngine) DSL() []byte {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.DSL()
	} else {
		return nil
	}
}

// Definition returns the rule chain definition structure.
// Definition 返回规则链定义结构。
func (e *RuleEngine) Definition() types.RuleChain {
	if e.rootRuleChainCtx != nil {
		return *e.rootRuleChainCtx.SelfDefinition
	} else {
		return types.RuleChain{}
	}
}

// NodeDSL returns the configuration of a specific node within the rule chain.
// NodeDSL 返回规则链中特定节点的配置。
func (e *RuleEngine) NodeDSL(chainId types.RuleNodeId, childNodeId types.RuleNodeId) []byte {
	if e.rootRuleChainCtx != nil {
		if chainId.Id == "" {
			if node, ok := e.rootRuleChainCtx.GetNodeById(childNodeId); ok {
				return node.DSL()
			}
		} else {
			if node, ok := e.rootRuleChainCtx.GetNodeById(chainId); ok {
				if childNode, ok := node.GetNodeById(childNodeId); ok {
					return childNode.DSL()
				}
			}
		}
	}
	return nil
}

// Initialized returns whether the rule engine has been properly initialized.
// Initialized 返回规则引擎是否已正确初始化。
func (e *RuleEngine) Initialized() bool {
	return atomic.LoadInt32(&e.initialized) == 1 && e.rootRuleChainCtx != nil
}

// RootRuleChainCtx returns the root rule chain context.
// RootRuleChainCtx 返回根规则链上下文。
func (e *RuleEngine) RootRuleChainCtx() types.ChainCtx {
	return e.rootRuleChainCtx
}

// Stop shuts down the rule engine and releases all resources.
// Implements a two-phase graceful shutdown strategy:
//
// Phase 1: Graceful Shutdown (优雅停机阶段)
// - Set shutdown flag to reject new messages  设置停机标志拒绝新消息
// - Wait for all active messages to complete naturally  等待所有活跃消息自然完成
// - Respect the provided context timeout  遵守提供的上下文超时
//
// Phase 2: Force Shutdown (强制停机阶段)
// - If timeout exceeded, cancel contexts to interrupt operations  如果超时，取消上下文以中断操作
// - Give brief time for operations to respond to cancellation  给操作短暂时间响应取消
// - Clean up all resources immediately  立即清理所有资源
//
// Context handling:
// 上下文处理：
// - If ctx is provided with deadline: uses that timeout  如果提供了带截止时间的ctx：使用该超时
// - If ctx is context.Background(): uses default 10s timeout  如果ctx是context.Background()：使用默认10秒超时
// - If ctx is nil: performs immediate shutdown  如果ctx为nil：执行立即停机
//
// Concurrent calls handling:
// 并发调用处理：
//   - If graceful shutdown is already in progress, subsequent calls wait for completion
//     如果优雅停机已在进行中，后续调用等待其完成
//   - Only one graceful shutdown process can execute at a time
//     一次只能执行一个优雅停机过程
//   - No forced interruption of ongoing graceful shutdown
//     不会强制中断正在进行的优雅停机
//
// Stop 关闭规则引擎并释放所有资源。
// 实现两阶段优雅停机策略：
func (e *RuleEngine) Stop(ctx context.Context) {
	// Handle concurrent calls: if already shutting down, wait for completion instead of forcing
	// 处理并发调用：如果已在停机，等待完成而不是强制停机
	if e.IsShuttingDown() {
		// Check if shutdown is already completed by checking if the engine is initialized
		// If the engine is not initialized, shutdown has already completed
		// 检查停机是否已经完成，通过检查引擎是否已初始化
		// 如果引擎未初始化，说明停机已经完成
		if !e.Initialized() {
			// Shutdown has already completed, no need to wait
			// 停机已经完成，无需等待
			return
		}

		// Wait for the ongoing shutdown to complete with a reasonable timeout
		// 等待正在进行的停机完成，设置合理的超时
		shutdownWaitTimeout := 10 * time.Second // Reduced from 30s for better responsiveness
		if ctx != nil {
			if deadline, ok := ctx.Deadline(); ok {
				// Use the remaining time from the provided context, but with a minimum wait time
				// 使用提供的上下文的剩余时间，但设置最小等待时间
				remainingTime := time.Until(deadline)
				if remainingTime > 0 && remainingTime < shutdownWaitTimeout {
					shutdownWaitTimeout = remainingTime
				}
			}
		}

		// Wait for the ongoing shutdown to complete
		// 等待正在进行的停机完成
		ticker := time.NewTicker(50 * time.Millisecond) // More frequent checks for faster response
		defer ticker.Stop()

		waitCtx, cancel := context.WithTimeout(context.Background(), shutdownWaitTimeout)
		defer cancel()

		for {
			select {
			case <-waitCtx.Done():
				// Timeout waiting for shutdown to complete, force cleanup
				// 等待停机完成超时，强制清理
				e.Config.Logger.Printf("Timeout waiting for ongoing shutdown to complete, forcing cleanup")
				e.forceStop()
				return
			case <-ticker.C:
				// Check if shutdown completed by checking initialization status
				// Shutdown is complete when the engine is no longer initialized
				// 通过检查初始化状态来检查停机是否完成
				// 当引擎不再初始化时停机完成
				if !e.Initialized() {
					// Shutdown completed successfully
					// 停机成功完成
					return
				}
			}
		}
	}

	// Calculate timeout from context, handling negative durations explicitly
	// 从上下文计算超时，明确处理负持续时间
	var timeout time.Duration
	var isExpiredContext bool

	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
			if timeout <= 0 {
				// Context deadline has already passed
				// 上下文截止时间已过
				isExpiredContext = true
				e.Config.Logger.Printf("Context deadline has already passed (negative duration: %v), performing immediate shutdown", timeout)
				timeout = 0 // Use immediate shutdown for expired contexts
			}
		} else {
			// Default timeout for context.Background() or contexts without deadline
			// 对于 context.Background() 或没有截止时间的上下文使用默认超时
			timeout = 10 * time.Second
		}
	} else {
		// Immediate shutdown for nil context
		// 对于nil上下文立即停机
		timeout = 0
	}

	// Perform graceful shutdown
	// 执行优雅停机
	e.GracefulShutdown.GracefulStop(func() {
		if isExpiredContext || timeout == 0 {
			// For expired contexts or nil context, skip graceful wait and go straight to cleanup
			// 对于过期上下文或nil上下文，跳过优雅等待直接清理
			e.Config.Logger.Printf("Performing immediate shutdown")
			e.GracefulShutdown.ForceStop()
		} else {
			// Phase 1: Wait for all active messages to complete naturally
			// 第一阶段：等待所有活跃消息自然完成
			allCompleted := e.WaitForActiveOperations(timeout)
			if !allCompleted {
				e.Config.Logger.Printf("Graceful shutdown timeout after %v, forcing context cancellation", timeout)
				// Phase 2: Force cancel context to interrupt ongoing operations
				// 第二阶段：强制取消上下文以中断正在进行的操作
				e.GracefulShutdown.ForceStop()
				// Give a brief moment for operations to respond to cancellation
				// 给操作一个短暂的时间来响应取消
				e.WaitForActiveOperations(500 * time.Millisecond)
			}
		}
		// Clean up resources
		// 清理资源
		e.forceStop()
	})
}

// applyShutdownContext applies graceful shutdown context handling to the rule context copy.
// This method ensures that message processing respects shutdown signals while preserving
// user-provided context functionality.
//
// Context application strategy:
// 上下文应用策略：
//  1. If user hasn't provided custom context: use shutdown context directly
//     如果用户没有提供自定义上下文：直接使用停机上下文
//  2. If user provided custom context: combine both contexts
//     如果用户提供了自定义上下文：组合两个上下文
//     - Preserves user context values and behavior  保留用户上下文值和行为
//     - Adds shutdown cancellation capability  添加停机取消能力
//     - Cancellation triggers when either context is cancelled  任一上下文取消时触发取消
//
// This design ensures both user functionality and graceful shutdown work correctly together.
//
// applyShutdownContext 对规则上下文副本应用优雅停机上下文处理。
// 此方法确保消息处理尊重停机信号，同时保留用户提供的上下文功能。
// applyShutdownContext applies shutdown context handling to the rule context.
// This method now only preserves context values from user context while using shutdown context for cancellation.
// The user context cancellation feature has been removed to prevent goroutine leaks in high concurrency scenarios.
//
// applyShutdownContext 将停机上下文处理应用到规则上下文。
// 此方法现在只保留用户上下文的值，同时使用停机上下文进行取消。
// 已移除用户上下文取消功能以防止高并发场景下的协程泄漏。
func (e *RuleEngine) applyShutdownContext(rootCtxCopy, rootCtx *DefaultRuleContext) {
	shutdownCtx := e.GetShutdownContext()
	if shutdownCtx == nil {
		return
	}

	if rootCtxCopy.GetContext() == rootCtx.GetContext() {
		// No custom context was set by user options, use shutdown context directly
		// 用户选项没有设置自定义上下文，直接使用停机上下文
		rootCtxCopy.SetContext(shutdownCtx)
	} else {
		// User provided custom context, preserve its values but use shutdown context for cancellation
		// This prevents goroutine leaks while maintaining context value inheritance
		// 用户提供了自定义上下文，保留其值但使用停机上下文进行取消
		// 这防止了协程泄漏同时保持上下文值继承
		userCtx := rootCtxCopy.GetContext()
		combinedCtx := e.combineContextsValueOnly(userCtx, shutdownCtx)
		rootCtxCopy.SetContext(combinedCtx)
	}
}

// combineContextsValueOnly creates a context that inherits values from user context
// but uses shutdown context for cancellation only. This prevents goroutine leaks
// that occurred in the previous implementation while preserving context value inheritance.
//
// The returned context:
// 1. Inherits all values from the user context
// 2. Can only be cancelled by the shutdown context (not user context)
// 3. Does not create any monitoring goroutines
//
// combineContextsValueOnly 创建一个从用户上下文继承值但仅使用停机上下文进行取消的上下文。
// 这防止了之前实现中出现的协程泄漏，同时保留了上下文值继承。
//
// 返回的上下文：
// 1. 从用户上下文继承所有值
// 2. 只能被停机上下文取消（不能被用户上下文取消）
// 3. 不创建任何监控协程
func (e *RuleEngine) combineContextsValueOnly(userCtx, shutdownCtx context.Context) context.Context {
	// Create a value-only context that inherits values from user context
	// but uses shutdown context for cancellation
	// 创建一个仅值上下文，从用户上下文继承值但使用停机上下文进行取消
	return &valueOnlyContext{
		Context:  shutdownCtx,
		valueCtx: userCtx,
	}
}

// valueOnlyContext is a context implementation that inherits values from one context
// but uses another context for cancellation and deadlines.
// valueOnlyContext 是一个上下文实现，从一个上下文继承值但使用另一个上下文进行取消和截止时间。
type valueOnlyContext struct {
	context.Context                 // Used for cancellation and deadlines 用于取消和截止时间
	valueCtx        context.Context // Used for values 用于值
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. It first checks the value context,
// then falls back to the cancellation context.
// Value 返回与此上下文中键关联的值，如果没有与键关联的值则返回nil。
// 它首先检查值上下文，然后回退到取消上下文。
func (c *valueOnlyContext) Value(key interface{}) interface{} {
	if val := c.valueCtx.Value(key); val != nil {
		return val
	}
	return c.Context.Value(key)
}

// incrementActiveMessages 增加活跃消息计数
func (e *RuleEngine) incrementActiveMessages() {
	e.IncrementActiveOperations()
}

// decrementActiveMessages 减少活跃消息计数
func (e *RuleEngine) decrementActiveMessages() {
	e.DecrementActiveOperations()
}

// forceStop performs immediate cleanup of all rule engine resources.
// This method is called during shutdown to ensure complete resource cleanup,
// regardless of whether graceful shutdown completed successfully.
//
// Cleanup operations (with panic recovery):
// 清理操作（带恐慌恢复）：
// 1. Force cancellation of graceful shutdown context  强制取消优雅停机上下文
// 2. Destroy rule chain context and all nodes  销毁规则链上下文和所有节点
// 3. Clear instance cache entries  清理实例缓存条目
// 4. Reset initialization state  重置初始化状态
//
// Each cleanup operation is wrapped with panic recovery to ensure
// that failures in one cleanup step don't prevent others from executing.
//
// forceStop 执行规则引擎资源的立即清理。
// 此方法在停机期间调用以确保完整的资源清理，无论优雅停机是否成功完成。
func (e *RuleEngine) forceStop() {
	defer func() {
		if r := recover(); r != nil {
			e.Config.Logger.Printf("RuleEngine.forceStop() panic recovered: %v", r)
		}
	}()

	// Force cancellation of graceful shutdown context
	// 强制取消优雅停机上下文
	e.GracefulShutdown.ForceStop()

	if e.rootRuleChainCtx != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.Config.Logger.Printf("RuleChainCtx.Destroy() panic recovered: %v", r)
				}
			}()
			e.rootRuleChainCtx.Destroy()
		}()
	}

	// 清理实例缓存
	if e.Config.Cache != nil && e.rootRuleChainCtx != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.Config.Logger.Printf("Cache cleanup panic recovered: %v", r)
				}
			}()
			_ = e.Config.Cache.DeleteByPrefix(e.rootRuleChainCtx.GetNodeId().Id + types.NamespaceSeparator)
		}()
	}

	atomic.StoreInt32(&e.initialized, 0)
}

// OnMsg asynchronously processes a message using the rule engine.
// It accepts optional RuleContextOption parameters to customize the execution context.
//
// OnMsg 使用规则引擎异步处理消息。
// 它接受可选的 RuleContextOption 参数来自定义执行上下文。
func (e *RuleEngine) OnMsg(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// OnMsgAndWait synchronously processes a message using the rule engine and waits for all nodes in the rule chain to complete before returning.
// OnMsgAndWait 使用规则引擎同步处理消息，并在返回前等待规则链中的所有节点完成。
func (e *RuleEngine) OnMsgAndWait(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, true, opts...)
}

// RootRuleContext returns the root rule context for advanced operations.
// RootRuleContext 返回用于高级操作的根规则上下文。
func (e *RuleEngine) RootRuleContext() types.RuleContext {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.rootRuleContext
	}
	return nil
}

// GetMetrics returns engine metrics if the metrics aspect is enabled.
// GetMetrics 如果启用了指标切面，则返回引擎指标。
func (e *RuleEngine) GetMetrics() *metrics.EngineMetrics {
	for _, aop := range e.Aspects {
		if metricsAspect, ok := aop.(*aspect.MetricsAspect); ok {
			return metricsAspect.GetMetrics()
		}
	}
	return nil
}

// OnMsgWithEndFunc is a deprecated method that asynchronously processes a message using the rule engine.
// The endFunc callback is used to obtain the results after the rule chain execution is complete.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use OnMsg instead.
//
// OnMsgWithEndFunc 是一个已弃用的方法，使用规则引擎异步处理消息。
// endFunc 回调用于在规则链执行完成后获取结果。
// 注意：如果规则链有多个端点，回调函数将被执行多次。
// 已弃用：请改用 OnMsg。
func (e *RuleEngine) OnMsgWithEndFunc(msg types.RuleMsg, endFunc types.OnEndFunc) {
	e.OnMsg(msg, types.WithOnEnd(endFunc))
}

// OnMsgWithOptions is a deprecated method that asynchronously processes a message using the rule engine.
// It allows carrying context options and an end callback option.
// The context is used for sharing data between different component instances.
// The endFunc callback is used to obtain the results after the rule chain execution is complete.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use OnMsg instead.
//
// OnMsgWithOptions 是一个已弃用的方法，使用规则引擎异步处理消息。
// 它允许携带上下文选项和结束回调选项。
// 上下文用于在不同组件实例之间共享数据。
// endFunc 回调用于在规则链执行完成后获取结果。
// 注意：如果规则链有多个端点，回调函数将被执行多次。
// 已弃用：请改用 OnMsg。
func (e *RuleEngine) OnMsgWithOptions(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// doOnAllNodeCompleted handles the completion of all nodes within the rule chain.
// It executes aspects, completes the run snapshot, and triggers any custom callback functions.
// doOnAllNodeCompleted 处理规则链内所有节点的完成。
// 它执行切面、完成运行快照并触发任何自定义回调函数。
func (e *RuleEngine) doOnAllNodeCompleted(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, customFunc func()) {
	// Execute aspects upon completion of all nodes.
	// 在所有节点完成后执行切面。
	e.onAllNodeCompleted(rootCtxCopy, msg)

	// Complete the run snapshot if it exists.
	// 如果运行快照存在，则完成它。
	if rootCtxCopy.runSnapshot != nil {
		rootCtxCopy.runSnapshot.onRuleChainCompleted(rootCtxCopy)
	}
	// Trigger custom callback if provided.
	// 如果提供了自定义回调，则触发它。
	if customFunc != nil {
		customFunc()
	}
	// 减少活跃消息计数
	e.decrementActiveMessages()
}

// onErrHandler handles the scenario where the rule chain has no nodes or fails to process the message.
// It logs an error and triggers the end-of-chain callbacks.
// onErrHandler 处理规则链没有节点或处理消息失败的场景。
// 它记录错误并触发链结束回调。
func (e *RuleEngine) onErrHandler(msg types.RuleMsg, rootCtxCopy *DefaultRuleContext, err error, needDecrement bool) {
	// Trigger the configured OnEnd callback with the error.
	// 使用错误触发配置的 OnEnd 回调。
	if rootCtxCopy.config.OnEnd != nil {
		rootCtxCopy.config.OnEnd(rootCtxCopy, msg, err, types.Failure)
	}
	// Trigger the onEnd callback with the error and Failure relation type.
	// 使用错误和失败关系类型触发 onEnd 回调。
	if rootCtxCopy.onEnd != nil {
		rootCtxCopy.onEnd(rootCtxCopy, msg, err, types.Failure)
	}
	// Execute the onAllNodeCompleted callback if it exists.
	// 如果存在 onAllNodeCompleted 回调，则执行它。
	if rootCtxCopy.onAllNodeCompleted != nil {
		rootCtxCopy.onAllNodeCompleted()
	}
	// Decrement active messages only if needed (when there was a corresponding increment)
	// 只有在需要时才减少活跃消息计数（当有对应的增加时）
	if needDecrement {
		e.decrementActiveMessages()
	}
}

// onMsgAndWait processes a message through the rule engine with optional waiting for completion.
// This method implements a careful ordering of checks to prevent deadlocks during reload operations.
//
// Processing order to avoid deadlocks:
// 处理顺序以避免死锁：
// 1. Check engine initialization status  检查引擎初始化状态
// 2. Check shutdown status (before incrementing counters)  检查停机状态（在增加计数前）
// 3. Check reload status and wait for completion (before incrementing counters)  检查重载状态并等待完成（在增加计数前）
// 4. Increment active message counter only after state checks  只有在状态检查后才增加活跃消息计数
// 5. Process the message normally  正常处理消息
//
// This ordering prevents the deadlock where:
// 此顺序防止死锁，其中：
//   - Messages wait for reload to complete, but
//     消息等待重载完成，但
//   - Reload waits for active message count to reach zero
//     重载等待活跃消息计数归零
//
// onMsgAndWait 通过规则引擎处理消息，可选择等待完成。
func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	// Check if the rule engine is initialized
	// 检查规则引擎是否已初始化
	if e.rootRuleChainCtx == nil {
		// Handle uninitialized engine error through callback if options are provided
		// 如果提供了选项，通过回调处理未初始化引擎错误
		e.handleEngineNotInitializedError(msg, opts...)
		return
	}

	// Check if engine is shutting down first (before incrementing counter to avoid resource leak)
	// IMPORTANT: Check before incrementing counter to prevent resource leaks
	// 首先检查是否正在停机（在增加计数前检查以避免资源泄漏）
	// 重要：在增加计数前检查以防止资源泄漏
	if e.IsShuttingDown() {
		// Create context and handle shutdown error through callback
		// 创建上下文并通过回调处理停机错误
		rootCtxCopy := e.createRootContextCopy(msg, opts...)
		e.onErrHandler(msg, rootCtxCopy, types.ErrEngineShuttingDown, false)
		return
	}

	// Check if engine is reloading and wait for reload to complete (before incrementing counter)
	// CRITICAL: This prevents deadlock where messages wait for reload completion
	// while reload waits for active message count to reach zero
	// MEMORY SAFETY: Implements backpressure control to prevent memory overflow
	// 检查引擎是否正在重载并等待重载完成（在增加计数前检查）
	// 关键：这防止了消息等待重载完成而重载等待活跃消息计数归零的死锁
	// 内存安全：实现背压控制以防止内存溢出
	if e.IsReloading() {
		// Implement backpressure control to prevent memory overflow during reload
		// 实现背压控制以防止重载期间的内存溢出
		if !e.incrementReloadWaiters() {
			// Backpressure limit reached - reject message to prevent memory overflow
			// 达到背压限制 - 拒绝消息以防止内存溢出
			rootCtxCopy := e.createRootContextCopy(msg, opts...)
			e.Config.Logger.Printf("RuleEngine: %s", types.ErrEngineReloadBackpressureLimit.Error())
			e.onErrHandler(msg, rootCtxCopy, types.ErrEngineReloadBackpressureLimit, false)
			return
		}

		// Ensure we decrement the waiter count when done
		// 确保完成时减少等待者计数
		defer e.decrementReloadWaiters()

		// Wait for reload to complete with timeout
		// 等待重载完成，设置超时
		reloadTimeout := 30 * time.Second
		if !e.WaitForReloadComplete(reloadTimeout) {
			// Reload timeout, handle as error
			// 重载超时，作为错误处理
			rootCtxCopy := e.createRootContextCopy(msg, opts...)
			e.onErrHandler(msg, rootCtxCopy, errors.New("engine reload timeout"), false)
			return
		}
	}

	// Now increment active message count after all state checks pass
	// This ensures the counter is only incremented for messages that will actually be processed
	// 在所有状态检查通过后现在增加活跃消息计数
	// 这确保计数器只为实际将被处理的消息增加
	e.incrementActiveMessages()

	// Double-check shutdown status after incrementing counter to handle race condition
	// If shutdown was initiated between our first check and counter increment,
	// we need to decrement the counter and exit to prevent Stop() from hanging
	// 在增加计数器后再次检查停机状态以处理竞态条件
	// 如果在我们首次检查和计数器增加之间启动了停机，
	// 我们需要减少计数器并退出以防止Stop()挂起
	if e.IsShuttingDown() {
		// Create context and handle shutdown error through callback
		// 创建上下文并通过回调处理停机错误
		rootCtxCopy := e.createRootContextCopy(msg, opts...)
		e.onErrHandler(msg, rootCtxCopy, types.ErrEngineShuttingDown, true)
		return
	}

	// Create root context copy for message processing
	// 创建根上下文副本来处理消息
	rootCtxCopy := e.createRootContextCopy(msg, opts...)

	// Apply graceful shutdown context handling
	// This combines user-provided context with shutdown context for proper cancellation
	// 应用优雅停机上下文处理
	// 这将用户提供的上下文与停机上下文组合以实现正确的取消
	rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
	e.applyShutdownContext(rootCtxCopy, rootCtx)

	// Validate rule chain and context state
	// 验证规则链和上下文状态
	if err := e.validateRuleChainState(rootCtxCopy); err != nil {
		e.onErrHandler(msg, rootCtxCopy, err, true)
		return
	}

	// Execute start aspects
	// 执行开始切面
	processedMsg, err := e.onStart(rootCtxCopy, msg)
	if err != nil {
		e.onErrHandler(msg, rootCtxCopy, err, true)
		return
	}

	// Setup end callback wrapper
	// 设置结束回调包装器
	e.setupEndCallback(rootCtxCopy)

	// Process message with or without waiting
	// 处理消息，可选择是否等待
	e.processMessage(rootCtxCopy, processedMsg, wait)
}

// onStart executes the list of start aspects before the rule chain begins processing a message.
// onStart 在规则链开始处理消息前执行开始切面列表。
// handleEngineNotInitializedError handles the case when the rule engine is not initialized.
// handleEngineNotInitializedError 处理规则引擎未初始化的情况。
func (e *RuleEngine) handleEngineNotInitializedError(msg types.RuleMsg, opts ...types.RuleContextOption) {
	// Extract OnEnd callback from options if provided
	// 从选项中提取 OnEnd 回调（如果提供）
	var onEndCallback types.OnEndFunc
	for _, opt := range opts {
		// Create a temporary context to extract the OnEnd callback
		// 创建临时上下文以提取 OnEnd 回调
		tempCtx := &DefaultRuleContext{}
		opt(tempCtx)
		if tempCtx.onEnd != nil {
			onEndCallback = tempCtx.onEnd
			break
		}
		if tempCtx.config.OnEnd != nil {
			onEndCallback = tempCtx.config.OnEnd
			break
		}
	}

	// Trigger the OnEnd callback with the initialization error if available
	// 如果可用，使用初始化错误触发 OnEnd 回调
	if onEndCallback != nil {
		onEndCallback(nil, msg, types.ErrEngineNotInitialized, types.Failure)
	}
}

// createRootContextCopy creates a copy of the root context for message processing.
// createRootContextCopy 创建根上下文的副本来处理消息。
func (e *RuleEngine) createRootContextCopy(msg types.RuleMsg, opts ...types.RuleContextOption) *DefaultRuleContext {
	rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
	rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.ruleChainPool)
	rootCtxCopy.isFirst = rootCtx.isFirst
	rootCtxCopy.runSnapshot = NewRunSnapshot(msg.Id, rootCtxCopy.ruleChainCtx, time.Now().UnixMilli())

	// Create a new nodeOutputCache instance for current message processing and set cross-node dependencies
	rootCtxCopy.nodeOutputCache.SetCacheableNodes(rootCtx.ruleChainCtx.referencedNodes)

	// Apply the provided options to the context copy
	// 将提供的选项应用于上下文副本
	for _, opt := range opts {
		opt(rootCtxCopy)
	}

	return rootCtxCopy
}

// validateRuleChainState validates the rule chain and context state.
// validateRuleChainState 验证规则链和上下文状态。
func (e *RuleEngine) validateRuleChainState(rootCtxCopy *DefaultRuleContext) error {
	// Check if the rule chain has no nodes
	// 检查规则链是否没有节点
	if rootCtxCopy.ruleChainCtx.isEmpty {
		return types.ErrRuleChainHasNoNodes
	}

	// Check if there's an error in the context
	// 检查上下文中是否有错误
	if rootCtxCopy.err != nil {
		return rootCtxCopy.err
	}

	return nil
}

// setupEndCallback sets up the end callback wrapper for the context.
// setupEndCallback 为上下文设置结束回调包装器。
func (e *RuleEngine) setupEndCallback(rootCtxCopy *DefaultRuleContext) {
	customOnEndFunc := rootCtxCopy.onEnd
	rootCtxCopy.onEnd = func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		// Execute end aspects and update the message accordingly
		// 执行结束切面并相应地更新消息
		msg = e.onEnd(rootCtxCopy, msg, err, relationType)
		// Trigger the custom end callback if provided
		// 如果提供了自定义结束回调，则触发它
		if customOnEndFunc != nil {
			customOnEndFunc(ctx, msg, err, relationType)
		}
	}
}

// processMessage processes the message through the rule chain with optional waiting.
// processMessage 通过规则链处理消息，可选择等待。
func (e *RuleEngine) processMessage(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, wait bool) {
	// Set up a custom function to be called upon completion of all nodes
	// 设置在所有节点完成时要调用的自定义函数
	customFunc := rootCtxCopy.onAllNodeCompleted

	if wait {
		// If waiting is required, set up a channel to synchronize the completion
		// 如果需要等待，设置通道来同步完成
		c := make(chan struct{})
		rootCtxCopy.onAllNodeCompleted = func() {
			defer close(c)
			// Execute the completion handling function
			// 执行完成处理函数
			e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
		}
		// Process the message through the rule chain
		// 通过规则链处理消息
		rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
		// Block until all nodes have completed
		// 阻塞直到所有节点完成
		<-c
	} else {
		// If not waiting, simply set the completion handling function
		// 如果不等待，只需设置完成处理函数
		rootCtxCopy.onAllNodeCompleted = func() {
			e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
		}
		// Process the message through the rule chain
		// 通过规则链处理消息
		rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
	}
}

func (e *RuleEngine) onStart(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	var err error
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.startAspects {
			if aop.PointCut(ctx, msg, "") {
				if err != nil {
					return msg, err
				}
				msg, err = aop.Start(ctx, msg)
			}
		}
	}
	return msg, err
}

// onEnd executes the list of end aspects when a branch of the rule chain ends.
// onEnd 在规则链分支结束时执行结束切面列表。
func (e *RuleEngine) onEnd(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.endAspects {
			if aop.PointCut(ctx, msg, relationType) {
				msg = aop.End(ctx, msg, err, relationType)
			}
		}
	}
	return msg
}

// onAllNodeCompleted executes the list of completed aspects after all branches of the rule chain have ended.
// onAllNodeCompleted 在规则链的所有分支结束后执行完成切面列表。
func (e *RuleEngine) onAllNodeCompleted(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.completedAspects {
			if aop.PointCut(ctx, msg, "") {
				msg = aop.Completed(ctx, msg)
			}
		}
	}
	return msg
}

// getAspectsHolder safely retrieves the aspects holder with high performance
// using atomic operations to avoid lock contention.
// getAspectsHolder 使用原子操作安全地获取切面持有者，以避免锁竞争的高性能方法。
func (e *RuleEngine) getAspectsHolder() *aspectsHolder {
	ptr := atomic.LoadPointer(&e.aspectsPtr)
	if ptr == nil {
		return nil
	}
	return (*aspectsHolder)(ptr)
}

// NewConfig creates a new Config and applies the options.
// It initializes all necessary components with sensible defaults.
//
// NewConfig 创建新的配置并应用选项。
// 它使用合理的默认值初始化所有必要的组件。
//
// Parameters:
// 参数：
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - types.Config: Initialized configuration  已初始化的配置
//
// Default components include:
// 默认组件包括：
//   - JSON parser for rule chain definitions  规则链定义的 JSON 解析器
//   - Default component registry with built-in components  包含内置组件的默认组件注册表
//   - User-defined functions registry  用户定义函数注册表
//   - Default cache implementation  默认缓存实现
func NewConfig(opts ...types.Option) types.Config {
	c := types.NewConfig(opts...)
	if c.Parser == nil {
		c.Parser = &JsonParser{}
	}
	if c.ComponentsRegistry == nil {
		c.ComponentsRegistry = Registry
	}
	// register all udfs
	// 注册所有用户定义函数
	for name, f := range funcs.ScriptFunc.GetAll() {
		c.RegisterUdf(name, f)
	}
	if c.Cache == nil {
		c.Cache = cache.DefaultCache
	}
	return c
}

// WithConfig is an option that sets the Config of the RuleEngine.
// WithConfig 是设置 RuleEngine 配置的选项。
func WithConfig(config types.Config) types.RuleEngineOption {
	return func(re types.RuleEngine) error {
		re.SetConfig(config)
		return nil
	}
}

// SetMaxReloadWaiters configures the maximum number of concurrent goroutines
// that can wait for reload completion. This prevents memory overflow during
// high-traffic reload scenarios.
//
// SetMaxReloadWaiters 配置可以等待重载完成的最大并发 goroutine 数量。
// 这防止高流量重载场景下的内存溢出。
//
// Parameters:
// 参数：
//   - maxWaiters: Maximum number of concurrent goroutines allowed to wait
//     If 0, disables the limit (unlimited waiters)
//     If negative, keeps current setting unchanged
//     maxWaiters: 允许等待的最大并发 goroutine 数量
//     如果为 0，禁用限制（无限等待者）
//     如果为负数，保持当前设置不变
//
// Thread Safety:
// 线程安全：
//
//	This method is thread-safe and can be called during message processing.
//	此方法是线程安全的，可以在消息处理期间调用。
func (e *RuleEngine) SetMaxReloadWaiters(maxWaiters int64) {
	if maxWaiters < 0 {
		// Keep current setting unchanged for negative values
		// 负数时保持当前设置不变
		return
	} else if maxWaiters == 0 {
		// Disable backpressure control (unlimited waiters)
		// 禁用背压控制（无限等待者）
		e.reloadBackpressureEnabled = false
		atomic.StoreInt64(&e.maxConcurrentReloadWaiters, 0)
	} else {
		// Enable backpressure control with specified limit
		// 启用背压控制并设置指定限制
		e.reloadBackpressureEnabled = true
		atomic.StoreInt64(&e.maxConcurrentReloadWaiters, maxWaiters)
	}
}

// GetReloadWaitersStats returns current reload waiters statistics for monitoring.
// This provides insight into reload behavior under load.
//
// GetReloadWaitersStats 返回当前重载等待者统计信息用于监控。
// 这提供了负载下重载行为的洞察。
//
// Returns:
// 返回：
//   - maxWaiters: Maximum allowed concurrent waiters (0 means unlimited)
//     maxWaiters: 最大允许的并发等待者（0 表示无限制）
//   - currentWaiters: Current number of goroutines waiting for reload
//     currentWaiters: 当前等待重载的 goroutine 数量
//   - isReloading: Whether engine is currently reloading
//     isReloading: 引擎当前是否正在重载
func (e *RuleEngine) GetReloadWaitersStats() (maxWaiters int64, currentWaiters int64, isReloading bool) {
	if !e.reloadBackpressureEnabled {
		return 0, atomic.LoadInt64(&e.currentReloadWaiters), e.IsReloading()
	}
	return atomic.LoadInt64(&e.maxConcurrentReloadWaiters),
		atomic.LoadInt64(&e.currentReloadWaiters),
		e.IsReloading()
}

// incrementReloadWaiters atomically increments the reload waiter count.
// Returns false if the increment would exceed the maximum allowed waiters.
//
// incrementReloadWaiters 原子地增加重载等待者计数。
// 如果增加会超过最大允许的等待者数量，则返回false。
func (e *RuleEngine) incrementReloadWaiters() bool {
	if !e.reloadBackpressureEnabled {
		return true // No limit when backpressure is disabled
	}

	maxWaiters := atomic.LoadInt64(&e.maxConcurrentReloadWaiters)
	for {
		current := atomic.LoadInt64(&e.currentReloadWaiters)
		if current >= maxWaiters {
			return false // Would exceed maximum
		}
		if atomic.CompareAndSwapInt64(&e.currentReloadWaiters, current, current+1) {
			return true
		}
		// Retry if another goroutine modified the counter
	}
}

// decrementReloadWaiters atomically decrements the reload waiter count.
// decrementReloadWaiters 原子地减少重载等待者计数。
func (e *RuleEngine) decrementReloadWaiters() {
	if e.reloadBackpressureEnabled {
		atomic.AddInt64(&e.currentReloadWaiters, -1)
	}
}
