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

package types

// RuleChain defines a rule chain.
// RuleChain defines the rule chain.
//
// RuleChain is the top-level structure that represents a complete rule processing workflow.
// It contains both basic information about the chain and detailed metadata about its
// internal structure, including nodes, connections, and routing configurations.
// RuleChain is a top-level structure representing a complete rule-handling workflow.
// It contains basic information about the chain and detailed metadata about its internal structure, including nodes, connections, and routing configurations.
//
// Structure Overview:
// Structure Overview:
//   - RuleChain: Basic information (ID, name, configuration)
//     RuleChain: Basic information (ID, name, configuration)
//   - Metadata: Structural details (nodes, connections, endpoints)
//     Metadata: detailed structure information (nodes, connections, endpoints)
//
// Usage in DSL:
// Use in DSL:
//
//	{
//	  "ruleChain": {
//	    "id": "temperature_monitoring",
//	    "name": "Temperature Monitoring Chain",
//	    "debugMode": true
//	  },
//	  "metadata": {
//	    "firstNodeIndex": 0,
//	    "nodes": [...],
//	    "connections": [...]
//	  }
//	}
type RuleChain struct {
	// RuleChain contains the basic information of the rule chain.
	// RuleChain contains the basic information of the rule chain.
	RuleChain RuleChainBaseInfo `json:"ruleChain"`
	// Metadata includes information about the nodes and connections within the rule chain.
	// Metadata contains information about nodes and connections within the rule chain.
	Metadata RuleMetadata `json:"metadata"`
}

func (r RuleChain) GetNode(nodeId string) (*RuleNode, bool) {
	for _, item := range r.Metadata.Nodes {
		if item.Id == nodeId {
			return item, true
		}
	}
	return nil, false
}

