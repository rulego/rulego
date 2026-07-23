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
	"math"
	"strings"
	"time"

	"github.com/rulego/rulego/utils/pool"
)

// OnDebug is a global debug callback function for nodes.
// OnDebug is the global debugging callback function for nodes.
var OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)

// Config defines the configuration for the rule engine.
// Config defines the configuration of the rule engine.
//
// This structure contains all the necessary configuration parameters for initializing
// and running a RuleGo rule engine instance. It provides control over execution behavior,
// resource management, debugging, scripting, and integration with external systems.
// This structure contains all the configuration parameters needed to initialize and run the RuleGo rule engine instance.
// It provides control over execution behavior, resource management, debugging, scripting, and integration with external systems.
//
// Configuration Categories:
// Usage Example:
// Example:
//
//	config := NewConfig(
//	    WithPool(myPool),
//	    WithLogger(myLogger),
//	    WithOnDebug(debugHandler),
//	)
//	engine := rulego.New("chainId", chainDSL, rulego.WithConfig(config))
type Config struct {
	// OnDebug is a callback function for node debug information. It is only called if the node's debugMode is set to true.
	// - ruleChainId: The ID of the rule chain.
	// - flowType: The event type, either IN (incoming) or OUT (outgoing) for the component.
	// - nodeId: The ID of the node.
	// - msg: The current message being processed.
	// - relationType: If flowType is IN, it represents the connection relation between the previous node and this node (e.g., True/False).
	//                 If flowType is OUT, it represents the connection relation between this node and the next node (e.g., True/False).
	// - err: Error information, if any.
	// OnDebug is a callback function for node debug information. It is only called when the node's debugMode is set to true.
	// - ruleChainId: The ID of the rule chain
	// - flowType: Event type, component IN (pass-in) or OUT (output)
	// - nodeId: The node's ID
	// - msg: The current message being processed
	// - relationType: If flowType is IN, it indicates the connection between the previous node and this node (e.g., True/False)
	//                If flowType is OUT, it indicates the connection between this node and the next node (e.g., True/False)
	// - err: Error message (if any)
	//
	// This callback is essential for development, testing, and production monitoring.
	// It provides real-time visibility into message flow and transformation within rule chains.
	// This callback is crucial for development, testing, and production monitoring.
	// It provides real-time visibility into message flow and conversion within the rule chain.
	OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// The OnEnd rule chain subchain executes a full bureau callback
	OnEnd OnEndFunc
	// OnEndWithFailure indicates whether to trigger the OnEnd callback when no connected node is found and the relation type is Failure.
	// OnEndWithFailure means no connected node was found, and whether an OnEnd callback is triggered when the relationship type is Failure.
	OnEndWithFailure bool
	// ScriptMaxExecutionTime is the maximum execution time for scripts, defaulting to 2000 milliseconds.
	// ScriptMaxExecutionTime is the maximum execution time for a script, defaulting to 2000 milliseconds.
	//
	// This setting prevents runaway scripts from consuming excessive resources or causing
	// system hangs. It applies to JavaScript components
	// When a script exceeds this time limit, it will be terminated and an error will be returned.
	// This setting prevents runaway scripts from consuming excessive resources or causing system crashes.
	// It is suitable for components of JavaScript scripts.
	// If the script exceeds this time limit, it will be terminated and an error will be returned.
	//
	ScriptMaxExecutionTime time.Duration
	// Pool is the interface for a coroutine pool. If not configured, the go func method is used by default.
	// The default implementation is `pool.WorkerPool`. It is compatible with ants coroutine pool and can be implemented using ants.
	// Example:
	//   pool, _ := ants.NewPool(math.MaxInt32)
	//   config := rulego.NewConfig(types.WithPool(pool))
	// Pool is the interface for coroutine pools. If not configured, the go func method is used by default.
	// The default implementation is `pool.WorkerPool`. It is compatible with ants coroutine pools and can be implemented using ants.
	// Example:
	//   pool, _ := ants.NewPool(math.MaxInt32)
	//   config := rulego.NewConfig(types.WithPool(pool))
	//
	Pool Pool
	// ComponentsRegistry is the component registry for managing available rule chain components.
	// ComponentsRegistry is a component registry that manages available rule chain components.
	//
	// Key Features:
	//   - Component isolation: Supports different engine instances using independent component sets
	//   - Dynamic management: runtime component registration/unregistration and plugin loading
	//   - Visual support: Provides component metadata for UI configuration tools
	//
	// Configuration Examples:
	//
	//	Use custom component registry
	//	customRegistry := components.NewRegistry()
	//	customRegistry.Register(&MyCustomNode{})
	//	config := rulego.NewConfig(types.WithComponentsRegistry(customRegistry))
	//
	//	Dynamic plugin loading
	//	registry.RegisterPlugin("myPlugin", "./plugins/custom.so")
	//
	// By default, use `rulego.Registry`, which contains all standard components. For detailed features, please refer to the ComponentRegistry interface documentation.
	// Defaults to `rulego.Registry` with all standard components. See ComponentRegistry interface for detailed functionality.
	//
	ComponentsRegistry ComponentRegistry
	// Parser is the rule chain parser interface, defaulting to `rulego.JsonParser`.
	// Parser is the rule chain parser interface, defaulting to `rulego.JsonParser`.
	//
	// The parser converts rule chain definitions from various formats (JSON, YAML, XML)
	// into internal data structures that the engine can execute.
	// The parser converts the rule chain definition from various formats (JSON, YAML, XML) to
	// The internal data structures that engines can execute.
	//
	// Custom parsers can be implemented to support:
	// You can implement custom parsers to support:
	//   - Domain-specific configuration languages
	//     domain-specific configuration language
	//   - Legacy configuration formats
	//     Traditional configuration format
	//   - Compressed or encrypted rule definitions
	//     Rule definitions for compression or encryption
	//   - Runtime rule generation from databases
	//     Generate rules from the database runtime
	Parser Parser
	// Logger is the logging interface, defaulting to `DefaultLogger()`.
	// Logger is the log interface, defaulted to `DefaultLogger()`.
	//
	// The logger provides structured logging capabilities for the rule engine,
	// supporting different log levels and output formats.
	// The log recorder provides structured logging functionality for the rule engine,
	Logger Logger
	// Properties are global properties in key-value format.
	// Rule chain node configurations can replace values with ${global.propertyKey}.
	// Replacement occurs during node initialization and only once.
	// Properties are global properties in key-value format.
	// Rule chain node configuration can replace values with ${global.propertyKey}.
	// Replacement occurs during node initialization and only happens once.
	//
	// Example usage in rule configuration:
	// Example usage in rule configuration:
	//   {
	//     "type": "restApiCall",
	//     "configuration": {
	//       "restEndpointUrlPattern": "${global.apiBaseUrl}/users"
	//     }
	//   }
	Properties Properties
	// Udf is a map for registering custom Golang functions and native scripts that can be called at runtime by script engines like JavaScript Lua.
	// Function names can be repeated for different script types.
	// UDF is a mapping used to register custom Golang functions and native scripts, and can be called at runtime by scripting engines like JavaScript Lua.
	// Function names of different script types can be repeated.
	//
	// UDF (User Defined Functions) extend the scripting capabilities by providing:
	// UDF (User-Defined Function) extends scripting functionality by providing the following:
	//   - Access to Go standard library functions
	//     Access Go standard library functions
	//   - Custom business logic implementation
	//     Custom business logic implementation
	//   - Integration with external systems
	//     Integration with external systems
	//   - Performance-critical operations in native code
	//     Performance-critical operations in native code
	//
	// Function registration example:
	// Example of function registration:
	//   config.RegisterUdf("encrypt", func(data string) string {
	//       // Custom encryption logic
	//       return encryptedData
	//   })
	Udf map[string]interface{}
	// SecretKey is an AES-256 key of 32 characters in length, used for decrypting the `Secrets` configuration in the rule chain.
	// SecretKey is a 32-character AES-256 key used to decrypt the `Secrets` configuration in the rule chain.
	SecretKey string
	// EndpointEnabled indicates whether the endpoint module in the rule chain DSL is enabled.
	// When enabled, the rule chain DSL can configure input endpoint components for external message ingestion.
	// EndpointEnabled indicates whether the endpoint module in the Rule Chain DSL is enabled.
	// When enabled, the Rule Chain DSL can be configured with input endpoint components for external message access.
	//
	// DSL configuration example - DSL configuration example:
	//	{
	//	  "ruleChain": {...},
	//	  "endpoints": [{
	//	    "id": "restEndpoint",
	//	    "type": "rest",
	//	    "configuration": {"port": 8080}
	//	  }]
	//	}
	EndpointEnabled bool
	// NodePool is the interface for a shared Component Pool.
	// NodePool is an interface for sharing component pools.
	//
	// The network pool manages shared network resources such as HTTP clients,
	// database connections, and message queue connections across multiple rule chains.
	// This enables resource reuse and connection pooling for improved performance.
	// The network pool manages shared network resources between multiple rule chains,
	// Such as HTTP clients, database connections, and message queue connections.
	// This supports resource reuse and connection pools to improve performance.
	NodePool NodePool
	// NodeClientInitNow indicates whether to initialize the net client node immediately after creation.
	//True: During the component's Init phase, the client connection is established. If the client initialization fails, the rule chain initialization fails.
	//False: During the component's OnMsg phase, the client connection is established.
	// NodeClientInitNow indicates whether the network client node is initialized immediately after creation.
	// True: Establish a client connection during the component's Init phase. If the client initialization fails, the rule chain initialization fails.
	// False: Establishes a client connection during the component's OnMsg phase.
	NodeClientInitNow bool
	// AllowCycle indicates whether nodes in the rule chain are allowed to form cycles.
	// AllowCycle indicates whether nodes in the rule chain are allowed to form loops.
	AllowCycle bool
	// Cache is a global cache instance shared across all rule chains in the pool, used for storing runtime shared data.
	// A cache is a global cache instance shared by all rule chains in the pool, used to store runtime shared data.
	//
	// Cache Implementation:
	//   - Default implementation: uses in-memory cache (utils/cache.MemoryCache) - Default: uses in-memory cache (utils/cache. MemoryCache)
	//   - Custom implementation: Users can implement the Cache interface for external cache
	//   - Supports Redis, Memcached, and other distributed caches
	//
	// Default Configuration:
	//   If not specified, the system will use cache.DefaultCache (in-memory cache, 5-minute GC cycle)
	//   If not specified, system uses cache.DefaultCache (in-memory cache with 5-minute GC cycle)
	//
	// Custom Examples:
	//
	//	Using Redis cache
	//	redisCache := &MyRedisCache{client: redisClient}
	//	config := NewConfig(WithCache(redisCache))
	//
	//	Using memory cache (custom GC interval)
	//	memCache := cache.NewMemoryCache(time.Minute * 10)
	//	config := NewConfig(WithCache(memCache))
	Cache Cache
}

