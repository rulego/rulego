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

// Package endpoint provides a module that abstracts different input source data routing, providing a consistent user experience for different protocols. It is an optional module of `RuleGo` that enables RuleGo to run independently and provide services.
//
// It allows you to easily create and start different receiving services, such as http, mqtt, kafka, gRpc, websocket, schedule, tpc, udp, etc., to achieve data integration of heterogeneous systems, and then perform conversion, processing, flow, etc. operations according to different requests or messages, and finally hand them over to the rule chain or component for processing.
//
// Additionally, it supports dynamic creation and updates through DSL.
//
// # Usage
//
// Endpoint DSL Example:
//
//	{
//	  "id": "e1",
//	  "type": "http",
//	  "name": "http server",
//	  "configuration": {
//	    "server": ":9090"
//	  },
//	 "routers": [
//	   {
//	     "id":"r1",
//	     "params": [
//	       "post"
//	     ],
//	     "from": {
//	       "path": "/api/v1/test/:chainId",
//	       "configuration": {
//	       }
//	     },
//	     "to": {
//	       "path": "${chainId}"
//	     },
//	     "additionalInfo": {
//	       "aa":"aa"
//	     }
//	   }
//	 ]
//	}
//
// Create a endpoint Instance
//
//	ep, err := endpoint.New("e1", []byte(endpointFile))
//
// Start Endpoint
//
//	err := ep.Start()
//
// Get Reload Endpoint
//
//	_ = ep.Reload(newEndpointFile)
//
//	_ = ep.Reload(newEndpointFile, endpoint.DynamicEndpointOptions.WithRestart(true))//Restart Endpoint
//
// Add Or Reload Router
//
//	 routerDsl :=`{
//	     "id":"r1",
//	     "params": [
//	       "post"
//	     ],
//	     "from": {
//	       "path": "/api/v3/test/:chainId",
//	       "configuration": {
//	       }
//	     },
//	     "to": {
//	       "path": "${chainId}"
//	     }
//	   }`
//	_ = ep.AddOrReloadRouter([]byte(routerDsl))
//
// Get Endpoint
//
//	ep,ok:=Get("id")
//
// Destroy Endpoint
//
//	Del("id")
package endpoint

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/json"
	"reflect"
	"sync"
)

// Endpoint is an alias for the Endpoint interface in the endpoint package.
type Endpoint = endpoint.Endpoint

// Exchange is deprecated. Use Flow from github.com/rulego/rulego/api/types/endpoint.Exchange instead.
type Exchange = endpoint.Exchange

// NewRouter creates a new router with the provided options.
func NewRouter(opts ...endpoint.RouterOption) endpoint.Router {
	return impl.NewRouter(opts...)
}

// Ensure DynamicEndpoint implements the DynamicEndpoint interface.
var _ endpoint.DynamicEndpoint = (*DynamicEndpoint)(nil)

// DynamicEndpoint represents a dynamic endpoint with additional properties and methods.
type DynamicEndpoint struct {
	Endpoint
	id         string
	definition types.EndpointDsl
	ruleConfig types.Config
	// Interceptors are the interceptors for the endpoint.
	interceptors []endpoint.Process
	// RouterOpts are the router options for the endpoint.
	routerOpts []endpoint.RouterOption
	// Restart indicates whether the endpoint should be restarted.
	restart bool
	locker  sync.RWMutex
}

// NewFromDsl creates a new DynamicEndpoint from the provided DSL definition and options.
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
func (e *DynamicEndpoint) Id() string {
	return e.id
}

// SetId sets the identifier of the DynamicEndpoint.
func (e *DynamicEndpoint) SetId(id string) {
	e.id = id
}

// SetConfig sets the configuration for the DynamicEndpoint.
func (e *DynamicEndpoint) SetConfig(config types.Config) {
	e.ruleConfig = config
}

// SetRouterOptions sets the router options for the DynamicEndpoint.
func (e *DynamicEndpoint) SetRouterOptions(opts ...endpoint.RouterOption) {
	e.routerOpts = opts
}

// SetRestart sets the restart flag for the DynamicEndpoint.
func (e *DynamicEndpoint) SetRestart(restart bool) {
	e.restart = restart
}

// SetInterceptors sets the interceptors for the DynamicEndpoint.
func (e *DynamicEndpoint) SetInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = interceptors
}

// AddInterceptors adds interceptors to the DynamicEndpoint.
func (e *DynamicEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = append(e.interceptors, interceptors...)
	e.Endpoint.AddInterceptors(interceptors...)
}

// Reload reloads the DynamicEndpoint with the provided definition and options.
func (e *DynamicEndpoint) Reload(dsl []byte, opts ...endpoint.DynamicEndpointOption) error {
	if dsl, err := e.unmarshal(dsl); err != nil {
		return err
	} else {
		return e.ReloadFromDef(dsl, opts...)
	}
}

// AddOrReloadRouter reloads the router for the DynamicEndpoint with the provided definition and options.
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

// newEndpoint creates a new Endpoint with the provided DSL.
func (e *DynamicEndpoint) newEndpoint(dsl types.EndpointDsl) error {
	if ep, err := Registry.New(dsl.Type, e.ruleConfig, dsl.Configuration); err != nil {
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
