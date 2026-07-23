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

// Package types defines the core interfaces, data structures, and contracts for the RuleGo rule engine framework.
// Package types define the core interfaces, data structures, and contracts of the RuleGo rules engine framework.
//
// This package serves as the foundation for the entire RuleGo ecosystem, providing:
// This package forms the foundation of the entire RuleGo ecosystem, providing:
//
//   - Core interfaces for components, nodes, and rule engines
//     Core interfaces for components, nodes, and rule engines
//   - Message structures for data flow between nodes
//     Message structure for data flow between nodes
//   - Configuration and context types for rule execution
//     Configuration and context type for rule execution
//   - Aspect-oriented programming (AOP) support
//     Face-to-face programming (AOP) support
//   - Plugin and component registry mechanisms
//     Plugin and component registration mechanisms
//
// # Extension Component Libraries
// # Expanding the component library ecosystem
//
// RuleGo provides a complete ecosystem of extension component libraries:
// RuleGo offers a complete ecosystem of extended component libraries:
//
//  1. rulego-components (https://github.com/rulego/rulego-components)
//     Core extension components including Kafka, Redis, RabbitMQ, NATS, gRPC, FastHTTP
//     Core extension components, including general endpoints and processing components such as Kafka, Redis, RabbitMQ, NATS, gRPC, FastHTTP, and others
//
//  2. rulego-components-ai (https://github.com/rulego/rulego-components-ai)
//     AI scenario components for intelligent inference, model invocation, data preprocessing
//     AI scenario component library includes AI-related endpoints and components such as intelligent inference, model calls, and data preprocessing
//
//  3. rulego-components-ci (https://github.com/rulego/rulego-components-ci)
//     CI/CD scenario components for code repositories, build tools, deployment platforms
//     CI/CD scenario component libraries, including code warehouses, build tools, deployment platform integrations, and other DevOps-related components
//
//  4. rulego-components-iot (https://github.com/rulego/rulego-components-iot)
//     IoT scenario components for device connectivity, protocol conversion, data acquisition
//     IoT scenario component library, including IoT-related components such as device connection, protocol conversion, and data collection
//
//  5. rulego-components-etl (https://github.com/rulego/rulego-components-etl)
//     ETL scenario components for database connections, file processing, data cleansing
//     ETL scenario component library includes data processing components such as database connections, file processing, and data cleaning
//
// These extension libraries provide modular architecture, specialized solutions, unified API interfaces,
// and support on-demand selection and seamless integration.
// These expansion libraries offer modular architectures, dedicated solutions, and unified API interfaces, supporting on-demand selection and seamless integration.
//
// # Key Components
// # Key components
//
//   - Node: Interface for implementing rule engine components
//     Node: The interface for implementing rule engine components
//   - RuleMsg: Core message structure for data flow
//     RuleMsg: The core message structure for data flow
//   - RuleContext: Execution context for message processing
//     RuleContext: The execution context for message processing
//   - RuleEngine: Main engine interface for rule execution
//     RuleEngine: The main engine interface for rule execution
//   - ComponentRegistry: Component registration and management
//     ComponentRegistry: Registers and manages components
//
// # Architecture Overview
// # Architecture Overview
//
// The RuleGo framework follows a modular, component-based architecture:
// The RuleGo framework follows a modular, component-based architecture:
//
//  1. Messages flow through a chain of interconnected nodes
//     Messages flow through interconnected node chains
//  2. Each node implements specific business logic or transformation
//     Each node implements specific business logic or transformations
//  3. Relationships between nodes define the message routing
//     The relationships between nodes define message routing
//  4. AOP aspects provide cross-cutting concerns like monitoring
//     The AOP aspect provides monitoring and other cross-sectional points of concern
//
// # Example Usage
// # Usage examples
//
//	// Implement a custom node component
//	Implement custom node components
//	type MyNode struct{}
//
//	func (n *MyNode) Type() string { return "myNode" }
//	func (n *MyNode) New() types.Node { return &MyNode{} }
//	func (n *MyNode) Init(config types.Config, configuration types.Configuration) error { return nil }
//	func (n *MyNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
//		// Process message and forward to next nodes
//		Processes messages and forwards them to the next node
//		ctx.TellSuccess(msg)
//	}
//	func (n *MyNode) Destroy() {}
//
//	// Register the component
//	Register the components
//	registry.Register(&MyNode{})
//
//	// Use in rule chain DSL
//	Used in Rule Chain DSL
//	{
//		"ruleChain": {
//			"nodes": [
//				{
//					"id": "s1",
//					"type": "myNode",
//					"configuration": {}
//				}
//			]
//		}
//	}
//
// For detailed usage examples and documentation, see the RuleGo main package and extension libraries.
// For detailed usage examples and documentation, please refer to the RuleGo main package and extension library.
package types

import (
	"context"
)

// Relation types define the connections between nodes. These are common relations that can be customized.
// Relationship types define connections between nodes. These are common relationships that can be customized.
//
// These constants represent the standard relationship types used to route messages between nodes:
// These constants represent the standard relationship types used to route messages between nodes:
//   - Success: Message processed successfully, continue to success path
//     Success: Message processing successful, continue the success path
//   - Failure: Message processing failed, route to error handling
//     Failure: Message processing failure, routing to error handling
//   - True/False: Boolean logic routing for filter and condition nodes
//     True/False: Boolean logic routing for filters and condition nodes
//   - Stream: Streaming data flow for real-time data processing
//     Stream: A streaming data stream used for real-time data processing
const (
	Success = "Success"
	Failure = "Failure"
	True    = "True"
	False   = "False"
	Stream  = "Stream"
)

// Flow direction types indicate the direction of message flow into and out of nodes.
// The flow type indicates the direction in which messages flow in and out of nodes.
//
// These constants are used for debugging, monitoring, and AOP aspects to track message flow:
// These constants are used for debugging, monitoring, and AOP sections to track message flows:
const (
	In  = "IN"  // Represents a message flowing into a node. Indicates the message flowing into nodes
	Out = "OUT" // Represents a message flowing out of a node. Indicates the message outflow node
	Log = "Log" // Used for logging purposes. Used for logging purposes
)

// Script types define the scripting languages supported for script execution within nodes.
// Script types define the scripting languages supported for script execution within the node.
//
// These constants are used by script-enabled components to specify the execution engine:
// These constants are used by components that support scripts to specify the execution engine:
const (
	AllScript = ""       // All script match. Matches all script types
	Js        = "Js"     // Represents JavaScript scripting language. Represents the JavaScript scripting language
	Lua       = "Lua"    // Represents Lua scripting language. Represents the Lua scripting language
	Python    = "Python" // Represents Python scripting language. Represents the Python scripting language
	AiTool    = "AiTool" // Represents AI Tool. Refers to AI tools
)

// OnEndFunc is a callback function type that is executed when a branch of the rule chain completes.
// OnEndFunc is a callback function type executed when a rule chain branch completes.
//
// This callback provides detailed information about the execution result:
// This callback provides detailed information about the execution results:
//   - ctx: The rule context containing execution state
//     ctx: Contains the rule context of the execution state
//   - msg: The final message after processing
//     msg: The final message after processing
//   - err: Any error that occurred during processing
//     err: Any errors that occur during processing
//   - relationType: The relationship type that led to this endpoint
//     relationType: The type of relationship that causes this endpoint
type OnEndFunc = func(ctx RuleContext, msg RuleMsg, err error, relationType string)

