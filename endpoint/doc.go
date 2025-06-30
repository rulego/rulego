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

// Package endpoint provides input endpoint components for the RuleGo rule engine.
// Endpoints serve as entry points for external data to flow into rule chains,
// enabling integration with various protocols and data sources.
//
// Package endpoint 为 RuleGo 规则引擎提供输入端点组件。
// 端点作为外部数据流入规则链的入口点，支持与各种协议和数据源的集成。
//
// Core Architecture:
// 核心架构：
//
// Package endpoint is a module that abstracts different input source data routing,
// providing a consistent interface for different protocols. It enables RuleGo
// to run independently and provide services through protocol-specific endpoints.
//
// Package endpoint 是一个抽象不同输入源数据路由的模块，
// 为不同协议提供一致的接口。它使 RuleGo 能够独立运行并通过协议特定的端点提供服务。
//
// Built-in Endpoint Types:
// 内置端点类型：
//
// Based on the actual implementation, the following endpoint types are available:
// 基于实际实现，提供以下端点类型：
//
// • RestEndpoint: HTTP/REST API server (endpoint/rest)  HTTP/REST API 服务器
// • MqttEndpoint: MQTT client (endpoint/mqtt)  MQTT 客户端
// • WebsocketEndpoint: WebSocket server (endpoint/websocket)  WebSocket 服务器
// • NetEndpoint: TCP/UDP network server (endpoint/net)  TCP/UDP 网络服务器
// • ScheduleEndpoint: Timer-based message generation (endpoint/schedule)  基于定时器的消息生成
//
// Extended Endpoint Components:
// 扩展端点组件：
//
// The RuleGo ecosystem includes several extension component libraries that provide
// additional endpoint types and specialized components for various scenarios:
// RuleGo 生态系统包含多个扩展组件库，为各种场景提供额外的端点类型和专用组件：
//
// Core Extension Libraries / 核心扩展库：
//
//   - rulego-components: Additional general-purpose endpoint and processing components
//     (https://github.com/rulego/rulego-components)
//     rulego-components：额外的通用端点和处理组件
//     包含 Kafka、Redis、RabbitMQ、NATS、gRPC、FastHTTP 等端点组件
//
// Specialized Extension Libraries / 专用扩展库：
//
//   - rulego-components-ai: AI and machine learning scenario components
//     (https://github.com/rulego/rulego-components-ai)
//     rulego-components-ai：AI 和机器学习场景组件
//     包含智能推理、模型调用、数据预处理等 AI 相关端点和组件
//
//   - rulego-components-ci: CI/CD and DevOps scenario components
//     (https://github.com/rulego/rulego-components-ci)
//     rulego-components-ci：CI/CD 和 DevOps 场景组件
//     包含代码仓库、构建工具、部署平台集成等 CI/CD 相关端点和组件
//
//   - rulego-components-iot: Internet of Things scenario components
//     (https://github.com/rulego/rulego-components-iot)
//     rulego-components-iot：物联网场景组件
//     包含设备连接、协议转换、数据采集等 IoT 相关端点和组件
//
//   - rulego-components-etl: Extract, Transform, Load scenario components
//     (https://github.com/rulego/rulego-components-etl)
//     rulego-components-etl：数据提取、转换、加载场景组件
//     包含数据库连接、文件处理、数据清洗等 ETL 相关端点和组件
//
// Installation and Usage / 安装和使用：
//
// These extension libraries can be imported and used alongside the core RuleGo framework:
// 这些扩展库可以与核心 RuleGo 框架一起导入和使用：
//
//	import (
//	    "github.com/rulego/rulego"
//	    "github.com/rulego/rulego-components/endpoint/kafka"
//	    "github.com/rulego/rulego-components-ai/llm/openai"
//	    "github.com/rulego/rulego-components-ci/git/github"
//	    "github.com/rulego/rulego-components-iot/modbus"
//	    "github.com/rulego/rulego-components-etl/database/mysql"
//	)
//
// Integration with Rule Chains:
// 与规则链的集成：
//
// Endpoints are integrated into rule chains through DSL configuration. The complete
// DSL structure includes both the rule chain definition and endpoint configuration:
//
// 端点通过 DSL 配置集成到规则链中。完整的 DSL 结构包括规则链定义和端点配置：
//
//	{
//	  "ruleChain": {
//	    "id": "test-chain",
//	    "name": "Test Chain",
//	    "debugMode": true,
//	    "root": true
//	  },
//	  "metadata": {
//	    "firstNodeIndex": 0,
//	    "endpoints": [
//	      {
//	        "id": "endpoint_1",
//	        "type": "endpoint/mqtt",
//	        "name": "MQTT Subscriber",
//	        "configuration": {
//	          "server": "127.0.0.1:1883"
//	        },
//	        "routers": [
//	          {
//	            "from": {
//	              "path": "device/+/msg"
//	            },
//	            "to": {
//	              "path": "test-chain:node_1"
//	            }
//	          }
//	        ]
//	      }
//	    ],
//	    "nodes": [
//	      {
//	        "id": "node_1",
//	        "type": "jsTransform",
//	        "name": "Transform Message",
//	        "configuration": {
//	          "jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//	        }
//	      }
//	    ],
//	    "connections": []
//	  }
//	}
//
// Router Configuration:
// 路由器配置：
//
// Each endpoint can define multiple routers that map input paths to rule chain nodes.
// The router structure varies by endpoint type:
//
// 每个端点可以定义多个路由器，将输入路径映射到规则链节点。
// 路由器结构因端点类型而异：
//
// • from.path: Input pattern specific to the endpoint type  特定于端点类型的输入模式
//   - HTTP: URL path pattern (e.g., "/api/v1/msg")  URL 路径模式
//   - MQTT: Topic pattern (e.g., "device/+/msg")  主题模式
//   - Schedule: Cron expression (e.g., "0 */5 * * * *")  Cron 表达式
//   - TCP/UDP: Message pattern  消息模式
//
// • to.path: Target rule chain node in format "chainId:nodeId"  目标规则链节点，格式为 "chainId:nodeId"
//
// • params: Protocol-specific parameters (e.g., HTTP methods)  协议特定参数（例如，HTTP 方法）
//
// Complete Example with Redis Endpoint:
// Redis 端点完整示例：
//
// The following example shows a Redis pub/sub endpoint integrated with a rule chain:
// 以下示例显示了与规则链集成的 Redis 发布/订阅端点：
//
//	{
//	  "ruleChain": {
//	    "id": "redis-chain",
//	    "name": "Redis Pub/Sub",
//	    "debugMode": true,
//	    "root": true
//	  },
//	  "metadata": {
//	    "firstNodeIndex": 0,
//	    "endpoints": [
//	      {
//	        "id": "redis_endpoint",
//	        "type": "endpoint/redis",
//	        "name": "Redis Subscriber",
//	        "configuration": {
//	          "server": "127.0.0.1:6379",
//	          "db": 0
//	        },
//	        "routers": [
//	          {
//	            "from": {
//	              "path": "device/msg"
//	            },
//	            "to": {
//	              "path": "redis-chain:transform_node"
//	            }
//	          },
//	          {
//	            "from": {
//	              "path": "system/alert"
//	            },
//	            "to": {
//	              "path": "redis-chain:alert_node"
//	            }
//	          }
//	        ]
//	      }
//	    ],
//	    "nodes": [
//	      {
//	        "id": "transform_node",
//	        "type": "jsTransform",
//	        "name": "Transform Device Message",
//	        "configuration": {
//	          "jsScript": "return {'processed': true, 'data': msg.data};"
//	        }
//	      },
//	      {
//	        "id": "alert_node",
//	        "type": "jsFilter",
//	        "name": "Alert Filter",
//	        "configuration": {
//	          "jsScript": "return msg.severity === 'critical';"
//	        }
//	      }
//	    ],
//	    "connections": [
//	      {
//	        "fromId": "alert_node",
//	        "toId": "transform_node",
//	        "type": "True"
//	      }
//	    ]
//	  }
//	}
//
// Dynamic Management:
// 动态管理：
//
// Endpoints support dynamic lifecycle management through the Pool interface:
// 端点通过 Pool 接口支持动态生命周期管理：
//
// • Creation from DSL configuration  从 DSL 配置创建
// • Hot reloading of configuration  配置热重载
// • Router addition and modification  路由器添加和修改
// • Graceful shutdown and cleanup  优雅关闭和清理
//
// Message Flow:
// 消息流：
//
// 1. External data arrives at the endpoint  外部数据到达端点
// 2. Endpoint converts data to RuleMsg format  端点将数据转换为 RuleMsg 格式
// 3. Router matches input pattern and routes to target node  路由器匹配输入模式并路由到目标节点
// 4. Rule chain processes the message  规则链处理消息
// 5. Results can be sent back through the endpoint if needed  如果需要，可以通过端点发送回结果
//
// For detailed implementation examples and advanced usage patterns,
// see the test files and example directories.
// 有关详细的实现示例和高级使用模式，请参见测试文件和示例目录。
package endpoint
