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

// SharedNode represents a network resource node component such as a client or server connection.
// SharedNode represents network resource node components, such as client or server connections.
//
// SharedNode extends the basic Node interface with resource sharing capabilities,
// enabling connection pooling, resource reuse, and shared state management across
// multiple rule chains and components.
// SharedNode extends the basic Node interface and adds resource sharing features,
// Supports connection pools, resource reuse, and shared state management among multiple rule chains and components.
//
// Key Features:
// Key features:
//   - Resource sharing across multiple rule chains
//     Resource sharing across multiple rule chains
//   - Connection pooling for network clients
//     The connection pool for network clients
//   - Lifecycle management of shared resources
//     Lifecycle management of shared resources
//   - Performance optimization through reuse
//     Performance optimization is achieved through reuse
//
// Common Use Cases:
// Common Use Cases:
//   - HTTP client pooling for REST API calls
//     HTTP client pool for REST API calls
//   - Database connection pooling
//     Database connection pool
//   - Message queue connection sharing
//     Message queue connection sharing
//   - TCP/UDP socket connection management
//     Manage TCP/UDP socket connections
//   - Cache client sharing (Redis, Memcached)
//     Cached client sharing (Redis, Memcached)
type SharedNode interface {
	Node
	// GetInstance retrieves the underlying net client or server connection.
	// Used for connection pool reuse
	// GetInstance retrieves connections between underlying network clients or servers.
	// Used for reuse of connection pools
	//
	// This method provides access to the actual network resource (connection, client, etc.)
	// that can be shared across multiple components. The returned instance should be
	// thread-safe and ready for concurrent use.
	// This method provides access to actual network resources (connections, clients, etc.),
	// It can be shared across multiple components. The returned instance should be thread-safe and ready for concurrent use.
	//
	// Returns:
	// Returns:
	//   - interface{}: The shared resource instance (HTTP client, DB connection, etc.)
	//     interface{}: Shared resource instance (HTTP client, database connection, etc.)
	//   - error: Any error that occurred while accessing the resource
	//     error: Any error that occurs when accessing resources
	//
	// Example implementations:
	// Implementation example:
	//   - HTTP client: return &http.Client{}, nil
	//     HTTP client: return &http.Client{}, nil
	//   - DB connection: return sql.DB instance, nil
	//     Database connection: return sql.DB instance, nil
	//   - Redis client: return redis.Client instance, nil
	//     Redis client: return redis.Client instance, nil
	GetInstance() (interface{}, error)
}

// SharedNodeCtx represents the context wrapper for shared node components.
// SharedNodeCtx represents a context wrapper for sharing node components.
//
// SharedNodeCtx extends NodeCtx with additional capabilities for managing shared
// resources and provides both the node context and direct access to the underlying
// shared resource instance.
// SharedNodeCtx extends NodeCtx, adding additional features for managing shared resources,
// It also provides node context and direct access to underlying shared resource instances.
//
// This interface serves as a bridge between the rule engine's node management
// system and the shared resource pooling system, enabling efficient resource
// utilization while maintaining proper isolation and lifecycle management.
// This interface serves as a bridge between the node management system of the rule engine and the shared resource pool system,
// Achieve efficient resource utilization while maintaining proper isolation and lifecycle management.
//
// Architectural Benefits:
// Architecture Advantages:
//   - Unified interface for both node and resource management
//     Unified interface for node and resource management
//   - Consistent access patterns across different resource types
//     Consistent access patterns for different resource types
//   - Simplified integration with existing rule chain infrastructure
//     Simplified integration with existing rule chain infrastructure
//   - Enhanced monitoring and debugging capabilities
//     Enhanced monitoring and commissioning capabilities
type SharedNodeCtx interface {
	NodeCtx
	// GetInstance Obtain shared component resource instance
	// GetInstance obtains the shared component resource instance
	//
	// This method provides direct access to the shared resource managed by this node context.
	// It's a convenience method that delegates to the underlying SharedNode's GetInstance method.
	// This method provides direct access to shared resources managed by this node's context.
	// It is a convenient method delegated to the underlying SharedNode's GetInstance method.
	//
	// Returns:
	// Returns:
	//   - interface{}: The shared resource instance
	//     interface{}: Shared resource instance
	//   - error: Any error that occurred during resource access
	//     error: Any error that occurs during resource access
	GetInstance() (interface{}, error)

	// GetNode returns the underlying node instance
	// GetNode returns the underlying node instance
	//
	// This method provides access to the raw node implementation, which can be useful
	// for advanced operations, debugging, or when type-specific functionality is needed.
	// This method provides access to the original node implementation, which is useful for advanced operations, debugging, or when specific types of functionality are needed.
	//
	// Returns:
	// Returns:
	//   - interface{}: The underlying node instance (typically implementing SharedNode)
	//     interface{}: Underlying node instance (usually SharedNode)
	GetNode() interface{}
}

