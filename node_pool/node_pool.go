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

// Package node_pool provides shared node resource management for efficient connection reuse
// across different rule chains and components. It enables network connection-type components
// to share their instantiated connection resources (clients) with other components,
// achieving the purpose of saving system resources.
//
// Package node_pool 提供共享节点资源管理，实现不同规则链和组件间的高效连接复用。
// 它使网络连接类型的组件能够将其实例化的连接资源（客户端）与其他组件共享，
// 达到节省系统资源的目的。
//
// Connection Reuse Scenarios:
// 连接复用场景：
//
// Multiple components can reuse the same connection resources:
// 多个组件可以复用相同的连接资源：
//   - Multiple MQTT clients sharing the same MQTT connection  多个MQTT客户端共享同一个MQTT连接
//   - Multiple database operations sharing the same database connection  多个数据库操作共享同一个数据库连接
//   - Multiple HTTP endpoints sharing the same port  多个HTTP端点共享同一个端口
//   - Message queue clients sharing connection pools  消息队列客户端共享连接池
//
// SharedNode Interface Requirement:
// SharedNode 接口要求：
//
// Components that support connection sharing must implement the SharedNode interface:
// 支持连接共享的组件必须实现 SharedNode 接口：
//
//	type SharedNode interface {
//		GetInstance() (interface{}, error)
//		// ... other methods
//	}
//
// Most officially provided network connection components support this pattern.
// 大部分官方提供的网络连接组件都支持这种模式。
//
// Usage Pattern:
// 使用模式：
//
//  1. Initialize shared resource nodes by loading a rule chain definition:
//     通过加载规则链定义初始化共享资源节点：
//
//     node_pool.DefaultNodePool.Load(dsl []byte)
//
//  2. Reference shared resources using the ref://{resourceId} pattern:
//     使用 ref://{resourceId} 模式引用共享资源：
//
//     {
//     "id": "node_2",
//     "type": "mqttClient",
//     "configuration": {
//     "server": "ref://local_mqtt_client",
//     "topic": "/device/msg"
//     }
//     }
//
// Node Pool Configuration Example:
// 节点池配置示例：
//
//	{
//		"ruleChain": {
//			"id": "default_node_pool",
//			"name": "全局共享节点池"
//		},
//		"metadata": {
//			"endpoints": [...],
//			"nodes": [
//				{
//					"id": "local_mqtt_client",
//					"type": "mqttClient",
//					"configuration": {
//						"server": "127.0.0.1:1883"
//					}
//				}
//			]
//		}
//	}
//
// Difference from Node Reference:
// 与节点引用的区别：
//
//   - Node Reference: Completely references the specified node instance, including all configurations
//     节点引用：完全引用指定节点实例，包括节点所有配置
//
//   - Shared Resource Node: Reuses the node's connection instance, but other configurations are independent
//     共享资源节点：复用节点的连接实例，但是节点的其他配置是独立的
//
// For example, with MQTT client nodes:
// 例如，对于MQTT客户端节点：
//   - Shared: Connection configuration (MQTT address, reconnection interval, etc.)
//     共享：连接类配置（MQTT地址、重连间隔等）
//   - Independent: Other configurations like publish topics for each node
//     独立：其他配置如每个节点的发布主题
//
// RuleGo-Server Integration:
// RuleGo-Server 集成：
//
// In RuleGo-Server, configure the node pool file in config.conf:
// 在 RuleGo-Server 中，在 config.conf 中配置节点池文件：
//
//	node_pool_file=./node_pool.json
//
// Thread Safety:
// 线程安全：
//
// The node pool implementation is thread-safe and supports concurrent access
// from multiple goroutines. All operations are protected by appropriate
// synchronization mechanisms.
// 节点池实现是线程安全的，支持多个 goroutine 的并发访问。
// 所有操作都受适当的同步机制保护。
package node_pool

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rulego/rulego/utils/json"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/engine"
)

var (
	// ErrNotImplemented is returned when a component does not implement the SharedNode interface.
	// Only components implementing SharedNode can be added to the node pool for connection sharing.
	//
	// ErrNotImplemented 当组件未实现 SharedNode 接口时返回。
	// 只有实现了 SharedNode 的组件才能添加到节点池中进行连接共享。
	ErrNotImplemented = errors.New("not SharedNode")
)
var _ types.NodePool = (*NodePool)(nil)

// DefaultNodePool is the global default component resource pool manager.
// It provides a convenient singleton instance for managing shared node resources
// across the entire application. Most applications can use this default instance
// without creating custom node pools.
//
// DefaultNodePool 是全局默认组件资源池管理器。
// 它为整个应用程序管理共享节点资源提供便捷的单例实例。
// 大多数应用程序可以使用此默认实例，无需创建自定义节点池。
//
// Usage:
// 使用：
//
//	// Load shared nodes from JSON configuration
//	// 从JSON配置加载共享节点
//	DefaultNodePool.Load(jsonConfig)
//
//	// Get a shared connection instance
//	// 获取共享连接实例
//	if instance, err := DefaultNodePool.GetInstance("local_mqtt_client"); err == nil {
//	    // Use the shared MQTT client
//	}
var DefaultNodePool = NewNodePool(engine.NewConfig())

