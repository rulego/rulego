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

package types

import (
	"sort"
)

// Aspect defines the base interface for implementing Aspect-Oriented Programming (AOP) in RuleGo.
// AOP provides cross-cutting functionality that can intercept and enhance rule chain execution
// without modifying the original business logic of components.
//
// Aspect 定义在 RuleGo 中实现面向切面编程（AOP）的基础接口。
// AOP 提供横切功能，可以拦截和增强规则链执行而不修改组件的原始业务逻辑。
//
// Engine Instance Level:
// 引擎实例级别：
//
// Aspects are registered at the rule engine level and each engine instance gets its own
// aspect instances through the New() method during initialization. This ensures proper
// isolation between different rule engine instances.
//
// 切面在规则引擎级别注册，每个引擎实例在初始化期间通过 New() 方法获得自己的
// 切面实例。这确保了不同规则引擎实例之间的适当隔离。
//
// Aspect Categories:
// 切面类别：
//
//   - Engine Lifecycle Aspects: OnChainBeforeInit, OnNodeBeforeInit, OnCreated, OnReload, OnDestroy
//     引擎生命周期切面：OnChainBeforeInit、OnNodeBeforeInit、OnCreated、OnReload、OnDestroy
//   - Chain Execution Aspects: Start, End, Completed
//     链执行切面：Start、End、Completed
//   - Node Execution Aspects: Before, After, Around
//     节点执行切面：Before、After、Around
//
// Execution Order:
// 执行顺序：
//
//  1. Engine Level (during rule engine operations):
//     引擎级别（规则引擎操作期间）：
//     OnChainBeforeInit -> OnNodeBeforeInit -> OnCreated -> OnReload -> OnDestroy
//
//  2. Chain Level (for each message processing):
//     链级别（每次消息处理）：
//     Start (onStart) -> [Node Processing] -> End (onEnd) -> Completed (onAllNodeCompleted)
//
//  3. Node Level (for each node execution in rule_context.go executeAroundAop):
//     节点级别（rule_context.go executeAroundAop 中每个节点执行）：
//     Before -> Around -> [Node.OnMsg] -> After
//
// Built-in Aspects:
// 内置切面：
//
// RuleGo includes built-in aspects that are automatically registered:
// RuleGo 包含自动注册的内置切面：
//   - Validator: Data validation and schema checking
//     Validator：数据验证和模式检查
//   - Debug: Debug information collection and logging
//     Debug：调试信息收集和日志记录
//   - MetricsAspect: Performance metrics and monitoring
//     MetricsAspect：性能指标和监控
type Aspect interface {
	// Order returns the execution priority of the aspect.
	// Lower values indicate higher priority and earlier execution in the aspect chain.
	//
	// Order 返回切面的执行优先级。
	// 较小的值表示更高的优先级和在切面链中更早的执行。
	//
	// Returns:
	// 返回：
	//   - int: Priority value, lower numbers execute first
	//     int：优先级值，较小的数字先执行
	Order() int

	// New creates a new instance of the aspect for a specific rule engine instance.
	// This method is called during rule engine initialization (in initBuiltinsAspects and initChain)
	// to ensure each rule engine has its own aspect instance with isolated state.
	//
	// New 为特定的规则引擎实例创建切面的新实例。
	// 此方法在规则引擎初始化期间调用（在 initBuiltinsAspects 和 initChain 中），
	// 确保每个规则引擎都有自己的切面实例和隔离的状态。
	//
	// Implementation Requirements:
	// 实现要求：
	//   - Create a completely independent instance
	//     创建完全独立的实例
	//   - Copy necessary configuration
	//     复制必要的配置
	//   - Ensure no shared mutable state between instances
	//     确保实例之间没有共享的可变状态
	//
	// Returns:
	// 返回：
	//   - Aspect: New aspect instance for the rule engine
	//     Aspect：规则引擎的新切面实例
	New() Aspect
}

// NodeAspect defines the base interface for aspects that operate at the individual node level.
// These aspects can intercept and modify the execution of specific nodes based on PointCut criteria.
//
// NodeAspect 定义在单个节点级别操作的切面的基础接口。
// 这些切面可以基于 PointCut 条件拦截和修改特定节点的执行。
//
// Node aspects are executed during message processing through nodes and provide
// fine-grained control over individual node behavior.
//
// 节点切面在消息通过节点处理期间执行，提供对单个节点行为的细粒度控制。
type NodeAspect interface {
	Aspect

	// PointCut determines whether this aspect should be applied to a specific node execution.
	// This method enables selective aspect application based on runtime conditions.
	//
	// PointCut 确定此切面是否应应用于特定的节点执行。
	// 此方法基于运行时条件启用选择性切面应用。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Message being processed
	//     msg：正在处理的消息
	//   - relationType: Connection type between nodes
	//     relationType：节点间的连接类型
	//
	// Returns:
	// 返回：
	//   - bool: true to apply aspect, false to skip
	//     bool：true 应用切面，false 跳过
	PointCut(ctx RuleContext, msg RuleMsg, relationType string) bool
}

