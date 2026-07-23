package lca

import (
	"sync"

	"github.com/rulego/rulego/api/types"
)

// ParentProvider defines the interface for getting parent nodes
// ParentProvider defines the interface for obtaining the parent node
type ParentProvider interface {
	GetParentNodeIds(id types.RuleNodeId) ([]types.RuleNodeId, bool)
}

// LCACalculator provides optimized Lowest Common Ancestor calculation
// LCACalculator provides optimized minimum common ancestor computation
type LCACalculator struct {
	parentProvider ParentProvider
	cache          map[types.RuleNodeId]types.RuleNodeId
	cacheMutex     sync.RWMutex // Protect the cache's read/write lock
}

// NewLCACalculator creates a new LCA calculator
// NewLCACalculator creates a new LCA calculator
func NewLCACalculator(parentProvider ParentProvider) *LCACalculator {
	return &LCACalculator{
		parentProvider: parentProvider,
		cache:          make(map[types.RuleNodeId]types.RuleNodeId),
	}
}

// GetLCA finds the lowest common ancestor of a node's parent nodes
// GetLCA finds the lowest common ancestor of all parent nodes of the node
func (lca *LCACalculator) GetLCA(nodeId types.RuleNodeId) (types.RuleNodeId, bool) {
	// Check cache first with read lock
	// First, use lock reading to check the cache
	lca.cacheMutex.RLock()
	if cachedLCA, exists := lca.cache[nodeId]; exists {
		lca.cacheMutex.RUnlock()
		return cachedLCA, true
	}
	lca.cacheMutex.RUnlock()

	// Get parent nodes
	// Get the parent node
	parentIds, exists := lca.parentProvider.GetParentNodeIds(nodeId)
	if !exists || len(parentIds) == 0 {
		return types.RuleNodeId{}, false
	}

	var result types.RuleNodeId
	var found bool

	// Handle single parent case
	// Handle single parent node situations
	if len(parentIds) == 1 {
		result, found = lca.computeSingleParentLCA(parentIds[0])
	} else {
		// Handle multiple parents case
		// Handle multi-parent node situations
		result, found = lca.computeMultipleParentsLCA(parentIds)
	}

	// Cache the result if found with write lock
	// If the result is found, write lock caching is used
	if found {
		lca.cacheMutex.Lock()
		lca.cache[nodeId] = result
		lca.cacheMutex.Unlock()
		return result, true
	}

	return types.RuleNodeId{}, false
}

