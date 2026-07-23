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

// Package endpoint provides the core definitions and interfaces for endpoints in the RuleGo framework.
// Endpoints serve as entry points for external data to flow into rule chains, abstracting different
// input sources and providing a unified interface for message processing and routing.
//
// Package endpoint provides core definitions and interfaces for endpoints in the RuleGo framework.
// Endpoints serve as entry points for external data flowing into the rule chain, abstracting different input sources and providing unified message processing and routing interfaces.
//
// # Core Concepts
//
// • Endpoint: The main abstraction for input sources (HTTP, MQTT, WebSocket, etc.)
// • Router: Defines how incoming messages are routed to rule chains
// • Message: Abstracts incoming and outgoing message data
// • Exchange: Contains both request and response messages
// • Process: Middleware-style processing functions
//
// # Architecture
//
// The endpoint system follows a layered architecture:
// Endpoint systems follow a hierarchical architecture:
//
// 1. Message Layer: Abstracts incoming/outgoing data
// 2. Processing Layer: Transforms and validates messages
// 3. Routing Layer: Routes messages to appropriate destinations
// 4. Execution Layer: Executes rule chains or components
//
// # Endpoint Lifecycle
//
// 1. Creation: Endpoint is created and configured
// 2. Initialization: Resources are allocated and connections established
// 3. Start: Endpoint begins accepting incoming messages
// 4. Processing: Messages are processed through routers
// 5. Shutdown: Resources are cleaned up gracefully
//
// # Usage Patterns
//
// Static Configuration
//
//	endpoint := &rest.Rest{}
//	endpoint.Init(config, restConfig)
//	router := endpoint.NewRouter().From("/api/data").To("chain:processing")
//	endpoint.POST(router)
//	endpoint.Start()
//
// Dynamic Configuration
//
//	factory := endpoint.NewFactory()
//	dynamicEndpoint, err := factory.NewFromDsl(dslBytes)
//	dynamicEndpoint.Start()
//
// # Message Processing Pipeline
//
// The message processing follows this flow:
// Message processing follows this process:
//
// 1. External Message → Endpoint
// 2. Message → RequestMessage
// 3. Router Matching & Processing
// 4. Rule Chain/Component Execution
// 5. Response Generation → ResponseMessage
// 6. ResponseMessage → External Response
package endpoint

import (
	"context"
	"net/http"
	"net/textproto"
	"sync"

	"github.com/rulego/rulego/api/types"
)

// Event constants define various lifecycle and operational events in the endpoint system.
// These events enable monitoring and handling of endpoint state changes and operations.
// Event constants define various lifecycle and operational events in the endpoint system.
// These events make it possible to monitor and handle endpoint state changes and operations.
const (
	// EventConnect represents a connection establishment event.
	// Triggered when a new client connection is established (e.g., WebSocket connection).
	// EventConnect represents the connection establishment event.
	// Triggered when a new client connection is established (for example, a WebSocket connection).
	EventConnect = "Connect"

	// EventDisconnect represents a connection termination event.
	// Triggered when a client connection is closed or lost.
	// EventDisconnect indicates a connection termination event.
	// Triggered when a client connection is closed or lost.
	EventDisconnect = "Disconnect"

	// EventInitServer represents a server initialization event.
	// Triggered when the endpoint server is being initialized.
	// EventInitServer represents the server initialization event.
	// Triggered when the endpoint server is initializing.
	EventInitServer = "InitServer"

	// EventCompletedServer represents a server completion event.
	// Triggered when the endpoint server has completed its operations.
	// EventCompletedServer indicates the server completion event.
	// Triggered when the endpoint server completes its operation.
	EventCompletedServer = "completedServer"

	// EventRestart represents a server restart event.
	// Triggered when the endpoint server is being restarted.
	// EventRestart indicates a server restart event.
	// Triggered when the endpoint server is rebooting.
	EventRestart = "Restart"
)

// OnEvent is a callback function type for handling endpoint events.
// It provides a flexible way to respond to various endpoint lifecycle and operational events.
//
// OnEvent is a type of callback function used to handle endpoint events.
// It offers flexible ways to respond to various endpoint lifecycles and operational events.
//
// Parameters
// • eventName: The name of the event being triggered
// • params: Variable number of parameters specific to the event type
//
// Usage Examples
//
//	endpoint.SetOnEvent(func(eventName string, params ...interface{}) {
//	    switch eventName {
//	    case endpoint.EventConnect:
//	        log.Printf("Client connected: %v", params[0])
//	    case endpoint.EventDisconnect:
//	        log.Printf("Client disconnected: %v", params[0])
//	    }
//	})
type OnEvent func(eventName string, params ...interface{})

// Endpoint defines the core interface for all endpoint implementations in the RuleGo framework.
// It provides the fundamental operations needed for message input, routing, and lifecycle management.
//
// Endpoint defines the core interface for all endpoint implementations in the RuleGo framework.
// It provides the basic operations needed for message input, routing, and lifecycle management.
//
// # Key Responsibilities
//
// • Message Input: Accept messages from external sources
// • Router Management: Add, remove, and configure message routers
// • Lifecycle Control: Start, stop, and manage endpoint lifecycle
// • Event Handling: Process and notify about endpoint events
// • Interceptor Support: Apply cross-cutting concerns through interceptors
//
// # Implementation Guidelines
//
// All endpoint implementations should:
// All endpoint implementations should:
//
// • Be thread-safe for concurrent operations
// • Support graceful shutdown procedures
// • Handle connection failures and recovery
// • Provide meaningful error messages
// • Support dynamic router configuration
type Endpoint interface {
	// Node interface provides basic component functionality including initialization,
	// type identification, and lifecycle management.
	// The Node interface provides basic component functions, including initialization, type recognition, and lifecycle management.
	types.Node

	// Id returns a unique identifier for the endpoint instance.
	// This ID is used for endpoint registration, lookup, and management operations.
	//
	// Id returns a unique identifier for the endpoint instance.
	// This ID is used for endpoint registration, search, and management operations.
	//
	// Returns
	// • string: Unique endpoint identifier
	Id() string

	// SetOnEvent registers an event listener function for the endpoint.
	// The listener will be called for various endpoint lifecycle and operational events.
	//
	// SetOnEvent is the endpoint to register the event listener function.
	// Listeners will be called for various endpoint lifecycles and operational events.
	//
	// Parameters
	// • onEvent: Event listener function
	SetOnEvent(onEvent OnEvent)

	// Start initiates the endpoint service and begins accepting incoming messages.
	// This method should establish necessary connections, bind to ports, and prepare
	// the endpoint for message processing.
	//
	// Start the endpoint service and begin accepting incoming messages.
	// This method should establish the necessary connections, bind them to ports, and prepare endpoints for message processing.
	//
	// Returns
	// • error: Error if startup fails, nil on success
	//
	// Behavior
	// • Should be idempotent (safe to call multiple times)
	// • Should not block the calling goroutine
	// • Should handle resource allocation and connection establishment
	Start() error

	// AddInterceptors adds global interceptors to the endpoint processing pipeline.
	// Interceptors are executed in the order they are added for all incoming messages.
	//
	// AddInterceptors: Adds a global interceptor to the endpoint processing pipe.
	// Interceptors execute all incoming messages in the order they are added.
	//
	// Parameters
	// • interceptors: Processing functions to add to the pipeline
	//
	// Usage
	// Interceptors can be used for cross-cutting concerns such as:
	// Interceptors can be used for cross-cutting points of concern, such as:
	// • Authentication and authorization
	// • Logging and monitoring
	// • Rate limiting and throttling
	// • Message transformation and validation
	AddInterceptors(interceptors ...Process)

	// AddRouter adds a message router to the endpoint with optional configuration parameters.
	// Routers define how incoming messages are matched and routed to rule chains or components.
	//
	// AddRouter adds a message router to the endpoint with optional configuration parameters.
	// Routers define how incoming messages match and route to the rule chain or component.
	//
	// Parameters
	// • router: Router configuration defining message routing logic
	// • params: Protocol-specific parameters (e.g., HTTP methods)
	//
	// Returns
	// • string: Router ID for future reference and management
	// • error: Error if router addition fails
	//
	// Note
	// Some endpoints may return a modified router ID that should be used for future operations.
	// Some endpoints may return the modified router ID, which will be applied for future operations.
	AddRouter(router Router, params ...interface{}) (string, error)

	// RemoveRouter removes a message router from the endpoint by its ID.
	// This operation immediately stops routing messages to the specified router.
	//
	// RemoveRouter deletes message routers from endpoints by ID.
	// This operation immediately stops routing messages to the specified router.
	//
	// Parameters
	// • routerId: ID of the router to remove
	// • params: Protocol-specific parameters for removal
	//
	// Returns
	// • error: Error if router removal fails or router not found
	//
	// Behavior
	// • Should gracefully handle in-flight messages
	// • Should clean up any router-specific resources
	RemoveRouter(routerId string, params ...interface{}) error
}

