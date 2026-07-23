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
// Endpoint is another name for the Endpoint interface in the endpoint packet.
type Endpoint = endpoint.Endpoint

// Exchange is deprecated. Use Flow from github.com/rulego/rulego/api/types/endpoint.Exchange instead.
// Exchange has been deprecated. Please use Flow in github.com/rulego/rulego/api/types/endpoint.Exchange.
type Exchange = endpoint.Exchange

// NewRouter creates a new router with the provided options.
// This function provides a convenient way to create routers for endpoint configuration.
//
// NewRouter uses the provided options to create a new router.
// This function provides a convenient way to create routers for endpoint configuration.
//
// Parameters:
// Parameters:
//   - opts: Router configuration options
//
// Returns:
// Returns:
//   - endpoint.Router: Configured router instance
func NewRouter(opts ...endpoint.RouterOption) endpoint.Router {
	return impl.NewRouter(opts...)
}

// Ensure DynamicEndpoint implements the DynamicEndpoint interface.
var _ endpoint.DynamicEndpoint = (*DynamicEndpoint)(nil)

// DynamicEndpoint represents a dynamic endpoint with additional properties and methods.
// It provides hot-reloading capabilities and dynamic configuration management for endpoints.
//
// DynamicEndpoint represents a dynamic endpoint with additional properties and methods.
// It provides hot reload functionality and dynamic configuration management for endpoints.
//
// Key Features:
// Key features:
//   - Dynamic DSL-based configuration
//   - Hot reloading without service interruption
//   - Router management with add/remove/update operations
//   - Interceptor support for processing pipelines
//   - Rule chain integration
//   - Thread-safe operations
//
// Lifecycle:
// Lifecycle:
//  1. Creation from DSL configuration
//  2. Router and interceptor setup
//  3. Service startup
//  4. Dynamic updates and reloads
//  5. Graceful shutdown and cleanup
//
// Configuration Management:
// Configuration Management:
//   - Supports JSON DSL for declarative configuration
//   - Enables runtime configuration changes
//   - Validates configuration before applying changes
//   - Maintains configuration history for rollback
type DynamicEndpoint struct {
	// Endpoint is the embedded endpoint implementation providing core functionality
	// Endpoint is an embedded endpoint implementation that provides core functionality
	Endpoint

	// id is the unique identifier for this endpoint instance
	// id is the unique identifier for this endpoint instance
	id string

	// ruleChain contains the rule chain DSL definition when initialized from rule chain
	// ruleChain contains the rule chain DSL definition when initialized from the rule chain
	ruleChain *types.RuleChain

	// definition contains the endpoint DSL configuration
	// definition includes endpoint DSL configurations
	definition types.EndpointDsl

	// ruleConfig contains the rule engine configuration
	// ruleConfig contains the rule engine configuration
	ruleConfig types.Config

	// interceptors are the processing interceptors for the endpoint
	// Interceptors are endpoint interceptors
	interceptors []endpoint.Process

	// routerOpts are the router configuration options for the endpoint
	// routerOpts are the router configuration options for endpoints
	routerOpts []endpoint.RouterOption

	// restart indicates whether the endpoint should be restarted during reload
	// restart indicates whether the endpoint should be restarted during the reload
	restart bool

	// locker provides thread-safe access to endpoint state
	// Locker provides thread-safe access to endpoint states
	locker sync.RWMutex
}

// NewFromDsl creates a new DynamicEndpoint from the provided DSL definition and options.
// This function parses JSON DSL configuration and creates a fully configured dynamic endpoint.
//
// NewFromDsl creates a new DynamicEndpoint from the provided DSL definitions and options.
// This function parses JSON DSL configurations and creates fully configured dynamic endpoints.
//
// Parameters:
// Parameters:
//   - def: JSON DSL definition bytes JSON DSL definition bytes
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - *DynamicEndpoint: Configured dynamic endpoint
//   - error: Creation error if any
//
// Example DSL:
// DSL example:
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
// NewFromDef creates a new DynamicEndpoint from the provided DSL definition structure and options.
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
// Id returns the DynamicEndpoint identifier.
func (e *DynamicEndpoint) Id() string {
	return e.id
}

// SetId sets the identifier of the DynamicEndpoint.
// SetId sets the identifier for DynamicEndpoint.
func (e *DynamicEndpoint) SetId(id string) {
	e.id = id
}

// SetConfig sets the configuration for the DynamicEndpoint.
// SetConfig sets the configuration of the DynamicEndpoint.
func (e *DynamicEndpoint) SetConfig(config types.Config) {
	e.ruleConfig = config
}

// SetRouterOptions sets the router options for the DynamicEndpoint.
// SetRouterOptions sets the router options for DynamicEndpoint.
func (e *DynamicEndpoint) SetRouterOptions(opts ...endpoint.RouterOption) {
	e.routerOpts = opts
}

// SetRestart sets the restart flag for the DynamicEndpoint.
// SetRestart sets the restart flag for DynamicEndpoint.
func (e *DynamicEndpoint) SetRestart(restart bool) {
	e.restart = restart
}

// SetInterceptors sets the interceptors for the DynamicEndpoint.
// SetInterceptors sets the interceptor for DynamicEndpoint.
func (e *DynamicEndpoint) SetInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = interceptors
}

// AddInterceptors adds interceptors to the DynamicEndpoint.
// AddInterceptors adds interceptors to DynamicEndpoint.
func (e *DynamicEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = append(e.interceptors, interceptors...)
	e.Endpoint.AddInterceptors(interceptors...)
}

// Reload reloads the DynamicEndpoint with the provided definition and options.
// This method supports hot reloading of endpoint configuration without service interruption.
//
// Reload Reloads DynamicEndpoint using the provided definitions and options.
// This method supports hot-reloading endpoint configuration without interrupting services.
//
// Parameters:
// Parameters:
//   - dsl: New JSON DSL configuration
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - error: Reload error if any
//
// Hot Reload Process:
// Thermal Heavy-Loading Process:
//  1. Parse new DSL configuration
//  2. Compare with current configuration
//  3. Determine if restart is needed
//  4. Apply configuration changes
//  5. Update routers as needed
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
// AddOrReloadRouter uses the provided definitions and options to reload the router for DynamicEndpoint.
// This method allows dynamic addition or modification of individual routers without affecting the entire endpoint.
//
// Parameters:
// Parameters:
//   - dsl: Router JSON DSL configuration
//   - opts: Optional configuration functions
//
// Returns:
// Returns:
//   - error: Operation error if any
//
// Router Management:
// Router Management:
//   - Automatically removes existing router with same ID
//   - Validates router configuration before applying
//   - Supports both addition and modification operations
//   - Can trigger endpoint restart if configured
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
	//Inject a complete rule chain definition
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