// RuleChainBaseInfo defines the basic information of a rule chain.
// RuleChainBaseInfo defines the basic information of the rule chain.
//
// This structure contains the essential metadata and configuration that applies
// to the entire rule chain. It provides identification, behavioral settings,
// and extensibility through additional information fields.
// This structure contains the basic metadata and configurations applicable to the entire rule chain.
// It provides identification, behavior settings, and scalability through additional information fields.
//
// Key Features:
// Key features:
//   - Unique identification with ID and Name
//     Use unique identifiers for ID and Name
//   - Global debug mode control
//     Global debugging mode control
//   - Hierarchical chain organization (root/sub-chain)
//     Layered chain organization (root/child chain)
//   - Runtime state management (enabled/disabled)
//     Runtime State Management (Enable/Disable)
//   - Flexible configuration and extension support
//     Flexible configuration and expansion support
type RuleChainBaseInfo struct {
	// ID is the unique identifier of the rule chain.
	// The ID is the unique identifier of the rule chain.
	//
	// The ID must be unique within the rule engine context and is used for
	// chain references, sub-chain invocation, and management operations.
	// The ID must be unique in the context of the rule engine and is used for chain references, subchain calls, and management operations.
	ID string `json:"id"`

	// Name is the name of the rule chain.
	// Name is the name of the rule chain.
	//
	// The name provides a human-readable identifier for the chain, useful
	// for UI display, logging, and administrative purposes.
	// The name provides a human-readable identifier for the chain, which is useful for UI display, logging, and management purposes.
	Name string `json:"name"`

	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting overrides the `DebugMode` configuration of the node.
	// DebugMode indicates whether the node is in debug mode. If true, the node triggers the debug callback function when processing the message.
	// This setting overrides the node's `DebugMode` configuration.
	//
	// Global Debug Control:
	// Global Debugging and Control:
	// When enabled at the chain level, debug mode affects all nodes within the chain,
	// regardless of individual node debug settings. This provides convenient
	// chain-wide debugging control.
	// When enabled at the chain level, debug mode affects all nodes within the chain,
	// Regardless of the debugging settings of individual nodes. This provides convenient chain-wide debugging control.
	DebugMode bool `json:"debugMode"`

	// Root indicates whether this rule chain is a root or a sub-rule chain. (Used only as a marker, not applied in actual logic)
	// Root indicates whether the rule chain is the root chain or the subchain rule. (Used only as a marker, not applied in actual logic)
	//
	// This field serves as organizational metadata to help distinguish between
	// main processing chains and subsidiary chains in complex rule hierarchies.
	// This field serves as organizational metadata, helping to distinguish between primary processing chains and auxiliary chains within a complex hierarchy of rules.
	Root bool `json:"root"`

	// Disabled indicates whether the rule chain is disabled.
	// Disabled indicates whether the rule chain is disabled.
	//
	// When disabled, the rule chain will not process messages and can be used
	// for maintenance, testing, or gradual rollout scenarios.
	// When disabled, the rule chain does not process messages and can be used for maintenance, testing, or incremental rollout scenarios.
	Disabled bool `json:"disabled"`

	// Configuration contains the configuration information of the rule chain.
	// Configuration contains configuration information for the rule chain.
	//
	// This flexible configuration map allows chain-level settings that can
	// be accessed by nodes within the chain for shared configuration data.
	// This flexible configuration mapping allows chain-level settings, which on-chain nodes can access to access for shared configuration data.
	Configuration Configuration `json:"configuration,omitempty"`

	// AdditionalInfo is an extension field.
	// AdditionalInfo is an extension field.
	//
	// This field provides extensibility for custom metadata, UI information,
	// or integration-specific data without modifying the core structure.
	// This field provides scalability for custom metadata, UI information, or integrating specific data,
	// No modifications to the core structure are required.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// GetAdditionalInfo retrieves additional information by key.
// GetAdditionalInfo retrieves additional information via key.
//
// This method provides safe access to additional information with existence checking.
// It returns both the value and a boolean indicating whether the key was found.
// This method provides secure access to additional information and includes presence checks.
// It returns a value and a boolean value, indicating whether the key has been found.
//
// Parameters:
// Parameters:
//   - key: The key to look up in additional information
//     key: The key to look for in the additional information
//
// Returns:
// Returns:
//   - interface{}: The value associated with the key, or empty string if not found
//     interface{}: The value associated with the key; if not found, it is an empty string
//   - bool: True if the key exists, false otherwise
//     bool: If the key exists, it is true; otherwise, it is false
func (r RuleChainBaseInfo) GetAdditionalInfo(key string) (interface{}, bool) {
	if r.AdditionalInfo == nil {
		return "", false
	}
	v, ok := r.AdditionalInfo[key]
	return v, ok
}

// PutAdditionalInfo adds additional information by key and value.
// PutAdditionalInfo adds additional information through keys and values.
//
// This method safely adds or updates additional information, automatically
// initializing the map if it doesn't exist.
// This method securely adds or updates additional information, and automatically initializes if the mapping does not exist.
//
// Parameters:
// Parameters:
//   - key: The key to store the information under
//     key: The key used to store information
//   - value: The value to associate with the key
//     value: The value associated with the key
//
// Usage:
// Usage:
//
//	chainInfo.PutAdditionalInfo("version", "1.0.0")
//	chainInfo.PutAdditionalInfo("author", "admin")
//	chainInfo.PutAdditionalInfo("lastModified", time.Now())
func (r RuleChainBaseInfo) PutAdditionalInfo(key string, value interface{}) {
	if r.AdditionalInfo == nil {
		r.AdditionalInfo = make(map[string]interface{})
	}
	r.AdditionalInfo[key] = value
}

// RuleMetadata defines the metadata of a rule chain, including information about nodes and connections.
// RuleMetadata defines the metadata of the rule chain, including information about nodes and connections.
//
// This structure contains the detailed topology and routing information that defines
// how messages flow through the rule chain. It includes node definitions, connections
// between nodes, endpoint configurations, and legacy sub-chain connections.
// This structure contains detailed topology and routing information that defines how messages flow through the chain of rules.
// It includes node definition, inter-node connections, endpoint configuration, and traditional subchain connections.
//
// Structural Components:
// Structural components:
//   - FirstNodeIndex: Entry point identification
//     FirstNodeIndex: Entry point identifier
//   - Endpoints: External connectivity configuration
//     Endpoints: External connection configuration
//   - Nodes: Processing component definitions
//     Nodes: Handles component definitions
//   - Connections: Inter-node message routing
//     Connections: Message routing between nodes
//   - RuleChainConnections: Legacy sub-chain integration
//     RuleChainConnections: Traditional subchain integration
type RuleMetadata struct {
	// FirstNodeIndex is the index of the first node in data flow, default is 0.
	// FirstNodeIndex is the index of the first node in the data stream, defaulting to 0.
	//
	// This index identifies the entry point for message processing within the rule chain.
	// It corresponds to the position in the Nodes array where execution begins.
	// This index identifies the entry point for message processing within the rule chain.
	// It corresponds to the position in the array of nodes where execution begins.
	FirstNodeIndex int `json:"firstNodeIndex"`

	// Endpoints are the component definitions of the endpoints.
	// Endpoints are the component definitions of endpoints.
	//
	// Endpoints define external connectivity points for the rule chain, including
	// REST APIs, MQTT brokers, WebSocket servers, and other protocol handlers.
	// They serve as the bridge between external systems and the rule processing logic.
	// Endpoints define external connection points of the rule chain, including REST API, MQTT proxies, WebSocket servers, and other protocol handlers.
	// They serve as bridges between external systems and the logic of rule processing.
	Endpoints []*EndpointDsl `json:"endpoints,omitempty"`

	// Nodes are the component definitions of the nodes.
	// Each object represents a rule node within the rule chain.
	// Nodes are the component definitions of nodes.
	// Each object represents a rule node within the rule chain.
	//
	// Nodes define the processing components that transform, filter, route, and
	// act upon messages as they flow through the rule chain. Each node encapsulates
	// specific business logic or integration functionality.
	// Nodes define processing components that transform, filter, route, and operate messages as they flow through the rule chain.
	// Each node encapsulates specific business logic or integration functions.
	Nodes []*RuleNode `json:"nodes"`

	// Connections define the connections between two nodes in the rule chain.
	// Connections defines the connection between two nodes in a rule chain.
	//
	// Connections establish the message flow topology by specifying how messages
	// move from one node to another based on processing results and relationship types.
	// Connections establish the message stream topology by specifying how messages move from one node to another based on processing results and relationship types.
	Connections []NodeConnection `json:"connections"`

	// Deprecated: Use Flow Node instead.
	// RuleChainConnections are the connections between a node and a sub-rule chain.
	// Deprecated: Switched to Flow Node.
	// RuleChainConnections are the connections between nodes and subrule chains.
	//
	// This field is maintained for backward compatibility with older rule chain
	// definitions that use direct sub-chain connections instead of Flow nodes.
	// This field maintains backward compatibility with the old rule chain definition that uses direct child chain connections instead of Flow nodes.
	RuleChainConnections []RuleChainConnection `json:"ruleChainConnections,omitempty"`
}

// RuleNode defines the information of a rule chain node.
// RuleNode defines information about rule chain nodes.
//
// RuleNode represents a single processing component within a rule chain. Each node
// encapsulates specific business logic, data transformation, or integration functionality.
// Nodes are connected through relationships to form complete processing workflows.
// RuleNode represents a single processing component within the rule chain. Each node encapsulates specific business logic,
// Data conversion or integration capabilities. Nodes form a complete processing workflow through relational connections.
//
// Node Lifecycle:
// Node Lifecycle:
//  1. Configuration parsing and validation
//     Configuration analysis and verification
//  2. Component initialization with configuration
//     Component initialization is performed using configuration
//  3. Message processing during rule execution
//     Message processing during rule execution
//  4. Resource cleanup when node is destroyed
//     Resource cleanup during node destruction
type RuleNode struct {
	// Id is the unique identifier of the node, which can be any string.
	// An ID is a unique identifier for a node and can be any string.
	//
	// The ID must be unique within the rule chain and is used for:
	// The ID must be unique within the rule chain and is used for:
	//   - Node references in connections
	//     Node references in the connection
	//   - Debug and monitoring identification
	//     Debugging and monitoring identification
	//   - Dynamic node lookup and management
	//     Dynamic node search and management
	//   - Error reporting and logging
	//     Error reporting and logging
	Id string `json:"id"`

	// AdditionalInfo is an extension field for visualization position information (reserved field).
	// AdditionalInfo is an extended field (reserved field) used to visualize location information.
	//
	// This field is primarily used by visual rule chain editors to store
	// UI-specific information such as node positioning, styling, and metadata
	// that doesn't affect rule execution but aids in visual representation.
	// This field is mainly used by the Visual Rule Chain Editor to store UI-specific information,
	// Such as node positioning, styles, and metadata that do not affect rule execution but help visualize the representation.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`

	// Type is the type of the node, which determines the logic and behavior of the node. It should match one of the node types registered in the rule engine.
	// Type is the type of node, determining its logic and behavior. It should match one of the node types registered in the rule engine.
	//
	// The type serves as a factory key to create the appropriate node implementation.
	// Common types include filters, transformers, actions, and integrations.
	// Types act as factory keys to create appropriate node implementations.
	// Common types include filters, converters, action, and integration.
	//
	// Standard Node Types:
	// Standard Node Types:
	//   - jsFilter: JavaScript-based filtering logic
	//     jsFilter: JavaScript-based filtering logic
	//   - jsTransform: JavaScript-based data transformation
	//     jsTransform: JavaScript-based data transformation
	//   - restApiCall: HTTP REST API integration
	//     restApiCall: HTTP REST API integration
	//   - log: Logging and debugging output
	//     log: Log recording and debugging output
	Type string `json:"type"`

	// Name is the name of the node, which can be any string.
	// Name is the name of a node, which can be any string.
	//
	// The name provides a human-readable identifier for the node, useful for:
	// The name provides a human-readable identifier for nodes, used for:
	//   - Documentation and understanding
	//     Documentation and understanding
	//   - Visual representation in editors
	//     Visualization in the editor
	//   - Debugging and error messages
	//     Debugging and error messages
	//   - Administrative and monitoring purposes
	//     Management and monitoring purposes
	Name string `json:"name"`

	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting can be overridden by the RuleChain `DebugMode` configuration.
	// DebugMode indicates whether the node is in debug mode. If true, the node triggers the debug callback function when processing the message.
	// This setting can be overridden by the RuleChain `DebugMode` configuration.
	//
	// Debug Mode Benefits:
	// Benefits of debugging modes:
	//   - Real-time message flow visibility
	//     Real-time message stream visibility
	//   - Performance monitoring and profiling
	//     Performance monitoring and analysis
	//   - Error tracking and diagnostics
	//     Error tracking and diagnosis
	//   - Development and testing support
	//     Development and testing support
	//
	// Performance Considerations:
	// Performance considerations:
	// Debug callbacks add overhead, so disable in production unless monitoring is required.
	// Debugging and callbacks increase overhead, so unless monitoring is required, they are disabled in production environments.
	DebugMode bool `json:"debugMode"`

	// Configuration contains the configuration parameters of the node, which vary depending on the node type.
	// For example, a JS filter node might have a `jsScript` field defining the filtering logic,
	// while a REST API call node might have a `restEndpointUrlPattern` field defining the URL to call.
	// Configuration contains the configuration parameters of nodes, which vary depending on the node type.
	// For example, a JS filter node may have a `jsScript` field that defines the filtering logic,
	// REST API call nodes may have a `restEndpointUrlPattern` field that defines the URL to be called.
	//
	// Configuration supports:
	// Configuration support:
	//   - Type-specific parameters for node behavior
	//     Specific type parameters for node behavior
	//   - Environment variable substitution (${global.key})
	//     Environment variable replacement (${global.key})
	//   - Dynamic configuration updates
	//     Dynamic configuration updates
	//
	Configuration Configuration `json:"configuration"`
}

func (n RuleNode) GetAdditionalInfo(key string) (interface{}, bool) {
	if n.AdditionalInfo == nil {
		return "", false
	}
	v, ok := n.AdditionalInfo[key]
	return v, ok
}

// NodeAdditionalInfo is used for visualization position information (reserved field).
// NodeAdditionalInfo is used to visualize location information (reserved fields).
//
// This structure defines the standard additional information fields used by
// visual rule chain editors for node positioning and documentation.
// This structure defines standard additional information fields for node positioning and documentation in the Visual Rule Chain Editor.
type NodeAdditionalInfo struct {
	// Description provides detailed documentation for the node
	// Description provides detailed documentation for nodes
	Description string `json:"description"`
	// LayoutX represents the horizontal position in the visual editor
	// LayoutX represents the horizontal position in the visual editor
	LayoutX int `json:"layoutX"`
	// LayoutY represents the vertical position in the visual editor
	// LayoutY represents the vertical position in the visual editor
	LayoutY int `json:"layoutY"`
}

// NodeConnection defines the connection between two nodes in a rule chain.
// NodeConnection defines the connection between two nodes in a rule chain.
//
// NodeConnection establishes the message flow topology by specifying how messages
// move from one processing node to another based on the results of message processing.
// The connection type determines the conditions under which messages flow.
// NodeConnection establishes the message stream topology by specifying how messages move from one processing node to another based on the result of message processing.
// The connection type determines the conditions for message flow.
//
// Connection Flow Logic:
// Connection flow logic:
//  1. Source node processes message
//     Source nodes process messages
//  2. Processing result determines relationship type
//     The outcome of the process determines the type of relationship
//  3. Message is routed to target node if relationship matches
//     If the relationship matches, the message is routed to the target node
//  4. Multiple connections enable parallel or conditional flows
//     Multiple connections support parallel or conditional flow
//
// Common Connection Types:
// Common connection types:
//   - Success/Failure: General processing outcomes
//     Success/Failure: General handling results
//   - True/False: Boolean logic for filters and conditions
//     True/False: Boolean logic for filters and conditions
//   - Custom types: Domain-specific routing logic
//     Custom Type: Domain-specific routing logic
type NodeConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	// FromId is the ID of the source node and should match the node ID in the node array.
	//
	// This field establishes the starting point of the message flow connection.
	// The referenced node must exist in the rule chain's node list.
	// This field establishes the starting point of the message stream connection.
	// The referenced node must exist in the node list of the rule chain.
	FromId string `json:"fromId"`

	// ToId is the id of the target node, which should match the id of a node in the nodes array.
	// ToId is the ID of the target node and should match the node ID in the node array.
	//
	// This field establishes the destination of the message flow connection.
	// The referenced node must exist in the rule chain's node list.
	// This field establishes the destination of the message stream connection.
	// The referenced node must exist in the node list of the rule chain.
	ToId string `json:"toId"`

	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	// For example, a JS filter node might support two connection types: "True" and "False," indicating whether the message passes or fails the filter condition.
	// Type is the connection type, which determines when and how to send messages from one node to another. It should match one of the connection types supported by the source node type.
	// For example, a JS filter node may support two connection types: "True" and "False"," indicating whether the message passed or failed the filtering condition.
	//
	// The type acts as a conditional gate that controls message flow based on
	// processing results. Each node type defines its own set of supported relationship types.
	// type as a condition gate for controlling the message stream based on processing results.
	// Each node type defines its own set of relationship types it supports.
	Type string `json:"type"`

	// Label is the label of the connection, used for display.
	// Label is a connected label used for display.
	//
	// The label provides a human-readable description of the connection,
	// useful for visual editors and documentation purposes.
	// Tags provide a human-readable description of the connection,
	// Very useful for visual editors and document purposes.
	Label string `json:"label,omitempty"`
}