// Configuration is a type for component configurations, represented as a map with string keys and interface{} values.
// Configuration is a type of component configuration, represented as a mapping with string keys and interface{} values.
//
// This flexible configuration format allows components to define their own configuration schema
// while providing type safety through validation during component initialization.
// This flexible configuration format allows components to define their own configuration patterns while providing type safety through validation during component initialization.
//
// Example:
// Example:
//
//	config := Configuration{
//	    "timeout": 30,
//	    "host": "localhost",
//	    "port": 8080,
//	    "enabled": true,
//	}
type Configuration map[string]interface{}

// Copy creates a shallow copy of the Configuration.
// Copy: Create a shallow copy of the Configuration.
//
// This method creates a new Configuration map and copies all key-value pairs from the original.
// Note that this is a shallow copy - if values are pointers or reference types,
// they will still reference the same underlying data.
// This method creates a new Configuration mapping and copies all key-value pairs from the original map.
// Note that this is a shallow copy—if the value is a pointer or reference type, they will still reference the same underlying data.
//
// Returns:
// Returns:
//   - Configuration: A new Configuration containing copies of all key-value pairs
//     Configuration: A new configuration containing all key-value pair replicas
func (c Configuration) Copy() Configuration {
	if c == nil {
		return nil
	}
	copy := make(Configuration, len(c))
	for key, value := range c {
		copy[key] = value
	}
	return copy
}

// ComponentType is an enum for component types: rule nodes or sub-rule chains.
// ComponentType is an enumeration of component types: rule nodes or subrule chains.
//
// This type distinguishes between different kinds of components in the rule chain:
// This type distinguishes different types of components in the rule chain:
type ComponentType int

const (
	NODE     ComponentType = iota // NODE represents a rule node component. NODE stands for Rule Node component
	CHAIN                         // CHAIN represents a sub-rule chain component. CHAIN represents the sub-rule chain component
	ENDPOINT                      // ENDPOINT represents an endpoint component. ENDPOINT stands for Endpoint Component
)

// PluginRegistry is an interface for providing node components via Go plugins.
// PluginRegistry provides an interface for node components via Go plugins.
//
// This interface enables dynamic loading of components at runtime, allowing for modular architecture
// and third-party component distribution. Plugins are compiled as .so files and loaded dynamically.
// This interface supports dynamic component loading at runtime, allowing modular architectures and third-party component distribution. The plugin compiles the file into a.so file and loads it dynamically.
//
// Implementation Guidelines:
// Implementation Guide:
//  1. Plugin must export a variable named "Plugins" implementing this interface
//     Plugins must export a variable named "Plugins" to implement this interface
//  2. Init() should handle plugin initialization and resource setup
//     Init() should handle plugin initialization and resource settings
//  3. Components() should return all components provided by the plugin
//     Components() should return all components provided by the plugin
//
// Example:
// Example:
//
//	package main
//	var Plugins MyPlugins // Plugin entry point
//	type MyPlugins struct{}
//
//	func (p *MyPlugins) Init() error {
//		return nil // Initialization logic for the plugin
//	}
//
//	func (p *MyPlugins) Components() []types.Node {
//		return []types.Node{&UpperNode{}, &TimeNode{}, &FilterNode{}} // A plugin can provide multiple components
//	}
//
//	// Build command:
//	// go build -buildmode=plugin -o plugin.so plugin.go # Compile the plugin to generate a plugin.so file
//	// Registration:
//	// rulego.Registry.RegisterPlugin("test", "./plugin.so") // Register the plugin with the default RuleGo registry
type PluginRegistry interface {
	// Init initializes the plugin.
	// Init initialization plugin.
	Init() error
	// Components returns a list of components provided by the plugin.
	// Components returns a list of components provided by the plugin.
	Components() []Node
}

// ComponentRegistry is an interface for registering node components.
// ComponentRegistry is the interface for registering node components.
//
// This registry manages the lifecycle of components and provides factory methods for component creation.
// It supports both static registration (compile-time) and dynamic registration (runtime via plugins).
// This registry manages the lifecycle of components and provides factory methods for component creation.
// It supports static registration (compile time) and dynamic registration (runtime via plugins).
//
// Thread Safety:
// Thread safety:
// Implementations should be thread-safe to support concurrent registration and component creation
// in multi-threaded environments.
// The implementation should be thread-safe to support concurrent registration and component creation in multithreaded environments.
//
// Usage Pattern:
// Usage mode:
//  1. Register components during application startup
//     Register components during application startup
//  2. Use NewNode() to create component instances for rule chains
//     Use NewNode() to create component instances for the rule chain
//  3. Retrieve component metadata for UI configuration
//
// ComponentRegistry is the interface for managing rule engine components with isolation and discovery capabilities.
// ComponentRegistry is the interface for managing rule engine components, featuring isolation and discovery capabilities.
//
// Core Responsibilities:
// 1. Component lifecycle management - Component lifecycle management
// 2. Namespace isolation - Namespace isolation
// 3. Dynamic loading and unloading
// 4. Visual configuration support
//
// Isolation Features:
//   - Independent component space: Each registry maintains separate component collections
//   - Type namespaces: Supports "domain/type" format to prevent conflicts
//   - Multi-tenant support: Different business domains use isolated component registries
//   - Version management: multiple versions of the same component type can coexist
//
// Component Discovery:
//   - GetComponents(): Get a list of all available components
//   - GetComponentForms(): Retrieves component configuration forms, supports UI tools - Get component configuration forms for UI tools
//   - NewNode(): Instantiate components by type name
//   - Automatic component categorization and metadata extraction
//
// Usage Patterns:
//
//	Basic Registration - Basic registration
//	registry.Register(&MyCustomNode{})
//
//	Namespace registration
//	registry.Register(&MyNode{}) // Type() returns "mycompany/processor"
//
//	Plugin dynamic loading
//	registry.RegisterPlugin("businessPlugin", "./plugins/business.so")
//
//	Get available components
//	components := registry.GetComponents()
//	for typeName, node := range components {
//		fmt.Printf("Available: %s\n", typeName)
//	}
type ComponentRegistry interface {
	// Register adds a new component. If `node.Type()` already exists, it returns an 'already exists' error.
	// Register to add new components. If `node.Type()` already exists, returning an error "Existed".
	Register(node Node) error
	// RegisterPlugin loads and registers a component from an external .so file using the plugin mechanism.
	// If `name` already exists or the component list provided by the plugin `node.Type()` exists, it returns an 'already exists' error.
	// RegisterPlugin uses a plugin mechanism to load and register components from external.so files.
	// If `name` already exists or the plugin provides a list of components called `node.Type()` exists, returning an "Existent" error.
	RegisterPlugin(name string, file string) error
	// Unregister removes a component or a batch of components by plugin name.
	// Unregister deletes components or batch components by plugin name.
	Unregister(componentType string) error
	// NewNode creates a new instance of a node by nodeType.
	// NewNode creates a new instance of a node using nodeType.
	NewNode(nodeType string) (Node, error)
	// GetComponents retrieves a complete list of all registered components in this registry instance.
	// GetComponents retrieves the complete list of all registered components in this registry instance.
	//
	// This method provides component discovery capabilities for:
	// This method provides component discovery capabilities for the following scenarios:
	//   - Runtime component enumeration and validation
	//   - UI tools displaying available component types
	//   - Dynamic rule chain composition and validation
	//   - Component inventory management and auditing
	//
	// Returns:
	// Returns:
	//   - map[string]Node: Map of component type names to component instances
	//     map[string]Node: Mapping the component type name to the component instance
	//
	// The returned map contains:
	// The returned mapping includes:
	//   - Key: Component type identifier (e.g., "jsTransform", "mycompany/processor")
	//     Key: Component type identifier (e.g., "jsTransform", "mycompany/processor")
	//   - Value: Component prototype instance for metadata access
	//     Value: Component prototype instances used for metadata access
	//
	// Note: The returned instances are prototypes for metadata only.
	// Use NewNode() to create working instances for rule chains.
	// Note: The returned instances are prototypes used only for metadata.
	// Use NewNode() to create a working instance for the rule chain.
	GetComponents() map[string]Node

	// GetComponentForms retrieves configuration forms for all registered components, enabling visual configuration tools.
	// GetComponentForms retrieves configuration forms for all registered components and supports visual configuration tools.
	//
	// This method supports visual rule chain builders by providing:
	// This method supports the visual rule chain builder by providing the following:
	//   - Component configuration schemas and form definitions
	//   - Input validation rules and constraints
	//   - UI rendering hints and component categorization
	//   - Documentation and help text for each component
	//
	// Returns:
	// Returns:
	//   - ComponentFormList: Structured metadata for UI configuration tools
	//     ComponentFormList: Structured metadata for UI configuration tools
	//
	// The returned forms enable:
	// The returned form supports:
	//   - Drag-and-drop rule chain editors
	//   - Dynamic configuration forms generation
	//   - Real-time configuration validation
	//   - Component documentation integration
	GetComponentForms() ComponentFormList
}

