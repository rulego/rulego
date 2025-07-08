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
	"errors"
	"reflect"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/json"
)

// Endpoint is an alias for the Endpoint interface in the endpoint package.
// Endpoint 是端点包中 Endpoint 接口的别名。
type Endpoint = endpoint.Endpoint

// Exchange is deprecated. Use Flow from github.com/rulego/rulego/api/types/endpoint.Exchange instead.
// Exchange 已弃用。请使用 github.com/rulego/rulego/api/types/endpoint.Exchange 中的 Flow。
type Exchange = endpoint.Exchange

// NewRouter creates a new router with the provided options.
// This function provides a convenient way to create routers for endpoint configuration.
//
// NewRouter 使用提供的选项创建新的路由器。
// 此函数为端点配置提供了创建路由器的便捷方式。
//
// Parameters:
// 参数：
//   - opts: Router configuration options  路由器配置选项
//
// Returns:
// 返回：
//   - endpoint.Router: Configured router instance  配置的路由器实例
func NewRouter(opts ...endpoint.RouterOption) endpoint.Router {
	return impl.NewRouter(opts...)
}

// Ensure DynamicEndpoint implements the DynamicEndpoint interface.
var _ endpoint.DynamicEndpoint = (*DynamicEndpoint)(nil)

// DynamicEndpoint represents a dynamic endpoint with additional properties and methods.
// It provides hot-reloading capabilities and dynamic configuration management for endpoints.
//
// DynamicEndpoint 表示具有附加属性和方法的动态端点。
// 它为端点提供热重载功能和动态配置管理。
//
// Key Features:
// 主要特性：
//   - Dynamic DSL-based configuration  基于 DSL 的动态配置
//   - Hot reloading without service interruption  无服务中断的热重载
//   - Router management with add/remove/update operations  具有添加/删除/更新操作的路由器管理
//   - Interceptor support for processing pipelines  支持处理管道的拦截器
//   - Rule chain integration  规则链集成
//   - Thread-safe operations  线程安全操作
//
// Lifecycle:
// 生命周期：
//  1. Creation from DSL configuration  从 DSL 配置创建
//  2. Router and interceptor setup  路由器和拦截器设置
//  3. Service startup  服务启动
//  4. Dynamic updates and reloads  动态更新和重载
//  5. Graceful shutdown and cleanup  优雅关闭和清理
//
// Configuration Management:
// 配置管理：
//   - Supports JSON DSL for declarative configuration  支持用于声明式配置的 JSON DSL
//   - Enables runtime configuration changes  支持运行时配置更改
//   - Validates configuration before applying changes  在应用更改前验证配置
//   - Maintains configuration history for rollback  维护配置历史以供回滚
type DynamicEndpoint struct {
	// Endpoint is the embedded endpoint implementation providing core functionality
	// Endpoint 是嵌入的端点实现，提供核心功能
	Endpoint

	// id is the unique identifier for this endpoint instance
	// id 是此端点实例的唯一标识符
	id string

	// ruleChain contains the rule chain DSL definition when initialized from rule chain
	// ruleChain 包含从规则链初始化时的规则链 DSL 定义
	ruleChain *types.RuleChain

	// definition contains the endpoint DSL configuration
	// definition 包含端点 DSL 配置
	definition types.EndpointDsl

	// ruleConfig contains the rule engine configuration
	// ruleConfig 包含规则引擎配置
	ruleConfig types.Config

	// interceptors are the processing interceptors for the endpoint
	// interceptors 是端点的处理拦截器
	interceptors []endpoint.Process

	// routerOpts are the router configuration options for the endpoint
	// routerOpts 是端点的路由器配置选项
	routerOpts []endpoint.RouterOption

	// restart indicates whether the endpoint should be restarted during reload
	// restart 指示在重载期间是否应重启端点
	restart bool

	// locker provides thread-safe access to endpoint state
	// locker 为端点状态提供线程安全访问
	locker sync.RWMutex
}

// NewFromDsl creates a new DynamicEndpoint from the provided DSL definition and options.
// This function parses JSON DSL configuration and creates a fully configured dynamic endpoint.
//
// NewFromDsl 从提供的 DSL 定义和选项创建新的 DynamicEndpoint。
// 此函数解析 JSON DSL 配置并创建完全配置的动态端点。
//
// Parameters:
// 参数：
//   - def: JSON DSL definition bytes  JSON DSL 定义字节
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - *DynamicEndpoint: Configured dynamic endpoint  配置的动态端点
//   - error: Creation error if any  如果有的话，创建错误
//
// Example DSL:
// DSL 示例：
//
//	{
//	  "id": "http-endpoint",
//	  "type": "http",
//	  "configuration": {"server": ":8080"},
//	  "routers": [{"id": "r1", "from": {"path": "/api"}}]
//	}
func NewFromDsl(def []byte, opts ...endpoint.DynamicEndpointOption) (*DynamicEndpoint, error) {
	if len(def) == 0 {
		return nil, errors.New("def cannot be nil")
	}
	e := &DynamicEndpoint{}
	if err := e.Reload(def, opts...); err != nil {
		return nil, err
	}
	if e.id == "" && e.definition.Id != "" {
		e.id = e.definition.Id
	}
	return e, nil
}

