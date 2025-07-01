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
	"errors"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/rulego/rulego/api/types/metrics"
	"github.com/rulego/rulego/utils/cache"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/builtin/funcs"
)

// Ensuring RuleEngine implements types.RuleEngine interface.
var _ types.RuleEngine = (*RuleEngine)(nil)

// ErrDisabled is returned when attempting to use a disabled rule chain.
// ErrDisabled 在尝试使用已禁用的规则链时返回。
var ErrDisabled = errors.New("the rule chain has been disabled")

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
// 每个 RuleEngine 实例管理恰好一个根规则链，并为消息处理和规则执行
// 提供主要接口。
//
// Key Features:
// 主要特性：
//   - Single root rule chain management  单根规则链管理
//   - Aspect-oriented programming support  面向切面编程支持
//   - Hot reloading of rule configurations  规则配置的热重载
//   - Concurrent message processing  并发消息处理
//   - Metrics and debugging capabilities  指标和调试功能
//   - Sub-rule chain pool integration  子规则链池集成
//
// Lifecycle:
// 生命周期：
//  1. Creation with NewRuleEngine()  使用 NewRuleEngine() 创建
//  2. Initialization with rule chain definition  使用规则链定义初始化
//  3. Message processing via OnMsg()  通过 OnMsg() 进行消息处理
//  4. Optional reloading with ReloadSelf()  使用 ReloadSelf() 可选重载
//  5. Cleanup with Stop()  使用 Stop() 清理
//
// Thread Safety:
// 线程安全：
//
//	RuleEngine is designed for concurrent access. Message processing
//	is thread-safe and multiple messages can be processed simultaneously.
//	However, reloading operations should be coordinated carefully.
//
//	RuleEngine 设计为并发访问。消息处理是线程安全的，
//	可以同时处理多个消息。但是，重载操作应该仔细协调。
type RuleEngine struct {
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
	// initialized 指示规则引擎是否已正确初始化
	initialized bool

	// Aspects is a list of AOP (Aspect-Oriented Programming) aspects
	// that provide cross-cutting concerns like logging, validation, and metrics
	// Aspects 是面向切面编程（AOP）切面列表，提供如日志、验证和指标等横切关注点
	Aspects types.AspectList

	// OnUpdated is a callback function triggered when the rule chain is updated
	// OnUpdated 是规则链更新时触发的回调函数
	OnUpdated func(chainId, nodeId string, dsl []byte)
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
	}
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
		return ErrDisabled
	}
	if ctx, err := InitRuleChainCtx(e.Config, e.Aspects, &def); err == nil {
		if e.rootRuleChainCtx != nil {
			ctx.Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
		//执行创建切面逻辑
		_, _, createdAspects, _, _ := e.Aspects.GetEngineAspects()
		for _, aop := range createdAspects {
			if err := aop.OnCreated(e.rootRuleChainCtx); err != nil {
				return err
			}
		}
		e.initialized = true
		return nil
	} else {
		return err
	}
}

