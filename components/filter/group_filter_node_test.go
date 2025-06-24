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

package filter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// MockRuleContext 模拟 RuleContext 进行测试
type MockRuleContext struct {
	mutex   sync.RWMutex
	nodeMap map[string]func(msg types.RuleMsg) (string, error)
	results []string
}

func NewMockRuleContext() *MockRuleContext {
	return &MockRuleContext{
		nodeMap: make(map[string]func(msg types.RuleMsg) (string, error)),
		results: make([]string, 0),
	}
}

func (m *MockRuleContext) GetContext() context.Context {
	return context.Background()
}

func (m *MockRuleContext) TellNode(ctx context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool,
	onEnd types.OnEndFunc, onAllNodeCompleted func()) {

	if handler, exists := m.nodeMap[nodeId]; exists {
		// 模拟异步执行
		go func() {
			relationType, err := handler(msg)
			if onEnd != nil {
				onEnd(m, msg, err, relationType)
			}
		}()
	}
}

func (m *MockRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(relationTypes) > 0 {
		m.results = append(m.results, relationTypes[0])
	}
}

func (m *MockRuleContext) TellFailure(msg types.RuleMsg, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.results = append(m.results, "Failure")
}

func (m *MockRuleContext) TellSuccess(msg types.RuleMsg) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.results = append(m.results, "Success")
}

func (m *MockRuleContext) SetNodeHandler(nodeId string, handler func(msg types.RuleMsg) (string, error)) {
	m.nodeMap[nodeId] = handler
}

func (m *MockRuleContext) GetResults() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	results := make([]string, len(m.results))
	copy(results, m.results)
	return results
}

// 实现其他必要的方法（空实现）
func (m *MockRuleContext) TellSelf(msg types.RuleMsg, delayMs int64) {}
func (m *MockRuleContext) TellNextOrElse(msg types.RuleMsg, defaultRelationType string, relationTypes ...string) {
}
func (m *MockRuleContext) TellFlow(ctx context.Context, ruleChainId string, msg types.RuleMsg, endFunc types.OnEndFunc, onAllNodeCompleted func()) {
}
func (m *MockRuleContext) TellChainNode(ctx context.Context, ruleChainId, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc, onAllNodeCompleted func()) {
}
func (m *MockRuleContext) NewMsg(msgType string, metaData *types.Metadata, data string) types.RuleMsg {
	return types.RuleMsg{}
}
func (m *MockRuleContext) GetSelfId() string                                         { return "" }
func (m *MockRuleContext) Self() types.NodeCtx                                       { return nil }
func (m *MockRuleContext) From() types.NodeCtx                                       { return nil }
func (m *MockRuleContext) RuleChain() types.NodeCtx                                  { return nil }
func (m *MockRuleContext) Config() types.Config                                      { return types.Config{} }
func (m *MockRuleContext) SubmitTack(task func())                                    {}
func (m *MockRuleContext) SubmitTask(task func())                                    {}
func (m *MockRuleContext) SetEndFunc(f types.OnEndFunc) types.RuleContext            { return m }
func (m *MockRuleContext) GetEndFunc() types.OnEndFunc                               { return nil }
func (m *MockRuleContext) SetContext(c context.Context) types.RuleContext            { return m }
func (m *MockRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func())           {}
func (m *MockRuleContext) DoOnEnd(msg types.RuleMsg, err error, relationType string) {}
func (m *MockRuleContext) SetCallbackFunc(functionName string, f interface{})        {}
func (m *MockRuleContext) GetCallbackFunc(functionName string) interface{}           { return nil }
func (m *MockRuleContext) OnDebug(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
}
func (m *MockRuleContext) SetExecuteNode(nodeId string, relationTypes ...string) {}
func (m *MockRuleContext) TellCollect(msg types.RuleMsg, callback func(msgList []types.WrapperMsg)) bool {
	return false
}
func (m *MockRuleContext) GetOut() types.RuleMsg    { return types.RuleMsg{} }
func (m *MockRuleContext) GetErr() error            { return nil }
func (m *MockRuleContext) GlobalCache() types.Cache { return nil }
func (m *MockRuleContext) ChainCache() types.Cache  { return nil }
func (m *MockRuleContext) GetEnv(msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	return nil
}