// NewFromDef creates a new DynamicEndpoint from the provided DSL definition structure and options.
// NewFromDef 从提供的 DSL 定义结构和选项创建新的 DynamicEndpoint。
func NewFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) (*DynamicEndpoint, error) {
	e := &DynamicEndpoint{}
	if err := e.ReloadFromDef(def, opts...); err != nil {
		return nil, err
	}
	if e.id == "" && e.definition.Id != "" {
		e.id = e.definition.Id
	}
	return e, nil
}

// Id returns the identifier of the DynamicEndpoint.
// Id 返回 DynamicEndpoint 的标识符。
func (e *DynamicEndpoint) Id() string {
	return e.id
}

// SetId sets the identifier of the DynamicEndpoint.
// SetId 设置 DynamicEndpoint 的标识符。
func (e *DynamicEndpoint) SetId(id string) {
	e.id = id
}

// SetConfig sets the configuration for the DynamicEndpoint.
// SetConfig 设置 DynamicEndpoint 的配置。
func (e *DynamicEndpoint) SetConfig(config types.Config) {
	e.ruleConfig = config
}

// SetRouterOptions sets the router options for the DynamicEndpoint.
// SetRouterOptions 设置 DynamicEndpoint 的路由器选项。
func (e *DynamicEndpoint) SetRouterOptions(opts ...endpoint.RouterOption) {
	e.routerOpts = opts
}

// SetRestart sets the restart flag for the DynamicEndpoint.
// SetRestart 设置 DynamicEndpoint 的重启标志。
func (e *DynamicEndpoint) SetRestart(restart bool) {
	e.restart = restart
}

// SetInterceptors sets the interceptors for the DynamicEndpoint.
// SetInterceptors 设置 DynamicEndpoint 的拦截器。
func (e *DynamicEndpoint) SetInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = interceptors
}

// AddInterceptors adds interceptors to the DynamicEndpoint.
// AddInterceptors 向 DynamicEndpoint 添加拦截器。
func (e *DynamicEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = append(e.interceptors, interceptors...)
	e.Endpoint.AddInterceptors(interceptors...)
}

// Reload reloads the DynamicEndpoint with the provided definition and options.
// This method supports hot reloading of endpoint configuration without service interruption.
//
// Reload 使用提供的定义和选项重新加载 DynamicEndpoint。
// 此方法支持在不中断服务的情况下热重载端点配置。
//
// Parameters:
// 参数：
//   - dsl: New JSON DSL configuration  新的 JSON DSL 配置
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - error: Reload error if any  如果有的话，重载错误
//
// Hot Reload Process:
// 热重载过程：
//  1. Parse new DSL configuration  解析新的 DSL 配置
//  2. Compare with current configuration  与当前配置比较
//  3. Determine if restart is needed  确定是否需要重启
//  4. Apply configuration changes  应用配置更改
//  5. Update routers as needed  根据需要更新路由器
func (e *DynamicEndpoint) Reload(dsl []byte, opts ...endpoint.DynamicEndpointOption) error {
	if dsl, err := e.unmarshal(dsl); err != nil {
		return err
	} else {
		return e.ReloadFromDef(dsl, opts...)
	}
}

// AddOrReloadRouter reloads the router for the DynamicEndpoint with the provided definition and options.
// This method allows dynamic addition or modification of individual routers without affecting the entire endpoint.
//
// AddOrReloadRouter 使用提供的定义和选项为 DynamicEndpoint 重新加载路由器。
// 此方法允许动态添加或修改单个路由器，而不影响整个端点。
//
// Parameters:
// 参数：
//   - dsl: Router JSON DSL configuration  路由器 JSON DSL 配置
//   - opts: Optional configuration functions  可选的配置函数
//
// Returns:
// 返回：
//   - error: Operation error if any  如果有的话，操作错误
//
// Router Management:
// 路由器管理：
//   - Automatically removes existing router with same ID  自动删除具有相同 ID 的现有路由器
//   - Validates router configuration before applying  在应用前验证路由器配置
//   - Supports both addition and modification operations  支持添加和修改操作
//   - Can trigger endpoint restart if configured  如果配置，可以触发端点重启
func (e *DynamicEndpoint) AddOrReloadRouter(dsl []byte, opts ...endpoint.DynamicEndpointOption) error {
	var routerDsl types.RouterDsl
	if err := json.Unmarshal(dsl, &routerDsl); err != nil {
		return err
	}
	_, err := e.AddRouterFromDef(&routerDsl)
	e.restart = false
	for _, opt := range opts {
		_ = opt(e)
	}
	if e.restart {
		return e.reloadEndpoint(e.definition)
	}
	return err
}

