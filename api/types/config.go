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

package types

import (
	"math"
	"time"

	"github.com/rulego/rulego/utils/pool"
)

// OnDebug is a global debug callback function for nodes.
// OnDebug 是节点的全局调试回调函数。
var OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)

// Config defines the configuration for the rule engine.
// Config 定义规则引擎的配置。
//
// This structure contains all the necessary configuration parameters for initializing
// and running a RuleGo rule engine instance. It provides control over execution behavior,
// resource management, debugging, scripting, and integration with external systems.
// 此结构包含初始化和运行 RuleGo 规则引擎实例所需的所有配置参数。
// 它提供对执行行为、资源管理、调试、脚本和与外部系统集成的控制。
//
// Configuration Categories:
// Usage Example:
// 使用示例：
//
//	config := NewConfig(
//	    WithPool(myPool),
//	    WithLogger(myLogger),
//	    WithOnDebug(debugHandler),
//	)
//	engine := rulego.New("chainId", chainDSL, rulego.WithConfig(config))
type Config struct {
	// OnDebug is a callback function for node debug information. It is only called if the node's debugMode is set to true.
	// - ruleChainId: The ID of the rule chain.
	// - flowType: The event type, either IN (incoming) or OUT (outgoing) for the component.
	// - nodeId: The ID of the node.
	// - msg: The current message being processed.
	// - relationType: If flowType is IN, it represents the connection relation between the previous node and this node (e.g., True/False).
	//                 If flowType is OUT, it represents the connection relation between this node and the next node (e.g., True/False).
	// - err: Error information, if any.
	// OnDebug 是节点调试信息的回调函数。仅在节点的 debugMode 设置为 true 时调用。
	// - ruleChainId：规则链的 ID
	// - flowType：事件类型，组件的 IN（传入）或 OUT（传出）
	// - nodeId：节点的 ID
	// - msg：正在处理的当前消息
	// - relationType：如果 flowType 是 IN，表示前一个节点与此节点之间的连接关系（如 True/False）
	//                如果 flowType 是 OUT，表示此节点与下一个节点之间的连接关系（如 True/False）
	// - err：错误信息（如果有）
	//
	// This callback is essential for development, testing, and production monitoring.
	// It provides real-time visibility into message flow and transformation within rule chains.
	// 此回调对于开发、测试和生产监控至关重要。
	// 它提供规则链内消息流和转换的实时可见性。
	OnDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)
	// OnEnd 规则链子链执行完全局回调
	OnEnd OnEndFunc
	// ScriptMaxExecutionTime is the maximum execution time for scripts, defaulting to 2000 milliseconds.
	// ScriptMaxExecutionTime 是脚本的最大执行时间，默认为 2000 毫秒。
	//
	// This setting prevents runaway scripts from consuming excessive resources or causing
	// system hangs. It applies to JavaScript components
	// When a script exceeds this time limit, it will be terminated and an error will be returned.
	// 此设置防止失控脚本消耗过多资源或导致系统挂起。
	// 它适用于JavaScript脚本的组件。
	// 当脚本超过此时间限制时，将被终止并返回错误。
	//
	ScriptMaxExecutionTime time.Duration
	// Pool is the interface for a coroutine pool. If not configured, the go func method is used by default.
	// The default implementation is `pool.WorkerPool`. It is compatible with ants coroutine pool and can be implemented using ants.
	// Example:
	//   pool, _ := ants.NewPool(math.MaxInt32)
	//   config := rulego.NewConfig(types.WithPool(pool))
	// Pool 是协程池的接口。如果未配置，默认使用 go func 方法。
	// 默认实现是 `pool.WorkerPool`。它与 ants 协程池兼容，可以使用 ants 实现。
	// 示例：
	//   pool, _ := ants.NewPool(math.MaxInt32)
	//   config := rulego.NewConfig(types.WithPool(pool))
	//
	Pool Pool
	// ComponentsRegistry is the component registry for managing available rule chain components.
	// ComponentsRegistry 是管理可用规则链组件的组件注册表。
	//
	// 主要特性 - Key Features:
	//   - 组件隔离：支持不同引擎实例使用独立的组件集合 - Component isolation: supports different engine instances using independent component sets
	//   - 动态管理：运行时注册/注销组件和插件加载 - Dynamic management: runtime component registration/unregistration and plugin loading
	//   - 可视化支持：提供UI配置工具所需的组件元数据 - Visual support: provides component metadata for UI configuration tools
	//
	// 配置示例 - Configuration Examples:
	//
	//	// 使用自定义组件注册表 - Use custom component registry
	//	customRegistry := components.NewRegistry()
	//	customRegistry.Register(&MyCustomNode{})
	//	config := rulego.NewConfig(types.WithComponentsRegistry(customRegistry))
	//
	//	// 动态加载插件 - Dynamic plugin loading
	//	registry.RegisterPlugin("myPlugin", "./plugins/custom.so")
	//
	// 默认使用 `rulego.Registry`，包含所有标准组件。详细功能请参见 ComponentRegistry 接口文档。
	// Defaults to `rulego.Registry` with all standard components. See ComponentRegistry interface for detailed functionality.
	//
	ComponentsRegistry ComponentRegistry
	// Parser is the rule chain parser interface, defaulting to `rulego.JsonParser`.
	// Parser 是规则链解析器接口，默认为 `rulego.JsonParser`。
	//
	// The parser converts rule chain definitions from various formats (JSON, YAML, XML)
	// into internal data structures that the engine can execute.
	// 解析器将规则链定义从各种格式（JSON、YAML、XML）转换为
	// 引擎可以执行的内部数据结构。
	//
	// Custom parsers can be implemented to support:
	// 可以实现自定义解析器来支持：
	//   - Domain-specific configuration languages
	//     领域特定的配置语言
	//   - Legacy configuration formats
	//     传统配置格式
	//   - Compressed or encrypted rule definitions
	//     压缩或加密的规则定义
	//   - Runtime rule generation from databases
	//     从数据库运行时生成规则
	Parser Parser
	// Logger is the logging interface, defaulting to `DefaultLogger()`.
	// Logger 是日志接口，默认为 `DefaultLogger()`。
	//
	// The logger provides structured logging capabilities for the rule engine,
	// supporting different log levels and output formats.
	// 日志记录器为规则引擎提供结构化日志功能，
	Logger Logger
	// Properties are global properties in key-value format.
	// Rule chain node configurations can replace values with ${global.propertyKey}.
	// Replacement occurs during node initialization and only once.
	// Properties 是键值格式的全局属性。
	// 规则链节点配置可以用 ${global.propertyKey} 替换值。
	// 替换在节点初始化期间发生，只发生一次。
	//
	// Example usage in rule configuration:
	// 规则配置中的示例用法：
	//   {
	//     "type": "restApiCall",
	//     "configuration": {
	//       "restEndpointUrlPattern": "${global.apiBaseUrl}/users"
	//     }
	//   }
	Properties Properties
	// Udf is a map for registering custom Golang functions and native scripts that can be called at runtime by script engines like JavaScript Lua.
	// Function names can be repeated for different script types.
	// Udf 是用于注册自定义 Golang 函数和原生脚本的映射，可以在运行时被 JavaScript Lua 等脚本引擎调用。
	// 不同脚本类型的函数名可以重复。
	//
	// UDF (User Defined Functions) extend the scripting capabilities by providing:
	// UDF（用户定义函数）通过提供以下功能扩展脚本功能：
	//   - Access to Go standard library functions
	//     访问 Go 标准库函数
	//   - Custom business logic implementation
	//     自定义业务逻辑实现
	//   - Integration with external systems
	//     与外部系统集成
	//   - Performance-critical operations in native code
	//     原生代码中的性能关键操作
	//
	// Function registration example:
	// 函数注册示例：
	//   config.RegisterUdf("encrypt", func(data string) string {
	//       // Custom encryption logic
	//       return encryptedData
	//   })
	Udf map[string]interface{}
	// SecretKey is an AES-256 key of 32 characters in length, used for decrypting the `Secrets` configuration in the rule chain.
	// SecretKey 是长度为 32 个字符的 AES-256 密钥，用于解密规则链中的 `Secrets` 配置。
	SecretKey string
	// EndpointEnabled indicates whether the endpoint module in the rule chain DSL is enabled.
	// When enabled, the rule chain DSL can configure input endpoint components for external message ingestion.
	// EndpointEnabled 表示是否启用规则链 DSL 中的端点模块。
	// 启用时，规则链 DSL 可以配置输入端点组件用于外部消息接入。
	//
	// DSL配置示例 - DSL configuration example:
	//	{
	//	  "ruleChain": {...},
	//	  "endpoints": [{
	//	    "id": "restEndpoint",
	//	    "type": "rest",
	//	    "configuration": {"port": 8080}
	//	  }]
	//	}
	EndpointEnabled bool
	// NodePool is the interface for a shared Component Pool.
	// NodePool 是共享组件池的接口。
	//
	// The network pool manages shared network resources such as HTTP clients,
	// database connections, and message queue connections across multiple rule chains.
	// This enables resource reuse and connection pooling for improved performance.
	// 网络池管理多个规则链间的共享网络资源，
	// 如 HTTP 客户端、数据库连接和消息队列连接。
	// 这支持资源重用和连接池以提高性能。
	NodePool NodePool
	// NodeClientInitNow indicates whether to initialize the net client node immediately after creation.
	//True: During the component's Init phase, the client connection is established. If the client initialization fails, the rule chain initialization fails.
	//False: During the component's OnMsg phase, the client connection is established.
	// NodeClientInitNow 表示是否在创建后立即初始化网络客户端节点。
	// True：在组件的 Init 阶段建立客户端连接。如果客户端初始化失败，规则链初始化失败。
	// False：在组件的 OnMsg 阶段建立客户端连接。
	NodeClientInitNow bool
	// AllowCycle indicates whether nodes in the rule chain are allowed to form cycles.
	// AllowCycle 表示是否允许规则链中的节点形成循环。
	AllowCycle bool
	// Cache is a global cache instance shared across all rule chains in the pool, used for storing runtime shared data.
	// Cache 是池中所有规则链共享的全局缓存实例，用于存储运行时共享数据。
	//
	// 缓存实现 - Cache Implementation:
	//   - 默认实现：使用内存缓存 (utils/cache.MemoryCache) - Default: uses in-memory cache (utils/cache.MemoryCache)
	//   - 自定义实现：用户可实现 Cache 接口使用外部缓存 - Custom: users can implement Cache interface for external cache
	//   - 支持Redis、Memcached等分布式缓存 - Supports Redis, Memcached and other distributed caches
	//
	// 默认配置 - Default Configuration:
	//   如果未指定，系统将使用 cache.DefaultCache（内存缓存，5分钟GC周期）
	//   If not specified, system uses cache.DefaultCache (in-memory cache with 5-minute GC cycle)
	//
	// 自定义示例 - Custom Examples:
	//
	//	// 使用Redis缓存 - Using Redis cache
	//	redisCache := &MyRedisCache{client: redisClient}
	//	config := NewConfig(WithCache(redisCache))
	//
	//	// 使用内存缓存（自定义GC间隔）- Using memory cache (custom GC interval)
	//	memCache := cache.NewMemoryCache(time.Minute * 10)
	//	config := NewConfig(WithCache(memCache))
	Cache Cache
}

