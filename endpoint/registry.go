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
	"strings"

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
// init 向默认 Registry 注册所有内置端点组件。
// 此初始化自动注册以下端点类型：
// • endpoint/mqtt：用于物联网消息传递的 MQTT 客户端端点
// • endpoint/rest：HTTP/REST API 服务器端点
// • endpoint/net：TCP/UDP 网络服务器端点
// • endpoint/websocket：WebSocket 服务器端点
// • endpoint/schedule：基于定时器的消息生成端点
func init() {
	_ = Registry.Register(&mqtt.Endpoint{})
	_ = Registry.Register(&rest.Endpoint{})
	_ = Registry.Register(&net.Endpoint{})
	_ = Registry.Register(&websocket.Endpoint{})
	_ = Registry.Register(&schedule.Endpoint{})
}

// Registry is the default global registry for endpoint components.
// It provides a centralized way to register and create endpoint instances.
// All built-in endpoint types are automatically registered during initialization.
//
// Registry 是端点组件的默认全局注册表。
// 它提供了注册和创建端点实例的集中化方式。
// 所有内置端点类型在初始化期间自动注册。
var Registry = new(ComponentRegistry)

// ComponentRegistry is a registry for endpoint components that manages
// the registration and creation of different endpoint types.
// It extends the base RuleComponentRegistry to provide endpoint-specific functionality.
//
// ComponentRegistry 是端点组件的注册表，管理不同端点类型的注册和创建。
// 它扩展了基础的 RuleComponentRegistry 以提供端点特定的功能。
//
// Architecture / 架构：
// • Component Registration: Maps endpoint type names to their implementations  组件注册：将端点类型名称映射到其实现
// • Instance Creation: Creates new endpoint instances with proper configuration  实例创建：使用适当配置创建新的端点实例
// • Type Compatibility: Handles backward compatibility for older type names  类型兼容性：处理旧类型名称的向后兼容性
type ComponentRegistry struct {
	engine.RuleComponentRegistry
}

// Register adds a new endpoint component to the registry.
// The component must implement the endpoint.Endpoint interface.
//
// Register 向注册表添加新的端点组件。
// 组件必须实现 endpoint.Endpoint 接口。
//
// Parameters / 参数：
// • component: The endpoint component implementation  端点组件实现
//
// Returns / 返回值：
// • error: Registration error if any  注册错误（如果有）
//
// Usage Example / 使用示例：
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
// New 根据组件类型创建端点的新实例。
// 它支持新旧类型命名约定以保持向后兼容性。
//
// Parameters / 参数：
// • componentType: The type identifier of the endpoint (e.g., "endpoint/mqtt", "mqtt")  端点的类型标识符
// • ruleConfig: Rule engine configuration  规则引擎配置
// • configuration: Component-specific configuration (types.Configuration or struct)  组件特定配置
//
// Returns / 返回值：
// • endpoint.Endpoint: The created endpoint instance  创建的端点实例
// • error: Creation error if any  创建错误（如果有）
//
// Type Naming / 类型命名：
// • New format: "endpoint/mqtt", "endpoint/rest", etc.  新格式：带 endpoint/ 前缀
// • Legacy format: "mqtt", "rest", "http", "ws", etc.  旧格式：直接类型名称
//
// Configuration Types / 配置类型：
// The configuration parameter can be either:  配置参数可以是：
// • types.Configuration: Generic key-value configuration  通用键值配置
// • Specific Config struct: Type-specific configuration structure  特定配置结构体
//
// Usage Example / 使用示例：
//
//	// Create MQTT endpoint  创建 MQTT 端点
//	mqttEndpoint, err := Registry.New("endpoint/mqtt", config, types.Configuration{
//	    "server": "127.0.0.1:1883",
//	    "clientId": "rulego-client",
//	})
//
//	// Create REST endpoint  创建 REST 端点
//	restEndpoint, err := Registry.New("endpoint/rest", config, types.Configuration{
//	    "server": ":9090",
//	})
func (r *ComponentRegistry) New(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	// Handle backward compatibility for legacy type names
	// 处理旧类型名称的向后兼容性
	if strings.Contains("http,ws,mqtt,net,schedule,kafka,nats", componentType) {
		//Compatible with older versions  兼容旧版本
		componentType = types.EndpointTypePrefix + componentType
	}

	// Create new node instance from registry  从注册表创建新节点实例
	newNode, err := r.RuleComponentRegistry.NewNode(componentType)
	if err != nil {
		return nil, err
	}

	// Process configuration parameter  处理配置参数
	var config = make(types.Configuration)
	if configuration != nil {
		if c, ok := configuration.(types.Configuration); ok {
			config = c
		} else if err = maps.Map2Struct(configuration, &config); err != nil {
			return nil, err
		}
	}

	// Initialize endpoint with configuration  使用配置初始化端点
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
