package lca

import (
	"testing"

	"github.com/rulego/rulego/api/types"
)

// mockParentProvider implements ParentProvider for testing
// mockParentProvider 实现用于测试的ParentProvider
type mockParentProvider struct {
	parentMap map[types.RuleNodeId][]types.RuleNodeId
}

func newMockParentProvider() *mockParentProvider {
	return &mockParentProvider{
		parentMap: make(map[types.RuleNodeId][]types.RuleNodeId),
	}
}

func (m *mockParentProvider) GetParentNodeIds(id types.RuleNodeId) ([]types.RuleNodeId, bool) {
	parents, exists := m.parentMap[id]
	return parents, exists
}

func (m *mockParentProvider) addParent(nodeId, parentId types.RuleNodeId) {
	if m.parentMap[nodeId] == nil {
		m.parentMap[nodeId] = make([]types.RuleNodeId, 0)
	}
	m.parentMap[nodeId] = append(m.parentMap[nodeId], parentId)
}

func createNodeId(id string) types.RuleNodeId {
	return types.RuleNodeId{Id: id}
}

func TestLCACalculator_SingleParent(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create a simple chain: node1 -> node2 -> node3
	// 创建简单链：node1 -> node2 -> node3
	node1 := createNodeId("node1")
	node2 := createNodeId("node2")
	node3 := createNodeId("node3")

	provider.addParent(node2, node1)
	provider.addParent(node3, node2)

	// Test single parent LCA
	// 测试单父节点LCA
	lca, found := calculator.GetLCA(node3)
	if !found {
		t.Error("Expected to find LCA for node3")
	}
	if lca.Id != node1.Id {
		t.Errorf("Expected LCA to be node1, got %s", lca.Id)
	}
}

func TestLCACalculator_MultipleParents(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create diamond structure:
	//     node1
	//    /     \
	//  node2   node3
	//    \     /
	//     node4
	// 创建菱形结构
	node1 := createNodeId("node1")
	node2 := createNodeId("node2")
	node3 := createNodeId("node3")
	node4 := createNodeId("node4")

	provider.addParent(node2, node1)
	provider.addParent(node3, node1)
	provider.addParent(node4, node2)
	provider.addParent(node4, node3)

	// Test multiple parents LCA
	// 测试多父节点LCA
	lca, found := calculator.GetLCA(node4)
	if !found {
		t.Error("Expected to find LCA for node4")
	}
	if lca.Id != node1.Id {
		t.Errorf("Expected LCA to be node1, got %s", lca.Id)
	}
}

func TestLCACalculator_ComplexJoinCase(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Recreate the complex join case from the original test
	// 重新创建原始测试中的复杂join案例
	node4 := createNodeId("node_4")
	node5 := createNodeId("node_5")
	node7 := createNodeId("node_7")
	node13 := createNodeId("node_13")
	node14 := createNodeId("node_14")

	// Set up the relationships as in the original test
	// 设置与原始测试相同的关系
	provider.addParent(node5, node13)
	provider.addParent(node14, node4)
	provider.addParent(node14, node13) // node14 has two parents: node4 and node13
	provider.addParent(node7, node5)
	provider.addParent(node7, node14)

	// Test the complex join case
	// 测试复杂join案例
	lca, found := calculator.GetLCA(node7)
	if !found {
		t.Error("Expected to find LCA for node7")
	}
	if lca.Id != node13.Id {
		t.Errorf("Expected LCA to be node13, got %s", lca.Id)
	}
}

func TestLCACalculator_NoParents(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	node1 := createNodeId("node1")

	// Test node with no parents
	// 测试没有父节点的节点
	_, found := calculator.GetLCA(node1)
	if found {
		t.Error("Expected not to find LCA for node with no parents")
	}
}

