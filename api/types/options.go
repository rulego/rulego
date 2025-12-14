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

import (
	"math"
	"time"

	"github.com/rulego/rulego/utils/pool"
)

// Option is a function type that modifies the Config.
// Option 是修改 Config 的函数类型。
//
// The Option pattern provides a flexible and extensible way to configure RuleGo instances.
// It allows users to specify only the configuration aspects they need while maintaining
// default values for other settings.
// Option 模式提供了配置 RuleGo 实例的灵活和可扩展方式。
// 它允许用户仅指定他们需要的配置方面，同时为其他设置保持默认值。
//
// Usage Pattern:
// 使用模式：
//
//	config := NewConfig(
//	    WithPool(customPool),
//	    WithLogger(customLogger),
//	    WithOnDebug(debugHandler),
//	)
type Option func(*Config) error

// WithComponentsRegistry is an option that sets the components' registry of the Config.
// WithComponentsRegistry 是设置 Config 组件注册表的选项。
//
// The components registry manages all available node types that can be used in rule chains.
// Setting a custom registry allows for component isolation, versioning, and custom component sets.
// 组件注册表管理规则链中可使用的所有可用节点类型。
// 设置自定义注册表允许组件隔离、版本控制和自定义组件集。
//
// Use Cases:
// 使用案例：
//   - Multi-tenant applications with different component sets per tenant
//     每个租户具有不同组件集的多租户应用程序
//   - Plugin-based architectures with dynamic component loading
//     具有动态组件加载的基于插件的架构
//   - Testing environments with mock components
//     具有模拟组件的测试环境
//   - Component versioning and A/B testing
//     组件版本控制和 A/B 测试
//
// Example:
// 示例：
//
//	registry := &MyCustomRegistry{}
//	registry.Register(&MyCustomNode{})
//	config := NewConfig(WithComponentsRegistry(registry))
func WithComponentsRegistry(componentsRegistry ComponentRegistry) Option {
	return func(c *Config) error {
		c.ComponentsRegistry = componentsRegistry
		return nil
	}
}

// WithOnDebug is an option that sets the on debug callback of the Config.
// WithOnDebug 是设置 Config 调试回调的选项。
//
// The debug callback provides real-time visibility into message flow and processing
// within rule chains. It's essential for development, testing, and production monitoring.
// 调试回调提供规则链内消息流和处理的实时可见性。
// 它对于开发、测试和生产监控至关重要。
//
// Callback Parameters:
// 回调参数：
//   - ruleChainId: Identifier of the rule chain processing the message
//     ruleChainId：处理消息的规则链标识符
//   - flowType: Direction of message flow (IN/OUT)
//     flowType：消息流方向（IN/OUT）
//   - nodeId: Identifier of the node processing the message
//     nodeId：处理消息的节点标识符
//   - msg: The message being processed
//     msg：正在处理的消息
//   - relationType: Relationship type determining the flow path
//     relationType：确定流路径的关系类型
//   - err: Any error that occurred during processing
//     err：处理过程中发生的任何错误
//
// Example:
// 示例：
//
//	debugHandler := func(chainId, flowType, nodeId string, msg RuleMsg, relationType string, err error) {
//	    log.Printf("[%s] %s -> %s: %s (%s)", chainId, flowType, nodeId, msg.Type, relationType)
//	    if err != nil {
//	        log.Printf("Error: %v", err)
//	    }
//	}
//	config := NewConfig(WithOnDebug(debugHandler))
func WithOnDebug(onDebug func(ruleChainId string, flowType string, nodeId string, msg RuleMsg, relationType string, err error)) Option {
	return func(c *Config) error {
		c.OnDebug = onDebug
		return nil
	}
}

// WithOnEndGlobal is an option that sets the global on end callback of the Config.
func WithOnEndGlobal(onEnd func(ctx RuleContext, msg RuleMsg, err error, relationType string)) Option {
	return func(c *Config) error {
		c.OnEnd = onEnd
		return nil
	}
}

// WithOnEndWithFailure is an option that sets the OnEndWithFailure of the Config.
// WithOnEndWithFailure 是设置 Config 的 OnEndWithFailure 的选项。
//
// If true, the OnEnd callback will be triggered when no connected node is found and the relation type is Failure.
// 如果为 true，当没有找到连接的节点，并且关系类型为 Failure 时，触发 OnEnd 回调。
func WithOnEndWithFailure(onEndWithFailure bool) Option {
	return func(c *Config) error {
		c.OnEndWithFailure = onEndWithFailure
		return nil
	}
}

// WithPool is an option that sets the pool of the Config.
// WithPool 是设置 Config 池的选项。
//
// The worker pool controls concurrency and resource usage for rule chain execution.
// Proper pool configuration is crucial for performance and stability in production environments.
// 工作器池控制规则链执行的并发性和资源使用。
//
// Example:
// 示例：
//
//	// Bounded pool for production
//	// 生产环境的有界池
//	pool := &pool.WorkerPool{MaxWorkersCount: 100}
//	pool.Start()
//	config := NewConfig(WithPool(pool))
//
//	// Ants pool integration
//	// Ants 池集成
//	antsPool, _ := ants.NewPool(50)
//	config := NewConfig(WithPool(antsPool))
func WithPool(pool Pool) Option {
	return func(c *Config) error {
		c.Pool = pool
		return nil
	}
}

