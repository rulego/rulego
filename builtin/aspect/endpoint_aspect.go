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

package aspect

import (
	"reflect"
	"sync"

	"github.com/gofrs/uuid/v5"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/utils/dsl"
	"github.com/rulego/rulego/utils/str"
)

var (
	_ types.OnCreatedAspect = (*EndpointAspect)(nil)
	_ types.OnReloadAspect  = (*EndpointAspect)(nil)
	_ types.OnDestroyAspect = (*EndpointAspect)(nil)
)

// EndpointAspect manages the lifecycle of rule chain endpoints, providing
// automatic endpoint creation, configuration, and cleanup. It bridges the
// gap between rule chains and endpoint management.
//
// EndpointAspect 管理规则链端点的生命周期，提供自动端点创建、配置和清理。
// 它在规则链和端点管理之间架起桥梁。
//
// Features:
// 功能特性：
//   - Automatic endpoint lifecycle management  自动端点生命周期管理
//   - Dynamic endpoint creation and destruction  动态端点创建和销毁
//   - Hot reloading of endpoint configurations  端点配置的热重载
//   - Integration with rule engine pools  与规则引擎池的集成
//   - Support for multiple endpoint types  支持多种端点类型
//
// Lifecycle Events:
// 生命周期事件：
//   - OnCreated: Creates endpoints when rule chain is created
//     OnCreated：规则链创建时创建端点
//   - OnReload: Updates endpoints when rule chain is reloaded
//     OnReload：规则链重新加载时更新端点
//   - OnDestroy: Cleans up endpoints when rule chain is destroyed
//     OnDestroy：规则链销毁时清理端点
//
// Usage:
// 使用方法：
//
//	// Create endpoint aspect with pool
//	// 使用池创建端点切面
//	endpointPool := endpoint.NewPool()
//	aspect := &EndpointAspect{EndpointPool: endpointPool}
//
//	// Apply to rule engine
//	// 应用到规则引擎
//	config := types.NewConfig().WithAspects(aspect)
//	engine := rulego.NewRuleEngine(config)
type EndpointAspect struct {
	EndpointPool      endpoint.Pool      // Pool for managing endpoint instances  管理端点实例的池
	ruleChainEndpoint *RuleChainEndpoint // Associated rule chain endpoint manager  关联的规则链端点管理器
}

// Order returns the execution order of this aspect. Higher values execute later.
// EndpointAspect has order 900, executing late to ensure other aspects are set up first.
//
// Order 返回此切面的执行顺序。值越高，执行越晚。
// EndpointAspect 的顺序为 900，执行较晚以确保其他切面首先设置。
func (aspect *EndpointAspect) Order() int {
	return 900
}

// New creates a new instance of the endpoint aspect for each rule engine.
// Each instance shares the same endpoint pool but maintains separate state.
//
// New 为每个规则引擎创建端点切面的新实例。
// 每个实例共享相同的端点池但维护独立的状态。
func (aspect *EndpointAspect) New() types.Aspect {
	return &EndpointAspect{EndpointPool: aspect.EndpointPool}
}

// Type returns the unique identifier for this aspect type.
//
// Type 返回此切面类型的唯一标识符。
func (aspect *EndpointAspect) Type() string {
	return "endpoint"
}