// Node is the core interface for rule engine node components.
// It defines the fundamental contract for all components in the RuleGo ecosystem,
// encapsulating business logic or common functionality that can be invoked through rule chain configurations.
//
// Node is the core interface of the rule engine node components.
// It defines the fundamental contracts for all components within the RuleGo ecosystem,
// Encapsulation can be configured through the rule chain to configure the business logic or general functions called.
//
// Architecture Overview:
// Architecture Overview:
//
//	The Node interface represents the atomic unit of computation in RuleGo rule chains.
//	Each component encapsulates specific functionality and can be connected to other
//	components to form complex processing workflows. Components are stateless by design,
//	with each rule chain instance receiving its own component instance for data isolation.
//
//	The Node interface represents the atomic computing unit in the RuleGo rule chain. Each component encapsulates specific functions,
//	It can connect to other components to form complex processing workflows. Components are stateless by design,
//	Each instance of the rule chain receives its own component instance to achieve data isolation.
//
// Component Categories:
// Module categories:
//   - Filter components: Data filtering and routing based on conditions
//     Filter Components: Condition-based data filtering and routing
//   - Transform components: Data transformation and enrichment
//     Converter components: Data conversion and enrichment
//   - Action components: Business logic execution and external service integration
//     Action components: business logic execution and external service integration
//   - Flow components: Control flow and rule chain orchestration
//     Process components: control flow and rule chain orchestration
//   - External components: Integration with external systems and protocols
//     External components: integration with external systems and protocols
//
// Lifecycle Management:
// Lifecycle Management:
//
//  1. Registration: Components are registered with the ComponentRegistry
//     Registration: Components are registered through the ComponentRegistry
//  2. Instantiation: New() creates isolated instances for each rule chain
//     Instantiation: New() creates isolated instances for each rule chain
//  3. Initialization: Init() configures the component with specific parameters
//     Initialization: Init() uses specific parameters to configure components
//  4. Execution: OnMsg() processes incoming messages
//     Execute: OnMsg() to handle incoming messages
//  5. Cleanup: Destroy() releases resources when no longer needed
//     Cleanup: Destroy() releases resources when no longer needed
//
// Optional Interface Extensions:
// Optional Interface Expansion:
//
//	Components can implement additional interfaces for enhanced functionality:
//	Components can implement additional interfaces to enhance functionality:
//	- ComponentDefGetter: Provides metadata for visual configuration tools
//	  ComponentDefGetter: Provides metadata for the visualization configuration tool
//	- CategoryGetter: Defines component categorization for UI organization
//	  CategoryGetter: Defines component categories to organize the UI
//	- DescGetter: Supplies component descriptions and documentation
//	  DescGetter: Provides component descriptions and documentation
//
// Thread Safety Considerations:
// Thread safety considerations:
//
//   - Each rule chain receives its own component instance (data isolation)
//     Each rule chain receives its own component instance (data isolation)
//   - OnMsg() may be called concurrently from multiple goroutines
//     OnMsg() may be called concurrently from multiple goroutines
//   - Components should avoid shared mutable state without proper synchronization
//     Components should avoid sharing variable states without proper synchronization
//   - Use NodePool for expensive resource sharing across multiple instances
//     Use NodePool to share expensive resources across multiple instances
//
// Best Practices:
// Best Practices:
//   - Keep components stateless for better scalability
//     Keep components stateless for better scalability
//   - Use meaningful type names with namespace prefixes (e.g., "myCompany/dataProcessor")
//     Use meaningful type names and namespace prefixes (e.g., "myCompany/dataProcessor")
//   - Implement proper error handling and resource cleanup
//     Achieve proper error handling and resource cleanup
//   - Consider implementing optional interfaces for better tooling support
//     Consider implementing optional interfaces for better tool support
//   - Use configuration validation in Init() to catch errors early
//     Use configuration validation in Init() to catch errors early
//
// Registration Example:
// Registration example:
//
//	// Register a custom component
//	Register custom components
//	rulego.Registry.Register(&MyCustomNode{})
//
//	// Register from plugin
//	Register from the plugin
//	rulego.Registry.RegisterPlugin("myPlugin", "./plugin.so")
//
// Implementation Reference:
// Implementation reference:
//
//	Standard implementations can be found in the `components` package.
//	Extension components are available in separate repositories:
//	The standard implementation can be found in the `components` package.
//	Expansion components are available in separate warehouses:
//	- github.com/rulego/rulego-components
//	- github.com/rulego/rulego-components-ai
//	- github.com/rulego/rulego-components-iot
//	- github.com/rulego/rulego-components-ci
//	- github.com/rulego/rulego-components-etl
type Node interface {
	// New creates a new instance of the component for each rule chain.
	// This method ensures data isolation between different rule chain instances,
	// preventing state sharing and potential race conditions.
	//
	// New: Create a new instance of the component for each rule chain.
	// This method ensures data isolation between different rule chain instances,
	// Prevent state sharing and potential race conditions.
	//
	// Design Pattern:
	// Design Pattern:
	//	This follows the Prototype pattern, where the registered component
	//	serves as a template for creating new instances. Each instance
	//	maintains its own state and configuration.
	//
	//	This follows the prototype pattern, where registered components serve as templates for creating new instances.
	//	Each instance maintains its own state and configuration.
	//
	// Returns:
	// Returns:
	//   - Node: A new component instance ready for initialization
	//     Node: Ready to initialize the new component instance
	//
	// Implementation Notes:
	// Implementation Notes:
	//   - Return a new instance of the same type, not a copy of existing data
	//     Returns a new instance of the same type, rather than a copy of existing data
	//   - Initialize only default values, detailed configuration happens in Init()
	//     Only initialize the default value; detailed configuration is done in Init().
	//   - Avoid expensive operations that should be deferred to Init()
	//     Avoid expensive operations that should be delayed until Init().
	New() Node

	// Type returns the unique component type identifier.
	// This identifier is used for component lookup, registration, and rule chain configuration.
	//
	// Type returns a unique component type identifier.
	// This identifier is used for component lookup, registration, and rule chain configuration.
	//
	// Naming Convention:
	// Naming Agreement:
	//	It is recommended to use forward slashes (/) to distinguish namespaces
	//	and prevent type name conflicts between different component libraries.
	//
	//	It is recommended to use a positive slash (/) to distinguish namespaces and prevent type name conflicts between different component libraries.
	//
	// Examples:
	// Example:
	//   - Standard components: "jsTransform", "httpClient", "delay"
	//     Standard components: "jsTransform", "httpClient", "delay"
	//   - Company-specific: "myCompany/dataProcessor", "acme/validator"
	//     Company specific: "myCompany/dataProcessor", "acme/validator"
	//   - Protocol-specific: "mqtt/publish", "kafka/consumer"
	//     Protocol specific: "mqtt/publish", "kafka/consumer"
	//
	// Returns:
	// Returns:
	//   - string: Unique component type identifier
	//     string: Unique component type identifier
	//
	// Requirements:
	// Requirements:
	//   - Must be unique across all registered components
	//     It must be unique among all registered components
	//   - Should be descriptive and self-explanatory
	//     It should be descriptive and self-explanatory
	//   - Should remain stable across component versions
	//     Stability should be maintained between component versions
	Type() string

	// Init initializes the component with configuration parameters and rule engine context.
	// This method is called once during rule chain initialization and should perform
	// all necessary setup operations including parameter validation and resource allocation.
	//
	// Init uses configuration parameters and rules engine context to initialize components.
	// This method is called once during the rule chain initialization and should perform all necessary configuration operations,
	// This includes parameter validation and resource allocation.
	//
	// Initialization Responsibilities:
	// Initialization Responsibilities:
	//   - Parse and validate component configuration
	//     Parsing and verifying component configurations
	//   - Initialize external clients (HTTP, database, message queue)
	//     Initialize external clients (HTTP, database, message queue)
	//   - Set up internal state and caches
	//     Set internal state and cache
	//   - Validate required dependencies and resources
	//     Verify the dependencies and resources required
	//   - Register with external services if needed
	//     If needed, register with external services
	//
	// Configuration Processing:
	// Configuration Processing:
	//	The configuration parameter contains the component-specific settings
	//	extracted from the rule chain DSL. Use the maps.Map2Struct utility
	//	to convert the configuration map to your component's configuration struct.
	//
	//	Configuration parameters include component-specific settings extracted from the rule chain DSL.
	//	Use the maps.Map2Struct tool to convert configuration mappings into component configuration structures.
	//
	// Error Handling:
	// Error handling:
	//	Return an error if initialization fails. This will prevent the rule chain
	//	from starting and provide early feedback about configuration issues.
	//
	//	If initialization fails, an error is returned. This will prevent the rule chain from launching and provide early feedback on configuration issues.
	//
	// Parameters:
	// Parameters:
	//   - ruleConfig: Global rule engine configuration and shared resources
	//     ruleConfig: Global rules engine configuration and resource sharing
	//   - configuration: Component-specific configuration from the rule chain DSL
	//     configuration: Component-specific configuration from the rule chain DSL
	//
	// Returns:
	// Returns:
	//   - error: Initialization error, or nil if successful
	//     error: Initialization error, on successful it is nil
	Init(ruleConfig Config, configuration Configuration) error

	// OnMsg processes incoming messages and implements the component's core functionality.
	// This method is the heart of the component and will be called for each message
	// that flows through this node in the rule chain.
	//
	// OnMsg handles incoming messages and implements the core functions of components.
	// This method is the core of the component and will be called for every message passing through this node in the rule chain.
	//
	// Message Processing Contract:
	// Message Processing Contract:
	//
	//	After processing the message, the component MUST call one of the following
	//	methods to continue the rule chain execution, otherwise the chain will hang:
	//
	//	After processing the message, the component must call one of the following methods to continue the execution of the rule chain; otherwise, the chain will be suspended:
	//	- ctx.TellSuccess(msg): Forward message via "Success" relationship
	//	  ctx.TellSuccess (msg): Forwards messages through the "Success" relationship
	//	- ctx.TellFailure(msg, err): Forward message via "Failure" relationship
	//	  ctx.TellFailure(msg, err): Forwards messages through the "Failure" relationship
	//	- ctx.TellNext(msg, relationTypes...): Forward via specific relationship types
	//	  ctx.TellNext(msg, relationTypes...): Forwarded through specific relationship types
	//	- ctx.DoOnEnd(msg, err, relationType): End this chain branch
	//	  ctx.DoOnEnd(msg, err, relationType): Ends this chain branch
	//
	// Message Modification:
	// Message Modification:
	//	Components can modify message content, metadata, or type before forwarding.
	//	Use message copy methods when modifications might affect parallel processing branches.
	//
	//	Components can modify message content, metadata, or type before forwarding.
	//	When modifications may affect parallel branches, use the message replication method.
	//
	// Asynchronous Processing:
	// Asynchronous processing:
	//	For long-running operations, use ctx.SubmitTask() to execute work in background
	//	goroutines while ensuring proper chain continuation.
	//
	//	For long-term operation, use ctx.SubmitTask() executes work in the backend goroutine,
	//	At the same time, ensure proper chain continuation.
	//
	// Parameters:
	// Parameters:
	//   - ctx: Rule context providing message routing and utility functions
	//     ctx: Provides message routing and the rule context for utility functions
	//   - msg: The message to be processed by this component
	//     msg: The message this component will handle
	OnMsg(ctx RuleContext, msg RuleMsg)

	// Destroy releases any resources held by the component when it's no longer needed.
	// This method is called during rule chain shutdown, component updates, or engine destruction.
	//
	// Destroy releases any resources held by a component when it is no longer needed.
	// This method is called during rule chain closures, component updates, or engine destruction.
	//
	// Cleanup Responsibilities:
	// Cleanup responsibilities:
	//   - Close external connections (HTTP clients, database connections)
	//     Disable external connections (HTTP client, database connection)
	//   - Release file handles and network resources
	//     Release file handles and network resources
	//   - Cancel background goroutines and timers
	//     Disable background goroutines and timers
	//   - Clear internal caches and temporary data
	//     Clear internal caches and temporary data
	//   - Unregister from external services
	//     Deregistered from external services
	//
	// Graceful Shutdown:
	// Graceful Close:
	//	The rule engine ensures that no new messages are sent to OnMsg()
	//	when Destroy() is called. Components can safely clean up resources
	//	without worrying about concurrent access from OnMsg().
	//
	//	The rule engine ensures that no new message is sent to OnMsg() when Destroy() is called.
	//	Components can safely clean resources without worrying about concurrent access from OnMsg().
	//
	// Error Handling:
	// Error handling:
	//	This method should not panic. Log any cleanup errors but don't fail
	//	the entire shutdown process for individual component cleanup failures.
	//
	//	This method should not collapse. Record any cleanup errors, but do not let individual component cleanup failures cause the entire shutdown process to fail.
	//
	// Implementation Notes:
	// Implementation Notes:
	//   - This method may be called multiple times, implement idempotent cleanup
	//     This method may be called multiple times to achieve idempotent cleanup
	//   - Use timeout contexts for cleanup operations to prevent hanging
	//     Use timeout context to prevent suspension during cleanup operations
	//   - Consider implementing a cleanup timeout to avoid blocking shutdown
	//     Consider implementing a clearing timeout to avoid blocking shutdowns
	Destroy()
}

