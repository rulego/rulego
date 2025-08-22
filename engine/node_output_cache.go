/*
 * Copyright 2025 The RuleGo Authors.
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
	"github.com/rulego/rulego/api/types"
	"sync"
	"sync/atomic"
)

// NodeOutputCache 节点输出缓存，支持并发安全访问
// NodeOutputCache provides thread-safe storage for node outputs
type NodeOutputCache struct {
	// outputs 存储节点输出，key为nodeId，value为RuleMsg
	outputs sync.Map
	// hasOutputs 标记是否启用了跨节点取值功能
	hasOutputs int32
	// 需要缓存的节点ID集合，用于选择性缓存
	cacheableNodes sync.Map // map[string]bool
}

// StoreNodeOutput 存储节点输出
// 只有在节点被其他节点引用时才进行缓存
// StoreNodeOutput stores the output of a node
func (cache *NodeOutputCache) StoreNodeOutput(nodeId string, msg types.RuleMsg) {
	if nodeId == "" {
		return
	}

	// 检查节点是否需要缓存
	isCacheable := cache.IsNodeCacheable(nodeId)
	if isCacheable {
		// 储存节点输出
		cache.outputs.Store(nodeId, msg.Copy())
		// 设置hasOutputs标志
		atomic.StoreInt32(&cache.hasOutputs, 1)
	}
}

// SetCacheableNodes 设置需要缓存的节点ID集合
// SetCacheableNodes sets the collection of node IDs that need to be cached
func (cache *NodeOutputCache) SetCacheableNodes(nodeIds []string) {
	for _, nodeId := range nodeIds {
		cache.cacheableNodes.Store(nodeId, true)
	}
}

// IsNodeCacheable 检查节点是否需要缓存
// IsNodeCacheable checks if a node needs to be cached
func (cache *NodeOutputCache) IsNodeCacheable(nodeId string) bool {
	_, exists := cache.cacheableNodes.Load(nodeId)
	return exists
}

// AddCacheableNode 添加单个需要缓存的节点
// AddCacheableNode adds a single node that needs to be cached
func (cache *NodeOutputCache) AddCacheableNode(nodeId string) {
	cache.cacheableNodes.Store(nodeId, true)
}

// RemoveCacheableNode 移除不需要缓存的节点
// RemoveCacheableNode removes a node that no longer needs to be cached
func (cache *NodeOutputCache) RemoveCacheableNode(nodeId string) {
	cache.cacheableNodes.Delete(nodeId)
}

// GetNodeRuleMsg 获取节点的完整消息信息
// GetNodeRuleMsg retrieves the complete RuleMsg of a node
func (cache *NodeOutputCache) GetNodeRuleMsg(nodeId string) (types.RuleMsg, bool) {
	if atomic.LoadInt32(&cache.hasOutputs) == 0 {
		return types.RuleMsg{}, false
	}

	if value, ok := cache.outputs.Load(nodeId); ok {
		return value.(types.RuleMsg), true
	}
	return types.RuleMsg{}, false
}

// HasOutputs 检查是否有节点输出
// HasOutputs checks if there are any node outputs
func (cache *NodeOutputCache) HasOutputs() bool {
	return atomic.LoadInt32(&cache.hasOutputs) != 0
}

// Clear 清空所有节点输出
// Clear removes all node outputs
func (cache *NodeOutputCache) Clear() {
	cache.outputs.Range(func(key, value interface{}) bool {
		cache.outputs.Delete(key)
		return true
	})
	atomic.StoreInt32(&cache.hasOutputs, 0)
	// 清空可缓存节点集合
	cache.cacheableNodes.Range(func(key, value interface{}) bool {
		cache.cacheableNodes.Delete(key)
		return true
	})
}

// EnableCrossNodeAccess 启用跨节点取值功能
// EnableCrossNodeAccess enables cross-node value access for this cache
func (cache *NodeOutputCache) EnableCrossNodeAccess() {
	atomic.StoreInt32(&cache.hasOutputs, 1)
}
