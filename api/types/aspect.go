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
// Aspect defines RuleGo's basic interface for aspect-oriented programming (AOP).
// AOP provides cross-cutting functionality that intercepts and enhances the execution of the rule chain without modifying the original business logic of components.
//
// Engine Instance Level:
// Engine instance level:
//
// Aspects are registered at the rule engine level and each engine instance gets its own
// aspect instances through the New() method during initialization. This ensures proper
// isolation between different rule engine instances.
//
// Aspects are registered at the rule engine level. During initialization, each engine instance obtains its own
// aspect instance through the New() method, ensuring proper isolation between rule engine instances.
//
// Aspect Categories:
// Section Category:
//
//   - Engine Lifecycle Aspects: OnChainBeforeInit, OnNodeBeforeInit, OnCreated, OnReload, OnDestroy
//     Engine lifecycle section: OnChainBeforeInit, OnNodeBeforeInit, OnCreated, OnReload, OnDestroy
//   - Chain Execution Aspects: Start, End, Completed
//     Chain execution aspects: Start, End, Completed
//   - Node Execution Aspects: Before, After, Around
//     Node execution aspects: Before, After, Around
//
// Execution Order:
// Execution sequence:
//
//  1. Engine Level (during rule engine operations):
//     Engine Level (during the rule engine operation):
//     OnChainBeforeInit -> OnNodeBeforeInit -> OnCreated -> OnReload -> OnDestroy
//
//  2. Chain Level (for each message processing):
//     Chain level (per message processed):
//     Start (onStart) -> [Node Processing] -> End (onEnd) -> Completed (onAllNodeCompleted)
//
//  3. Node Level (for each node execution in rule_context.go executeAroundAop):
//     Node level (executed per node in rule_context.go executeAroundAop):
//     Before -> Around -> [Node.OnMsg] -> After
//
// Built-in Aspects:
// Built-in Aspects:
//
// RuleGo includes built-in aspects that are automatically registered:
// RuleGo includes built-in aspects for automatic registration:
//   - Validator: Data validation and schema checking
//     Validator: Data validation and pattern checking
//   - Debug: Debug information collection and logging
//     Debug: Collection and logging of debug information
//   - MetricsAspect: Performance metrics and monitoring
//     MetricsAspect: Performance metrics and monitoring
type Aspect interface {
	// Order returns the execution priority of the aspect.
	// Lower values indicate higher priority and earlier execution in the aspect chain.
	//
	// Order returns the execution priority of the facet.
	// Smaller values indicate higher priority and earlier execution in the faceted chain.
	//
	// Returns:
	// Returns:
	//   - int: Priority value, lower numbers execute first
	//     int: priority value; smaller numbers are executed first
	Order() int

	// New creates a new instance of the aspect for a specific rule engine instance.
	// This method is called during rule engine initialization (in initBuiltinsAspects and initChain)
	// to ensure each rule engine has its own aspect instance with isolated state.
	//
	// New creates a new instance of a face for a specific rule engine instance.
	// This method is called during rule engine initialization (in initBuiltinsAspects and initChain),
	// Ensure that each rule engine has its own faceted instance and isolated state.
	//
	// Implementation Requirements:
	// Implementation requirements:
	//   - Create a completely independent instance
	//     Create completely independent instances
	//   - Copy necessary configuration
	//     Copy the necessary configurations
	//   - Ensure no shared mutable state between instances
	//     Ensure that there is no mutable state shared between instances
	//
	// Returns:
	// Returns:
	//   - Aspect: New aspect instance for the rule engine
	//     Aspect: A new facet example of the rule engine
	New() Aspect
}

// NodeAspect defines the base interface for aspects that operate at the individual node level.
// These aspects can intercept and modify the execution of specific nodes based on PointCut criteria.
//
// NodeAspect defines the basic interface for aspects operating at the individual node level.
// These aspects can intercept and modify executions of specific nodes based on PointCut conditions.
//
// Node aspects are executed during message processing through nodes and provide
// fine-grained control over individual node behavior.
//
// The node aspect is executed during message processing through the node, providing fine-grained control over individual node behavior.
type NodeAspect interface {
	Aspect

	// PointCut determines whether this aspect should be applied to a specific node execution.
	// This method enables selective aspect application based on runtime conditions.
	//
	// PointCut determines whether this slice should be applied to a specific node for execution.
	// This method enables selective facet applications based on runtime conditions.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Message being processed
	//     msg: Messages being processed
	//   - relationType: Connection type between nodes
	//     relationType: The type of connection between nodes
	//
	// Returns:
	// Returns:
	//   - bool: true to apply aspect, false to skip
	//     bool:true applies the facet, false skips
	PointCut(ctx RuleContext, msg RuleMsg, relationType string) bool
}

