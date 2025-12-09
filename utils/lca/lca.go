package lca

import (
	"sync"

	"github.com/rulego/rulego/api/types"
)

// ParentProvider defines the interface for getting parent nodes
// ParentProvider 定义获取父节点的接口
type ParentProvider interface {
	GetParentNodeIds(id types.RuleNodeId) ([]types.RuleNodeId, bool)
}

// LCACalculator provides optimized Lowest Common Ancestor calculation
// LCACalculator 提供优化的最低共同祖先计算
type LCACalculator struct {
	parentProvider ParentProvider
	cache          map[types.RuleNodeId]types.RuleNodeId
	cacheMutex     sync.RWMutex // 保护缓存的读写锁
}

// NewLCACalculator creates a new LCA calculator
// NewLCACalculator 创建新的LCA计算器
func NewLCACalculator(parentProvider ParentProvider) *LCACalculator {
	return &LCACalculator{
		parentProvider: parentProvider,
		cache:          make(map[types.RuleNodeId]types.RuleNodeId),
	}
}

// GetLCA finds the lowest common ancestor of a node's parent nodes
// GetLCA 查找节点所有父节点的最低共同祖先
func (lca *LCACalculator) GetLCA(nodeId types.RuleNodeId) (types.RuleNodeId, bool) {
	// Check cache first with read lock
	// 首先使用读锁检查缓存
	lca.cacheMutex.RLock()
	if cachedLCA, exists := lca.cache[nodeId]; exists {
		lca.cacheMutex.RUnlock()
		return cachedLCA, true
	}
	lca.cacheMutex.RUnlock()

	// Get parent nodes
	// 获取父节点
	parentIds, exists := lca.parentProvider.GetParentNodeIds(nodeId)
	if !exists || len(parentIds) == 0 {
		return types.RuleNodeId{}, false
	}

	var result types.RuleNodeId
	var found bool

	// Handle single parent case
	// 处理单父节点情况
	if len(parentIds) == 1 {
		result, found = lca.computeSingleParentLCA(parentIds[0])
	} else {
		// Handle multiple parents case
		// 处理多父节点情况
		result, found = lca.computeMultipleParentsLCA(parentIds)
	}

	// Cache the result if found with write lock
	// 如果找到结果则使用写锁缓存
	if found {
		lca.cacheMutex.Lock()
		lca.cache[nodeId] = result
		lca.cacheMutex.Unlock()
		return result, true
	}

	return types.RuleNodeId{}, false
}

// GetLCAOfNodes finds the lowest common ancestor of multiple nodes.
// GetLCAOfNodes 查找多个节点的最低共同祖先。
func (lca *LCACalculator) GetLCAOfNodes(nodeIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	if len(nodeIds) == 0 {
		return types.RuleNodeId{}, false
	}
	if len(nodeIds) == 1 {
		// Use GetParentNodeIds to find parents.
		// If multiple parents, it's ambiguous. But usually we look for a common fork node.
		// Let's assume we need to find a common ancestor in the graph.
		// But wait, GetLCA is designed for finding LCA of parents of a SINGLE node (Join node).
		// Here we have multiple nodes (branches), we want to find THEIR common ancestor.

		// We can reuse lcaCalculator logic if it supports finding LCA of a set of nodes.
		// lcaCalculator usually builds parent pointers.
		// Let's check lcaCalculator implementation. It's likely internal or not exposed fully.
		// But we have GetParentNodeIds(id).

		// Simple approach: Get parents of the first node.
		parents, ok := lca.parentProvider.GetParentNodeIds(nodeIds[0])
		if ok && len(parents) > 0 {
			// 如果有多个父节点，返回其中一个（通常在树状结构中只有一个父节点，或者多个父节点最终汇聚）
			// 这里简单返回第一个，作为上下文的父节点
			return parents[0], true
		} else {
			// 如果没有父节点（即根节点），那么它自己就是自己的"父上下文"挂载点？
			// 或者返回自己？
			// 如果返回自己，那么 engine.processRestoreNodes 会使用它作为 parentCtx。
			// 如果它是根节点，parentCtx.parentRuleCtx 应该是 rootCtxCopy。
			// 如果它是根节点，它没有父节点。
			return nodeIds[0], true
		}
	}

	// Check if they share a direct parent
	// Get all ancestors for each node
	allAncestors := make([]map[types.RuleNodeId]bool, len(nodeIds))

	for i, startNode := range nodeIds {
		allAncestors[i] = make(map[types.RuleNodeId]bool)
		// Add self as ancestor (LCA can be one of the nodes)
		allAncestors[i][startNode] = true

		queue := []types.RuleNodeId{startNode}
		visited := make(map[types.RuleNodeId]bool)
		visited[startNode] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]

			parents, ok := lca.parentProvider.GetParentNodeIds(curr)
			if ok {
				for _, p := range parents {
					if !visited[p] {
						visited[p] = true
						allAncestors[i][p] = true
						queue = append(queue, p)
					}
				}
			}
		}
	}

	// Find intersection
	common := make([]types.RuleNodeId, 0)
	// Iterate over ancestors of first node
	for anc := range allAncestors[0] {
		isCommon := true
		for i := 1; i < len(nodeIds); i++ {
			if !allAncestors[i][anc] {
				isCommon = false
				break
			}
		}
		if isCommon {
			common = append(common, anc)
		}
	}

	if len(common) == 0 {
		return types.RuleNodeId{}, false
	}

	// Find lowest (no other common ancestor is its descendant)
	for _, candidate := range common {
		// Check if candidate is ancestor of any OTHER candidate
		isAncestorOfOther := false
		for _, other := range common {
			if candidate == other {
				continue
			}
			// Is candidate an ancestor of other?
			if lca.isAncestor(candidate, other) {
				isAncestorOfOther = true
				break
			}
		}
		if !isAncestorOfOther {
			return candidate, true
		}
	}
	if len(common) > 0 {
		return common[0], true
	}

	return types.RuleNodeId{}, false
}