// NodePool is a thread-safe component resource pool manager that manages shared node instances
// and their connection resources. It enables efficient reuse of network connections and
// other expensive resources across multiple rule chains and components.
//
// NodePool 是线程安全的组件资源池管理器，管理共享节点实例及其连接资源。
// 它使网络连接和其他昂贵资源能够在多个规则链和组件间高效复用。
//
// Key Features:
// 主要功能：
//   - Thread-safe concurrent access to shared resources  线程安全的共享资源并发访问
//   - Support for both endpoint and rule node sharing  支持端点和规则节点的共享
//   - Automatic resource lifecycle management  自动资源生命周期管理
//   - JSON-based configuration loading  基于JSON的配置加载
//   - Dynamic resource addition and removal  动态资源添加和删除
//
// Resource Management:
// 资源管理：
// The pool maintains a mapping of resource IDs to shared node contexts,
// allowing components to reference shared resources by ID using the ref://{resourceId} pattern.
// 池维护资源ID到共享节点上下文的映射，允许组件使用ref://{resourceId}模式通过ID引用共享资源。
type NodePool struct {
	// Config provides the rule engine configuration used for creating and managing shared nodes.
	// This configuration determines how nodes are parsed, initialized, and managed.
	//
	// Config 提供用于创建和管理共享节点的规则引擎配置。
	// 此配置决定节点如何解析、初始化和管理。
	Config types.Config
	// entries is a thread-safe map storing shared node contexts.
	// Key: resourceId (string) - unique identifier for the shared resource
	// Value: *sharedNodeCtx - wrapper containing the shared node and its metadata
	//
	// entries 是存储共享节点上下文的线程安全映射。
	// 键：resourceId (string) - 共享资源的唯一标识符
	// 值：*sharedNodeCtx - 包含共享节点及其元数据的包装器
	entries sync.Map
}

// NewNodePool creates a new node pool instance with the specified configuration.
// The configuration determines how nodes are parsed, initialized, and managed within the pool.
//
// NewNodePool 使用指定配置创建新的节点池实例。
// 配置决定节点在池中如何解析、初始化和管理。
//
// Parameters:
// 参数：
//   - config: Rule engine configuration for node management  用于节点管理的规则引擎配置
//
// Returns:
// 返回：
//   - *NodePool: New node pool instance  新的节点池实例
//
// Usage:
// 使用：
//
//	config := engine.NewConfig()
//	pool := NewNodePool(config)
func NewNodePool(config types.Config) *NodePool {
	return &NodePool{
		Config: config,
	}
}

// Load parses and loads shared node definitions from JSON/DSL configuration data.
// This is the primary method for initializing a node pool from configuration files.
// The DSL should contain a rule chain definition with endpoints and nodes sections.
//
// Load 从JSON/DSL配置数据解析并加载共享节点定义。
// 这是从配置文件初始化节点池的主要方法。
// DSL应包含带有端点和节点部分的规则链定义。
//
// Parameters:
// 参数：
//   - dsl: JSON configuration data defining the shared nodes  定义共享节点的JSON配置数据
//
// Returns:
// 返回：
//   - types.NodePool: The node pool instance (self) for method chaining  用于方法链的节点池实例（自身）
//   - error: Parse or initialization error if any  解析或初始化错误（如果有）
//
// Configuration Format:
// 配置格式：
//
//	{
//	    "ruleChain": {"id": "pool_id", "name": "Pool Name"},
//	    "metadata": {
//	        "endpoints": [{"id": "ep1", "type": "endpoint/type", "configuration": {...}}],
//	        "nodes": [{"id": "node1", "type": "nodeType", "configuration": {...}}]
//	    }
//	}
//
// Error Conditions:
// 错误条件：
//   - Invalid JSON format  无效的JSON格式
//   - Node type not found in registry  注册表中未找到节点类型
//   - Duplicate node IDs  重复的节点ID
//   - Component doesn't implement SharedNode interface  组件未实现SharedNode接口
func (n *NodePool) Load(dsl []byte) (types.NodePool, error) {
	if def, err := n.Config.Parser.DecodeRuleChain(dsl); err != nil {
		return nil, err
	} else {
		return n.LoadFromRuleChain(def)
	}
}