// DynamicEndpoint extends the basic Endpoint interface with dynamic configuration capabilities.
// It allows endpoints to be created, modified, and reloaded at runtime using DSL configurations.
//
// DynamicEndpoint extends the basic Endpoint interface with dynamic configuration capabilities.
// It allows endpoints to be created, modified, and reloaded at runtime using DSL configurations.
//
// # Key Features
//
// • Runtime Configuration: Modify endpoint behavior without restart
// • DSL Integration: Use declarative configuration for complex setups
// • Hot Reloading: Update routing rules and configurations dynamically
// • Template Support: Support for variable substitution in configurations
//
// # Use Cases
//
// • Configuration Management Systems
// • Multi-tenant Applications
// • Dynamic API Gateways
// • Development and Testing Environments
type DynamicEndpoint interface {
	// Endpoint provides all basic endpoint functionality
	// Endpoint provides all basic endpoint functions
	Endpoint

	// SetId sets the unique identifier for the dynamic endpoint.
	// This method is typically called during endpoint initialization.
	//
	// SetId sets the unique identifier for dynamic endpoints.
	// This method is usually called during endpoint initialization.
	//
	// Parameters
	// • id: Unique identifier for the endpoint
	SetId(id string)

	// SetConfig sets the rule engine configuration for the dynamic endpoint.
	// This configuration affects how the endpoint interacts with rule chains.
	//
	// SetConfig sets the rule engine configuration for dynamic endpoints.
	// This configuration affects how endpoints interact with the rule chain.
	//
	// Parameters
	// • config: Rule engine configuration
	SetConfig(config types.Config)

	// SetRouterOptions sets default options that will be applied to all routers.
	// These options provide common configuration for router behavior.
	//
	// The SetRouterOptions setting will be applied to the default options for all routers.
	// These options provide a universal configuration for router behavior.
	//
	// Parameters
	// • opts: Router configuration options
	SetRouterOptions(opts ...RouterOption)

	// SetRestart sets whether the endpoint should restart when configuration changes.
	// When true, the endpoint will be fully restarted; when false, only routing is updated.
	//
	// SetRestart sets whether the endpoint should restart when the configuration changes.
	// If set to true, the endpoint will fully reboot; If false, only the route is updated.
	//
	// Parameters
	// • restart: Whether to restart on configuration changes
	SetRestart(restart bool)

	// SetInterceptors sets the global interceptors for the dynamic endpoint.
	// This replaces any existing interceptors with the provided ones.
	//
	// SetInterceptors sets global interceptors for dynamic endpoints.
	// This will replace any existing interceptors with the provided interceptors.
	//
	// Parameters
	// • interceptors: Processing functions to set as global interceptors
	SetInterceptors(interceptors ...Process)

	// Reload reloads the dynamic endpoint with a new DSL configuration.
	// The behavior depends on the restart setting and configuration changes.
	//
	// Reload reloads dynamic endpoints using the new DSL configuration.
	// Behavior depends on restarting settings and configuration changes.
	//
	// Parameters
	// • dsl: JSON byte array containing the new endpoint configuration
	// • opts: Additional options for the reload operation
	//
	// Returns
	// • error: Error if reload fails
	//
	// Behavior
	// • If restart=true: Full endpoint restart with new configuration
	// • If restart=false: Only update routing without service interruption
	// • Routing conflicts may force a restart regardless of setting
	Reload(dsl []byte, opts ...DynamicEndpointOption) error

	// ReloadFromDef reloads the dynamic endpoint with a new DSL definition structure.
	// This is an alternative to Reload() that accepts a structured configuration.
	//
	// ReloadFromDef uses a new DSL definition structure to reload dynamic endpoints.
	// This is an alternative to Reload(), accepting structured configurations.
	//
	// Parameters
	// • def: Structured endpoint DSL definition
	// • opts: Additional options for the reload operation
	//
	// Returns
	// • error: Error if reload fails
	ReloadFromDef(def types.EndpointDsl, opts ...DynamicEndpointOption) error

	// AddOrReloadRouter adds a new router or reloads an existing one with new configuration.
	// This method provides fine-grained control over individual router updates.
	//
	// AddOrReloadRouter: Add a new router or reload an existing router with a new configuration.
	// This method provides fine-grained control over updates to individual routers.
	//
	// Parameters
	// • dsl: JSON byte array containing the router configuration
	// • opts: Additional options for the operation
	//
	// Returns
	// • error: Error if operation fails
	AddOrReloadRouter(dsl []byte, opts ...DynamicEndpointOption) error

	// Definition returns the current DSL definition of the dynamic endpoint.
	// This provides access to the endpoint's configuration structure.
	//
	// Definition Returns the current DSL definition of the dynamic endpoint.
	// This provides access to the endpoint configuration structure.
	//
	// Returns
	// • types.EndpointDsl: Current endpoint DSL definition
	Definition() types.EndpointDsl

	// DSL returns the current DSL configuration as a JSON byte array.
	// This is useful for serialization, storage, or external processing.
	//
	// DSL returns the current DSL configuration as a JSON byte array.
	// This is useful for serialization, storage, or external processing.
	//
	// Returns
	// • []byte: JSON representation of the current configuration
	DSL() []byte

	// Target returns the underlying concrete endpoint implementation.
	// This allows access to protocol-specific functionality when needed.
	//
	// Target: Returns the specific endpoint implementation at the underlying level.
	// This allows access to protocol-specific features when needed.
	//
	// Returns
	// • Endpoint: The underlying endpoint implementation
	Target() Endpoint

	// SetRuleChain stores the original rule chain DSL definition when the endpoint
	// is initialized from a rule chain configuration.
	//
	// SetRuleChain When the endpoint is initialized from the rule chain configuration, the original rule chain DSL definition is stored.
	//
	// Parameters
	// • ruleChain: Original rule chain DSL definition
	SetRuleChain(ruleChain *types.RuleChain)

	// GetRuleChain retrieves the original rule chain DSL definition that was used
	// to initialize this endpoint, if any.
	//
	// GetRuleChain retrieves the original rule chain DSL definition used to initialize this endpoint (if any).
	//
	// Returns
	// • *types.RuleChain: Original rule chain DSL, nil if not initialized from rule chain
	GetRuleChain() *types.RuleChain
}

