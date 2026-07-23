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
// The package engine provides the core features of the RuleGo rule engine.
// It includes the implementation of rule contexts, rule engines, and related components,
// These components support the execution and management of the rule chain.
//
// The engine package is responsible for:
// The engine package is responsible for:
//   - Defining and managing rule contexts (DefaultRuleContext)
//     Define and manage the rule context (DefaultRuleContext)
//   - Implementing the main rule engine (RuleEngine)
//     Implementing the main rule engine (RuleEngine)
//   - Handling rule chain execution and flow control
//     Handle rule chain execution and process control
//   - Managing built-in aspects and extensions
//     Manage built-in aspects and extensions
//   - Providing utilities for rule processing and message handling
//     Provides tools for rule processing and message processing
//
// Key Components:
// Key components:
//   - RuleEngine: Main engine instance managing rule chain execution
//     RuleEngine: The main engine instance that manages rule chain execution
//   - RuleChainCtx: Context for individual rule chains
//     RuleChainCtx: The context of a single rule chain
//   - DefaultRuleContext: Context for message processing within rule chains
//     DefaultRuleContext: The context for message processing within the rule chain
//   - RuleNodeCtx: Context wrapper for individual node components
//     RuleNodeCtx: Context wrapper for a single node component
//
// Architecture Overview:
// Architecture Overview:
//
//	The engine follows a hierarchical structure where a RuleEngine contains
//	one root RuleChainCtx, which manages multiple RuleNodeCtx instances.
//	Message processing flows through DefaultRuleContext instances that
//	coordinate between nodes and handle aspect-oriented programming features.
//
//	The engine follows a hierarchical structure, where the RuleEngine contains a root RuleChainCtx,
//	This context manages multiple RuleNodeCtx instances. Message processing is done through DefaultRuleContext
//	Instance lifecycle, where instances coordinate aspect-oriented programming behavior across nodes.
//
// This package is central to the RuleGo framework, offering the primary mechanisms
// for rule-based processing and decision making in various applications.
//
// This package is the core of the RuleGo framework, designed for rule-based processing and decision-making in various applications
// Providing the main mechanism.
package engine