// ReloadSelf reloads the rule chain with new definition and options.
// This method supports hot reloading of rule configurations without stopping the engine.
//
// ReloadSelf 使用新定义和选项重新加载规则链。
// 此方法支持在不停止引擎的情况下热重载规则配置。
//
// Parameters:
// 参数：
//   - dsl: Rule chain definition in byte format  字节格式的规则链定义
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - error: Reload error if any  如果有的话，重载错误
//
// The reload process:
// 重载过程：
//  1. Apply configuration options  应用配置选项
//  2. Initialize or update aspects  初始化或更新切面
//  3. Parse new rule chain definition  解析新的规则链定义
//  4. Update or create rule chain context  更新或创建规则链上下文
//  5. Update atomic aspect pointers  更新原子切面指针
func (e *RuleEngine) ReloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	// 将选项应用于 RuleEngine。
	for _, opt := range opts {
		_ = opt(e)
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
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
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

// ReloadChild updates a specific node within the root rule chain.
// If ruleNodeId is empty, it updates the entire root rule chain.
//
// ReloadChild 更新根规则链中的特定节点。
// 如果 ruleNodeId 为空，则更新整个根规则链。
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
	if len(dsl) == 0 {
		return errors.New("dsl can not empty")
	} else if e.rootRuleChainCtx == nil {
		return errors.New("ReloadNode error.RuleEngine not initialized")
	} else if ruleNodeId == "" {
		//更新根规则链
		return e.ReloadSelf(dsl)
	} else {
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
	return e.initialized && e.rootRuleChainCtx != nil
}

// RootRuleChainCtx returns the root rule chain context.
// RootRuleChainCtx 返回根规则链上下文。
func (e *RuleEngine) RootRuleChainCtx() types.ChainCtx {
	return e.rootRuleChainCtx
}

// Stop gracefully shuts down the rule engine, cleaning up all resources.
// Stop 优雅地关闭规则引擎，清理所有资源。
func (e *RuleEngine) Stop() {
	defer func() {
		if r := recover(); r != nil {
			e.Config.Logger.Printf("RuleEngine.Stop() panic recovered: %v", r)
		}
	}()

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

	e.initialized = false
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
}

// onErrHandler handles the scenario where the rule chain has no nodes or fails to process the message.
// It logs an error and triggers the end-of-chain callbacks.
// onErrHandler 处理规则链没有节点或处理消息失败的场景。
// 它记录错误并触发链结束回调。
func (e *RuleEngine) onErrHandler(msg types.RuleMsg, rootCtxCopy *DefaultRuleContext, err error) {
	// Trigger the configured OnEnd callback with the error.
	// 使用错误触发配置的 OnEnd 回调。
	if rootCtxCopy.config.OnEnd != nil {
		rootCtxCopy.config.OnEnd(msg, err)
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
}

// onMsgAndWait processes a message through the rule engine, optionally waiting for all nodes to complete.
// It applies any provided RuleContextOptions to customize the execution context.
// onMsgAndWait 通过规则引擎处理消息，可选择等待所有节点完成。
// 它应用任何提供的 RuleContextOptions 来自定义执行上下文。
func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	if e.rootRuleChainCtx != nil {
		// Create a copy of the root context for processing the message.
		// 创建根上下文的副本来处理消息。
		rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
		rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.ruleChainPool)
		rootCtxCopy.isFirst = rootCtx.isFirst
		rootCtxCopy.runSnapshot = NewRunSnapshot(msg.Id, rootCtxCopy.ruleChainCtx, time.Now().UnixMilli())
		// Apply the provided options to the context copy.
		// 将提供的选项应用于上下文副本。
		for _, opt := range opts {
			opt(rootCtxCopy)
		}
		// Handle the case where the rule chain has no nodes.
		// 处理规则链没有节点的情况。
		if rootCtxCopy.ruleChainCtx.isEmpty {
			e.onErrHandler(msg, rootCtxCopy, errors.New("the rule chain has no nodes"))
			return
		}
		if rootCtxCopy.err != nil {
			e.onErrHandler(msg, rootCtxCopy, rootCtxCopy.err)
			return
		}
		var err error
		// Execute start aspects and update the message accordingly.
		// 执行开始切面并相应地更新消息。
		msg, err = e.onStart(rootCtxCopy, msg)
		if err != nil {
			e.onErrHandler(msg, rootCtxCopy, err)
			return
		}
		// Set up a custom end callback function.
		// 设置自定义结束回调函数。
		customOnEndFunc := rootCtxCopy.onEnd
		rootCtxCopy.onEnd = func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// Execute end aspects and update the message accordingly.
			// 执行结束切面并相应地更新消息。
			msg = e.onEnd(rootCtxCopy, msg, err, relationType)
			// Trigger the custom end callback if provided.
			// 如果提供了自定义结束回调，则触发它。
			if customOnEndFunc != nil {
				customOnEndFunc(ctx, msg, err, relationType)
			}

		}
		// Set up a custom function to be called upon completion of all nodes.
		// 设置在所有节点完成时要调用的自定义函数。
		customFunc := rootCtxCopy.onAllNodeCompleted
		// If waiting is required, set up a channel to synchronize the completion.
		// 如果需要等待，设置通道来同步完成。
		if wait {
			c := make(chan struct{})
			rootCtxCopy.onAllNodeCompleted = func() {
				defer close(c)
				// Execute the completion handling function.
				// 执行完成处理函数。
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			// Process the message through the rule chain.
			// 通过规则链处理消息。
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
			// Block until all nodes have completed.
			// 阻塞直到所有节点完成。
			<-c
		} else {
			// If not waiting, simply set the completion handling function.
			// 如果不等待，只需设置完成处理函数。
			rootCtxCopy.onAllNodeCompleted = func() {
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			// Process the message through the rule chain.
			// 通过规则链处理消息。
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
		}

	} else {
		// Log an error if the rule engine is not initialized or the root rule chain is not defined.
		// 如果规则引擎未初始化或根规则链未定义，则记录错误。
		e.Config.Logger.Printf("onMsg error.RuleEngine not initialized")
	}
}

// onStart executes the list of start aspects before the rule chain begins processing a message.
// onStart 在规则链开始处理消息前执行开始切面列表。
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
