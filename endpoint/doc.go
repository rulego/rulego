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
// Package endpoint provides input endpoint components for the RuleGo rule engine.
// Endpoints serve as entry points for external data flowing into the rule chain, supporting integration with various protocols and data sources.
//
// Core Architecture:
// Core Architecture:
//
// Package endpoint is a module that abstracts different input source data routing,
// providing a consistent interface for different protocols. It enables RuleGo
// to run independently and provide services through protocol-specific endpoints.
//
// The Package endpoint is a module that abstracts routing data from different input sources,
// Provides consistent interfaces for different protocols. It enables RuleGo to operate independently and provide services through protocol-specific endpoints.
//
// Built-in Endpoint Types:
// Types of built-in endpoints:
//
// Based on the actual implementation, the following endpoint types are available:
// Based on actual implementations, the following endpoint types are provided:
//
// • RestEndpoint: HTTP/REST API server (endpoint/rest) HTTP/REST API server
// • MqttEndpoint: MQTT client (endpoint/mqtt) MQTT client
// • WebsocketEndpoint: WebSocket server (endpoint/websocket) The WebSocket server
// • NetEndpoint: TCP/UDP network server (endpoint/net) is a TCP/UDP network server
// • ScheduleEndpoint: Timer-based message generation (endpoint/schedule)
//
// Extended Endpoint Components:
// Extended endpoint components:
//
// The RuleGo ecosystem includes several extension component libraries that provide
// additional endpoint types and specialized components for various scenarios:
// The RuleGo ecosystem includes multiple library of extended components, providing additional endpoint types and dedicated components for various scenarios:
//
// Core Extension Libraries
//
//   - rulego-components: Additional general-purpose endpoint and processing components
//     (https://github.com/rulego/rulego-components)
//     rulego-components: additional general endpoints and processing components
//     Includes endpoint components such as Kafka, Redis, RabbitMQ, NATS, gRPC, FastHTTP, and others
//
// Specialized Extension Libraries
//
//   - rulego-components-ai: AI and machine learning scenario components
//     (https://github.com/rulego/rulego-components-ai)
//     rulego-components-ai: AI and machine learning scenario components
//     Includes AI-related endpoints and components such as intelligent inference, model calls, and data preprocessing
//
//   - rulego-components-ci: CI/CD and DevOps scenario components
//     (https://github.com/rulego/rulego-components-ci)
//     rulego-components-ci: CI/CD and DevOps scenario components
//     Includes code warehouses, build tools, deployment platform integrations, and other CI/CD-related endpoints and components
//
//   - rulego-components-iot: Internet of Things scenario components
//     (https://github.com/rulego/rulego-components-iot)
//     rulego-components-iot: IoT scenario components
//     Includes IoT-related endpoints and components such as device connectivity, protocol conversion, and data collection
//
//   - rulego-components-etl: Extract, Transform, Load scenario components
//     (https://github.com/rulego/rulego-components-etl)
//     rulego-components-etl: Data extraction, transformation, and scene loading components
//     Includes ETL-related endpoints and components such as database connections, file processing, and data cleaning
//
// # Installation and Usage
//
// These extension libraries can be imported and used alongside the core RuleGo framework:
// These extension libraries can be imported and used together with the core RuleGo framework:
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
// Integration with Rule Chains:
//
// Endpoints are integrated into rule chains through DSL configuration. The complete
// DSL structure includes both the rule chain definition and endpoint configuration:
//
// Endpoints are integrated into the rule chain through DSL configuration. A complete DSL structure includes rule chain definition and endpoint configuration:
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
// Router configuration:
//
// Each endpoint can define multiple routers that map input paths to rule chain nodes.
// The router structure varies by endpoint type:
//
// Each endpoint can define multiple routers to map input paths to regular chain nodes.
// Router architecture varies depending on the type of endpoint:
//
// • from.path: Input pattern specific to the endpoint type
//   - HTTP: URL path pattern (e.g., "/api/v1/msg") URL path pattern
//   - MQTT: Topic pattern (e.g., "device/+/msg")
//   - Schedule: Cron expression (e.g., "0 */5 * * * *") Cron expression
//   - TCP/UDP: Message pattern
//
// • to.path: Target rule chain node in format "chainId:nodeId"
//
// • params: Protocol-specific parameters (e.g., HTTP methods)
//
// Complete Example with Redis Endpoint:
// Complete example of the Redis endpoint:
//
// The following example shows a Redis pub/sub endpoint integrated with a rule chain:
// The following example shows Redis publish/subscribe endpoints integrated with Rule Chain:
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
// Dynamic management:
//
// Endpoints support dynamic lifecycle management through the Pool interface:
// Endpoints support dynamic lifecycle management via the Pool interface:
//
// • Creation from DSL configuration
// • Hot reloading of configuration
// • Router addition and modification
// • Graceful shutdown and cleanup
//
// Message Flow:
// News Flow:
//
// 1. External data arrives at the endpoint
// 2. Endpoint converts data to RuleMsg format
// 3. Router matches input pattern and routes to target node
// 4. Rule chain processes the message
// 5. Results can be sent back through the endpoint if needed
//
// For detailed implementation examples and advanced usage patterns,
// see the test files and example directories.
// For detailed implementation examples and advanced usage patterns, please refer to the test files and sample directory.
package endpoint
