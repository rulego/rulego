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
	"github.com/rulego/rulego/api/types/endpoint"
	"sync"
)

// DefaultPool is the default instance of the Pool.
var DefaultPool = &Pool{}

// Ensure that Pool implements the endpoint.Pool interface.
var _ endpoint.Pool = (*Pool)(nil)

// Pool is a structure that holds DynamicEndpoints.
type Pool struct {
	entries sync.Map // entries is a thread-safe map that stores DynamicEndpoints.
}

// NewPool creates a new instance of a Pool.
func NewPool() *Pool {
	return &Pool{}
}

// New creates a new DynamicEndpoint instance with the specified ID.
// If the id is empty, it uses the id defined in def.
func (p *Pool) New(id string, def []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	if v, ok := p.entries.Load(id); ok {
		return v.(endpoint.DynamicEndpoint), nil
	} else {
		if e, err := NewFromDsl(id, def, opts...); err != nil {
			return e, err
		} else {
			p.entries.Store(e.Id(), e)
			return e, nil
		}
	}
}

// Get retrieves a DynamicEndpoint instance by its ID.
func (p *Pool) Get(id string) (endpoint.DynamicEndpoint, bool) {
	v, ok := p.entries.Load(id)
	if ok {
		return v.(endpoint.DynamicEndpoint), ok
	} else {
		return nil, false
	}
}

// Del deletes a DynamicEndpoint instance by its ID.
func (p *Pool) Del(id string) {
	v, ok := p.entries.Load(id)
	if ok {
		v.(endpoint.DynamicEndpoint).Destroy()
		p.entries.Delete(id)
	}
}

// Stop releases all DynamicEndpoint instances.
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
func (p *Pool) Range(f func(key, value any) bool) {
	p.entries.Range(f)
}

// Reload reloads all DynamicEndpoint instances with the provided options.
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
func New(id string, def []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return DefaultPool.New(id, def, opts...)
}

// Get retrieves a DynamicEndpoint instance by its ID from the default pool.
func Get(id string) (endpoint.DynamicEndpoint, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a DynamicEndpoint instance by its ID from the default pool.
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all DynamicEndpoint instances in the default pool.
func Stop() {
	DefaultPool.Stop()
}

// Range iterates over all DynamicEndpoint instances in the default pool.
func Range(f func(key, value any) bool) {
	DefaultPool.Range(f)
}

// Reload reloads all DynamicEndpoint instances in the default pool with the provided options.
func Reload(opts ...endpoint.DynamicEndpointOption) {
	DefaultPool.Reload(opts...)
}