func (n *NodePool) LoadFromRuleChain(def types.RuleChain) (types.NodePool, error) {
	for _, item := range def.Metadata.Endpoints {
		if item != nil {
			if _, err := n.NewFromEndpoint(*item); err != nil {
				return nil, err
			}
		}
	}
	for _, item := range def.Metadata.Nodes {
		if item != nil {
			if _, err := n.NewFromRuleNode(*item); err != nil {
				return nil, err
			}
		}
	}
	return n, nil
}

func (n *NodePool) NewFromEndpoint(def types.EndpointDsl) (types.SharedNodeCtx, error) {
	if _, ok := n.entries.Load(def.Id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", def.Id)
	}

	if ctx, err := endpoint.NewFromDef(types.EndpointDsl{RuleNode: def.RuleNode}, endpointApi.DynamicEndpointOptions.WithRestart(true)); err == nil {
		if _, ok := ctx.Target().(types.SharedNode); !ok {
			return nil, ErrNotImplemented
		} else {
			rCtx := newSharedNodeCtx(nil, ctx)
			n.entries.Store(rCtx.GetNodeId().Id, rCtx)
			return rCtx, nil
		}
	} else {
		return nil, err
	}

}

func (n *NodePool) NewFromRuleNode(def types.RuleNode) (types.SharedNodeCtx, error) {
	if _, ok := n.entries.Load(def.Id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", def.Id)
	}
	if ctx, err := engine.InitNetResourceNodeCtx(n.Config, nil, nil, &def); err == nil {
		if _, ok := ctx.Node.(types.SharedNode); !ok {
			return nil, ErrNotImplemented
		} else {
			rCtx := newSharedNodeCtx(ctx, nil)
			n.entries.Store(rCtx.GetNodeId().Id, rCtx)
			return rCtx, nil
		}
	} else {
		return nil, err
	}
}

func (n *NodePool) AddNode(node types.Node) (types.SharedNodeCtx, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	if endpointNode, ok := node.(endpointApi.Endpoint); ok {
		return n.addEndpointNode(endpointNode)
	} else if nodeCtx, ok := node.(*engine.RuleNodeCtx); ok {
		return n.addNode(nodeCtx)
	} else {
		return nil, fmt.Errorf("node is not endpointApi.Endpoint or *engine.RuleNodeCtx")
	}
}

func (n *NodePool) addEndpointNode(endpointNode endpointApi.Endpoint) (types.SharedNodeCtx, error) {
	id := endpointNode.Id()
	if _, ok := n.entries.Load(id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", id)
	}
	if _, ok := endpointNode.(types.SharedNode); !ok {
		return nil, ErrNotImplemented
	} else {
		rCtx := newSharedNodeCtx(nil, endpointNode)
		n.entries.Store(id, rCtx)
		return rCtx, nil
	}
}

func (n *NodePool) addNode(nodeCtx *engine.RuleNodeCtx) (types.SharedNodeCtx, error) {
	id := nodeCtx.GetNodeId().Id
	if _, ok := n.entries.Load(id); ok {
		return nil, fmt.Errorf("duplicate node id:%s", id)
	}
	if _, ok := nodeCtx.Node.(types.SharedNode); !ok {
		return nil, ErrNotImplemented
	} else {
		rCtx := newSharedNodeCtx(nodeCtx, nil)
		n.entries.Store(id, rCtx)
		return rCtx, nil
	}
}

// Get retrieves a SharedNode by its ID.
func (n *NodePool) Get(id string) (types.SharedNodeCtx, bool) {
	if v, ok := n.entries.Load(id); ok {
		return v.(*sharedNodeCtx), ok
	} else {
		return nil, false
	}
}

// GetInstance retrieves a net client or server connection by its ID.
func (n *NodePool) GetInstance(id string) (interface{}, error) {
	if ctx, ok := n.Get(id); ok {
		return ctx.GetInstance()
	} else {
		return nil, fmt.Errorf("node resource not found id=%s", id)
	}
}

// Del deletes a SharedNode instance by its ID.
func (n *NodePool) Del(id string) {
	if v, ok := n.entries.Load(id); ok {
		v.(*sharedNodeCtx).Destroy()
		n.entries.Delete(id)
	}
}

// Stop stops and releases all SharedNode instances.
func (n *NodePool) Stop() {
	n.entries.Range(func(key, value any) bool {
		n.Del(key.(string))
		return true
	})
}

// GetAll get all SharedNode instances
func (n *NodePool) GetAll() []types.SharedNodeCtx {
	var items []types.SharedNodeCtx
	n.entries.Range(func(key, value any) bool {
		items = append(items, value.(*sharedNodeCtx))
		return true
	})
	return items
}

