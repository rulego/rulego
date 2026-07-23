package runlog

import (
	"sort"
	"sync"
)

// DebugDataStore is a memory-based debug data store
// Each node retains a fixed number of entries, and the oldest data is automatically deleted
type DebugDataStore struct {
	// chainData rules: chainID -> node debug data
	chainData map[string]*nodeDebugData
	// maxSize: The maximum number allowed per node
	maxSize int
	mu      sync.RWMutex
}

// NewDebugDataStore creates debug data storage
func NewDebugDataStore(maxSize int) *DebugDataStore {
	if maxSize <= 0 {
		maxSize = 60
	}
	return &DebugDataStore{
		chainData: make(map[string]*nodeDebugData),
		maxSize:   maxSize,
	}
}

// Add debug data
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

// GetPage retrieves debug data for specified nodes (pagination)
func (s *DebugDataStore) GetPage(chainId, nodeId string, page, size int) map[string]interface{} {
	s.mu.RLock()
	nodes, ok := s.chainData[chainId]
	s.mu.RUnlock()
	if !ok {
		return emptyPage(page, size)
	}
	return nodes.GetPage(nodeId, page, size)
}

// Clear: Clears the debug data of the specified rule chain
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

// nodeDebugData node debug data
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
	// Sort by ts descending order
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

// fixedQueue A fixed-size queue
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