// Message defines the abstraction for data received at an endpoint.
// It provides a unified interface for accessing message content, headers, parameters,
// and converting messages to the RuleGo processing format.
//
// Message defines the abstraction of data received at the endpoint.
// It provides a unified interface for accessing message content, headers, parameters, and converting messages into RuleGo processing formats.
//
// # Key Concepts
//
// • Protocol Abstraction: Provides common interface across different protocols
// • Data Access: Uniform access to message body, headers, and parameters
// • Format Conversion: Seamless conversion to RuleGo message format
// • Error Handling: Built-in error tracking and status management
//
// # Implementation Notes
//
// • Message implementations should be thread-safe where possible
// • Body() should support lazy loading for performance optimization
// • GetMsg() should cache converted messages to avoid repeated conversion
type Message interface {
	// Body returns the raw message body as a byte slice.
	// Implementations may use lazy loading for performance optimization.
	//
	// Body returns the original message body as a byte slice.
	// Performance optimization may be implemented using delayed loading.
	//
	// Returns
	// • []byte: Raw message body content
	Body() []byte

	// Headers returns the message headers in a standardized format.
	// This provides access to protocol-specific metadata and properties.
	//
	// Headers return message headers in a standardized format.
	// This provides access to protocol-specific metadata and attributes.
	//
	// Returns
	// • textproto.MIMEHeader: Standardized header format
	Headers() textproto.MIMEHeader

	// From returns the origin identifier of the message.
	// The format depends on the protocol (URL for HTTP, topic for MQTT, etc.).
	//
	// From returns the source identifier of the message.
	// The format depends on the protocol (HTTP URL, MQTT topic, etc.).
	//
	// Returns
	// • string: Origin identifier specific to the protocol
	From() string

	// GetParam retrieves a parameter value by key.
	// This may include query parameters, path parameters, or protocol-specific data.
	//
	// GetParam retrieves parameter values through keys.
	// This may include query parameters, path parameters, or protocol-specific data.
	//
	// Parameters
	// • key: Parameter name to retrieve
	//
	// Returns
	// • string: Parameter value, empty string if not found
	GetParam(key string) string

	// SetMsg sets the converted RuleMsg for this message.
	// This is typically used for caching the conversion result.
	//
	// SetMsg sets the converted RuleMsg for this message.
	// This is usually used to cache the conversion results.
	//
	// Parameters
	// • msg: Converted RuleMsg to associate with this message
	SetMsg(msg *types.RuleMsg)

	// GetMsg converts the message to RuleGo's internal message format.
	// This method should handle data type detection and metadata population.
	//
	// GetMsg converts messages into RuleGo's internal message format.
	// This method should handle data type detection and metadata filling.
	//
	// Returns
	// • *types.RuleMsg: Converted message ready for rule chain processing
	//
	// Conversion Behavior
	// • Should detect appropriate data type (JSON, TEXT, BINARY)
	// • Should populate metadata with relevant protocol information
	// • Should cache the result for subsequent calls
	GetMsg() *types.RuleMsg

	// SetStatusCode sets the response status code for protocols that support it.
	// For protocols without status codes (e.g., MQTT), this may be a no-op.
	//
	// SetStatusCode sets the response status code for the supporting protocol.
	// For protocols without status codes (e.g., MQTT), this may be an operationless procedure.
	//
	// Parameters
	// • statusCode: Protocol-specific status code
	SetStatusCode(statusCode int)

	// SetBody sets or modifies the message body content.
	// This is typically used for response messages or message transformation.
	//
	// SetBody sets or modifies the message body content.
	// This is usually used in response to messages or message transformation.
	//
	// Parameters
	// • body: New message body content
	SetBody(body []byte)

	// SetError associates an error with this message.
	// This is used for error tracking and debugging purposes.
	//
	// SetError associates the error with this message.
	// This is used for error tracking and debugging purposes.
	//
	// Parameters
	// • err: Error to associate with the message
	SetError(err error)

	// GetError retrieves any error associated with this message.
	// This is useful for error handling and debugging.
	//
	// GetError retrieves any errors associated with this message.
	// This is useful for error handling and debugging.
	//
	// Returns
	// • error: Associated error, nil if no error
	GetError() error
}

// Exchange represents a complete message exchange containing both request and response.
// It provides the context for processing a single message interaction through the endpoint system.
//
// Exchange represents a complete message exchange containing requests and responses.
// It provides context for handling individual message interactions through endpoint systems.
//
// # Architecture
//
// Exchange follows the request-response pattern common in many protocols:
// Exchange follows the request-response pattern common in many protocols:
//
// 1. Request Processing: In message is processed through the pipeline
// 2. Business Logic: Rule chains or components execute business logic
// 3. Response Generation: Results are populated in the Out message
// 4. Protocol Handling: Protocol-specific response is sent back
//
// # Thread Safety
//
// Exchange includes RWMutex for protecting concurrent access to its fields.
// Exchange includes RWMutex to protect concurrent access to its fields.
//
// Usage Pattern
//
//	exchange := &endpoint.Exchange{
//	    In:  requestMessage,
//	    Out: responseMessage,
//	    Context: context.Background(),
//	}
//	// Process through pipeline
//	router.Execute(exchange)
type Exchange struct {
	// In represents the incoming request message.
	// This contains the original data received from the external source.
	// In indicates the incoming request message.
	// This includes raw data received from external sources.
	In Message

	// Out represents the outgoing response message.
	// This will be populated with the processing results and sent back.
	// Out indicates outgoing response messages.
	// This will fill the processing results and send them back.
	Out Message

	// Context provides additional context for the exchange operation.
	// This can include timeouts, cancellation, and request-scoped values.
	// Context provides additional context for switching operations.
	// This can include timeouts, cancellations, and requests for range values.
	Context context.Context

	// RWMutex protects concurrent access to Exchange fields.
	// This ensures thread-safe operations when multiple goroutines access the exchange.
	// RWMutex protects concurrent access to Exchange fields.
	// This ensures thread-safe operation when multiple coroutines access exchange.
	sync.RWMutex
}

