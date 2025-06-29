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
// RuleChain 定义规则链。
//
// RuleChain is the top-level structure that represents a complete rule processing workflow.
// It contains both basic information about the chain and detailed metadata about its
// internal structure, including nodes, connections, and routing configurations.
// RuleChain 是表示完整规则处理工作流的顶级结构。
// 它包含链的基本信息和关于其内部结构的详细元数据，包括节点、连接和路由配置。
//
// Structure Overview:
// 结构概览：
//   - RuleChain: Basic information (ID, name, configuration)
//     RuleChain：基本信息（ID、名称、配置）
//   - Metadata: Structural details (nodes, connections, endpoints)
//     Metadata：结构详细信息（节点、连接、端点）
//
// Usage in DSL:
// DSL 中的使用：
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
	// RuleChain 包含规则链的基本信息。
	RuleChain RuleChainBaseInfo `json:"ruleChain"`
	// Metadata includes information about the nodes and connections within the rule chain.
	// Metadata 包含规则链内节点和连接的信息。
	Metadata RuleMetadata `json:"metadata"`
}

// RuleChainBaseInfo defines the basic information of a rule chain.
// RuleChainBaseInfo 定义规则链的基本信息。
//
// This structure contains the essential metadata and configuration that applies
// to the entire rule chain. It provides identification, behavioral settings,
// and extensibility through additional information fields.
// 此结构包含适用于整个规则链的基本元数据和配置。
// 它提供标识、行为设置和通过附加信息字段的可扩展性。
//
// Key Features:
// 关键特性：
//   - Unique identification with ID and Name
//     使用 ID 和 Name 的唯一标识
//   - Global debug mode control
//     全局调试模式控制
//   - Hierarchical chain organization (root/sub-chain)
//     分层链组织（根/子链）
//   - Runtime state management (enabled/disabled)
//     运行时状态管理（启用/禁用）
//   - Flexible configuration and extension support
//     灵活的配置和扩展支持
type RuleChainBaseInfo struct {
	// ID is the unique identifier of the rule chain.
	// ID 是规则链的唯一标识符。
	//
	// The ID must be unique within the rule engine context and is used for
	// chain references, sub-chain invocation, and management operations.
	// ID 在规则引擎上下文中必须是唯一的，用于链引用、子链调用和管理操作。
	ID string `json:"id"`

	// Name is the name of the rule chain.
	// Name 是规则链的名称。
	//
	// The name provides a human-readable identifier for the chain, useful
	// for UI display, logging, and administrative purposes.
	// 名称为链提供人类可读的标识符，对 UI 显示、日志记录和管理目的很有用。
	Name string `json:"name"`

	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting overrides the `DebugMode` configuration of the node.
	// DebugMode 表示节点是否处于调试模式。如果为 true，节点处理消息时触发调试回调函数。
	// 此设置覆盖节点的 `DebugMode` 配置。
	//
	// Global Debug Control:
	// 全局调试控制：
	// When enabled at the chain level, debug mode affects all nodes within the chain,
	// regardless of individual node debug settings. This provides convenient
	// chain-wide debugging control.
	// 在链级别启用时，调试模式影响链内所有节点，
	// 无论单个节点的调试设置如何。这提供了方便的链范围调试控制。
	DebugMode bool `json:"debugMode"`

	// Root indicates whether this rule chain is a root or a sub-rule chain. (Used only as a marker, not applied in actual logic)
	// Root 表示此规则链是根链还是子规则链。（仅用作标记，不在实际逻辑中应用）
	//
	// This field serves as organizational metadata to help distinguish between
	// main processing chains and subsidiary chains in complex rule hierarchies.
	// 此字段作为组织元数据，帮助在复杂规则层次结构中区分主处理链和辅助链。
	Root bool `json:"root"`

	// Disabled indicates whether the rule chain is disabled.
	// Disabled 表示规则链是否被禁用。
	//
	// When disabled, the rule chain will not process messages and can be used
	// for maintenance, testing, or gradual rollout scenarios.
	// 禁用时，规则链不会处理消息，可用于维护、测试或渐进式推出场景。
	Disabled bool `json:"disabled"`

	// Configuration contains the configuration information of the rule chain.
	// Configuration 包含规则链的配置信息。
	//
	// This flexible configuration map allows chain-level settings that can
	// be accessed by nodes within the chain for shared configuration data.
	// 此灵活的配置映射允许链级别设置，链内节点可以访问它们以获取共享配置数据。
	Configuration Configuration `json:"configuration,omitempty"`

	// AdditionalInfo is an extension field.
	// AdditionalInfo 是扩展字段。
	//
	// This field provides extensibility for custom metadata, UI information,
	// or integration-specific data without modifying the core structure.
	// 此字段为自定义元数据、UI 信息或集成特定数据提供可扩展性，
	// 无需修改核心结构。
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// GetAdditionalInfo retrieves additional information by key.
// GetAdditionalInfo 通过键检索附加信息。
//
// This method provides safe access to additional information with existence checking.
// It returns both the value and a boolean indicating whether the key was found.
// 此方法提供对附加信息的安全访问，包含存在性检查。
// 它返回值和一个布尔值，指示是否找到了键。
//
// Parameters:
// 参数：
//   - key: The key to look up in additional information
//     key：在附加信息中查找的键
//
// Returns:
// 返回：
//   - interface{}: The value associated with the key, or empty string if not found
//     interface{}：与键关联的值，如果未找到则为空字符串
//   - bool: True if the key exists, false otherwise
//     bool：如果键存在则为 true，否则为 false
func (r RuleChainBaseInfo) GetAdditionalInfo(key string) (interface{}, bool) {
	if r.AdditionalInfo == nil {
		return "", false
	}
	v, ok := r.AdditionalInfo[key]
	return v, ok
}

// PutAdditionalInfo adds additional information by key and value.
// PutAdditionalInfo 通过键和值添加附加信息。
//
// This method safely adds or updates additional information, automatically
// initializing the map if it doesn't exist.
// 此方法安全地添加或更新附加信息，如果映射不存在则自动初始化。
//
// Parameters:
// 参数：
//   - key: The key to store the information under
//     key：存储信息的键
//   - value: The value to associate with the key
//     value：与键关联的值
//
// Usage:
// 使用：
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
// RuleMetadata 定义规则链的元数据，包括节点和连接的信息。
//
// This structure contains the detailed topology and routing information that defines
// how messages flow through the rule chain. It includes node definitions, connections
// between nodes, endpoint configurations, and legacy sub-chain connections.
// 此结构包含定义消息如何通过规则链流动的详细拓扑和路由信息。
// 它包括节点定义、节点间连接、端点配置和传统子链连接。
//
// Structural Components:
// 结构组件：
//   - FirstNodeIndex: Entry point identification
//     FirstNodeIndex：入口点标识
//   - Endpoints: External connectivity configuration
//     Endpoints：外部连接配置
//   - Nodes: Processing component definitions
//     Nodes：处理组件定义
//   - Connections: Inter-node message routing
//     Connections：节点间消息路由
//   - RuleChainConnections: Legacy sub-chain integration
//     RuleChainConnections：传统子链集成
type RuleMetadata struct {
	// FirstNodeIndex is the index of the first node in data flow, default is 0.
	// FirstNodeIndex 是数据流中第一个节点的索引，默认为 0。
	//
	// This index identifies the entry point for message processing within the rule chain.
	// It corresponds to the position in the Nodes array where execution begins.
	// 此索引标识规则链内消息处理的入口点。
	// 它对应于执行开始的 Nodes 数组中的位置。
	FirstNodeIndex int `json:"firstNodeIndex"`

	// Endpoints are the component definitions of the endpoints.
	// Endpoints 是端点的组件定义。
	//
	// Endpoints define external connectivity points for the rule chain, including
	// REST APIs, MQTT brokers, WebSocket servers, and other protocol handlers.
	// They serve as the bridge between external systems and the rule processing logic.
	// 端点定义规则链的外部连接点，包括 REST API、MQTT 代理、WebSocket 服务器和其他协议处理程序。
	// 它们作为外部系统和规则处理逻辑之间的桥梁。
	Endpoints []*EndpointDsl `json:"endpoints,omitempty"`

	// Nodes are the component definitions of the nodes.
	// Each object represents a rule node within the rule chain.
	// Nodes 是节点的组件定义。
	// 每个对象表示规则链内的一个规则节点。
	//
	// Nodes define the processing components that transform, filter, route, and
	// act upon messages as they flow through the rule chain. Each node encapsulates
	// specific business logic or integration functionality.
	// 节点定义在消息通过规则链流动时对消息进行转换、过滤、路由和操作的处理组件。
	// 每个节点封装特定的业务逻辑或集成功能。
	Nodes []*RuleNode `json:"nodes"`

	// Connections define the connections between two nodes in the rule chain.
	// Connections 定义规则链中两个节点之间的连接。
	//
	// Connections establish the message flow topology by specifying how messages
	// move from one node to another based on processing results and relationship types.
	// 连接通过指定消息如何基于处理结果和关系类型从一个节点移动到另一个节点来建立消息流拓扑。
	Connections []NodeConnection `json:"connections"`

	// Deprecated: Use Flow Node instead.
	// RuleChainConnections are the connections between a node and a sub-rule chain.
	// 已弃用：改用 Flow Node。
	// RuleChainConnections 是节点和子规则链之间的连接。
	//
	// This field is maintained for backward compatibility with older rule chain
	// definitions that use direct sub-chain connections instead of Flow nodes.
	// 此字段保持与使用直接子链连接而不是 Flow 节点的旧规则链定义的向后兼容性。
	RuleChainConnections []RuleChainConnection `json:"ruleChainConnections,omitempty"`
}

// RuleNode defines the information of a rule chain node.
// RuleNode 定义规则链节点的信息。
//
// RuleNode represents a single processing component within a rule chain. Each node
// encapsulates specific business logic, data transformation, or integration functionality.
// Nodes are connected through relationships to form complete processing workflows.
// RuleNode 表示规则链内的单个处理组件。每个节点封装特定的业务逻辑、
// 数据转换或集成功能。节点通过关系连接形成完整的处理工作流。
//
// Node Lifecycle:
// 节点生命周期：
//  1. Configuration parsing and validation
//     配置解析和验证
//  2. Component initialization with configuration
//     使用配置进行组件初始化
//  3. Message processing during rule execution
//     规则执行期间的消息处理
//  4. Resource cleanup when node is destroyed
//     节点销毁时的资源清理
type RuleNode struct {
	// Id is the unique identifier of the node, which can be any string.
	// Id 是节点的唯一标识符，可以是任何字符串。
	//
	// The ID must be unique within the rule chain and is used for:
	// ID 在规则链内必须唯一，用于：
	//   - Node references in connections
	//     连接中的节点引用
	//   - Debug and monitoring identification
	//     调试和监控标识
	//   - Dynamic node lookup and management
	//     动态节点查找和管理
	//   - Error reporting and logging
	//     错误报告和日志记录
	Id string `json:"id"`

	// AdditionalInfo is an extension field for visualization position information (reserved field).
	// AdditionalInfo 是用于可视化位置信息的扩展字段（保留字段）。
	//
	// This field is primarily used by visual rule chain editors to store
	// UI-specific information such as node positioning, styling, and metadata
	// that doesn't affect rule execution but aids in visual representation.
	// 此字段主要由可视化规则链编辑器用于存储 UI 特定信息，
	// 如节点定位、样式和不影响规则执行但有助于可视化表示的元数据。
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`

	// Type is the type of the node, which determines the logic and behavior of the node. It should match one of the node types registered in the rule engine.
	// Type 是节点的类型，决定节点的逻辑和行为。它应匹配规则引擎中注册的节点类型之一。
	//
	// The type serves as a factory key to create the appropriate node implementation.
	// Common types include filters, transformers, actions, and integrations.
	// 类型作为工厂键来创建适当的节点实现。
	// 常见类型包括过滤器、转换器、动作和集成。
	//
	// Standard Node Types:
	// 标准节点类型：
	//   - jsFilter: JavaScript-based filtering logic
	//     jsFilter：基于 JavaScript 的过滤逻辑
	//   - jsTransform: JavaScript-based data transformation
	//     jsTransform：基于 JavaScript 的数据转换
	//   - restApiCall: HTTP REST API integration
	//     restApiCall：HTTP REST API 集成
	//   - log: Logging and debugging output
	//     log：日志记录和调试输出
	Type string `json:"type"`

	// Name is the name of the node, which can be any string.
	// Name 是节点的名称，可以是任何字符串。
	//
	// The name provides a human-readable identifier for the node, useful for:
	// 名称为节点提供人类可读的标识符，用于：
	//   - Documentation and understanding
	//     文档和理解
	//   - Visual representation in editors
	//     编辑器中的可视化表示
	//   - Debugging and error messages
	//     调试和错误消息
	//   - Administrative and monitoring purposes
	//     管理和监控目的
	Name string `json:"name"`

	// DebugMode indicates whether the node is in debug mode. If true, a debug callback function is triggered when the node processes messages.
	// This setting can be overridden by the RuleChain `DebugMode` configuration.
	// DebugMode 表示节点是否处于调试模式。如果为 true，节点处理消息时触发调试回调函数。
	// 此设置可以被 RuleChain `DebugMode` 配置覆盖。
	//
	// Debug Mode Benefits:
	// 调试模式好处：
	//   - Real-time message flow visibility
	//     实时消息流可见性
	//   - Performance monitoring and profiling
	//     性能监控和分析
	//   - Error tracking and diagnostics
	//     错误跟踪和诊断
	//   - Development and testing support
	//     开发和测试支持
	//
	// Performance Considerations:
	// 性能考虑：
	// Debug callbacks add overhead, so disable in production unless monitoring is required.
	// 调试回调增加开销，因此除非需要监控，否则在生产环境中禁用。
	DebugMode bool `json:"debugMode"`

	// Configuration contains the configuration parameters of the node, which vary depending on the node type.
	// For example, a JS filter node might have a `jsScript` field defining the filtering logic,
	// while a REST API call node might have a `restEndpointUrlPattern` field defining the URL to call.
	// Configuration 包含节点的配置参数，根据节点类型而变化。
	// 例如，JS 过滤器节点可能有定义过滤逻辑的 `jsScript` 字段，
	// 而 REST API 调用节点可能有定义要调用的 URL 的 `restEndpointUrlPattern` 字段。
	//
	// Configuration supports:
	// 配置支持：
	//   - Type-specific parameters for node behavior
	//     节点行为的特定类型参数
	//   - Environment variable substitution (${global.key})
	//     环境变量替换（${global.key}）
	//   - Dynamic configuration updates
	//     动态配置更新
	//
	Configuration Configuration `json:"configuration"`
}

// NodeAdditionalInfo is used for visualization position information (reserved field).
// NodeAdditionalInfo 用于可视化位置信息（保留字段）。
//
// This structure defines the standard additional information fields used by
// visual rule chain editors for node positioning and documentation.
// 此结构定义了可视化规则链编辑器用于节点定位和文档的标准附加信息字段。
type NodeAdditionalInfo struct {
	// Description provides detailed documentation for the node
	// Description 为节点提供详细文档
	Description string `json:"description"`
	// LayoutX represents the horizontal position in the visual editor
	// LayoutX 表示可视化编辑器中的水平位置
	LayoutX int `json:"layoutX"`
	// LayoutY represents the vertical position in the visual editor
	// LayoutY 表示可视化编辑器中的垂直位置
	LayoutY int `json:"layoutY"`
}

// NodeConnection defines the connection between two nodes in a rule chain.
// NodeConnection 定义规则链中两个节点之间的连接。
//
// NodeConnection establishes the message flow topology by specifying how messages
// move from one processing node to another based on the results of message processing.
// The connection type determines the conditions under which messages flow.
// NodeConnection 通过指定消息如何基于消息处理结果从一个处理节点移动到另一个节点来建立消息流拓扑。
// 连接类型决定消息流动的条件。
//
// Connection Flow Logic:
// 连接流逻辑：
//  1. Source node processes message
//     源节点处理消息
//  2. Processing result determines relationship type
//     处理结果决定关系类型
//  3. Message is routed to target node if relationship matches
//     如果关系匹配，消息路由到目标节点
//  4. Multiple connections enable parallel or conditional flows
//     多个连接支持并行或条件流
//
// Common Connection Types:
// 常见连接类型：
//   - Success/Failure: General processing outcomes
//     Success/Failure：一般处理结果
//   - True/False: Boolean logic for filters and conditions
//     True/False：过滤器和条件的布尔逻辑
//   - Custom types: Domain-specific routing logic
//     自定义类型：领域特定的路由逻辑
type NodeConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	// FromId 是源节点的 id，应匹配节点数组中节点的 id。
	//
	// This field establishes the starting point of the message flow connection.
	// The referenced node must exist in the rule chain's node list.
	// 此字段建立消息流连接的起始点。
	// 引用的节点必须存在于规则链的节点列表中。
	FromId string `json:"fromId"`

	// ToId is the id of the target node, which should match the id of a node in the nodes array.
	// ToId 是目标节点的 id，应匹配节点数组中节点的 id。
	//
	// This field establishes the destination of the message flow connection.
	// The referenced node must exist in the rule chain's node list.
	// 此字段建立消息流连接的目标。
	// 引用的节点必须存在于规则链的节点列表中。
	ToId string `json:"toId"`

	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	// For example, a JS filter node might support two connection types: "True" and "False," indicating whether the message passes or fails the filter condition.
	// Type 是连接类型，决定何时以及如何将消息从一个节点发送到另一个节点。它应匹配源节点类型支持的连接类型之一。
	// 例如，JS 过滤器节点可能支持两种连接类型："True" 和 "False"，表示消息是通过还是未通过过滤条件。
	//
	// The type acts as a conditional gate that controls message flow based on
	// processing results. Each node type defines its own set of supported relationship types.
	// 类型作为基于处理结果控制消息流的条件门。
	// 每种节点类型定义自己支持的关系类型集。
	Type string `json:"type"`

	// Label is the label of the connection, used for display.
	// Label 是连接的标签，用于显示。
	//
	// The label provides a human-readable description of the connection,
	// useful for visual editors and documentation purposes.
	// 标签提供连接的人类可读描述，
	// 对于可视化编辑器和文档目的很有用。
	Label string `json:"label,omitempty"`
}

// RuleChainConnection defines the connection between a node and a sub-rule chain.
// RuleChainConnection 定义节点和子规则链之间的连接。
//
// This structure represents the legacy way of connecting to sub-rule chains
// directly. Modern implementations should use Flow nodes instead for better
// flexibility and consistency.
// 此结构表示直接连接到子规则链的传统方式。
// 现代实现应使用 Flow 节点以获得更好的灵活性和一致性。
type RuleChainConnection struct {
	// FromId is the id of the source node, which should match the id of a node in the nodes array.
	// FromId 是源节点的 id，应匹配节点数组中节点的 id。
	FromId string `json:"fromId"`
	// ToId is the id of the target sub-rule chain, which should match one of the sub-rule chains registered in the rule engine.
	// ToId 是目标子规则链的 id，应匹配规则引擎中注册的子规则链之一。
	ToId string `json:"toId"`
	// Type is the type of connection, which determines when and how messages are sent from one node to another. It should match one of the connection types supported by the source node type.
	// Type 是连接类型，决定何时以及如何将消息从一个节点发送到另一个节点。它应匹配源节点类型支持的连接类型之一。
	Type string `json:"type"`
}

// RuleChainRunSnapshot is a snapshot of the rule chain execution log.
// RuleChainRunSnapshot 是规则链执行日志的快照。
//
// This structure captures the complete execution trace of a rule chain run,
// including timing information, node execution logs, and metadata.
// It's primarily used for debugging, monitoring, and audit purposes.
// 此结构捕获规则链运行的完整执行跟踪，
// 包括时间信息、节点执行日志和元数据。
// 它主要用于调试、监控和审计目的。
//
// Snapshot Use Cases:
// 快照用例：
//   - Performance analysis and optimization
//     性能分析和优化
//   - Error investigation and debugging
//     错误调查和调试
//   - Audit trails for compliance
//     合规审计跟踪
//   - Execution monitoring and alerting
//     执行监控和警报
type RuleChainRunSnapshot struct {
	// Deprecated: User ctx.RuleChain() instead.
	// 已弃用：使用 ctx.RuleChain() 代替。
	RuleChain
	// Id is the execution ID.
	// Id 是执行 ID。
	//
	// This unique identifier tracks a specific rule chain execution instance,
	// enabling correlation across distributed systems and log aggregation.
	// 此唯一标识符跟踪特定的规则链执行实例，
	// 支持跨分布式系统的关联和日志聚合。
	Id string `json:"id"`
	// StartTs is the start time of execution.
	// StartTs 是执行开始时间。
	//
	// Timestamp when the rule chain execution began, used for performance
	// measurement and timing analysis.
	// 规则链执行开始的时间戳，用于性能测量和时序分析。
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	// EndTs 是执行结束时间。
	//
	// Timestamp when the rule chain execution completed, used for calculating
	// total execution duration and performance analysis.
	// 规则链执行完成的时间戳，用于计算总执行持续时间和性能分析。
	EndTs int64 `json:"endTs"`
	// Logs are the logs for each node.
	// Logs 是每个节点的日志。
	//
	// Detailed execution logs for each node that processed messages during
	// this rule chain run, providing fine-grained visibility into the execution flow.
	// 在此规则链运行期间处理消息的每个节点的详细执行日志，
	// 提供对执行流的细粒度可见性。
	Logs []RuleNodeRunLog `json:"logs"`
	// AdditionalInfo is an extension field.
	// AdditionalInfo 是扩展字段。
	//
	// Extensible field for storing custom metadata, monitoring data,
	// or integration-specific information related to this execution.
	// 用于存储与此执行相关的自定义元数据、监控数据或集成特定信息的可扩展字段。
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// RuleNodeRunLog is the log for a node.
// RuleNodeRunLog 是节点的日志。
//
// This structure captures detailed execution information for a single node
// during rule chain processing, including input/output messages, timing,
// errors, and custom log entries.
// 此结构捕获规则链处理期间单个节点的详细执行信息，
// 包括输入/输出消息、时序、错误和自定义日志条目。
//
// Log Analysis Applications:
// 日志分析应用：
//   - Performance bottleneck identification
//     性能瓶颈识别
//   - Message transformation tracking
//     消息转换跟踪
//   - Error pattern analysis
//     错误模式分析
//   - Compliance and audit reporting
//     合规和审计报告
type RuleNodeRunLog struct {
	// Id is the node ID.
	// Id 是节点 ID。
	//
	// Identifier of the node that generated this log entry,
	// corresponding to the node's ID in the rule chain definition.
	// 生成此日志条目的节点标识符，
	// 对应于规则链定义中节点的 ID。
	Id string `json:"nodeId"`
	// InMsg is the input message.
	// InMsg 是输入消息。
	//
	// The message that was received by this node for processing,
	// capturing the state before node execution.
	// 此节点接收用于处理的消息，
	// 捕获节点执行前的状态。
	InMsg RuleMsg `json:"inMsg"`
	// OutMsg is the output message.
	// OutMsg 是输出消息。
	//
	// The message that was produced by this node after processing,
	// showing any transformations or modifications made.
	// 此节点处理后产生的消息，
	// 显示所做的任何转换或修改。
	OutMsg RuleMsg `json:"outMsg"`
	// RelationType is the connection type with the next node.
	// RelationType 是与下一个节点的连接类型。
	//
	// The relationship type that determined the routing of the output message
	// to subsequent nodes in the rule chain.
	// 决定输出消息路由到规则链中后续节点的关系类型。
	RelationType string `json:"relationType"`
	// Err is the error information.
	// Err 是错误信息。
	//
	// Textual representation of any error that occurred during node execution,
	// useful for debugging and error analysis.
	// 节点执行期间发生的任何错误的文本表示，
	// 对于调试和错误分析很有用。
	Err string `json:"err"`
	// LogItems are the logs during execution.
	// LogItems 是执行期间的日志。
	//
	// Custom log entries generated by the node during processing,
	// providing detailed visibility into internal operations.
	// 节点在处理期间生成的自定义日志条目，
	// 提供对内部操作的详细可见性。
	LogItems []string `json:"logItems"`
	// StartTs is the start time of execution.
	// StartTs 是执行开始时间。
	//
	// Timestamp when this node began processing the message.
	// 此节点开始处理消息的时间戳。
	StartTs int64 `json:"startTs"`
	// EndTs is the end time of execution.
	// EndTs 是执行结束时间。
	//
	// Timestamp when this node completed processing the message,
	// used for calculating node-level execution duration.
	// 此节点完成处理消息的时间戳，
	// 用于计算节点级执行持续时间。
	EndTs int64 `json:"endTs"`
}

// EndpointDsl defines the DSL for an endpoint.
// EndpointDsl 定义端点的 DSL。
//
// EndpointDsl extends RuleNode with endpoint-specific functionality, combining
// node behavior with external connectivity capabilities. It serves as the bridge
// between external systems and rule chain processing logic.
// EndpointDsl 扩展了 RuleNode，增加了端点特定功能，结合了
// 节点行为和外部连接功能。它作为外部系统和规则链处理逻辑之间的桥梁。
//
// Endpoint Architecture:
// 端点架构：
//   - Node foundation: Inherits all RuleNode capabilities
//     节点基础：继承所有 RuleNode 功能
//   - Protocol handling: Support for various communication protocols
//     协议处理：支持各种通信协议
//   - Request routing: Flexible routing based on request characteristics
//     请求路由：基于请求特征的灵活路由
//   - Processing pipeline: Configurable processors for request/response handling
//     处理管道：请求/响应处理的可配置处理器
//
// Supported Protocols:
// 支持的协议：
//   - HTTP/HTTPS: REST APIs, webhooks, web services
//     HTTP/HTTPS：REST API、webhook、Web 服务
//   - MQTT: IoT messaging and device communication
//     MQTT：IoT 消息传递和设备通信
//   - WebSocket: Real-time bidirectional communication
//     WebSocket：实时双向通信
//   - TCP/UDP: Custom protocol implementations
//     TCP/UDP：自定义协议实现
type EndpointDsl struct {
	RuleNode
	// Processors is the list of global processors for the endpoint.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors 是端点的全局处理器列表。
	// 使用按名称在 builtin/processor#Builtins xx 中注册的处理器。
	//
	// Processors provide pre/post-processing capabilities for endpoint requests,
	// enabling cross-cutting concerns like authentication, validation, logging,
	// and format conversion to be applied consistently across all routes.
	// 处理器为端点请求提供预/后处理功能，
	// 使认证、验证、日志记录和格式转换等横切关注点
	// 能够在所有路由中一致应用。
	//
	// Common Global Processors:
	// 常见全局处理器：
	//   - auth: Authentication and authorization
	//     auth：认证和授权
	//   - validate: Request validation and sanitization
	//     validate：请求验证和清理
	//   - cors: Cross-Origin Resource Sharing handling
	//     cors：跨域资源共享处理
	//   - rateLimit: Request rate limiting and throttling
	//     rateLimit：请求速率限制和节流
	//   - compress: Response compression
	//     compress：响应压缩
	Processors []string `json:"processors"`

	// Routers is the list of routers.
	// Routers 是路由器列表。
	//
	// Routers define the specific routing rules that determine how incoming
	// requests are matched and processed. Each router can have its own
	// configuration, processors, and destination rules.
	// 路由器定义确定传入请求如何匹配和处理的特定路由规则。
	// 每个路由器可以有自己的配置、处理器和目标规则。
	//
	// Router Organization:
	// 路由器组织：
	//   - Pattern matching: URL patterns, method filtering, header matching
	//     模式匹配：URL 模式、方法过滤、标头匹配
	//   - Parameter extraction: Path variables, query parameters
	//     参数提取：路径变量、查询参数
	//   - Content handling: Request/response format negotiation
	//     内容处理：请求/响应格式协商
	//   - Conditional routing: Dynamic routing based on request content
	//     条件路由：基于请求内容的动态路由
	Routers []*RouterDsl `json:"routers"`
}

// RouterDsl defines a router for an endpoint.
// RouterDsl 定义端点的路由器。
//
// RouterDsl represents a single routing rule within an endpoint that defines
// how specific requests should be processed and where they should be forwarded.
// It provides fine-grained control over request matching, processing, and routing.
// RouterDsl 表示端点内的单个路由规则，定义特定请求应如何处理以及应转发到何处。
// 它提供对请求匹配、处理和路由的细粒度控制。
//
// Routing Flow:
// 路由流程：
//  1. Request arrives at endpoint
//     请求到达端点
//  2. Router parameters match request characteristics
//     路由器参数匹配请求特征
//  3. From configuration processes the request
//     From 配置处理请求
//  4. Message is forwarded to To destination
//     消息转发到 To 目标
//  5. Response is processed and returned
//     响应被处理并返回
//
// Routing Strategies:
// 路由策略：
//   - Path-based: Route by URL path patterns
//     基于路径：按 URL 路径模式路由
//   - Method-based: Route by HTTP methods (GET, POST, etc.)
//     基于方法：按 HTTP 方法路由（GET、POST 等）
//   - Header-based: Route by request headers or content types
//     基于标头：按请求标头或内容类型路由
//   - Content-based: Route by request body content
//     基于内容：按请求正文内容路由
type RouterDsl struct {
	// Id is the router ID, optional and by default uses From.Path.
	// Id 是路由器 ID，可选，默认使用 From.Path。
	//
	// The ID provides a unique identifier for the router within the endpoint,
	// useful for debugging, monitoring, and dynamic router management.
	// If not specified, the system uses the From.Path as the identifier.
	// ID 为端点内的路由器提供唯一标识符，
	// 对于调试、监控和动态路由器管理很有用。
	// 如果未指定，系统使用 From.Path 作为标识符。
	Id string `json:"id"`

	// Params is the parameters for the router.
	// HTTP Endpoint router params is POST/GET/PUT...
	// Params 是路由器的参数。
	// HTTP 端点路由器参数是 POST/GET/PUT...
	//
	// Parameters define the matching criteria for incoming requests.
	// The format and meaning of parameters depend on the endpoint type:
	// 参数定义传入请求的匹配条件。
	// 参数的格式和含义取决于端点类型：
	//
	// HTTP Parameters:
	// HTTP 参数：
	//   - HTTP methods: ["GET", "POST", "PUT", "DELETE"]
	//     HTTP 方法：["GET", "POST", "PUT", "DELETE"]
	//   - Content types: ["application/json", "text/plain"]
	//     内容类型：["application/json", "text/plain"]
	//
	// MQTT Parameters:
	// MQTT 参数：
	//   - QoS levels: [0, 1, 2]
	//     QoS 级别：[0, 1, 2]
	//   - Retained flag: [true, false]
	//     保留标志：[true, false]
	Params []interface{} `json:"params"`

	// From is the source for the router.
	// From 是路由器的源。
	//
	// The From configuration defines how incoming requests are received,
	// processed, and prepared for routing to the destination.
	// From 配置定义如何接收、处理传入请求并为路由到目标做准备。
	From FromDsl `json:"from"`

	// To is the destination for the router.
	// To 是路由器的目标。
	//
	// The To configuration defines where processed requests should be forwarded
	// and how responses should be handled and returned to the client.
	// To 配置定义处理的请求应转发到何处，以及如何处理响应并返回给客户端。
	To ToDsl `json:"to"`

	// AdditionalInfo is an extension field.
	// AdditionalInfo 是扩展字段。
	//
	// This field provides extensibility for custom router metadata,
	// monitoring data, or protocol-specific configuration.
	// 此字段为自定义路由器元数据、监控数据或协议特定配置提供可扩展性。
	AdditionalInfo map[string]interface{} `json:"additionalInfo,omitempty"`
}

// FromDsl defines the source for an endpoint router.
// FromDsl 定义端点路由器的源。
//
// FromDsl configures how incoming requests are received and initially processed
// before being forwarded to the rule chain or component destination. It defines
// the request reception pattern, processing pipeline, and data extraction rules.
// FromDsl 配置如何接收和初始处理传入请求，然后转发到规则链或组件目标。
// 它定义请求接收模式、处理管道和数据提取规则。
//
// Source Processing Pipeline:
// 源处理管道：
//  1. Request reception: Accept incoming requests matching the path pattern
//     请求接收：接受匹配路径模式的传入请求
//  2. Preprocessing: Apply source-specific processors
//     预处理：应用源特定处理器
//  3. Data extraction: Extract relevant data from the request
//     数据提取：从请求中提取相关数据
//  4. Message creation: Create RuleMsg for rule chain processing
//     消息创建：为规则链处理创建 RuleMsg
//
// Path Pattern Examples:
// 路径模式示例：
//   - Static paths: "/api/users", "/webhook/github"
//     静态路径："/api/users"、"/webhook/github"
//   - Parameterized: "/api/users/{id}", "/orders/{orderId}/items"
//     参数化："/api/users/{id}"、"/orders/{orderId}/items"
//   - Wildcards: "/files/*", "/api/v1/**"
//     通配符："/files/*"、"/api/v1/**"
//   - MQTT topics: "sensor/+/temperature", "devices/+/+/telemetry"
//     MQTT 主题："sensor/+/temperature"、"devices/+/+/telemetry"
type FromDsl struct {
	// Path is the path of the source.
	// Path 是源的路径。
	//
	// The path defines the pattern that incoming requests must match to be
	// processed by this router. The format depends on the endpoint protocol:
	// 路径定义传入请求必须匹配的模式才能被此路由器处理。
	// 格式取决于端点协议：
	//
	// HTTP Paths:
	// HTTP 路径：
	//   - Support path parameters with {} syntax
	//     支持使用 {} 语法的路径参数
	//   - Wildcard matching with * and **
	//     使用 * 和 ** 的通配符匹配
	//   - Query parameter extraction
	//     查询参数提取
	//
	// MQTT Topics:
	// MQTT 主题：
	//   - Single-level wildcard: +
	//     单级通配符：+
	//   - Multi-level wildcard: #
	//     多级通配符：#
	//   - Topic parameter extraction
	//     主题参数提取
	Path string `json:"path"`

	// Configuration is the configuration for the source.
	// Configuration 是源的配置。
	//
	// Source-specific configuration that controls how requests are received
	// and initially processed. Common configurations include timeouts,
	// buffer sizes, validation rules, and protocol-specific settings.
	// 控制如何接收和初始处理请求的源特定配置。
	// 常见配置包括超时、缓冲区大小、验证规则和协议特定设置。
	//
	// HTTP Configuration:
	// HTTP 配置：
	//   - maxRequestSize: Maximum request body size
	//     maxRequestSize：最大请求正文大小
	//   - timeout: Request timeout duration
	//     timeout：请求超时持续时间
	//   - cors: CORS policy configuration
	//     cors：CORS 策略配置
	//
	// MQTT Configuration:
	// MQTT 配置：
	//   - qos: Quality of Service level
	//     qos：服务质量级别
	//   - retained: Message retention flag
	//     retained：消息保留标志
	//   - clientId: MQTT client identifier
	//     clientId：MQTT 客户端标识符
	Configuration Configuration `json:"configuration"`

	// Processors is the list of processors for the source.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors 是源的处理器列表。
	// 使用按名称在 builtin/processor#Builtins xx 中注册的处理器。
	//
	// Source processors handle request preprocessing before the message
	// is forwarded to the destination. They can modify, validate, or
	// enrich the incoming request data.
	// 源处理器在消息转发到目标之前处理请求预处理。
	// 它们可以修改、验证或丰富传入的请求数据。
	//
	// Common Source Processors:
	// 常见源处理器：
	//   - auth: Authentication verification
	//     auth：认证验证
	//   - validate: Request validation
	//     validate：请求验证
	//   - transform: Data format transformation
	//     transform：数据格式转换
	//   - enrich: Data enrichment from external sources
	//     enrich：来自外部源的数据丰富
	Processors []string `json:"processors"`
}

// ToDsl defines the destination for an endpoint router.
// ToDsl 定义端点路由器的目标。
//
// ToDsl configures where processed requests should be forwarded and how
// responses should be handled. It supports various destination types including
// rule chains, components, and external services, with flexible response handling.
// ToDsl 配置处理的请求应转发到何处以及如何处理响应。
// 它支持各种目标类型，包括规则链、组件和外部服务，具有灵活的响应处理。
//
// Destination Types:
// 目标类型：
//   - Rule chains: Forward to complete rule processing workflows
//     规则链：转发到完整的规则处理工作流
//   - Components: Direct component execution
//     组件：直接组件执行
//   - External services: Proxy to external APIs
//     外部服务：代理到外部 API
//   - Custom handlers: User-defined processing logic
//     自定义处理程序：用户定义的处理逻辑
//
// Response Handling:
// 响应处理：
//   - Synchronous: Wait for processing completion and return response
//     同步：等待处理完成并返回响应
//   - Asynchronous: Fire-and-forget processing
//     异步：即发即忘处理
//   - Streaming: Real-time response streaming
//     流式：实时响应流
//   - Callback: Response via callback mechanisms
//     回调：通过回调机制的响应
type ToDsl struct {
	// Path is the path of the executor for the destination.
	// For example, "chain:default" to execute by a rule chain for `default`, "component:jsTransform" to execute a JS transform component.
	// Path 是目标执行器的路径。
	// 例如，"chain:default" 表示由 `default` 规则链执行，"component:jsTransform" 表示执行 JS 转换组件。
	//
	// Path Format and Examples:
	// 路径格式和示例：
	//   - Rule chain execution: "chain:{chainId}"
	//     规则链执行："chain:{chainId}"
	//   - Component execution: "component:{componentType}"
	//     组件执行："component:{componentType}"
	//   - Node execution: "node:{nodeId}"
	//     节点执行："node:{nodeId}"
	//   - External service: "http://external-api.com/endpoint"
	//     外部服务："http://external-api.com/endpoint"
	//   - Custom handler: "handler:{handlerName}"
	//     自定义处理程序："handler:{handlerName}"
	//
	// The path determines how the rule engine interprets and routes
	// the processed message for execution.
	// 路径决定规则引擎如何解释和路由处理的消息以供执行。
	Path string `json:"path"`

	// Configuration is the configuration for the destination.
	// Configuration 是目标的配置。
	//
	// Destination-specific configuration that controls how the message
	// is processed at the destination and how responses are handled.
	// 控制消息在目标处如何处理以及如何处理响应的目标特定配置。
	//
	// Common Configuration Options:
	// 常见配置选项：
	//   - timeout: Processing timeout duration
	//     timeout：处理超时持续时间
	//   - retries: Number of retry attempts on failure
	//     retries：失败时的重试次数
	//   - headers: Additional headers for external services
	//     headers：外部服务的附加标头
	//   - authentication: Authentication credentials
	//     authentication：认证凭据
	Configuration Configuration `json:"configuration"`

	// Wait indicates whether to wait for the 'To' executor to finish before proceeding.
	// Wait 表示是否等待 'To' 执行器完成后再继续。
	//
	// This flag controls the execution mode and response handling:
	// 此标志控制执行模式和响应处理：
	//
	// Synchronous (Wait = true):
	// 同步（Wait = true）：
	//   - Wait for destination processing to complete
	//     等待目标处理完成
	//   - Return the actual processing result to client
	//     向客户端返回实际处理结果
	//   - Higher latency but guaranteed response
	//     较高延迟但保证响应
	//   - Suitable for request-response patterns
	//     适用于请求-响应模式
	//
	// Asynchronous (Wait = false):
	// 异步（Wait = false）：
	//   - Immediately return acknowledgment to client
	//     立即向客户端返回确认
	//   - Process request in background
	//     在后台处理请求
	//   - Lower latency but fire-and-forget
	//     较低延迟但即发即忘
	//   - Suitable for event processing and webhooks
	//     适用于事件处理和 webhook
	Wait bool `json:"wait"`

	// Processors is the list of processors for the destination.
	// Using processors registered in builtin/processor#Builtins xx by name.
	// Processors 是目标的处理器列表。
	// 使用按名称在 builtin/processor#Builtins xx 中注册的处理器。
	//
	// Destination processors handle response postprocessing after the
	// destination has completed processing. They can transform, format,
	// or enhance the response before it's returned to the client.
	// 目标处理器在目标完成处理后处理响应后处理。
	// 它们可以在响应返回给客户端之前转换、格式化或增强响应。
	//
	// Common Destination Processors:
	// 常见目标处理器：
	//   - format: Response format conversion (JSON, XML, etc.)
	//     format：响应格式转换（JSON、XML 等）
	//   - cache: Response caching
	//     cache：响应缓存
	//   - compress: Response compression
	//     compress：响应压缩
	//   - audit: Response auditing and logging
	//     audit：响应审计和日志记录
	//   - metrics: Performance metrics collection
	//     metrics：性能指标收集
	Processors []string `json:"processors"`
}
