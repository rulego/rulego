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

// Package endpoint provides the definitions and structures for endpoints in the rulego.

package endpoint

import (
	"context"
	"errors"
	"github.com/rulego/rulego/api/types"
	"net/textproto"
)

var (
	// ErrServerStopped represents an error when the endpoint is stopped.
	ErrServerStopped = errors.New("endpoint stopped")
)

// Event constants define various event types in the endpoints.
const (
	// EventConnect represents a connection event.
	EventConnect = "Connect"
	// EventDisconnect represents a disconnection event.
	EventDisconnect = "Disconnect"
	// EventInitServer represents an event for server initialization.
	EventInitServer = "InitServer"
	// EventCompletedServer represents an event for a completed server.
	EventCompletedServer = "completedServer"
)

// OnEvent is a function type that listens to named events with optional parameters.
type OnEvent func(eventName string, params ...interface{})

// Endpoint is an interface defining the basic structure of an endpoint in the rulego.
type Endpoint interface {
	types.Node
	// Id returns a unique identifier for the endpoint.
	Id() string
	// SetOnEvent sets the event listener function for the endpoint.
	SetOnEvent(onEvent OnEvent)
	// Start initiates the service.
	Start() error
	// AddInterceptors adds global interceptors to the endpoint.
	AddInterceptors(interceptors ...Process)
	// AddRouter adds a router with optional parameters and returns a router ID.
	// Some endpoints will return a new router ID, which needs to be updated with a new router ID
	AddRouter(router Router, params ...interface{}) (string, error)
	// RemoveRouter removes a router by its ID with optional parameters.
	RemoveRouter(routerId string, params ...interface{}) error
}

// DynamicEndpoint is an interface for dynamically defining an endpoint using a DSL (`types.EndpointDsl`).
type DynamicEndpoint interface {
	Endpoint
	// SetConfig sets the configuration for the dynamic endpoint.
	SetConfig(config types.Config)
	// SetRouterOption sets options for the router.
	SetRouterOption(opts ...RouterOption)
	// SetRestart restart the endpoint.
	SetRestart(restart bool)
	// SetInterceptors sets the interceptors for the dynamic endpoint.
	SetInterceptors(interceptors ...Process)
	// Reload reloads the dynamic endpoint with a new DSL configuration.
	// If need to reload the endpoint, use `endpoint.DynamicEndpointOptions.WithRestart(true)`
	// Else the endpoint only update the route without restarting
	// Routing conflict, endpoint must be restarted
	Reload(dsl []byte, opts ...DynamicEndpointOption) error
	// ReloadRouter reloads or add the router with a new DSL configuration.
	ReloadRouter(dsl []byte, opts ...DynamicEndpointOption) error
	// Definition returns the DSL definition of the dynamic endpoint.
	Definition() types.EndpointDsl
	// DSL returns the DSL configuration of the dynamic endpoint.
	DSL() []byte
}

// Message is an interface abstracting the data received at an endpoint.
type Message interface {
	//Body message body
	Body() []byte
	// Headers returns the message headers.
	Headers() textproto.MIMEHeader
	// From returns the origin of the message.
	From() string
	// GetParam retrieves a parameter value by key.
	GetParam(key string) string
	// SetMsg sets the RuleMsg for the message.
	SetMsg(msg *types.RuleMsg)
	// GetMsg converts the received data to a `types.RuleMsg`.
	GetMsg() *types.RuleMsg
	// SetStatusCode sets the response status code.
	SetStatusCode(statusCode int)
	//SetBody set body
	SetBody(body []byte)
	//SetError set error
	SetError(err error)
	//GetError get error
	GetError() error
}

// Exchange is a structure containing both inbound and outbound messages.
type Exchange struct {
	// In represents the incoming message.
	In Message
	// Out represents the outgoing message.
	Out Message
	// Context provides a context for the exchange.
	Context context.Context
}