// NodeCtx is the context for instantiating rule nodes.
// NodeCtx is the context for instantiating rule nodes.
//
// NodeCtx extends the basic Node interface with additional context-aware functionality,
// providing access to configuration, debug information, and node management capabilities.
// NodeCtx extends the basic Node interface and adds context-aware features,
// Provides access to configuration, debugging information, and node management functions.
//
// This interface serves as a wrapper around Node instances within the rule engine,
// enabling advanced features like hot reloading, debugging, and hierarchical node access.
// This interface acts as a wrapper for Node instances within the rule engine,
// Enable advanced features such as hot reloading, debugging, and layered node access.
//
// Key Features:
// Key features:
//   - Configuration management and hot reloading
//     Configuration management and thermal heavy loading
//   - Debug mode control for development and monitoring
//     Development and monitoring of debugging mode control
//   - Node identification and metadata access
//     Node identification and metadata access
//   - DSL (Domain Specific Language) configuration access
//     DSL (Domain-Specific Language) configuration access
type NodeCtx interface {
	Node
	Config() Config
	// IsDebugMode checks if the node is in debug mode.
	// True: When messages flow in and out of the node, the config.OnDebug callback function is called; otherwise, it is not.
	// IsDebugMode checks whether the node is in debug mode.
	// True: When messages flow into and out of nodes, call config.OnDebug callback function; Otherwise, it will not be recalled.
	IsDebugMode() bool
	// GetNodeId retrieves the component ID.
	// GetNodeId retrieves the component ID.
	GetNodeId() RuleNodeId
	// ReloadSelf refreshes the configuration of the component.
	// ReloadSelf refreshes component configuration.
	//
	// This method enables hot reloading of component configuration without restarting the entire rule chain.
	// The def parameter should contain the new configuration in the same format as the original DSL.
	// This method enables hot overloading of component configurations without restarting the entire rule chain.
	// The def parameter should include a new configuration in the same format as the original DSL.
	ReloadSelf(def []byte) error
	// GetNodeById retrieves the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	// GetNodeById retrieves the configuration of the specified ID component in the subrule chain.
	// If it is a node type, this method is not supported.
	GetNodeById(nodeId RuleNodeId) (NodeCtx, bool)
	// DSL returns the configuration DSL of the node.
	// DSL returns the node's configuration DSL.
	//
	// The returned byte slice contains the Domain Specific Language definition
	// used to configure this node, typically in JSON format.
	// The returned byte fragment contains domain-specific language definitions used to configure this node, usually in JSON format.
	DSL() []byte
}