// NodePool provides centralized management for shared node resources across rule chains.
// NodePool provides centralized management for shared node resources between rule chains.
//
// NodePool serves as a registry and factory for shared network resources, enabling
// efficient resource pooling, lifecycle management, and configuration consistency
// across multiple rule chains within an application.
// NodePool acts as a registry and factory for shared network resources, supporting multiple rule chains within the application
// Efficient resource pooling, lifecycle management, and allocation consistency.
//
// Architecture Overview:
// Architecture Overview:
//   - Centralized resource management for all rule chains
//     Centralized resource management of all rule chains
//   - Factory pattern for creating shared node instances
//     Create a factory pattern for shared node instances
//   - Configuration-driven resource initialization
//     Configuration-driven resource initialization
//   - Automatic lifecycle management
//     Automated lifecycle management
//
// Resource Lifecycle:
// Resource lifecycle:
//  1. Load: Parse configuration and prepare resources
//     Load: Parses the configuration and prepares resources
//  2. Create: Instantiate shared node contexts
//     Create: Instantiate the shared node context
//  3. Manage: Provide access and maintain connections
//     Manage: Provides access and maintains connections
//  4. Cleanup: Properly dispose of resources
//     Cleanup: Appropriate disposal of resources
//
// Thread Safety:
// Thread safety:
// All methods should be thread-safe to support concurrent access from
// multiple rule chains and components.
// All methods should be thread-safe to support concurrent access from multiple rule chains and components.
type NodePool interface {
	// NodePool is a resource pool, meaning that when parsing ResourceLookup:ref:// to fetch instances by ID,
	// NodePool.Lookup serves as the shared pool fallback source (alongside ChainCtx.Resources() in the same chain directory).
	ResourceLookup

	// Load loads sharedNode list from a ruleChain DSL definition.
	// Load: loads the list of shared nodes from the rule chain DSL definition.
	//
	// This method parses a complete rule chain DSL and extracts shared node
	// configurations, creating a new NodePool instance with those resources.
	// This method parses the complete rule chain DSL and extracts the shared node configuration,
	// Use these resources to create new NodePool instances.
	//
	// Parameters:
	// Parameters:
	//   - dsl: Rule chain DSL in byte format (typically JSON)
	//     dsl: Byte-format rule chain DSL (usually JSON)
	//
	// Returns:
	// Returns:
	//   - NodePool: New pool instance with loaded shared nodes
	//     NodePool: Contains new pool instances loaded on shared nodes
	//   - error: Any error that occurred during parsing or loading
	//     error: Any error that occurs during parsing or loading
	//
	// Usage:
	// Usage:
	//   dsl := []byte(`{"endpoints": [...], "metadata": {...}}`)
	//   pool, err := nodePool.Load(dsl)
	Load(dsl []byte) (NodePool, error)

	// LoadFromRuleChain loads sharedNode list from a ruleChain definition.
	// LoadFromRuleChain Loads a list of shared nodes from the rule chain definition.
	//
	// This method accepts a parsed RuleChain structure and extracts shared node
	// configurations from it, providing a more direct way to initialize the pool
	// when the rule chain structure is already available.
	// This method accepts the parsed RuleChain structure and extracts the shared node configuration from it,
	// When the rule chain structure is already available, it provides a more direct way to initialize the pool.
	//
	// Parameters:
	// Parameters:
	//   - def: Parsed rule chain definition structure
	//     def: The definition structure of the rule chain for parsing
	//
	// Returns:
	// Returns:
	//   - NodePool: New pool instance with loaded shared nodes
	//     NodePool: Contains new pool instances loaded on shared nodes
	//   - error: Any error that occurred during loading
	//     error: Any error that occurs during loading
	LoadFromRuleChain(def RuleChain) (NodePool, error)

	// NewFromEndpoint new an endpoint sharedNode
	// NewFromEndpoint Creates a new shared node from the endpoint
	//
	// This method creates a shared node context from an endpoint DSL definition,
	// enabling endpoint components to be managed as shared resources.
	// This method creates a shared node context from the endpoint DSL definition,
	// Enables endpoint components to be managed as shared resources.
	//
	// Parameters:
	// Parameters:
	//   - def: Endpoint DSL definition
	//     def: Endpoint DSL definition
	//
	// Returns:
	// Returns:
	//   - SharedNodeCtx: Configured shared node context for the endpoint
	//     SharedNodeCtx: The endpoint configuration shares node context
	//   - error: Any error that occurred during creation
	//     error: Any error that occurs during creation
	NewFromEndpoint(def EndpointDsl) (SharedNodeCtx, error)

	// NewFromRuleNode new a rule node sharedNode
	// NewFromRuleNode creates a new shared node from the rule node
	//
	// This method creates a shared node context from a rule node definition,
	// enabling regular rule nodes to be managed as shared resources.
	// This method creates a shared node context from the rule node definition,
	// Allows regular rule nodes to be managed as shared resources.
	//
	// Parameters:
	// Parameters:
	//   - def: Rule node definition
	//     def: Rule node definition
	//
	// Returns:
	// Returns:
	//   - SharedNodeCtx: Configured shared node context for the rule node
	//     SharedNodeCtx: Configuration of rule nodes that share node context
	//   - error: Any error that occurred during creation
	//     error: Any error that occurs during creation
	NewFromRuleNode(def RuleNode) (SharedNodeCtx, error)

	// AddNode add a sharedNode
	// AddNode adds a shared node
	//
	// This method adds a pre-configured node to the pool, wrapping it in a
	// SharedNodeCtx for management. This is useful for programmatically
	// adding nodes or integrating with external resource management systems.
	// This method adds pre-configured nodes to the pool and wraps them in SharedNodeCtx for management.
	// This is useful for adding nodes programmatically or integrating with external resource management systems.
	//
	// Parameters:
	// Parameters:
	//   - endpoint: Pre-configured node instance to add
	//     Endpoint: The preconfigured node instance to add
	//
	// Returns:
	// Returns:
	//   - SharedNodeCtx: Wrapped node context for the added node
	//     SharedNodeCtx: Adds the node's wrapper node context
	//   - error: Any error that occurred during addition
	//     error: Any errors that occur during the addition process
	AddNode(endpoint Node) (SharedNodeCtx, error)

	// Get retrieves a SharedNode instance by its ID.
	// Get retrieves the SharedNode instance by ID.
	//
	// This method provides access to a previously registered shared node context
	// by its unique identifier. It's the primary way to access shared resources
	// from rule chain components.
	// This method provides access to the context of previously registered shared nodes through its unique identifier.
	// This is the main way to access shared resources from rule chain components.
	//
	// Parameters:
	// Parameters:
	//   - id: Unique identifier of the shared node
	//     id: The unique identifier of the shared node
	//
	// Returns:
	// Returns:
	//   - SharedNodeCtx: The shared node context if found
	//     SharedNodeCtx: Returns the shared node context if found
	//   - bool: True if the node was found, false otherwise
	//     bool: if a node is found, it is true; otherwise, it is false
	Get(id string) (SharedNodeCtx, bool)

	// GetInstance retrieves a net client or server connection by its nodeTye and ID.
	// GetInstance retrieves network client or server connections by node type and ID.
	//
	// This is a convenience method that combines node lookup and instance access
	// in a single call, providing direct access to the underlying shared resource.
	// This is a convenient method that combines node lookup and instance access in a single call,
	// Provide direct access to underlying shared resources.
	//
	// Parameters:
	// Parameters:
	//   - id: Unique identifier of the shared node
	//     id: The unique identifier of the shared node
	//
	// Returns:
	// Returns:
	//   - interface{}: The shared resource instance if found
	//     interface{}: If found, returns a shared resource instance
	//   - error: Any error that occurred during lookup or access
	//     error: Any error that occurs during searching or access
	GetInstance(id string) (interface{}, error)

	// Del deletes a SharedNode instance by its nodeTye and ID.
	// Del deletes SharedNode instances by node type and ID.
	//
	// This method removes a shared node from the pool and properly cleans up
	// its resources. It should be used when a shared resource is no longer needed
	// or when updating configurations.
	// This method removes shared nodes from the pool and properly cleans up their resources.
	// It should be used when shared resources are no longer needed or when configuration updates are needed.
	//
	// Parameters:
	// Parameters:
	//   - id: Unique identifier of the shared node to delete
	//     id: The unique identifier of the shared node to be deleted
	//
	// Cleanup Process:
	// Cleaning process:
	//   1. Locate the node by ID
	//      Nodes are located by ID
	//   2. Call the node's Destroy() method
	//      Call the node's Destroy() method
	//   3. Remove from internal registry
	//      Removed from the internal registry
	//   4. Clean up any associated metadata
	//      Clean up any associated metadata
	Del(id string)

	// Stop stops and releases all SharedNode instances.
	// Stop and release all SharedNode instances.
	//
	// This method performs a complete shutdown of the node pool, properly
	// cleaning up all shared resources. It should be called during application
	// shutdown to ensure proper resource cleanup.
	// This method completes the node pool and properly cleans all shared resources.
	// It should be called during application shutdown to ensure proper resource cleanup.
	//
	// Shutdown Process:
	// Closing process:
	//   1. Iterate through all registered nodes
	//      Traverse all registered nodes
	//   2. Call Destroy() on each node
	//      Call Destroy() on each node
	//   3. Clear internal registries
	//      Clear the internal registry
	//   4. Release pool resources
	//      Release pool resources
	//
	// Thread Safety:
	// Thread safety:
	// This method should handle concurrent access gracefully and ensure
	// that no new operations can start while shutdown is in progress.
	// This method should elegantly handle concurrent access and ensure that no new operations can be initiated while the shutdown is in progress.
	Stop()

	// GetAll get all SharedNode instances
	// GetAll retrieves all SharedNode instances
	//
	// This method returns a snapshot of all currently registered shared node
	// contexts in the pool. It's useful for monitoring, debugging, and
	// administrative operations.
	// This method returns a snapshot of the context of all currently registered shared nodes in the pool.
	// It is useful for monitoring, debugging, and managing operations.
	//
	// Returns:
	// Returns:
	//   - []SharedNodeCtx: Slice containing all shared node contexts
	//     [] SharedNodeCtx: A slice containing all shared node contexts
	//
	// Note: The returned slice is a snapshot and modifications to it
	// will not affect the actual pool contents.
	// Note: The returned slices are snapshots; modifying them will not affect the actual pool content.
	GetAll() []SharedNodeCtx

	// GetAllDef get all SharedNode instances definition
	// GetAllDef retrieves all SharedNode instance definitions
	//
	// This method returns the configuration definitions for all shared nodes
	// in the pool, organized by node type. It's useful for configuration
	// export, backup, and debugging purposes.
	// This method returns the configuration definitions for all shared nodes in the pool, organized by node type.
	// It is useful for configuring export, backup, and debugging purposes.
	//
	// Returns:
	// Returns:
	//   - map[string][]*RuleNode: Map of node type to list of node definitions
	//     map[string][] *RuleNode: Mapping node type to node definition list
	//   - error: Any error that occurred during definition extraction
	//     error: Defines any errors that occur during extraction
	//
	// The returned map structure allows for easy organization and
	// categorization of shared resources by their type.
	// The returned mapping structure allows for easy organization and classification of shared resources by type.
	GetAllDef() (map[string][]*RuleNode, error)

	// Range iterates over all shared node instances in the pool using a callback function.
	// Range uses the callback function to traverse all shared node instances in the pool.
	//
	// This method provides a flexible way to process all shared nodes without loading
	// them all into memory at once. The callback function receives key-value pairs
	// representing the node ID and its corresponding SharedNodeCtx.
	// This method offers a flexible way to handle all shared nodes without having to load them all into memory at once.
	// The callback function receives the key-value pair representing the node ID and its corresponding SharedNodeCtx.
	//
	// Parameters:
	// Parameters:
	//   - f: Callback function that receives (key, value) pairs
	//     f: Callback function for receiving (key, value) pairs
	//     - key: Node ID (string)
	//       key: Node ID (string)
	//     - value: SharedNodeCtx instance
	//       value: SharedNodeCtx instance
	//     - return: false to stop iteration, true to continue
	//       return: returns false to stop iteration, true continues
	//
	// Usage:
	// Usage:
	//   pool.Range(func(key, value any) bool {
	//       id := key.(string)
	//       ctx := value.(SharedNodeCtx)
	//       // Process each shared node
	//       return true // Continue iteration
	//   })
	//
	// Thread Safety:
	// Thread safety:
	// This method is thread-safe and can be called concurrently.
	// The iteration provides a consistent snapshot at the time of the call.
	// This method is thread-safe and can be called concurrently.
	// Iteration provides a consistent snapshot at the time of invocation.
	Range(f func(key, value any) bool)
}