// BeforeAspect defines the interface for aspects that execute before node message processing.
// These aspects are executed in rule_context.go executeAroundAop() before the node's OnMsg method.
//
// BeforeAspect defines the faceted interface executed before node message processing.
// These aspects are executed before the node's OnMsg method in rule_context.go executeAroundAop().
//
// Execution Flow:
// Execution process:
//  1. Message arrives at node
//     Messages reach nodes
//  2. BeforeAspect.Before() is called
//     Call BeforeAspect.Before()
//  3. Modified message is passed to node OnMsg()
//     The modified message is passed to node OnMsg()
type BeforeAspect interface {
	NodeAspect

	// Before is executed before the node's OnMsg method processes the message.
	// The returned message will be used as input for the node's OnMsg method.
	//
	// Before executing before the node's OnMsg method processes the message.
	// The returned message will be used as input to the node's OnMsg method.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Original message to be processed
	//     msg: The original message to be processed
	//   - relationType: Connection type that led to this node execution
	//     relationType: The type of connection that causes this node to execute
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Modified message for node processing
	//     RuleMsg: A modified message used for node processing
	Before(ctx RuleContext, msg RuleMsg, relationType string) RuleMsg
}

// AfterAspect defines the interface for aspects that execute after node message processing.
// These aspects are executed in rule_context.go executeAfterAop() after the node's OnMsg method.
//
// AfterAspect defines the faceted interface executed after node message processing.
// These aspects are executed after the node's OnMsg method in rule_context.go executeAfterAop().
//
// Execution Flow:
// Execution process:
//  1. Node processes message with OnMsg()
//     Nodes use OnMsg() to process messages
//  2. AfterAspect.After() is called with result/error
//     Using result/error call AfterAspect.After()
//  3. Modified message is passed to next node
//     The modified message is passed to the next node
type AfterAspect interface {
	NodeAspect

	// After is executed after the node's OnMsg method completes processing.
	// The returned message will be used for subsequent processing.
	//
	// After the node's OnMsg method is processed and executed.
	// The returned messages will be used for subsequent processing.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Message that was processed by the node
	//     msg: The message processed by the node
	//   - err: Error returned by the node processing, nil if successful
	//     err: The node handles the returned error; if successful, it is nil
	//   - relationType: Connection type for the next node execution
	//     relationType: The type of connection executed by the next node
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Modified message for next processing
	//     RuleMsg: The modified message used for the next step to process
	After(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// AroundAspect defines the interface for aspects that wrap around node message processing.
// These aspects are executed in rule_context.go executeAroundAop() and provide complete control
// over whether the node's OnMsg method is executed.
//
// AroundAspect defines the interface for message processing of the wrapping node.
// These aspects are executed in rule_context.go executeAroundAop(), providing OnMsg methods for nodes
// Whether it is executed under complete control.
//
// Execution Control:
// Execution Control:
//   - Return (msg, true): Engine will call node's OnMsg method
//     Return (msg, true): The engine will call the node's OnMsg method
//   - Return (msg, false): Engine will skip node's OnMsg method
//     Return (msg, false): The engine will skip the node's OnMsg method
type AroundAspect interface {
	NodeAspect

	// Around wraps the node's OnMsg method execution, providing complete control over whether
	// and how the node executes. Based on rule_context.go executeAroundAop() implementation.
	//
	// The OnMsg method execution of the Around Wrapped node provides complete control over whether and how the node executes.
	// Implemented based on rule_context.go executeAroundAop().
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Message to be processed
	//     msg: The message to be processed
	//   - relationType: Connection type that led to this node execution
	//     relationType: The type of connection that causes this node to execute
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Message after aspect processing
	//     RuleMsg: The message processed by the face
	//   - bool: true to allow engine to call node's OnMsg, false to skip node execution
	//     bool:true allows the engine to call the node's OnMsg, false skips node execution
	//
	// Note: Currently, the message return value cannot affect the next node's input parameters.
	// Note: Currently, message return values cannot affect the input parameters of the next node.
	Around(ctx RuleContext, msg RuleMsg, relationType string) (RuleMsg, bool)
}

// StartAspect defines the interface for aspects executed before rule chain message processing.
// These aspects are called in engine.go onStart() method before any node processing begins.
//
// StartAspect defines the interface executed before the rule chain message is processed.
// These aspects are called in the engine.go onStart() method before any node starts processing.
type StartAspect interface {
	NodeAspect

	// Start is executed before the rule chain processes the message.
	// Called in engine.go onStart() method with aspectsHolder.startAspects.
	//
	// Start is executed before the rule chain processes messages.
	// Use aspectsHolder.startAspects in the engine.go onStart() method to call aspectsHolder.startAspects.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Message to be processed by the rule chain
	//     msg: The message the rule chain will handle
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Modified message for rule chain processing
	//     RuleMsg: Modified messages used for processing the rule chain
	//   - error: Error to terminate execution, nil to continue
	//     error: Error that terminates execution; nil means to continue
	Start(ctx RuleContext, msg RuleMsg) (RuleMsg, error)
}

// EndAspect defines the interface for aspects executed when a rule chain branch ends.
// These aspects are called in engine.go onEnd() method when a branch of execution completes.
//
// EndAspect defines the interface that executes at the end of a rule chain branch.
// These aspects are called in the engine.go onEnd() method when the branch is finished.
type EndAspect interface {
	NodeAspect

	// End is executed when a branch of the rule chain execution ends.
	// Called in engine.go onEnd() method with aspectsHolder.endAspects.
	//
	// End is executed at the end of a branch executing the rule chain.
	// Uses aspectsHolder.endAspects in the engine.go onEnd() method to call aspectsHolder.endAspects.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Message at the end of branch execution
	//     msg: The message at the end of branch execution
	//   - err: Error from branch execution, nil if successful
	//     err: Error in branch execution, nil if successful
	//   - relationType: Final relation type of the branch
	//     relationType: The final relationship type of the branch
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Modified message for subsequent processing
	//     RuleMsg: Modified messages used for subsequent processing
	End(ctx RuleContext, msg RuleMsg, err error, relationType string) RuleMsg
}

// CompletedAspect defines the interface for aspects executed when all rule chain branches complete.
// These aspects are called in engine.go onAllNodeCompleted() method when all branches finish.
//
// CompletedAspect defines the faceted interface executed when all rule chain branches are completed.
// These aspects are called in the engine.go onAllNodeCompleted() method when all branches are finished.
type CompletedAspect interface {
	NodeAspect

	// Completed is executed when all branches of the rule chain execution complete.
	// Called in engine.go onAllNodeCompleted() method with aspectsHolder.completedAspects.
	//
	// Completed: Executed when all branches of the rule chain are finished.
	// Aspects is called using aspectsHolder.completedAspects in the engine.go onAllNodeCompleted() method.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule execution context
	//     ctx: Rule execution context
	//   - msg: Final message after all processing
	//     msg: The final message after all processing
	//
	// Returns:
	// Returns:
	//   - RuleMsg: Modified message for final processing
	//     RuleMsg: The modified message used for final processing
	Completed(ctx RuleContext, msg RuleMsg) RuleMsg
}

// OnChainBeforeInitAspect defines the interface for aspects executed before rule chain initialization.
// These aspects are called in engine.go initChain() method before the rule chain is created.
//
// OnChainBeforeInitAspect defines the faceted interface executed before the rule chain is initialized.
// These aspects are called in the engine.go initChain() method before the rule chain is created.
type OnChainBeforeInitAspect interface {
	Aspect

	// OnChainBeforeInit is executed before rule chain initialization.
	// If an error is returned, the chain creation will fail.
	//
	// OnChainBeforeInit is executed before the rule chain is initialized.
	// If an error is returned, the chain creation will fail.
	//
	// Parameters:
	// Parameters:
	//   - config: Rule engine configuration
	//     config: Rules engine configuration
	//   - def: Rule chain definition to be initialized
	//     def: Definition of the rule chain to be initialized
	//
	// Returns:
	// Returns:
	//   - error: Error to prevent chain creation, nil to continue
	//     error: An error blocking chain creation; nil means to continue
	OnChainBeforeInit(config Config, def *RuleChain) error
}

// OnNodeBeforeInitAspect defines the interface for aspects executed before rule node initialization.
// These aspects are called during node initialization in the rule chain setup process.
//
// OnNodeBeforeInitAspect defines the interface executed before the rule node is initialized.
// These aspects are called during node initialization during the rule chain setup process.
type OnNodeBeforeInitAspect interface {
	Aspect

	// OnNodeBeforeInit is executed before rule node initialization.
	// If an error is returned, the node creation will fail.
	//
	// OnNodeBeforeInit is executed before the rule node is initialized.
	// If an error is returned, the node creation will fail.
	//
	// Parameters:
	// Parameters:
	//   - config: Rule engine configuration
	//     config: Rules engine configuration
	//   - def: Rule node definition to be initialized
	//     def: Definition of the rule node to be initialized
	//
	// Returns:
	// Returns:
	//   - error: Error to prevent node creation, nil to continue
	//     error: Prevents errors created by nodes; nil means to continue
	OnNodeBeforeInit(config Config, def *RuleNode) error
}

// OnCreatedAspect defines the interface for aspects executed after successful rule engine creation.
// These aspects are called in engine.go initChain() method after the rule chain is successfully created.
//
// OnCreatedAspect defines the interface executed after the rule engine successfully creates it.
// These aspects are called after the rule chain is successfully created in the engine.go initChain() method.
type OnCreatedAspect interface {
	Aspect

	// OnCreated is executed after the rule engine is successfully created.
	// Called in engine.go initChain() with createdAspects from GetEngineAspects().
	//
	// OnCreated executes after the rule engine successfully creates it.
	// Use the createdAspects call from GetEngineAspects() in engine.go initChain().
	//
	// Parameters:
	// Parameters:
	//   - chainCtx: The created rule chain context
	//     chainCtx: The created rule chain context
	//
	// Returns:
	// Returns:
	//   - error: Error if post-creation setup fails
	//     error: an error when the setting fails to create a post-creation setup
	OnCreated(chainCtx NodeCtx) error
}

// OnReloadAspect defines the interface for aspects executed after rule engine configuration reload.
// These aspects are called when the rule engine or its nodes are reloaded with new configurations.
//
// OnReloadAspect defines the interface executed after the rule engine configuration overloads.
// These aspects are called when the rule engine or its nodes are reloaded with new configurations.
type OnReloadAspect interface {
	Aspect

	// OnReload is executed after rule engine configuration reload.
	// When a rule chain is updated, this triggers OnDestroy, OnBeforeReload, and OnReload in sequence.
	//
	// OnReload is executed after the rule engine configuration overloads.
	// When the rule chain is updated, this triggers OnDestroy, OnBeforeReload, and OnReload in sequence.
	//
	// Parameters:
	// Parameters:
	//   - chainCtx: The rule chain context (equals ctx if rule chain is updated)
	//     chainCtx: Rules chain context (if the rule chain is updated, it equals ctx)
	//   - ctx: The specific node context that was reloaded
	//     ctx: The context of the specific node being overloaded
	//
	// Returns:
	// Returns:
	//   - error: Error if reload post-processing fails
	//     error: The error occurring when the overload process fails
	OnReload(chainCtx NodeCtx, ctx NodeCtx) error
}

// OnDestroyAspect defines the interface for aspects executed after rule engine instance destruction.
// These aspects are called when the rule engine instance is being destroyed or cleaned up.
//
// OnDestroyAspect defines the faceted interface executed after the rule engine instance is destroyed.
// These aspects are called when the rule engine instance is destroyed or cleaned up.
type OnDestroyAspect interface {
	Aspect

	// OnDestroy is executed after the rule engine instance is destroyed.
	// This is called during engine shutdown or when reloading configurations.
	//
	// OnDestroy executes after the rule engine instance is destroyed.
	// This is called when the engine is off or in a heavy load configuration.
	//
	// Parameters:
	// Parameters:
	//   - chainCtx: The rule chain context being destroyed
	//     chainCtx: The context of the rule chain being burned
	OnDestroy(chainCtx NodeCtx)
}

type AspectList []Aspect

// GetNodeAspects returns the node aspects for an execution type.
func (list AspectList) GetNodeAspects() ([]AroundAspect, []BeforeAspect, []AfterAspect) {

	//Sort from small to large
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

// GetChainAspects returns the chain aspects for an execution type.
func (list AspectList) GetChainAspects() ([]StartAspect, []EndAspect, []CompletedAspect) {

	//Sort from small to large
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

// GetEngineAspects to obtain the rule engine type enhancement point section list
func (list AspectList) GetEngineAspects() ([]OnChainBeforeInitAspect, []OnNodeBeforeInitAspect, []OnCreatedAspect, []OnReloadAspect, []OnDestroyAspect) {

	//Sort from small to large
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