// RuleChainConnection defines the connection between a node and a sub-rule chain.
// RuleChainConnection defines the connection between nodes and sub-rule chains.
//
// This structure represents the legacy way of connecting to sub-rule chains
// directly. Modern implementations should use Flow nodes instead for better
// flexibility and consistency.
// This structure represents the traditional way of directly connecting to the sub-rule chain.
// Modern implementations should use Flow nodes for better flexibility and consistency.
type RuleChainConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	// FromId is the ID of the source node and should match the node ID in the node array.
	FromId string `json:"fromId"`
	// ToId is the id of the target sub-rule chain, which should match one of the sub-rule chains registered in the rule engine.
	// ToId is the ID of the target subrule chain and should match one of the subrule chains registered in the rule engine.
	ToId string `json:"toId"`
	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	// Type is the connection type, which determines when and how to send messages from one node to another. It should match one of the connection types supported by the source node type.
	Type string `json:"type"`
}

// RuleChainRunSnapshot is a snapshot of the rule chain execution log.
// RuleChainRunSnapshot is a snapshot of the execution log of the rule chain.
//
// This structure captures the complete execution trace of a rule chain run,
// including timing information, node execution logs, and metadata.
// It's primarily used for debugging, monitoring, and audit purposes.
// This structure captures the complete execution trace of the rule chain's execution,
// Including time information, node execution logs, and metadata.
// It is mainly used for commissioning, monitoring, and auditing purposes.
//
// Snapshot Use Cases:
// Snapshot use case:
//   - Performance analysis and optimization
//     Performance analysis and optimization
//   - Error investigation and debugging
//     Bug investigation and debugging
//   - Audit trails for compliance
//     Compliance audit trails
//   - Execution monitoring and alerting
//     Monitoring and alerts are enforced
type RuleChainRunSnapshot struct {
	// Deprecated: User ctx.RuleChain() instead.
	// Deprecated: Use ctx.RuleChain() instead.
	RuleChain
	// Id is the execution ID.
	// Id is the execution ID.
	//
	// This unique identifier tracks a specific rule chain execution instance,
	// enabling correlation across distributed systems and log aggregation.
	// This unique identifier tracks specific rule chain execution instances,
	// Supports association and log aggregation across distributed systems.
	Id string `json:"id"`
	// StartTs is the start time of execution.
	// StartTs is the execution start time.
	//
	// Timestamp when the rule chain execution began, used for performance
	// measurement and timing analysis.
	// Timestamps for the start of rule chain execution, used for performance measurement and timing analysis.
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	// EndTs is the execution end time.
	//
	// Timestamp when the rule chain execution completed, used for calculating
	// total execution duration and performance analysis.
	// Timestamp of rule chain execution completion, used to calculate total execution duration and performance analysis.
	EndTs int64 `json:"endTs"`
	// Logs are the logs for each node.
	// Logs are logs for each node.
	//
	// Detailed execution logs for each node that processed messages during
	// this rule chain run, providing fine-grained visibility into the execution flow.
	// During the execution of this rule chain, detailed execution logs are recorded for each node handling messages,
	// Provides granular visibility into execution flows.
	Logs []RuleNodeRunLog `json:"logs"`
	// AdditionalInfo is an extension field.
	// AdditionalInfo is an extension field.
	//
	// Extensible field for storing custom metadata, monitoring data,
	// or integration-specific information related to this execution.
	// Extensible fields used to store custom metadata, monitoring data, or integrating specific information related to this execution.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// RuleNodeRunLog is the log for a node.
// RuleNodeRunLog is a log of a node.
//
// This structure captures detailed execution information for a single node
// during rule chain processing, including input/output messages, timing,
// errors, and custom log entries.
// This structure captures detailed execution information of individual nodes during rule chain processing,
// Includes input/output messages, timing, errors, and custom log entries.
//
// Log Analysis Applications:
// Log Analysis Applications:
//   - Performance bottleneck identification
//     Performance bottleneck identification
//   - Message transformation tracking
//     Message conversion tracking
//   - Error pattern analysis
//     Error pattern analysis
//   - Compliance and audit reporting
//     Compliance and audit reports
type RuleNodeRunLog struct {
	// Id is the node ID.
	// Id is the node ID.
	//
	// Identifier of the node that generated this log entry,
	// corresponding to the node's ID in the rule chain definition.
	// Generate node identifiers for this log entry,
	// Corresponds to the node ID in the rule chain definition.
	Id string `json:"nodeId"`
	// InMsg is the input message.
	// InMsg is the input message.
	//
	// The message that was received by this node for processing,
	// capturing the state before node execution.
	// This node receives messages used for processing,
	// Captures the state of a node before execution.
	InMsg RuleMsg `json:"inMsg"`
	// OutMsg is the output message.
	// OutMsg is the output message.
	//
	// The message that was produced by this node after processing,
	// showing any transformations or modifications made.
	// The message generated by this node after processing,
	// Displays any changes or transformations made.
	OutMsg RuleMsg `json:"outMsg"`
	// RelationType is the connection type with the next node.
	// RelationType is the connection type to the next node.
	//
	// The relationship type that determined the routing of the output message
	// to subsequent nodes in the rule chain.
	// Determines the type of relationship for routing output messages to subsequent nodes in the rule chain.
	RelationType string `json:"relationType"`
	// Err is the error information.
	// ERR is misinformation.
	//
	// Textual representation of any error that occurred during node execution,
	// useful for debugging and error analysis.
	// Any error that occurs during node execution is represented in text,
	// It is useful for debugging and error analysis.
	Err string `json:"err"`
	// LogItems are the logs during execution.
	// LogItems are logs during execution.
	//
	// Custom log entries generated by the node during processing,
	// providing detailed visibility into internal operations.
	// Custom log entries generated by nodes during processing,
	// Provides detailed visibility into internal operations.
	LogItems []string `json:"logItems"`
	// StartTs is the start time of execution.
	// StartTs is the execution start time.
	//
	// Timestamp when this node began processing the message.
	// This node begins processing the message timestamp.
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	// EndTs is the execution end time.
	//
	// Timestamp when this node completed processing the message,
	// used for calculating node-level execution duration.
	// This node completes the timestamp of the message processing,
	// Used to calculate the duration of node-level execution.
	EndTs int64 `json:"endTs"`
}

// EndpointDsl defines the DSL for an endpoint.
// EndpointDsl defines the DSL of the endpoint.
//
// EndpointDsl extends RuleNode with endpoint-specific functionality, combining
// node behavior with external connectivity capabilities. It serves as the bridge
// between external systems and rule chain processing logic.
// EndpointDsl extends RuleNode, adding endpoint-specific features that combine
// Node behavior and external connection functions. It serves as a bridge between the external system and the logic of the rule chain processing.
//
// Endpoint Architecture:
// Endpoint architecture:
//   - Node foundation: Inherits all RuleNode capabilities
//     Node Basics: Inherits all RuleNode functionality
//   - Protocol handling: Support for various communication protocols
//     Protocol Processing: Supports various communication protocols
//   - Request routing: Flexible routing based on request characteristics
//     Request routing: Flexible routing based on request characteristics
//   - Processing pipeline: Configurable processors for request/response handling
//     Processing pipeline: a configurable processor for request/response processing
//
// Supported Protocols:
// Supported protocols:
//   - HTTP/HTTPS: REST APIs, webhooks, web services
//     HTTP/HTTPS: REST API, webhooks, web services
//   - MQTT: IoT messaging and device communication
//     MQTT: IoT Messaging and Device Communication
//   - WebSocket: Real-time bidirectional communication
//     WebSocket: Real-time two-way communication
//   - TCP/UDP: Custom protocol implementations
//     TCP/UDP: custom protocol implementation
type EndpointDsl struct {
	RuleNode
	// Processors is the list of global processors for the endpoint.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors are a list of global processors at the endpoint.
	// Use processors registered by name in builtin/processor#Builtins xx.
	//
	// Processors provide pre/post-processing capabilities for endpoint requests,
	// enabling cross-cutting concerns like authentication, validation, logging,
	// and format conversion to be applied consistently across all routes.
	// The processor provides pre/post-processing functions for endpoint requests,
	// Enables cross-section of concerns such as authentication, validation, logging, and format conversion
	// Consistent application across all routes.
	//
	// Common Global Processors:
	// Common global processors:
	//   - auth: Authentication and authorization
	//     auth: Certification and authorization
	//   - validate: Request validation and sanitization
	//     validate: Requests for validation and cleanup
	//   - cors: Cross-Origin Resource Sharing handling
	//     cors: Cross-domain resource sharing processing
	//   - rateLimit: Request rate limiting and throttling
	//     rateLimit: Request rate limit and throttling
	//   - compress: Response compression
	//     compress: response compression
	Processors []string `json:"processors"`

	// Routers is the list of routers.
	// Routers is a list of routers.
	//
	// Routers define the specific routing rules that determine how incoming
	// requests are matched and processed. Each router can have its own
	// configuration, processors, and destination rules.
	// Routers define specific routing rules that determine how incoming requests are matched and handled.
	// Each router can have its own configuration, processor, and target rules.
	//
	// Router Organization:
	// Router organization:
	//   - Pattern matching: URL patterns, method filtering, header matching
	//     Pattern matching: URL patterns, method filtering, header matching
	//   - Parameter extraction: Path variables, query parameters
	//     Parameter extraction: path variables, query parameters
	//   - Content handling: Request/response format negotiation
	//     Content Processing: Negotiation of request/response formats
	//   - Conditional routing: Dynamic routing based on request content
	//     Conditional routing: dynamic routing based on the content of the request
	Routers []*RouterDsl `json:"routers"`
}

// RouterDsl defines a router for an endpoint.
// RouterDsl defines routers with endpoints.
//
// RouterDsl represents a single routing rule within an endpoint that defines
// how specific requests should be processed and where they should be forwarded.
// It provides fine-grained control over request matching, processing, and routing.
// RouterDsl represents a single routing rule within an endpoint, defining how specific requests should be handled and where they should be forwarded.
// It provides fine-grained control over request matching, processing, and routing.
//
// Routing Flow:
// Routing process:
//  1. Request arrives at endpoint
//     Request to reach the endpoint
//  2. Router parameters match request characteristics
//     Router parameters match request characteristics
//  3. From configuration processes the request
//     From configuration to handle requests
//  4. Message is forwarded to To destination
//     Messages are forwarded to To targets
//  5. Response is processed and returned
//     Responses are processed and returned
//
// Routing Strategies:
// Routing strategy:
//   - Path-based: Route by URL path patterns
//     Path-based: Route by URL path mode
//   - Method-based: Route by HTTP methods (GET, POST, etc.)
//     Method-based: routing by HTTP method (GET, POST, etc.)
//   - Header-based: Route by request headers or content types
//     Header-based: Route by request header or content type
//   - Content-based: Route by request body content
//     Content-based: routing content according to the requested body content
type RouterDsl struct {
	// Id is the router ID, optional and by default uses From.Path.
	// Id is the router ID, optional, default uses From.Path.
	//
	// The ID provides a unique identifier for the router within the endpoint,
	// useful for debugging, monitoring, and dynamic router management.
	// If not specified, the system uses the From.Path as the identifier.
	// ID provides a unique identifier for routers within the endpoint,
	// It is useful for debugging, monitoring, and dynamic router management.
	// If not specified, the system uses From.Path as the identifier.
	Id string `json:"id"`

	// Params is the parameters for the router.
	// HTTP Endpoint router params is POST/GET/PUT...
	// Params are the parameters of the router.
	// The HTTP endpoint router parameters are POST/GET/PUT...
	//
	// Parameters define the matching criteria for incoming requests.
	// The format and meaning of parameters depend on the endpoint type:
	// Parameters define the matching conditions for the incoming request.
	// The format and meaning of parameters depend on the endpoint type:
	//
	// HTTP Parameters:
	// HTTP parameters:
	//   - HTTP methods: ["GET", "POST", "PUT", "DELETE"]
	//     HTTP method: ["GET", "POST", "PUT", "DELETE"]
	//   - Content types: ["application/json", "text/plain"]
	//     Content type: ["application/json", "text/plain"]
	//
	// MQTT Parameters:
	// MQTT parameters:
	//   - QoS levels: [0, 1, 2]
	//     QoS level: [0, 1, 2]
	//   - Retained flag: [true, false]
	//     Reserve flag: [true, false]
	Params []interface{} `json:"params"`

	// From is the source for the router.
	// From is the source of the router.
	//
	// The From configuration defines how incoming requests are received,
	// processed, and prepared for routing to the destination.
	// The From configuration defines how to receive, process incoming requests, and prepare them for routing to the destination.
	From FromDsl `json:"from"`

	// To is the destination for the router.
	// To is the router's target.
	//
	// The To configuration defines where processed requests should be forwarded
	// and how responses should be handled and returned to the client.
	// To configure the definition of where the processed request should be forwarded, how to handle the response, and how to return it to the client.
	To ToDsl `json:"to"`

	// AdditionalInfo is an extension field.
	// AdditionalInfo is an extension field.
	//
	// This field provides extensibility for custom router metadata,
	// monitoring data, or protocol-specific configuration.
	// This field provides scalability for customizing router metadata, monitoring data, or protocol-specific configurations.
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// FromDsl defines the source for an endpoint router.
// FromDsl defines the source of the endpoint router.
//
// FromDsl configures how incoming requests are received and initially processed
// before being forwarded to the rule chain or component destination. It defines
// the request reception pattern, processing pipeline, and data extraction rules.
// FromDsl configures how to receive and initially process incoming requests, then forward them to the rule chain or component target.
// It defines request reception patterns, processing pipelines, and data extraction rules.
//
// Source Processing Pipeline:
// Source treatment pipeline:
//  1. Request reception: Accept incoming requests matching the path pattern
//     Request Reception: Accepts incoming requests that match the path pattern
//  2. Preprocessing: Apply source-specific processors
//     Preprocessing: Apply source-specific processors
//  3. Data extraction: Extract relevant data from the request
//     Data extraction: extracting relevant data from requests
//  4. Message creation: Create RuleMsg for rule chain processing
//     Message creation: Creates a RuleMsg for rule chain processing
//
// Path Pattern Examples:
// Example of path pattern:
//   - Static paths: "/api/users", "/webhook/github"
//     Static paths: "/api/users", "/webhook/github"
//   - Parameterized: "/api/users/{id}", "/orders/{orderId}/items"
//     Parameterization: "/api/users/{id}", "/orders/{orderId}/items"
//   - Wildcards: "/files/*", "/api/v1/**"
//     Wildcards: "/files/*", "/api/v1/**"
//   - MQTT topics: "sensor/+/temperature", "devices/+/+/telemetry"
//     MQTT topic: "sensor/+/temperature", "devices/+/+/telemetry"
type FromDsl struct {
	// Path is the path of the source.
	// Path is the path of the source.
	//
	// The path defines the pattern that incoming requests must match to be
	// processed by this router. The format depends on the endpoint protocol:
	// The path defines the mode that the incoming request must match before it can be processed by this router.
	// The format depends on the endpoint protocol:
	//
	// HTTP Paths:
	// HTTP path:
	//   - Support path parameters with {} syntax
	//     Supports path parameters using the {} syntax
	//   - Wildcard matching with * and **
	//     Use wildcards for * and ** to match
	//   - Query parameter extraction
	//     Query parameter extraction
	//
	// MQTT Topics:
	// MQTT Topic:
	//   - Single-level wildcard: +
	//     Single-level wildcard: +
	//   - Multi-level wildcard: #
	//     Multi-level wildcards: #
	//   - Topic parameter extraction
	//     Topic parameter extraction
	Path string `json:"path"`

	// Configuration is the configuration for the source.
	// Configuration is the configuration of the source.
	//
	// Source-specific configuration that controls how requests are received
	// and initially processed. Common configurations include timeouts,
	// buffer sizes, validation rules, and protocol-specific settings.
	// Controls the source-specific configuration for how requests are received and initially processed.
	// Common configurations include timeout, buffer size, validation rules, and protocol-specific settings.
	//
	// HTTP Configuration:
	// HTTP configuration:
	//   - maxRequestSize: Maximum request body size
	//     maxRequestSize: Maximum body size of the request
	//   - timeout: Request timeout duration
	//     timeout: Duration of the request timeout
	//   - cors: CORS policy configuration
	//     cors: CORS policy configuration
	//
	// MQTT Configuration:
	// MQTT configuration:
	//   - qos: Quality of Service level
	//     QoS (QOS): Service Quality Level
	//   - retained: Message retention flag
	//     retained: message retention flag
	//   - clientId: MQTT client identifier
	//     clientId: MQTT client identifier
	Configuration Configuration `json:"configuration"`

	// Processors is the list of processors for the source.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors are a list of processors from the source.
	// Use processors registered by name in builtin/processor#Builtins xx.
	//
	// Source processors handle request preprocessing before the message
	// is forwarded to the destination. They can modify, validate, or
	// enrich the incoming request data.
	// The source processor handles request preprocessing before the message is forwarded to the destination.
	// They can modify, validate, or enrich incoming request data.
	//
	// Common Source Processors:
	// Common source processors:
	//   - auth: Authentication verification
	//     auth: Authentication verification
	//   - validate: Request validation
	//     validate: Request validation
	//   - transform: Data format transformation
	//     transform: Data format conversion
	//   - enrich: Data enrichment from external sources
	//     enrich: Rich data from external sources
	Processors []string `json:"processors"`
}

// ToDsl defines the destination for an endpoint router.
// ToDsl defines the endpoint router's target.
//
// ToDsl configures where processed requests should be forwarded and how
// responses should be handled. It supports various destination types including
// rule chains, components, and external services, with flexible response handling.
// ToDsl configures where requests to be forwarded and how to handle responses.
// It supports various target types, including rule chains, components, and external services, offering flexible response handling.
//
// Destination Types:
// Target types:
//   - Rule chains: Forward to complete rule processing workflows
//     Rule chain: forwards to the complete rule processing workflow
//   - Components: Direct component execution
//     Components: Directly executed by components
//   - External services: Proxy to external APIs
//     External services: Proxy to external APIs
//   - Custom handlers: User-defined processing logic
//     Custom handler: User-defined processing logic
//
// Response Handling:
// Response Handling:
//   - Synchronous: Wait for processing completion and return response
//     Synchronous: Wait for processing to finish and return a response
//   - Asynchronous: Fire-and-forget processing
//     Asynchronous: Instant forgetting and handling immediately
//   - Streaming: Real-time response streaming
//     Stream: Real-time response flow
//   - Callback: Response via callback mechanisms
//     Callback: Response through the callback mechanism
type ToDsl struct {
	// Path is the path of the executor for the destination.
	// For example, "chain:default" to execute by a rule chain for `default`, "component:jsTransform" to execute a JS transform component.
	// Path is the path of the target executor.
	// For example, "chain:default" means executed by `default` rule chains, "component:jsTransform" means executing JS conversion components.
	//
	// Path Format and Examples:
	// Path format and examples:
	//   - Rule chain execution: "chain:{chainId}"
	//     Rule chain execution: "chain:{chainId}"
	//   - Component execution: "component:{componentType}"
	//     Component executes: "component:{componentType}"
	//   - Node execution: "node:{nodeId}"
	//     Node executes: "node:{nodeId}"
	//   - External service: "http://external-api.com/endpoint"
	//     External Services: "http://external-api.com/endpoint"
	//   - Custom handler: "handler:{handlerName}"
	//     Custom handler: "handler:{handlerName}"
	//
	// The path determines how the rule engine interprets and routes
	// the processed message for execution.
	// The path determines how the rule engine interprets and routes messages for execution.
	Path string `json:"path"`

	// Configuration is the configuration for the destination.
	// Configuration is the configuration of the target.
	//
	// Destination-specific configuration that controls how the message
	// is processed at the destination and how responses are handled.
	// Control how messages are handled at the target and how to address the specific configuration of the response.
	//
	// Common Configuration Options:
	// Common configuration options:
	//   - timeout: Processing timeout duration
	//     timeout: Handles timeout duration
	//   - retries: Number of retry attempts on failure
	//     retries: Number of retries upon failure
	//   - headers: Additional headers for external services
	//     Headers: Additional headers for external services
	//   - authentication: Authentication credentials
	//     authentication: authentication credentials
	Configuration Configuration `json:"configuration"`

	// Wait indicates whether to wait for the 'To' executor to finish before proceeding.
	// Wait indicates whether to wait for the 'To' actuator to finish before continuing.
	//
	// This flag controls the execution mode and response handling:
	// This flag controls execution mode and response handling:
	//
	// Synchronous (Wait = true):
	// Synchronization (Wait = true):
	//   - Wait for destination processing to complete
	//     Wait for the target processing to complete
	//   - Return the actual processing result to client
	//     Returns the actual processing result to the client
	//   - Higher latency but guaranteed response
	//     High latency but guaranteed responsiveness
	//   - Suitable for request-response patterns
	//     Suitable for request-response mode
	//
	// Asynchronous (Wait = false):
	// Asynchronous (Wait = false):
	//   - Immediately return acknowledgment to client
	//     Immediately return the acknowledgment to the client
	//   - Process request in background
	//     Handle requests in the background
	//   - Lower latency but fire-and-forget
	//     Lower latency but instant forgetfulness
	//   - Suitable for event processing and webhooks
	//     Suitable for event handling and webhooks
	Wait bool `json:"wait"`

	// Processors is the list of processors for the destination.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors are the list of processors for the target.
	// Use processors registered by name in builtin/processor#Builtins xx.
	//
	// Destination processors handle response postprocessing after the
	// destination has completed processing. They can transform, format,
	// or enhance the response before it's returned to the client.
	// The target processor responds to post-processing after the target completes processing.
	// They can convert, format, or enhance responses before they are returned to the client.
	//
	// Common Destination Processors:
	// Common target processors:
	//   - format: Response format conversion (JSON, XML, etc.)
	//     format: Response format conversion (JSON, XML, etc.)
	//   - cache: Response caching
	//     cache: response cache
	//   - compress: Response compression
	//     compress: response compression
	//   - audit: Response auditing and logging
	//     audit: Response audit and log recording
	//   - metrics: Performance metrics collection
	//     metrics: Collection of performance metrics
	Processors []string `json:"processors"`
}