// ChainCtx represents the context for rule chain management and execution.
// ChainCtx represents the context for managing and executing rules in chains.
//
// ChainCtx extends NodeCtx with capabilities specific to managing entire rule chains,
// including child node management, rule chain definitions, and engine pool access.
// ChainCtx extends NodeCtx, adding specific functions for managing the entire rule chain,
// Including subnode management, rule chain definition, and engine pool access.
//
// This interface is used for rule chain instances that contain multiple interconnected nodes,
// providing hierarchical management and advanced configuration capabilities.
// This interface is used for rule chain instances containing multiple interconnected nodes,
// Provides layered management and advanced configuration features.
//
// Key Responsibilities:
// Main Responsibilities:
//   - Child node lifecycle management
//     Subnode lifecycle management
//   - Rule chain definition and metadata access
//     Rule chain definition and metadata access
//   - Engine pool integration for resource management
//     Engine pool integration is used for resource management
//   - Hierarchical configuration updates
//     Layered configuration updates
type ChainCtx interface {
	NodeCtx
	// ReloadChild refreshes the configuration of a specified ID component in a sub-rule chain.
	// If it is a node type, this method is not supported.
	// ReloadChild refreshes the configuration of the specified ID component in the subrule chain.
	// If it is a node type, this method is not supported.
	//
	// This method enables fine-grained hot reloading of individual nodes within a rule chain
	// without affecting other nodes or the overall chain structure.
	// This method enables fine-grained hot overloading of individual nodes within the rule chain,
	// It does not affect other nodes or the overall chain structure.
	ReloadChild(nodeId RuleNodeId, def []byte) error
	// Definition returns the definition of the rule chain.
	// Definition Returns the definition of the rule chain.
	//
	// The returned RuleChain contains the complete structural definition,
	// including all nodes, connections, and metadata.
	// The returned RuleChain contains the complete structural definition,
	// Includes all nodes, connections, and metadata.
	Definition() *RuleChain
	// GetRuleEnginePool retrieves the rule engine pool.
	// GetRuleEnginePool retrieves the rule engine pool.
	//
	// The engine pool manages multiple rule engine instances for load balancing
	// and resource optimization in high-concurrency scenarios.
	// The engine pool manages multiple rule engine instances for load balancing in high-concurrency scenarios
	// and resource optimization.
	GetRuleEnginePool() RuleEnginePool
	// AddNodeDependency adds a dependency relationship between nodes.
	// AddNodeDependency Adds dependencies between nodes.
	//
	// This method allows dynamic addition of node dependencies for runtime
	// dependency management and cache optimization.
	// This method allows dynamic addition of node dependencies for runtime
	// Dependency management and caching optimization.
	AddNodeDependency(nodeId string, dependentNodeId string)
	// Resources returns a read-only view of the chain's resource directory for ref:// same-chain parsing (such as the consumer's NetNode and other read-only lookups).
	Resources() ResourceLookup
	// EndpointRegistry returns a writable resource directory (Register/Unregister), only the resource producer
	// (EndpointAspect etc.) is used; The consumer should not write through it (interface isolation ISP).
	EndpointRegistry() ResourceRegistry
}

// NodeRequest request to restore node execution
type NodeRequest struct {
	NodeId string
	// RelationTypes is the list of relation types to find child nodes.
	// If nil, execute the NodeId node itself.
	// If empty, find and execute the child nodes of the NodeId via the default relation.
	// If not empty, find and execute the child nodes of the NodeId via the specified relations.
	RelationTypes []string
	// Msg is the message to be processed by the node.
	// If nil, use the default message.
	Msg *RuleMsg
}

// ExecuteNode creates a NodeRequest to execute the specified node itself.
func ExecuteNode(nodeId string) NodeRequest {
	return NodeRequest{
		NodeId:        nodeId,
		RelationTypes: nil,
	}
}

// ExecuteNodeWithMsg creates a NodeRequest to execute the specified node itself with the specified message.
func ExecuteNodeWithMsg(nodeId string, msg RuleMsg) NodeRequest {
	return NodeRequest{
		NodeId:        nodeId,
		RelationTypes: nil,
		Msg:           &msg,
	}
}

// ExecuteNext creates a NodeRequest to find and execute the child nodes of the specified node.
func ExecuteNext(nodeId string, relationTypes ...string) NodeRequest {
	if relationTypes == nil {
		relationTypes = []string{}
	}
	return NodeRequest{
		NodeId:        nodeId,
		RelationTypes: relationTypes,
	}
}

// ExecuteNextWithMsg creates a NodeRequest to find and execute the child nodes of the specified node with the specified message.
func ExecuteNextWithMsg(nodeId string, msg RuleMsg, relationTypes ...string) NodeRequest {
	if relationTypes == nil {
		relationTypes = []string{}
	}
	return NodeRequest{
		NodeId:        nodeId,
		RelationTypes: relationTypes,
		Msg:           &msg,
	}
}