// RegisterUdf registers a custom function. Function names can be repeated for different script types.
// RegisterUdf registers custom functions. Function names of different script types can be repeated.
//
// This method provides a convenient way to register User Defined Functions (UDFs) that can be
// called from script components. It handles function name resolution and conflict prevention
// for different script engines.
// This method provides a convenient way to register user-defined functions (UDFs) that can be called from script components.
// It handles function name parsing and conflict prevention for different scripting engines.
//
// Function Registration Process:
// Function registration process:
//  1. Initialize Udf map if not already created
//     If not yet created, initialize the UDF mapping
//  2. Check if value is a Script type with specific engine
//     Check if the value is a script type with a specific engine
//  3. Resolve naming conflicts using script type prefixes
//     Use script type prefixes to resolve naming conflicts
//  4. Store function with resolved name
//     Functions stored using the name of parsing
//
// Examples:
// Example:
//
//	// Register a Go function for all script types
//	Register Go functions for all script types
//	config.RegisterUdf("stringUtils", myStringUtilsFunc)
//
//	// Register a JavaScript-specific function
//	Register JavaScript-specific functions
//	config.RegisterUdf("jsHelper", Script{
//	    Type: "Js",
//	    Content: "function jsHelper(data) { return data.toUpperCase(); }"
//	})
//
//	// Register a Lua-specific function
//	Register Lua-specific functions
//	config.RegisterUdf("luaHelper", Script{
//	    Type: "Lua",
//	    Content: "function luaHelper(data) return string.upper(data) end"
//	})
func (c *Config) RegisterUdf(name string, value interface{}) {
	if c.Udf == nil {
		c.Udf = make(map[string]interface{})
	}
	if script, ok := value.(Script); ok {
		if script.Type != AllScript {
			// Resolve function name conflicts for different script types.
			// Resolve function name conflicts between different script types.
			name = script.Type + ScriptFuncSeparator + name
		}
	}
	c.Udf[name] = value
}

