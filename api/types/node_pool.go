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
// SharedNode 表示网络资源节点组件，如客户端或服务器连接。
//
// SharedNode extends the basic Node interface with resource sharing capabilities,
// enabling connection pooling, resource reuse, and shared state management across
// multiple rule chains and components.
// SharedNode 扩展了基本的 Node 接口，增加了资源共享功能，
// 支持多个规则链和组件间的连接池、资源重用和共享状态管理。
//
// Key Features:
// 关键特性：
//   - Resource sharing across multiple rule chains
//     多个规则链间的资源共享
//   - Connection pooling for network clients
//     网络客户端的连接池
//   - Lifecycle management of shared resources
//     共享资源的生命周期管理
//   - Performance optimization through reuse
//     通过重用实现性能优化
//
// Common Use Cases:
// 常见用例：
//   - HTTP client pooling for REST API calls
//     REST API 调用的 HTTP 客户端池
//   - Database connection pooling
//     数据库连接池
//   - Message queue connection sharing
//     消息队列连接共享
//   - TCP/UDP socket connection management
//     TCP/UDP 套接字连接管理
//   - Cache client sharing (Redis, Memcached)
//     缓存客户端共享（Redis、Memcached）
type SharedNode interface {
	Node
	// GetInstance retrieves the underlying net client or server connection.
	// Used for connection pool reuse
	// GetInstance 检索底层网络客户端或服务器连接。
	// 用于连接池重用
	//
	// This method provides access to the actual network resource (connection, client, etc.)
	// that can be shared across multiple components. The returned instance should be
	// thread-safe and ready for concurrent use.
	// 此方法提供对实际网络资源（连接、客户端等）的访问，
	// 可以在多个组件间共享。返回的实例应该是线程安全的并准备好并发使用。
	//
	// Returns:
	// 返回：
	//   - interface{}: The shared resource instance (HTTP client, DB connection, etc.)
	//     interface{}：共享资源实例（HTTP 客户端、数据库连接等）
	//   - error: Any error that occurred while accessing the resource
	//     error：访问资源时发生的任何错误
	//
	// Example implementations:
	// 实现示例：
	//   - HTTP client: return &http.Client{}, nil
	//     HTTP 客户端：return &http.Client{}, nil
	//   - DB connection: return sql.DB instance, nil
	//     数据库连接：return sql.DB 实例, nil
	//   - Redis client: return redis.Client instance, nil
	//     Redis 客户端：return redis.Client 实例, nil
	GetInstance() (interface{}, error)
}

// SharedNodeCtx represents the context wrapper for shared node components.
// SharedNodeCtx 表示共享节点组件的上下文包装器。
//
// SharedNodeCtx extends NodeCtx with additional capabilities for managing shared
// resources and provides both the node context and direct access to the underlying
// shared resource instance.
// SharedNodeCtx 扩展了 NodeCtx，增加了管理共享资源的额外功能，
// 并提供节点上下文和对底层共享资源实例的直接访问。
//
// This interface serves as a bridge between the rule engine's node management
// system and the shared resource pooling system, enabling efficient resource
// utilization while maintaining proper isolation and lifecycle management.
// 此接口作为规则引擎的节点管理系统和共享资源池系统之间的桥梁，
// 在保持适当隔离和生命周期管理的同时实现高效的资源利用。
//
// Architectural Benefits:
// 架构优势：
//   - Unified interface for both node and resource management
//     节点和资源管理的统一接口
//   - Consistent access patterns across different resource types
//     不同资源类型的一致访问模式
//   - Simplified integration with existing rule chain infrastructure
//     与现有规则链基础设施的简化集成
//   - Enhanced monitoring and debugging capabilities
//     增强的监控和调试功能
type SharedNodeCtx interface {
	NodeCtx
	// GetInstance Obtain shared component resource instance
	// GetInstance 获取共享组件资源实例
	//
	// This method provides direct access to the shared resource managed by this node context.
	// It's a convenience method that delegates to the underlying SharedNode's GetInstance method.
	// 此方法提供对此节点上下文管理的共享资源的直接访问。
	// 它是委托给底层 SharedNode 的 GetInstance 方法的便利方法。
	//
	// Returns:
	// 返回：
	//   - interface{}: The shared resource instance
	//     interface{}：共享资源实例
	//   - error: Any error that occurred during resource access
	//     error：资源访问期间发生的任何错误
	GetInstance() (interface{}, error)

	// GetNode returns the underlying node instance
	// GetNode 返回底层节点实例
	//
	// This method provides access to the raw node implementation, which can be useful
	// for advanced operations, debugging, or when type-specific functionality is needed.
	// 此方法提供对原始节点实现的访问，这对于高级操作、调试或需要特定类型功能时很有用。
	//
	// Returns:
	// 返回：
	//   - interface{}: The underlying node instance (typically implementing SharedNode)
	//     interface{}：底层节点实例（通常实现 SharedNode）
	GetNode() interface{}
}

