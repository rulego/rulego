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

// Package external provides components for interacting with external systems in the RuleGo rule engine.
// These components facilitate communication and integration with external systems:
//
// - MqttClientNode: Connects to MQTT brokers and publishes messages
// - RestApiCallNode: Performs HTTP requests to external APIs
// - DbClientNode: Connects to databases and performs SQL operations
// Each component is registered with the Registry, allowing them to be used
// within rule chains. These components enable the rule engine to interact
// with external systems, expanding its capabilities for data input, output,
// and integration with other services.
//
// You can use these components in your rule chain DSL file by referencing
// their Type. For example:
//
//	{
//	  "id": "node1",
//	  "type": "restApiCall",
//	  "name": "HTTP Request",
//	  "configuration": {
//	    "restEndpointUrlPattern": "https://api.example.com/data",
//	    "requestMethod": "POST"
//	  }
//	}
package external