// GetUdf returns the UDF by name and script type.
// GetUdf returns UDF by name and script type.
//
// If scriptType is empty, it returns the UDF with the name directly.
// If scriptType is not empty, it returns the UDF with the name prefixed by scriptType.
// If scriptType is empty, it returns a UDF named name directly.
// If scriptType is not empty, it returns a UDF with the scriptType prefix.
func (c *Config) GetUdf(name string, scriptType string) interface{} {
	if c.Udf == nil {
		return nil
	}
	var udf interface{}
	var ok bool
	if scriptType == "" {
		udf, ok = c.Udf[name]
	} else {
		key := scriptType + ScriptFuncSeparator + name
		udf, ok = c.Udf[key]
		// Try without prefix if not found with prefix (fallback mechanism if needed, though RegisterUdf ensures consistency)
		// Or if the user registered it without prefix but specified type?
		// RegisterUdf logic: if type != AllScript, it adds prefix.
		// So strict matching is preferred.
	}

	if ok {
		if script, ok := udf.(Script); ok {
			return script.Content
		}
		return udf
	}
	return nil
}

// GetUdfs returns a map of UDFs that satisfy the provided script type.
// GetUdfs returns a UDF mapping that meets the provided script type.
//
// If scriptType is empty, it returns all UDFs.
// If scriptType is empty, all UDFs are returned.
func (c *Config) GetUdfs(scriptType string) map[string]interface{} {
	udfs := make(map[string]interface{})
	for k, v := range c.Udf {
		if scriptType == "" {
			udfs[k] = v
			continue
		}
		if script, ok := v.(Script); ok {
			if script.Type == scriptType {
				// Remove the prefix from the function name.
				// Remove prefixes from function names.
				name := strings.TrimPrefix(k, script.Type+ScriptFuncSeparator)
				udfs[name] = script.Content
			}
		}
	}
	return udfs
}

