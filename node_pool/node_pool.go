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
// Package node_pool provides shared node resource management, enabling efficient connection and reuse between different rule chains and components.
// It enables network-connected components to share their instantiated connection resources (clients) with other components,
// This achieves the goal of saving system resources.
//
// Connection Reuse Scenarios:
// Connection multiplexing scenarios:
//
// Multiple components can reuse the same connection resources:
// Multiple components can reuse the same connection resources:
//   - Multiple MQTT clients sharing the same MQTT connection
//   - Multiple database operations sharing the same database connection
//   - Multiple HTTP endpoints sharing the same port
//   - Message queue clients sharing connection pools
//
// SharedNode Interface Requirement:
// SharedNode interface requirements:
//
// Components that support connection sharing must implement the SharedNode interface:
// Components supporting connection sharing must implement the SharedNode interface:
//
//	type SharedNode interface {
//		GetInstance() (interface{}, error)
//		// ... other methods
//	}
//
// Most officially provided network connection components support this pattern.
// Most official network connection components support this mode.
//
// Usage Pattern:
// Usage mode:
//
//  1. Initialize shared resource nodes by loading a rule chain definition:
//     Initialize shared resource nodes by loading the rule chain and defining them:
//
//     node_pool.DefaultNodePool.Load(dsl []byte)
//
//  2. Reference shared resources using the ref://{resourceId} pattern:
//     Use the ref://{resourceId} pattern to reference shared resources:
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
// Example of node pool configuration:
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
// Differences from node references:
//
//   - Node Reference: Completely references the specified node instance, including all configurations
//     Node reference: fully references the specified node instance, including all node configurations
//
//   - Shared Resource Node: Reuses the node's connection instance, but other configurations are independent
//     Shared Resource Node: Reuses connection instances of nodes, but other configurations of nodes are independent
//
// For example, with MQTT client nodes:
// For example, for an MQTT client node:
//   - Shared: Connection configuration (MQTT address, reconnection interval, etc.)
//     Sharing: Connection class configuration (MQTT address, reconnect interval, etc.)
//   - Independent: Other configurations like publish topics for each node
//     Independent: Other configurations such as each node's release theme
//
// RuleGo-Server Integration:
// RuleGo-Server Integration:
//
// In RuleGo-Server, configure the node pool file in config.conf:
// In RuleGo-Server, configure the node pool file in config.conf:
//
//	node_pool_file=./node_pool.json
//
// Thread Safety:
// Thread safety:
//
// The node pool implementation is thread-safe and supports concurrent access
// from multiple goroutines. All operations are protected by appropriate
// synchronization mechanisms.
// The node pool implementation is thread-safe and supports concurrent access to multiple goroutines.
// All operations are protected by appropriate synchronization mechanisms.
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
	// ErrNotImplemented Returns when the component does not implement the SharedNode interface.
	// Only components that implement SharedNode can be added to the node pool for connection sharing.
	ErrNotImplemented = errors.New("not SharedNode")
)
var _ types.NodePool = (*NodePool)(nil)

// DefaultNodePool is the global default component resource pool manager.
// It provides a convenient singleton instance for managing shared node resources
// across the entire application. Most applications can use this default instance
// without creating custom node pools.
//
// DefaultNodePool is the global default component resource pool manager.
// It provides convenient singleton instances for managing shared node resources across the entire application.
// Most applications can use this default instance without creating custom node pools.
//
// Usage:
// Usage:
//
//	// Load shared nodes from JSON configuration
//	Load the shared node from JSON configuration
//	DefaultNodePool.Load(jsonConfig)
//
//	// Get a shared connection instance
//	Obtain the shared connection instance
//	if instance, err := DefaultNodePool.GetInstance("local_mqtt_client"); err == nil {
//	    // Use the shared MQTT client
//	}
var DefaultNodePool = NewNodePool(engine.NewConfig())

