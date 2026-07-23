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
// RouterOption defines the function type for configuring router components using option mode.
// This mode offers a flexible and scalable way to configure various router settings,
// While maintaining type safety and backward compatibility.
//
// # Design Benefits
//
// • Type Safety: Compile-time validation of configuration options
// • Extensibility: Easy to add new options without breaking existing code
// • Fluent Interface: Chainable configuration for better readability
// • Default Values: Options can provide sensible defaults
//
// Usage Pattern
//
//	options := []RouterOption{
//	    RouterOptions.WithRuleGo(ruleGo),
//	    RouterOptions.WithRuleConfig(config),
//	    RouterOptions.WithContextFunc(customContextFunc),
//	}
//	router.ApplyOptions(options...)
//
// # Error Handling
//
// RouterOption functions return an error to indicate configuration failures.
// This allows for graceful handling of invalid configurations.
// The RouterOption function returns an error indicating a configuration failure.
// This allows for the elegant handling of ineffective configurations.
type RouterOption func(OptionsSetter) error

// RouterOptions provides a global instance of router configuration options.
// This singleton instance offers convenient methods for creating router options
// without requiring explicit instantiation.
//
// RouterOptions provides a global instance of router configuration options.
// This singleton instance provides a convenient way to create router options,
// No explicit instantiation is required.
//
// # Usage
//
// The RouterOptions variable provides access to all available router configuration methods:
// The RouterOptions variable provides access to all available router configuration methods:
//
//	option1 := RouterOptions.WithRuleGo(ruleGo)
//	option2 := RouterOptions.WithRuleConfig(config)
//	option3 := RouterOptions.WithContextFunc(contextFunc)
//
// # Thread Safety
//
// The RouterOptions instance is stateless and thread-safe.
// The RouterOptions instance is stateless and thread-safe.
var RouterOptions = routerOptions{}

// routerOptions is the concrete implementation of router configuration options.
// It provides methods for creating RouterOption functions that configure various
// aspects of router behavior and integration.
//
// routerOptions are the specific implementation of router configuration options.
// It provides a method for creating RouterOption functions, which configure various aspects of router behavior and integration.
type routerOptions struct {
}