func TestGroupFilterNode(t *testing.T) {
	var targetNodeType = "groupFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &GroupFilterNode{}, types.Configuration{
			"allMatches": false,
		}, Registry)
	})

	t.Run("InitNode1", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"allMatches": true,
		}, types.Configuration{
			"allMatches": true,
		}, Registry)
	})
	t.Run("InitNode2", func(t *testing.T) {
		node1, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 2,
			"nodeIds":  "s1,s2",
			"timeout":  10,
		}, Registry)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 2,
			"nodeIds":  []string{"s1", "s2"},
			"timeout":  10,
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 2,
			"nodeIds":  []interface{}{"s1", "s2"},
			"timeout":  10,
		}, Registry)
		assert.Equal(t, node1.(*GroupFilterNode).NodeIdList, node2.(*GroupFilterNode).NodeIdList)
		assert.Equal(t, node3.(*GroupFilterNode).NodeIdList, node2.(*GroupFilterNode).NodeIdList)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"allMatches": false,
		}, types.Configuration{
			"allMatches": false,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {

		groupFilterNode1, err := test.CreateAndInitNode("groupFilter", types.Configuration{
			"allMatches": false,
			"nodeIds":    "node1,node2,node3,noFoundId",
			"timeout":    10,
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode2, err := test.CreateAndInitNode("groupFilter", types.Configuration{
			"allMatches": true,
			"nodeIds":    "node1,node2",
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode3, err := test.CreateAndInitNode("groupFilter", types.Configuration{
			"allMatches": false,
			"nodeIds":    "node1,node2,node3,noFoundId",
		}, Registry)

		groupFilterNode4, err := test.CreateAndInitNode("groupFilter", types.Configuration{
			"allMatches": false,
		}, Registry)

		groupFilterNode5, err := test.CreateAndInitNode("groupFilter", types.Configuration{
			"allMatches": true,
			"nodeIds":    "node1,node2,node3,noFoundId",
		}, Registry)

		//groupFilterNode6, err := test.CreateAndInitNode("groupFilter", types.Configuration{
		//	"allMatches": true,
		//	"nodeIds":    "timeoutNode",
		//	"timeout":    1,
		//}, Registry)

		node1, err := test.CreateAndInitNode("jsFilter", types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, Registry)

		node2, _ := test.CreateAndInitNode("jsFilter", types.Configuration{
			"jsScript": `return msg.humidity > 80;`,
		}, Registry)
		node3, _ := test.CreateAndInitNode("jsFilter", types.Configuration{
			"jsScript": `return a`,
		}, Registry)
		//timeoutNode, _ := test.CreateAndInitNode("jsFilter", types.Configuration{
		//	"jsScript": `sleep(2000);return a`,
		//}, Registry)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "{\"temperature\":41,\"humidity\":90}",
				AfterSleep: time.Millisecond * 200,
			},
		}
		msgList2 := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "{\"temperature\":61,\"humidity\":90}",
				AfterSleep: time.Millisecond * 200,
			},
		}
		childrenNodes := map[string]types.Node{
			"node1": node1,
			"node2": node2,
			"node3": node3,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:          groupFilterNode1,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:          groupFilterNode2,
				MsgList:       msgList2,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:          groupFilterNode3,
				MsgList:       msgList2,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:          groupFilterNode4,
				MsgList:       msgList2,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:          groupFilterNode5,
				MsgList:       msgList2,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.False, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

	})
}

