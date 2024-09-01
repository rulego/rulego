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

package engine

import (
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"sync"
)

var (
	ErrNotImplemented = errors.New("not net resource node")
)

var _ types.NetPool = (*NetPool)(nil)

// DefaultNetPool 默认组件资源池管理器
var DefaultNetPool = NewNetPool(types.NewConfig())

// NetPool 组件资源池管理器
type NetPool struct {
	Config types.Config
	// key:nodeType value:NodeNetPool
	nodeNetPoolMap sync.Map
}

func NewNetPool(config types.Config) *NetPool {
	return &NetPool{
		Config: config,
	}
}

// New creates a new NetNode instance.
func (n *NetPool) New(nodeType, id string, dsl []byte) (types.NetNodeCtx, error) {
	v, _ := n.nodeNetPoolMap.LoadOrStore(nodeType, NewNodeNetPool(n.Config, nodeType))
	return v.(*NodeNetPool).New(id, dsl)
}

// NewFromDef creates a new NetNode instance from a RuleNode definition.
func (n *NetPool) NewFromDef(def types.RuleNode) (types.NetNodeCtx, error) {
	v, _ := n.nodeNetPoolMap.LoadOrStore(def.Type, NewNodeNetPool(n.Config, def.Type))
	return v.(*NodeNetPool).NewFromDef(def)
}

// Get retrieves a NetNode instance by its nodeTye and ID.
func (n *NetPool) Get(nodeType string, id string) (types.NetNodeCtx, bool) {
	if v, ok := n.nodeNetPoolMap.Load(nodeType); ok {
		return v.(*NodeNetPool).Get(id)
	} else {
		return nil, false
	}
}

// GetNetResource retrieves a net client or server connection by its nodeTye and ID.
func (n *NetPool) GetNetResource(nodeType string, id string) (interface{}, error) {
	if v, ok := n.nodeNetPoolMap.Load(nodeType); ok {
		return v.(*NodeNetPool).GetNetResource(id)
	} else {
		return nil, fmt.Errorf("net resource not found id=%s", id)
	}
}

// Del deletes a NetNode instance by its nodeTye and ID.
func (n *NetPool) Del(nodeType string, id string) {
	if v, ok := n.nodeNetPoolMap.Load(nodeType); ok {
		v.(*NodeNetPool).Del(id)
	}
}

// Stop stops and releases all NetNode instances.
func (n *NetPool) Stop() {
	n.nodeNetPoolMap.Range(func(key, value any) bool {
		value.(*NodeNetPool).Stop()
		n.nodeNetPoolMap.Delete(key)
		return true
	})
}

// GetAll get all NetNode instances
func (n *NetPool) GetAll() map[string][]types.NetNodeCtx {
	nodeTypeItems := make(map[string][]types.NetNodeCtx)
	n.nodeNetPoolMap.Range(func(key, value any) bool {
		items := value.(*NodeNetPool).GetAll()
		nodeTypeItems[key.(string)] = items
		return true
	})
	return nodeTypeItems
}

// NodeNetPool Network connection type component resource pool
type NodeNetPool struct {
	Config types.Config
	//NodeType node type
	NodeType string
	// key:resourceId value:NetNodeCtx
	entries sync.Map
}

func NewNodeNetPool(config types.Config, nodeType string) *NodeNetPool {
	return &NodeNetPool{
		Config:   config,
		NodeType: nodeType,
	}
}

// New creates a new NetNode and stores it in the Pool.
func (n *NodeNetPool) New(id string, dsl []byte) (types.NetNodeCtx, error) {
	if v, ok := n.entries.Load(id); ok {
		return v.(types.NetNodeCtx), nil
	}
	if nodeDef, err := n.Config.Parser.DecodeRuleNode(dsl); err == nil {
		if id != "" {
			nodeDef.Id = id
		}
		return n.NewFromDef(nodeDef)
	} else {
		return nil, err
	}
}

func (n *NodeNetPool) NewFromDef(def types.RuleNode) (types.NetNodeCtx, error) {
	if v, ok := n.entries.Load(def.Id); ok {
		return v.(types.NetNodeCtx), nil
	}
	if ctx, err := InitNetResourceNodeCtx(n.Config, nil, nil, &def); err == nil {
		rCtx := NewNetResourceCtx(ctx)
		if _, ok := rCtx.Node.(types.NetNode); !ok {
			return nil, ErrNotImplemented
		}
		n.entries.Store(rCtx.GetNodeId().Id, rCtx)
		return rCtx, nil
	} else {
		return nil, err
	}
}

// Get retrieves a NetNode by its ID.
func (n *NodeNetPool) Get(id string) (types.NetNodeCtx, bool) {
	if v, ok := n.entries.Load(id); ok {
		return v.(types.NetNodeCtx), ok
	} else {
		return nil, false
	}
}

// GetNetResource retrieves a net client or server connection by its ID.
func (n *NodeNetPool) GetNetResource(id string) (interface{}, error) {
	if ctx, ok := n.Get(id); ok {
		return ctx.GetNetResource()
	} else {
		return nil, fmt.Errorf("net resource not found id=%s", id)
	}
}

// Del deletes a NetNode instance by its ID.
func (n *NodeNetPool) Del(id string) {
	if v, ok := n.entries.Load(id); ok {
		v.(types.NetNodeCtx).Destroy()
		n.entries.Delete(id)
	}
}

// Stop stops and releases all NetNode instances.
func (n *NodeNetPool) Stop() {
	n.entries.Range(func(key, value any) bool {
		n.Del(key.(string))
		return true
	})
}

// GetAll get all NetNode instances
func (n *NodeNetPool) GetAll() []types.NetNodeCtx {
	var items []types.NetNodeCtx
	n.entries.Range(func(key, value any) bool {
		items = append(items, value.(types.NetNodeCtx))
		return true
	})
	return items
}

// Range iterates over all NetNode instances in the pool.
func (n *NodeNetPool) Range(f func(key, value any) bool) {
	n.entries.Range(f)
}

type NetResourceCtx struct {
	*RuleNodeCtx
}

func NewNetResourceCtx(ctx *RuleNodeCtx) *NetResourceCtx {
	return &NetResourceCtx{RuleNodeCtx: ctx}
}

// GetNetResource retrieves a net client or server connection.
// Node must implement types.NetNode interface
func (n *NetResourceCtx) GetNetResource() (interface{}, error) {
	if ctx, ok := n.Node.(types.NetNode); ok {
		return ctx.GetNetResource()
	} else {
		return nil, ErrNotImplemented
	}
}