// RuleContext is the interface for message processing context within the rule engine.
// It handles the transfer of messages to the next or multiple nodes and triggers their business logic.
// It also controls and orchestrates the node flow of the current execution instance.
type RuleContext interface {
	// TellSuccess notifies the rule engine that the current message has been successfully processed and sends the message to the next node via the 'Success' relationship.
	TellSuccess(msg RuleMsg)
	// TellFailure notifies the rule engine that the current message has failed to process and sends the message to the next node via the 'Failure' relationship.
	TellFailure(msg RuleMsg, err error)
	// TellNext sends the message to the next node using the specified relationTypes.
	TellNext(msg RuleMsg, relationTypes ...string)
	// TellSelf sends a message to the current node after a specified delay (in milliseconds).
	TellSelf(msg RuleMsg, delayMs int64)
	// TellNextOrElse sends the message to the next node using the specified relationTypes. If the corresponding relationType does not find the next node, it uses defaultRelationType to search.
	TellNextOrElse(msg RuleMsg, defaultRelationType string, relationTypes ...string)
	// TellFlow executes a sub-rule chain.
	// ruleChainId: The ID of the rule chain.
	// onEndFunc: Callback for when a branch of the sub-rule chain completes, returning the result of that chain. If multiple branches are triggered, it will be called multiple times.
	// onAllNodeCompleted: Callback for when all nodes have completed, with no result returned.
	// If the rule chain is not found, the message is sent to the next node via the 'Failure' relationship.
	TellFlow(ruleChainId string, msg RuleMsg, opts ...RuleContextOption)
	// TellNode starts execution from a specified node. If skipTellNext=true, only the current node is executed without notifying the next node.
	// onEnd is used to view the final execution result.
	TellNode(ctx context.Context, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// TellChainNode executes the specified node in the specified rule chain.
	// If skipTellNext=true, only the current node is executed, and no message is sent to the next node.
	TellChainNode(ctx context.Context, ruleChainId, nodeId string, msg RuleMsg, skipTellNext bool, onEnd OnEndFunc, onAllNodeCompleted func())
	// NewMsg creates a new message instance.
	NewMsg(msgType string, metaData *Metadata, data string) RuleMsg
	// GetSelfId retrieves the current node ID.
	GetSelfId() string
	// Self retrieves the current node instance.
	Self() NodeCtx
	// From retrieves the node instance from which the message entered the current node.
	From() NodeCtx
	// RuleChain retrieves the rule chain instance where the current node resides.
	RuleChain() NodeCtx
	// Config retrieves the configuration of the rule engine.
	Config() Config
	// SubmitTack submits an asynchronous task for execution.
	//Deprecated: Use Flow SubmitTask instead.
	SubmitTack(task func())
	// SubmitTask submits an asynchronous task for execution.
	SubmitTask(task func())
	// SetEndFunc sets the callback function for when the current message processing ends.
	SetEndFunc(f OnEndFunc) RuleContext
	// GetEndFunc retrieves the callback function for when the current message processing ends.
	GetEndFunc() OnEndFunc
	// SetContext sets a context for sharing semaphores or data across different component instances.
	SetContext(c context.Context) RuleContext
	// GetContext retrieves the context for sharing semaphores or data across different component instances.
	GetContext() context.Context
	// SetOnAllNodeCompleted sets the callback for when all nodes have completed execution.
	SetOnAllNodeCompleted(onAllNodeCompleted func())
	// DoOnEnd triggers the OnEnd callback function.
	DoOnEnd(msg RuleMsg, err error, relationType string)
	// SetCallbackFunc sets a callback function.
	SetCallbackFunc(functionName string, f interface{})
	// GetCallbackFunc retrieves a callback function.
	GetCallbackFunc(functionName string) interface{}
	// OnDebug calls the configured OnDebug callback function.
	OnDebug(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// SetExecuteNodes sets the nodes to execute.
	// If multiple nodes are provided, it behaves like restoring from a snapshot.
	// If a single node is provided, it sets the current node execution.
	SetExecuteNodes(nodes ...NodeRequest)
	// TellCollect gathers the execution results from multiple nodes and registers a callback function to collect the result list.
	// If it is the first time to register, it returns true; otherwise, it returns false.
	TellCollect(msg RuleMsg, callback func(msgList []WrapperMsg)) bool
	// GetOut retrieves the OUT message.
	GetOut() RuleMsg
	// GetErr retrieves the IN or OUT error.
	GetErr() error
	// GetRelationTypes retrieves the IN relationTypes
	GetRelationTypes() []string
	// GlobalCache returns a Cache instance for global cache operations
	// The cache items will persist until manually deleted or expired
	GlobalCache() Cache
	// ChainCache returns a Cache instance for rule chain cache operations
	// This is a namespaced version of GlobalCache that automatically prefixes all keys with the rule chain ID
	// The cache items with rule chain prefix will be automatically cleared when the rule chain is destroyed
	ChainCache() Cache
	// GetEnv gets environment variables and metadata from message
	// useMetadata: whether to include metadata in the result
	GetEnv(msg RuleMsg, useMetadata bool) map[string]interface{}
	// SetDebugMode sets per-message debug mode override.
	// When true, debug logging is enabled for this execution regardless of chain/node level settings.
	SetDebugMode(debugMode bool)
	// SetSkipTellNext prevents propagation to successor nodes.
	// When true, only the current node executes and TellNext calls become no-ops.
	SetSkipTellNext(skip bool)
	// GetNodeRuleMsg retrieves the complete RuleMsg of a specific executed node by nodeId
	// Returns the RuleMsg and a boolean indicating if the node was found
	// This method provides access to message data, metadata, and other information
	//
	// IMPORTANT: Node dependency must be established before accessing node output data.
	// The dependency relationship is automatically established when:
	// 1. Using FetchNodeOutputNode component - calls chainCtx.AddNodeDependency() in Init()
	// 2. Manually calling chainCtx.AddNodeDependency(currentNodeId, targetNodeId)
	// 3. Node configuration contains references to other nodes. e.g. ${nodeId.msg.xx} (auto-detected)
	//
	// Only nodes with established dependencies will have their outputs cached and accessible.
	// Without dependency relationship, this method will return (RuleMsg{}, false).
	GetNodeRuleMsg(nodeId string) (RuleMsg, bool)
}

// RuleContextOption is a function type for modifying RuleContext options.
type RuleContextOption func(RuleContext)

// WithEndFunc is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// If an explicit end node is configured in the rule chain, the callback will only be triggered
// when the message flow reaches that specific end node branch.
// Deprecated: Use `types.WithOnEnd` instead.
func WithEndFunc(endFunc func(ctx RuleContext, msg RuleMsg, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(func(ctx RuleContext, msg RuleMsg, err error, relationType string) {
			endFunc(ctx, msg, err)
		})
	}
}

// WithOnEnd is a callback function for when a branch of the rule chain completes.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// If an explicit end node is configured in the rule chain, the callback will only be triggered
// when the message flow reaches that specific end node branch.
func WithOnEnd(endFunc func(ctx RuleContext, msg RuleMsg, err error, relationType string)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetEndFunc(endFunc)
	}
}

// WithContext sets a context for sharing data or semaphores between different component instances.
// It is also used for timeout cancellation.
func WithContext(c context.Context) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetContext(c)
	}
}

// WithOnAllNodeCompleted is a callback function for when the rule chain execution completes.
func WithOnAllNodeCompleted(onAllNodeCompleted func()) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetOnAllNodeCompleted(onAllNodeCompleted)
	}
}