// RegisterUdf registers a custom function. Function names can be repeated for different script types.
// RegisterUdf 注册自定义函数。不同脚本类型的函数名可以重复。
//
// This method provides a convenient way to register User Defined Functions (UDFs) that can be
// called from script components. It handles function name resolution and conflict prevention
// for different script engines.
// 此方法提供了注册用户定义函数（UDF）的便捷方式，这些函数可以从脚本组件中调用。
// 它处理不同脚本引擎的函数名解析和冲突预防。
//
// Function Registration Process:
// 函数注册过程：
//  1. Initialize Udf map if not already created
//     如果尚未创建则初始化 Udf 映射
//  2. Check if value is a Script type with specific engine
//     检查值是否是具有特定引擎的 Script 类型
//  3. Resolve naming conflicts using script type prefixes
//     使用脚本类型前缀解决命名冲突
//  4. Store function with resolved name
//     使用解析的名称存储函数
//
// Examples:
// 示例：
//
//	// Register a Go function for all script types
//	// 为所有脚本类型注册 Go 函数
//	config.RegisterUdf("stringUtils", myStringUtilsFunc)
//
//	// Register a JavaScript-specific function
//	// 注册 JavaScript 特定函数
//	config.RegisterUdf("jsHelper", Script{
//	    Type: "Js",
//	    Content: "function jsHelper(data) { return data.toUpperCase(); }"
//	})
//
//	// Register a Lua-specific function
//	// 注册 Lua 特定函数
//	config.RegisterUdf("luaHelper", Script{
//	    Type: "Lua",
//	    Content: "function luaHelper(data) return string.upper(data) end"
//	})
func (c *Config) RegisterUdf(name string, value interface{}) {
	if c.Udf == nil {
		c.Udf = make(map[string]interface{})
	}
	if script, ok := value.(Script); ok {
		if script.Type != AllScript {
			// Resolve function name conflicts for different script types.
			// 解决不同脚本类型的函数名冲突。
			name = script.Type + ScriptFuncSeparator + name
		}
	}
	c.Udf[name] = value
}

