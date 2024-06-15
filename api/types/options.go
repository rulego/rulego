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
	"github.com/rulego/rulego/api/pool"
	"math"
	"time"
)

// Option is a function type that modifies the Config.
type Option func(*Config) error

// WithComponentsRegistry is an option that sets the components' registry of the Config.
func WithComponentsRegistry(componentsRegistry ComponentRegistry) Option {
	return func(c *Config) error {
		c.ComponentsRegistry = componentsRegistry
		return nil
	}
}

// WithOnDebug is an option that sets the on debug callback of the Config.
func WithOnDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) Option {
	return func(c *Config) error {
		c.OnDebug = onDebug
		return nil
	}
}

// WithPool is an option that sets the pool of the Config.
func WithPool(pool Pool) Option {
	return func(c *Config) error {
		c.Pool = pool
		return nil
	}
}

func WithDefaultPool() Option {
	return func(c *Config) error {
		wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
		wp.Start()
		c.Pool = wp
		return nil
	}
}

// WithScriptMaxExecutionTime is an option that sets the js max execution time of the Config.
func WithScriptMaxExecutionTime(scriptMaxExecutionTime time.Duration) Option {
	return func(c *Config) error {
		c.ScriptMaxExecutionTime = scriptMaxExecutionTime
		return nil
	}
}

// WithParser is an option that sets the parser of the Config.
func WithParser(parser Parser) Option {
	return func(c *Config) error {
		c.Parser = parser
		return nil
	}
}

// WithLogger is an option that sets the logger of the Config.
func WithLogger(logger Logger) Option {
	return func(c *Config) error {
		c.Logger = logger
		return nil
	}
}

// WithSecretKey is an option that sets the secret key of the Config.
func WithSecretKey(secretKey string) Option {
	return func(c *Config) error {
		c.SecretKey = secretKey
		return nil
	}
}

// WithEndpointEnabled creates an Option to enable or disable the endpoint functionality in the Config.
func WithEndpointEnabled(endpointEnabled bool) Option {
	return func(c *Config) error {
		c.EndpointEnabled = endpointEnabled
		return nil
	}
}
