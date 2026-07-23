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
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

// DefaultPool is the default instance of the Pool.
// It provides a global pool for managing endpoint instances across the application.
// DefaultPool is the default instance of the Pool.
// It provides a global pool to manage endpoint instances across the entire application.
var DefaultPool = &Pool{factory: DefaultFactory}

// DefaultFactory is the default factory instance for creating endpoints.
// DefaultFactory is the default factory instance used to create endpoints.
var DefaultFactory = &Factory{registry: Registry}

// Ensure that Pool implements the endpoint.Pool interface.
var _ endpoint.Pool = (*Pool)(nil)

// Factory is a factory that creates Endpoints.
// It provides a centralized way to create different types of endpoints
// using consistent configuration patterns.
//
// A factory is the factory that creates endpoints.
// It provides a centralized way to create different types of endpoints using a consistent configuration pattern.
//
// Key Features:
// Key features:
//   - DSL-based endpoint creation
//   - Type-based endpoint instantiation
//   - Configuration management
//   - Component registry integration
type Factory struct {
	// registry provides access to registered endpoint components
	// The registry provides access to registered endpoint components
	registry *ComponentRegistry
}

// NewFromDsl creates a new DynamicEndpoint instance from DSL.
// This method parses JSON DSL and creates a configured dynamic endpoint.
//
// NewFromDsl creates a new DynamicEndpoint instance from the DSL.
// This method parses the JSON DSL and creates configured dynamic endpoints.
//
// Parameters:
// Parameters:
//   - dsl: JSON DSL configuration bytes JSON DSL configuration bytes
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - endpoint.DynamicEndpoint: Created dynamic endpoint
//   - error: Creation error if any
func (f *Factory) NewFromDsl(dsl []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return NewFromDsl(dsl, opts...)
}

// NewFromDef creates a new DynamicEndpoint instance from DSL definition structure.
// NewFromDef creates a new DynamicEndpoint instance from the DSL definition structure.
func (f *Factory) NewFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return NewFromDef(def, opts...)
}

// NewFromType creates a new Endpoint instance from type.
// This method provides type-based endpoint creation for programmatic use.
//
// NewFromType creates a new endpoint instance from the type.
// This method provides type-based endpoint creation for programming use.
//
// Parameters:
// Parameters:
//   - componentType: Type of endpoint to create
//   - ruleConfig: Rule engine configuration
//   - configuration: Endpoint-specific configuration
//
// Returns:
// Returns:
//   - endpoint.Endpoint: Created endpoint instance
//   - error: Creation error if any
func (f *Factory) NewFromType(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	return f.registry.New(componentType, ruleConfig, configuration)
}

// Pool is a structure that holds DynamicEndpoints.
// It provides centralized management of multiple endpoint instances with
// concurrent-safe operations and lifecycle management.
//
// Pool is the structure that stores DynamicEndpoints.
// It provides centralized management of multiple endpoint instances, with concurrent security operations and lifecycle management.
type Pool struct {
	// entries is a thread-safe map that stores DynamicEndpoints
	// using endpoint IDs as keys
	// entries are thread-safe maps storing DynamicEndpoints, using endpoint IDs as keys
	entries sync.Map

	// factory provides endpoint creation capabilities
	// factory provides endpoint creation functionality
	factory *Factory
}

// NewPool creates a new instance of a Pool.
// NewPool creates a new instance of the pool.
func NewPool() *Pool {
	return &Pool{
		factory: &Factory{
			registry: Registry,
		},
	}
}

// Factory returns the factory instance used by this pool.
// Factory returns the factory instance used by this pool.
func (p *Pool) Factory() endpoint.Factory {
	return p.factory
}

// New creates a new DynamicEndpoint instance with the specified ID.
// If the id is empty, it uses the id defined in def.
// This method implements a singleton pattern per ID.
//
// New Create a new DynamicEndpoint instance using the specified ID.
// If the id is null, use the id defined in def.
// This method implements the singleton pattern for each ID.
//
// Parameters:
// Parameters:
//   - id: Unique identifier for the endpoint
//   - def: JSON DSL definition bytes JSON DSL definition bytes
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - endpoint.DynamicEndpoint: Created or existing endpoint
//   - error: Creation error if any
func (p *Pool) New(id string, def []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	if v, ok := p.entries.Load(id); ok {
		return v.(endpoint.DynamicEndpoint), nil
	} else {
		if id != "" {
			opts = append(opts, endpoint.DynamicEndpointOptions.WithId(id))
		}
		if e, err := NewFromDsl(def, opts...); err != nil {
			return e, err
		} else {
			p.entries.Store(e.Id(), e)
			return e, nil
		}
	}
}

