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
	"math"
	"time"

	"github.com/rulego/rulego/utils/pool"
)

// Option is a function type that modifies the Config.
// Option is the function type that modifies the config.
//
// The Option pattern provides a flexible and extensible way to configure RuleGo instances.
// It allows users to specify only the configuration aspects they need while maintaining
// default values for other settings.
// The Option pattern provides a flexible and scalable way to configure RuleGo instances.
// It allows users to specify only the configuration aspects they need while keeping default values for other settings.
//
// Usage Pattern:
// Usage mode:
//
//	config := NewConfig(
//	    WithPool(customPool),
//	    WithLogger(customLogger),
//	    WithOnDebug(debugHandler),
//	)
type Option func(*Config) error

// WithComponentsRegistry is an option that sets the components' registry of the Config.
// WithComponentsRegistry is an option to set up the Config component registry.
//
// The components registry manages all available node types that can be used in rule chains.
// Setting a custom registry allows for component isolation, versioning, and custom component sets.
// The component registry manages all available node types in the rule chain.
// Setting up a custom registry allows component isolation, version control, and custom component sets.
//
// Use Cases:
// Use Cases:
//   - Multi-tenant applications with different component sets per tenant
//     Each tenant has a different set of components for multi-tenant applications
//   - Plugin-based architectures with dynamic component loading
//     A plugin-based architecture with dynamic component loading
//   - Testing environments with mock components
//     Test environments with simulated components
//   - Component versioning and A/B testing
//     Component version control and A/B testing
//
// Example:
// Example:
//
//	registry := &MyCustomRegistry{}
//	registry.Register(&MyCustomNode{})
//	config := NewConfig(WithComponentsRegistry(registry))
func WithComponentsRegistry(componentsRegistry ComponentRegistry) Option {
	return func(c *Config) error {
		c.ComponentsRegistry = componentsRegistry
		return nil
	}
}

// WithOnDebug is an option that sets the on debug callback of the Config.
// WithOnDebug is an option to set the Config debugging callback.
//
// The debug callback provides real-time visibility into message flow and processing
// within rule chains. It's essential for development, testing, and production monitoring.
// Debug callbacks provide real-time visibility into the message flow and processing within the rule chain.
// It is crucial for development, testing, and production monitoring.
//
// Callback Parameters:
// Callback parameters:
//   - ruleChainId: Identifier of the rule chain processing the message
//     ruleChainId: The rule chain identifier for processing messages
//   - flowType: Direction of message flow (IN/OUT)
//     flowType: Message flow direction (IN/OUT)
//   - nodeId: Identifier of the node processing the message
//     nodeId: The node identifier for processing messages
//   - msg: The message being processed
//     msg: Messages being processed
//   - relationType: Relationship type determining the flow path
//     relationType: Determines the type of relationship in the stream path
//   - err: Any error that occurred during processing
//     err: Any errors that occur during processing
//
// Example:
// Example:
//
//	debugHandler := func(chainId, flowType, nodeId string, msg RuleMsg, relationType string, err error) {
//	    log.Printf("[%s] %s -> %s: %s (%s)", chainId, flowType, nodeId, msg.Type, relationType)
//	    if err != nil {
//	        log.Printf("Error: %v", err)
//	    }
//	}
//	config := NewConfig(WithOnDebug(debugHandler))
func WithOnDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) Option {
	return func(c *Config) error {
		c.OnDebug = onDebug
		return nil
	}
}

// WithOnEndGlobal is an option that sets the global on end callback of the Config.
func WithOnEndGlobal(onEnd func(ctx RuleContext, msg RuleMsg, err error, relationType string)) Option {
	return func(c *Config) error {
		c.OnEnd = onEnd
		return nil
	}
}

// WithOnEndWithFailure is an option that sets the OnEndWithFailure of the Config.
// WithOnEndWithFailure is an option to set the Config's OnEndWithFailure.
//
// If true, the OnEnd callback will be triggered when no connected node is found and the relation type is Failure.
// If true, an OnEnd callback is triggered when no connected node is found and the relationship type is Failure.
func WithOnEndWithFailure(onEndWithFailure bool) Option {
	return func(c *Config) error {
		c.OnEndWithFailure = onEndWithFailure
		return nil
	}
}

// WithPool is an option that sets the pool of the Config.
// WithPool is an option to set up the Config pool.
//
// The worker pool controls concurrency and resource usage for rule chain execution.
// Proper pool configuration is crucial for performance and stability in production environments.
// Worker pools control concurrency and resource usage in the execution of the rule chain.
//
// Example:
// Example:
//
//	// Bounded pool for production
//	Bounded pools in the production environment
//	pool := &pool.WorkerPool{MaxWorkersCount: 100}
//	pool.Start()
//	config := NewConfig(WithPool(pool))
//
//	// Ants pool integration
//	Ants pool integration
//	antsPool, _ := ants.NewPool(50)
//	config := NewConfig(WithPool(antsPool))
func WithPool(pool Pool) Option {
	return func(c *Config) error {
		c.Pool = pool
		return nil
	}
}