// WithOnRuleChainCompleted is a callback function for when the rule chain execution completes and collects the runtime logs of each node.
func WithOnRuleChainCompleted(onCallback func(ctx RuleContext, snapshot RuleChainRunSnapshot)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnRuleChainCompleted, onCallback)
	}
}

// WithOnNodeCompleted is a callback function for when a node execution completes and collects the node's runtime log.
func WithOnNodeCompleted(onCallback func(ctx RuleContext, nodeRunLog RuleNodeRunLog)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncOnNodeCompleted, onCallback)
	}
}

// WithOnNodeDebug is a callback function for node debug logs, called in real-time asynchronously. It is triggered only if the node is configured with debugMode.
func WithOnNodeDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetCallbackFunc(CallbackFuncDebug, onDebug)
	}
}

// WithDebugMode enables or disables debug mode for a single message execution.
// When set to true, it forces debug logging for this execution regardless of the
// persisted chain/node level debugMode setting.
func WithDebugMode(debugMode bool) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetDebugMode(debugMode)
	}
}

// WithSkipTellNext creates an option that prevents propagation to successor nodes.
func WithSkipTellNext() RuleContextOption {
	return func(rc RuleContext) {
		rc.SetSkipTellNext(true)
	}
}

// WithStartNode sets the first node to start execution.
func WithStartNode(nodeIds ...string) RuleContextOption {
	return func(rc RuleContext) {
		if len(nodeIds) == 0 {
			return
		}
		requests := make([]NodeRequest, len(nodeIds))
		for i, id := range nodeIds {
			requests[i] = ExecuteNode(id)
		}
		rc.SetExecuteNodes(requests...)
	}
}

// WithTellNext is set to find the next or more execution nodes by specifying the node ID. Used to restore the rule chain execution link
// WithTellNext sets the next or multiple execution nodes by specifying the node ID.
// It is used to restore the execution path of the rule chain.
func WithTellNext(fromNodeId string, relationTypes ...string) RuleContextOption {
	return func(rc RuleContext) {
		if fromNodeId == "" {
			return
		}
		rc.SetExecuteNodes(ExecuteNext(fromNodeId, relationTypes...))
	}
}

// WithRestoreNodes sets the nodes to execute.
// You can use types.ExecuteNode(id) or types.ExecuteNext(id, relationTypes...) to create the request.
func WithRestoreNodes(nodes ...NodeRequest) RuleContextOption {
	return func(rc RuleContext) {
		rc.SetExecuteNodes(nodes...)
	}
}

// JsEngine is a JavaScript script engine interface.
// JsEngine is the JavaScript scripting engine interface.
//
// This interface provides an abstraction layer for JavaScript execution within RuleGo components,
// enabling dynamic script execution for data transformation, filtering, and business logic.
// This interface provides an abstraction layer for JavaScript execution within RuleGo components,
// Enable dynamic scripting for data transformation, filtering, and business logic.
//
// The JavaScript engine supports:
// JavaScript engine supports:
//   - Function execution with parameter passing
//     Function execution with parameter passing
//   - Access to RuleContext for message processing
//     Access RuleContext for message processing
//   - Resource management and cleanup
//     Resource management and cleanup
type JsEngine interface {
	// Execute runs a specified function in the JS script, which is initialized when the JsEngine instance is created.
	// ctx is the message chain context.
	// functionName is the name of the function to execute.
	// argumentList is the list of arguments for the function.
	// Execute runs the specified function in the JS script, which is initialized when creating the JsEngine instance.
	// ctx is the context of the message chain.
	// functionName is the name of the function to execute.
	// argumentList is a list of function parameters.
	Execute(ctx RuleContext, functionName string, argumentList ...interface{}) (interface{}, error)
	// Stop releases the resources of the JS engine.
	// Stop releasing JS engine resources.
	//
	// This method should be called when the engine is no longer needed to prevent memory leaks
	// and ensure proper cleanup of JavaScript contexts and associated resources.
	// This method should be called when the engine is no longer needed to prevent memory leaks and ensure proper cleanup of JavaScript context and related resources.
	Stop()
}

// Parser is an interface for parsing rule chain definition files (DSL).
// The default implementation uses JSON. If other formats are used to define rule chains, this interface can be implemented.
// Then register it with the rule engine like this: `rulego.NewConfig(WithParser(&MyParser{})`
// Parser is an interface for parsing rule chain definition files (DSLs).
// By default, JSON is used. If you define the rule chain in another format, you can implement this interface.
// Then register it into the rules engine like this: `rulego.NewConfig(WithParser(&MyParser{})`
//
// This interface enables support for multiple DSL formats, allowing users to define rule chains
// using their preferred configuration language (JSON, YAML, XML, etc.).
// This interface enables support for multiple DSL formats, allowing users to define rule chains using their preferred configuration languages (JSON, YAML, XML, etc.).
type Parser interface {
	// DecodeRuleChain parses a rule chain structure from a description file.
	// DecodeRuleChain parses the rule chain structure from the descriptor.
	DecodeRuleChain(rootRuleChain []byte) (RuleChain, error)
	// DecodeRuleNode parses a rule node structure from a description file.
	// DecodeRuleNode parses the rule node structure from the description file.
	DecodeRuleNode(rootRuleChain []byte) (RuleNode, error)
	// EncodeRuleChain converts a rule chain structure into a description file.
	// EncodeRuleChain converts the rule chain structure into a description file.
	EncodeRuleChain(def interface{}) ([]byte, error)
	// EncodeRuleNode converts a rule node structure into a description file.
	// EncodeRuleNode converts the rule node structure into a description file.
	EncodeRuleNode(def interface{}) ([]byte, error)
}

// Pool is an interface for a coroutine pool.
// Pool is the interface for coroutine pools.
//
// This interface provides an abstraction for managing goroutine pools to control concurrency
// and resource usage in high-throughput scenarios. It enables efficient task scheduling
// and prevents resource exhaustion in concurrent message processing.
// This interface provides an abstraction for managing coroutine pools to control concurrency and resource usage in high-throughput scenarios.
// It enables efficient task scheduling and prevents resource exhaustion in concurrent message processing.
//
// Implementation Characteristics:
// Implementation features:
//   - Fixed or dynamic pool sizing based on load
//     Fixed or dynamic pool size based on load
//   - Task queue management for pending operations
//     Task queue management for pending operations
//   - Graceful shutdown and resource cleanup
//     Gracefully close and clear resources
//   - Load balancing across available workers
//     Load balancing is performed among available workers
//
// Usage Pattern:
// Usage mode:
//
//	pool := NewWorkerPool(maxWorkers)
//	defer pool.Release()
//
//	if err := pool.Submit(func() {
//	    // Task execution
//	}); err != nil {
//	    // Handle pool full or error
//	}
type Pool interface {
	// Submit submits a task to the coroutine pool.
	// Returns an error if the coroutine pool is full.
	// Submit the task to the coroutine pool.
	// If the coroutine pool is full, an error is returned.
	Submit(task func()) error
	// Release releases the resources of the pool.
	// Release: Release: Releases pool resources.
	//
	// This method should be called during application shutdown to ensure
	// all pending tasks are completed and resources are properly cleaned up.
	// This method should be called during application shutdown to ensure all pending tasks are completed and resources are properly cleaned up.
	Release()
}

// EmptyRuleNodeId is an empty node ID.
// EmptyRuleNodeId is an empty node ID.
//
// This constant represents an uninitialized or invalid node identifier,
// commonly used for comparison and validation purposes.
// This constant represents node identifiers that are not initialized or are invalid,
// It is usually used for comparison and verification purposes.
var EmptyRuleNodeId = RuleNodeId{}