// WithRuleGoFunc creates a RouterOption that sets a function to dynamically determine the rule engine pool.
// This option enables advanced scenarios such as load balancing, multi-tenancy, and context-based pool selection.
//
// WithRuleGoFunc creates a RouterOption and sets the function to dynamically determine the rule engine pool.
// This option enables advanced scenarios such as load balancing, multi-tenancy, and context-based pool selection.
//
// Parameters
// • f: Function that returns a rule engine pool based on the exchange context
//
// Returns
// • RouterOption: Configuration function that applies the rule engine pool function
//
// # Function Behavior
//
// The provided function is called for each message exchange to determine which rule engine pool
// should process the message. This enables:
// A function provided for each message exchange call to determine which rule engine pool should handle the message. This enables:
//
// • Dynamic Pool Selection: Choose pools based on message content or metadata
// • Load Balancing: Distribute load across multiple pools
// • Multi-tenancy: Route messages to tenant-specific pools
// • Failover: Implement pool failover mechanisms
//
// Example Usage
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
// WithRuleGo creates a RouterOption to set up a specific rule engine pool for routers.
// This option configures routers to handle all messages using a fixed rule engine pool.
//
// Parameters
// • ruleGo: Rule engine pool instance to use for message processing
//
// Returns
// • RouterOption: Configuration function that applies the rule engine pool
//
// # Use Cases
//
// • Static Configuration: Use a predefined pool for all messages
// • Single-tenant Applications: Use a single shared pool
// • Development and Testing: Use a controlled pool environment
//
// # Default Behavior
//
// If no rule engine pool is specified, the router will use `rulego.DefaultPool`.
// If the rule engine pool is not specified, the router will use `rulego.DefaultPool`.
//
// Example Usage
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
// WithRuleConfig creates a RouterOption to configure the rule engine for the router.
// This configuration affects how the router interacts with the rule engine and processes messages.
//
// Parameters
// • config: Rule engine configuration containing various settings
//
// Returns
// • RouterOption: Configuration function that applies the rule engine configuration
//
// # Configuration Aspects
//
// The rule engine configuration can control:
// The rule engine configuration can control:
//
// • Script Execution: Timeout settings and script engine configuration
// • Component Registry: Available components and their registration
// • Logging and Debugging: Debug modes and logging configurations
// • Worker Pool: Concurrent processing settings
// • Global Properties: Shared configuration values
//
// Example Usage
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
// WithContextFunc creates a RouterOption and sets a context modification function for routing operations.
// This function can enhance or modify the context for each message exchange, enabling request-specific configurations and cross-cutting concerns.
//
// Parameters
// • f: Function that takes the current context and exchange and returns a modified context
//
// Returns
// • RouterOption: Configuration function that applies the context modification function
//
// # Context Enhancement Capabilities
//
// The context function can:
// The context function can be:
//
// • Add Request-specific Values: Inject request metadata, user information, etc.
// • Set Custom Timeouts: Apply different timeouts based on request type
// • Apply Security Context: Add authentication and authorization data
// • Enable Tracing: Add distributed tracing information
// • Inject Dependencies: Provide access to external services or resources
//
// # Function Execution
//
// The context function is called for each message exchange before rule chain execution.
// Before the rule chain executes, a context function is called for each message exchange.
//
// Example Usage
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
// WithDefinition creates a RouterOption to set the DSL definition for the router.
// This option provides access to the original DSL configuration for introspection, debugging, and advanced configuration scenarios.
//
// Parameters
// • def: Router DSL definition containing the original configuration structure
//
// Returns
// • RouterOption: Configuration function that applies the DSL definition
//
// DSL Definition Benefits / DSL Definition Benefits:
//
// • Configuration Introspection: Access to the complete original configuration
// • Debugging Support: Better error messages and debugging information
// • Dynamic Reconfiguration: Support for runtime configuration updates
// • Serialization: Ability to serialize and persist configuration
//
// # Use Cases
//
// • Configuration Management: Store and manage router configurations
// • Hot Reloading: Update router configuration without restart
// • Audit and Compliance: Track configuration changes and compliance
// • Template Processing: Support for configuration templates
//
// Example Usage
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
// DynamicEndpointOption defines the function type for configuring dynamic endpoint instances using option mode.
// This mode provides a flexible and type-safe way to configure various settings for dynamic endpoints,
// At the same time, it maintains backward compatibility and scalability.
//
// # Key Features
//
// • Runtime Configuration: Modify endpoint behavior at runtime
// • Hot Reloading: Support for configuration updates without restart
// • Type Safety: Compile-time validation of configuration options
// • Composable Options: Combine multiple options for complex configurations
//
// # Configuration Scope
//
// DynamicEndpointOption can configure:
// DynamicEndpointOption can be configured as:
//
// • Endpoint Identity: ID and naming configuration
// • Rule Engine Integration: Rule engine pools and configurations
// • Router Behavior: Default router options and settings
// • Event Handling: Event listeners and callbacks
// • Lifecycle Management: Restart policies and resource management
//
// Usage Pattern
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
// DynamicEndpointOptions provides a global instance of dynamic endpoint configuration options.
// This singleton instance provides a convenient way to create dynamic endpoint options,
// No explicit instantiation is required.
//
// # Global Access
//
// The DynamicEndpointOptions variable provides centralized access to all configuration methods:
// The DynamicEndpointOptions variable provides centralized access to all configuration methods:
//
//	option1 := DynamicEndpointOptions.WithId("endpoint-1")
//	option2 := DynamicEndpointOptions.WithConfig(config)
//	option3 := DynamicEndpointOptions.WithRestart(true)
//
// # Thread Safety
//
// The DynamicEndpointOptions instance is stateless and thread-safe.
// The DynamicEndpointOptions instance is stateless and thread-safe.
var DynamicEndpointOptions = dynamicEndpointOptions{}