// PointCut determines which nodes this aspect applies to.
// Returns true for all nodes as endpoint management is chain-level.
//
// PointCut 确定此切面应用于哪些节点。
// 对所有节点返回 true，因为端点管理是链级别的。
func (aspect *EndpointAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// OnCreated is called when a rule chain is created. It initializes endpoints
// defined in the rule chain metadata if endpoint functionality is enabled.
//
// OnCreated 在规则链创建时调用。如果启用了端点功能，它会初始化规则链元数据中定义的端点。
//
// Process:
// 处理过程：
//  1. Check if context is a chain context  检查上下文是否为链上下文
//  2. Verify endpoint functionality is enabled  验证端点功能是否启用
//  3. Create rule chain endpoint manager  创建规则链端点管理器
//  4. Initialize all defined endpoints  初始化所有定义的端点
//
// Parameters:
// 参数：
//   - ctx: Node context containing rule chain information
//     ctx：包含规则链信息的节点上下文
//
// Returns:
// 返回：
//   - error: Endpoint creation error if any, nil on success
//     error：端点创建错误（如果有），成功时为 nil
func (aspect *EndpointAspect) OnCreated(ctx types.NodeCtx) error {
	if chainCtx, ok := ctx.(types.ChainCtx); ok {
		if !chainCtx.Config().EndpointEnabled {
			return nil
		}
		if ruleChainEndpoint, err := NewRuleChainEndpoint(ctx.GetNodeId().Id, chainCtx.Config(),
			aspect.EndpointPool, chainCtx.GetRuleEnginePool(),
			chainCtx.Definition(), chainCtx.Definition().Metadata.Endpoints); err != nil {
			return err
		} else {
			aspect.ruleChainEndpoint = ruleChainEndpoint
		}
	}
	return nil
}

// OnReload is called when a rule chain is reloaded. It updates the endpoint
// configuration and manages endpoint lifecycle changes (add/remove/modify).
//
// OnReload 在规则链重新加载时调用。它更新端点配置并管理端点生命周期变化（添加/删除/修改）。
//
// Process:
// 处理过程：
//  1. Check if endpoints are still enabled  检查端点是否仍然启用
//  2. Update configuration and pool references  更新配置和池引用
//  3. Compare old and new endpoint definitions  比较旧的和新的端点定义
//  4. Apply endpoint changes (add/remove/modify)  应用端点变化（添加/删除/修改）
//
// Parameters:
// 参数：
//   - _: Previous node context (unused)  之前的节点上下文（未使用）
//   - ctx: New node context with updated configuration
//     ctx：具有更新配置的新节点上下文
//
// Returns:
// 返回：
//   - error: Reload error if any, nil on success
//     error：重新加载错误（如果有），成功时为 nil
func (aspect *EndpointAspect) OnReload(_ types.NodeCtx, ctx types.NodeCtx) error {
	if chainCtx, ok := ctx.(types.ChainCtx); ok && aspect.ruleChainEndpoint != nil {
		if !ctx.Config().EndpointEnabled {
			aspect.ruleChainEndpoint.Destroy()
			return nil
		}
		aspect.ruleChainEndpoint.config = ctx.Config()
		aspect.ruleChainEndpoint.ruleGoPool = chainCtx.GetRuleEnginePool()
		return aspect.ruleChainEndpoint.Reload(chainCtx.Definition(), chainCtx.Definition().Metadata.Endpoints)
	}
	return nil
}

// OnDestroy is called when a rule chain is destroyed. It performs cleanup
// of all associated endpoints to prevent resource leaks.
//
// OnDestroy 在规则链销毁时调用。它执行所有关联端点的清理以防止资源泄漏。
func (aspect *EndpointAspect) OnDestroy(ctx types.NodeCtx) {
	if aspect.ruleChainEndpoint != nil {
		aspect.ruleChainEndpoint.Destroy()
	}
}

type RuleChainEndpoint struct {
	ruleEngineId string
	endpointPool endpoint.Pool
	ruleGoPool   types.RuleEnginePool
	endpoints    map[string]endpoint.DynamicEndpoint
	config       types.Config
	sync.RWMutex
}

func NewRuleChainEndpoint(ruleEngineId string, config types.Config, endpointPool endpoint.Pool, ruleGoPool types.RuleEnginePool, ruleChain *types.RuleChain, defs []*types.EndpointDsl) (*RuleChainEndpoint, error) {
	ruleChainEndpoint := &RuleChainEndpoint{
		ruleEngineId: ruleEngineId,
		endpointPool: endpointPool,
		ruleGoPool:   ruleGoPool,
		config:       config,
		endpoints:    make(map[string]endpoint.DynamicEndpoint),
	}
	for _, item := range defs {
		if ruleChain != nil {
			processEndpointDsl(ruleChainEndpoint.config, ruleChain, item)
		}
		ruleChainEndpoint.bindTo(item, ruleEngineId)
		if err := ruleChainEndpoint.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithConfig(config),
			endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(ruleGoPool)),
			endpoint.DynamicEndpointOptions.WithRuleChain(ruleChain)); err != nil {
			return nil, err
		}
	}
	return ruleChainEndpoint, nil
}

// Start 启动服务
func (e *RuleChainEndpoint) Start() error {
	endpoints := e.GetEndpoints()
	for _, ep := range endpoints {
		if err := ep.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (e *RuleChainEndpoint) Reload(ruleChain *types.RuleChain, newDefs []*types.EndpointDsl) error {
	var oldDefs []*types.EndpointDsl
	endpoints := e.GetEndpoints()
	for _, ep := range endpoints {
		tmp := ep.Definition()
		if ruleChain != nil {
			processEndpointDsl(e.config, ruleChain, &tmp)
		}
		oldDefs = append(oldDefs, &tmp)
	}
	// process newDefs variables
	if ruleChain != nil {
		for _, item := range newDefs {
			processEndpointDsl(e.config, ruleChain, item)
		}
	}
	added, removed, modified := e.checkEndpointChanges(oldDefs, newDefs)
	for _, item := range removed {
		e.RemoveEndpoint(item.Id)
	}
	for _, item := range added {
		e.bindTo(item, e.ruleEngineId)
		if err := e.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithConfig(e.config),
			endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(e.ruleGoPool)),
			endpoint.DynamicEndpointOptions.WithRuleChain(ruleChain),
		); err != nil {
			return err
		}
	}
	for _, item := range modified {
		e.bindTo(item, e.ruleEngineId)
		e.RemoveEndpoint(item.Id)
		if err := e.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithConfig(e.config),
			endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(e.ruleGoPool)),
			endpoint.DynamicEndpointOptions.WithRuleChain(ruleChain),
		); err != nil {
			return err
		}
	}
	return nil
}