// RuleNodeId is a type definition for component IDs.
// RuleNodeId is the type definition of the component ID.
//
// This structure uniquely identifies components within the RuleGo framework,
// combining both identification and type information for proper routing and management.
// This structure uniquely identifies components within the RuleGo framework,
// Combining identification and type information to achieve proper routing and management.
//
// The combination of Id and Type allows the framework to:
// The combination of Id and Type allows the following frameworks:
//   - Distinguish between different component categories
//     Distinguish between different component categories
//   - Route messages to appropriate handlers
//     Routing messages to appropriate handlers
//   - Manage component lifecycles effectively
//     Effectively manage the component lifecycle
//   - Support hierarchical node structures
//     Supports layered node structure
type RuleNodeId struct {
	// Id is the node ID.
	// Id is the node ID.
	//
	// This should be unique within the scope of a rule chain or engine instance.
	// This should be unique within the scope of the rule chain or engine instance.
	Id string
	// Type is the component type, either a node or a sub-rule chain.
	// Type is the component type, which can be a node or a sub-rule chain.
	//
	// This field determines how the component is processed and managed by the engine.
	// This field determines how components are processed and managed by the engine.
	Type ComponentType
}

// RuleNodeRelation defines the relationship between nodes.
// RuleNodeRelation defines the relationships between nodes.
//
// This structure represents the directed connections between components in a rule chain,
// enabling message flow and execution path determination. Relations form the backbone
// of rule chain topology and determine how messages are routed through the system.
// This structure represents the directed connections between components in the rule chain,
// Enable message stream and execution path determination. Relations form the backbone of the rule chain topology,
// And decide how messages are routed through the system.
//
// Key Characteristics:
// Key features:
//   - Directed relationships (from InId to OutId)
//     Directed Relations (from InId to OutId)
//   - Conditional routing based on RelationType
//     Conditional routing based on RelationType
//   - Support for multiple output paths per node
//     Supports multiple output paths per node
//   - Dynamic relationship evaluation during runtime
//     Dynamic relationship evaluation at runtime
type RuleNodeRelation struct {
	// InId is the incoming component ID.
	// InId is the input component ID.
	//
	// This represents the source node from which messages originate.
	// This indicates the source node of the source.
	InId RuleNodeId
	// OutId is the outgoing component ID.
	// OutId is the outgoing component ID.
	//
	// This represents the destination node to which messages are routed.
	// This indicates the message is routed to the target node.
	OutId RuleNodeId
	// RelationType is the type of relationship, such as True, False, Success, Failure, or other custom types.
	// RelationType is a relationship type, such as True, False, Success, Failure, or other custom types.
	//
	// This field determines the condition under which messages flow from InId to OutId.
	// Custom relationship types enable domain-specific routing logic.
	// This field determines the conditions under which messages flow from InId to OutId.
	// Custom relationship types enable domain-specific routing logic.
	RelationType string
}

// ScriptFuncSeparator is the delimiter for script function names.
// ScriptFuncSeparator is the separator for script function names.
//
// This constant is used to separate script type from function name in composite identifiers,
// enabling support for multiple script engines and function namespacing.
// This constant is used to separate script types and function names in composite identifiers,
// Enable support for multiple script engines and function namespaces.
//
// Usage pattern: "scriptType#functionName"
// Usage mode: "scriptType#functionName"
// Example: "Js#processData" or "Lua#filterMessage"
// Example: "Js#processData" or "Lua#filterMessage"
const ScriptFuncSeparator = "#"

// Script is used to register native functions or custom functions defined in Go.
// Scripts are used to register native or custom functions defined in Go.
//
// This structure provides a flexible mechanism for extending RuleGo with custom logic,
// supporting both traditional scripting languages and native Go functions.
// This structure provides a flexible mechanism for extending RuleGo using custom logic,
// Supports both traditional scripting languages and native Go functions.
//
// Script Registration Patterns:
// Script registration mode:
//  1. JavaScript/Lua script content as string
//     JavaScript/Lua script content as a string
//  2. Go function references for direct execution
//     Go function references are used for direct execution
//  3. Plugin-based script loading for dynamic functionality
//     Plugin-based script loading is used for dynamic functionality
//
// Type-Content Mapping:
// Type-Content Mapping:
//   - "Js": JavaScript source code (string)
//     "Js": JavaScript source code (string)
//   - "Lua": Lua source code (string)
//     "Lua": Lua source code (string)
//   - "Go": Go function reference (func interface{})
//     "Go": Go function reference (func interface{})
type Script struct {
	// Type is the script type, default is Js.
	// Type is the script type, default is JS.
	//
	// Supported types include predefined constants (Js, Lua, Python) and custom types.
	// Supported types include predefined constants (Js, Lua, Python) and custom types.
	Type string
	// Content is the script content or custom function.
	// Content refers to script content or custom functions.
	//
	// The content type varies based on the script Type:
	// Content types vary depending on the script type:
	//   - String: Script source code for interpreted languages
	//     String: Script source code for the language
	//   - Function: Go function reference for native execution
	//     Function: A native Go function reference
	//   - []byte: Compiled bytecode for optimized execution
	//     []byte: Optimizes the bytecode for compile execution
	Content interface{}
}

// Callbacks is a set of callback functions for pool events.
// Callbacks are a set of callback functions for pool events.
//
// This structure provides event-driven notifications for rule chain and component lifecycle events,
// enabling monitoring, logging, and integration with external systems.
// This structure provides event-driven notifications for rule chains and component lifecycle events,
// Enable monitoring, logging, and integration with external systems.
//
// Event Lifecycle:
// Event Lifecycle:
//  1. OnNew: Triggered when new rule chains are created
//     OnNew: Triggered when creating a new rule chain
//  2. OnUpdated: Triggered when existing components are modified
//     OnUpdated: Triggers when modifying existing components
//  3. OnDeleted: Triggered when components are removed
//     OnDeleted: Triggered when deleting a component
//
// Use Cases:
// Use Cases:
//   - Audit logging for configuration changes
//     Configure the audit log of the change
//   - Cache invalidation for updated components
//     Cache failure of update components
//   - Metrics collection for monitoring systems
//     Monitoring system metrics collection
//   - External system synchronization
//     External system synchronization
type Callbacks struct {
	// OnNew is called when a new rule chain is created.
	// OnNew is called when creating a new chain of rules.
	//
	// Parameters:
	// Parameters:
	//   - chainId: Unique identifier of the new rule chain
	//     chainId: The unique identifier for the new rule chain
	//   - dsl: Complete DSL definition of the rule chain
	//     dsl: The complete DSL definition of the rule chain
	OnNew func(chainId string, dsl []byte)

	// OnUpdated is called when an existing component is updated.
	// OnUpdated is called when updating existing components.
	//
	// Parameters:
	// Parameters:
	//   - chainId: Identifier of the parent rule chain
	//     chainId: The identifier of the parent rule chain
	//   - nodeId: Identifier of the updated component
	//     nodeId: The identifier for the updated component
	//   - dsl: Updated DSL definition of the component
	//     dsl: Component update DSL definition
	OnUpdated func(chainId, nodeId string, dsl []byte)

	// OnDeleted is called when a component or rule chain is deleted.
	// OnDeleted is called when deleting components or rule chains.
	//
	// Parameters:
	// Parameters:
	//   - id: Identifier of the deleted entity (chain or node)
	//     id: The identifier of the deleted entity (chain or node)
	OnDeleted func(id string)
}