// Get retrieves a DynamicEndpoint instance by its ID.
// Get retrieves the DynamicEndpoint instance by ID.
func (p *Pool) Get(id string) (endpoint.DynamicEndpoint, bool) {
	v, ok := p.entries.Load(id)
	if ok {
		return v.(endpoint.DynamicEndpoint), ok
	} else {
		return nil, false
	}
}

// Del deletes a DynamicEndpoint instance by its ID.
// This method performs cleanup by calling Destroy() before removing the endpoint.
//
// Del deletes the DynamicEndpoint instance by its ID.
// This method calls Destroy() to perform cleanup before deleting endpoints.
func (p *Pool) Del(id string) {
	v, ok := p.entries.Load(id)
	if ok {
		v.(endpoint.DynamicEndpoint).Destroy()
		p.entries.Delete(id)
	}
}

// Stop releases all DynamicEndpoint instances.
// This method gracefully shuts down all endpoints in the pool.
//
// Stop to release all DynamicEndpoint instances.
// This method elegantly closes all endpoints in the pool.
func (p *Pool) Stop() {
	p.entries.Range(func(key, value any) bool {
		if item, ok := value.(endpoint.DynamicEndpoint); ok {
			item.Destroy()
		}
		p.entries.Delete(key)
		return true
	})
}

// Range iterates over all DynamicEndpoint instances.
// Range traverses all DynamicEndpoint instances.
func (p *Pool) Range(f func(key, value any) bool) {
	p.entries.Range(f)
}

// Reload reloads all DynamicEndpoint instances with the provided options.
// This method applies the same options to all endpoints in the pool.
//
// Reload uses the provided option to reload all DynamicEndpoint instances.
// This method applies the same options to all endpoints in the pool.
func (p *Pool) Reload(opts ...endpoint.DynamicEndpointOption) {
	DefaultPool.entries.Range(func(key, value any) bool {
		if item, ok := value.(endpoint.DynamicEndpoint); ok {
			_ = item.Reload(nil, opts...)
		}
		return true
	})
}

// New creates or retrieves a DynamicEndpoint instance with the specified ID from the default pool.
// If the id is empty, it uses the id defined in def.
//
// New Create or retrieve a DynamicEndpoint instance with the specified ID from the default pool.
// If the id is null, use the id defined in def.
//
// Parameters:
// Parameters:
//   - id: Unique identifier for the endpoint
//   - def: JSON DSL definition bytes JSON DSL definition bytes
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - endpoint.DynamicEndpoint: Created or existing endpoint
//   - error: Creation error if any
func New(id string, def []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return DefaultPool.New(id, def, opts...)
}

// Get retrieves a DynamicEndpoint instance by its ID from the default pool.
//
// Get retrieves the DynamicEndpoint instance from the default pool by ID.
//
// Parameters:
// Parameters:
//   - id: Unique identifier for the endpoint
//
// Returns:
// Returns:
//   - endpoint.DynamicEndpoint: Retrieved endpoint
//   - bool: True if endpoint exists, false otherwise
func Get(id string) (endpoint.DynamicEndpoint, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a DynamicEndpoint instance by its ID from the default pool.
//
// Del deletes the DynamicEndpoint instance from the default pool by ID.
//
// Parameters:
// Parameters:
//   - id: Unique identifier for the endpoint
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all DynamicEndpoint instances in the default pool.
//
// Stop releases all DynamicEndpoint instances in the default pool.
func Stop() {
	DefaultPool.Stop()
}

// Range iterates over all DynamicEndpoint instances in the default pool.
//
// Range traverses all DynamicEndpoint instances in the default pool.
//
// Parameters:
// Parameters:
//   - f: Function to apply to each key-value pair
func Range(f func(key, value any) bool) {
	DefaultPool.Range(f)
}

// Reload reloads all DynamicEndpoint instances in the default pool with the provided options.
//
// Reload uses the provided option to reload all DynamicEndpoint instances in the default pool.
//
// Parameters:
// Parameters:
//   - opts: Optional configuration functions
func Reload(opts ...endpoint.DynamicEndpointOption) {
	DefaultPool.Reload(opts...)
}