func (e *RuleChainEndpoint) AddEndpointAndStart(def *types.EndpointDsl, opts ...endpoint.DynamicEndpointOption) error {
	ep, err := e.endpointPool.Factory().NewFromDef(*def, opts...)
	if err != nil {
		return err
	}
	if ep.Id() == "" {
		uid, _ := uuid.NewV4()
		id := uid.String()
		ep.SetId(id)
		def.Id = id
	}
	e.AddEndpoint(ep)
	return ep.Start()
}

func (e *RuleChainEndpoint) AddEndpoint(ep endpoint.DynamicEndpoint) {
	e.Lock()
	defer e.Unlock()
	e.endpoints[ep.Id()] = ep
}

func (e *RuleChainEndpoint) GetEndpoint(id string) (endpoint.DynamicEndpoint, bool) {
	e.RLock()
	defer e.RUnlock()
	ep, ok := e.endpoints[id]
	return ep, ok
}

func (e *RuleChainEndpoint) GetEndpoints() []endpoint.DynamicEndpoint {
	e.RLock()
	defer e.RUnlock()
	var endpoints []endpoint.DynamicEndpoint
	for _, ep := range e.endpoints {
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

func (e *RuleChainEndpoint) RemoveEndpoint(id string) {
	e.Lock()
	defer e.Unlock()
	if ep, ok := e.endpoints[id]; ok {
		ep.Destroy()
		delete(e.endpoints, id)
	}
}

func (e *RuleChainEndpoint) Destroy() {
	e.RLock()
	defer e.RUnlock()
	for _, ep := range e.endpoints {
		ep.Destroy()
	}
	e.endpoints = make(map[string]endpoint.DynamicEndpoint)
}

// Helper function to determine if two EndpointDsl instances are equal.
func (e *RuleChainEndpoint) isEndpointModified(old, new *types.EndpointDsl) bool {
	// Use reflect.DeepEqual to compare two EndpointDsl instances.
	// This will check all fields for equality.
	return !reflect.DeepEqual(old, new)
}

// checkEndpointChanges compares two slices of EndpointDsl and returns slices of added, removed, and modified EndpointDsl instances.
func (e *RuleChainEndpoint) checkEndpointChanges(oldEndpoints, newEndpoints []*types.EndpointDsl) (added, removed, modified []*types.EndpointDsl) {
	oldMap := make(map[string]*types.EndpointDsl) // Map to store old endpoints for quick lookup.
	newMap := make(map[string]*types.EndpointDsl) // Map to store new endpoints for quick lookup.

	// Populate the oldMap.
	for _, ep := range oldEndpoints {
		oldMap[ep.Id] = ep
	}

	// Check for removed and modified endpoints.
	for _, ep := range newEndpoints {
		newMap[ep.Id] = ep
		if oldEp, exists := oldMap[ep.Id]; exists {
			if e.isEndpointModified(oldEp, ep) {
				modified = append(modified, ep)
			}
			delete(oldMap, ep.Id) // Remove from oldMap since it's not removed.
		} else {
			added = append(added, ep) // It's a new ruleChainEndpoint.
		}
	}

	// Anything left in oldMap is removed.
	for _, ep := range oldMap {
		removed = append(removed, ep)
	}

	return added, removed, modified
}

// 绑定To,To必须是当前规则链ID
func (e *RuleChainEndpoint) bindTo(def *types.EndpointDsl, ruleEngineId string) {
	for _, r := range def.Routers {
		if r.To.Path == "" {
			r.To.Path = ruleEngineId
		}
	}
}

func processEndpointDsl(config types.Config, ruleChain *types.RuleChain, item *types.EndpointDsl) {
	if ruleChain == nil {
		return
	}
	env := dsl.GetInitNodeEnv(config, *ruleChain)

	// Configuration
	item.Configuration = processConfiguration(env, item.Configuration)

	// Processors
	item.Processors = processSlice(env, item.Processors)

	// Routers
	for _, router := range item.Routers {
		// Params
		router.Params = processInterfaceSlice(env, router.Params)

		// From
		router.From.Path = str.ExecuteTemplate(router.From.Path, env)
		router.From.Configuration = processConfiguration(env, router.From.Configuration)
		router.From.Processors = processSlice(env, router.From.Processors)

		// To
		router.To.Path = str.ExecuteTemplate(router.To.Path, env)
		router.To.Configuration = processConfiguration(env, router.To.Configuration)
		router.To.Processors = processSlice(env, router.To.Processors)
	}
}

func processConfiguration(env map[string]interface{}, config types.Configuration) types.Configuration {
	newConfig := make(types.Configuration)
	for k, v := range config {
		if strV, ok := v.(string); ok {
			newConfig[k] = str.ExecuteTemplate(strV, env)
		} else {
			newConfig[k] = v
		}
	}
	return newConfig
}

func processSlice(env map[string]interface{}, slice []string) []string {
	var newSlice []string
	for _, s := range slice {
		newSlice = append(newSlice, str.ExecuteTemplate(s, env))
	}
	return newSlice
}

func processInterfaceSlice(env map[string]interface{}, slice []interface{}) []interface{} {
	var newSlice []interface{}
	for _, v := range slice {
		if s, ok := v.(string); ok {
			newSlice = append(newSlice, str.ExecuteTemplate(s, env))
		} else {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}