// NodePool is a thread-safe component resource pool manager that manages shared node instances
// and their connection resources. It enables efficient reuse of network connections and
// other expensive resources across multiple rule chains and components.
//
// NodePool is a thread-safe component resource pool manager that manages shared node instances and their connection resources.
// It enables network connections and other expensive resources to be efficiently reused across multiple rule chains and components.
//
// Key Features:
// Main features:
//   - Thread-safe concurrent access to shared resources
//   - Support for both endpoint and rule node sharing
//   - Automatic resource lifecycle management
//   - JSON-based configuration loading
//   - Dynamic resource addition and removal
//
// Resource Management:
// Resource Management:
// The pool maintains a mapping of resource IDs to shared node contexts,
// allowing components to reference shared resources by ID using the ref://{resourceId} pattern.
// The pool maintains the mapping of resource IDs to shared node contexts, allowing components to reference shared resources via ID using the ref://{resourceId} pattern.
type NodePool struct {
	// Config provides the rule engine configuration used for creating and managing shared nodes.
	// This configuration determines how nodes are parsed, initialized, and managed.
	//
	// Config provides a rule engine configuration for creating and managing shared nodes.
	// This configuration determines how nodes are parsed, initialized, and managed.
	Config types.Config
	// entries is a thread-safe map storing shared node contexts.
	// Key: resourceId (string) - unique identifier for the shared resource
	// Value: *sharedNodeCtx - wrapper containing the shared node and its metadata
	//
	// entries are thread-safe mappings that store the context of shared nodes.
	// Key: resourceId (string) - The unique identifier for shared resources
	// Value: *sharedNodeCtx - A wrapper containing shared nodes and their metadata
	entries sync.Map
}

// NewNodePool creates a new node pool instance with the specified configuration.
// The configuration determines how nodes are parsed, initialized, and managed within the pool.
//
// NewNodePool creates a new node pool instance using the specified configuration.
// Configuration determines how nodes are parsed, initialized, and managed within the pool.
//
// Parameters:
// Parameters:
//   - config: Rule engine configuration for node management
//
// Returns:
// Returns:
//   - *NodePool: New node pool instance
//
// Usage:
// Usage:
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
// Load: Configure data parsing from JSON/DSL and load the shared node definition.
// This is the main method for initializing the node pool from the configuration file.
// The DSL should include a rule chain definition with endpoints and node sections.
//
// Parameters:
// Parameters:
//   - dsl: JSON configuration data defining the shared nodes
//
// Returns:
// Returns:
//   - types.NodePool: The node pool instance (self) for method chaining
//   - error: Parse or initialization error if any
//
// Configuration Format:
// Configuration format:
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
// False condition:
//   - Invalid JSON format
//   - Node type not found in registry
//   - Duplicate node IDs
//   - Component doesn't implement SharedNode interface
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
			if _, ok := n.entries.Load(item.Id); ok {
				continue
			}
			if _, err := n.NewFromEndpoint(*item); err != nil {
				return nil, err
			}
		}
	}
	for _, item := range def.Metadata.Nodes {
		if item != nil {
			if _, ok := n.entries.Load(item.Id); ok {
				continue
			}
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

// Lookup implements types.ResourceLookup: forwards GetInstance for unified parsing by ref://.
// Read paths run through the lower entries sync.Map, unlocked.
func (n *NodePool) Lookup(id string) (any, bool) {
	v, err := n.GetInstance(id)
	if err != nil {
		return nil, false
	}
	return v, true
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

	// Use a read lock to protect access to node instances
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

// RewriteSelf Rewrites the ReloadSelf method to ensure thread safety during reloading
func (n *sharedNodeCtx) ReloadSelf(def []byte) error {
	if n.Endpoint != nil {
		// For the endpoint type, first check whether it is a DynamicEndpoint interface
		if dynamicEp, ok := n.Endpoint.(endpointApi.DynamicEndpoint); ok {
			return dynamicEp.Reload(def)
		}
		return fmt.Errorf("endpoint does not support reload")
	}
	if n.RuleNodeCtx == nil {
		return fmt.Errorf("RuleNodeCtx is nil")
	}
	// For the RuleNodeCtx type, thread safety has already been handled in RuleNodeCtx.ReloadSelf
	return n.RuleNodeCtx.ReloadSelf(def)
}

func (n *sharedNodeCtx) Destroy() {
	if n.Endpoint != nil {
		n.Endpoint.Destroy()
	} else if n.RuleNodeCtx != nil {
		n.RuleNodeCtx.Destroy()
	}
}