import (
	"context"
	"errors"
	"fmt"
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
// BuiltinsAspects stores the list of built-in faces in the rule engine.
// These aspects provide basic cross-cutting functionality and are automatically integrated into each rule engine instance to ensure consistent behavior.
//
// Built-in Aspects:
// Built-in Aspects:
//
//   - Validator: Validates node configurations and rule chain definitions
//     before execution to prevent runtime errors.
//     Verifier: Validates node configuration and rule chain definitions before execution to prevent runtime errors.
//
//   - Debug: Provides debugging capabilities including execution tracing,
//     state inspection, and development-time diagnostics.
//     Debugger: Provides debugging functions, including tracking execution, status checks, and diagnostics during development.
//
//   - MetricsAspect: Collects performance metrics, execution statistics,
//     and operational data for monitoring and observability.
//     Metric Section: Collect performance metrics, execution statistics, and operational data for monitoring and observability.
//
// Automatic Integration:
// Automatic integration:
//
//	These aspects are automatically added to the rule engine during initialization
//	via the initBuiltinsAspects() method. If custom aspects are provided, the
//	built-in aspects are still included unless an aspect of the same type already
//	exists in the custom list. This ensures that essential functionality is always
//	available without requiring explicit configuration.
//
//	These aspects are automatically added to the rule engine during initialization using the initBuiltinsAspects() method.
//	If a custom facet is provided, it will still be included unless the same type of facet already exists in the custom list
//	built-in aspects. This ensures that essential functions are always available without explicit configuration.
var BuiltinsAspects = []types.Aspect{&aspect.Validator{}, &aspect.Debug{}, &aspect.MetricsAspect{}}

// aspectsHolder holds the aspects for atomic access to improve performance
// by avoiding lock contention during high-frequency aspect operations.
// aspectsHolder stores aspects for atomic access, avoiding lock contention during frequent aspect operations.
type aspectsHolder struct {
	// startAspects are executed before rule chain processing begins
	// startAspects is executed before the rule chain process begins
	startAspects []types.StartAspect
	// endAspects are executed when a rule chain branch ends
	// endAspects is executed at the end of the rule chain branch
	endAspects []types.EndAspect
	// completedAspects are executed when all rule chain branches complete
	// completedAspects is executed when all branches of the rule chain are completed
	completedAspects []types.CompletedAspect
}

// RuleEngine is the core structure for a rule engine instance.
// Each RuleEngine instance manages exactly one root rule chain and provides
// the primary interface for message processing and rule execution.
//
// RuleEngine is the core structure of rule engine instances.
// Each RuleEngine instance manages exactly one root rule chain and provides the main interface for message processing and rule execution.
//
// Architecture & Features:
// Architecture and Features:
//   - Single root rule chain management with hot reloading capability
//     Single rule chain management, supports hot reload functionality
//   - Aspect-oriented programming support for cross-cutting concerns
//     Aspect-oriented programming support for cross-cutting concerns
//   - Two-phase graceful shutdown for safe resource cleanup
//     Two-stage elegant shutdown ensures safe resource clearance
//   - Deadlock-free reload mechanism with message queuing
//     No deadlock overload mechanism, supports message queues
//   - Concurrent message processing with atomic operations
//     Concurrent message processing uses atomic operations
//   - Context-aware processing with shutdown signal integration
//     Context-aware processing, integrated shutdown signals
//   - Sub-rule chain pool integration for nested execution
//     Sub-rule chain pool integration, supporting nested execution
//   - Comprehensive metrics and debugging capabilities
//     Comprehensive metrics and debugging functions
//   - Backpressure control during reload to prevent memory overflow
//     Backpressure control during heavy load to prevent memory overflow
//
// Lifecycle Management:
// Lifecycle Management:
//  1. Creation with NewRuleEngine() and rule chain definition
//     Created using NewRuleEngine() and rule chain definitions
//  2. Message processing via OnMsg() with concurrent safety
//     Message processing is performed via OnMsg(), ensuring concurrency security
//  3. Optional hot reloading with ReloadSelf() without downtime
//     Use ReloadSelf() for optional heat reloading without shutdown
//  4. Graceful cleanup with Stop() and proper resource release
//     Use Stop() for elegant cleanup and appropriate resource release
//
// Thread Safety & Concurrency:
// Thread Safety and Concurrency:
//
//	RuleEngine is designed for high-concurrency scenarios with:
//	RuleEngine is designed for high-concurrency scenarios and features:
//	- Lock-free message processing using atomic operations
//	  Handle unlocked messages using atomic operations
//	- Safe concurrent access to rule chain definitions
//	  Secure concurrent access to the rule chain as defined
//	- Coordinated reload operations without blocking message flow
//	  Coordinated overloading operations without blocking the message flow
//	- Graceful shutdown handling for all concurrent operations
//	  Elegant shutdown handling for all concurrent operations
//	- Backpressure control to prevent resource exhaustion
//	  Backpressure control prevents resource depletion
//
// Memory Safety During Reload:
// Memory safety during overload:
//
//	The engine implements sophisticated backpressure mechanisms to prevent
//	memory overflow during reload operations:
//	The engine implements a complex backpressure mechanism to prevent memory overflow during heavy load operations:
//	- Limited concurrent goroutines waiting for reload completion
//	  Limit the number of goroutines waiting for concurrent overload to complete
//	- Fast-fail strategy for excessive reload wait requests
//	  A quick failure strategy for overloading and waiting for requests
//	- Configurable memory protection thresholds
//	  Configurable memory protection thresholds
//	- Automatic degradation to reject mode under high load
//	  Automatically downgraded to reject mode under heavy load
//
// This design ensures reliable, high-performance rule processing in production environments.
// This design ensures reliable, high-performance rule handling in production environments.
type RuleEngine struct {
	// Embed graceful shutdown functionality
	// Embedded elegant shutdown function
	base.GracefulShutdown

	// Config is the configuration for the rule engine containing
	// global settings, component registry, and execution parameters
	// Config is the configuration of the rule engine, including global settings, component registry, and execution parameters
	Config types.Config

	// ruleChainPool is a pool of sub-rule engines for handling nested rule chains
	// ruleChainPool is a sub-rule engine pool that handles nested rule chains
	ruleChainPool types.RuleEnginePool

	// id is the unique identifier for the rule engine instance
	// id is the unique identifier of the rule engine instance
	id string

	// aliases is an additional lookup key for the engine besides the main ID. Pool.Get(alias) can parse into the engine.
	// When the NewRuleEngine's id is different from the def.ruleChain.id, ruleChain.id is recorded as an alias.
	aliases []string

	// rootRuleChainCtx is the context of the root rule chain containing
	// all nodes and their relationships
	// rootRuleChainCtx is the context of the root rule chain, containing all nodes and their relationships
	rootRuleChainCtx *RuleChainCtx

	// aspectsPtr provides high-performance atomic access to aspects
	// to avoid lock contention during message processing
	// aspectsPtr provides high-performance atomic access to the facet to avoid lock contention during message processing
	aspectsPtr unsafe.Pointer

	// initialized indicates whether the rule engine has been properly initialized
	// Use atomic operations to prevent data races during concurrent access
	// initialized indicates whether the rule engine has been properly initialized
	// Use atomic operations to prevent data gridlock during concurrent access
	initialized int32

	// Aspects is a list of AOP (Aspect-Oriented Programming) aspects
	// that provide cross-cutting concerns like logging, validation, and metrics
	// Aspects are face-to-face programming (AOP) listings that provide cross-cutting concerns such as logs, validations, and metrics
	Aspects types.AspectList

	// OnUpdated is a callback function triggered when the rule chain is updated
	// OnUpdated is a callback function triggered when the rule chain is updated
	OnUpdated func(chainId, nodeId string, dsl []byte)

	// Backpressure control fields for memory safety during reload
	// Backpressure control field for memory safety during overload

	// maxConcurrentReloadWaiters limits the number of goroutines that can wait for reload completion
	// to prevent memory overflow during high-traffic reload operations
	// maxConcurrentReloadWaiters limits the number of goroutines that can wait for the overload to complete,
	// Prevents memory overflow during high-traffic overload operations
	maxConcurrentReloadWaiters int64

	// currentReloadWaiters tracks the current number of goroutines waiting for reload
	// currentReloadWaiters tracks the number of goroutines currently waiting to be overloaded
	currentReloadWaiters int64

	// reloadBackpressureEnabled enables/disables backpressure control
	// reloadBackpressureEnabled Enable/Disables backpressure control
	reloadBackpressureEnabled bool
	reloadLock                sync.Mutex
}

// NewRuleEngine creates a new RuleEngine instance with the given ID and definition.
// It applies the provided RuleEngineOptions during the creation process.
//
// NewRuleEngine creates new RuleEngine instances using a given ID and definition.
// It applies the provided RuleEngineOptions during the creation process.
//
// Parameters:
// Parameters:
//   - id: Unique identifier for the rule engine (can be empty to use chain ID)
//     Unique identifier for the rule engine (can be empty to use the chain ID)
//   - def: Rule chain definition in JSON or other supported format
//     Define a rule chain in JSON or other supported formats
//   - opts: Optional configuration functions to customize the engine
//     Optional configuration functions to customize the engine
//
// Returns:
// Returns:
//   - *RuleEngine: Initialized rule engine instance
//   - error: Initialization error if any
//
// The creation process involves:
// The creation process includes:
//  1. Parsing the rule chain definition
//  2. Initializing all components and their relationships
//  3. Setting up aspects and callback functions
//  4. Validating the configuration
//  5. Configuring backpressure control for memory safety
func NewRuleEngine(id string, def []byte, opts ...types.RuleEngineOption) (*RuleEngine, error) {
	if len(def) == 0 {
		return nil, errors.New("def can not nil")
	}

	// Create a new RuleEngine with the Id
	// Create a new RuleEngine using the ID
	ruleEngine := &RuleEngine{
		id:            id,
		Config:        NewConfig(),
		ruleChainPool: DefaultPool,
		// Initialize backpressure control with default values
		// Initialize backpressure control using default values
		maxConcurrentReloadWaiters: 1000, // Default: allow max 1000 concurrent waiters
		reloadBackpressureEnabled:  true, // Enable backpressure by default
	}

	// Initialize graceful shutdown functionality
	// Initialize the elegant shutdown function
	if ruleEngine.Config.Logger == nil {
		ruleEngine.Config.Logger = types.DefaultLogger()
	}
	ruleEngine.InitGracefulShutdown(ruleEngine.Config.Logger, 10*time.Second)

	err := ruleEngine.ReloadSelf(def, opts...)
	if err == nil && ruleEngine.rootRuleChainCtx != nil {
		// ruleChain.id in def (obtained by ReloadSelf parsing).
		ruleChainId := ruleEngine.rootRuleChainCtx.Id.Id
		if id != "" {
			ruleEngine.rootRuleChainCtx.Id = types.RuleNodeId{Id: id, Type: types.CHAIN}
			// When id overrides ruleChain.id, ruleChain.id is recorded as an alias.
			if ruleChainId != "" && ruleChainId != id {
				ruleEngine.aliases = append(ruleEngine.aliases, ruleChainId)
			}
		} else {
			// Use the rule chain ID if no ID is provided.
			// If no ID is provided, the rule chain ID is used.
			ruleEngine.id = ruleChainId
		}

	}

	return ruleEngine, err
}

// Aliases returns an alias for the engine (besides the main id, it can be used as a lookup key for Pool.Get).
func (e *RuleEngine) Aliases() []string {
	return e.aliases
}

// Id returns the unique identifier of the rule engine instance.
// Id returns a unique identifier for the rule engine instance.
func (e *RuleEngine) Id() string {
	return e.id
}

// SetConfig updates the configuration of the rule engine.
// This should be called before initialization for best results.
// SetConfig updates the configuration of the rule engine.
// For best results, it should be called before initialization.
func (e *RuleEngine) SetConfig(config types.Config) {
	e.Config = config
}

// SetAspects updates the list of aspects used by the rule engine.
// Aspects provide cross-cutting functionality like logging and validation.
// SetAspects updates the list of faces used by the rule engine.
// The interface provides cross-cutting functions such as logging and verification.
func (e *RuleEngine) SetAspects(aspects ...types.Aspect) {
	e.Aspects = aspects
}

// SetRuleEnginePool sets the pool used for managing sub-rule chains.
// This allows for nested rule chain execution and resource sharing.
// SetRuleEnginePool sets the pool used to manage the subrule chain.
// This allows nested rule chains to be executed and resource sharing.
func (e *RuleEngine) SetRuleEnginePool(ruleChainPool types.RuleEnginePool) {
	e.ruleChainPool = ruleChainPool
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.SetRuleEnginePool(ruleChainPool)
	}
}