// dynamicEndpointOptions is the concrete implementation of dynamic endpoint configuration options.
// It provides methods for creating DynamicEndpointOption functions that configure various
// aspects of dynamic endpoint behavior and lifecycle.
//
// dynamicEndpointOptions is the specific implementation of dynamic endpoint configuration options.
// It provides a method for creating DynamicEndpointOption functions that configure various aspects of dynamic endpoint behavior and lifecycle.
type dynamicEndpointOptions struct {
}

// WithId creates a DynamicEndpointOption that sets the unique identifier for the dynamic endpoint.
// This option is essential for endpoint identification, management, and reference within the system.
//
// WithId creates a DynamicEndpointOption to set a unique identifier for the dynamic endpoint.
// This option is crucial for endpoint identification, management, and reference within the system.
//
// Parameters
// • id: Unique identifier string for the endpoint
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the endpoint ID
//
// ID Requirements / ID Requirements:
//
// • Uniqueness: Must be unique within the endpoint pool or system
// • Persistence: Should remain stable across reloads and restarts
// • Readability: Should be human-readable for debugging and management
//
// # Use Cases
//
// • Endpoint Lookup: Find specific endpoints in pools or registries
// • Configuration Management: Associate configurations with specific endpoints
// • Monitoring and Logging: Track endpoint-specific metrics and logs
// • Hot Reloading: Update specific endpoints without affecting others
//
// Example Usage
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
// WithConfig creates a DynamicEndpointOption to configure the rule engine for dynamic endpoints.
// This configuration affects how endpoints interact with the rule engine and process messages.
//
// Parameters
// • config: Rule engine configuration containing various settings and options
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the rule engine configuration
//
// # Configuration Impact
//
// The rule engine configuration affects:
// Impact of rule engine configuration:
//
// • Message Processing: How messages are parsed, validated, and transformed
// • Component Behavior: Available components and their configurations
// • Performance Settings: Worker pools, timeouts, and resource limits
// • Debugging and Logging: Debug modes and logging configurations
// • Security Settings: Authentication, authorization, and encryption
//
// # Configuration Inheritance
//
// Endpoint-specific configurations override global defaults while preserving unspecified settings.
// Endpoint-specific configurations override global defaults while retaining unspecified settings.
//
// Example Usage
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
// WithRouterOpts creates a DynamicEndpointOption to set default router options for dynamic endpoints.
// These options apply to all routers created within the endpoint, providing consistent configuration.
//
// Parameters
// • opts: Variable number of router options to apply as defaults
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the default router options
//
// # Default Option Behavior
//
// • Global Application: Applied to all routers within the endpoint
// • Override Support: Individual routers can override these defaults
// • Consistency: Ensures consistent behavior across all endpoint routers
//
// # Common Default Options
//
// • Rule Engine Configuration: Default rule engine pools and configurations
// • Context Functions: Default context enhancement functions
// • Timeout Settings: Default timeout and retry configurations
// • Security Policies: Default authentication and authorization settings
//
// Example Usage
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
// WithOnEvent creates a DynamicEndpointOption to set the event handler for the dynamic endpoint.
// This processor receives notifications about endpoint lifecycle events and changes in operating status.
//
// Parameters
// • onEvent: Event handler function that processes endpoint events
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the event handler
//
// # Event Types
//
// The event handler can receive various types of events:
// Event processors can receive various types of events:
//
// • Lifecycle Events: Start, stop, reload, and destruction events
// • Connection Events: Client connections and disconnections
// • Error Events: Configuration errors and runtime failures
// • Performance Events: Throughput metrics and performance indicators
//
// # Event Handler Use Cases
//
// • Monitoring and Alerting: Track endpoint health and performance
// • Logging and Auditing: Record endpoint activities and changes
// • Auto-scaling: Trigger scaling based on load and performance
// • Integration: Notify external systems about endpoint status
//
// Example Usage
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
// WithRestart creates a DynamicEndpointOption to set the restart behavior of the dynamic endpoint.
// This option controls whether configuration changes trigger a full endpoint restart or only route updates.
//
// Parameters
// • restart: Boolean flag indicating whether to restart on configuration changes
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the restart behavior
//
// # Restart Behavior
//
// • restart = true: Full endpoint restart with complete reinitialization restart = true: Full endpoint restart and complete reinitialization
//   - Closes all existing connections
//   - Releases all resources
//   - Recreates the endpoint with new configuration
//   - Establishes new connections and resources
//
// • restart = false: Hot reload with minimal disruption restart = false: Thermal reload with minimal interference
//   - Updates routing rules without stopping the endpoint
//   - Preserves existing connections where possible
//   - Applies configuration changes gradually
//   - May force restart if routing conflicts occur
//
// # Decision Factors
//
// Choose restart = true for:
// Selecting restart = true applies to:
// • Major configuration changes (port, protocol, authentication)
// • Resource allocation changes
// • Breaking changes in routing structure
//
// Choose restart = false for:
// Choosing restart = false applies to:
// • Minor routing updates
// • Rule chain modifications
// • Non-breaking configuration adjustments
//
// Example Usage
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
// WithInterceptors creates a DynamicEndpointOption to set a global interceptor for dynamic endpoints.
// These interceptors are applied to all incoming messages, providing cross-cutting functionality,
// Such as authentication, logging, and message transformation.
//
// Parameters
// • interceptors: Variable number of processing functions to use as global interceptors
//
// Returns
// • DynamicEndpointOption: Configuration function that applies the interceptors
//
// # Interceptor Processing
//
// • Execution Order: Interceptors are executed in the order they are provided
// • Pipeline Behavior: If any interceptor returns false, processing stops
// • Global Scope: Applied to all routers and messages within the endpoint
//
// # Common Interceptor Types
//
// • Authentication: Verify user credentials and tokens
// • Authorization: Check permissions and access rights
// • Logging: Record request details and processing information
// • Rate Limiting: Control request frequency and throttling
// • Validation: Validate message format and content
// • Transformation: Modify message content or headers
// • Monitoring: Collect metrics and performance data
//
// # Implementation Guidelines
//
// • Performance: Keep interceptors lightweight and fast
// • Error Handling: Handle errors gracefully and provide meaningful messages
// • State Management: Avoid shared state between interceptor invocations
// • Idempotency: Ensure interceptors can be safely executed multiple times
//
// Example Usage
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
// WithRuleChain creates a DynamicEndpointOption and sets the original RuleChain DSL definition.
// This option is used when the endpoint initializes from the rule chain configuration,
// Retain the original DSL for reference and potential recovery.
//
// Parameters
// • ruleChain: Original rule chain DSL definition that created this endpoint
//
// Returns
// • DynamicEndpointOption: Configuration function that stores the rule chain definition
//
// # Purpose and Benefits
//
// • Historical Reference: Maintain link to the original configuration source
// • Configuration Restoration: Enable restoration to original configuration
// • Audit Trail: Track the origin and evolution of endpoint configuration
// • Template Processing: Support for configuration templates and inheritance
//
// # Use Cases
//
// • Rule Chain Integration: When endpoints are created from rule chain DSL
// • Configuration Management: Track configuration sources and dependencies
// • Rollback Support: Enable rollback to previous configurations
// • Documentation: Provide context for endpoint configuration decisions
//
// # Storage and Retrieval
//
// The stored rule chain can be retrieved using endpoint.GetRuleChain() for:
// You can use endpoint.GetRuleChain() retrieves the stored rule chain for:
// • Configuration introspection
// • Template processing
// • Debugging and troubleshooting
//
// Example Usage
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