// computeSingleParentLCA computes LCA for nodes with only one parent
// computeSingleParentLCA 计算只有一个父节点的节点的LCA
func (lca *LCACalculator) computeSingleParentLCA(parentId types.RuleNodeId) (types.RuleNodeId, bool) {
	// For single parent case, find the topmost ancestor
	// 对于单父节点情况，查找最顶层的祖先

	// Get all ancestors by level
	// 获取所有层级的祖先
	ancestors := lca.getAncestorsByLevel(parentId)

	// If parent has ancestors, return the topmost one
	// 如果父节点有祖先，返回最顶层的祖先
	if len(ancestors) > 0 {
		// Find the last level (topmost ancestors)
		// 查找最后一层（最顶层的祖先）
		lastLevel := ancestors[len(ancestors)-1]
		if len(lastLevel) > 0 {
			return lastLevel[0], true
		}
	}

	// If parent has no ancestors, return parent itself as LCA
	// 如果父节点没有祖先，返回父节点本身作为 LCA
	return parentId, true
}

// computeMultipleParentsLCA computes LCA for nodes with multiple parents
// computeMultipleParentsLCA 计算有多个父节点的节点的LCA
func (lca *LCACalculator) computeMultipleParentsLCA(parentIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	// First check if any parent is an ancestor of all other parents
	// 首先检查是否有任何父节点是所有其他父节点的祖先
	for _, candidateParent := range parentIds {
		if lca.isCommonAncestorOfAll(candidateParent, parentIds) {
			return candidateParent, true
		}
	}

	// If no parent is a common ancestor, use optimized cross-level algorithm
	// 如果没有父节点是公共祖先，使用优化的跨层级算法
	return lca.findOptimizedCrossLevelLCA(parentIds)
}

// isCommonAncestorOfAll checks if a candidate is an ancestor of all other nodes
// isCommonAncestorOfAll 检查候选节点是否是所有其他节点的祖先
func (lca *LCACalculator) isCommonAncestorOfAll(candidate types.RuleNodeId, nodeIds []types.RuleNodeId) bool {
	for _, nodeId := range nodeIds {
		if candidate.Id == nodeId.Id {
			continue // Skip self
		}
		if !lca.isAncestor(candidate, nodeId) {
			return false
		}
	}
	return true
}