// From defines the interface for message source configuration in routing operations.
// It represents the input side of a routing rule, defining where messages originate
// and how they should be processed before being sent to their destination.
//
// From Defines the interface configured for message sources in routing operations.
// It represents the input to routing rules, defines the source of the message, and how it should be handled before being sent to the target.
//
// # Key Features
//
// • Source Definition: Specifies the origin pattern for incoming messages
// • Processing Pipeline: Supports transformation and processing functions
// • Flexible Routing: Supports various destination types and configurations
// • Fluent API: Provides a fluent interface for configuration chaining
//
// # Processing Pipeline
//
// The From interface supports a processing pipeline that can:
// The From interface supports a processing pipeline that can:
//
// 1. Transform message content and format
// 2. Validate message data and structure
// 3. Apply security and authentication checks
// 4. Route to appropriate destinations
type From interface {
	// ToString returns a string representation of the source configuration.
	// This is typically the pattern or path used to match incoming messages.
	//
	// ToString returns the string representation of the source configuration.
	// This is usually used to match the pattern or path of the incoming message.
	//
	// Returns
	// • string: Source pattern or path
	ToString() string

	// Transform adds a transformation process to the source processing pipeline.
	// Transformations modify message content, format, or structure.
	//
	// Transform: adds a conversion process to the source processing pipeline.
	// Modify message content, format, or structure.
	//
	// Parameters
	// • transform: Processing function that modifies the message
	//
	// Returns
	// • From: The same From instance for method chaining
	//
	// Usage
	//	from.Transform(func(router Router, exchange *Exchange) bool {
	//	    // Modify exchange.In or exchange.Out
	//	    return true // Continue processing
	//	})
	Transform(transform Process) From

	// Process adds a general processing function to the source pipeline.
	// Processors can perform validation, filtering, or other operations.
	//
	// Process: Adds a general handler function to the source pipeline.
	// The processor can perform verification, filtering, or other operations.
	//
	// Parameters
	// • process: Processing function to execute
	//
	// Returns
	// • From: The same From instance for method chaining
	//
	// Processing Control
	// If the process function returns false, subsequent processing is interrupted.
	// If the handler returns false, subsequent processing will be interrupted.
	Process(process Process) From

	// GetProcessList returns all processing functions configured for this source.
	// This is useful for introspection and debugging.
	//
	// GetProcessList returns all handlers configured for this source.
	// This is useful for introspection and debugging.
	//
	// Returns
	// • []Process: List of configured processing functions
	GetProcessList() []Process

	// ExecuteProcess executes all configured processing functions in order.
	// This is called by the endpoint when a matching message is received.
	//
	// ExecuteProcess executes all configured handlers in order.
	// When a matching message is received, the endpoint calls this method.
	//
	// Parameters
	// • router: The router containing this source configuration
	// • exchange: The message exchange to process
	//
	// Returns
	// • bool: true to continue processing, false to interrupt
	//
	// Execution Flow
	// If any processor returns false, execution stops and subsequent operations are interrupted.
	// If any processor returns false, execution stops and subsequent operations are interrupted.
	ExecuteProcess(router Router, exchange *Exchange) bool

	// To defines the destination for messages from this source.
	// This creates a routing rule that connects the source to a destination.
	//
	// To define the target of messages from this source.
	// This creates routing rules that connect the source to the target.
	//
	// Parameters
	// • to: Destination path (e.g., "chain:chainId", "component:nodeType")
	// • configs: Optional configuration for the destination
	//
	// Returns
	// • To: Destination configuration interface
	//
	// Destination Formats
	// • "chainId" - Route to a specific rule chain (chain: prefix optional, default processor)
	// • "chain:chainId" - Route to a specific rule chain (explicit prefix)
	// • "chainId:nodeId" - Route to a specific node within a rule chain
	// • "chain:chainId:nodeId" - Route to a specific node within a rule chain (explicit prefix)
	// • "component:nodeType" - Route to a component type
	// • Variable substitution is supported using ${variable} syntax
	//
	// Examples
	// • To("userProcessing") - Process through entire user processing chain (default)
	// • To("chain:userProcessing") - Process through entire user processing chain (explicit)
	// • To("userProcessing:validateInput") - Start from validateInput node
	// • To("chain:userProcessing:validateInput") - Start from validateInput node (explicit)
	// • To("component:jsTransform") - Execute JavaScript transform component
	To(to string, configs ...types.Configuration) To

	// GetTo retrieves the current destination configuration.
	// Returns nil if no destination has been configured.
	//
	// GetTo retrieves the current target configuration.
	// If no target is configured, nil is returned.
	//
	// Returns
	// • To: Current destination configuration, nil if not set
	GetTo() To

	// ToComponent sets the destination to be executed by a specific component instance.
	// This allows direct routing to a pre-configured component.
	//
	// ToComponent settings are executed by specific component instances.
	// This allows direct routing to preconfigured components.
	//
	// Parameters
	// • node: Component instance to execute
	//
	// Returns
	// • To: Destination configuration interface
	ToComponent(node types.Node) To

	// GetConfiguration returns the configuration for this source.
	// Processors can use this method to access configuration values.
	//
	// GetConfiguration returns the configuration of this source.
	// The processor can access configuration values using this method.
	//
	// Returns
	// • Configuration: Source configuration map
	GetConfiguration() types.Configuration

	// End finalizes the source configuration and returns the complete router.
	// This method completes the fluent configuration chain.
	//
	// End completes the source configuration and returns the complete router.
	// This method completes a smooth configuration chain.
	//
	// Returns
	// • Router: Complete router configuration
	End() Router
}

// To defines the interface for message destination configuration in routing operations.
// It represents the output side of a routing rule, defining where processed messages
// are sent and how they should be handled during execution.
//
// To Define the interface configured for message targets in routing operations.
// It represents the output of routing rules, defines where processed messages are sent, and how they should be handled during execution.
//
// # Key Features
//
// • Destination Execution: Controls how and where messages are processed
// • Processing Pipeline: Supports post-processing and transformation
// • Execution Control: Supports synchronous and asynchronous execution modes
// • Variable Substitution: Supports dynamic destination paths with variables
//
// # Execution Modes
//
// • Asynchronous (default): Messages are processed in background goroutines
// • Synchronous (with Wait()): Execution waits for completion before returning
//
// # Destination Types
//
// • Rule Chains: "chainId" or "chain:chainId" - Execute a rule chain (chain: prefix optional)
// • Chain Nodes: "chainId:nodeId" or "chain:chainId:nodeId" - Execute from a specific node
// • Components: "component:nodeType" - Execute a component type
// • Direct Components: Pre-configured component instances
//
// Chain Node Routing Examples
// • "deviceProcessing" - Execute entire deviceProcessing chain (default)
// • "chain:deviceProcessing" - Execute entire deviceProcessing chain (explicit)
// • "deviceProcessing:filterNode" - Start from filterNode in deviceProcessing chain
// • "chain:deviceProcessing:filterNode" - Start from filterNode (explicit prefix)
// • "${tenant}:validateData" - Dynamic chain with specific node
// • "alertChain:notificationNode" - Route to notification node in alert chain
type To interface {
	// ToString returns a string representation of the destination configuration.
	// This typically includes the destination path and any configured parameters.
	//
	// ToString returns the string representation of the target configuration.
	// This usually includes the target path and any configured parameters.
	//
	// Returns
	// • string: Destination configuration string
	ToString() string

	// Execute performs the actual routing operation for the given exchange.
	// This is the core method that processes messages according to the destination configuration.
	//
	// Execute performs the actual routing operation on a given switch.
	// This is the core method for processing messages based on target configuration.
	//
	// Parameters
	// • ctx: Context for the execution operation
	// • exchange: Message exchange containing request and response
	//
	// Execution Behavior
	// • Synchronous execution if Wait() was called
	// • Asynchronous execution by default
	// • Processing pipeline is applied before destination execution
	Execute(ctx context.Context, exchange *Exchange)

	// Transform adds a transformation process to the destination processing pipeline.
	// Transformations are applied after the destination execution completes.
	//
	// Transform adds a conversion process to the target processing pipeline.
	// Transformations are applied after the goal is completed.
	//
	// Parameters
	// • transform: Processing function that modifies the response
	//
	// Returns
	// • To: The same To instance for method chaining
	//
	// Usage
	//	to.Transform(func(router Router, exchange *Exchange) bool {
	//	    // Modify exchange.Out after rule chain execution
	//	    return true
	//	})
	Transform(transform Process) To

	// Process adds a general processing function to the destination pipeline.
	// Processors can perform validation, logging, or other operations.
	//
	// Process Adds a general handler function to the target pipe.
	// The processor can perform verification, logging, or other operations.
	//
	// Parameters
	// • process: Processing function to execute
	//
	// Returns
	// • To: The same To instance for method chaining
	//
	// Processing Order
	// Processors are executed in the order they are added.
	// Processors execute in the order they are added.
	Process(process Process) To

	// Wait configures the destination to use synchronous execution mode.
	// When Wait() is called, Execute() will block until processing is complete.
	//
	// Wait configuration targets use synchronous execution mode.
	// When Wait() is called, Execute() will block until processing is complete.
	//
	// Returns
	// • To: The same To instance for method chaining
	//
	// Use Cases
	// • Request-response patterns where response is needed immediately
	// • Validation scenarios where errors must be caught
	// • Testing and debugging scenarios
	Wait() To

	// IsWait checks if the destination is configured for synchronous execution.
	// This can be used to determine the execution mode before calling Execute().
	//
	// IsWait checks whether the target is configured to execute synchronously.
	// This can be used to determine the execution mode before calling Execute().
	//
	// Returns
	// • bool: true if synchronous mode, false if asynchronous. True means synchronous mode, false means asynchronous
	IsWait() bool

	// SetOpts applies rule context options that will be used during execution.
	// These options control various aspects of rule chain execution.
	//
	// SetOpts applies rule context options used during execution.
	// These options control every aspect of the rule chain execution.
	//
	// Parameters
	// • opts: Rule context options to apply
	//
	// Returns
	// • To: The same To instance for method chaining
	//
	// Available Options
	// • Timeout settings
	// • Debug configurations
	// • Custom metadata
	SetOpts(opts ...types.RuleContextOption) To

	// GetOpts returns the currently configured rule context options.
	// This is useful for introspection and debugging.
	//
	// GetOpts returns the current configured rule context options.
	// This is useful for introspection and debugging.
	//
	// Returns
	// • []types.RuleContextOption: List of configured options
	GetOpts() []types.RuleContextOption

	// GetProcessList returns all processing functions configured for this destination.
	// This includes both Transform and Process functions in order.
	//
	// GetProcessList returns all handlers configured for this target.
	// This includes sequential Transform and Process functions.
	//
	// Returns
	// • []Process: List of configured processing functions
	GetProcessList() []Process

	// ToStringByDict returns a string representation with variable substitution.
	// Variables in the destination path are replaced with values from the dictionary.
	//
	// ToStringByDict returns a string representation with variable replacement.
	// Variables in the target path are replaced by values in the dictionary.
	//
	// Parameters
	// • dict: Dictionary for variable substitution
	//
	// Returns
	// • string: Destination path with variables substituted
	//
	// Variable Format
	// Variables are specified using ${variableName} syntax.
	// Variables are specified using the ${variableName} syntax.
	ToStringByDict(dict map[string]string) string

	// End finalizes the destination configuration and returns the complete router.
	// This method completes the fluent configuration chain.
	//
	// End completes the target configuration and returns the complete router.
	// This method completes a smooth configuration chain.
	//
	// Returns
	// • Router: Complete router configuration
	End() Router
}

