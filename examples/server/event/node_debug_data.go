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

package event

import (
	"github.com/rulego/rulego/api/types"
	"sort"
	"sync"
)

//基于内存的日志存储，用于查询节点调试数据
//每个节点只保留一定的条数，最旧的数据会被自动删除
//如果需要查询历史数据，请把调试日志数据存放数据库等可以持久化载体

// RuleChainDebugData 规则链下节点调试数据
type RuleChainDebugData struct {
	//Data 规则链ID->节点列表调试数据
	Data map[string]*NodeDebugData
	// MaxSize 每个节点允许的最大数量
	MaxSize int
	mu      sync.RWMutex
}

// NewRuleChainDebugData 创建一个新的规则链调试数据列表数据
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

// Get 获取指定规则链的节点调试数据列表
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
func (d *RuleChainDebugData) GetToPage(chainId string, nodeId string) DebugDataPage {
	list := d.Get(chainId, nodeId)
	var page = DebugDataPage{}
	if list != nil {
		page.Total = list.Len()
		//ts降序排序
		sort.Slice(list.Items, func(i, j int) bool {
			return list.Items[i].Ts > list.Items[j].Ts
		})
		page.Items = list.Items
	}
	return page
}
func (d *RuleChainDebugData) Clear(chainId string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.Data, chainId)
}

// NodeDebugData 节点调试数据
type NodeDebugData struct {
	Data map[string]*FixedQueue
	// MaxSize 每个节点允许的最大数量
	MaxSize int
	mu      sync.RWMutex
}

// NewNodeDebugData 创建一个新的节点调试数据列表数据
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

	list.Push(data)
}

// Get 获取自定节点列表数据
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

// DebugData 调试数据
// OnDebug 回调函数提供的数据
type DebugData struct {
	//debug数据发生时间
	Ts int64 `json:"ts"`
	//节点ID
	NodeId string `json:"nodeId"`
	//流向OUT/IN
	FlowType string `json:"flowType"`
	//消息
	Msg types.RuleMsg `json:"msg"`
	//关系
	RelationType string `json:"relationType"`
	//Err 错误
	Err string `json:"err"`
}

// DebugDataPage 分页返回数据
type DebugDataPage struct {
	//每页多少条，默认读取所有
	Size int `json:"Size"`
	//当前第几页，默认读取所有
	Current int `json:"current"`
	//总数
	Total int `json:"total"`
	//记录
	Items []DebugData `json:"items"`
}

// FixedQueue 固定大小的队列，如果超过会自动清除最旧的数据
type FixedQueue struct {
	// Items 数据列表
	Items []DebugData
	// MaxSize 最大允许的条数
	MaxSize int
	mu      sync.RWMutex
}

// NewFixedQueue 创建一个新的固定大小的队列
func NewFixedQueue(maxSize int) *FixedQueue {
	return &FixedQueue{
		Items:   make([]DebugData, 0, maxSize),
		MaxSize: maxSize,
	}
}

// Push 向队列中添加一个元素，如果超过最大大小，会删除最旧的元素
func (q *FixedQueue) Push(item DebugData) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Items) == q.MaxSize {
		q.Items = q.Items[1:]
	}
	q.Items = append(q.Items, item)
}

// Pop 从队列中弹出一个元素，如果队列为空，返回false
func (q *FixedQueue) Pop() (DebugData, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Items) == 0 {
		return DebugData{}, false
	}
	item := q.Items[0]
	q.Items = q.Items[1:]
	return item, true
}

// Len 返回队列中的元素个数
func (q *FixedQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.Items)
}

// Peek 返回队列中的第一个元素，但不删除它，如果队列为空，返回false
func (q *FixedQueue) Peek() (DebugData, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.Items) == 0 {
		return DebugData{}, false
	}
	return q.Items[0], true
}

// Clear 清空队列中的所有元素
func (q *FixedQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Items = make([]DebugData, 0, q.MaxSize)
}