// TestGroupFilterConcurrencySafety 测试 GroupFilter 的并发安全性
func TestGroupFilterConcurrencySafety(t *testing.T) {
	// 测试 AllMatches=true 的场景
	t.Run("AllMatches=true Concurrency Safety", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": true,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		// 进行多次测试以捕获竞态条件
		for i := 0; i < 100; i++ {
			mockCtx := NewMockRuleContext()

			// 设置节点处理器：node1 返回 False，node2 和 node3 返回 True
			mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 1) // 模拟处理时间
				return types.False, nil
			})
			mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 2) // 模拟处理时间
				return types.True, nil
			})
			mockCtx.SetNodeHandler("node3", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 3) // 模拟处理时间
				return types.True, nil
			})

			msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

			// 执行 GroupFilter
			node.OnMsg(mockCtx, msg)

			// 等待处理完成
			time.Sleep(time.Millisecond * 50)

			results := mockCtx.GetResults()
			assert.Equal(t, 1, len(results), "Should have exactly one result")
			assert.Equal(t, types.False, results[0], fmt.Sprintf("Iteration %d: AllMatches=true with one False should return False, got %s", i, results[0]))
		}
	})

	// 测试 AllMatches=true 且所有节点都返回 True 的场景
	t.Run("AllMatches=true All True", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": true,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		mockCtx := NewMockRuleContext()

		// 设置所有节点都返回 True
		for _, nodeId := range []string{"node1", "node2", "node3"} {
			nodeId := nodeId // capture loop variable
			mockCtx.SetNodeHandler(nodeId, func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 1)
				return types.True, nil
			})
		}

		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)
		node.OnMsg(mockCtx, msg)

		time.Sleep(time.Millisecond * 50)

		results := mockCtx.GetResults()
		assert.Equal(t, 1, len(results), "Should have exactly one result")
		assert.Equal(t, types.True, results[0], "AllMatches=true with all True should return True")
	})

	// 测试 AllMatches=false 的场景
	t.Run("AllMatches=false Concurrency Safety", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": false,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		for i := 0; i < 100; i++ {
			mockCtx := NewMockRuleContext()

			// 设置节点处理器：node1 返回 True，其他返回 False
			mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 1)
				return types.True, nil
			})
			mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 2)
				return types.False, nil
			})
			mockCtx.SetNodeHandler("node3", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 3)
				return types.False, nil
			})

			msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)
			node.OnMsg(mockCtx, msg)

			time.Sleep(time.Millisecond * 50)

			results := mockCtx.GetResults()
			assert.Equal(t, 1, len(results), "Should have exactly one result")
			assert.Equal(t, types.True, results[0], fmt.Sprintf("Iteration %d: AllMatches=false with one True should return True", i))
		}
	})
}

// TestGroupFilterRaceCondition 专门测试竞态条件
func TestGroupFilterRaceCondition(t *testing.T) {
	node := &GroupFilterNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"allMatches": true,
		"nodeIds":    []string{"node1", "node2", "node3", "node4", "node5"},
	})
	assert.Nil(t, err)

	var errorCount int32
	iterations := 500

	for i := 0; i < iterations; i++ {
		mockCtx := NewMockRuleContext()

		// 设置快速并发的节点处理器
		for j, nodeId := range []string{"node1", "node2", "node3", "node4", "node5"} {
			nodeId := nodeId
			j := j
			mockCtx.SetNodeHandler(nodeId, func(msg types.RuleMsg) (string, error) {
				// 第一个节点返回 False，其他返回 True
				if j == 0 {
					return types.False, nil
				}
				return types.True, nil
			})
		}

		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)
		node.OnMsg(mockCtx, msg)

		time.Sleep(time.Millisecond * 10)

		results := mockCtx.GetResults()
		if len(results) != 1 || results[0] != types.False {
			atomic.AddInt32(&errorCount, 1)
		}
	}

	//errorRate := float64(atomic.LoadInt32(&errorCount)) / float64(iterations)
	//t.Logf("Race condition test: %d errors out of %d iterations (%.2f%% error rate)",
	//	atomic.LoadInt32(&errorCount), iterations, errorRate*100)

	// 修复后的代码应该没有竞态条件错误
	assert.Equal(t, int32(0), atomic.LoadInt32(&errorCount), "Should have no race condition errors")
}
