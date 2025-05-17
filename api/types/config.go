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
	"time"

	"github.com/rulego/rulego/api/pool"
)

// OnDebug is a global debug callback function for nodes.
var OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)

// Config defines the configuration for the rule engine.
type Config struct {
	// OnDebug is a callback function for node debug information. It is only called if the node's debugMode is set to true.
	// - ruleChainId: The ID of the rule chain.
	// - flowType: The event type, either IN (incoming) or OUT (outgoing) for the component.
	// - nodeId: The ID of the node.
	// - msg: The current message being processed.
	// - relationType: If flowType is IN, it represents the connection relation between the previous node and this node (e.g., True/False).
	//                 If flowType is OUT, it represents the connection relation between this node and the next node (e.g., True/False).
	// - err: Error information, if any.
	OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// OnEnd is a deprecated callback function that is called when the rule chain execution is complete. If there are multiple endpoints, it will be executed multiple times.
	// Deprecated: Use types.WithEndFunc instead.
	OnEnd func(msg RuleMsg, err error)
	// ScriptMaxExecutionTime is the maximum execution time for scripts, defaulting to 2000 milliseconds.
	ScriptMaxExecutionTime time.Duration
	// Pool is the interface for a coroutine pool. If not configured, the go func method is used by default.
	// The default implementation is `pool.WorkerPool`. It is compatible with ants coroutine pool and can be implemented using ants.
	// Example:
	//   pool, _ := ants.NewPool(math.MaxInt32)
	//   config := rulego.NewConfig(types.WithPool(pool))
	Pool Pool
	// ComponentsRegistry is the component registry, defaulting to `rulego.Registry`.
	ComponentsRegistry ComponentRegistry
	// Parser is the rule chain parser interface, defaulting to `rulego.JsonParser`.
	Parser Parser
	// Logger is the logging interface, defaulting to `DefaultLogger()`.
	Logger Logger
	// Properties are global properties in key-value format.
	// Rule chain node configurations can replace values with ${global.propertyKey}.
	// Replacement occurs during node initialization and only once.
	Properties Metadata
	// Udf is a map for registering custom Golang functions and native scripts that can be called at runtime by script engines like JavaScript.
	// Function names can be repeated for different script types.
	Udf map[string]interface{}
	// SecretKey is an AES-256 key of 32 characters in length, used for decrypting the `Secrets` configuration in the rule chain.
	SecretKey string
	// EndpointEnabled indicates whether the endpoint module in the rule chain DSL is enabled.
	EndpointEnabled bool
	// NetPool is the interface for a shared Component Pool.
	NetPool NodePool
	// NodeClientInitNow indicates whether to initialize the net client node immediately after creation.
	//True: During the component's Init phase, the client connection is established. If the client initialization fails, the rule chain initialization fails.
	//False: During the component's OnMsg phase, the client connection is established.
	NodeClientInitNow bool
	// AllowCycle indicates whether nodes in the rule chain are allowed to form cycles.
	AllowCycle bool
	// Cache is a global cache instance shared across all rule chains in the pool, used for storing runtime shared data.
	Cache Cache
}

// RegisterUdf registers a custom function. Function names can be repeated for different script types.
func (c *Config) RegisterUdf(name string, value interface{}) {
	if c.Udf == nil {
		c.Udf = make(map[string]interface{})
	}
	if script, ok := value.(Script); ok {
		// Resolve function name conflicts for different script types.
		name = script.Type + ScriptFuncSeparator + name
	}
	c.Udf[name] = value
}

// NewConfig creates a new Config with default values and applies the provided options.
func NewConfig(opts ...Option) Config {
	c := &Config{
		ScriptMaxExecutionTime: time.Millisecond * 2000,
		Logger:                 DefaultLogger(),
		Properties:             NewMetadata(),
		EndpointEnabled:        true,
	}

	for _, opt := range opts {
		_ = opt(c)
	}
	return *c
}

// DefaultPool provides a default coroutine pool.
func DefaultPool() Pool {
	wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	return wp
}