func (n *NodePool) GetAllDef() (map[string][]*types.RuleNode, error) {
	var result = make(map[string][]*types.RuleNode)
	var resultErr error
	n.entries.Range(func(key, value any) bool {
		ctx := value.(*sharedNodeCtx)
		def, err := n.Config.Parser.DecodeRuleNode(ctx.DSL())
		if err != nil {
			resultErr = err
			return false
		}
		nodeList, ok := result[ctx.SharedNode().Type()]
		if !ok {
			result[ctx.SharedNode().Type()] = []*types.RuleNode{&def}
		} else {
			result[ctx.SharedNode().Type()] = append(nodeList, &def)
		}
		return true
	})
	return result, resultErr
}

// Range iterates over all SharedNode instances in the pool.
func (n *NodePool) Range(f func(key, value any) bool) {
	n.entries.Range(f)
}

type sharedNodeCtx struct {
	*engine.RuleNodeCtx
	Endpoint   endpointApi.Endpoint
	IsEndpoint bool
}

func newSharedNodeCtx(nodeCtx *engine.RuleNodeCtx, endpointCtx endpointApi.Endpoint) *sharedNodeCtx {
	return &sharedNodeCtx{RuleNodeCtx: nodeCtx, Endpoint: endpointCtx, IsEndpoint: endpointCtx != nil}
}

// GetInstance retrieves a net client or server connection.
// Node must implement types.SharedNode interface
func (n *sharedNodeCtx) GetInstance() (interface{}, error) {
	if n.Endpoint != nil {
		if v, ok := n.Endpoint.(*endpoint.DynamicEndpoint); ok {
			return v.Endpoint.(types.SharedNode).GetInstance()
		} else {
			return n.Endpoint.(types.SharedNode).GetInstance()
		}
	}

	// 使用读锁保护节点实例的访问
	if n.RuleNodeCtx == nil {
		return nil, fmt.Errorf("RuleNodeCtx is nil")
	}

	n.RuleNodeCtx.RLock()
	node := n.RuleNodeCtx.Node
	n.RuleNodeCtx.RUnlock()

	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	return node.(types.SharedNode).GetInstance()
}

func (n *sharedNodeCtx) GetNode() interface{} {
	if n.Endpoint != nil {
		return n.Endpoint
	}
	if n.RuleNodeCtx == nil {
		return nil
	}
	n.RuleNodeCtx.RLock()
	node := n.RuleNodeCtx.Node
	n.RuleNodeCtx.RUnlock()
	return node
}

func (n *sharedNodeCtx) DSL() []byte {
	if n.Endpoint != nil {
		if v, ok := n.Endpoint.(*endpoint.DynamicEndpoint); ok {
			return v.DSL()
		} else {
			var def = types.RuleNode{
				Id:   n.Endpoint.Id(),
				Name: n.Endpoint.Id(),
				Type: n.Endpoint.Type(),
			}
			//TODO Configuration
			dsl, _ := json.Marshal(def)
			return dsl
		}
	}
	if n.RuleNodeCtx == nil {
		return nil
	}
	return n.RuleNodeCtx.DSL()
}

func (n *sharedNodeCtx) GetNodeId() types.RuleNodeId {
	if n.Endpoint != nil {
		return types.RuleNodeId{Id: n.Endpoint.Id(), Type: types.ENDPOINT}
	}
	if n.RuleNodeCtx == nil {
		return types.RuleNodeId{}
	}
	return n.RuleNodeCtx.GetNodeId()
}

func (n *sharedNodeCtx) SharedNode() types.SharedNode {
	if n.Endpoint != nil {
		if v, ok := n.Endpoint.(*endpoint.DynamicEndpoint); ok {
			return v.Endpoint.(types.SharedNode)
		}
		return n.Endpoint.(types.SharedNode)
	}
	if n.RuleNodeCtx == nil {
		return nil
	}
	n.RuleNodeCtx.RLock()
	node := n.RuleNodeCtx.Node
	n.RuleNodeCtx.RUnlock()
	if node == nil {
		return nil
	}
	return node.(types.SharedNode)
}

// ReloadSelf 重写ReloadSelf方法以确保线程安全的重新加载
func (n *sharedNodeCtx) ReloadSelf(def []byte) error {
	if n.Endpoint != nil {
		// 对于endpoint类型，先检查是否是DynamicEndpoint接口
		if dynamicEp, ok := n.Endpoint.(endpointApi.DynamicEndpoint); ok {
			return dynamicEp.Reload(def)
		}
		return fmt.Errorf("endpoint does not support reload")
	}
	if n.RuleNodeCtx == nil {
		return fmt.Errorf("RuleNodeCtx is nil")
	}
	// 对于RuleNodeCtx类型，已经在RuleNodeCtx.ReloadSelf中处理了线程安全
	return n.RuleNodeCtx.ReloadSelf(def)
}

func (n *sharedNodeCtx) Destroy() {
	if n.Endpoint != nil {
		n.Endpoint.Destroy()
	} else if n.RuleNodeCtx != nil {
		n.RuleNodeCtx.Destroy()
	}
}
