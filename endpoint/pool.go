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
// DefaultPool 是 Pool 的默认实例。
// 它提供一个全局池来管理整个应用程序中的端点实例。
var DefaultPool = &Pool{factory: DefaultFactory}

// DefaultFactory is the default factory instance for creating endpoints.
// DefaultFactory 是用于创建端点的默认工厂实例。
var DefaultFactory = &Factory{registry: Registry}

// Ensure that Pool implements the endpoint.Pool interface.
var _ endpoint.Pool = (*Pool)(nil)

// Factory is a factory that creates Endpoints.
// It provides a centralized way to create different types of endpoints
// using consistent configuration patterns.
//
// Factory 是创建端点的工厂。
// 它提供了使用一致配置模式创建不同类型端点的集中方式。
//
// Key Features:
// 主要特性：
//   - DSL-based endpoint creation  基于 DSL 的端点创建
//   - Type-based endpoint instantiation  基于类型的端点实例化
//   - Configuration management  配置管理
//   - Component registry integration  组件注册表集成
type Factory struct {
	// registry provides access to registered endpoint components
	// registry 提供对已注册端点组件的访问
	registry *ComponentRegistry
}

// NewFromDsl creates a new DynamicEndpoint instance from DSL.
// This method parses JSON DSL and creates a configured dynamic endpoint.
//
// NewFromDsl 从 DSL 创建新的 DynamicEndpoint 实例。
// 此方法解析 JSON DSL 并创建配置的动态端点。
//
// Parameters:
// 参数：
//   - dsl: JSON DSL configuration bytes  JSON DSL 配置字节
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - endpoint.DynamicEndpoint: Created dynamic endpoint  创建的动态端点
//   - error: Creation error if any  如果有的话，创建错误
func (f *Factory) NewFromDsl(dsl []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return NewFromDsl(dsl, opts...)
}

// NewFromDef creates a new DynamicEndpoint instance from DSL definition structure.
// NewFromDef 从 DSL 定义结构创建新的 DynamicEndpoint 实例。
func (f *Factory) NewFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return NewFromDef(def, opts...)
}

// NewFromType creates a new Endpoint instance from type.
// This method provides type-based endpoint creation for programmatic use.
//
// NewFromType 从类型创建新的端点实例。
// 此方法为编程使用提供基于类型的端点创建。
//
// Parameters:
// 参数：
//   - componentType: Type of endpoint to create  要创建的端点类型
//   - ruleConfig: Rule engine configuration  规则引擎配置
//   - configuration: Endpoint-specific configuration  端点特定配置
//
// Returns:
// 返回：
//   - endpoint.Endpoint: Created endpoint instance  创建的端点实例
//   - error: Creation error if any  如果有的话，创建错误
func (f *Factory) NewFromType(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	return f.registry.New(componentType, ruleConfig, configuration)
}

// Pool is a structure that holds DynamicEndpoints.
// It provides centralized management of multiple endpoint instances with
// concurrent-safe operations and lifecycle management.
//
// Pool 是保存 DynamicEndpoints 的结构。
// 它提供多个端点实例的集中管理，具有并发安全操作和生命周期管理。
type Pool struct {
	// entries is a thread-safe map that stores DynamicEndpoints
	// using endpoint IDs as keys
	// entries 是存储 DynamicEndpoints 的线程安全映射，使用端点 ID 作为键
	entries sync.Map

	// factory provides endpoint creation capabilities
	// factory 提供端点创建功能
	factory *Factory
}

// NewPool creates a new instance of a Pool.
// NewPool 创建 Pool 的新实例。
func NewPool() *Pool {
	return &Pool{
		factory: &Factory{
			registry: Registry,
		},
	}
}

// Factory returns the factory instance used by this pool.
// Factory 返回此池使用的工厂实例。
func (p *Pool) Factory() endpoint.Factory {
	return p.factory
}

