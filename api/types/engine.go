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

import (
	"context"

	"github.com/rulego/rulego/api/types/metrics"
)

// RuleEngineOption defines a function type for configuring a RuleEngine.
// It follows the functional options pattern for flexible configuration.
//
// RuleEngineOption defines the function type used to configure RuleEngine.
// It follows a function option pattern, offering flexible configuration.
//
// Example usage:
// Example:
//
//	engine, err := rulego.New("myEngine", ruleChainDef,
//		types.WithConfig(myConfig),
//		types.WithAspects(debugAspect, metricsAspect))
type RuleEngineOption func(RuleEngine) error

// WithConfig creates a RuleEngineOption to set the configuration of a RuleEngine.
// This allows customizing the engine's behavior, logging, caching, and other settings.
//
// WithConfig creates a RuleEngineOption to set the configuration of the RuleEngine.
// This allows for custom engine behavior, logging records, caching, and other settings.
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
// WithAspects creates a RuleEngineOption that sets the RuleEngine aspects.
// The Face-to-Face (Face-to-Face) functionality provides AOP (Face-to-Face Programming) for cross-cutting concerns such as logging, metrics, validation, and debugging.
func WithAspects(aspects ...Aspect) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetAspects(aspects...) // Apply the provided aspects to the RuleEngine.
		return nil                // Return no error.
	}
}

// WithRuleEnginePool creates a RuleEngineOption to set the rule engine pool.
// This enables the engine to manage sub-rule chains and cross-chain communication.
//
// WithRuleEnginePool creates a RuleEngineOption to set the Rule Engine pool.
// This enables the engine to manage sub-rule chains and cross-chain communication.
func WithRuleEnginePool(ruleEnginePool RuleEnginePool) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetRuleEnginePool(ruleEnginePool)
		return nil
	}
}

// WithMaxReloadWaiters creates a RuleEngineOption to limit concurrent goroutines waiting for reload completion.
// This prevents memory overflow during high-traffic reload operations by controlling the maximum number
// of messages that can wait for reload to finish.
//
// WithMaxReloadWaiters creates a RuleEngineOption to limit the number of concurrent goroutines waiting for the reload to complete.
// This controls the maximum number of messages that can wait for the overload to complete, preventing memory overflow during high-traffic overload operations.
//
// Parameters:
// Parameters:
//   - maxWaiters: Maximum number of concurrent goroutines allowed to wait for reload
//     If 0, disables the limit (unlimited waiters - use with caution)
//     If negative, uses default value (1000)
//     maxWaiters: The maximum number of concurrent goroutines allowed to wait for the overload
//     If set to 0, disable restrictions (Infinite Waiters - Use with caution)
//     If the number is negative, use the default value (1000)
//
// Memory Safety Benefits:
// Memory Security Advantages:
//   - Prevents unlimited goroutine creation during reload
//   - Avoids memory overflow in high-traffic scenarios
//   - Provides predictable memory usage patterns
//   - Enables graceful degradation under load
//
// Usage Examples:
// Example:
//
//	// Limit to 500 concurrent waiters (recommended for high-traffic systems)
//	Limited to 500 concurrent waiters (recommended for high-traffic systems)
//	engine, err := NewRuleEngine("myEngine", dsl, WithMaxReloadWaiters(500))
//
//	// Disable limit (allow unlimited waiters - use with caution in production)
//	Disable restrictions (allow unlimited waiters – use cautiously in production environments)
//	engine, err := NewRuleEngine("myEngine", dsl, WithMaxReloadWaiters(0))
//
//	// Use default limit (1000 waiters)
//	Use default limit (1000 waiters)
//	engine, err := NewRuleEngine("myEngine", dsl, WithMaxReloadWaiters(-1))
func WithMaxReloadWaiters(maxWaiters int64) RuleEngineOption {
	return func(re RuleEngine) error {
		re.SetMaxReloadWaiters(maxWaiters)
		return nil
	}
}

