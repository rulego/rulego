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

// NodeOutputCache Node output cache, supports concurrent secure access
// NodeOutputCache provides thread-safe storage for node outputs
type NodeOutputCache struct {
	// outputs stores node outputs, with the key being nodeId and the value being RuleMsg
	outputs sync.Map
	// Whether the hasOutputs tag enables cross-node value retrieval
	hasOutputs int32
	// A collection of node IDs that need to be cached, used for selective caching
	cacheableNodes sync.Map // map[string]bool
}

// StoreNodeOutput: Store the node output
// Caching is only performed when a node is referenced by another node
// StoreNodeOutput stores the output of a node
func (cache *NodeOutputCache) StoreNodeOutput(nodeId string, msg types.RuleMsg) {
	if nodeId == "" {
		return
	}

	// Check if nodes need caching
	isCacheable := cache.IsNodeCacheable(nodeId)
	if isCacheable {
		// Stored node output
		cache.outputs.Store(nodeId, msg.Copy())
		// Set the hasOutputs flag
		atomic.StoreInt32(&cache.hasOutputs, 1)
	}
}

// SetCacheableNodes sets the collection of node IDs to be cached
// SetCacheableNodes sets the collection of node IDs that need to be cached
func (cache *NodeOutputCache) SetCacheableNodes(nodeIds []string) {
	for _, nodeId := range nodeIds {
		cache.cacheableNodes.Store(nodeId, true)
	}
}

// IsNodeCacheable checks whether a node needs caching
// IsNodeCacheable checks if a node needs to be cached
func (cache *NodeOutputCache) IsNodeCacheable(nodeId string) bool {
	_, exists := cache.cacheableNodes.Load(nodeId)
	return exists
}

// AddCacheableNode adds a single node that needs to be cached
// AddCacheableNode adds a single node that needs to be cached
func (cache *NodeOutputCache) AddCacheableNode(nodeId string) {
	cache.cacheableNodes.Store(nodeId, true)
}

// RemoveCacheableNode removes nodes that do not require caching
// RemoveCacheableNode removes a node that no longer needs to be cached
func (cache *NodeOutputCache) RemoveCacheableNode(nodeId string) {
	cache.cacheableNodes.Delete(nodeId)
}

// GetNodeRuleMsg retrieves the complete message information of the node
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

// HasOutputs checks whether there is a node output
// HasOutputs checks if there are any node outputs
func (cache *NodeOutputCache) HasOutputs() bool {
	return atomic.LoadInt32(&cache.hasOutputs) != 0
}

// Clear: Clears all node outputs
// Clear removes all node outputs
func (cache *NodeOutputCache) Clear() {
	cache.outputs.Range(func(key, value interface{}) bool {
		cache.outputs.Delete(key)
		return true
	})
	atomic.StoreInt32(&cache.hasOutputs, 0)
	// Clearing the cacheable node set
	cache.cacheableNodes.Range(func(key, value interface{}) bool {
		cache.cacheableNodes.Delete(key)
		return true
	})
}

// EnableCrossNodeAccess enables cross-node value retrieval
// EnableCrossNodeAccess enables cross-node value access for this cache
func (cache *NodeOutputCache) EnableCrossNodeAccess() {
	atomic.StoreInt32(&cache.hasOutputs, 1)
}