// Router defines the interface for complete routing configurations.
// A router combines source (From) and destination (To) configurations to create
// a complete message routing rule within an endpoint.
//
// Router defines the interface for complete routing configuration.
// Routers combine source (From) and destination (To) configurations to create complete message routing rules within endpoints.
//
// # Key Responsibilities
//
// • Configuration Management: Maintain complete routing configuration
// • Pattern Matching: Determine if incoming messages match the router
// • Execution Context: Provide context for message processing
// • State Management: Track router state and availability
//
// # Router Lifecycle
//
// 1. Creation: Router is created with source and destination configuration
// 2. Registration: Router is registered with an endpoint
// 3. Activation: Router becomes active and begins processing messages
// 4. Processing: Messages are routed according to configuration
// 5. Deactivation: Router can be disabled or removed
//
// # Error Handling
//
// Routers track configuration and runtime errors through the Err() method.
// The router tracks configuration and runtime errors using the Err() method.
type Router interface {
	// SetId sets the unique identifier for the router.
	// This ID is used for router management and reference within the endpoint.
	//
	// SetId sets the unique identifier for the router.
	// This ID is used for router management and references within the endpoint.
	//
	// Parameters
	// • id: Unique identifier for the router
	//
	// Returns
	// • Router: The same Router instance for method chaining
	SetId(id string) Router

	// GetId retrieves the unique identifier of the router.
	// Returns an empty string if no ID has been set.
	//
	// GetId retrieves the unique identifier of the router.
	// If no ID is set, an empty string is returned.
	//
	// Returns
	// • string: Router identifier, empty if not set
	GetId() string

	// FromToString returns a string representation of the source configuration.
	// This is typically the pattern or path used to match incoming messages.
	//
	// FromToString Returns the string representation configured by the source.
	// This is usually used to match the pattern or path of the incoming message.
	//
	// Returns
	// • string: Source configuration string
	FromToString() string

	// From defines the source configuration for the routing operation.
	// This specifies where messages originate and how they should be matched.
	//
	// From defines the source configuration for routing operations.
	// This specifies the source of the message and how it should be matched.
	//
	// Parameters
	// • from: Source pattern or path (e.g., "/api/*", "topic/+")
	// • configs: Optional configuration for the source
	//
	// Returns
	// • From: Source configuration interface
	//
	// Pattern Formats
	// • HTTP: URL patterns with wildcards "/api/*" HTTP: URL pattern with wildcards
	// • MQTT: Topic patterns with wildcards "device/+/data" MQTT: Topic patterns with wildcards
	// • Custom: Protocol-specific patterns
	From(from string, configs ...types.Configuration) From

	// GetFrom retrieves the current source configuration.
	// Returns nil if no source has been configured.
	//
	// GetFrom retrieves the current source configuration.
	// If there is no configuration source, return nil.
	//
	// Returns
	// • From: Current source configuration, nil if not set
	GetFrom() From

	// GetRuleGo retrieves the rule engine pool associated with this router.
	// The pool is determined based on the exchange context and router configuration.
	//
	// GetRuleGo retrieves the pool of rule engines associated with this router.
	// The pool is determined based on the exchange context and router configuration.
	//
	// Parameters
	// • exchange: Message exchange providing context
	//
	// Returns
	// • types.RuleEnginePool: Rule engine pool for processing
	//
	// Pool Selection
	// • May return different pools based on exchange properties
	// • Supports multi-tenant scenarios with pool isolation
	GetRuleGo(exchange *Exchange) types.RuleEnginePool

	// GetContextFunc retrieves the context function for exchange processing.
	// This function can modify or enhance the context for each exchange.
	//
	// GetContextFunc retrieves the context function used for exchange processing.
	// This function can modify or enhance the context for each exchange.
	//
	// Returns
	// • func: Context modification function, nil if not set
	//
	// Context Enhancement
	// The function can add request-specific values, timeouts, or cancellation.
	// Functions can add specific requests, timeouts, or cancellations.
	GetContextFunc() func(ctx context.Context, exchange *Exchange) context.Context

	// Disable sets the availability state of the router.
	// Disabled routers will not process incoming messages.
	//
	// Disable to set the router's available status.
	// A disabled router will not handle incoming messages.
	//
	// Parameters
	// • disable: true to disable, false to enable true means disabled, false means enabled
	//
	// Returns
	// • Router: The same Router instance for method chaining
	//
	// Use Cases
	// • Maintenance mode
	// • A/B testing scenarios
	// • Gradual rollout of new routes
	Disable(disable bool) Router

	// IsDisable checks the availability state of the router.
	// This can be used to determine if the router will process messages.
	//
	// IsDisable checks the availability status of the router.
	// This can be used to determine whether the router will handle messages.
	//
	// Returns
	// • bool: true if disabled, false if enabled True means disabled, false means enabled
	IsDisable() bool

	// Definition returns the DSL definition of the router if available.
	// This provides access to the original configuration structure.
	//
	// Definition returns the router's DSL definition (if available).
	// This provides access to the original configuration structure.
	//
	// Returns
	// • *types.RouterDsl: Router DSL definition, nil if not set
	//
	// Usage
	// Useful for configuration introspection, debugging, and serialization.
	// It is useful for configuration introspection, debugging, and serialization.
	Definition() *types.RouterDsl

	// SetParams sets protocol-specific parameters for the router.
	// These parameters control protocol-specific behavior and matching.
	//
	// SetParams sets protocol-specific parameters for routers.
	// These parameters control the protocol's specific behavior and matching.
	//
	// Parameters
	// • args: Variable number of protocol-specific arguments
	//
	// Parameter Examples
	// • HTTP: HTTP methods ["GET", "POST"] HTTP:HTTP methods
	// • MQTT: QoS levels, retain flags. MQTT: QoS levels, retain flags
	// • WebSocket: Sub-protocols
	SetParams(args ...interface{})

	// GetParams retrieves the protocol-specific parameters.
	// Returns the parameters set by SetParams().
	//
	// GetParams retrieves protocol-specific parameters.
	// Returns the parameters set by SetParams().
	//
	// Returns
	// • []interface{}: List of protocol-specific parameters
	GetParams() []interface{}

	// Err returns any error associated with the router configuration or operation.
	// This includes initialization errors, configuration errors, and runtime errors.
	//
	// ERR returns any errors associated with router configuration or operation.
	// This includes initialization errors, configuration errors, and runtime errors.
	//
	// Returns
	// • error: Associated error, nil if no error
	//
	// Error Types
	// • Configuration errors during router setup
	// • Pattern compilation errors
	// • Runtime processing errors
	Err() error
}