// Definition returns the DSL definition of the DynamicEndpoint.
func (e *DynamicEndpoint) Definition() types.EndpointDsl {
	return e.definition
}

// DSL returns the DSL as a byte slice.
func (e *DynamicEndpoint) DSL() []byte {
	dsl, _ := json.Marshal(e.definition)
	return dsl
}

// Target returns the underlying Endpoint of the DynamicEndpoint.
func (e *DynamicEndpoint) Target() endpoint.Endpoint {
	return e.Endpoint
}

// RemoveRouter removes a router from the DynamicEndpoint by its ID and parameters.
func (e *DynamicEndpoint) RemoveRouter(routerId string, params ...interface{}) error {
	e.locker.Lock()
	defer e.locker.Unlock()
	if err := e.Endpoint.RemoveRouter(routerId, params...); err == nil {
		var newRouters []*types.RouterDsl
		for _, item := range e.definition.Routers {
			if item.Id != routerId {
				newRouters = append(newRouters, item)
			}
		}
		e.definition.Routers = newRouters
	}
	return nil
}

// AddRouterFromDef adds a router to the DynamicEndpoint from the provided DSL.
func (e *DynamicEndpoint) AddRouterFromDef(routerDsl *types.RouterDsl) (string, error) {
	if routerDsl == nil {
		return "", errors.New("routerDsl cannot be nil")
	}
	_ = e.RemoveRouter(routerDsl.Id, routerDsl.Params...)

	var opts = []endpoint.RouterOption{endpoint.RouterOptions.WithDefinition(routerDsl)}
	opts = append(opts, e.routerOpts...)

	e.locker.Lock()
	defer e.locker.Unlock()
	from := NewRouter(opts...).SetId(routerDsl.Id).From(routerDsl.From.Path, routerDsl.From.Configuration)
	for _, item := range routerDsl.From.Processors {
		if p, ok := processor.InBuiltins.Get(item); ok {
			from.Process(p)
		} else {
			return "", errors.New("processor not found: " + item)
		}
	}
	if routerDsl.To.Path != "" {
		to := from.To(routerDsl.To.Path, routerDsl.To.Configuration)
		for _, item := range routerDsl.To.Processors {
			if p, ok := processor.OutBuiltins.Get(item); ok {
				to.Process(p)
			} else {
				return "", errors.New("processor not found: " + item)
			}
		}
		if routerDsl.To.Wait {
			to.Wait()
		}
	}
	router := from.End()
	if id, err := e.Endpoint.AddRouter(router, routerDsl.Params...); err != nil {
		return "", err
	} else {
		routerDsl.Id = id
		e.definition.Routers = append(e.definition.Routers, routerDsl)
		return id, err
	}
}

// ReloadFromDef initializes the DynamicEndpoint with the provided DSL and options.
func (e *DynamicEndpoint) ReloadFromDef(def types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) error {
	e.restart = false
	e.ruleConfig = engine.NewConfig(types.WithDefaultPool())
	for _, opt := range opts {
		_ = opt(e)
	}
	if e.Endpoint != nil {
		return e.reloadEndpoint(def)
	} else {
		return e.newEndpoint(def)
	}
}

func (e *DynamicEndpoint) Config() types.Config {
	return e.ruleConfig
}

// IsDebugMode checks if the node is in debug mode.
// True: When messages flow in and out of the node, the config.OnDebug callback function is called; otherwise, it is not.
func (e *DynamicEndpoint) IsDebugMode() bool {
	return false
}

// GetNodeId retrieves the component ID.
func (e *DynamicEndpoint) GetNodeId() types.RuleNodeId {
	return types.RuleNodeId{Id: e.Id(), Type: types.ENDPOINT}
}

// ReloadSelf refreshes the configuration of the component.
func (e *DynamicEndpoint) ReloadSelf(def []byte) error {
	return e.Reload(def)
}

// GetNodeById not supported.
func (e *DynamicEndpoint) GetNodeById(_ types.RuleNodeId) (types.NodeCtx, bool) {
	return nil, false
}

