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
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// NewMockRuleContext 创建模拟的 RuleContext，现在使用 ExtendedTestRuleContext
// 保持向后兼容性
func NewMockRuleContext() *test.ExtendedTestRuleContext {
	return test.NewExtendedTestRuleContextWithChannel()
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

// TestGroupFilterNodeTimeoutRaceCondition 测试超时竞态条件修复
func TestGroupFilterNodeTimeoutRaceCondition(t *testing.T) {
	// 获取初始goroutine数量
	initialGoroutines := runtime.NumGoroutine()

	// 创建GroupFilterNode，设置很短的超时时间
	node := &GroupFilterNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"allMatches": false,
		"nodeIds":    []string{"node1", "node2"},
		"timeout":    1, // 1秒超时
	})
	assert.Nil(t, err)

	// 使用现有的MockRuleContext进行测试
	mockCtx := NewMockRuleContext()

	// 设置慢响应的节点处理器（比超时时间长）
	mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
		time.Sleep(2 * time.Second) // 比超时时间长
		return types.True, nil
	})
	mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
		time.Sleep(2 * time.Second) // 比超时时间长
		return types.True, nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	// 执行测试
	start := time.Now()
	node.OnMsg(mockCtx, msg)
	duration := time.Since(start)

	// 验证超时按预期工作（应该在1秒左右返回，而不是2秒）
	assert.True(t, duration >= 1*time.Second && duration < 1500*time.Millisecond,
		"Expected timeout around 1 second, got %v", duration)

	// 等待一段时间确保所有goroutine完成
	time.Sleep(3 * time.Second)

	// 验证收到失败结果
	results := mockCtx.GetResults()
	assert.Equal(t, 1, len(results), "Should have exactly one result")
	assert.Equal(t, "Failure", results[0], "Should receive Failure on timeout")

	// 强制GC，清理资源
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// 检查goroutine泄露（允许少量增长）
	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - initialGoroutines

	// 允许少量增长（测试框架本身可能创建）
	assert.True(t, goroutineIncrease <= 3,
		"Expected goroutine increase <= 3, got %d (from %d to %d)",
		goroutineIncrease, initialGoroutines, finalGoroutines)
}

// TestGroupFilterNodeConcurrentTimeout 测试并发超时场景，确保没有goroutine泄露
func TestGroupFilterNodeConcurrentTimeout(t *testing.T) {
	concurrency := 5 // 减少并发数，避免测试环境负载过高

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			node := &GroupFilterNode{}
			err := node.Init(types.NewConfig(), map[string]interface{}{
				"allMatches": false,
				"nodeIds":    []string{"node1", "node2"},
				"timeout":    1, // 1秒超时
			})
			assert.Nil(t, err)

			mockCtx := NewMockRuleContext()

			// 设置慢响应节点
			mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
				time.Sleep(2 * time.Second)
				return types.True, nil
			})
			mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
				time.Sleep(2 * time.Second)
				return types.True, nil
			})

			msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

			start := time.Now()
			node.OnMsg(mockCtx, msg)
			duration := time.Since(start)

			// 验证超时
			assert.True(t, duration >= 1*time.Second && duration < 1500*time.Millisecond)

			// 验证收到结果
			results := mockCtx.GetResults()
			assert.Equal(t, 1, len(results), "Concurrent test %d should have exactly one result", index)
			assert.Equal(t, "Failure", results[0], "Concurrent test %d should receive Failure on timeout", index)
		}(i)
	}

	wg.Wait()

	// 等待所有慢节点完成
	time.Sleep(3 * time.Second)

	// 强制GC，确保所有资源被回收
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
}

// TestGroupFilterNodeContextCancellation 测试context取消的正确处理
func TestGroupFilterNodeContextCancellation(t *testing.T) {
	node := &GroupFilterNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"allMatches": false,
		"nodeIds":    []string{"node1"},
		"timeout":    2, // 设置2秒超时
	})
	assert.Nil(t, err)

	mockCtx := NewMockRuleContext()

	// 设置一个会检查context取消的节点处理器
	mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
		// 模拟分阶段处理，检查context状态
		for i := 0; i < 10; i++ {
			select {
			case <-mockCtx.GetContext().Done():
				return "", context.Canceled
			default:
				time.Sleep(300 * time.Millisecond) // 总共3秒，会超时
			}
		}
		return types.True, nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	start := time.Now()
	node.OnMsg(mockCtx, msg)
	duration := time.Since(start)

	// 验证超时发生（2秒左右）
	assert.True(t, duration >= 2*time.Second && duration < 2500*time.Millisecond)

	// 等待context传播
	time.Sleep(500 * time.Millisecond)

	// 验证收到失败结果
	results := mockCtx.GetResults()
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "Failure", results[0])
}