// Process defines a processing function type for the endpoint pipeline system.
// Process functions implement middleware-style processing that can transform,
// validate, filter, or otherwise modify messages during routing operations.
//
// Process defines the type of processing function for the endpoint pipeline system.
// The Process function implements middleware-style processing, allowing messages to be transformed, validated, filtered, or otherwise modified during routing operations.
//
// # Function Signature
//
// The function receives a Router and Exchange and returns a boolean indicating
// whether processing should continue.
// The function receives Router and Exchange and returns a boolean value indicating whether processing should continue.
//
// # Return Value Behavior
//
// • true: Continue processing to the next processor or destination true: Continue processing to the next processor or destination
// • false: Stop processing pipeline immediately false: Stop processing pipeline immediately
//
// # Common Use Cases
//
// • Authentication and Authorization
// • Message Validation and Transformation
// • Logging and Monitoring
// • Rate Limiting and Throttling
// • Error Handling and Recovery
//
// # Processing Context
//
// • Router: Provides access to routing configuration and context
// • Exchange: Contains request/response messages and processing context
//
// Example Implementations
//
//	// Authentication processor
//	func authProcessor(router Router, exchange *Exchange) bool {
//	    token := exchange.In.Headers().Get("Authorization")
//	    if !isValidToken(token) {
//	        exchange.Out.SetStatusCode(401)
//	        return false // Stop processing
//	    }
//	    return true // Continue processing
//	}
//
//	// Logging processor
//	func logProcessor(router Router, exchange *Exchange) bool {
//	    log.Printf("Processing request from %s", exchange.In.From())
//	    return true // Always continue
//	}
//
//	// Message transformation processor
//	func transformProcessor(router Router, exchange *Exchange) bool {
//	    body := exchange.In.Body()
//	    transformed := transform(body)
//	    exchange.In.SetBody(transformed)
//	    return true
//	}
type Process func(router Router, exchange *Exchange) bool

// OptionsSetter defines the interface for configuring routing components with various options.
// This interface is used by RouterOption functions to apply configuration settings
// to routers, endpoints, and other routing components.
//
// OptionsSetter is defined as the interface for routing components to configure various options.
// This interface is used by the RouterOption function to apply configuration settings to routers, endpoints, and other routing components.
//
// # Design Pattern
//
// OptionsSetter follows the Options pattern for flexible component configuration:
// OptionsSetter follows the options pattern to enable flexible component configuration:
//
// 1. Centralized Configuration: All configuration options are applied through a single interface
// 2. Type Safety: Strongly typed configuration methods
// 3. Extensibility: Easy to add new configuration options
// 4. Consistency: Uniform configuration approach across components
//
// Usage Pattern
//
//	func WithCustomOption(value string) RouterOption {
//	    return func(setter OptionsSetter) error {
//	        // Apply configuration
//	        setter.SetConfig(customConfig)
//	        return nil
//	    }
//	}
type OptionsSetter interface {
	// SetConfig sets the rule engine configuration for the component.
	// This configuration affects how the component interacts with the rule engine.
	//
	// SetConfig sets the rule engine configuration for components.
	// This configuration affects how components interact with the rule engine.
	//
	// Parameters
	// • config: Rule engine configuration to apply
	SetConfig(config types.Config)

	// SetRuleEnginePool sets a specific rule engine pool for the component.
	// This allows for custom pool selection and multi-tenancy support.
	//
	// SetRuleEnginePool sets a specific rule engine pool for components.
	// This allows for custom pool selection and multi-tenant support.
	//
	// Parameters
	// • pool: Rule engine pool to use
	SetRuleEnginePool(pool types.RuleEnginePool)

	// SetRuleEnginePoolFunc sets a function to dynamically determine the rule engine pool.
	// This enables advanced scenarios like load balancing and context-based pool selection.
	//
	// SetRuleEnginePoolFunc sets the function that dynamically determines the rule engine pool.
	// This enables advanced scenarios such as load balancing and context-based pool selection.
	//
	// Parameters
	// • f: Function that returns a rule engine pool based on the exchange
	//
	// Function Behavior
	// The function is called for each message exchange to determine the appropriate pool.
	// The function calls each message exchange to determine the appropriate pool.
	SetRuleEnginePoolFunc(f func(exchange *Exchange) types.RuleEnginePool)

	// SetContextFunc sets a function to modify or enhance the context for each exchange.
	// This allows for request-specific context modification and enrichment.
	//
	// SetContextFunc is set to the function that swaps modifications or enhances context for each change.
	// This allows requests for specific contextual modifications and enrichment.
	//
	// Parameters
	// • f: Function that modifies the context
	//
	// Context Enhancement
	// • Add request-specific values
	// • Set custom timeouts
	// • Apply security context
	// • Inject dependencies
	SetContextFunc(f func(ctx context.Context, exchange *Exchange) context.Context)

	// SetDefinition sets the DSL configuration for the component.
	// This provides access to the original DSL definition for introspection.
	//
	// SetDefinition sets the DSL configuration for components.
	// This provides access to the original DSL definition for introspection.
	//
	// Parameters
	// • dsl: Router DSL definition
	SetDefinition(dsl *types.RouterDsl)
}

// Executor defines the interface for executing the destination side of routing operations.
// Executors are responsible for processing messages according to specific destination types
// and configurations, providing a pluggable execution model for different target systems.
//
// Executor defines the interface for the target end of routing operations.
// The actuator handles messages according to specific target types and configurations, providing pluggable execution models for different target systems.
//
// # Key Concepts
//
// • Pluggable Execution: Different executors for different destination types
// • Configuration-Driven: Behavior controlled by configuration parameters
// • Variable Support: Dynamic path resolution with variable substitution
// • Stateless Design: Executors should be stateless and reusable
//
// # Executor Types
//
// • Chain Executor: Executes rule chains
// • Component Executor: Executes individual components
// • Script Executor: Executes script-based logic
// • Custom Executors: User-defined execution logic
//
// # Performance Considerations
//
// • Executors should be lightweight and fast to create
// • Heavy initialization should be done in Init()
// • Execute() should be optimized for high throughput
type Executor interface {
	// New creates a new instance of the executor.
	// This method should return a clean, initialized executor instance.
	//
	// New: Create a new instance of the executor.
	// This method should return a clean, initialized executor instance.
	//
	// Returns
	// • Executor: New executor instance
	//
	// Implementation Notes
	// • Should be lightweight and fast
	// • Should not share state between instances
	// • Should copy any necessary configuration
	New() Executor

	// IsPathSupportVar indicates whether the executor supports variable substitution in paths.
	// This determines if the destination path can contain dynamic variables.
	//
	// IsPathSupportVar indicates whether the executor supports variable substitution in the path.
	// This determines whether the target path can contain dynamic variables.
	//
	// Returns
	// • bool: true if variable substitution is supported true Indicates support for variable substitution
	//
	// Variable Format
	// Variables are typically specified using ${variableName} syntax.
	// Variables are usually specified using the ${variableName} syntax.
	IsPathSupportVar() bool

	// Init initializes the executor with configuration and destination-specific settings.
	// This method is called once during executor setup and should perform any heavy initialization.
	//
	// Init initializes the executor using configuration and target-specific settings.
	// This method is called once during actuator setup and should perform any heavy initialization.
	//
	// Parameters
	// • config: Rule engine configuration
	// • configuration: Destination-specific configuration
	//
	// Returns
	// • error: Initialization error, nil on success
	//
	// Initialization Tasks
	// • Parse and validate configuration
	// • Establish connections if needed
	// • Compile patterns or scripts
	// • Allocate resources
	Init(config types.Config, configuration types.Configuration) error

	// Execute performs the actual execution of the destination logic.
	// This is the core method that processes messages according to the executor's purpose.
	//
	// Execute: Actually executes the target logic.
	// This is the core method for processing messages according to the executor's purpose.
	//
	// Parameters
	// • ctx: Context for the execution operation
	// • router: Router providing execution context
	// • exchange: Message exchange to process
	//
	// Execution Flow
	// 1. Extract message and metadata from exchange
	// 2. Apply any variable substitution if supported
	// 3. Execute the destination logic (rule chain, component, etc.)
	// 4. Populate response in exchange.Out if needed
	//
	// Error Handling
	// Errors should be handled gracefully and may be set in the exchange.
	// Mistakes should be handled gracefully, possibly set during exchanges.
	Execute(ctx context.Context, router Router, exchange *Exchange)
}