// New creates a new DynamicEndpoint instance with the specified ID.
// If the id is empty, it uses the id defined in def.
// This method implements a singleton pattern per ID.
//
// New 使用指定的 ID 创建新的 DynamicEndpoint 实例。
// 如果 id 为空，则使用 def 中定义的 id。
// 此方法为每个 ID 实现单例模式。
//
// Parameters:
// 参数：
//   - id: Unique identifier for the endpoint  端点的唯一标识符
//   - def: JSON DSL definition bytes  JSON DSL 定义字节
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - endpoint.DynamicEndpoint: Created or existing endpoint  创建的或现有的端点
//   - error: Creation error if any  如果有的话，创建错误
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
// Get 通过 ID 检索 DynamicEndpoint 实例。
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
// Del 通过 ID 删除 DynamicEndpoint 实例。
// 此方法在删除端点前调用 Destroy() 执行清理。
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
// Stop 释放所有 DynamicEndpoint 实例。
// 此方法优雅地关闭池中的所有端点。
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
// Range 遍历所有 DynamicEndpoint 实例。
func (p *Pool) Range(f func(key, value any) bool) {
	p.entries.Range(f)
}

// Reload reloads all DynamicEndpoint instances with the provided options.
// This method applies the same options to all endpoints in the pool.
//
// Reload 使用提供的选项重新加载所有 DynamicEndpoint 实例。
// 此方法将相同的选项应用于池中的所有端点。
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
// New 从默认池中创建或检索具有指定 ID 的 DynamicEndpoint 实例。
// 如果 id 为空，则使用 def 中定义的 id。
//
// Parameters:
// 参数：
//   - id: Unique identifier for the endpoint  端点的唯一标识符
//   - def: JSON DSL definition bytes  JSON DSL 定义字节
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - endpoint.DynamicEndpoint: Created or existing endpoint  创建的或现有的端点
//   - error: Creation error if any  如果有的话，创建错误
func New(id string, def []byte, opts ...endpoint.DynamicEndpointOption) (endpoint.DynamicEndpoint, error) {
	return DefaultPool.New(id, def, opts...)
}

// Get retrieves a DynamicEndpoint instance by its ID from the default pool.
//
// Get 从默认池中通过 ID 检索 DynamicEndpoint 实例。
//
// Parameters:
// 参数：
//   - id: Unique identifier for the endpoint  端点的唯一标识符
//
// Returns:
// 返回：
//   - endpoint.DynamicEndpoint: Retrieved endpoint  检索到的端点
//   - bool: True if endpoint exists, false otherwise  如果端点存在则为 true，否则为 false
func Get(id string) (endpoint.DynamicEndpoint, bool) {
	return DefaultPool.Get(id)
}

// Del deletes a DynamicEndpoint instance by its ID from the default pool.
//
// Del 从默认池中通过 ID 删除 DynamicEndpoint 实例。
//
// Parameters:
// 参数：
//   - id: Unique identifier for the endpoint  端点的唯一标识符
func Del(id string) {
	DefaultPool.Del(id)
}

// Stop releases all DynamicEndpoint instances in the default pool.
//
// Stop 释放默认池中的所有 DynamicEndpoint 实例。
func Stop() {
	DefaultPool.Stop()
}

// Range iterates over all DynamicEndpoint instances in the default pool.
//
// Range 遍历默认池中的所有 DynamicEndpoint 实例。
//
// Parameters:
// 参数：
//   - f: Function to apply to each key-value pair  要应用于每个键值对的函数
func Range(f func(key, value any) bool) {
	DefaultPool.Range(f)
}

// Reload reloads all DynamicEndpoint instances in the default pool with the provided options.
//
// Reload 使用提供的选项重新加载默认池中的所有 DynamicEndpoint 实例。
//
// Parameters:
// 参数：
//   - opts: Optional configuration functions  可选的配置函数
func Reload(opts ...endpoint.DynamicEndpointOption) {
	DefaultPool.Reload(opts...)
}
