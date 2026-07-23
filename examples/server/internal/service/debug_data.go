/*
 * Copyright 2023 The RuleGo Authors.
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

package service

import (
	"sort"
	"sync"

	"github.com/rulego/rulego/api/types"
)

// DebugDataPool provides object pool optimization for DebugData
type DebugDataPool struct {
	pool sync.Pool
}

// Global DebugData object pool
var globalDebugDataPool = &DebugDataPool{
	pool: sync.Pool{
		New: func() interface{} {
			return &DebugData{}
		},
	},
}

// GetDebugData retrieves DebugData instances from the object pool
func (p *DebugDataPool) Get() *DebugData {
	return p.pool.Get().(*DebugData)
}

// PutDebugData retrieves DebugData instances into the object pool
func (p *DebugDataPool) Put(data *DebugData) {
	if data != nil {
		data.Reset()
		p.pool.Put(data)
	}
}

// Reset: Resets the DebugData state to prepare for object pool reuse
func (d *DebugData) Reset() {
	d.Ts = 0
	d.NodeId = ""
	d.FlowType = ""
	d.Msg = types.RuleMsg{} // Reset to zero
	d.RelationType = ""
	d.Err = ""
}

// NewDebugData creates new DebugData instances and optimizes them using an object pool
func NewDebugData(ts int64, nodeId, flowType string, msg types.RuleMsg, relationType, errStr string) *DebugData {
	data := globalDebugDataPool.Get()
	data.Ts = ts
	data.NodeId = nodeId
	data.FlowType = flowType
	data.Msg = msg
	data.RelationType = relationType
	data.Err = errStr
	return data
}

//Memory-based log storage for querying node debug data
//Each node only retains a certain number of entries, and the oldest data is automatically deleted
//If you need to query historical data, please store the debug log data in a database or other persistent medium

// RuleChainDebugData Rules off-chain node debug data
type RuleChainDebugData struct {
	//Data Rule Chain ID- > node list for debugging data
	Data map[string]*NodeDebugData
	// MaxSize: The maximum number allowed per node
	MaxSize int
	mu      sync.RWMutex
}

// NewRuleChainDebugData creates a new list of rule chain debug data
func NewRuleChainDebugData(maxSize int) *RuleChainDebugData {
	if maxSize <= 0 {
		maxSize = 60
	}
	return &RuleChainDebugData{
		Data:    make(map[string]*NodeDebugData),
		MaxSize: maxSize,
	}
}

func (d *RuleChainDebugData) Add(chainId string, nodeId string, data DebugData) {
	d.mu.Lock()
	ruleChainData, ok := d.Data[chainId]
	if !ok {
		ruleChainData = NewNodeDebugData(d.MaxSize)
		d.Data[chainId] = ruleChainData
	}
	defer d.mu.Unlock()

	ruleChainData.Add(nodeId, data)
}

// Get the list of node debug data for the specified rule chain
func (d *RuleChainDebugData) Get(chainId string, nodeId string) *FixedQueue {
	d.mu.RLock()
	ruleChainData, ok := d.Data[chainId]
	defer d.mu.RUnlock()
	if ok {
		return ruleChainData.Get(nodeId)
	} else {
		return nil
	}
}
func (d *RuleChainDebugData) GetToPage(chainId string, nodeId string, pageSize, current int) DebugDataPage {
	list := d.Get(chainId, nodeId)
	var page = DebugDataPage{}
	if list != nil {
		page.Total = list.Len()
		//ts descending sort
		sort.Slice(list.Items, func(i, j int) bool {
			return list.Items[i].Ts > list.Items[j].Ts
		})
		if pageSize == 0 {
			pageSize = page.Total
		}
		if current <= 0 {
			current = 1
		}
		// Calculate the page index
		start := (current - 1) * pageSize
		end := start + pageSize
		page.PageSize = pageSize
		page.Current = current
		// Check if the starting index is outside the list range
		if start >= page.Total {
			page.Items = []DebugData{} // If it exceeds the range, it returns to an empty list
		} else {
			// Calculate the end index to prevent exceeding the maximum list length
			if end > page.Total {
				end = page.Total
			}
			// To retrieve paginated data based on the index range, you need to dereference pointers
			items := make([]DebugData, end-start)
			for i, ptr := range list.Items[start:end] {
				items[i] = *ptr
			}
			page.Items = items
		}
	}
	return page
}
func (d *RuleChainDebugData) Clear(chainId string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.Data, chainId)
}

// NodeDebugData node debug data
type NodeDebugData struct {
	Data map[string]*FixedQueue
	// MaxSize: The maximum number allowed per node
	MaxSize int
	mu      sync.RWMutex
}

// NewNodeDebugData creates a new node debug data list data
func NewNodeDebugData(maxSize int) *NodeDebugData {
	if maxSize <= 0 {
		maxSize = 60
	}
	return &NodeDebugData{
		Data:    make(map[string]*FixedQueue),
		MaxSize: maxSize,
	}
}

func (d *NodeDebugData) Add(nodeId string, data DebugData) {
	d.mu.Lock()
	list, ok := d.Data[nodeId]
	if !ok {
		list = NewFixedQueue(d.MaxSize)
		d.Data[nodeId] = list
	}
	defer d.mu.Unlock()

	// Create DebugData pointers using the object pool
	dataPtr := globalDebugDataPool.Get()
	*dataPtr = data // Copy data to pooling objects
	list.Push(dataPtr)
}

// Get the custom node list data
func (d *NodeDebugData) Get(nodeId string) *FixedQueue {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if list, ok := d.Data[nodeId]; ok {
		return list
	} else {
		return nil
	}

}

func (d *NodeDebugData) Clear(nodeId string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.Data, nodeId)
}

// DebugData debugging data
// The data provided by the OnDebug callback function
type DebugData struct {
	//Debug data occurrence time
	Ts int64 `json:"ts"`
	//Node ID
	NodeId string `json:"nodeId"`
	//Flow to OUT/IN
	FlowType string `json:"flowType"`
	//News
	Msg types.RuleMsg `json:"msg"`
	//Relationships
	RelationType string `json:"relationType"`
	//Err is incorrect
	Err string `json:"err"`
}

// DebugDataPage paginates to return data
type DebugDataPage struct {
	//How many entries per page is read by default
	PageSize int `json:"pageSize"`
	//Current page number, read all by default
	Current int `json:"current"`
	//Total
	Total int `json:"total"`
	//Record
	Items []DebugData `json:"items"`
}

// FixedQueue: A fixed-size queue; if exceeded, the oldest data will be automatically cleared
type FixedQueue struct {
	// Items data list, using pointers for object pool collection
	Items []*DebugData
	// MaxSize: The maximum number of entries allowed
	MaxSize int
	mu      sync.RWMutex
}

// NewFixedQueue creates a new fixed-size queue
func NewFixedQueue(maxSize int) *FixedQueue {
	return &FixedQueue{
		Items:   make([]*DebugData, 0, maxSize),
		MaxSize: maxSize,
	}
}

// Push adds an element to the queue; if it exceeds the maximum size, the oldest element is deleted
func (q *FixedQueue) Push(item *DebugData) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Items) == q.MaxSize {
		// Recycle the oldest elements into the object pool
		oldData := q.Items[0]
		globalDebugDataPool.Put(oldData)
		q.Items = q.Items[1:]
	}
	q.Items = append(q.Items, item)
}

// Pop pops an element from the queue, and returns false if the queue is empty
func (q *FixedQueue) Pop() (*DebugData, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Items) == 0 {
		return nil, false
	}
	item := q.Items[0]
	q.Items = q.Items[1:]
	return item, true
}

// Len returns the number of elements in the queue
func (q *FixedQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.Items)
}

// Peek returns the first element in the queue but does not remove it; if the queue is empty, it returns false
func (q *FixedQueue) Peek() (*DebugData, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.Items) == 0 {
		return nil, false
	}
	return q.Items[0], true
}

// Clear: Clears all elements in the queue
func (q *FixedQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	// Retrieve all elements into the object pool
	for _, item := range q.Items {
		globalDebugDataPool.Put(item)
	}
	q.Items = make([]*DebugData, 0, q.MaxSize)
}