// Factory defines the interface for creating endpoint instances.
// Factories provide a centralized mechanism for creating and configuring endpoints
// from various sources including DSL configurations and type specifications.
//
// Factory defines the interface for creating endpoint instances.
// The factory provides a centralized mechanism for creating and configuring endpoints from various sources, including DSL configuration and type specifications.
//
// # Factory Pattern Benefits
//
// • Centralized Creation: Single point for endpoint instantiation
// • Configuration Management: Consistent configuration application
// • Type Safety: Strongly typed endpoint creation
// • Extensibility: Easy to add new endpoint types
//
// # Supported Creation Methods
//
// • DSL-based: Create from JSON/YAML DSL definitions
// • Type-based: Create from component type and configuration
// • Definition-based: Create from structured definitions
type Factory interface {
	// NewFromDsl creates a new DynamicEndpoint instance from a DSL byte array.
	// This method parses the DSL and creates a fully configured dynamic endpoint.
	//
	// NewFromDsl creates a new DynamicEndpoint instance from a DSL byte array.
	// This method parses the DSL and creates fully configured dynamic endpoints.
	//
	// Parameters
	// • dsl: JSON byte array containing endpoint configuration
	// • opts: Additional options for endpoint creation
	//
	// Returns
	// • DynamicEndpoint: Created dynamic endpoint instance
	// • error: Creation error, nil on success
	//
	// DSL Format / DSL Format:
	// The DSL should follow the types.EndpointDsl structure format.
	// DSL should follow the types.EndpointDsl structure format.
	NewFromDsl(dsl []byte, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// NewFromDef creates a new DynamicEndpoint instance from a structured definition.
	// This method accepts a pre-parsed endpoint definition structure.
	//
	// NewFromDef creates a new DynamicEndpoint instance from the structured definition.
	// This method accepts a pre-resolved endpoint definition structure.
	//
	// Parameters
	// • def: Structured endpoint DSL definition
	// • opts: Additional options for endpoint creation
	//
	// Returns
	// • DynamicEndpoint: Created dynamic endpoint instance
	// • error: Creation error, nil on success
	//
	// Advantages
	// • No JSON parsing overhead
	// • Type safety at compile time
	// • Easier to programmatically generate
	NewFromDef(def types.EndpointDsl, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// NewFromType creates a new Endpoint instance from a component type and configuration.
	// This method creates a static endpoint for a specific protocol or component type.
	//
	// NewFromType creates a new Endpoint instance from the component type and configuration.
	// This method creates static endpoints for specific protocols or component types.
	//
	// Parameters
	// • componentType: Type identifier for the endpoint (e.g., "rest", "mqtt")
	// • ruleConfig: Rule engine configuration
	// • configuration: Component-specific configuration
	//
	// Returns
	// • Endpoint: Created endpoint instance
	// • error: Creation error, nil on success
	//
	// Supported Types
	// • "rest" - REST/HTTP endpoints REST/HTTP endpoints
	// • "mqtt" - MQTT pub/sub endpoints MQTT publish/subscribe endpoints
	// • "websocket" - WebSocket endpoints
	// • Custom types registered with the component registry
	NewFromType(componentType string, ruleConfig types.Config, configuration interface{}) (Endpoint, error)
}

// Pool defines the interface for managing a collection of dynamic endpoints.
// The pool provides centralized lifecycle management, including creation, retrieval,
// and cleanup of endpoint instances with support for hot reloading and configuration updates.
//
// Pool defines the interface for managing dynamic endpoint collections.
// The pool provides centralized lifecycle management, including creation, retrieval, and cleanup of endpoint instances, supporting hot reloading and configuration updates.
//
// # Pool Architecture
//
// • Centralized Management: Single point for endpoint lifecycle
// • Hot Reloading: Support for runtime configuration updates
// • Resource Cleanup: Automatic cleanup of unused endpoints
// • Thread Safety: Safe for concurrent access and operations
//
// # Use Cases
//
// • Multi-tenant Applications: Separate endpoints per tenant
// • Dynamic API Gateways: Runtime endpoint configuration
// • Microservice Orchestration: Dynamic service endpoint management
// • Development and Testing: Easy endpoint switching and testing
type Pool interface {
	// New creates a new dynamic endpoint with the specified ID and configuration.
	// The endpoint is automatically added to the pool for management.
	//
	// New creates a new dynamic endpoint using the specified ID and configuration.
	// Endpoints are automatically added to the pool for management.
	//
	// Parameters
	// • id: Unique identifier for the endpoint
	// • del: DSL configuration as byte array
	// • opts: Additional options for endpoint creation
	//
	// Returns
	// • DynamicEndpoint: Created dynamic endpoint instance
	// • error: Creation error, nil on success
	//
	// Behavior
	// • If an endpoint with the same ID exists, it may be replaced or return an error
	// • The endpoint is automatically started if the configuration allows
	New(id string, del []byte, opts ...DynamicEndpointOption) (DynamicEndpoint, error)

	// Get retrieves a dynamic endpoint by its unique identifier.
	// This method provides access to existing endpoints in the pool.
	//
	// Get retrieves dynamic endpoints using unique identifiers.
	// This method provides access to existing endpoints in the pool.
	//
	// Parameters
	// • id: Unique identifier of the endpoint to retrieve
	//
	// Returns
	// • DynamicEndpoint: Retrieved endpoint instance
	// • bool: true if found, false if not found. True means found, false means not found
	Get(id string) (DynamicEndpoint, bool)

	// Del removes a dynamic endpoint from the pool by its ID.
	// This method performs cleanup and stops the endpoint before removal.
	//
	// Del removes dynamic endpoints from the pool by ID.
	// This method performs cleanup and stops endpoints before deletion.
	//
	// Parameters
	// • id: Unique identifier of the endpoint to remove
	//
	// Cleanup Behavior
	// • Stops the endpoint gracefully
	// • Releases any held resources
	// • Removes from internal storage
	Del(id string)

	// Stop gracefully shuts down all dynamic endpoints in the pool.
	// This method should be called during application shutdown.
	//
	// Stop gracefully shuts down all dynamic endpoints in the pool.
	// This method should be called during application shutdown.
	//
	// Shutdown Process
	// 1. Stop accepting new requests
	// 2. Complete processing of in-flight requests
	// 3. Release resources and cleanup
	// 4. Clear the pool
	Stop()

	// Reload reloads all dynamic endpoints with new configuration options.
	// This allows for global configuration updates across all endpoints.
	//
	// Reload uses the new configuration option to reload all dynamic endpoints.
	// This allows for global configuration updates for all endpoints.
	//
	// Parameters
	// • opts: New configuration options to apply
	//
	// Reload Behavior
	// • Updates configuration for all endpoints
	// • May restart endpoints if required
	// • Preserves endpoint state where possible
	Reload(opts ...DynamicEndpointOption)

	// Range iterates over all dynamic endpoints in the pool.
	// This method provides a way to inspect and operate on all managed endpoints.
	//
	// Range traverses all dynamic endpoints in the pool.
	// This method provides a way to check and operate all managed endpoints.
	//
	// Parameters
	// • f: Function to call for each endpoint
	//
	// Function Signature
	// • key: Endpoint ID
	// • value: Endpoint instance
	// • Return bool: true to continue iteration, false to stop
	//
	// Usage
	//	pool.Range(func(key, value any) bool {
	//	    id := key.(string)
	//	    endpoint := value.(DynamicEndpoint)
	//	    // Process endpoint
	//	    return true
	//	})
	Range(f func(key, value any) bool)

	// Factory returns the factory instance used to create endpoints.
	// This provides access to the underlying factory for direct endpoint creation.
	//
	// Factory returns the factory instance used to create endpoints.
	// This provides access to the underlying factory for direct endpoint creation.
	//
	// Returns
	// • Factory: Factory instance used by the pool
	Factory() Factory
}

// HeaderModifier defines the interface for modifying headers in endpoint messages.
// This interface provides a standardized way to manipulate message headers and metadata
// across different protocols and message types.
//
// HeaderModifier defines the interface for modifying headers in endpoint messages.
// This interface provides a standardized way to operate message headers and metadata across different protocols and message types.
//
// # Key Features
//
// • Protocol Abstraction: Unified header manipulation across protocols
// • Metadata Integration: Direct access to RuleGo metadata
// • Standard Operations: Add, set, and delete header operations
// • Type Safety: Strongly typed header operations
//
// # Use Cases
//
// • Response Header Management: Set response headers for HTTP
// • Metadata Propagation: Forward metadata between systems
// • Protocol Translation: Convert headers between protocols
// • Security Headers: Add security-related headers
type HeaderModifier interface {
	// AddHeader adds a header value to the message.
	// If the header already exists, the value is appended.
	//
	// AddHeader adds a header value to the message.
	// If the head is already present, the value will be added.
	//
	// Parameters
	// • key: Header name
	// • value: Header value to add
	//
	// Behavior
	// • Multiple values for the same key are supported
	// • Case-insensitive key handling (protocol dependent)
	AddHeader(key, value string)

	// SetHeader sets a header value, replacing any existing value.
	// This operation overwrites any existing values for the specified key.
	//
	// SetHeader sets the header value and replaces any existing value.
	// This operation overrides any existing value of the specified key.
	//
	// Parameters
	// • key: Header name
	// • value: Header value to set
	//
	// Behavior
	// • Overwrites existing values
	// • Creates the header if it doesn't exist
	SetHeader(key, value string)

	// DelHeader removes a header from the message.
	// This operation removes all values associated with the specified key.
	//
	// DelHeader removes heads from messages.
	// This operation deletes all values associated with the specified key.
	//
	// Parameters
	// • key: Header name to remove
	//
	// Behavior
	// • Removes all values for the key
	// • No-op if the header doesn't exist
	DelHeader(key string)

	// GetMetadata returns the RuleGo metadata associated with the message.
	// This provides access to the internal metadata structure for advanced operations.
	//
	// GetMetadata returns the RuleGo metadata associated with the message.
	// This provides access to internal metadata structures for advanced operations.
	//
	// Returns
	// • *types.Metadata: Message metadata structure
	//
	// Metadata Usage
	// • Access to internal message properties
	// • Cross-component data sharing
	// • Protocol-specific information
	GetMetadata() *types.Metadata
}

// Flusher defines the interface for flushing response streams.
// This interface is used for streaming responses (like SSE) to immediately
// send buffered data to the client.
//
// Flusher defines the interface for refreshing response streams.
// This interface is used for stream responses (such as SSE) to immediately send buffered data to the client.
type Flusher interface {
	// Flush sends any buffered data to the client.
	// This is particularly important for streaming responses like SSE.
	//
	// Flush sends buffered data to the client.
	// This is especially important for stream responses like SSE.
	Flush()
}

// HttpEndpoint defines the interface for HTTP-specific endpoint functionality.
// This interface extends the basic Endpoint interface with HTTP-specific methods
// for handling different HTTP methods and static file serving.
//
// HttpEndpoint defines the interface for HTTP-specific endpoint functionality.
// This interface extends the basic Endpoint interface, adding HTTP-specific methods for handling different HTTP methods and static file services.
//
// # Key Features
//
// • HTTP Method Support: Dedicated methods for each HTTP verb
// • Static File Serving: Built-in static file hosting capabilities
// • Global Options: Support for global OPTIONS handler
// • Fluent API: Chainable method calls for configuration
//
// HTTP Method Mapping / HTTP Method Mapping:
//
// Each HTTP method has a corresponding configuration method:
// Each HTTP method has a corresponding configuration method:
//
// • GET: Retrieve resources. GET: Search resources
// • POST: Create new resources POST: Create new resources
// • PUT: Update existing resources PUT: Update existing resources
// • DELETE: Remove resources DELETE: Deletes resources
// • PATCH: Partial updates PATCH
// • HEAD: Retrieve headers only HEAD: Retrieve headers only
// • OPTIONS: CORS and capability discovery OPTIONS: CORS and capability discovery
//
// Usage Pattern
//
//	httpEndpoint.
//	    GET(router1, router2).
//	    POST(router3).
//	    PUT(router4).
//	    RegisterStaticFiles("/static/*").
//	    Start()
type HttpEndpoint interface {
	// Endpoint provides all basic endpoint functionality
	// Endpoint provides all basic endpoint functions
	Endpoint

	// GET configures routers for HTTP GET requests.
	// GET requests are typically used for retrieving resources without side effects.
	//
	// GET Configure the router for HTTP GET requests.
	// GET requests are usually used to retrieve resources without causing side effects.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	GET(routers ...Router) HttpEndpoint

	// HEAD configures routers for HTTP HEAD requests.
	// HEAD requests return the same headers as GET but without the response body.
	//
	// HEAD is configured for HTTP HEAD requests.
	// HEAD requests return the same head as GET but without a response body.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	HEAD(routers ...Router) HttpEndpoint

	// OPTIONS configures routers for HTTP OPTIONS requests.
	// OPTIONS requests are used for CORS preflight checks and capability discovery.
	//
	// OPTIONS Configure the router for HTTP OPTIONS requests.
	// OPTIONS requests are used for CORS pre-check and feature discovery.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	OPTIONS(routers ...Router) HttpEndpoint

	// POST configures routers for HTTP POST requests.
	// POST requests are typically used for creating new resources.
	//
	// POST Configure the router for HTTP POST requests.
	// POST requests are typically used to create new resources.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	POST(routers ...Router) HttpEndpoint

	// PUT configures routers for HTTP PUT requests.
	// PUT requests are typically used for updating or creating resources with known IDs.
	//
	// PUT Configure the router for HTTP PUT requests.
	// PUT requests are typically used to update or create resources with known IDs.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	PUT(routers ...Router) HttpEndpoint

	// PATCH configures routers for HTTP PATCH requests.
	// PATCH requests are used for partial updates to existing resources.
	//
	// PATCH Configuration of the router for HTTP PATCH requests.
	// PATCH requests are used to partially update existing resources.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	PATCH(routers ...Router) HttpEndpoint

	// DELETE configures routers for HTTP DELETE requests.
	// DELETE requests are used for removing resources.
	//
	// DELETE is configured for HTTP DELETE requests.
	// DELETE requests are used to delete resources.
	//
	// Parameters
	// • routers: Variable number of router configurations
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	DELETE(routers ...Router) HttpEndpoint

	// GlobalOPTIONS sets a global handler for all OPTIONS requests.
	// This is typically used for CORS configuration and global OPTIONS responses.
	//
	// GlobalOPTIONS sets up a global processor for all OPTIONS requests.
	// This is typically used for CORS configuration and global OPTIONS responses.
	//
	// Parameters
	// • handler: HTTP handler for OPTIONS requests The HTTP handler for OPTIONS requests
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	//
	// Use Cases
	// • CORS preflight handling
	// • API capability advertisement API-enabled advertising
	// • Global OPTIONS responses
	GlobalOPTIONS(handler http.Handler) HttpEndpoint

	// RegisterStaticFiles enables static file serving for the specified path pattern.
	// This allows the endpoint to serve static files like HTML, CSS, JS, and images.
	//
	// RegisterStaticFiles enables static file services for specified path modes.
	// This allows endpoints to serve static files such as HTML, CSS, JS, and images.
	//
	// Parameters
	// • resourceMapping: Path pattern for static file mapping
	//
	// Returns
	// • HttpEndpoint: The same endpoint instance for method chaining
	//
	// Resource Mapping Examples
	// • "/ui/=/home/demo/dist" - Map /ui/ path to /home/demo/dist directory
	// • "/editor/=./editor,/images/=./editor/images" - Multiple mappings
	// • "/static/=./public" - Map /static/ to local public directory
	//
	// File Serving Features
	// • MIME type detection: MIME type detection
	// • Caching headers
	// • Range request support
	// • Directory listing (configurable)
	RegisterStaticFiles(resourceMapping string) HttpEndpoint
}
