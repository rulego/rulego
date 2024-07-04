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
	"github.com/gofrs/uuid/v5"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"reflect"
	"sync"
)

var (
	_ types.OnCreatedAspect = (*EndpointAspect)(nil)
	_ types.OnReloadAspect  = (*EndpointAspect)(nil)
	_ types.OnDestroyAspect = (*EndpointAspect)(nil)
)

type EndpointAspect struct {
	EndpointPool      endpoint.Pool
	ruleChainEndpoint *RuleChainEndpoint
}

func (aspect *EndpointAspect) Order() int {
	return 900
}

func (aspect *EndpointAspect) New() types.Aspect {
	return &EndpointAspect{EndpointPool: aspect.EndpointPool}
}

func (aspect *EndpointAspect) Type() string {
	return "endpoint"
}

func (aspect *EndpointAspect) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *EndpointAspect) OnCreated(chainCtx types.NodeCtx) error {
	if ctx, ok := chainCtx.(types.ChainCtx); ok {
		if !ctx.Config().EndpointEnabled {
			return nil
		}
		if ruleChainEndpoint, err := NewRuleChainEndpoint(chainCtx.GetNodeId().Id, ctx.Config(), aspect.EndpointPool, ctx.GetRuleEnginePool(), ctx.Definition().Metadata.Endpoints); err != nil {
			return err
		} else {
			aspect.ruleChainEndpoint = ruleChainEndpoint
		}
	}
	return nil
}

func (aspect *EndpointAspect) OnReload(_ types.NodeCtx, ctx types.NodeCtx) error {
	if chainCtx, ok := ctx.(types.ChainCtx); ok && aspect.ruleChainEndpoint != nil {
		if !ctx.Config().EndpointEnabled {
			aspect.ruleChainEndpoint.Destroy()
			return nil
		}
		aspect.ruleChainEndpoint.config = ctx.Config()
		aspect.ruleChainEndpoint.ruleGoPool = chainCtx.GetRuleEnginePool()
		return aspect.ruleChainEndpoint.Reload(chainCtx.Definition().Metadata.Endpoints)
	}
	return nil
}
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

func NewRuleChainEndpoint(ruleEngineId string, config types.Config, endpointPool endpoint.Pool, ruleGoPool types.RuleEnginePool, defs []*types.EndpointDsl) (*RuleChainEndpoint, error) {
	ruleChainEndpoint := &RuleChainEndpoint{
		ruleEngineId: ruleEngineId,
		endpointPool: endpointPool,
		ruleGoPool:   ruleGoPool,
		config:       config,
		endpoints:    make(map[string]endpoint.DynamicEndpoint),
	}
	for _, item := range defs {
		ruleChainEndpoint.bindTo(item, ruleEngineId)
		if err := ruleChainEndpoint.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(ruleGoPool))); err != nil {
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

func (e *RuleChainEndpoint) Reload(newDefs []*types.EndpointDsl) error {
	var oldDefs []*types.EndpointDsl
	endpoints := e.GetEndpoints()
	for _, ep := range endpoints {
		tmp := ep.Definition()
		oldDefs = append(oldDefs, &tmp)
	}
	added, removed, modified := e.checkEndpointChanges(oldDefs, newDefs)
	for _, item := range removed {
		e.RemoveEndpoint(item.Id)
	}
	for _, item := range added {
		e.bindTo(item, e.ruleEngineId)
		if err := e.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(e.ruleGoPool))); err != nil {
			return err
		}
	}
	for _, item := range modified {
		e.bindTo(item, e.ruleEngineId)
		e.RemoveEndpoint(item.Id)
		if err := e.AddEndpointAndStart(item, endpoint.DynamicEndpointOptions.WithRouterOpts(endpoint.RouterOptions.WithRuleGo(e.ruleGoPool))); err != nil {
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