func TestLCACalculator_Cache(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create a simple structure
	// 创建简单结构
	node1 := createNodeId("node1")
	node2 := createNodeId("node2")
	node3 := createNodeId("node3")

	provider.addParent(node2, node1)
	provider.addParent(node3, node2)

	// First call should compute and cache
	// 第一次调用应该计算并缓存
	lca1, found1 := calculator.GetLCA(node3)
	if !found1 {
		t.Error("Expected to find LCA for node3")
	}

	// Second call should use cache
	// 第二次调用应该使用缓存
	lca2, found2 := calculator.GetLCA(node3)
	if !found2 {
		t.Error("Expected to find LCA for node3 from cache")
	}

	if lca1.Id != lca2.Id {
		t.Error("Cache should return same result")
	}

	// Check cache size
	// 检查缓存大小
	if calculator.GetCacheSize() == 0 {
		t.Error("Expected cache to contain entries")
	}

	// Clear cache and verify
	// 清除缓存并验证
	calculator.ClearCache()
	if calculator.GetCacheSize() != 0 {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestLCACalculator_IsAncestor(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create chain: node1 -> node2 -> node3
	// 创建链：node1 -> node2 -> node3
	node1 := createNodeId("node1")
	node2 := createNodeId("node2")
	node3 := createNodeId("node3")

	provider.addParent(node2, node1)
	provider.addParent(node3, node2)

	// Test ancestor relationships
	// 测试祖先关系
	if !calculator.isAncestor(node1, node3) {
		t.Error("node1 should be ancestor of node3")
	}

	if !calculator.isAncestor(node2, node3) {
		t.Error("node2 should be ancestor of node3")
	}

	if calculator.isAncestor(node3, node1) {
		t.Error("node3 should not be ancestor of node1")
	}

	if calculator.isAncestor(node1, node1) {
		t.Error("node should not be ancestor of itself")
	}
}

func TestLCACalculator_CrossLevelAncestors(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create structure where common ancestor is at different levels
	// 创建共同祖先在不同层级的结构
	//       node1
	//      /     \
	//   node2    node3
	//   /          \
	// node4        node5
	//   \          /
	//    \        /
	//     node6
	node1 := createNodeId("node1")
	node2 := createNodeId("node2")
	node3 := createNodeId("node3")
	node4 := createNodeId("node4")
	node5 := createNodeId("node5")
	node6 := createNodeId("node6")

	provider.addParent(node2, node1)
	provider.addParent(node3, node1)
	provider.addParent(node4, node2)
	provider.addParent(node5, node3)
	provider.addParent(node6, node4)
	provider.addParent(node6, node5)

	// Test cross-level LCA
	// 测试跨层级LCA
	lca, found := calculator.GetLCA(node6)
	if !found {
		t.Error("Expected to find LCA for node6")
	}
	if lca.Id != node1.Id {
		t.Errorf("Expected LCA to be node1, got %s", lca.Id)
	}
}

func BenchmarkLCACalculator_GetLCA(b *testing.B) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create a complex structure for benchmarking
	// 创建复杂结构用于基准测试
	nodes := make([]types.RuleNodeId, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = createNodeId(string(rune('a' + i)))
	}

	// Create a tree structure
	// 创建树结构
	for i := 1; i < 100; i++ {
		provider.addParent(nodes[i], nodes[i/2])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculator.GetLCA(nodes[99])
	}
}

// TestLCACalculator_Concurrency tests concurrent access to LCA calculator
// TestLCACalculator_Concurrency 测试LCA计算器的并发访问
func TestLCACalculator_Concurrency(t *testing.T) {
	provider := newMockParentProvider()
	calculator := NewLCACalculator(provider)

	// Create test structure
	// 创建测试结构
	root := createNodeId("root")
	node1 := createNodeId("node_1")
	node2 := createNodeId("node_2")
	node3 := createNodeId("node_3")
	node4 := createNodeId("node_4")
	node5 := createNodeId("node_5")
	node6 := createNodeId("node_6")

	provider.addParent(node1, root)
	provider.addParent(node2, root)
	provider.addParent(node3, node1)
	provider.addParent(node3, node2)
	provider.addParent(node4, node1)
	provider.addParent(node5, node2)
	provider.addParent(node6, node3)
	provider.addParent(node6, node4)

	// Test concurrent reads and writes
	// 测试并发读写
	const numGoroutines = 100
	const numOperations = 1000

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines performing LCA calculations
	// 启动多个goroutine执行LCA计算
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			nodeIds := []types.RuleNodeId{node3, node4, node5, node6}

			for j := 0; j < numOperations; j++ {
				nodeId := nodeIds[j%len(nodeIds)]

				// Perform LCA calculation
				// 执行LCA计算
				lca, found := calculator.GetLCA(nodeId)
				if !found {
					t.Errorf("Goroutine %d: LCA not found for %s", id, nodeId.Id)
					return
				}

				// Verify result consistency
				// 验证结果一致性
				switch nodeId.Id {
				case "node_3", "node_4", "node_5", "node_6":
					// All these nodes should have root as LCA since they all trace back to root
					// 所有这些节点的LCA都应该是root，因为它们都可以追溯到root
					if lca.Id != "root" {
						t.Errorf("Goroutine %d: Expected root, got %s for %s", id, lca.Id, nodeId.Id)
						return
					}
				}

				// Occasionally clear cache to test concurrent cache operations
				// 偶尔清空缓存以测试并发缓存操作
				if j%100 == 0 && id == 0 {
					calculator.ClearCache()
				}

				// Test cache size access
				// 测试缓存大小访问
				if j%50 == 0 {
					_ = calculator.GetCacheSize()
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}