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

// Package external provides components for interacting with external systems and services in the RuleGo rule engine.
// These components enable rule chains to communicate with databases, message brokers, APIs, networks,
// and remote systems, expanding the rule engine's integration capabilities.
//
// Package external 为 RuleGo 规则引擎提供与外部系统和服务交互的组件。
// 这些组件使规则链能够与数据库、消息代理、API、网络和远程系统通信，
// 扩展规则引擎的集成能力。
//
// Available Components:
// 可用组件：
//
// Database Components:
// 数据库组件：
//   - DbClientNode: Connect to database via Go standard database/sql interface
//
// Message Broker Components:
// 消息代理组件：
//   - MqttClientNode: MQTT broker connectivity for IoT and messaging
//     MQTT 代理连接，用于物联网和消息传递
//
// Network Components:
// 网络组件：
//   - NetNode: TCP/UDP/Unix socket communication with various protocols
//     TCP/UDP/Unix 套接字通信，支持各种协议
//   - RestApiCallNode: HTTP/REST API client for web service integration
//     HTTP/REST API 客户端，用于 Web 服务集成
//
// Remote Execution Components:
// 远程执行组件：
//   - SshNode: SSH-based remote command execution
//     基于 SSH 的远程命令执行
//
// Cache Management Components:
// 缓存管理组件：
//   - CacheGetNode: Retrieve data from chain-level or global cache
//     从链级或全局缓存中检索数据
//   - CacheSetNode: Store data in cache with TTL support
//     在缓存中存储数据，支持 TTL
//   - CacheDeleteNode: Remove data from cache with pattern matching
//     从缓存中删除数据，支持模式匹配
//
// Component Categories:
// 组件分类：
//
// Data Integration:
// 数据集成：
//   - Database operations with SQL support
//     支持 SQL 的数据库操作
//   - Cache management for data persistence
//     数据持久化的缓存管理
//
// Communication:
// 通信：
//   - MQTT messaging for IoT scenarios
//     物联网场景的 MQTT 消息传递
//   - HTTP/REST API calls for web integration
//     Web 集成的 HTTP/REST API 调用
//   - Raw network protocols for custom communication
//     自定义通信的原始网络协议
//
// Remote Operations:
// 远程操作：
//   - SSH command execution for system administration
//     系统管理的 SSH 命令执行
//
// Registration:
// 注册：
//
// All components are automatically registered during package initialization:
// 所有组件在包初始化期间自动注册：
//
//	func init() {
//		Registry.Add(&DbClientNode{})
//		Registry.Add(&MqttClientNode{})
//		// ... other components
//	}
//
// Example Usage:
// 使用示例：
//
//	// Database query in rule chain
//	// 规则链中的数据库查询
//	{
//		"id": "queryUser",
//		"type": "dbClient",
//		"configuration": {
//			"driverName": "mysql",
//			"dsn": "user:pass@tcp(localhost:3306)/db",
//			"sql": "SELECT * FROM users WHERE id = ?",
//			"params": ["${metadata.userId}"]
//		}
//	}
//
//	// MQTT message publishing
//	// MQTT 消息发布
//	{
//		"id": "publishData",
//		"type": "mqttClient",
//		"configuration": {
//			"server": "mqtt.example.com:1883",
//			"topic": "/sensors/${metadata.deviceId}",
//			"qos": 1
//		}
//	}
//
// For detailed documentation on individual components, see their respective source files.
// 有关各个组件的详细文档，请参见其各自的源文件。
package external