// SetRuleChain When initializing from the rule chain DSL, set the DSL definition of the original rule chain
func (e *DynamicEndpoint) SetRuleChain(ruleChain *types.RuleChain) {
	e.ruleChain = ruleChain
}

// GetRuleChain Obtain the original DSL initialized from the rule chain
func (e *DynamicEndpoint) GetRuleChain() *types.RuleChain {
	return e.ruleChain
}

// newEndpoint creates a new Endpoint with the provided DSL.
func (e *DynamicEndpoint) newEndpoint(dsl types.EndpointDsl) error {
	var configuration = make(types.Configuration)
	if dsl.Configuration != nil {
		configuration = dsl.Configuration.Copy()
	}
	//注入完整的规则链定义
	def := e.GetRuleChain()

	if def != nil {
		configuration[types.NodeConfigurationKeyRuleChainDefinition] = def
	}
	if ep, err := Registry.New(dsl.Type, e.ruleConfig, configuration); err != nil {
		return err
	} else {
		e.Endpoint = ep
		e.definition = dsl
		if e.id == "" && e.definition.Id != "" {
			e.id = e.definition.Id
		}
		if e.id == "" {
			e.id = ep.Id()
		}
		e.AddInterceptors(e.interceptors...)
		for _, item := range dsl.Routers {
			if _, err := e.AddRouterFromDef(item); err != nil {
				return err
			}
		}
		// Add interceptors
		for _, item := range dsl.Processors {
			if p, ok := processor.InBuiltins.Get(item); ok {
				e.AddInterceptors(p)
			} else {
				return errors.New("processor not found: " + item)
			}
		}
		if e.restart {
			return ep.Start()
		} else {
			return nil
		}
	}
}

// reloadEndpoint reloads the Endpoint with the provided DSL.
func (e *DynamicEndpoint) reloadEndpoint(def types.EndpointDsl) error {
	if e.Endpoint != nil && (e.restart || needRestart(e.definition, def)) {
		e.Endpoint.Destroy()
		e.Endpoint = nil
		e.restart = true
		return e.newEndpoint(def)
	}
	// Check for changes in routers
	added, removed, modified := checkRouterChanges(e.definition.Routers, def.Routers)
	for _, item := range removed {
		_ = e.RemoveRouter(item.Id, item.Params...)
	}
	for _, item := range added {
		if _, err := e.AddRouterFromDef(item); err != nil {
			return err
		}
	}
	for _, item := range modified {
		if _, err := e.AddRouterFromDef(item); err != nil {
			return err
		}
	}
	e.definition = def
	return nil
}

// unmarshal converts the provided byte slice into an EndpointDsl.
func (e *DynamicEndpoint) unmarshal(def []byte) (types.EndpointDsl, error) {
	var dsl types.EndpointDsl
	if len(def) != 0 {
		if err := json.Unmarshal(def, &dsl); err != nil {
			return types.EndpointDsl{}, err
		}
	} else {
		dsl = e.definition
	}
	return dsl, nil
}

// needRestart determines whether the endpoint needs to be restarted based on the old and new EndpointBaseInfo
func needRestart(old, new types.EndpointDsl) bool {
	if old.Type != new.Type {
		return true
	}
	return !reflect.DeepEqual(old.Configuration, new.Configuration) || !reflect.DeepEqual(old.Processors, new.Processors)
}

// checkRouterChanges checks for added, removed, and modified routers in a list of RouterDsl.
func checkRouterChanges(oldRouters, newRouters []*types.RouterDsl) (added, removed, modified []*types.RouterDsl) {
	// Create a map to hold the old routers with their ID as the key.
	oldMap := make(map[string]*types.RouterDsl)
	// Create a map to hold the new routers with their ID as the key.
	newMap := make(map[string]*types.RouterDsl)

	// Convert the old and new routers into maps using their ID as the key.
	for _, r := range oldRouters {
		oldMap[r.Id] = r
	}
	for _, r := range newRouters {
		newMap[r.Id] = r
	}

	// Check for routers that are new in the newMap but not present in the oldMap.
	for id, r := range newMap {
		if _, exists := oldMap[id]; !exists {
			added = append(added, r) // Add new routers to the added slice.
		}
	}

	// Check for routers that are present in the oldMap but not in the newMap.
	for id, r := range oldMap {
		if _, exists := newMap[id]; !exists {
			removed = append(removed, r)
		}
	}

	// Check for routers that are modified, i.e., present in both maps but not equal.
	for id, newR := range newMap {
		if oldR, exists := oldMap[id]; exists {
			if !reflect.DeepEqual(oldR, newR) {
				modified = append(modified, newR)
			}
		}
	}
	return added, removed, modified
}
