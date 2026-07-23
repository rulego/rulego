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
// EndpointAspect manages the lifecycle of rule chain endpoints, providing automatic endpoint creation, configuration, and cleanup.
// It builds a bridge between the rule chain and endpoint management.
//
// Features:
// Features:
//   - Automatic endpoint lifecycle management
//   - Dynamic endpoint creation and destruction
//   - Hot reloading of endpoint configurations
//   - Integration with rule engine pools
//   - Support for multiple endpoint types
//
// Lifecycle Events:
// Lifecycle Events:
//   - OnCreated: Creates endpoints when rule chain is created
//     OnCreated: Creates endpoints when the rule chain is created
//   - OnReload: Updates endpoints when rule chain is reloaded
//     OnReload: Updates endpoints when the rule chain reloads
//   - OnDestroy: Cleans up endpoints when rule chain is destroyed
//     OnDestroy: Cleans endpoints when the rule chain is destroyed
//
// Usage:
// How to use:
//
//	// Create endpoint aspect with pool
//	Use the pool to create endpoint aspects
//	endpointPool := endpoint.NewPool()
//	aspect := &EndpointAspect{EndpointPool: endpointPool}
//
//	// Apply to rule engine
//	Applied to the rule engine
//	config := types.NewConfig().WithAspects(aspect)
//	engine := rulego.NewRuleEngine(config)
type EndpointAspect struct {
	EndpointPool      endpoint.Pool      // Pool for managing endpoint instances
	ruleChainEndpoint *RuleChainEndpoint // Associated rule chain endpoint manager
}

// Order returns the execution order of this aspect. Higher values execute later.
// EndpointAspect has order 900, executing late to ensure other aspects are set up first.
//
// Order returns the execution order of this aspect. The higher the value, the later it is executed.
// EndpointAspect has order 900, so it runs late to ensure other aspects are configured first.
func (aspect *EndpointAspect) Order() int {
	return 900
}

// New creates a new instance of the endpoint aspect for each rule engine.
// Each instance shares the same endpoint pool but maintains separate state.
//
// New creates a new endpoint aspect instance for each rule engine.
// Each instance shares the same endpoint pool but maintains independent states.
func (aspect *EndpointAspect) New() types.Aspect {
	return &EndpointAspect{EndpointPool: aspect.EndpointPool}
}

// Type returns the unique identifier for this aspect type.
//
// Type returns a unique identifier for this facet type.
func (aspect *EndpointAspect) Type() string {
	return "endpoint"
}

// PointCut determines which nodes this aspect applies to.
// Returns true for all nodes as endpoint management is chain-level.
//
// PointCut determines which nodes this section is applied to.
// Returns true for all nodes, because endpoint management is chain-level.
func (aspect *EndpointAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

// OnCreated is called when a rule chain is created. It initializes endpoints
// defined in the rule chain metadata if endpoint functionality is enabled.
//
// OnCreated is called when the rule chain is created. If endpoint functionality is enabled, it initializes the endpoints defined in the rule chain metadata.
//
// Process:
// Handling process:
//  1. Check if context is a chain context
//  2. Verify endpoint functionality is enabled
//  3. Create rule chain endpoint manager
//  4. Initialize all defined endpoints
//
// Parameters:
// Parameters:
//   - ctx: Node context containing rule chain information
//     ctx: The node context containing the rule chain information
//
// Returns:
// Returns:
//   - error: Endpoint creation error if any, nil on success
//     error: Endpoint creation error (if any), nil on success
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
			// Register same-chain endpoints for ref:// same-chain addressing (register underlying instances and stably implement TargetSender)
			aspect.syncResources(chainCtx, nil, ruleChainEndpoint.GetEndpoints())
		}
	}
	return nil
}

// syncResources: Delete oldEps and register the underlying instance of newEps.
// Used for OnCreated (oldEps = nil, full registration) and OnReload (deregistering old first, then registering new).
// Register the underlying endpoint.Endpoint (rather than DynamicEndpoint wrappers), which provides a stable implementation of TargetSender.
func (aspect *EndpointAspect) syncResources(chainCtx types.ChainCtx, oldEps, newEps []endpoint.DynamicEndpoint) {
	reg := chainCtx.EndpointRegistry()
	// First, register new ones (Store override) to ensure that resources in the window period always have the latest values, avoiding missed concurrent lookups.
	newIds := make(map[string]bool, len(newEps))
	for _, ep := range newEps {
		if ep == nil {
			continue
		}
		if inner := ep.Target(); inner != nil {
			reg.Register(ep.Id(), inner)
			newIds[ep.Id()] = true
		} else {
			// target()==nil: The underlying endpoint is not initialized, logging is easy to check (otherwise it will be silent and not registered, ref:// only reports 'not found')
			if l := chainCtx.Config().Logger; l != nil {
				l.Printf("endpoint %s Target() is nil, skip register to chain resources", ep.Id())
			}
		}
	}
	// Then Unregister only removes the one (not in newEps), and clears the gap in the middle of 'delete all first, then add'.
	for _, ep := range oldEps {
		if ep != nil && !newIds[ep.Id()] {
			reg.Unregister(ep.Id())
		}
	}
}

// OnReload is called when a rule chain is reloaded. It updates the endpoint
// configuration and manages endpoint lifecycle changes (add/remove/modify).
//
// OnReload is called when the rule chain reloads. It updates endpoint configurations and manages endpoint lifecycle changes (add/delete/modify).
//
// Process:
// Handling process:
//  1. Check if endpoints are still enabled
//  2. Update configuration and pool references
//  3. Compare old and new endpoint definitions
//  4. Apply endpoint changes (add/remove/modify)
//
// Parameters:
// Parameters:
//   - _: Previous node context (unused)
//   - ctx: New node context with updated configuration
//     ctx: New node context with updated configuration
//
// Returns:
// Returns:
//   - error: Reload error if any, nil on success
//     error: Reloading error (if any), nil on success
func (aspect *EndpointAspect) OnReload(_ types.NodeCtx, ctx types.NodeCtx) error {
	if chainCtx, ok := ctx.(types.ChainCtx); ok && aspect.ruleChainEndpoint != nil {
		if !ctx.Config().EndpointEnabled {
			aspect.syncResources(chainCtx, aspect.ruleChainEndpoint.GetEndpoints(), nil)
			aspect.ruleChainEndpoint.Destroy()
			return nil
		}
		aspect.ruleChainEndpoint.config = ctx.Config()
		aspect.ruleChainEndpoint.ruleGoPool = chainCtx.GetRuleEnginePool()
		// After reloading, the resource directory is synchronized according to the last survival state (including cases where the reload partially succeeds).
		err := aspect.ruleChainEndpoint.Reload(chainCtx.Definition(), chainCtx.Definition().Metadata.Endpoints)
		aspect.syncResources(chainCtx, nil, aspect.ruleChainEndpoint.GetEndpoints())
		return err
	}
	return nil
}

// OnDestroy is called when a rule chain is destroyed. It performs cleanup
// of all associated endpoints to prevent resource leaks.
//
// OnDestroy is called when the rule chain is destroyed. It cleans all associated endpoints to prevent resource leaks.
func (aspect *EndpointAspect) OnDestroy(ctx types.NodeCtx) {
	if aspect.ruleChainEndpoint != nil {
		// Destroy takes oldEps before and logs out (Destroy will clear the endpoints map).
		if chainCtx, ok := ctx.(types.ChainCtx); ok {
			aspect.syncResources(chainCtx, aspect.ruleChainEndpoint.GetEndpoints(), nil)
		}
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

// Start the service
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

// Bind To, and To must be the current rule chain ID
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
