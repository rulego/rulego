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

package endpoint

import (
	"fmt"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/mqtt"
	"github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/endpoint/schedule"
	"github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/maps"
)

// init registers all built-in endpoint components with the default Registry.
// This initialization automatically registers the following endpoint types:
// • endpoint/mqtt: MQTT client endpoint for IoT messaging
// • endpoint/rest: HTTP/REST API server endpoint
// • endpoint/net: TCP/UDP network server endpoint
// • endpoint/websocket: WebSocket server endpoint
// • endpoint/schedule: Timer-based message generation endpoint
//
// init registers all built-in endpoint components with the default Registry.
// This initialization automatically registers the following endpoint types:
// • endpoint/mqtt: MQTT client endpoints for IoT messaging
// • endpoint/rest: HTTP/REST API server endpoint
// • endpoint/net: TCP/UDP network server endpoint
// • endpoint/websocket: The endpoint of the WebSocket server
// • endpoint/schedule: Message generation endpoint based on timers
func init() {
	_ = Registry.Register(&mqtt.Endpoint{})
	_ = Registry.Register(&rest.Endpoint{})
	_ = Registry.Register(&net.Endpoint{})
	_ = Registry.Register(&net.ClientEndpoint{})
	_ = Registry.Register(&websocket.Endpoint{})
	_ = Registry.Register(&websocket.ClientEndpoint{})
	_ = Registry.Register(&schedule.Endpoint{})

	// Register aliases for backward compatibility
	// Register aliases to maintain backward compatibility
	// Note: rest.Type = "endpoint/http", websocket.Type = "endpoint/ws"
	_ = Registry.RegisterAlias(rest.Type, "rest", "http")
	_ = Registry.RegisterAlias(websocket.Type, "websocket", "ws")
	_ = Registry.RegisterAlias(mqtt.Type, "mqtt")
	_ = Registry.RegisterAlias(net.Type, "net", "tcp")
	_ = Registry.RegisterAlias(schedule.Type, "schedule", "timer")
}

// Registry is the default global registry for endpoint components.
// It provides a centralized way to register and create endpoint instances.
// All built-in endpoint types are automatically registered during initialization.
//
// Registry is the default global registry for endpoint components.
// It provides a centralized way to register and create endpoint instances.
// All built-in endpoint types are automatically registered during initialization.
var Registry = new(ComponentRegistry)

// ComponentRegistry is a registry for endpoint components that manages
// the registration and creation of different endpoint types.
// It extends the base RuleComponentRegistry to provide endpoint-specific functionality.
//
// ComponentRegistry is a registry of endpoint components, managing the registration and creation of different endpoint types.
// It extends the basic RuleComponentRegistry to provide endpoint-specific functionality.
//
// Architecture
// • Component Registration: Maps endpoint type names to their implementations
// • Instance Creation: Creates new endpoint instances with proper configuration
// • Type Compatibility: Handles backward compatibility for older type names
type ComponentRegistry struct {
	engine.RuleComponentRegistry
}

// Register adds a new endpoint component to the registry.
// The component must implement the endpoint.Endpoint interface.
//
// Register: Add new endpoint components to the registry.
// Components must implement endpoint.Endpoint interfaces.
//
// Parameters
// • component: The endpoint component implementation
//
// Returns
// • error: Registration error if any
//
// Usage Example
//
//	err := Registry.Register(&customEndpoint{})
//	if err != nil {
//	    log.Fatal("Failed to register endpoint:", err)
//	}
func (r *ComponentRegistry) Register(component endpoint.Endpoint) error {
	return r.RuleComponentRegistry.Register(component)
}

// New creates a new instance of an endpoint based on the component type.
// It supports both new and legacy type naming conventions for backward compatibility.
//
// New creates a new instance of the endpoint based on the component type.
// It supports new and old type naming conventions to maintain backward compatibility.
//
// Parameters
// • componentType: The type identifier of the endpoint (e.g., "endpoint/mqtt", "mqtt")
// • ruleConfig: Rule engine configuration
// • configuration: Component-specific configuration (types.Configuration or struct)
//
// Returns
// • endpoint.Endpoint: The created endpoint instance
// • error: Creation error if any
//
// Type Naming
// • New format: "endpoint/mqtt", "endpoint/rest", etc.
// • Legacy format: "mqtt", "rest", "http", "ws", etc.
//
// Configuration Types
// The configuration parameter can be either:
// • types.Configuration: Generic key-value configuration
// • Specific Config struct: Type-specific configuration structure
//
// Usage Example
//
//	// Create MQTT endpoint
//	mqttEndpoint, err := Registry.New("endpoint/mqtt", config, types.Configuration{
//	    "server": "127.0.0.1:1883",
//	    "clientId": "rulego-client",
//	})
//
//	// Create REST endpoint
//	restEndpoint, err := Registry.New("endpoint/rest", config, types.Configuration{
//	    "server": ":9090",
//	})
func (r *ComponentRegistry) New(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	// Create new node instance from registry
	// Alias resolution is handled by the underlying RuleComponentRegistry
	newNode, err := r.RuleComponentRegistry.NewNode(componentType)
	if err != nil {
		return nil, err
	}

	// Process configuration parameter
	var config = make(types.Configuration)
	if configuration != nil {
		if c, ok := configuration.(types.Configuration); ok {
			config = c
		} else if err = maps.Map2Struct(configuration, &config); err != nil {
			return nil, err
		}
	}

	// Initialize endpoint with configuration
	if ep, ok := newNode.(endpoint.Endpoint); ok {
		if err = ep.Init(ruleConfig, config); err != nil {
			return nil, err
		} else {
			return ep, nil
		}
	} else {
		return nil, fmt.Errorf("%s not type of Net", componentType)
	}
}