// WithNodePool is an option that sets the netPool of the Config.
// WithNodePool is an option to set up a Config network pool.
//
// The network pool manages shared network resources like HTTP clients, database connections,
// and message queue connections across multiple rule chains. This enables resource reuse
// and connection pooling for improved performance and efficiency.
// The network pool manages shared network resources between multiple rule chains, such as HTTP clients, database connections, and message queue connections.
// This supports resource reuse and connection pools to improve performance and efficiency.
//
// Benefits of Network Pooling:
// Benefits of network pools:
//   - Reduced connection establishment overhead
//     Reduce connection setup overhead
//   - Better resource utilization and limits
//     Better resource utilization and constraints
//   - Consistent connection management
//     Consistent connection management
//   - Simplified configuration across chains
//     Simplified cross-chain configuration
//
// Common Use Cases:
// Common Use Cases:
//   - Database connection pooling
//     Database connection pool
//   - Message queue connection sharing
//     Message queue connection sharing
func WithNodePool(pool NodePool) Option {
	return func(c *Config) error {
		c.NodePool = pool
		return nil
	}
}

// WithDefaultPool creates an option that sets a default worker pool with unlimited capacity.
// WithDefaultPool creates an option to set the default worker pool with unlimited capacity.
func WithDefaultPool() Option {
	return func(c *Config) error {
		wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
		wp.Start()
		c.Pool = wp
		return nil
	}
}

// WithScriptMaxExecutionTime is an option that sets the js max execution time of the Config.
// WithScriptMaxExecutionTime is an option to set the maximum execution time of the Config script.
//
// This setting controls the maximum time allowed for script execution in script-enabled
// components (JavaScript). It prevents runaway scripts from consuming
// excessive resources or causing system hangs.
// This setting controls the maximum allowed time for script execution in components (JavaScript) that support scripts.
//
// Example:
// Example:
//
//	// Development environment with generous timeout
//	Develop a relaxed timeout environment
//	config := NewConfig(WithScriptMaxExecutionTime(5 * time.Second))
func WithScriptMaxExecutionTime(scriptMaxExecutionTime time.Duration) Option {
	return func(c *Config) error {
		c.ScriptMaxExecutionTime = scriptMaxExecutionTime
		return nil
	}
}

// WithParser is an option that sets the parser of the Config.
// WithParser is an option to set up the Config parser.
//
// The parser converts rule chain definitions from various formats (JSON, YAML, XML)
// into internal data structures. Custom parsers enable support for different
// configuration languages and specialized formats.
// The parser converts the rule chain definitions from various formats (JSON, YAML, XML) into internal data structures.
// Custom parsers support different configuration languages and dedicated formats.
//
// Parser Capabilities:
// Parser Functions:
//   - Bidirectional conversion (encode/decode)
//     Bidirectional conversion (encoding/decoding)
//   - Format validation and error reporting
//     Format verification and error reporting
//   - Custom extension support
//     Custom extension support
//   - Schema validation
//     Model validation
//
// Common Parser Types:
// Common types of parsers:
//   - JSON Parser: Default, widely supported
//     JSON parser: Default, widely supported
//   - YAML Parser: Human-readable, configuration-friendly
//     YAML parser: human-readable and user-friendly
//   - XML Parser: Enterprise integration, legacy systems
//     XML parser: enterprise integration, traditional systems
//   - Binary Parser: Performance-optimized formats
//     Binary parser: Performance-optimized format
//
// Custom Parser Implementation:
// Custom parser implementation:
// Implement the Parser interface to support custom formats or add
// validation, transformation, or encryption capabilities.
// Implement Parser interfaces to support custom formats or add authentication, conversion, or encryption features.
//
// Example:
// Example:
//
//	// YAML parser for configuration-friendly format
//	A YAML parser for configuring a user-friendly format
//	yamlParser := &YamlParser{}
//	config := NewConfig(WithParser(yamlParser))
//
//	// Encrypted parser for sensitive configurations
//	Cryptographers for sensitive configurations
//	encryptedParser := &EncryptedJsonParser{Key: secretKey}
//	config := NewConfig(WithParser(encryptedParser))
func WithParser(parser Parser) Option {
	return func(c *Config) error {
		c.Parser = parser
		return nil
	}
}

// WithLogger is an option that sets the logger of the Config.
// WithLogger is an option to set up a Config log recorder.
//
// Example:
// Example:
//
//	// Logrus integration
//	Logrus integration
//	logrusLogger := &LogrusLogger{Logger: logrus.New()}
//	config := NewConfig(WithLogger(logrusLogger))
//
//	// Custom logger with monitoring integration
//	Custom log recorder with integrated monitoring
//	monitoringLogger := &MonitoringLogger{Service: "rulego"}
//	config := NewConfig(WithLogger(monitoringLogger))
func WithLogger(logger Logger) Option {
	return func(c *Config) error {
		c.Logger = logger
		return nil
	}
}

// WithSecretKey is an option that sets the secret key of the Config.
// WithSecretKey is the option to set the Config key.
func WithSecretKey(secretKey string) Option {
	return func(c *Config) error {
		c.SecretKey = secretKey
		return nil
	}
}

// WithEndpointEnabled creates an Option to enable or disable the endpoint functionality in the Config.
// WithEndpointEnabled creates an option in the Config to enable or disable endpoint functionality.
func WithEndpointEnabled(endpointEnabled bool) Option {
	return func(c *Config) error {
		c.EndpointEnabled = endpointEnabled
		return nil
	}
}

// WithCache is an option that sets the cache of the Config.
// WithCache is an option to set the config cache.
func WithCache(cache Cache) Option {
	return func(c *Config) error {
		c.Cache = cache
		return nil
	}
}