// WithNodePool is an option that sets the netPool of the Config.
// WithNodePool 是设置 Config 网络池的选项。
//
// The network pool manages shared network resources like HTTP clients, database connections,
// and message queue connections across multiple rule chains. This enables resource reuse
// and connection pooling for improved performance and efficiency.
// 网络池管理多个规则链间的共享网络资源，如 HTTP 客户端、数据库连接和消息队列连接。
// 这支持资源重用和连接池以提高性能和效率。
//
// Benefits of Network Pooling:
// 网络池的好处：
//   - Reduced connection establishment overhead
//     减少连接建立开销
//   - Better resource utilization and limits
//     更好的资源利用率和限制
//   - Consistent connection management
//     一致的连接管理
//   - Simplified configuration across chains
//     跨链的简化配置
//
// Common Use Cases:
// 常见用例：
//   - Database connection pooling
//     数据库连接池
//   - Message queue connection sharing
//     消息队列连接共享
func WithNodePool(pool NodePool) Option {
	return func(c *Config) error {
		c.NodePool = pool
		return nil
	}
}

// WithDefaultPool creates an option that sets a default worker pool with unlimited capacity.
// WithDefaultPool 创建一个设置具有无限容量的默认工作器池的选项。
func WithDefaultPool() Option {
	return func(c *Config) error {
		wp := &pool.WorkerPool{MaxWorkersCount: math.MaxInt32}
		wp.Start()
		c.Pool = wp
		return nil
	}
}

// WithScriptMaxExecutionTime is an option that sets the js max execution time of the Config.
// WithScriptMaxExecutionTime 是设置 Config 脚本最大执行时间的选项。
//
// This setting controls the maximum time allowed for script execution in script-enabled
// components (JavaScript). It prevents runaway scripts from consuming
// excessive resources or causing system hangs.
// 此设置控制支持脚本的组件（JavaScript）中脚本执行的最大允许时间。
//
// Example:
// 示例：
//
//	// Development environment with generous timeout
//	// 具有宽松超时的开发环境
//	config := NewConfig(WithScriptMaxExecutionTime(5 * time.Second))
func WithScriptMaxExecutionTime(scriptMaxExecutionTime time.Duration) Option {
	return func(c *Config) error {
		c.ScriptMaxExecutionTime = scriptMaxExecutionTime
		return nil
	}
}

// WithParser is an option that sets the parser of the Config.
// WithParser 是设置 Config 解析器的选项。
//
// The parser converts rule chain definitions from various formats (JSON, YAML, XML)
// into internal data structures. Custom parsers enable support for different
// configuration languages and specialized formats.
// 解析器将规则链定义从各种格式（JSON、YAML、XML）转换为内部数据结构。
// 自定义解析器支持不同的配置语言和专用格式。
//
// Parser Capabilities:
// 解析器功能：
//   - Bidirectional conversion (encode/decode)
//     双向转换（编码/解码）
//   - Format validation and error reporting
//     格式验证和错误报告
//   - Custom extension support
//     自定义扩展支持
//   - Schema validation
//     模式验证
//
// Common Parser Types:
// 常见解析器类型：
//   - JSON Parser: Default, widely supported
//     JSON 解析器：默认，广泛支持
//   - YAML Parser: Human-readable, configuration-friendly
//     YAML 解析器：人类可读，配置友好
//   - XML Parser: Enterprise integration, legacy systems
//     XML 解析器：企业集成，传统系统
//   - Binary Parser: Performance-optimized formats
//     二进制解析器：性能优化格式
//
// Custom Parser Implementation:
// 自定义解析器实现：
// Implement the Parser interface to support custom formats or add
// validation, transformation, or encryption capabilities.
// 实现 Parser 接口以支持自定义格式或添加验证、转换或加密功能。
//
// Example:
// 示例：
//
//	// YAML parser for configuration-friendly format
//	// 用于配置友好格式的 YAML 解析器
//	yamlParser := &YamlParser{}
//	config := NewConfig(WithParser(yamlParser))
//
//	// Encrypted parser for sensitive configurations
//	// 用于敏感配置的加密解析器
//	encryptedParser := &EncryptedJsonParser{Key: secretKey}
//	config := NewConfig(WithParser(encryptedParser))
func WithParser(parser Parser) Option {
	return func(c *Config) error {
		c.Parser = parser
		return nil
	}
}

// WithLogger is an option that sets the logger of the Config.
// WithLogger 是设置 Config 日志记录器的选项。
//
// Example:
// 示例：
//
//	// Logrus integration
//	// Logrus 集成
//	logrusLogger := &LogrusLogger{Logger: logrus.New()}
//	config := NewConfig(WithLogger(logrusLogger))
//
//	// Custom logger with monitoring integration
//	// 具有监控集成的自定义日志记录器
//	monitoringLogger := &MonitoringLogger{Service: "rulego"}
//	config := NewConfig(WithLogger(monitoringLogger))
func WithLogger(logger Logger) Option {
	return func(c *Config) error {
		c.Logger = logger
		return nil
	}
}

// WithSecretKey is an option that sets the secret key of the Config.
// WithSecretKey 是设置 Config 密钥的选项。
func WithSecretKey(secretKey string) Option {
	return func(c *Config) error {
		c.SecretKey = secretKey
		return nil
	}
}

// WithEndpointEnabled creates an Option to enable or disable the endpoint functionality in the Config.
// WithEndpointEnabled 创建一个在 Config 中启用或禁用端点功能的选项。
func WithEndpointEnabled(endpointEnabled bool) Option {
	return func(c *Config) error {
		c.EndpointEnabled = endpointEnabled
		return nil
	}
}

// WithCache is an option that sets the cache of the Config.
// WithCache 是设置 Config 缓存的选项。
func WithCache(cache Cache) Option {
	return func(c *Config) error {
		c.Cache = cache
		return nil
	}
}