// NewConfig creates a new Config with default values and applies the provided options.
// NewConfig 创建具有默认值的新 Config 并应用提供的选项。
//
// This function implements the functional options pattern, allowing for flexible
// and extensible configuration. It sets reasonable defaults while enabling
// 此函数实现函数式选项模式，允许灵活和可扩展的配置。
//
// Usage Examples:
// 使用示例：
//
//	// Basic configuration with defaults
//	// 具有默认值的基本配置
//	config := NewConfig()
//
//	// Configuration with custom options
//	// 具有自定义选项的配置
//	config := NewConfig(
//	    WithPool(customPool),
//	    WithLogger(customLogger),
//	    WithScriptMaxExecutionTime(5 * time.Second),
//	    WithEndpointEnabled(false),
//	)
func NewConfig(opts ...Option) Config {
	c := &Config{
		ScriptMaxExecutionTime: time.Millisecond * 2000,
		Logger:                 DefaultLogger(),
		Properties:             NewProperties(),
		EndpointEnabled:        true,
	}

	for _, opt := range opts {
		_ = opt(c)
	}
	return *c
}

// DefaultPool provides a default coroutine pool.
// DefaultPool 提供默认协程池。
//
// This function creates and returns a default WorkerPool implementation with
// virtually unlimited capacity (math.MaxInt32 workers). The pool is immediately
// 此函数创建并返回具有几乎无限容量（math.MaxInt32 个工作器）的默认 WorkerPool 实现。
func DefaultPool() Pool {
	wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
	wp.Start()
	return wp
}
