package runlog

import (
	"sort"
	"sync"
)

// DebugDataStore 基于内存的调试数据存储
// 每个节点保留固定条数，最旧的数据会被自动删除
type DebugDataStore struct {
	// chainData 规则链ID -> 节点调试数据
	chainData map[string]*nodeDebugData
	// maxSize 每个节点允许的最大数量
	maxSize int
	mu      sync.RWMutex
}

// NewDebugDataStore 创建调试数据存储
func NewDebugDataStore(maxSize int) *DebugDataStore {
	if maxSize <= 0 {
		maxSize = 60
	}
	return &DebugDataStore{
		chainData: make(map[string]*nodeDebugData),
		maxSize:   maxSize,
	}
}

// Add 添加调试数据
func (s *DebugDataStore) Add(chainId, nodeId string, data map[string]interface{}) {
	s.mu.Lock()
	nodes, ok := s.chainData[chainId]
	if !ok {
		nodes = newNodeDebugData(s.maxSize)
		s.chainData[chainId] = nodes
	}
	s.mu.Unlock()
	nodes.Add(nodeId, data)
}

// GetPage 获取指定节点的调试数据（分页）
func (s *DebugDataStore) GetPage(chainId, nodeId string, page, size int) map[string]interface{} {
	s.mu.RLock()
	nodes, ok := s.chainData[chainId]
	s.mu.RUnlock()
	if !ok {
		return emptyPage(page, size)
	}
	return nodes.GetPage(nodeId, page, size)
}

// Clear 清空指定规则链的调试数据
func (s *DebugDataStore) Clear(chainId string) {
	s.mu.Lock()
	delete(s.chainData, chainId)
	s.mu.Unlock()
}

func emptyPage(page, size int) map[string]interface{} {
	return map[string]interface{}{
		"page":  page,
		"size":  size,
		"total": 0,
		"items": []interface{}{},
	}
}

// nodeDebugData 节点调试数据
type nodeDebugData struct {
	data    map[string]*fixedQueue
	maxSize int
	mu      sync.RWMutex
}

func newNodeDebugData(maxSize int) *nodeDebugData {
	return &nodeDebugData{
		data:    make(map[string]*fixedQueue),
		maxSize: maxSize,
	}
}

func (d *nodeDebugData) Add(nodeId string, item map[string]interface{}) {
	d.mu.Lock()
	q, ok := d.data[nodeId]
	if !ok {
		q = newFixedQueue(d.maxSize)
		d.data[nodeId] = q
	}
	d.mu.Unlock()
	q.Push(item)
}

func (d *nodeDebugData) GetPage(nodeId string, page, size int) map[string]interface{} {
	d.mu.RLock()
	q, ok := d.data[nodeId]
	d.mu.RUnlock()
	if !ok {
		return emptyPage(page, size)
	}

	items := q.Items()
	// 按 ts 降序排序
	sort.Slice(items, func(i, j int) bool {
		tsI, _ := items[i]["ts"].(int64)
		tsJ, _ := items[j]["ts"].(int64)
		return tsI > tsJ
	})

	total := len(items)
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	start := (page - 1) * size
	if start >= total {
		return map[string]interface{}{
			"page":  page,
			"size":  size,
			"total": total,
			"items": []interface{}{},
		}
	}
	end := start + size
	if end > total {
		end = total
	}

	return map[string]interface{}{
		"page":  page,
		"size":  size,
		"total": total,
		"items": items[start:end],
	}
}

// fixedQueue 固定大小的队列
type fixedQueue struct {
	items   []map[string]interface{}
	maxSize int
	mu      sync.RWMutex
}

func newFixedQueue(maxSize int) *fixedQueue {
	return &fixedQueue{
		items:   make([]map[string]interface{}, 0, maxSize),
		maxSize: maxSize,
	}
}

func (q *fixedQueue) Push(item map[string]interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == q.maxSize {
		q.items = q.items[1:]
	}
	q.items = append(q.items, item)
}

func (q *fixedQueue) Items() []map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()
	result := make([]map[string]interface{}, len(q.items))
	copy(result, q.items)
	return result
}

func (q *fixedQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}