// GetAspects returns a copy of the current aspects list to avoid data races.
// GetAspects returns a copy of the current section list to avoid data contention.
func (e *RuleEngine) GetAspects() types.AspectList {
	// Return a copy to avoid data contention
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.GetAspects()
	}
	return e.Aspects
}

// Reload reloads the current rule chain with optional new configuration.
// This is a convenience method that uses the current DSL definition.
// Reload uses an optional new configuration to override the current chain of rules.
// This is a convenient way to use the current DSL definition.
func (e *RuleEngine) Reload(opts ...types.RuleEngineOption) error {
	return e.ReloadSelf(e.DSL(), opts...)
}

// initBuiltinsAspects initializes the built-in aspects if no custom aspects are provided.
// It ensures that essential aspects like validation and debugging are always available.
// initBuiltinsAspects initializes the built-in aspects if no custom aspects are provided.
// It ensures that essential aspects such as verification and debugging are always available.
func (e *RuleEngine) initBuiltinsAspects() {
	var newAspects types.AspectList
	// Initialize the built-in aspects
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
				//Determine whether they are the same type
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
// initChain uses the provided definition to initialize the rule chain.
// It sets all nodes and relationships and runs the aspect creation hooks.
func (e *RuleEngine) initChain(def types.RuleChain) error {
	if def.RuleChain.Disabled {
		return types.ErrEngineDisabled
	}
	if ctx, err := InitRuleChainCtx(e.Config, e.Aspects, &def, e.ruleChainPool); err == nil {
		if e.rootRuleChainCtx != nil {
			ctx.Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx
		//Execute the creation of faceted logic
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
// Phase 1: Preparation
// - Apply configuration options
// - Wait for any ongoing reload to complete
// - Set reloading state to block new messages
// - Wait for active messages to complete
//
// Phase 2: Reload
// - Parse new rule chain definition
// - Update or create rule chain context
// - Update atomic aspect pointers
// - Resume normal operation
//
// ReloadSelf uses new definitions and options to reload the chain of rules.
// This method supports hot reload rule configuration without stopping the engine.
// It achieves a two-stage elegant heavy loading process:
//
// Parameters:
// Parameters:
//   - dsl: Rule chain definition in byte format
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - error: Reload error if any
func (e *RuleEngine) ReloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	e.reloadLock.Lock()
	defer e.reloadLock.Unlock()
	return e.reloadSelf(dsl, opts...)
}

func (e *RuleEngine) reloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	// Apply options to RuleEngine.
	for _, opt := range opts {
		_ = opt(e)
	}

	// Check if engine is shutting down, if so, reject reload operation
	// Check if the engine is shutting down; if so, refuse heavy load operation
	if e.IsShuttingDown() {
		return types.ErrEngineShuttingDown
	}

	// Set reloading state to block new messages during reload
	// Set the overload state to block new messages during overload
	if e.Initialized() {
		e.SetReloading(true)
		defer e.SetReloading(false)

		// Wait for active messages to complete before reloading
		// Wait for the active message to complete before reloading
		waitTimeout := 10 * time.Second
		e.WaitForActiveOperations(waitTimeout)
	}

	var err error
	if e.Initialized() {
		// Initialize the built-in aspects
		if len(e.Aspects) == 0 {
			e.initBuiltinsAspects()
		}
		e.rootRuleChainCtx.config = e.Config
		e.rootRuleChainCtx.SetAspects(e.Aspects)
		//Update the rule chain
		err = e.rootRuleChainCtx.ReloadSelf(dsl)
		//Set up a sub-rule chain pool
		//e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, e.id, dsl)
		}
	} else {
		// Initialize the built-in aspects
		e.initBuiltinsAspects()
		var rootRuleChainDef types.RuleChain
		//Initialization
		if rootRuleChainDef, err = e.Config.Parser.DecodeRuleChain(dsl); err == nil {
			err = e.initChain(rootRuleChainDef)
		} else {
			return err
		}
	}

	// Set the aspect lists.
	// Set the section list.
	startAspects, endAspects, completedAspects := e.Aspects.GetChainAspects()
	holder := &aspectsHolder{startAspects: startAspects, endAspects: endAspects, completedAspects: completedAspects}
	atomic.StorePointer(&e.aspectsPtr, unsafe.Pointer(holder))
	return err
}

// waitForReloadComplete waits for any ongoing reload to complete before starting a new one.
// waitForReloadComplete waits for any ongoing reload to complete before starting a new one.
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
// ReloadChild updates specific nodes in the root rule chain.
// If ruleNodeId is empty, update the entire root rule chain.
// It gracefully stops receiving new messages, waits for active messages to complete, performs a reload, and then resumes normal operation.
//
// Parameters:
// Parameters:
//   - ruleNodeId: ID of the node to update (empty for root chain)
//     Node ID to be updated (root chain is empty)
//   - dsl: New configuration for the node/chain
//
// Returns:
// Returns:
//   - error: Update error if any
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
		//Update the root rule chain
		return e.reloadSelf(dsl)
	} else {
		// Set reloading state to block new messages during reload
		// Set the overload state to block new messages during overload
		e.SetReloading(true)
		defer e.SetReloading(false)

		// Wait for active messages to complete before reloading child node
		// Wait for active messages to complete before reloading child nodes
		waitTimeout := 10 * time.Second
		e.WaitForActiveOperations(waitTimeout)

		//Update the root rule chain subnodes
		err := e.rootRuleChainCtx.ReloadChild(types.RuleNodeId{Id: ruleNodeId}, dsl)

		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, ruleNodeId, e.DSL())
		}
		return err
	}
}

// DSL returns the current rule chain configuration in its original format.
// The DSL returns the current rule chain configuration in the original format.
func (e *RuleEngine) DSL() []byte {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.DSL()
	} else {
		return nil
	}
}

// Definition returns the rule chain definition structure.
// Definition Returns the rule chain definition structure.
func (e *RuleEngine) Definition() types.RuleChain {
	if e.rootRuleChainCtx != nil {
		return *e.rootRuleChainCtx.SelfDefinition
	} else {
		return types.RuleChain{}
	}
}

// NodeDSL returns the configuration of a specific node within the rule chain.
// NodeDSL returns the configuration of a specific node in the rule chain.
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
// Initialized: Returns whether the rule engine has been correctly initialized.
func (e *RuleEngine) Initialized() bool {
	return atomic.LoadInt32(&e.initialized) == 1 && e.rootRuleChainCtx != nil
}

// RootRuleChainCtx returns the root rule chain context.
// RootRuleChainCtx returns the root rule chain context.
func (e *RuleEngine) RootRuleChainCtx() types.ChainCtx {
	return e.rootRuleChainCtx
}

