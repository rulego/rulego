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

// NewMockRuleContext creates a simulated RuleContext and now uses ExtendedTestRuleContext
// Maintain backward compatibility
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

// TestGroupFilterConcurrencySafety Tests the concurrency security of GroupFilter
func TestGroupFilterConcurrencySafety(t *testing.T) {
	// Test the scenario with AllMatches=true
	t.Run("AllMatches=true Concurrency Safety", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": true,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		// Multiple tests were conducted to capture race conditions
		for i := 0; i < 100; i++ {
			mockCtx := NewMockRuleContext()

			// Set the node processor: node1 returns False, node2 and node3 return True
			mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 1) // Simulated processing time
				return types.False, nil
			})
			mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 2) // Simulated processing time
				return types.True, nil
			})
			mockCtx.SetNodeHandler("node3", func(msg types.RuleMsg) (string, error) {
				time.Sleep(time.Millisecond * 3) // Simulated processing time
				return types.True, nil
			})

			msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

			// Run GroupFilter
			node.OnMsg(mockCtx, msg)

			// Wait for processing to complete
			time.Sleep(time.Millisecond * 50)

			results := mockCtx.GetResults()
			assert.Equal(t, 1, len(results), "Should have exactly one result")
			assert.Equal(t, types.False, results[0], fmt.Sprintf("Iteration %d: AllMatches=true with one False should return False, got %s", i, results[0]))
		}
	})

	// Test a scenario where AllMatches=true and all nodes return true
	t.Run("AllMatches=true All True", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": true,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		mockCtx := NewMockRuleContext()

		// Set all nodes to return True
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

	// Test the scenario where AllMatches=false is used
	t.Run("AllMatches=false Concurrency Safety", func(t *testing.T) {
		node := &GroupFilterNode{}
		err := node.Init(types.NewConfig(), map[string]interface{}{
			"allMatches": false,
			"nodeIds":    []string{"node1", "node2", "node3"},
		})
		assert.Nil(t, err)

		for i := 0; i < 100; i++ {
			mockCtx := NewMockRuleContext()

			// Set the node processor: node1 returns True, others return False
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

// TestGroupFilterRaceCondition is specifically used to test race condition conditions
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

		// Configure fast-concurrency node processors
		for j, nodeId := range []string{"node1", "node2", "node3", "node4", "node5"} {
			nodeId := nodeId
			j := j
			mockCtx.SetNodeHandler(nodeId, func(msg types.RuleMsg) (string, error) {
				// The first node returns False, while the others return True
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

	// The fixed code should have no race condition errors
	assert.Equal(t, int32(0), atomic.LoadInt32(&errorCount), "Should have no race condition errors")
}

// TestGroupFilterNodeTimeoutRaceCondition Fixes timeout race condition for testing
func TestGroupFilterNodeTimeoutRaceCondition(t *testing.T) {
	// Get the initial goroutine quantity
	initialGoroutines := runtime.NumGoroutine()

	// Create a GroupFilterNode and set a very short timeout timeout
	node := &GroupFilterNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"allMatches": false,
		"nodeIds":    []string{"node1", "node2"},
		"timeout":    1, // Timeout of 1 second
	})
	assert.Nil(t, err)

	// Testing is conducted using the existing MockRuleContext
	mockCtx := NewMockRuleContext()

	// Set a slow-response node processor (longer than timeout)
	mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
		time.Sleep(2 * time.Second) // Longer than the overtime period
		return types.True, nil
	})
	mockCtx.SetNodeHandler("node2", func(msg types.RuleMsg) (string, error) {
		time.Sleep(2 * time.Second) // Longer than the overtime period
		return types.True, nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	// Perform the test
	start := time.Now()
	node.OnMsg(mockCtx, msg)
	duration := time.Since(start)

	// Verification timeout works as expected (should return in about 1 second, not 2 seconds)
	assert.True(t, duration >= 1*time.Second && duration < 1500*time.Millisecond,
		"Expected timeout around 1 second, got %v", duration)

	// Wait a while to ensure all goroutines are completed
	time.Sleep(3 * time.Second)

	// Verification received a failed result
	results := mockCtx.GetResults()
	assert.Equal(t, 1, len(results), "Should have exactly one result")
	assert.Equal(t, "Failure", results[0], "Should receive Failure on timeout")

	// Forced GC and resource clearance
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks (allow for small growth)
	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - initialGoroutines

	// Allow for small growth (the testing framework itself may be created)
	assert.True(t, goroutineIncrease <= 3,
		"Expected goroutine increase <= 3, got %d (from %d to %d)",
		goroutineIncrease, initialGoroutines, finalGoroutines)
}

// TestGroupFilterNodeConcurrentTimeout tests concurrent timeout scenarios to ensure no goroutine leaks
func TestGroupFilterNodeConcurrentTimeout(t *testing.T) {
	concurrency := 5 // Reduce concurrency and avoid excessive load in the test environment

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			node := &GroupFilterNode{}
			err := node.Init(types.NewConfig(), map[string]interface{}{
				"allMatches": false,
				"nodeIds":    []string{"node1", "node2"},
				"timeout":    1, // Timeout of 1 second
			})
			assert.Nil(t, err)

			mockCtx := NewMockRuleContext()

			// Set slow response nodes
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

			// Verification timeout
			assert.True(t, duration >= 1*time.Second && duration < 1500*time.Millisecond)

			// Verify the results received
			results := mockCtx.GetResults()
			assert.Equal(t, 1, len(results), "Concurrent test %d should have exactly one result", index)
			assert.Equal(t, "Failure", results[0], "Concurrent test %d should receive Failure on timeout", index)
		}(i)
	}

	wg.Wait()

	// Wait for all slow nodes to complete
	time.Sleep(3 * time.Second)

	// Forced GC to ensure all resources are recycled
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
}

// TestGroupFilterNodeContextCancellation Tests the correct handling of context cancellation
func TestGroupFilterNodeContextCancellation(t *testing.T) {
	node := &GroupFilterNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"allMatches": false,
		"nodeIds":    []string{"node1"},
		"timeout":    2, // Set a 2-second timeout
	})
	assert.Nil(t, err)

	mockCtx := NewMockRuleContext()

	// Set up a node processor that checks for context cancellation
	mockCtx.SetNodeHandler("node1", func(msg types.RuleMsg) (string, error) {
		// Simulate phased processing and check context status
		for i := 0; i < 10; i++ {
			select {
			case <-mockCtx.GetContext().Done():
				return "", context.Canceled
			default:
				time.Sleep(300 * time.Millisecond) // A total of 3 seconds, and time will run out
			}
		}
		return types.True, nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	start := time.Now()
	node.OnMsg(mockCtx, msg)
	duration := time.Since(start)

	// Verification timeout occurs (about 2 seconds)
	assert.True(t, duration >= 2*time.Second && duration < 2500*time.Millisecond)

	// Waiting for the context to spread
	time.Sleep(500 * time.Millisecond)

	// Verification received a failed result
	results := mockCtx.GetResults()
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "Failure", results[0])
}