// RuleEngine is the core interface for a rule engine instance.
// Each RuleEngine manages a single root rule chain and provides methods for
// message processing, configuration updates, and lifecycle management.
//
// RuleEngine is the core interface of the Rule Engine instance.
// Each RuleEngine manages a root rule chain and provides methods for message processing, configuration updates, and lifecycle management.
//
// Key Features:
// Key features:
//   - Rule chain execution and management
//   - Dynamic configuration reloading
//   - Aspect-oriented programming support
//   - Performance metrics collection
//   - Concurrent message processing
//
// Lifecycle:
// Lifecycle:
//  1. Create engine with New() or Load()
//  2. Process messages with OnMsg()
//  3. Update configuration with ReloadSelf()
//  4. Clean up resources with Stop()
type RuleEngine interface {
	// Id returns the unique identifier of the RuleEngine.
	// This ID is used for engine lookup and management within pools.
	// Id returns a unique identifier for RuleEngine.
	// This ID is used for engine lookup and management in the pool.
	Id() string

	// SetConfig sets the configuration for the RuleEngine.
	// This affects logging, caching, component registry, and other engine behaviors.
	// SetConfig sets the configuration of the RuleEngine.
	// This affects logging records, caching, component registry, and other engine behaviors.
	SetConfig(config Config)

	// SetAspects sets the aspects for the RuleEngine.
	// Aspects provide cross-cutting functionality like metrics, debugging, and validation.
	// SetAspects sets the RuleEngine aspects.
	// The face-to-face offers cross-cutting functions such as metrics, debugging, and validation.
	SetAspects(aspects ...Aspect)

	// SetRuleEnginePool sets the rule engine pool for the RuleEngine.
	// This enables sub-rule chain execution and cross-chain communication.
	// SetRuleEnginePool sets the rule engine pool for RuleEngine.
	// This enables sub-rule chain execution and cross-chain communication.
	SetRuleEnginePool(ruleEnginePool RuleEnginePool)

	// Reload reloads the RuleEngine with the given options.
	// This refreshes the current rule chain configuration while applying new options.
	// Reload Reloads RuleEngine using the given option.
	// This refreshes the current rule chain configuration while applying new options.
	Reload(opts ...RuleEngineOption) error

	// ReloadSelf reloads the RuleEngine itself with the given definition and options.
	// This completely replaces the current rule chain with a new configuration.
	// ReloadSelf reloads RuleEngine itself using given definitions and options.
	// This completely replaces the current rule chain with new configurations.
	ReloadSelf(def []byte, opts ...RuleEngineOption) error

	// ReloadChild reloads a specific child node within the RuleEngine.
	// This allows partial updates without affecting the entire rule chain.
	// ReloadChild Reloads specific child nodes in the RuleEngine.
	// This allows for partial updates without affecting the entire rule chain.
	ReloadChild(ruleNodeId string, dsl []byte) error

	// DSL returns the DSL (Domain Specific Language) representation of the RuleEngine.
	// This provides the complete rule chain configuration in serialized format.
	// DSL returns the DSL (domain-specific language) representation of RuleEngine.
	// This provides a complete rule chain configuration in serialized format.
	DSL() []byte

	// Definition returns the structured definition of the rule chain.
	// This provides programmatic access to the rule chain structure.
	// Definition Returns a structured definition of the rule chain.
	// This provides programmatic access to the rule chain structure.
	Definition() RuleChain

	// RootRuleChainCtx returns the context of the root rule chain.
	// This provides access to the chain's execution context and management methods.
	// RootRuleChainCtx returns the context of the root rule chain.
	// This provides access to the chain's execution context and management methods.
	RootRuleChainCtx() ChainCtx

	// NodeDSL returns the DSL of a specific node within the rule chain.
	// This enables inspection and management of individual nodes.
	// NodeDSL returns the DSL for a specific node in the rule chain.
	// This enables inspection and management of individual nodes.
	NodeDSL(chainId RuleNodeId, childNodeId RuleNodeId) []byte

	// Initialized checks if the RuleEngine is properly initialized and ready for use.
	// Returns true if the engine has a valid rule chain configuration.
	// Initialized checks whether the RuleEngine has been properly initialized and ready for use.
	// If the engine has a valid rule chain configuration, it returns true.
	Initialized() bool

	// IsShuttingDown returns whether the RuleEngine is currently in shutdown process.
	// This can be used to check shutdown status before performing operations.
	// IsShuttingDown returns whether the RuleEngine is currently in a shutdown process.
	// This can be used to check the shutdown status before performing operations.
	IsShuttingDown() bool

	// Stop shuts down the RuleEngine and releases all resources.
	// If ctx is provided, it will wait for active messages to complete within the context deadline.
	// If ctx is no deadline, it uses a default 10-second timeout.
	// If ctx is nil, it performs immediate shutdown.
	// Stop to close RuleEngine and release all resources.
	// If CTX is provided, it will wait for the active message to complete during context cutoff time.
	// If CTX has no cut-off, use the default 10-second timeout.
	// If ctx is nil, an immediate shutdown is performed.
	Stop(ctx context.Context)

	// OnMsg processes a message asynchronously with the given context options.
	// This is the primary method for feeding data into the rule engine.
	// OnMsg processes messages asynchronously using given context options.
	// This is the main method for inputting data into the rule engine.
	OnMsg(msg RuleMsg, opts ...RuleContextOption)

	// OnMsgAndWait processes a message synchronously and waits for completion.
	// This blocks until all rule chain execution is complete.
	// OnMsgAndWait synchronously processes messages and waits for completion.
	// This will block until all rule chains are executed.
	OnMsgAndWait(msg RuleMsg, opts ...RuleContextOption)

	// RootRuleContext returns the root rule context for advanced operations.
	// This provides access to the execution context of the root rule chain.
	// RootRuleContext returns the root rule context used for advanced operations.
	// This provides access to the execution context of the root rule chain.
	RootRuleContext() RuleContext

	// GetMetrics returns performance and execution metrics of the RuleEngine.
	// This is useful for monitoring, debugging, and performance optimization.
	// GetMetrics returns RuleEngine's performance and execution metrics.
	// This is useful for monitoring, debugging, and performance optimization.
	GetMetrics() *metrics.EngineMetrics

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
	//   This method is thread-safe and can be called during message processing.
	//   This method is thread-safe and can be called during message processing.
	SetMaxReloadWaiters(maxWaiters int64)
}