// Stop shuts down the rule engine and releases all resources.
// Implements a two-phase graceful shutdown strategy:
//
// Phase 1: Graceful Shutdown
// - Set shutdown flag to reject new messages
// - Wait for all active messages to complete naturally
// - Respect the provided context timeout
//
// Phase 2: Force Shutdown
// - If timeout exceeded, cancel contexts to interrupt operations
// - Give brief time for operations to respond to cancellation
// - Clean up all resources immediately
//
// Context handling:
// Context Handling:
// - If ctx is provided with deadline: uses that timeout
// - If ctx is context.Background(): uses default 10s timeout
// - If ctx is nil: performs immediate shutdown
//
// Concurrent calls handling:
// Concurrent Call Handling:
//   - If graceful shutdown is already in progress, subsequent calls wait for completion
//     If the elegant shutdown is already underway, subsequent calls await its completion
//   - Only one graceful shutdown process can execute at a time
//     Only one elegant downtime can be performed at a time
//   - No forced interruption of ongoing graceful shutdown
//     Elegant downtime in progress is not forcibly interrupted
//
// Stop Shut down the rule engine and release all resources.
// Achieve a two-stage elegant shutdown strategy:
func (e *RuleEngine) Stop(ctx context.Context) {
	// Handle concurrent calls: if already shutting down, wait for completion instead of forcing
	// Handling concurrent calls: If the machine is already down, wait for completion instead of forcing it
	if e.IsShuttingDown() {
		// Check if shutdown is already completed by checking if the engine is initialized
		// If the engine is not initialized, shutdown has already completed
		// Check if the shutdown has been completed, by checking if the engine has been initialized
		// If the engine is not initialized, it means the shutdown has been completed
		if !e.Initialized() {
			// Shutdown has already completed, no need to wait
			// The shutdown is complete, no waiting needed
			return
		}

		// Wait for the ongoing shutdown to complete with a reasonable timeout
		// Wait for the ongoing downtime to complete and set a reasonable timeout
		shutdownWaitTimeout := 10 * time.Second // Reduced from 30s for better responsiveness
		if ctx != nil {
			if deadline, ok := ctx.Deadline(); ok {
				// Use the remaining time from the provided context, but with a minimum wait time
				// Use the remaining time of the provided context, but set a minimum waiting time
				remainingTime := time.Until(deadline)
				if remainingTime > 0 && remainingTime < shutdownWaitTimeout {
					shutdownWaitTimeout = remainingTime
				}
			}
		}

		// Wait for the ongoing shutdown to complete
		// Waiting for the ongoing downtime to be completed
		ticker := time.NewTicker(50 * time.Millisecond) // More frequent checks for faster response
		defer ticker.Stop()

		waitCtx, cancel := context.WithTimeout(context.Background(), shutdownWaitTimeout)
		defer cancel()

		for {
			select {
			case <-waitCtx.Done():
				// Timeout waiting for shutdown to complete, force cleanup
				// Wait for the shutdown to complete and timeout, then force cleanup
				e.Config.Logger.Printf("Timeout waiting for ongoing shutdown to complete, forcing cleanup")
				e.forceStop()
				return
			case <-ticker.C:
				// Check if shutdown completed by checking initialization status
				// Shutdown is complete when the engine is no longer initialized
				// Check whether the shutdown is complete by checking the initialization status
				// Shutdown is complete when the engine is no longer initialized
				if !e.Initialized() {
					// Shutdown completed successfully
					// The shutdown was successfully completed
					return
				}
			}
		}
	}

	// Calculate timeout from context, handling negative durations explicitly
	// Timeouts are calculated from context, and negative durations are clearly handled
	var timeout time.Duration
	var isExpiredContext bool

	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
			if timeout <= 0 {
				// Context deadline has already passed
				// The context deadline has passed
				isExpiredContext = true
				e.Config.Logger.Printf("Context deadline has already passed (negative duration: %v), performing immediate shutdown", timeout)
				timeout = 0 // Use immediate shutdown for expired contexts
			}
		} else {
			// Default timeout for context.Background() or contexts without deadline
			// For context.Background() or contexts without a cutoff time use the default timeout
			timeout = 10 * time.Second
		}
	} else {
		// Immediate shutdown for nil context
		// Immediate shutdown for nil context
		timeout = 0
	}

	// Perform graceful shutdown
	// Perform elegant shutdowns
	e.GracefulShutdown.GracefulStop(func() {
		if isExpiredContext || timeout == 0 {
			// For expired contexts or nil context, skip graceful wait and go straight to cleanup
			// For expired or nil contexts, skip the elegant wait and clear directly
			e.Config.Logger.Printf("Performing immediate shutdown")
			e.GracefulShutdown.ForceStop()
		} else {
			// Phase 1: Wait for all active messages to complete naturally
			// Stage One: Wait for all active messages to be completed naturally
			allCompleted := e.WaitForActiveOperations(timeout)
			if !allCompleted {
				e.Config.Logger.Printf("Graceful shutdown timeout after %v, forcing context cancellation", timeout)
				// Phase 2: Force cancel context to interrupt ongoing operations
				// Stage Two: Enforcedly discontextualize to interrupt ongoing operations
				e.GracefulShutdown.ForceStop()
				// Give a brief moment for operations to respond to cancellation
				// Give the operation a brief moment to respond to cancellation
				e.WaitForActiveOperations(500 * time.Millisecond)
			}
		}
		// Clean up resources
		// Release resources
		e.forceStop()
	})
}

// applyShutdownContext applies graceful shutdown context handling to the rule context copy.
// This method ensures that message processing respects shutdown signals while preserving
// user-provided context functionality.
//
// Context application strategy:
// Contextual Application Strategy:
//  1. If user hasn't provided custom context: use shutdown context directly
//     If the user does not provide a custom context: use the downtime context directly
//  2. If user provided custom context: combine both contexts
//     If the user provides a custom context: combine two contexts
//     - Preserves user context values and behavior
//     - Adds shutdown cancellation capability
//     - Cancellation triggers when either context is cancelled
//
// This design ensures both user functionality and graceful shutdown work correctly together.
//
// applyShutdownContext applies elegant shutdown context handling to the rule context copy.
// This method ensures message processing respects the downtime signal while retaining user-provided contextual functionality.
// applyShutdownContext applies shutdown context handling to the rule context.
// This method now only preserves context values from user context while using shutdown context for cancellation.
// The user context cancellation feature has been removed to prevent goroutine leaks in high concurrency scenarios.
func (e *RuleEngine) applyShutdownContext(rootCtxCopy, rootCtx *DefaultRuleContext) {
	shutdownCtx := e.GetShutdownContext()
	if shutdownCtx == nil {
		return
	}

	if rootCtxCopy.GetContext() == rootCtx.GetContext() {
		// No custom context was set by user options, use shutdown context directly
		// User options do not set custom contexts and directly use downtime contexts
		rootCtxCopy.SetContext(shutdownCtx)
	} else {
		// User provided custom context, preserve its values but use shutdown context for cancellation
		// This prevents goroutine leaks while maintaining context value inheritance
		// Users provide custom contexts that retain values but cancel them using a downtime context
		userCtx := rootCtxCopy.GetContext()
		combinedCtx, cancel := e.combineContextsValueOnly(userCtx, shutdownCtx)
		rootCtxCopy.SetContext(combinedCtx)

		// Ensure context is cancelled when rule chain execution completes
		// Make sure the rule chain removes context once execution is complete
		if cancel != nil {
			originalOnAllNodeCompleted := rootCtxCopy.onAllNodeCompleted
			rootCtxCopy.SetOnAllNodeCompleted(func() {
				cancel()
				if originalOnAllNodeCompleted != nil {
					originalOnAllNodeCompleted()
				}
			})
		}
	}
}