// GetLCAOfNodes finds the lowest common ancestor of multiple nodes.
// GetLCAOfNodes finds the lowest common ancestor of multiple nodes.
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
			// If there are multiple parent nodes, return one of them (usually only one parent node in the tree structure, or multiple parent nodes eventually merg)
			// Here, simply return the first node as the parent node of the context
			return parents[0], true
		} else {
			// If there is no parent node (i.e., the root node), then it itself is its own "parent context" mount point?
			// Or return to yourself?
			// If it returns itself, engine.processRestoreNodes will use it as parentCtx.
			// If it is the root node, parentCtx.parentRuleCtx should be rootCtxCopy.
			// If it is a root node, it has no parent node.
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
// computeSingleParentLCA calculates the LCA of a node with only one parent
func (lca *LCACalculator) computeSingleParentLCA(parentId types.RuleNodeId) (types.RuleNodeId, bool) {
	// For single parent case, find the topmost ancestor
	// For single-parent nodes, find the top-level ancestor

	// Get all ancestors by level
	// Obtain ancestors at all levels
	ancestors := lca.getAncestorsByLevel(parentId)

	// If parent has ancestors, return the topmost one
	// If the parent node has an ancestor, it returns to the topmost ancestor
	if len(ancestors) > 0 {
		// Find the last level (topmost ancestors)
		// Find the last layer (the ancestor at the very top)
		lastLevel := ancestors[len(ancestors)-1]
		if len(lastLevel) > 0 {
			return lastLevel[0], true
		}
	}

	// If parent has no ancestors, return parent itself as LCA
	// If the parent node has no ancestor, the parent node itself is returned as an LCA
	return parentId, true
}

// computeMultipleParentsLCA computes LCA for nodes with multiple parents
// computeMultipleParentsLCA: Computes the LCA of nodes with multiple parent nodes
func (lca *LCACalculator) computeMultipleParentsLCA(parentIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	// First check if any parent is an ancestor of all other parents
	// First, check if any parent nodes are ancestors of all other parent nodes
	for _, candidateParent := range parentIds {
		if lca.isCommonAncestorOfAll(candidateParent, parentIds) {
			return candidateParent, true
		}
	}

	// If no parent is a common ancestor, use optimized cross-level algorithm
	// If no parent node is a common ancestor, use an optimized cross-level algorithm
	return lca.findOptimizedCrossLevelLCA(parentIds)
}

// isCommonAncestorOfAll checks if a candidate is an ancestor of all other nodes
// isCommonAncestorOfAll checks whether the candidate node is the ancestor of all other nodes
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
// findOptimizedCrossLevelLCA uses optimization algorithms to find common ancestors across levels
func (lca *LCACalculator) findOptimizedCrossLevelLCA(parentIds []types.RuleNodeId) (types.RuleNodeId, bool) {
	// Build all ancestors for each parent using BFS
	// Use BFS to build all ancestors for each parent node
	allAncestors := make([]map[types.RuleNodeId]int, len(parentIds)) // map[nodeId]level

	for i, parentId := range parentIds {
		allAncestors[i] = make(map[types.RuleNodeId]int)
		// Add the parent itself as level 0 ancestor
		// Add the parent node itself as the Layer 0 ancestor
		allAncestors[i][parentId] = 0

		// Add all ancestors of this parent with their levels
		// Add all ancestors and their levels of this parent node
		ancestorsByLevel := lca.getAncestorsByLevel(parentId)
		for level, levelAncestors := range ancestorsByLevel {
			for _, ancestor := range levelAncestors {
				allAncestors[i][ancestor] = level + 1
			}
		}
	}

	// Find common ancestors with their minimum levels
	// Find common ancestors and their lowest hierarchy
	commonAncestors := make(map[types.RuleNodeId]int)

	// Start with first parent's ancestors
	// Starting from the ancestors of the first parent node
	for ancestor, level := range allAncestors[0] {
		minLevel := level
		isCommon := true

		// Check if this ancestor exists in all other parents' ancestors
		// Check whether this ancestor exists among the ancestors of all other parent nodes
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
	// If no common ancestor is found, return false
	if len(commonAncestors) == 0 {
		return types.RuleNodeId{}, false
	}

	// Find the lowest (highest level number, closest to leaves) common ancestor
	// Find the lowest common ancestor (lowest level of layers, closest to leaf nodes).
	var lowestAncestor types.RuleNodeId
	minLevel := -1

	for ancestor, level := range commonAncestors {
		if minLevel == -1 || level < minLevel {
			minLevel = level
			lowestAncestor = ancestor
		}
	}

	return lowestAncestor, true
}

// getAncestorsByLevel performs level-by-level BFS to find ancestors grouped by distance
// getAncestorsByLevel performs layer-by-layer BFS searches for ancestors grouped by distance
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
// isAncestor checks whether ancestors are ancestors of descendants
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
// ClearCache clears LCA cache
func (lca *LCACalculator) ClearCache() {
	lca.cacheMutex.Lock()
	defer lca.cacheMutex.Unlock()
	lca.cache = make(map[types.RuleNodeId]types.RuleNodeId)
}

// GetCacheSize returns the current cache size
// GetCacheSize returns the current cache size
func (lca *LCACalculator) GetCacheSize() int {
	lca.cacheMutex.RLock()
	defer lca.cacheMutex.RUnlock()
	return len(lca.cache)
}