// RuleEnginePool is an interface for managing a collection of rule engines.
// It provides centralized management, loading, and coordination of multiple rule engines.
//
// RuleEnginePool is an interface for managing collections of rule engines.
// It provides centralized management, loading, and coordination of multiple rule engines.
//
// Key Features:
// Key features:
//   - Centralized rule engine management
//   - Dynamic loading from file system
//   - Cross-engine message broadcasting
//   - Lifecycle management for all engines
//
// Usage Example:
// Example:
//
//	// Load all rule chains from a directory
//	Load all rule chains from the directory
//	err := pool.Load("./rules")
//
//	// Get a specific engine
//	Obtain a specific engine
//	engine, ok := pool.Get("engineId")
//
//	// Broadcast message to all engines
//	Broadcast messages to all engines
//	pool.OnMsg(message)
type RuleEnginePool interface {
	// Load loads all rule chain configurations from a specified folder and its subfolders
	// into the rule engine instance pool. The rule chain ID is taken from the ruleChain.id
	// specified in the rule chain file.
	// Load: Load loads all rule chains from the specified folder and its subfolders into the rule engine instance pool.
	// The rule chain ID is taken from the ruleChain.id specified in the rule chain file.
	Load(folderPath string, opts ...RuleEngineOption) error

	// New creates a new RuleEngine and stores it in the rule engine pool.
	// If the specified id is empty, the ruleChain.id from the rule chain file is used.
	// New creates a new RuleEngine and stores it in the Rule Engine pool.
	// If the specified id is null, use the ruleChain.id from the rule chain file.
	New(id string, rootRuleChainSrc []byte, opts ...RuleEngineOption) (RuleEngine, error)

	// Get retrieves a RuleEngine by its unique identifier.
	// Returns the engine and a boolean indicating whether it was found.
	// Get retrieves the RuleEngine using a unique identifier.
	// Returns the engine and indicates whether the boolean value was found.
	Get(id string) (RuleEngine, bool)

	// Del removes and stops a RuleEngine instance by its ID.
	// This gracefully shuts down the engine and releases its resources.
	// Del deletes and stops the RuleEngine instance by ID.
	// This elegantly shuts down the engine and releases its resources.
	Del(id string)

	// Stop gracefully shuts down and releases all RuleEngine instances in the pool.
	// This should be called during application shutdown.
	// Stop gracefully closes and releases all RuleEngine instances in the pool.
	// This should be called during the application shutdown.
	Stop()

	// OnMsg broadcasts a message to all RuleEngine instances in the pool.
	// Each engine will attempt to process the message according to its rule chain.
	// OnMsg broadcasts messages to all RuleEngine instances in the pool.
	// Each engine will try to process messages according to its own chain of rules.
	OnMsg(msg RuleMsg)

	// Reload reloads all RuleEngine instances in the pool with the given options.
	// This applies configuration changes to all engines simultaneously.
	// Reload reloads all RuleEngine instances in the pool using the given option.
	// This simultaneously applies configuration changes to all engines.
	Reload(opts ...RuleEngineOption)

	// Range iterates over all RuleEngine instances in the pool.
	// The function should return false to stop iteration.
	// Range traverses all RuleEngine instances in the pool.
	// The function should return false to stop iteration.
	Range(f func(key, value any) bool)
}