// combineContextsValueOnly creates a context that inherits values from user context
// and can be cancelled by either the user context or shutdown context.
//
// The returned context:
// 1. Inherits all values from the user context
// 2. Can be cancelled by either user context or shutdown context
// 3. Uses a controlled goroutine that is cleaned up when context is cancelled
//
// combineContextsValueOnly creates a context that inherits a value from the user's context and can be canceled by the user's context or the downtime context.
//
// Returned context:
// 1. Inherit all values from the user's context
// 2. Can be canceled by user context or downtime context
// 3. Use controlled coroutines that are cleaned up when context is removed
func (e *RuleEngine) combineContextsValueOnly(userCtx, shutdownCtx context.Context) (context.Context, context.CancelFunc) {
	// Check if either context is already cancelled
	// Check whether any context has been canceled
	select {
	case <-userCtx.Done():
		// User context already cancelled, return it directly
		// User context is canceled and returns directly
		return userCtx, func() {}
	case <-shutdownCtx.Done():
		// Shutdown context already cancelled, return it directly
		// The downtime context has been removed, and you will return directly
		return shutdownCtx, func() {}
	default:
		// Both contexts are active, create combined context
		// Both contexts are active, creating a combined context
		c := newCombinedCancelContext(userCtx, shutdownCtx)
		return c, c.Cancel
	}
}

// combinedCancelContext is a context implementation that can be cancelled by either
// of two parent contexts. It uses lazy goroutine creation - only creates a goroutine
// when Done() is first called, avoiding unnecessary goroutines for contexts that are
// never checked for cancellation.
// combinedCancelContext is a context implementation that can be canceled by either of the two parent contexts.
// It uses delayed coroutine creation—only when Done() is called for the first time, avoiding unnecessary coroutines for contexts that don't require unchecking.
type combinedCancelContext struct {
	userCtx     context.Context
	shutdownCtx context.Context
	ctx         context.Context // Internal context for cancellation
	cancel      context.CancelFunc
	err         error
	errOnce     sync.Once
	errMu       sync.Mutex
	doneOnce    sync.Once
}

// newCombinedCancelContext creates a new combined context that can be cancelled
// by either userCtx or shutdownCtx. It uses a single goroutine to monitor both
// contexts, which exits immediately when either context is cancelled.
// newCombinedCancelContext creates a new combinatorial context that can be canceled by userCtx or shutdownCtx.
// It uses a single coroutine to monitor two contexts and immediately exits when either context is canceled.
func newCombinedCancelContext(userCtx, shutdownCtx context.Context) *combinedCancelContext {
	// Check if either context is already cancelled to avoid creating unnecessary goroutine
	// Check whether any context has been removed to avoid creating unnecessary coroutines
	select {
	case <-userCtx.Done():
		// User context already cancelled, create a context that's already cancelled
		// User context canceled, create a deleted context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel
		c := &combinedCancelContext{
			userCtx:     userCtx,
			shutdownCtx: shutdownCtx,
			ctx:         ctx,
			cancel:      cancel,
		}
		c.setErr(userCtx.Err())
		return c
	case <-shutdownCtx.Done():
		// Shutdown context already cancelled, create a context that's already cancelled
		// The downtime context has been removed, creating a canceled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel
		c := &combinedCancelContext{
			userCtx:     userCtx,
			shutdownCtx: shutdownCtx,
			ctx:         ctx,
			cancel:      cancel,
		}
		c.setErr(shutdownCtx.Err())
		return c
	default:
		// Both contexts are active, will create goroutine lazily when Done() is called
		// Both contexts are active and will delay the coroutine creation when Done() is first called
	}

	// Create an internal context that will be cancelled when either parent is cancelled
	// Create an inner context that will be canceled when either parent context is canceled
	ctx, cancel := context.WithCancel(context.Background())

	return &combinedCancelContext{
		userCtx:     userCtx,
		shutdownCtx: shutdownCtx,
		ctx:         ctx,
		cancel:      cancel,
		// done will be initialized lazily when Done() is first called
		// done will delay initialization when Done() is first called
	}
}

// setErr sets the error once when the context is cancelled
// setErr sets an error when the context is canceled (only once)
func (c *combinedCancelContext) setErr(err error) {
	c.errOnce.Do(func() {
		c.errMu.Lock()
		c.err = err
		c.errMu.Unlock()
	})
}

// Cancel cancels the context and stops the monitoring goroutine.
// Cancel removes context and stops monitoring coroutines.
func (c *combinedCancelContext) Cancel() {
	c.cancel()
}

// Done returns a channel that is closed when the context is cancelled.
// The goroutine is created lazily on first call to avoid unnecessary goroutines.
// Done: Returns a channel that was closed when the context was canceled.
// Coroutines are created with delay on the first call to avoid unnecessary coroutines.
func (c *combinedCancelContext) Done() <-chan struct{} {
	// Check if already cancelled before starting monitoring
	// Check if the monitoring has been canceled before starting monitoring
	select {
	case <-c.userCtx.Done():
		// Already cancelled, return closed channel
		// Canceled, returns to closed channels
		c.setErr(c.userCtx.Err())
		return c.userCtx.Done()
	case <-c.shutdownCtx.Done():
		// Already cancelled, return closed channel
		// Canceled, returns to closed channels
		c.setErr(c.shutdownCtx.Err())
		return c.shutdownCtx.Done()
	default:
		// Start monitoring lazily
		// Delayed start monitoring
		c.startMonitoring()
		return c.ctx.Done()
	}
}

// Err returns the error if either parent context is cancelled, nil otherwise.
// err returns an error if either parent context is canceled; otherwise, nil is returned.
func (c *combinedCancelContext) Err() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	if c.err != nil {
		return c.err
	}
	return c.ctx.Err()
}

// Deadline returns the earlier deadline of the two parent contexts, or ok=false if neither has a deadline.
// Deadline returns the earlier cutoff time in the two parent contexts; if there is no cutoff time, ok=false is used.
func (c *combinedCancelContext) Deadline() (time.Time, bool) {
	userDeadline, userOk := c.userCtx.Deadline()
	shutdownDeadline, shutdownOk := c.shutdownCtx.Deadline()

	if !userOk && !shutdownOk {
		return time.Time{}, false
	}
	if !userOk {
		return shutdownDeadline, shutdownOk
	}
	if !shutdownOk {
		return userDeadline, userOk
	}
	if userDeadline.Before(shutdownDeadline) {
		return userDeadline, true
	}
	return shutdownDeadline, true
}

// Value returns the value associated with this context for key, or nil if no value is associated with key.
// It first checks the user context, then falls back to the shutdown context.
// Value returns the value associated with the key in this context; if there is no value associated with the key, it returns nil.
// It first checks the user context, then falls back to the downtime context.
func (c *combinedCancelContext) Value(key interface{}) interface{} {
	if val := c.userCtx.Value(key); val != nil {
		return val
	}
	return c.shutdownCtx.Value(key)
}

// incrementActiveMessages increases the count of active messages
func (e *RuleEngine) incrementActiveMessages() {
	e.IncrementActiveOperations()
}