// findOptimizedCrossLevelLCA finds common ancestors across different levels using optimized algorithm
// findOptimizedCrossLevelLCA 使用优化算法查找跨层级的共同祖先
func (lca *LCACalculator) findOptimizedCrossLevelLCA(parentIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	// Build all ancestors for each parent using BFS
	// 使用BFS为每个父节点构建所有祖先
	allAncestors := make([]map[types.RuleNodeId]int, len(parentIds)) // map[nodeId]level

	for i, parentId := range parentIds {
		allAncestors[i] = make(map[types.RuleNodeId]int)
		// Add the parent itself as level 0 ancestor
		// 将父节点本身作为第0层祖先添加
		allAncestors[i][parentId] = 0

		// Add all ancestors of this parent with their levels
		// 添加此父节点的所有祖先及其层级
		ancestorsByLevel := lca.getAncestorsByLevel(parentId)
		for level, levelAncestors := range ancestorsByLevel {
			for _, ancestor := range levelAncestors {
				allAncestors[i][ancestor] = level + 1
			}
		}
	}

	// Find common ancestors with their minimum levels
	// 查找共同祖先及其最小层级
	commonAncestors := make(map[types.RuleNodeId]int)

	// Start with first parent's ancestors
	// 从第一个父节点的祖先开始
	for ancestor, level := range allAncestors[0] {
		minLevel := level
		isCommon := true

		// Check if this ancestor exists in all other parents' ancestors
		// 检查此祖先是否存在于所有其他父节点的祖先中
		for i := 1; i < len(allAncestors); i++ {
			if otherLevel, exists := allAncestors[i][ancestor]; exists {
				if otherLevel < minLevel {
					minLevel = otherLevel
				}
			} else {
				isCommon = false
				break
			}
		}

		if isCommon {
			commonAncestors[ancestor] = minLevel
		}
	}

	// If no common ancestors found, return false
	// 如果没有找到共同祖先，返回false
	if len(commonAncestors) == 0 {
		return types.RuleNodeId{}, false
	}

	// Find the lowest (highest level number, closest to leaves) common ancestor
	// 查找最低（层级数最高，最接近叶子节点）的共同祖先
	var lowestAncestor types.RuleNodeId
	maxLevel := -1

	for ancestor, level := range commonAncestors {
		if level > maxLevel {
			maxLevel = level
			lowestAncestor = ancestor
		}
	}

	return lowestAncestor, true
}

// getAncestorsByLevel performs level-by-level BFS to find ancestors grouped by distance
// getAncestorsByLevel 执行逐层BFS查找按距离分组的祖先
func (lca *LCACalculator) getAncestorsByLevel(nodeId types.RuleNodeId) [][]types.RuleNodeId {
	var result [][]types.RuleNodeId
	visited := make(map[types.RuleNodeId]bool)
	currentLevel := []types.RuleNodeId{nodeId}
	visited[nodeId] = true

	for len(currentLevel) > 0 {
		var nextLevel []types.RuleNodeId
		var ancestors []types.RuleNodeId

		for _, currentNode := range currentLevel {
			if parentIds, exists := lca.parentProvider.GetParentNodeIds(currentNode); exists {
				for _, parentId := range parentIds {
					if !visited[parentId] {
						visited[parentId] = true
						nextLevel = append(nextLevel, parentId)
						ancestors = append(ancestors, parentId)
					}
				}
			}
		}

		if len(ancestors) > 0 {
			result = append(result, ancestors)
		}
		currentLevel = nextLevel
	}

	return result
}

// isAncestor checks if ancestor is an ancestor of descendant
// isAncestor 检查ancestor是否是descendant的祖先
func (lca *LCACalculator) isAncestor(ancestor, descendant types.RuleNodeId) bool {
	if ancestor.Id == descendant.Id {
		return false // A node is not an ancestor of itself
	}

	visited := make(map[types.RuleNodeId]bool)
	queue := []types.RuleNodeId{descendant}
	visited[descendant] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if parentIds, exists := lca.parentProvider.GetParentNodeIds(current); exists {
			for _, parentId := range parentIds {
				if parentId.Id == ancestor.Id {
					return true
				}
				if !visited[parentId] {
					visited[parentId] = true
					queue = append(queue, parentId)
				}
			}
		}
	}

	return false
}

// ClearCache clears the LCA cache
// ClearCache 清空LCA缓存
func (lca *LCACalculator) ClearCache() {
	lca.cacheMutex.Lock()
	defer lca.cacheMutex.Unlock()
	lca.cache = make(map[types.RuleNodeId]types.RuleNodeId)
}

// GetCacheSize returns the current cache size
// GetCacheSize 返回当前缓存大小
func (lca *LCACalculator) GetCacheSize() int {
	lca.cacheMutex.RLock()
	defer lca.cacheMutex.RUnlock()
	return len(lca.cache)
}