// BeforeAspect defines the interface for aspects that execute before node message processing.
// These aspects are executed in rule_context.go executeAroundAop() before the node's OnMsg method.
//
// BeforeAspect 定义在节点消息处理之前执行的切面接口。
// 这些切面在 rule_context.go executeAroundAop() 中节点的 OnMsg 方法之前执行。
//
// Execution Flow:
// 执行流程：
//  1. Message arrives at node
//     消息到达节点
//  2. BeforeAspect.Before() is called
//     调用 BeforeAspect.Before()
//  3. Modified message is passed to node OnMsg()
//     修改后的消息传递给节点 OnMsg()
type BeforeAspect interface {
	NodeAspect

	// Before is executed before the node's OnMsg method processes the message.
	// The returned message will be used as input for the node's OnMsg method.
	//
	// Before 在节点的 OnMsg 方法处理消息之前执行。
	// 返回的消息将用作节点 OnMsg 方法的输入。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Original message to be processed
	//     msg：要处理的原始消息
	//   - relationType: Connection type that led to this node execution
	//     relationType：导致此节点执行的连接类型
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Modified message for node processing
	//     RuleMsg：用于节点处理的修改后消息
	Before(ctx RuleContext, msg RuleMsg, relationType string) RuleMsg
}

// AfterAspect defines the interface for aspects that execute after node message processing.
// These aspects are executed in rule_context.go executeAfterAop() after the node's OnMsg method.
//
// AfterAspect 定义在节点消息处理之后执行的切面接口。
// 这些切面在 rule_context.go executeAfterAop() 中节点的 OnMsg 方法之后执行。
//
// Execution Flow:
// 执行流程：
//  1. Node processes message with OnMsg()
//     节点使用 OnMsg() 处理消息
//  2. AfterAspect.After() is called with result/error
//     使用结果/错误调用 AfterAspect.After()
//  3. Modified message is passed to next node
//     修改后的消息传递给下一个节点
type AfterAspect interface {
	NodeAspect

	// After is executed after the node's OnMsg method completes processing.
	// The returned message will be used for subsequent processing.
	//
	// After 在节点的 OnMsg 方法完成处理后执行。
	// 返回的消息将用于后续处理。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Message that was processed by the node
	//     msg：被节点处理的消息
	//   - err: Error returned by the node processing, nil if successful
	//     err：节点处理返回的错误，成功时为 nil
	//   - relationType: Connection type for the next node execution
	//     relationType：下一个节点执行的连接类型
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Modified message for next processing
	//     RuleMsg：用于下一步处理的修改后消息
	After(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// AroundAspect defines the interface for aspects that wrap around node message processing.
// These aspects are executed in rule_context.go executeAroundAop() and provide complete control
// over whether the node's OnMsg method is executed.
//
// AroundAspect 定义包装节点消息处理的切面接口。
// 这些切面在 rule_context.go executeAroundAop() 中执行，提供对节点的 OnMsg 方法
// 是否执行的完全控制。
//
// Execution Control:
// 执行控制：
//   - Return (msg, true): Engine will call node's OnMsg method
//     返回 (msg, true)：引擎将调用节点的 OnMsg 方法
//   - Return (msg, false): Engine will skip node's OnMsg method
//     返回 (msg, false)：引擎将跳过节点的 OnMsg 方法
type AroundAspect interface {
	NodeAspect

	// Around wraps the node's OnMsg method execution, providing complete control over whether
	// and how the node executes. Based on rule_context.go executeAroundAop() implementation.
	//
	// Around 包装节点的 OnMsg 方法执行，提供对节点是否以及如何执行的完全控制。
	// 基于 rule_context.go executeAroundAop() 实现。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Message to be processed
	//     msg：要处理的消息
	//   - relationType: Connection type that led to this node execution
	//     relationType：导致此节点执行的连接类型
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Message after aspect processing
	//     RuleMsg：切面处理后的消息
	//   - bool: true to allow engine to call node's OnMsg, false to skip node execution
	//     bool：true 允许引擎调用节点的 OnMsg，false 跳过节点执行
	//
	// Note: Currently, the message return value cannot affect the next node's input parameters.
	// 注意：目前，消息返回值无法影响下一个节点的输入参数。
	Around(ctx RuleContext, msg RuleMsg, relationType string) (RuleMsg, bool)
}

// StartAspect defines the interface for aspects executed before rule chain message processing.
// These aspects are called in engine.go onStart() method before any node processing begins.
//
// StartAspect 定义在规则链消息处理之前执行的切面接口。
// 这些切面在 engine.go onStart() 方法中在任何节点处理开始之前调用。
type StartAspect interface {
	NodeAspect

	// Start is executed before the rule chain processes the message.
	// Called in engine.go onStart() method with aspectsHolder.startAspects.
	//
	// Start 在规则链处理消息之前执行。
	// 在 engine.go onStart() 方法中使用 aspectsHolder.startAspects 调用。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Message to be processed by the rule chain
	//     msg：规则链要处理的消息
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Modified message for rule chain processing
	//     RuleMsg：用于规则链处理的修改后消息
	//   - error: Error to terminate execution, nil to continue
	//     error：终止执行的错误，nil 表示继续
	Start(ctx RuleContext, msg RuleMsg) (RuleMsg, error)
}

// EndAspect defines the interface for aspects executed when a rule chain branch ends.
// These aspects are called in engine.go onEnd() method when a branch of execution completes.
//
// EndAspect 定义在规则链分支结束时执行的切面接口。
// 这些切面在 engine.go onEnd() 方法中当执行分支完成时调用。
type EndAspect interface {
	NodeAspect

	// End is executed when a branch of the rule chain execution ends.
	// Called in engine.go onEnd() method with aspectsHolder.endAspects.
	//
	// End 在规则链执行的分支结束时执行。
	// 在 engine.go onEnd() 方法中使用 aspectsHolder.endAspects 调用。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Message at the end of branch execution
	//     msg：分支执行结束时的消息
	//   - err: Error from branch execution, nil if successful
	//     err：分支执行的错误，成功时为 nil
	//   - relationType: Final relation type of the branch
	//     relationType：分支的最终关系类型
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Modified message for subsequent processing
	//     RuleMsg：用于后续处理的修改后消息
	End(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// CompletedAspect defines the interface for aspects executed when all rule chain branches complete.
// These aspects are called in engine.go onAllNodeCompleted() method when all branches finish.
//
// CompletedAspect 定义在所有规则链分支完成时执行的切面接口。
// 这些切面在 engine.go onAllNodeCompleted() 方法中当所有分支完成时调用。
type CompletedAspect interface {
	NodeAspect

	// Completed is executed when all branches of the rule chain execution complete.
	// Called in engine.go onAllNodeCompleted() method with aspectsHolder.completedAspects.
	//
	// Completed 在规则链执行的所有分支完成时执行。
	// 在 engine.go onAllNodeCompleted() 方法中使用 aspectsHolder.completedAspects 调用。
	//
	// Parameters:
	// 参数：
	//   - ctx: Rule execution context
	//     ctx：规则执行上下文
	//   - msg: Final message after all processing
	//     msg：所有处理后的最终消息
	//
	// Returns:
	// 返回：
	//   - RuleMsg: Modified message for final processing
	//     RuleMsg：用于最终处理的修改后消息
	Completed(ctx RuleContext, msg RuleMsg) RuleMsg
}

// OnChainBeforeInitAspect defines the interface for aspects executed before rule chain initialization.
// These aspects are called in engine.go initChain() method before the rule chain is created.
//
// OnChainBeforeInitAspect 定义在规则链初始化之前执行的切面接口。
// 这些切面在 engine.go initChain() 方法中规则链创建之前调用。
type OnChainBeforeInitAspect interface {
	Aspect

	// OnChainBeforeInit is executed before rule chain initialization.
	// If an error is returned, the chain creation will fail.
	//
	// OnChainBeforeInit 在规则链初始化之前执行。
	// 如果返回错误，链创建将失败。
	//
	// Parameters:
	// 参数：
	//   - config: Rule engine configuration
	//     config：规则引擎配置
	//   - def: Rule chain definition to be initialized
	//     def：要初始化的规则链定义
	//
	// Returns:
	// 返回：
	//   - error: Error to prevent chain creation, nil to continue
	//     error：阻止链创建的错误，nil 表示继续
	OnChainBeforeInit(config Config, def *RuleChain) error
}

// OnNodeBeforeInitAspect defines the interface for aspects executed before rule node initialization.
// These aspects are called during node initialization in the rule chain setup process.
//
// OnNodeBeforeInitAspect 定义在规则节点初始化之前执行的切面接口。
// 这些切面在规则链设置过程中的节点初始化期间调用。
type OnNodeBeforeInitAspect interface {
	Aspect

	// OnNodeBeforeInit is executed before rule node initialization.
	// If an error is returned, the node creation will fail.
	//
	// OnNodeBeforeInit 在规则节点初始化之前执行。
	// 如果返回错误，节点创建将失败。
	//
	// Parameters:
	// 参数：
	//   - config: Rule engine configuration
	//     config：规则引擎配置
	//   - def: Rule node definition to be initialized
	//     def：要初始化的规则节点定义
	//
	// Returns:
	// 返回：
	//   - error: Error to prevent node creation, nil to continue
	//     error：阻止节点创建的错误，nil 表示继续
	OnNodeBeforeInit(config Config, def *RuleNode) error
}

// OnCreatedAspect defines the interface for aspects executed after successful rule engine creation.
// These aspects are called in engine.go initChain() method after the rule chain is successfully created.
//
// OnCreatedAspect 定义在规则引擎成功创建后执行的切面接口。
// 这些切面在 engine.go initChain() 方法中规则链成功创建后调用。
type OnCreatedAspect interface {
	Aspect

	// OnCreated is executed after the rule engine is successfully created.
	// Called in engine.go initChain() with createdAspects from GetEngineAspects().
	//
	// OnCreated 在规则引擎成功创建后执行。
	// 在 engine.go initChain() 中使用来自 GetEngineAspects() 的 createdAspects 调用。
	//
	// Parameters:
	// 参数：
	//   - chainCtx: The created rule chain context
	//     chainCtx：创建的规则链上下文
	//
	// Returns:
	// 返回：
	//   - error: Error if post-creation setup fails
	//     error：后创建设置失败时的错误
	OnCreated(chainCtx NodeCtx) error
}

// OnReloadAspect defines the interface for aspects executed after rule engine configuration reload.
// These aspects are called when the rule engine or its nodes are reloaded with new configurations.
//
// OnReloadAspect 定义在规则引擎配置重载后执行的切面接口。
// 这些切面在规则引擎或其节点使用新配置重载时调用。
type OnReloadAspect interface {
	Aspect

	// OnReload is executed after rule engine configuration reload.
	// When a rule chain is updated, this triggers OnDestroy, OnBeforeReload, and OnReload in sequence.
	//
	// OnReload 在规则引擎配置重载后执行。
	// 当规则链更新时，这会依次触发 OnDestroy、OnBeforeReload 和 OnReload。
	//
	// Parameters:
	// 参数：
	//   - chainCtx: The rule chain context (equals ctx if rule chain is updated)
	//     chainCtx：规则链上下文（如果规则链更新则等于 ctx）
	//   - ctx: The specific node context that was reloaded
	//     ctx：被重载的特定节点上下文
	//
	// Returns:
	// 返回：
	//   - error: Error if reload post-processing fails
	//     error：重载后处理失败时的错误
	OnReload(chainCtx NodeCtx, ctx NodeCtx) error
}

// OnDestroyAspect defines the interface for aspects executed after rule engine instance destruction.
// These aspects are called when the rule engine instance is being destroyed or cleaned up.
//
// OnDestroyAspect 定义在规则引擎实例销毁后执行的切面接口。
// 这些切面在规则引擎实例被销毁或清理时调用。
type OnDestroyAspect interface {
	Aspect

	// OnDestroy is executed after the rule engine instance is destroyed.
	// This is called during engine shutdown or when reloading configurations.
	//
	// OnDestroy 在规则引擎实例销毁后执行。
	// 这在引擎关闭或重载配置时调用。
	//
	// Parameters:
	// 参数：
	//   - chainCtx: The rule chain context being destroyed
	//     chainCtx：正在销毁的规则链上下文
	OnDestroy(chainCtx NodeCtx)
}

type AspectList []Aspect

// GetNodeAspects 获取节点执行类型增强点切面列表
func (list AspectList) GetNodeAspects() ([]AroundAspect, []BeforeAspect, []AfterAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var aroundAspects []AroundAspect
	var beforeAspects []BeforeAspect
	var afterAspects []AfterAspect

	for _, item := range list {
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

// GetChainAspects 获取规则链执行类型增强点切面列表
func (list AspectList) GetChainAspects() ([]StartAspect, []EndAspect, []CompletedAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var startAspects []StartAspect
	var endAspects []EndAspect
	var completedAspects []CompletedAspect
	for _, item := range list {
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

// GetEngineAspects 获取规则引擎类型增强点切面列表
func (list AspectList) GetEngineAspects() ([]OnChainBeforeInitAspect, []OnNodeBeforeInitAspect, []OnCreatedAspect, []OnReloadAspect, []OnDestroyAspect) {

	//从小到大排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Order() < list[j].Order()
	})

	var chainBeforeInitAspects []OnChainBeforeInitAspect
	var nodeBeforeInitAspects []OnNodeBeforeInitAspect
	var createdAspects []OnCreatedAspect
	var afterReloadAspects []OnReloadAspect
	var destroyAspects []OnDestroyAspect

	for _, item := range list {
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