// decrementActiveMessages reduces the count of active messages
func (e *RuleEngine) decrementActiveMessages() {
	e.DecrementActiveOperations()
}

// forceStop performs immediate cleanup of all rule engine resources.
// This method is called during shutdown to ensure complete resource cleanup,
// regardless of whether graceful shutdown completed successfully.
//
// Cleanup operations (with panic recovery):
// Cleanup Operation (Recovery with Panic):
// 1. Force cancellation of graceful shutdown context
// 2. Destroy rule chain context and all nodes
// 3. Clear instance cache entries
// 4. Reset initialization state
//
// Each cleanup operation is wrapped with panic recovery to ensure
// that failures in one cleanup step don't prevent others from executing.
//
// forceStop executes rules and immediately cleans up engine resources.
// This method is called during downtime to ensure a complete resource cleanup, regardless of whether the graceful shutdown is successfully completed.
func (e *RuleEngine) forceStop() {
	defer func() {
		if r := recover(); r != nil {
			e.Config.Logger.Printf("RuleEngine.forceStop() panic recovered: %v", r)
		}
	}()

	// Force cancellation of graceful shutdown context
	// Forced cancellation of graceful downtime context
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

	// Clean the instance cache
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
// OnMsg uses a rule engine to process messages asynchronously.
// It accepts the optional RuleContextOption parameter to customize the execution context.
func (e *RuleEngine) OnMsg(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// OnMsgAndWait synchronously processes a message using the rule engine and waits for all nodes in the rule chain to complete before returning.
// OnMsgAndWait uses the rule engine to synchronously process messages and waits for all nodes in the rule chain to complete before returning.
func (e *RuleEngine) OnMsgAndWait(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, true, opts...)
}

// RootRuleContext returns the root rule context for advanced operations.
// RootRuleContext returns the root rule context used for advanced operations.
func (e *RuleEngine) RootRuleContext() types.RuleContext {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.rootRuleContext
	}
	return nil
}

// GetMetrics returns engine metrics if the metrics aspect is enabled.
// If GetMetrics is enabled, it returns the engine metrics.
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
// OnMsgWithEndFunc is a deprecated method that uses a rule engine to process messages asynchronously.
// endFunc callbacks are used to retrieve results after the rule chain has completed execution.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Please switch to OnMsg.
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
// OnMsgWithOptions is a deprecated method that uses a rule engine to process messages asynchronously.
// It allows for carrying context options and ending callback options.
// Context is used to share data between different component instances.
// endFunc callbacks are used to retrieve results after the rule chain has completed execution.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Please switch to OnMsg.
func (e *RuleEngine) OnMsgWithOptions(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// doOnAllNodeCompleted handles the completion of all nodes within the rule chain.
// It executes aspects, completes the run snapshot, and triggers any custom callback functions.
// doOnAllNodeCompleted handles the completion of all nodes within the rule chain.
// It executes the aspects, completes the snapshot, and triggers any custom callback functions.
func (e *RuleEngine) doOnAllNodeCompleted(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, customFunc func()) {
	// Execute aspects upon completion of all nodes.
	// Run the aspects after all nodes have completed.
	e.onAllNodeCompleted(rootCtxCopy, msg)

	// Trigger custom callback if provided.
	// If a custom callback is provided, it is triggered.
	if customFunc != nil {
		customFunc()
	}
	// Complete the run snapshot if it exists.
	// If a snapshot exists, complete it.
	if rootCtxCopy.runSnapshot != nil {
		rootCtxCopy.runSnapshot.onRuleChainCompleted(rootCtxCopy)
	}

	// Reduce active message counts
	e.decrementActiveMessages()
}

// onErrHandler handles the scenario where the rule chain has no nodes or fails to process the message.
// It logs an error and triggers the end-of-chain callbacks.
// onErrHandler handles scenarios where the rule chain has no nodes or when message processing fails.
// It records errors and triggers a chain-end callback.
func (e *RuleEngine) onErrHandler(msg types.RuleMsg, rootCtxCopy *DefaultRuleContext, err error, needDecrement bool) {
	// Trigger the configured OnEnd callback with the error.
	// Use error-triggered configured OnEnd callbacks.
	if rootCtxCopy.config.OnEnd != nil {
		rootCtxCopy.config.OnEnd(rootCtxCopy, msg, err, types.Failure)
	}
	// Trigger the onEnd callback with the error and Failure relation type.
	// Trigger onEnd callbacks using error and failure relationship types.
	if rootCtxCopy.onEnd != nil {
		rootCtxCopy.onEnd(rootCtxCopy, msg, err, types.Failure)
	}
	// Execute the onAllNodeCompleted callback if it exists.
	// If there is an onAllNodeCompleted callback, execute it.
	if rootCtxCopy.onAllNodeCompleted != nil {
		rootCtxCopy.onAllNodeCompleted()
	}
	// Decrement active messages only if needed (when there was a corresponding increment)
	// Reduce the active message count only when needed (when there is a corresponding increase).
	if needDecrement {
		e.decrementActiveMessages()
	}
}

// onMsgAndWait processes a message through the rule engine with optional waiting for completion.
// This method implements a careful ordering of checks to prevent deadlocks during reload operations.
//
// Processing order to avoid deadlocks:
// Processing sequence to avoid deadlocks:
// 1. Check engine initialization status
// 2. Check shutdown status (before incrementing counters)
// 3. Check reload status and wait for completion (before incrementing counters)
// 4. Increment active message counter only after state checks
// 5. Process the message normally
//
// This ordering prevents the deadlock where:
// This sequence prevents deadlocks, where:
//   - Messages wait for reload to complete, but
//     The message is waiting for the reload to finish, but
//   - Reload waits for active message count to reach zero
//     Overload waits for the active message count to reach zero
//
// onMsgAndWait processes messages through a rule engine and can choose to wait for completion.
func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	// Check if the rule engine is initialized
	// Check whether the rule engine has been initialized
	if e.rootRuleChainCtx == nil {
		// Handle uninitialized engine error through callback if options are provided
		// If an option is provided, the error of not initializing the engine is handled via callback
		e.handleEngineNotInitializedError(msg, opts...)
		return
	}

	// Check if engine is shutting down first (before incrementing counter to avoid resource leak)
	// IMPORTANT: Check before incrementing counter to prevent resource leaks
	// First, check if the machine is being shut down (check before increasing the count to avoid resource leakage).
	// Important: Check before increasing the count to prevent resource leakage
	if e.IsShuttingDown() {
		// Create context and handle shutdown error through callback
		// Create context and handle downtime errors through callbacks
		rootCtxCopy := e.createRootContextCopy(msg, opts...)
		e.onErrHandler(msg, rootCtxCopy, types.ErrEngineShuttingDown, false)
		return
	}

	// Check if engine is reloading and wait for reload to complete (before incrementing counter)
	// CRITICAL: This prevents deadlock where messages wait for reload completion
	// while reload waits for active message count to reach zero
	// MEMORY SAFETY: Implements backpressure control to prevent memory overflow
	// Check if the engine is being reloaded and wait for the reload to complete (check before adding counts).
	// Crucially: This prevents deadlocks where messages wait for overload to complete while overloading waits for active message counts to reset to zero
	// Memory safety: Implements back pressure control to prevent memory overflow
	if e.IsReloading() {
		// Implement backpressure control to prevent memory overflow during reload
		// Backpressure control is implemented to prevent memory overflow during overload
		if !e.incrementReloadWaiters() {
			// Backpressure limit reached - reject message to prevent memory overflow
			// Reaching the backpressure limit - rejecting messages to prevent memory overflow
			rootCtxCopy := e.createRootContextCopy(msg, opts...)
			e.Config.Logger.Printf("RuleEngine: %s", types.ErrEngineReloadBackpressureLimit.Error())
			e.onErrHandler(msg, rootCtxCopy, types.ErrEngineReloadBackpressureLimit, false)
			return
		}

		// Ensure we decrement the waiter count when done
		// Ensure the waiting count is reduced upon completion
		defer e.decrementReloadWaiters()

		// Wait for reload to complete with timeout
		// Wait for the reload to complete and set timeout
		reloadTimeout := 30 * time.Second
		if !e.WaitForReloadComplete(reloadTimeout) {
			// Reload timeout, handle as error
			// Overload timeout is treated as an error
			rootCtxCopy := e.createRootContextCopy(msg, opts...)
			e.onErrHandler(msg, rootCtxCopy, errors.New("engine reload timeout"), false)
			return
		}
	}

	// Now increment active message count after all state checks pass
	// This ensures the counter is only incremented for messages that will actually be processed
	// After all status checks pass, the active message count is now increased
	// This ensures the counter only increases for the messages that will actually be processed
	e.incrementActiveMessages()

	// Double-check shutdown status after incrementing counter to handle race condition
	// If shutdown was initiated between our first check and counter increment,
	// we need to decrement the counter and exit to prevent Stop() from hanging
	// After adding the counter, check the downtime status again to handle race conditions
	// If a shutdown is initiated between our first check and the counter increase,
	// We need to reduce the counter and exit to prevent Stop() from being suspended
	if e.IsShuttingDown() {
		// Create context and handle shutdown error through callback
		// Create context and handle downtime errors through callbacks
		rootCtxCopy := e.createRootContextCopy(msg, opts...)
		e.onErrHandler(msg, rootCtxCopy, types.ErrEngineShuttingDown, true)
		return
	}

	// Create root context copy for message processing
	// Create a root context replica to process messages
	rootCtxCopy := e.createRootContextCopy(msg, opts...)

	// Apply graceful shutdown context handling
	// This combines user-provided context with shutdown context for proper cancellation
	// Apply elegant downtime context handling
	// This combines user-provided context with downtime context to achieve proper cancellation
	rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
	e.applyShutdownContext(rootCtxCopy, rootCtx)

	// Validate rule chain and context state
	// Verify the rule chain and context state
	if err := e.validateRuleChainState(rootCtxCopy); err != nil {
		e.onErrHandler(msg, rootCtxCopy, err, true)
		return
	}

	// Execute start aspects
	// Start the execution section
	processedMsg, err := e.onStart(rootCtxCopy, msg)
	if err != nil {
		e.onErrHandler(msg, rootCtxCopy, err, true)
		return
	}

	// Setup end callback wrapper
	// Set the end-callback wrapper
	e.setupEndCallback(rootCtxCopy)

	// Process message with or without waiting
	// Handle messages and choose whether to wait
	e.processMessage(rootCtxCopy, processedMsg, wait)
}