// From is an interface representing the source of data in a routing operation.
type From interface {
	// ToString returns a string representation of the source.
	ToString() string
	// Transform applies a transformation process to the source.
	Transform(transform Process) From
	// Process applies a processing function to the source.
	Process(process Process) From
	// GetProcessList returns a list of processes applied to the source.
	GetProcessList() []Process
	// ExecuteProcess executes the processing functions.
	//If the processor returns false, interrupt the execution of subsequent operations
	ExecuteProcess(router Router, exchange *Exchange) bool
	// To defines the destination for the data.
	To(to string, configs ...types.Configuration) To
	// GetTo retrieves the destination configuration.
	GetTo() To
	// ToComponent sets the destination to executed by a specific component.
	ToComponent(node types.Node) To
	// End finalizes the routing configuration.
	End() Router
}

// To is an interface representing the destination of data in a routing operation.
type To interface {
	// ToString returns a string representation of the destination.
	ToString() string
	// Execute performs the routing operation.
	Execute(ctx context.Context, exchange *Exchange)
	// Transform applies a transformation process to the destination.
	Transform(transform Process) To
	// Process applies a processing function to the destination.
	Process(process Process) To
	// Wait for the executor to finish execution before returning to the main coroutine.
	Wait() To
	// IsWait checks if the routing operation is in a wait state.
	IsWait() bool
	// SetOpts applies options to the `types.RuleContextOption`.
	SetOpts(opts ...types.RuleContextOption) To
	// GetOpts returns the applied `types.RuleContextOption`
	GetOpts() []types.RuleContextOption
	// GetProcessList returns a list of processes applied to the destination.
	GetProcessList() []Process
	// ToStringByDict returns a string representation using a dictionary for variable substitution.
	ToStringByDict(dict map[string]string) string
	// End finalizes the routing configuration.
	End() Router
}

// Router is an interface defining the routing operations for data between sources and destinations.
type Router interface {
	// SetId sets the unique identifier for the router.
	SetId(id string) Router
	// GetId retrieves the unique identifier of the router.
	GetId() string
	// FromToString returns a string representation of the source configuration.
	FromToString() string
	// From defines the source for the routing operation.
	From(from string, configs ...types.Configuration) From
	// GetFrom retrieves the source configuration.
	GetFrom() From
	// GetRuleGo retrieves the rule engine pool associated with the exchange.
	GetRuleGo(exchange *Exchange) types.RuleEnginePool
	// GetContextFunc retrieves the context function for the exchange.
	GetContextFunc() func(ctx context.Context, exchange *Exchange) context.Context
	// Disable sets the availability state of the router.
	Disable(disable bool) Router
	// IsDisable checks the availability state of the router.
	IsDisable() bool
}

// Process is a function type defining a processing operation in a routing context.
type Process func(router Router, exchange *Exchange) bool

// OptionsSetter is an interface for setting various options for routing components.
type OptionsSetter interface {
	// SetConfig sets the configuration for the component.
	SetConfig(config types.Config)
	// SetRuleEnginePool sets the rule engine pool for the component.
	SetRuleEnginePool(pool types.RuleEnginePool)
	// SetRuleEnginePoolFunc sets a function to retrieve the rule engine pool.
	SetRuleEnginePoolFunc(f func(exchange *Exchange) types.RuleEnginePool)
	// SetContextFunc sets the context function for the component.
	SetContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context)
}

// Executor is an interface defining the execution operations for the 'to' end of a routing operation.
type Executor interface {
	// New creates a new instance of the executor.
	New() Executor
	// IsPathSupportVar checks if the 'to' path supports variable substitution.
	IsPathSupportVar() bool
	// Init initializes the executor with configuration.
	Init(config types.Config, configuration types.Configuration) error
	// Execute performs the execution operation.
	Execute(ctx context.Context, router Router, exchange *Exchange)
}

// Pool is an interface defining operations for managing a pool of endpoints.
type Pool interface {
	// New creates a new dynamic endpoint.
	New(id string, del []byte, opts ...DynamicEndpointOption) (DynamicEndpoint, error)
	// Get retrieves a dynamic endpoint by its ID.
	Get(id string) (DynamicEndpoint, bool)
	// Del removes a dynamic endpoint instance by its ID.
	Del(id string)
	// Stop releases all dynamic endpoint instances.
	Stop()
	// Reload reloads all dynamic endpoint instances.
	Reload(opts ...DynamicEndpointOption)
	// Range iterates over all dynamic endpoint instances.
	Range(f func(key, value any) bool)
}
