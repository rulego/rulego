/*
 * Copyright 2023 The RuleGo Authors.
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

// Package impl provides the core implementation of the endpoint module.
// It includes structures and methods for handling endpoints, routers,
// and message processing in the RuleGo framework.
//
// Package impl provides the core implementation of endpoint modules.
// It includes the structure and methods for handling endpoints, routers, and message processing within the RuleGo framework.
//
// # Core Components
//
// • From: Represents the source of incoming data with processing capabilities
// • To: Represents the destination for processed data with target execution
// • Router: Manages the routing of messages between From and To
// • BaseEndpoint: Base implementation for endpoint functionality
// • Executors: Different execution strategies for processing messages
//
// # Message Flow
//
// The implementation follows a pipeline pattern where messages flow through:
// Implementation follows a pipeline pattern, with messages flowing through:
//
// 1. From: Input processing and transformation
// 2. To: Target execution (rule chains or components)
// 3. Process: Post-processing and response handling
//
// # Configuration
//
// The implementation supports flexible configuration through:
// Flexible configuration is supported in the following ways:
//
// • Dynamic path variables using ${var} syntax
// • Multiple processing interceptors
// • Different executor types (chain, component)
// • Synchronous and asynchronous execution modes
package impl

import (
	"context"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/str"
)

const (
	pathKey = "_path"
	//Split flags such as: {chainId}:{nodeId} Split flag like: {chainId}:{nodeId}
	pathSplitFlag = ":"
)

var _ endpoint.From = (*From)(nil)

// From represents the input source of a router with processing capabilities.
// It handles incoming data transformation, processing, and routing to target destinations.
//
// From indicates the router input source with processing power.
// It handles the transformation, processing, and routing of incoming data to its destination.
//
// Architecture
// • Configuration Management: Stores source-specific configuration
// • Process Pipeline: Chain of processing functions for data transformation
// • Router Integration: Back-reference to the parent router
// • Target Binding: Connection to the destination (To) endpoint
//
// Processing Flow
// 1. Receive input data
// 2. Execute processing pipeline
// 3. Route to target destination
// 4. Handle responses if needed
type From struct {
	//Config Configuration for the endpoint Configuration of the endpoint
	Config types.Configuration
	//Router pointer to the parent router
	Router *Router
	//Source path pattern for input matching
	From string
	//Message processing interceptors
	processList []endpoint.Process
	//The target flow path, for example, "chain:{chainId}", is handed over to the rule engine to process the data. Target flow path, e.g., "chain:{chainId}" for rule engine processing The target flow path
	to *To
}

// ToString returns the string representation of the From path.
// This is used for identification and logging purposes.
//
// ToString returns the string representation of the From path.
// Used for identification and log recording purposes.
func (f *From) ToString() string {
	return f.From
}

// GetConfiguration returns the configuration for this From source.
// Processors can use this to access configuration values.
//
// GetConfiguration returns the configuration of this From source.
// The processor can access configuration values using this method.
func (f *From) GetConfiguration() types.Configuration {
	return f.Config
}

// Transform adds a transformation processor to the From processing pipeline.
// Transformations are applied to incoming data before routing to the destination.
//
// Transform adds a conversion processor to the From processing pipeline.
// Transformations are applied to incoming data before routing to the target.
//
// Parameters
// • transform: The transformation function to apply
//
// Returns
// • endpoint.From: Returns self for method chaining
func (f *From) Transform(transform endpoint.Process) endpoint.From {
	f.processList = append(f.processList, transform)
	return f
}

// Process adds a processing function to the From processing pipeline.
// Processing functions can modify, validate, or filter incoming data.
//
// Process: Add a handler function to the From processing pipe.
// Processing functions can modify, validate, or filter incoming data.
//
// Parameters
// • process: The processing function to add
//
// Returns
// • endpoint.From: Returns self for method chaining
func (f *From) Process(process endpoint.Process) endpoint.From {
	f.processList = append(f.processList, process)
	return f
}

// GetProcessList returns the list of processing functions in the pipeline.
// Used internally for execution and inspection purposes.
//
// GetProcessList returns a list of handler functions in the pipeline.
// Used for internal execution and inspection purposes.
func (f *From) GetProcessList() []endpoint.Process {
	return f.processList
}

// ExecuteProcess executes all processing functions in the pipeline sequentially.
// If any processing function returns false, the pipeline stops and returns false.
//
// ExecuteProcess executes all processing functions in the pipeline in order.
// If any handler returns false, the pipeline stops and returns false.
//
// Parameters
// • router: The router context
// • exchange: The message exchange containing input/output data
//
// Returns
// • bool: true if all processing succeeded, false otherwise
func (f *From) ExecuteProcess(router endpoint.Router, exchange *endpoint.Exchange) bool {
	result := true
	for _, process := range f.GetProcessList() {
		if !process(router, exchange) {
			result = false
			break
		}
	}
	return result
}

// To creates and configures the destination endpoint for message routing.
// The destination format follows the pattern: {executorType}:{path}
//
// To create and configure the destination endpoint for message routing.
// Target format follows the pattern: {executorType}:{path}
//
// Parameters
// • to: Target path string in format "executorType:path"
//   - "chain:{chainId}": Route to a rule chain
//   - "chain:{chainId}:{nodeId}": Route to specific node in rule chain
//   - "component:{nodeType}": Route to a registered component
//
// • configs: Optional configuration parameters for the destination
//
// Returns
// • endpoint.To: The configured destination endpoint
//
// Executor Types
// • chain: Rule chain executor for processing with rule engines
// • component: Component executor for individual node processing
//
// Variable Support
// The path can contain variables like "${userId}" that will be resolved at runtime
// Paths can contain variables like "${userId}" and will be parsed at runtime
func (f *From) To(to string, configs ...types.Configuration) endpoint.To {
	var toConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			toConfig[k] = v
		}
	}
	f.to = &To{Router: f.Router, To: to, Config: toConfig}

	//Check if the path contains variables like: chain:${userId}
	if strings.Contains(to, "${") && strings.Contains(to, "}") {
		f.to.HasVars = true
	}

	//Get To executor type
	executorType := strings.Split(to, pathSplitFlag)[0]

	//Get To executor
	if executor, ok := DefaultExecutorFactory.New(executorType); ok {
		if f.to.HasVars && !executor.IsPathSupportVar() {
			f.Router.err = fmt.Errorf("executor=%s, path not support variables", executorType)
			return f.to
		}
		f.to.ToPath = strings.TrimSpace(to[len(executorType)+1:])
		toConfig[pathKey] = f.to.ToPath
		//Initialize component
		err := executor.Init(f.Router.Config, toConfig)
		if err != nil {
			f.Router.err = err
			return f.to
		}
		f.to.executor = executor
	} else {
		f.to.executor = &ChainExecutor{}
		f.to.ToPath = to
	}
	return f.to
}

// GetTo returns the configured destination endpoint.
// Returns nil if no destination has been configured.
//
// GetTo returns the configured target endpoint.
// If no target is configured, nil is returned.
func (f *From) GetTo() endpoint.To {
	if f.to == nil {
		return nil
	}
	return f.to
}

// ToComponent creates a destination that routes directly to a specific component node.
// This bypasses the executor factory and uses the component directly.
//
// ToComponent creates a target that is routed directly to a specific component node.
// This bypasses the actuator factory and uses components directly.
//
// Parameters
// • node: The component node to route to
//
// Returns
// • endpoint.To: The configured component destination
func (f *From) ToComponent(node types.Node) endpoint.To {
	component := &ComponentExecutor{component: node, config: f.Router.Config}
	f.to = &To{Router: f.Router, To: node.Type(), ToPath: node.Type()}
	f.to.executor = component
	return f.to
}

// End completes the From configuration and returns the parent router.
// Used for method chaining to continue router configuration.
//
// End completes the From configuration and returns to the parent router.
// Used for method chains to continue router configuration.
func (f *From) End() endpoint.Router {
	return f.Router
}

// To represents the destination endpoint for message processing.
// It handles the execution of target logic and post-processing of results.
//
// To indicates the target endpoint for message processing.
// It handles the execution of objective logic and post-processing of results.
//
// Architecture
// • Variable Resolution: Supports dynamic path variables
// • Executor Integration: Pluggable execution strategies
// • Process Pipeline: Post-processing functions for results
// • Synchronous Support: Optional blocking execution for responses
//
// Execution Modes
// • Asynchronous: Fire-and-forget message processing
// • Synchronous: Wait for execution completion and results
type To struct {
	//toPath has placeholder variables toPath has placeholder variables
	HasVars bool
	//Config to Component Configuration Configuration for To component To Component Configuration
	Config types.Configuration
	//Router pointer to parent router
	Router *Router
	//The target flow path, for example, "chain:{chainId}", is handed over to the rule engine to process the data. Target flow path, e.g., "chain:{chainId}" for rule engine processing The target flow path
	To string
	//Delete the path marked by the executor type prefix removed
	ToPath string
	//Message processing interceptors
	processList []endpoint.Process
	//Target processor, default to rule chain processing
	executor endpoint.Executor
	//Wait for the rule chain/component to finish executing and restore to the parent process to synchronize the rule chain results. Wait for rule chain/component execution completion and return to parent process, synchronously get rule chain results
	//Used in scenarios where you need to wait for the execution result of the rule chain and keep the parent process; otherwise, this field does not need to be set. For example: HTTP response. Used for scenarios requiring rule chain execution results while preserving parent process, e.g., HTTP responses
	wait bool
	//Rule context configuration, if `types.WithOnEnd` needs to take over the `ChainExecutor` result response logic Rule context configuration, requires handling ChainExecutor result response logic if 'types. WithOnEnd` is configured
	opts []types.RuleContextOption
}

// ToStringByDict resolves variables in the path using the provided dictionary and returns the final string.
// This enables dynamic routing based on message metadata or other runtime values.
//
// ToStringByDict uses the provided dictionary to parse variables in the path and return the final string.
// This enables dynamic routing based on message metadata or other runtime values.
//
// Parameters
// • dict: Variable dictionary for path resolution
//
// Returns
// • string: Resolved path with variables substituted
func (t *To) ToStringByDict(dict map[string]string) string {
	if t.HasVars {
		return str.SprintfDict(t.ToPath, dict)
	}
	return t.ToPath
}

// ToString returns the string representation of the To path.
// Used for identification and logging purposes.
//
// ToString returns the string representation of the To path.
// Used for identification and log recording purposes.
func (t *To) ToString() string {
	return t.ToPath
}

// Execute executes the To endpoint logic using the configured executor.
// This is the main entry point for processing messages at the destination.
//
// Execute executes the To endpoint logic using the configured executor.
// This is the main entry point for target processing messages.
//
// Parameters
// • ctx: Execution context
// • exchange: Message exchange containing input/output data
func (t *To) Execute(ctx context.Context, exchange *endpoint.Exchange) {
	if t.executor != nil {
		t.executor.Execute(ctx, t.Router, exchange)
	}
}

// Transform adds a transformation processor to the To post-processing pipeline.
// These transformations are applied to results after To logic execution.
// If the rule chain has multiple end points, this will be executed multiple times.
//
// Transform adds a conversion processor to the To post-processing pipeline.
// These transformations are applied to the results after the To logic is executed.
// If the rule chain has multiple endpoints, it will be executed multiple times.
func (t *To) Transform(transform endpoint.Process) endpoint.To {
	t.processList = append(t.processList, transform)
	return t
}

// Process adds a processing function to the To post-processing pipeline.
// These processors handle results after To logic execution.
// If the rule chain has multiple end points, this will be executed multiple times.
//
// Process adds a handler function to the To post-processing pipeline.
// These processors process the results after the To logic is executed.
// If the rule chain has multiple endpoints, it will be executed multiple times.
func (t *To) Process(process endpoint.Process) endpoint.To {
	t.processList = append(t.processList, process)
	return t
}

// Wait enables synchronous execution mode for the To endpoint.
// When enabled, the execution waits for rule chain/component completion and returns to the parent process.
// This is used for scenarios requiring rule chain execution results while preserving the parent process.
// Example use case: HTTP response handling.
//
// Wait enables synchronous execution mode for the To endpoint.
// When enabled, the execution waits for the rule chain/component to complete and return to the parent process.
// Used in scenarios where the execution result of the rule chain is needed and the parent process is kept.
// Example of use: HTTP response processing.
func (t *To) Wait() endpoint.To {
	t.wait = true
	return t
}

// SetWait configures the synchronous execution mode.
//
// SetWait configures synchronous execution mode.
func (t *To) SetWait(wait bool) endpoint.To {
	t.wait = wait
	return t
}

// IsWait returns whether synchronous execution mode is enabled.
//
// IsWait returns whether synchronous execution mode is enabled.
func (t *To) IsWait() bool {
	return t.wait
}

// SetOpts configures rule context options for the To execution.
// These options are passed to the rule engine when executing rule chains.
//
// SetOpts to configure the rule context option for To execution.
// These options are passed to the rule engine when executing the rule chain.
func (t *To) SetOpts(opts ...types.RuleContextOption) endpoint.To {
	t.opts = opts
	return t
}

// GetOpts returns the configured rule context options.
//
// GetOpts returns the rule context options for the configuration.
func (t *To) GetOpts() []types.RuleContextOption {
	return t.opts
}

// GetProcessList returns the list of post-processing functions.
// Used internally for execution and inspection purposes.
//
// GetProcessList returns a list of post-processing functions.
// Used for internal execution and inspection purposes.
func (t *To) GetProcessList() []endpoint.Process {
	return t.processList
}

// End completes the To configuration and returns the parent router.
// Used for method chaining to continue router configuration.
//
// End completes the To configuration and returns to the parent router.
// Used for method chains to continue router configuration.
func (t *To) End() endpoint.Router {
	return t.Router
}

// Router provides message routing abstraction for different input sources.
// It manages the flow of messages from input endpoints (From), through transformation/processing,
// to target destinations (To) such as rule chains or components.
//
// The Router provides message routing abstraction for different input sources.
// It manages the flow of messages from the input endpoint (From) through transformation/processing to the target destination (To) (such as a rule chain or component).
//
// Architecture
// • Fluent API: Chain method calls for intuitive configuration
// • Context Management: Handles execution context and lifecycle
// • Pool Integration: Manages rule engine pool access
// • State Management: Tracks router state and configuration
//
// # Usage Patterns
//
// HTTP Endpoint Examples / HTTP Endpoint Examples:
//
//	router.From("/api/v1/msg/").Transform().To("chain:xx")
//	router.From("/api/v1/msg/").Transform().Process().To("chain:xx")
//	router.From("/api/v1/msg/").Transform().Process().To("component:nodeType")
//	router.From("/api/v1/msg/").Transform().Process()
//
// MQTT Endpoint Examples / MQTT Endpoint Examples:
//
//	router.From("#").Transform().Process().To("chain:xx")
//	router.From("device/+/msg").Transform().Process().To("chain:xx")
//
// Configuration
// • Dynamic Rule Engine Pool: Support for runtime pool selection
// • Context Customization: Custom context creation for each exchange
// • Error Handling: Centralized error tracking and reporting
// • State Control: Enable/disable routing dynamically
type Router struct {
	//Context creation callback function
	ContextFunc func(ctx context.Context, exchange *endpoint.Exchange) context.Context
	//Config ruleEngine Config  Rule engine configuration
	Config types.Config
	//Router unique identifier
	id string
	//Input endpoint configuration
	from *From
	//Rule chain pool, default uses engine.DefaultPool Rule chain pool, defaults to engine. DefaultPool
	RuleGo types.RuleEnginePool
	//Dynamic rule chain pool function for dynamic rule chain pool retrieval
	ruleGoFunc func(exchange *endpoint.Exchange) types.RuleEnginePool
	//Unavailable: 1: Unavailable; 0: Yes Disable state: 1=disabled, 0=enabled
	disable uint32
	//Routing definition: If not set, it will return nil Router definition, returns nil if not set
	def *types.RouterDsl
	//Configuration parameters
	params []interface{}
	//Records initialization errors
	err error
}

// RouterOption is a type alias for router configuration options.
// It enables functional options pattern for router customization.
//
// RouterOption is a type alias for router configuration options.
// It enables function options mode for router customization.
type RouterOption = endpoint.RouterOption

// NewRouter creates a new router instance with optional configuration.
// The router is initialized with default settings and can be customized using options.
//
// NewRouter creates a new router instance using optional configurations.
// The router is initialized using default settings, which can be customized using options.
//
// Parameters
// • opts: Optional configuration functions
//
// Returns
// • endpoint.Router: Configured router instance
//
// Default Configuration
// • RuleGo: Uses engine.DefaultPool for rule engine access
// • Config: Uses engine.NewConfig() for basic configuration
//
// Usage Example
//
//	router := NewRouter(
//	    RouterOptions.WithConfig(customConfig),
//	    RouterOptions.WithRuleEnginePool(customPool),
//	)
func NewRouter(opts ...RouterOption) endpoint.Router {
	router := &Router{RuleGo: engine.DefaultPool, Config: engine.NewConfig()}
	// Apply option values
	for _, opt := range opts {
		_ = opt(router)
	}
	return router
}

func (r *Router) SetConfig(config types.Config) {
	r.Config = config
}

func (r *Router) SetRuleEnginePool(pool types.RuleEnginePool) {
	r.RuleGo = pool
}

func (r *Router) SetRuleEnginePoolFunc(f func(exchange *endpoint.Exchange) types.RuleEnginePool) {
	r.ruleGoFunc = f
}

func (r *Router) SetContextFunc(f func(ctx context.Context, exchange *endpoint.Exchange) context.Context) {
	r.ContextFunc = f
}

func (r *Router) GetContextFunc() func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
	return r.ContextFunc
}

func (r *Router) SetDefinition(def *types.RouterDsl) {
	r.def = def
}

// Definition returns the routing definition; if not set, it will return nil
func (r *Router) Definition() *types.RouterDsl {
	return r.def
}

func (r *Router) SetId(id string) endpoint.Router {
	r.id = id
	return r
}

func (r *Router) GetId() string {
	return r.id
}
func (r *Router) FromToString() string {
	if r.from == nil {
		return ""
	} else {
		return r.from.ToString()
	}
}

func (r *Router) From(from string, configs ...types.Configuration) endpoint.From {
	var fromConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			fromConfig[k] = v
		}
	}
	r.from = &From{Router: r, From: from, Config: fromConfig}

	return r.from
}

func (r *Router) GetFrom() endpoint.From {
	if r.from == nil {
		return nil
	}
	return r.from
}

func (r *Router) GetRuleGo(exchange *endpoint.Exchange) types.RuleEnginePool {
	if r.ruleGoFunc != nil {
		return r.ruleGoFunc(exchange)
	} else {
		return r.RuleGo
	}
}

// Disdisable sets the status true: unavailable, false: yes
func (r *Router) Disable(disable bool) endpoint.Router {
	if disable {
		atomic.StoreUint32(&r.disable, 1)
	} else {
		atomic.StoreUint32(&r.disable, 0)
	}
	return r
}

// Is IsDisable unavailable? true: unavailable, false: yes
func (r *Router) IsDisable() bool {
	return atomic.LoadUint32(&r.disable) == 1
}

func (r *Router) SetParams(args ...interface{}) {
	r.params = args
}

func (r *Router) GetParams() []interface{} {
	return r.params
}
func (r *Router) Err() error {
	return r.err
}

// BaseEndpoint provides the fundamental implementation for all endpoint types.
// It implements common functionality including global interceptors, router management,
// and thread-safe operations for endpoint lifecycle management.
//
// BaseEndpoint provides foundational implementations for all endpoint types.
// It implements universal functions, including thread security operations for global interceptors, router management, and endpoint lifecycle management.
//
// Architecture
// • Router Management: Thread-safe storage and retrieval of routers
// • Global Interceptors: Cross-cutting concerns for all message processing
// • Event Handling: Callback mechanism for endpoint lifecycle events
// • Thread Safety: RWMutex protection for concurrent access
//
// Interceptor Pipeline
// The BaseEndpoint processes messages through the following pipeline:
// BaseEndpoint processes messages through the following channels:
//
// 1. Global interceptors execution
// 2. From endpoint processing
// 3. To endpoint execution
// 4. Post-processing and response handling
//
// Concurrency
// • Safe for concurrent router addition/removal
// • Lock-free interceptor access using exported field
// • Context creation and lifecycle management
type BaseEndpoint struct {
	//Endpoint Router storage for the endpoint
	RouterStorage map[string]endpoint.Router
	//Event callback handler
	OnEvent endpoint.OnEvent
	//Global interceptors - exported fields for direct access to avoid lock contention
	Interceptors []endpoint.Process
	//Logger
	Logger types.Logger
	sync.RWMutex
}

func (e *BaseEndpoint) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	panic("not support this method")
}

func (e *BaseEndpoint) SetOnEvent(onEvent endpoint.OnEvent) {
	e.OnEvent = onEvent
}

func (e *BaseEndpoint) Printf(format string, v ...interface{}) {
	if e.Logger != nil {
		e.Logger.Printf(format, v...)
	}
}

func (e *BaseEndpoint) Debugf(format string, v ...interface{}) {
	if e.Logger != nil {
		e.Logger.Debugf(format, v...)
	}
}

func (e *BaseEndpoint) Infof(format string, v ...interface{}) {
	if e.Logger != nil {
		e.Logger.Infof(format, v...)
	}
}

func (e *BaseEndpoint) Warnf(format string, v ...interface{}) {
	if e.Logger != nil {
		e.Logger.Warnf(format, v...)
	}
}

func (e *BaseEndpoint) Errorf(format string, v ...interface{}) {
	if e.Logger != nil {
		e.Logger.Errorf(format, v...)
	}
}

// AddInterceptors adds global interceptors to the endpoint processing pipeline.
// These interceptors are executed for all incoming messages before routing logic.
// Interceptors are applied in the order they are added.
//
// AddInterceptors: Adds a global interceptor to the endpoint processing pipe.
// These interceptors execute all incoming messages before routing logic.
// Interceptors are applied in the order they are added.
//
// Parameters
// • interceptors: Processing functions to add to the global pipeline
//
// Thread Safety
// This method is not thread-safe and should be called during initialization.
// This method is not thread-safe and should be called during initialization.
func (e *BaseEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.Interceptors = append(e.Interceptors, interceptors...)
}

// DoProcess executes the complete message processing pipeline for the endpoint.
// This is the main entry point for processing incoming messages through the endpoint system.
// The processing follows a specific order: context creation, global interceptors,
// From endpoint processing, and finally To endpoint execution.
//
// DoProcess executes the endpoint's complete message processing pipeline.
// This is the main entry point for processing incoming messages through the endpoint system.
// Processing follows a specific sequence: context creation, global interceptor, From endpoint processing, and finally To endpoint execution.
//
// Parameters
// • baseCtx: Base context for the processing
// • router: Router configuration for message routing
// • exchange: Message exchange containing input and output data
//
// Processing Pipeline
// 1. Create execution context using router's context function
// 2. Execute global interceptors in sequence
// 3. Execute From endpoint processing pipeline
// 4. Execute To endpoint target logic
//
// Early Termination
// If any interceptor or From processing returns false, the pipeline terminates early
// and subsequent steps are not executed.
// If any interceptor or From process returns false, the pipeline terminates early and subsequent steps will not be executed.
//
// Context Cancellation
// The method checks for context cancellation at key points to support graceful shutdown.
// This method checks context cancellation at key points to support elegant downtime.
//
// Thread Safety
// This method creates a thread-safe copy of interceptors to avoid race conditions
// during concurrent interceptor modifications.
// This method creates thread-safe copies of the interceptor to avoid race conditions during concurrent interceptor modifications.
func (e *BaseEndpoint) DoProcess(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	// Check if context is already cancelled before starting processing
	// Check whether the context has been removed before starting processing
	if baseCtx != nil {
		select {
		case <-baseCtx.Done():
			// Context cancelled, set error and return early
			// The context is canceled, the error is set, and the return is premature
			exchange.Out.SetError(fmt.Errorf("processing cancelled: %w", baseCtx.Err()))
			return
		default:
		}
	}

	//Create context
	ctx := e.createContext(baseCtx, router, exchange)

	// Thread-safely get interceptor copy
	e.RLock()
	interceptors := make([]endpoint.Process, len(e.Interceptors))
	copy(interceptors, e.Interceptors)
	e.RUnlock()

	for _, item := range interceptors {
		// Check for context cancellation before each interceptor
		// Check context cancellation before each interceptor
		if ctx != nil {
			select {
			case <-ctx.Done():
				exchange.Out.SetError(fmt.Errorf("processing cancelled during interceptor: %w", ctx.Err()))
				return
			default:
			}
		}

		//Execute global interceptors
		if !item(router, exchange) {
			return
		}
	}

	// Check for context cancellation before From processing
	// Check for context cancellation before From processing
	if ctx != nil {
		select {
		case <-ctx.Done():
			exchange.Out.SetError(fmt.Errorf("processing cancelled before From: %w", ctx.Err()))
			return
		default:
		}
	}

	//Execute from endpoint logic
	if fromFlow := router.GetFrom(); fromFlow != nil {
		if !fromFlow.ExecuteProcess(router, exchange) {
			return
		}
	}

	// Check for context cancellation before To processing
	// Check context cancellation before To processing
	if ctx != nil {
		select {
		case <-ctx.Done():
			exchange.Out.SetError(fmt.Errorf("processing cancelled before To: %w", ctx.Err()))
			return
		default:
		}
	}

	//Execute To endpoint logic
	if router.GetFrom() != nil && router.GetFrom().GetTo() != nil {
		router.GetFrom().GetTo().Execute(ctx, exchange)
	}
}

func (e *BaseEndpoint) createContext(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) context.Context {
	if router.GetContextFunc() != nil {
		if ctx := router.GetContextFunc()(baseCtx, exchange); ctx == nil {
			return context.Background()
		} else {
			exchange.Context = ctx
			return ctx
		}
	} else if baseCtx != nil {
		return baseCtx
	} else {
		return context.Background()
	}

}

func (e *BaseEndpoint) CheckAndSetRouterId(router endpoint.Router) string {
	if router.GetId() == "" {
		router.SetId(router.FromToString())
	}
	return router.GetId()
}

func (e *BaseEndpoint) Destroy() {
	e.Lock()
	defer e.Unlock()
	e.Interceptors = nil
	// Create a new map instead of clearing the existing one to avoid race conditions
	e.RouterStorage = make(map[string]endpoint.Router)
}

func (e *BaseEndpoint) GetRuleChainDefinition(configuration types.Configuration) *types.RuleChain {
	if v, ok := configuration[types.NodeConfigurationKeyRuleChainDefinition]; ok {
		if ruleNode, ok := v.(*types.RuleChain); ok {
			return ruleNode
		}
	}
	return nil
}

func (e *BaseEndpoint) HasRouter(id string) bool {
	e.RLock()
	defer e.RUnlock()
	_, ok := e.RouterStorage[id]
	return ok
}

// ExecutorFactory is a registry and factory for To endpoint executors.
// It manages different types of executors that handle the final destination logic
// for message processing in the endpoint system.
//
// ExecutorFactory is the registry and factory for To Endpoint Executors.
// It manages different types of actuators and handles the ultimate logic for message processing in endpoint systems.
//
// Architecture
// • Registration: Thread-safe executor type registration
// • Factory Pattern: Creates new executor instances on demand
// • Type Safety: Maps executor names to their implementations
//
// Built-in Executors
// • "chain": Rule chain executor for routing to rule engines
// • "component": Component executor for direct node processing
//
// Thread Safety
// All operations are protected by RWMutex for concurrent access safety.
// All operations are protected by RWMutex to ensure secure concurrent access.
type ExecutorFactory struct {
	sync.RWMutex
	//Executor registry, mapping names to implementations
	executors map[string]endpoint.Executor
}

// Register adds a new executor type to the factory.
// The executor serves as a prototype for creating new instances.
//
// Register adds a new actuator type to the factory.
// The actuator serves as a prototype for creating new instances.
//
// Parameters
// • name: Unique identifier for the executor type
// • executor: Prototype executor implementation
//
// Thread Safety
// This method is thread-safe and can be called concurrently.
// This method is thread-safe and can be called concurrently.
func (r *ExecutorFactory) Register(name string, executor endpoint.Executor) {
	r.Lock()
	defer r.Unlock()
	if r.executors == nil {
		r.executors = make(map[string]endpoint.Executor)
	}
	r.executors[name] = executor
}

// New creates a new executor instance by type name.
// Returns a new instance of the registered executor or false if not found.
//
// New creates a new executor instance based on the type name.
// Returns a new instance of the registered executor; if not found, returns false.
//
// Parameters
// • name: The executor type name to create
//
// Returns
// • endpoint.Executor: New executor instance
// • bool: True if executor type was found
//
// Thread Safety
// This method is thread-safe and uses read lock for optimal performance.
// This method is thread-safe and uses read locks for optimal performance.
func (r *ExecutorFactory) New(name string) (endpoint.Executor, bool) {
	r.RLock()
	defer r.RUnlock()
	h, ok := r.executors[name]
	if ok {
		return h.New(), true
	} else {
		return nil, false
	}
}

// ChainExecutor is an executor implementation that routes messages to rule chains.
// It handles the integration between endpoint routing and the RuleGo rule engine,
// supporting both synchronous and asynchronous execution modes.
//
// ChainExecutor is an executor that routes messages to the rule chain.
// It handles the integration between endpoint routing and the RuleGo rule engine, supporting both synchronous and asynchronous execution modes.
//
// Features
// • Dynamic Path Resolution: Supports variable substitution in chain paths
// • Multi-mode Execution: Synchronous and asynchronous processing
// • Node Targeting: Can route to specific nodes within a rule chain
// • Error Handling: Comprehensive error reporting and callback integration
//
// Path Format
// • "chainId": Route to rule chain root
// • "chainId:nodeId": Route to specific node in chain
//
// Variable Support
// Paths can contain variables like "${userId}" that are resolved from message metadata.
// Paths can contain variables like "${userId}" and parse from message metadata.
type ChainExecutor struct {
}

func (ce *ChainExecutor) New() endpoint.Executor {

	return &ChainExecutor{}
}

// The IsPathSupportVar to path allows variables
func (ce *ChainExecutor) IsPathSupportVar() bool {
	return true
}

func (ce *ChainExecutor) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

func (ce *ChainExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	fromFlow := router.GetFrom()
	if fromFlow == nil {
		return
	}
	inMsg := exchange.In.GetMsg()
	if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
		toChainId := toFlow.ToStringByDict(inMsg.Metadata.GetReadOnlyValues())
		tos := strings.Split(toChainId, pathSplitFlag)
		toChainId = tos[0]
		//Find the rule chain and execute it
		if ruleEngine, ok := router.GetRuleGo(exchange).Get(toChainId); ok {
			opts := toFlow.GetOpts()
			//End of listening, callback function
			endFunc := types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				// Create a ScopedMessage proxy to isolate the message state of each callback
				// Create ScopedMessage proxy to isolate message state for each callback
				scopedOut := &ScopedMessage{
					Message: exchange.Out, // Retain references to the original ResponseMessage (for IO operations)
					msg:     &msg,         // Bind specific data for the current callback
				}

				// Create ScopedExchange using the proxy's Out message
				// Create ScopedExchange using the proxied Out message
				// Note: This is a shallow copy, with Context and In unchanged, but Out replaced by ScopedMessage
				// RWMutex is reset because it is a value copy, but in the new exchange, we should not use the old lock
				scopedExchange := &endpoint.Exchange{
					In:      exchange.In,
					Out:     scopedOut,
					Context: exchange.Context,
				}

				if err != nil {
					scopedExchange.Out.SetError(err)
				}

				// Subsequent processors are executed using ScopedExchange
				// Execute processors using ScopedExchange
				for _, process := range toFlow.GetProcessList() {
					if !process(router, scopedExchange) {
						break
					}
				}
			})
			opts = append(opts, types.WithContext(ctx))
			if len(tos) > 1 {
				opts = append(opts, types.WithStartNode(tos[1]))
			}
			opts = append(opts, endFunc)
			// If the message metadata contains _debugMode parameters, dynamically enable per-message debugging mode
			if debugModeVal := inMsg.Metadata.GetValue(types.KeyDebugMode); debugModeVal == types.ValueTrue {
				opts = append(opts, types.WithDebugMode(true))
			}
			// If the message metadata contains _skipTellNext parameters, only the current node is executed, not propagated downward
			if skipVal := inMsg.Metadata.GetValue(types.KeySkipTellNext); skipVal == types.ValueTrue {
				opts = append(opts, types.WithSkipTellNext())
			}

			if toFlow.IsWait() {
				//Synchronized
				ruleEngine.OnMsgAndWait(*inMsg, opts...)
			} else {
				//Asynchronous
				ruleEngine.OnMsg(*inMsg, opts...)
			}
		} else {
			//Error returned when the rule chain was not found
			for _, process := range toFlow.GetProcessList() {
				exchange.Out.SetError(fmt.Errorf("chainId=%s not found error", toChainId))
				if !process(router, exchange) {
					break
				}
			}
		}

	}
}

// ComponentExecutor is an executor implementation that routes messages directly to individual components.
// It provides a way to execute single node components without the overhead of a full rule chain,
// suitable for simple processing scenarios or component testing.
//
// ComponentExecutor is an executor that routes messages directly to a single component.
// It provides a way to execute a single node component without the overhead of a complete rule chain, making it suitable for simple scenarios or component testing.
//
// Architecture
// • Direct Execution: Bypasses rule chain infrastructure for performance
// • Component Integration: Works with any registered component type
// • Context Management: Creates minimal rule context for component execution
// • Synchronous Support: Optional blocking execution for responses
//
// Use Cases
// • Simple transformations without complex routing
// • Component testing and validation
// • High-performance single-step processing
// • Microservice-style component execution
//
// Limitations
// • No variable path support (IsPathSupportVar returns false)
// • Single component execution only
// • Limited rule context features compared to full chains
type ComponentExecutor struct {
	//Component instance to execute
	component types.Node
	//Rule engine configuration
	config types.Config
}

func (ce *ComponentExecutor) New() endpoint.Executor {
	return &ComponentExecutor{}
}

// The IsPathSupportVar to path does not allow variables
func (ce *ComponentExecutor) IsPathSupportVar() bool {
	return false
}

func (ce *ComponentExecutor) Init(config types.Config, configuration types.Configuration) error {
	ce.config = config
	if configuration == nil {
		return fmt.Errorf("nodeType can't empty")
	}
	var nodeType = ""
	if v, ok := configuration[pathKey]; ok {
		nodeType = str.ToString(v)
	}
	node, err := config.ComponentsRegistry.NewNode(nodeType)
	if err == nil {
		ce.component = node
		err = ce.component.Init(config, configuration)
	}
	return err
}

func (ce *ComponentExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	if ce.component != nil {
		fromFlow := router.GetFrom()
		if fromFlow == nil {
			return
		}

		inMsg := exchange.In.GetMsg()
		if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
			//Initialized empty context
			ruleCtx := engine.NewRuleContext(ctx, ce.config, nil, nil, nil, ce.config.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				// Create a ScopedMessage proxy to isolate the message state of each callback
				// Create ScopedMessage proxy to isolate message state for each callback
				scopedOut := &ScopedMessage{
					Message: exchange.Out, // Retain references to the original ResponseMessage (for IO operations)
					msg:     &msg,         // Bind specific data for the current callback
				}

				// Create ScopedExchange
				scopedExchange := &endpoint.Exchange{
					In:      exchange.In,
					Out:     scopedOut,
					Context: exchange.Context,
				}

				if err != nil {
					scopedExchange.Out.SetError(err)
				}

				for _, process := range toFlow.GetProcessList() {
					if !process(router, scopedExchange) {
						break
					}
				}
			}, engine.DefaultPool)

			if toFlow.IsWait() {
				c := make(chan struct{})
				ruleCtx.SetOnAllNodeCompleted(func() {
					close(c)
				})
				//Execute component logic
				ce.component.OnMsg(ruleCtx, *inMsg)
				//Wait for the execution to finish
				<-c
			} else {
				//Execute component logic
				ce.component.OnMsg(ruleCtx, *inMsg)
			}
		}
	}
}

// DefaultExecutorFactory is the global factory instance for To endpoint executors.
// It provides a centralized registry for all executor types used in the endpoint system.
// The factory is pre-configured with built-in executor types during package initialization.
//
// DefaultExecutorFactory is the global factory instance of the To Endpoint executor.
// It provides a centralized registry for all executor types used in endpoint systems.
// The factory pre-configures the built-in actuator type during package initialization.
//
// Built-in Executors
// • "chain": Routes messages to rule chains with full rule engine features
// • "component": Routes messages to individual components for direct processing
//
// Extension
// Custom executor types can be registered using DefaultExecutorFactory.Register()
// You can register custom executor types using DefaultExecutorFactory.Register().
var DefaultExecutorFactory = new(ExecutorFactory)

// init registers the default executor types with the DefaultExecutorFactory.
// This initialization ensures that the basic executor types are available
// for use throughout the endpoint system.
//
// init registers the default executor type with DefaultExecutorFactory.
// This initialization ensures that the basic actuator type is available throughout the endpoint system.
//
// Registered Types
// • "chain": ChainExecutor for rule chain integration
// • "component": ComponentExecutor for direct component execution
func init() {
	DefaultExecutorFactory.Register("chain", &ChainExecutor{})
	DefaultExecutorFactory.Register("component", &ComponentExecutor{})
}

// ScopedMessage is a proxy wrapper for endpoint.Message that provides scope-specific RuleMsg.
// It intercepts GetMsg/SetMsg calls to use a local RuleMsg, while passing through
// other calls (like SetBody, Headers) to the underlying message.
//
// ScopedMessage is an endpoint.Message proxy wrapper that provides scope-specific RuleMsg.
// It intercepts GetMsg/SetMsg calls to use the local RuleMsg, while transmitting other calls (such as SetBody, Headers) to the underlying messages.
type ScopedMessage struct {
	endpoint.Message                // Embed original interface
	msg              *types.RuleMsg // Scope-specific RuleMsg
	err              error          // Scope-specific error
}

// GetMsg returns the scope-specific RuleMsg.
func (sm *ScopedMessage) GetMsg() *types.RuleMsg {
	return sm.msg
}

// SetMsg sets the scope-specific RuleMsg.
func (sm *ScopedMessage) SetMsg(msg *types.RuleMsg) {
	sm.msg = msg
}

// Pass-through methods ensuring underlying IO operations work correctly

func (sm *ScopedMessage) Body() []byte {
	return sm.Message.Body()
}

func (sm *ScopedMessage) Headers() textproto.MIMEHeader {
	return sm.Message.Headers()
}

func (sm *ScopedMessage) From() string {
	return sm.Message.From()
}

func (sm *ScopedMessage) GetParam(key string) string {
	return sm.Message.GetParam(key)
}

func (sm *ScopedMessage) SetStatusCode(statusCode int) {
	sm.Message.SetStatusCode(statusCode)
}

func (sm *ScopedMessage) SetBody(body []byte) {
	sm.Message.SetBody(body)
}

func (sm *ScopedMessage) SetError(err error) {
	sm.err = err
}

func (sm *ScopedMessage) GetError() error {
	return sm.err
}

// AddHeader delegates header append operations to the underlying message when supported.
func (sm *ScopedMessage) AddHeader(key, value string) {
	if modifier, ok := sm.Message.(endpoint.HeaderModifier); ok {
		modifier.AddHeader(key, value)
		return
	}
	sm.Message.Headers().Add(key, value)
}

// SetHeader delegates header replacement operations to the underlying message when supported.
func (sm *ScopedMessage) SetHeader(key, value string) {
	if modifier, ok := sm.Message.(endpoint.HeaderModifier); ok {
		modifier.SetHeader(key, value)
		return
	}
	sm.Message.Headers().Set(key, value)
}

// DelHeader delegates header deletion operations to the underlying message when supported.
func (sm *ScopedMessage) DelHeader(key string) {
	if modifier, ok := sm.Message.(endpoint.HeaderModifier); ok {
		modifier.DelHeader(key)
		return
	}
	sm.Message.Headers().Del(key)
}

// GetMetadata returns the underlying message metadata when header mutation is supported.
func (sm *ScopedMessage) GetMetadata() *types.Metadata {
	if modifier, ok := sm.Message.(endpoint.HeaderModifier); ok {
		return modifier.GetMetadata()
	}
	return nil
}

// Response returns the underlying http.ResponseWriter if available.
// This is used for HTTP-based endpoints to Returns nil if not available.
//
// Response: Returns the underlying http.ResponseWriter (if available).
// Used for HTTP-based endpoints. If unavailable, return nil.
func (sm *ScopedMessage) Response() http.ResponseWriter {
	// Try to get the Response() method from the underlying message
	if resp, ok := sm.Message.(interface{ Response() http.ResponseWriter }); ok {
		return resp.Response()
	}
	return nil
}

// Flush sends any buffered data to the client.
// It attempts to call Flush on the underlying Message if it implements Flusher interface.
//
// Flush sends buffered data to the client.
// It attempts to call the Flush method of the underlying message (if the Flusher interface is implemented).
func (sm *ScopedMessage) Flush() {
	if flusher, ok := sm.Message.(interface{ Flush() }); ok {
		flusher.Flush()
	}
}