// processRestoreNodes handles multi-node recovery execution
// 1. Create the parent node context and set waitingCount
// 2. Traverse the recovery node, create a subcontext, and execute it
func (e *RuleEngine) processRestoreNodes(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg) {
	restoreInfo := rootCtxCopy.restoreNodeInfo
	// Retrieves the parent node context
	var parentNodeId string

	// Try to automatically find common ancestors
	if rootCtxCopy.ruleChainCtx != nil {
		var ruleNodeIds []types.RuleNodeId
		for _, req := range restoreInfo.NodeRequests {
			ruleNodeIds = append(ruleNodeIds, types.RuleNodeId{Id: req.NodeId})
		}
		if lca, ok := rootCtxCopy.ruleChainCtx.GetLCAOfNodes(ruleNodeIds); ok {
			parentNodeId = lca.Id
		}
	}

	var parentNode types.NodeCtx
	if node, ok := rootCtxCopy.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: parentNodeId}); ok {
		parentNode = node
	} else {
		// Parent node not found, error reported
		e.onErrHandler(msg, rootCtxCopy, fmt.Errorf("restore parent node id=%s not found", parentNodeId), true)
		return
	}

	// Create parentCtx
	parentCtx := rootCtxCopy.NewNextNodeRuleContext(parentNode)
	// Manually set self to parentNode
	parentCtx.self = parentNode
	// Set waitingCount
	parentCtx.waitingCount = int32(len(restoreInfo.NodeRequests))
	// rootCtxCopy is the root, and parentCtx is the fork node.
	parentCtx.parentRuleCtx = rootCtxCopy

	rootCtxCopy.childReady(msg, types.Success)

	// Traverse the recovery node
	for _, req := range restoreInfo.NodeRequests {
		if node, ok := rootCtxCopy.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: req.NodeId}); ok {
			// Create childCtx, where parent points to parentCtx
			childCtx := parentCtx.NewNextNodeRuleContext(node)
			childCtx.parentRuleCtx = parentCtx
			// If no relation is specified, execute the current node (isFirst = true)
			// If a relationship is specified, the current node is not executed, but the next node is found and executed (isFirst = false)
			childCtx.isFirst = len(req.RelationTypes) == 0
			childCtx.relationTypes = req.RelationTypes

			// Use the message from the request if available, otherwise use the default message
			var msgCopy types.RuleMsg
			if req.Msg != nil {
				msgCopy = req.Msg.Copy()
			} else {
				msgCopy = msg.Copy()
			}

			childCtx.TellNext(msgCopy, childCtx.relationTypes...)
		} else {
			// If the node cannot be found, it reduces waitingCount
			parentCtx.childDone()
			e.Config.Logger.Printf("Restore node id=%s not found", req.NodeId)
		}
	}

}

// onStart executes the list of start aspects before the rule chain begins processing a message.
// onStart executes the start plane list before the rule chain starts processing messages.
// handleEngineNotInitializedError handles the case when the rule engine is not initialized.
// handleEngineNotInitializedError handles cases where the rule engine is not initialized.
func (e *RuleEngine) handleEngineNotInitializedError(msg types.RuleMsg, opts ...types.RuleContextOption) {
	// Extract OnEnd callback from options if provided
	// Extracting OnEnd callbacks from options (if provided)
	var onEndCallback types.OnEndFunc
	for _, opt := range opts {
		// Create a temporary context to extract the OnEnd callback
		// Create a temporary context to extract OnEnd callbacks
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
	// If available, use an initialization error to trigger the OnEnd callback
	if onEndCallback != nil {
		onEndCallback(nil, msg, types.ErrEngineNotInitialized, types.Failure)
	}
}

// createRootContextCopy creates a copy of the root context for message processing.
// createRootContextCopy creates a copy of the root context to process messages.
func (e *RuleEngine) createRootContextCopy(msg types.RuleMsg, opts ...types.RuleContextOption) *DefaultRuleContext {
	rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
	rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.ruleChainPool)
	rootCtxCopy.isFirst = rootCtx.isFirst
	rootCtxCopy.runSnapshot = NewRunSnapshot(msg.Id, rootCtxCopy.ruleChainCtx, time.Now().UnixMilli())

	// Create a new nodeOutputCache instance for current message processing and set cross-node dependencies
	rootCtxCopy.nodeOutputCache.SetCacheableNodes(rootCtx.ruleChainCtx.referencedNodes)

	// Precompute chain-level debugMode to debugModeOverride
	if rootCtxCopy.ruleChainCtx != nil && rootCtxCopy.ruleChainCtx.IsDebugMode() {
		atomic.StoreInt32(&rootCtxCopy.debugModeOverride, 1)
	}

	// Apply the WithDebugMode option, which overrides the precomputed values above
	for _, opt := range opts {
		opt(rootCtxCopy)
	}

	return rootCtxCopy
}

