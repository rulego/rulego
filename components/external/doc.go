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
// Package external provides components for the RuleGo rule engine that interact with external systems and services.
// These components enable the rule chain to communicate with databases, message proxies, APIs, networks, and remote systems,
// Expand the integration capabilities of the rule engine.
//
// Available Components:
// Available components:
//
// Database Components:
// Database components:
//   - DbClientNode: Connect to database via Go standard database/sql interface
//
// Message Broker Components:
// Message Proxy Components:
//   - MqttClientNode: MQTT broker connectivity for IoT and messaging
//     MQTT proxy connection for IoT and messaging
//
// Network Components:
// Network Components:
//   - NetNode: TCP/UDP/Unix socket communication with various protocols
//     TCP/UDP/Unix socket communication, supporting various protocols
//   - RestApiCallNode: HTTP/REST API client for web service integration
//     HTTP/REST API client for web service integration
//
// Remote Execution Components:
// Remote execution components:
//   - SshNode: SSH-based remote command execution
//     Remote command execution based on SSH
//
// Cache Management Components:
// Cache management components:
//   - CacheGetNode: Retrieve data from chain-level or global cache
//     Data is retrieved from chain-level or global caches
//   - CacheSetNode: Store data in cache with TTL support
//     Data is stored in cache and supports TTL
//   - CacheDeleteNode: Remove data from cache with pattern matching
//     Data is deleted from the cache, supporting pattern matching
//
// Component Categories:
// Component classification:
//
// Data Integration:
// Data Integration:
//   - Database operations with SQL support
//     Supports SQL database operations
//   - Cache management for data persistence
//     Data persistence and cache management
//
// Communication:
// Communication:
//   - MQTT messaging for IoT scenarios
//     MQTT messaging in IoT scenarios
//   - HTTP/REST API calls for web integration
//     Web integration HTTP/REST API calls
//   - Raw network protocols for custom communication
//     Custom communication with the original network protocol
//
// Remote Operations:
// Remote operation:
//   - SSH command execution for system administration
//     SSH command execution for system administration
//
// Registration:
// Registration:
//
// All components are automatically registered during package initialization:
// All components are automatically registered during package initialization:
//
//	func init() {
//		Registry.Add(&DbClientNode{})
//		Registry.Add(&MqttClientNode{})
//		// ... other components
//	}
//
// Example Usage:
// Example:
//
//	// Database query in rule chain
//	Database queries in the rule chain
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
//	MQTT message release
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
// For detailed documentation of each component, please refer to their respective source files.
package external