// NodePool provides centralized management for shared node resources across rule chains.
// NodePool 为规则链间的共享节点资源提供集中管理。
//
// NodePool serves as a registry and factory for shared network resources, enabling
// efficient resource pooling, lifecycle management, and configuration consistency
// across multiple rule chains within an application.
// NodePool 作为共享网络资源的注册表和工厂，支持应用程序内多个规则链间的
// 高效资源池、生命周期管理和配置一致性。
//
// Architecture Overview:
// 架构概览：
//   - Centralized resource management for all rule chains
//     所有规则链的集中资源管理
//   - Factory pattern for creating shared node instances
//     创建共享节点实例的工厂模式
//   - Configuration-driven resource initialization
//     配置驱动的资源初始化
//   - Automatic lifecycle management
//     自动生命周期管理
//
// Resource Lifecycle:
// 资源生命周期：
//  1. Load: Parse configuration and prepare resources
//     Load：解析配置并准备资源
//  2. Create: Instantiate shared node contexts
//     Create：实例化共享节点上下文
//  3. Manage: Provide access and maintain connections
//     Manage：提供访问并维护连接
//  4. Cleanup: Properly dispose of resources
//     Cleanup：适当处置资源
//
// Thread Safety:
// 线程安全性：
// All methods should be thread-safe to support concurrent access from
// multiple rule chains and components.
// 所有方法都应该是线程安全的，以支持来自多个规则链和组件的并发访问。
type NodePool interface {
	// Load loads sharedNode list from a ruleChain DSL definition.
	// Load 从规则链 DSL 定义加载共享节点列表。
	//
	// This method parses a complete rule chain DSL and extracts shared node
	// configurations, creating a new NodePool instance with those resources.
	// 此方法解析完整的规则链 DSL 并提取共享节点配置，
	// 使用这些资源创建新的 NodePool 实例。
	//
	// Parameters:
	// 参数：
	//   - dsl: Rule chain DSL in byte format (typically JSON)
	//     dsl：字节格式的规则链 DSL（通常是 JSON）
	//
	// Returns:
	// 返回：
	//   - NodePool: New pool instance with loaded shared nodes
	//     NodePool：包含已加载共享节点的新池实例
	//   - error: Any error that occurred during parsing or loading
	//     error：解析或加载期间发生的任何错误
	//
	// Usage:
	// 使用：
	//   dsl := []byte(`{"endpoints": [...], "metadata": {...}}`)
	//   pool, err := nodePool.Load(dsl)
	Load(dsl []byte) (NodePool, error)

	// LoadFromRuleChain loads sharedNode list from a ruleChain definition.
	// LoadFromRuleChain 从规则链定义加载共享节点列表。
	//
	// This method accepts a parsed RuleChain structure and extracts shared node
	// configurations from it, providing a more direct way to initialize the pool
	// when the rule chain structure is already available.
	// 此方法接受解析的 RuleChain 结构并从中提取共享节点配置，
	// 当规则链结构已经可用时提供更直接的初始化池的方式。
	//
	// Parameters:
	// 参数：
	//   - def: Parsed rule chain definition structure
	//     def：解析的规则链定义结构
	//
	// Returns:
	// 返回：
	//   - NodePool: New pool instance with loaded shared nodes
	//     NodePool：包含已加载共享节点的新池实例
	//   - error: Any error that occurred during loading
	//     error：加载期间发生的任何错误
	LoadFromRuleChain(def RuleChain) (NodePool, error)

	// NewFromEndpoint new an endpoint sharedNode
	// NewFromEndpoint 从端点创建新的共享节点
	//
	// This method creates a shared node context from an endpoint DSL definition,
	// enabling endpoint components to be managed as shared resources.
	// 此方法从端点 DSL 定义创建共享节点上下文，
	// 使端点组件能够作为共享资源进行管理。
	//
	// Parameters:
	// 参数：
	//   - def: Endpoint DSL definition
	//     def：端点 DSL 定义
	//
	// Returns:
	// 返回：
	//   - SharedNodeCtx: Configured shared node context for the endpoint
	//     SharedNodeCtx：端点的配置共享节点上下文
	//   - error: Any error that occurred during creation
	//     error：创建期间发生的任何错误
	NewFromEndpoint(def EndpointDsl) (SharedNodeCtx, error)

	// NewFromRuleNode new a rule node sharedNode
	// NewFromRuleNode 从规则节点创建新的共享节点
	//
	// This method creates a shared node context from a rule node definition,
	// enabling regular rule nodes to be managed as shared resources.
	// 此方法从规则节点定义创建共享节点上下文，
	// 使常规规则节点能够作为共享资源进行管理。
	//
	// Parameters:
	// 参数：
	//   - def: Rule node definition
	//     def：规则节点定义
	//
	// Returns:
	// 返回：
	//   - SharedNodeCtx: Configured shared node context for the rule node
	//     SharedNodeCtx：规则节点的配置共享节点上下文
	//   - error: Any error that occurred during creation
	//     error：创建期间发生的任何错误
	NewFromRuleNode(def RuleNode) (SharedNodeCtx, error)

	// AddNode add a sharedNode
	// AddNode 添加共享节点
	//
	// This method adds a pre-configured node to the pool, wrapping it in a
	// SharedNodeCtx for management. This is useful for programmatically
	// adding nodes or integrating with external resource management systems.
	// 此方法将预配置节点添加到池中，将其包装在 SharedNodeCtx 中进行管理。
	// 这对于以编程方式添加节点或与外部资源管理系统集成很有用。
	//
	// Parameters:
	// 参数：
	//   - endpoint: Pre-configured node instance to add
	//     endpoint：要添加的预配置节点实例
	//
	// Returns:
	// 返回：
	//   - SharedNodeCtx: Wrapped node context for the added node
	//     SharedNodeCtx：添加节点的包装节点上下文
	//   - error: Any error that occurred during addition
	//     error：添加期间发生的任何错误
	AddNode(endpoint Node) (SharedNodeCtx, error)

	// Get retrieves a SharedNode instance by its ID.
	// Get 通过 ID 检索 SharedNode 实例。
	//
	// This method provides access to a previously registered shared node context
	// by its unique identifier. It's the primary way to access shared resources
	// from rule chain components.
	// 此方法通过其唯一标识符提供对先前注册的共享节点上下文的访问。
	// 这是从规则链组件访问共享资源的主要方式。
	//
	// Parameters:
	// 参数：
	//   - id: Unique identifier of the shared node
	//     id：共享节点的唯一标识符
	//
	// Returns:
	// 返回：
	//   - SharedNodeCtx: The shared node context if found
	//     SharedNodeCtx：如果找到则返回共享节点上下文
	//   - bool: True if the node was found, false otherwise
	//     bool：如果找到节点则为 true，否则为 false
	Get(id string) (SharedNodeCtx, bool)

	// GetInstance retrieves a net client or server connection by its nodeTye and ID.
	// GetInstance 通过节点类型和 ID 检索网络客户端或服务器连接。
	//
	// This is a convenience method that combines node lookup and instance access
	// in a single call, providing direct access to the underlying shared resource.
	// 这是一个便利方法，在单次调用中结合节点查找和实例访问，
	// 提供对底层共享资源的直接访问。
	//
	// Parameters:
	// 参数：
	//   - id: Unique identifier of the shared node
	//     id：共享节点的唯一标识符
	//
	// Returns:
	// 返回：
	//   - interface{}: The shared resource instance if found
	//     interface{}：如果找到则返回共享资源实例
	//   - error: Any error that occurred during lookup or access
	//     error：查找或访问期间发生的任何错误
	GetInstance(id string) (interface{}, error)

	// Del deletes a SharedNode instance by its nodeTye and ID.
	// Del 通过节点类型和 ID 删除 SharedNode 实例。
	//
	// This method removes a shared node from the pool and properly cleans up
	// its resources. It should be used when a shared resource is no longer needed
	// or when updating configurations.
	// 此方法从池中删除共享节点并适当清理其资源。
	// 当不再需要共享资源或更新配置时应使用它。
	//
	// Parameters:
	// 参数：
	//   - id: Unique identifier of the shared node to delete
	//     id：要删除的共享节点的唯一标识符
	//
	// Cleanup Process:
	// 清理过程：
	//   1. Locate the node by ID
	//      通过 ID 定位节点
	//   2. Call the node's Destroy() method
	//      调用节点的 Destroy() 方法
	//   3. Remove from internal registry
	//      从内部注册表中删除
	//   4. Clean up any associated metadata
	//      清理任何关联的元数据
	Del(id string)

	// Stop stops and releases all SharedNode instances.
	// Stop 停止并释放所有 SharedNode 实例。
	//
	// This method performs a complete shutdown of the node pool, properly
	// cleaning up all shared resources. It should be called during application
	// shutdown to ensure proper resource cleanup.
	// 此方法执行节点池的完全关闭，适当清理所有共享资源。
	// 应在应用程序关闭期间调用它以确保适当的资源清理。
	//
	// Shutdown Process:
	// 关闭过程：
	//   1. Iterate through all registered nodes
	//      遍历所有注册的节点
	//   2. Call Destroy() on each node
	//      在每个节点上调用 Destroy()
	//   3. Clear internal registries
	//      清除内部注册表
	//   4. Release pool resources
	//      释放池资源
	//
	// Thread Safety:
	// 线程安全性：
	// This method should handle concurrent access gracefully and ensure
	// that no new operations can start while shutdown is in progress.
	// 此方法应优雅地处理并发访问，并确保在关闭进行时不能启动新操作。
	Stop()

	// GetAll get all SharedNode instances
	// GetAll 获取所有 SharedNode 实例
	//
	// This method returns a snapshot of all currently registered shared node
	// contexts in the pool. It's useful for monitoring, debugging, and
	// administrative operations.
	// 此方法返回池中当前注册的所有共享节点上下文的快照。
	// 它对于监控、调试和管理操作很有用。
	//
	// Returns:
	// 返回：
	//   - []SharedNodeCtx: Slice containing all shared node contexts
	//     []SharedNodeCtx：包含所有共享节点上下文的切片
	//
	// Note: The returned slice is a snapshot and modifications to it
	// will not affect the actual pool contents.
	// 注意：返回的切片是快照，对其的修改不会影响实际池内容。
	GetAll() []SharedNodeCtx

	// GetAllDef get all SharedNode instances definition
	// GetAllDef 获取所有 SharedNode 实例定义
	//
	// This method returns the configuration definitions for all shared nodes
	// in the pool, organized by node type. It's useful for configuration
	// export, backup, and debugging purposes.
	// 此方法返回池中所有共享节点的配置定义，按节点类型组织。
	// 它对于配置导出、备份和调试目的很有用。
	//
	// Returns:
	// 返回：
	//   - map[string][]*RuleNode: Map of node type to list of node definitions
	//     map[string][]*RuleNode：节点类型到节点定义列表的映射
	//   - error: Any error that occurred during definition extraction
	//     error：定义提取期间发生的任何错误
	//
	// The returned map structure allows for easy organization and
	// categorization of shared resources by their type.
	// 返回的映射结构允许按类型轻松组织和分类共享资源。
	GetAllDef() (map[string][]*RuleNode, error)

	// Range iterates over all shared node instances in the pool using a callback function.
	// Range 使用回调函数遍历池中的所有共享节点实例。
	//
	// This method provides a flexible way to process all shared nodes without loading
	// them all into memory at once. The callback function receives key-value pairs
	// representing the node ID and its corresponding SharedNodeCtx.
	// 此方法提供灵活的方式处理所有共享节点，而无需一次性将它们全部加载到内存中。
	// 回调函数接收表示节点ID及其对应SharedNodeCtx的键值对。
	//
	// Parameters:
	// 参数：
	//   - f: Callback function that receives (key, value) pairs
	//     f：接收(键,值)对的回调函数
	//     - key: Node ID (string)
	//       key：节点ID（字符串）
	//     - value: SharedNodeCtx instance
	//       value：SharedNodeCtx实例
	//     - return: false to stop iteration, true to continue
	//       return：返回false停止迭代，true继续
	//
	// Usage:
	// 使用：
	//   pool.Range(func(key, value any) bool {
	//       id := key.(string)
	//       ctx := value.(SharedNodeCtx)
	//       // Process each shared node
	//       return true // Continue iteration
	//   })
	//
	// Thread Safety:
	// 线程安全：
	// This method is thread-safe and can be called concurrently.
	// The iteration provides a consistent snapshot at the time of the call.
	// 此方法是线程安全的，可以并发调用。
	// 迭代在调用时提供一致的快照。
	Range(f func(key, value any) bool)
}