// validateRuleChainState validates the rule chain and context state.
// validateRuleChainState verifies the rule chain and context state.
func (e *RuleEngine) validateRuleChainState(rootCtxCopy *DefaultRuleContext) error {
	// Check if the rule chain has no nodes
	// Check whether the rule chain has no nodes
	if rootCtxCopy.ruleChainCtx.isEmpty {
		return types.ErrRuleChainHasNoNodes
	}

	// Check if there's an error in the context
	// Check for errors in context
	if rootCtxCopy.err != nil {
		return rootCtxCopy.err
	}

	return nil
}

// setupEndCallback sets up the end callback wrapper for the context.
// setupEndCallback sets the context to end the callback wrapper.
func (e *RuleEngine) setupEndCallback(rootCtxCopy *DefaultRuleContext) {
	customOnEndFunc := rootCtxCopy.onEnd
	rootCtxCopy.onEnd = func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		// Execute end aspects and update the message accordingly
		// Execute the end plane and update the message accordingly
		msg = e.onEnd(rootCtxCopy, msg, err, relationType)
		// Trigger the custom end callback if provided
		// If a custom end callback is provided, it is triggered
		if customOnEndFunc != nil {
			customOnEndFunc(ctx, msg, err, relationType)
		}
	}
}

// processMessage processes the message through the rule chain with optional waiting.
// processMessage processes messages through a chain of rules, and you can choose to wait.
func (e *RuleEngine) processMessage(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, wait bool) {
	// Set up a custom function to be called upon completion of all nodes
	// Set the custom function to call when all nodes are completed
	customFunc := rootCtxCopy.onAllNodeCompleted

	if wait {
		// If waiting is required, set up a channel to synchronize the completion
		// If waiting is needed, set up channels to synchronize completion
		c := make(chan struct{})
		rootCtxCopy.onAllNodeCompleted = func() {
			defer close(c)
			// Execute the completion handling function
			// Execute the completion processing function
			e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
		}
		// Process the message through the rule chain
		// Messages are processed through a rule chain
		if rootCtxCopy.restoreNodeInfo != nil {
			e.processRestoreNodes(rootCtxCopy, msg)
		} else {
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
		}
		// Block until all nodes have completed
		// Blocking until all nodes are completed
		<-c
	} else {
		// If not waiting, simply set the completion handling function
		// If you don't wait, just set the completion function
		rootCtxCopy.onAllNodeCompleted = func() {
			e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
		}
		// Process the message through the rule chain
		// Messages are processed through a rule chain
		if rootCtxCopy.restoreNodeInfo != nil {
			e.processRestoreNodes(rootCtxCopy, msg)
		} else {
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
		}
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
// onEnd executes the completion aspects at the end of a rule chain branch.
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
// onAllNodeCompleted executes the completed facet list after all branches of the rule chain have finished.
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
// getAspectsHolder uses atomic operations to securely obtain the facet holder, a high-performance method to avoid lock contention.
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
// NewConfig creates a new configuration and applies options.
// It initializes all necessary components using reasonable default values.
//
// Parameters:
// Parameters:
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - types.Config: Initialized configuration
//
// Default components include:
// The default components include:
//   - JSON parser for rule chain definitions
//   - Default component registry with built-in components
//   - User-defined functions registry
//   - Default cache implementation
func NewConfig(opts ...types.Option) types.Config {
	c := types.NewConfig(opts...)
	if c.Parser == nil {
		c.Parser = &JsonParser{}
	}
	if c.ComponentsRegistry == nil {
		c.ComponentsRegistry = Registry
	}
	// register all udfs
	// Register all user-defined functions
	for name, f := range funcs.ScriptFunc.GetAll() {
		c.RegisterUdf(name, f)
	}
	if c.Cache == nil {
		c.Cache = cache.DefaultCache
	}
	return c
}

// WithConfig is an option that sets the Config of the RuleEngine.
// WithConfig is an option to set the RuleEngine configuration.
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
// SetMaxReloadWaiters sets the maximum number of concurrent goroutines that can wait for the overload to complete.
// This prevents memory overflow in high-traffic heavy load scenarios.
//
// Parameters:
// Parameters:
//   - maxWaiters: Maximum number of concurrent goroutines allowed to wait
//     If 0, disables the limit (unlimited waiters)
//     If negative, keeps current setting unchanged
//     maxWaiters: The maximum number of concurrent goroutines allowed to wait
//     If set to 0, disable the limit (Infinite Waiter)
//     If the number is negative, keep the current setting unchanged
//
// Thread Safety:
// Thread safety:
//
//	This method is thread-safe and can be called during message processing.
//	This method is thread-safe and can be called during message processing.
func (e *RuleEngine) SetMaxReloadWaiters(maxWaiters int64) {
	if maxWaiters < 0 {
		// Keep current setting unchanged for negative values
		// If the number is negative, keep the current setting unchanged
		return
	} else if maxWaiters == 0 {
		// Disable backpressure control (unlimited waiters)
		// Disable backpressure control (Infinite Waiter)
		e.reloadBackpressureEnabled = false
		atomic.StoreInt64(&e.maxConcurrentReloadWaiters, 0)
	} else {
		// Enable backpressure control with specified limit
		// Enable backpressure control and set specified limits
		e.reloadBackpressureEnabled = true
		atomic.StoreInt64(&e.maxConcurrentReloadWaiters, maxWaiters)
	}
}

// GetReloadWaitersStats returns current reload waiters statistics for monitoring.
// This provides insight into reload behavior under load.
//
// GetReloadWaitersStats returns statistics for monitoring the current overloaded waiters.
// This provides insights into overload behavior under load.
//
// Returns:
// Returns:
//   - maxWaiters: Maximum allowed concurrent waiters (0 means unlimited)
//     maxWaiters: Maximum allowed concurrent waiters (0 means unlimited)
//   - currentWaiters: Current number of goroutines waiting for reload
//     currentWaiters: The current number of goroutines waiting to be overloaded
//   - isReloading: Whether engine is currently reloading
//     isReloading: Is the engine currently being reloaded?
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
// incrementReloadWaiters atomically increases the count of overloaded waiters.
// If the increase exceeds the maximum allowed number of waiters, return false.
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
// decrementReloadWaiters atomically reduces the count of overloaded waiters.
func (e *RuleEngine) decrementReloadWaiters() {
	if e.reloadBackpressureEnabled {
		atomic.AddInt64(&e.currentReloadWaiters, -1)
	}
}
