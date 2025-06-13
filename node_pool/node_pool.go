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

// Package node_pool manages shared node resources, allowing for efficient reuse of node instances
// across different rule chains and executions.
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
	ErrNotImplemented = errors.New("not SharedNode")
)
var _ types.NodePool = (*NodePool)(nil)

// DefaultNodePool 默认组件资源池管理器
var DefaultNodePool = NewNodePool(engine.NewConfig())

// NodePool is a component resource pool manager
type NodePool struct {
	Config types.Config
	// key:resourceId value:sharedNodeCtx
	entries sync.Map
}

func NewNodePool(config types.Config) *NodePool {
	return &NodePool{
		Config: config,
	}
}

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