// NewConfig creates a new Config with default values and applies the provided options.
// NewConfig creates a new Config with default values and applies the provided options.
//
// This function implements the functional options pattern, allowing for flexible
// and extensible configuration. It sets reasonable defaults while enabling
// This function implements a functional options pattern, allowing flexible and extensible configuration.
//
// Usage Examples:
// Example:
//
//	// Basic configuration with defaults
//	Basic configuration with default values
//	config := NewConfig()
//
//	// Configuration with custom options
//	Configuration with custom options
//	config := NewConfig(
//	    WithPool(customPool),
//	    WithLogger(customLogger),
//	    WithScriptMaxExecutionTime(5 * time.Second),
//	    WithEndpointEnabled(false),
//	)
func NewConfig(opts ...Option) Config {
	c := &Config{
		ScriptMaxExecutionTime: time.Millisecond * 2000,
		Logger:                 DefaultLogger(),
		Properties:             NewProperties(),
		EndpointEnabled:        true,
		OnEndWithFailure:       true,
	}

	for _, opt := range opts {
		_ = opt(c)
	}
	return *c
}

// DefaultPool provides a default coroutine pool.
// DefaultPool provides a default coroutine pool.
//
// This function creates and returns a default WorkerPool implementation with
// virtually unlimited capacity (math.MaxInt32 workers). The pool is immediately
// This function creates and returns a nearly unlimited capacity (math.MaxInt32 workers) is the default WorkerPool implementation.
func DefaultPool() Pool {
	wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	return wp
}
