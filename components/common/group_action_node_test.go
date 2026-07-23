/*
 * Copyright 2025 The RuleGo Authors.
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

package common

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/components/action"

	"github.com/rulego/rulego/utils/str"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
)

func TestGroupFilterNode(t *testing.T) {
	var targetNodeType = "groupAction"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &GroupActionNode{}, types.Configuration{
			"matchRelationType": types.Success,
		}, Registry)
	})

	t.Run("InitNode1", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"matchRelationType": "",
			"nodeIds":           "s1,s2",
		}, types.Configuration{
			"matchRelationType": types.Success,
			"matchNum":          2,
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
		assert.Equal(t, node1.(*GroupActionNode).NodeIdList, node2.(*GroupActionNode).NodeIdList)
		assert.Equal(t, node3.(*GroupActionNode).NodeIdList, node2.(*GroupActionNode).NodeIdList)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"matchRelationType": types.Success,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {

		//Testing the function
		action.Functions.Register("groupActionTest1", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test1", time.Now().String())
			msg.SetData(`{"addValue":"addFromTest1"}`)
			ctx.TellSuccess(msg)
		})

		action.Functions.Register("groupActionTest2", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test2", time.Now().String())
			msg.SetData(`{"addValue":"addFromTest2"}`)
			ctx.TellSuccess(msg)
		})

		action.Functions.Register("groupActionTestFailure", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 100)
			ctx.TellFailure(msg, errors.New("test error"))
		})

		groupFilterNode1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 2,
			"nodeIds":  "node1,node2,node3,noFoundId",
			"timeout":  10,
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 2,
			"nodeIds":  "node1,node2",
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 1,
			"nodeIds":  "node1,node2,node3,noFoundId",
		}, Registry)

		groupFilterNode4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"nodeIds": "node1,node2",
		}, Registry)

		groupFilterNode5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 4,
			"nodeIds":  "node1,node2,node3,noFoundId",
		}, Registry)

		groupFilterNode6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"nodeIds": "",
		}, Registry)

		groupFilterNode7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"matchNum": 1,
			"nodeIds":  "node3,node4",
		}, Registry)

		node1, err := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTest1",
		}, action.Registry)

		node2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTest2",
		}, action.Registry)
		node3, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTestFailure",
		}, action.Registry)
		node4, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "notFound",
		}, action.Registry)

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
		childrenNodes := map[string]types.Node{
			"node1": node1,
			"node2": node2,
			"node3": node3,
			"node4": node4,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:          groupFilterNode1,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) >= 1)
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:          groupFilterNode2,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) == 2)
					assert.Equal(t, "node1", result[0].(map[string]interface{})["nodeId"])
					assert.Equal(t, "node2", result[1].(map[string]interface{})["nodeId"])
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:          groupFilterNode3,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) >= 1)
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:          groupFilterNode4,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) == 2)
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:          groupFilterNode5,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) >= 0)
					assert.Equal(t, "node1", result[0].(map[string]interface{})["nodeId"])
					assert.Equal(t, "node2", result[1].(map[string]interface{})["nodeId"])
					assert.Equal(t, "node3", result[2].(map[string]interface{})["nodeId"])

					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:          groupFilterNode6,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:          groupFilterNode7,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					var result []interface{}
					_ = json.Unmarshal([]byte(msg.GetData()), &result)
					assert.True(t, len(result) >= 0)
					assert.Equal(t, "node3", result[0].(map[string]interface{})["nodeId"])
					assert.Equal(t, "node4", result[1].(map[string]interface{})["nodeId"])

					assert.Equal(t, types.Failure, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)

	})

	t.Run("MergeToMap", func(t *testing.T) {
		//Testing the function
		action.Functions.Register("groupActionTestJson1", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.DataType = types.JSON
			msg.SetData(`{"a": 1, "b": 2}`)
			ctx.TellSuccess(msg)
		})
		action.Functions.Register("groupActionTestJson2", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.DataType = types.JSON
			msg.SetData(`{"c": 3}`)
			ctx.TellSuccess(msg)
		})
		action.Functions.Register("groupActionTestText", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.DataType = types.TEXT
			msg.SetData(`not json`)
			ctx.TellSuccess(msg)
		})

		node1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTestJson1",
		}, action.Registry)
		node2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTestJson2",
		}, action.Registry)
		node3, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTestText",
		}, action.Registry)

		childrenNodes := map[string]types.Node{
			"node1": node1,
			"node2": node2,
			"node3": node3,
		}

		// Case 1: MergeToMap = true
		t.Run("True", func(t *testing.T) {
			groupNode, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"matchNum":   3,
				"nodeIds":    "node1,node2,node3",
				"mergeToMap": true,
			}, Registry)
			assert.Nil(t, err)

			msgList := []test.Msg{
				{
					MsgType:    "ACTIVITY_EVENT1",
					Data:       "{}",
					AfterSleep: time.Millisecond * 200,
				},
			}

			nodeCallback := test.NodeAndCallback{
				Node:          groupNode,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					var result map[string]interface{}
					json.Unmarshal([]byte(msg.GetData()), &result)
					// Verify result content, allowing for flexible key order or type conversions if needed
					// Since map iteration order is random, we check specific keys
					// Note: JSON unmarshal numbers to float64 by default

					// Check 'a'
					if val, ok := result["a"]; ok {
						assert.Equal(t, 1.0, val)
					} else {
						t.Errorf("Key 'a' not found in result: %v", result)
					}

					// Check 'b'
					if val, ok := result["b"]; ok {
						assert.Equal(t, 2.0, val)
					} else {
						t.Errorf("Key 'b' not found in result: %v", result)
					}

					// Check 'c'
					if val, ok := result["c"]; ok {
						assert.Equal(t, 3.0, val)
					} else {
						t.Errorf("Key 'c' not found in result: %v", result)
					}

					// Check 'node3' (from text node)
					if val, ok := result["node3"]; ok {
						assert.Equal(t, "not json", val)
					} else {
						t.Error("Key 'node3' not found in result")
					}
				},
			}
			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		})

		// Case 2: MergeToMap = false
		t.Run("False", func(t *testing.T) {
			groupNode, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"matchNum":   2,
				"nodeIds":    "node1,node2",
				"mergeToMap": false,
			}, Registry)
			assert.Nil(t, err)

			msgList := []test.Msg{
				{
					MsgType:    "ACTIVITY_EVENT1",
					Data:       "{}",
					AfterSleep: time.Millisecond * 200,
				},
			}

			nodeCallback := test.NodeAndCallback{
				Node:          groupNode,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					var list []interface{}
					err = json.Unmarshal([]byte(msg.GetData()), &list)
					assert.Nil(t, err)
					assert.Equal(t, 2, len(list))
				},
			}
			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		})
	})
}

// TestGroupActionConcurrencySafety Tests the concurrency security of GroupActionNode
func TestGroupActionConcurrencySafety(t *testing.T) {
	t.Run("Concurrent Match Count Race Condition", func(t *testing.T) {
		// Register a function for testing
		action.Functions.Register("testConcurrentSuccess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 1) // Simulated processing time
			ctx.TellSuccess(msg)
		})

		action.Functions.Register("testConcurrentFailure", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 2) // Simulated processing time
			ctx.TellFailure(msg, errors.New("test failure"))
		})

		// Create a GroupActionNode, requiring matching two Successes
		node, err := test.CreateAndInitNode("groupAction", types.Configuration{
			"matchRelationType": types.Success,
			"matchNum":          2,
			"nodeIds":           "success1,success2,failure1,failure2",
		}, Registry)
		assert.Nil(t, err)

		// Create child nodes
		successNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, action.Registry)
		successNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, action.Registry)
		failureNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, action.Registry)
		failureNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, action.Registry)

		childrenNodes := map[string]types.Node{
			"success1": successNode1,
			"success2": successNode2,
			"failure1": failureNode1,
			"failure2": failureNode2,
		}

		// Multiple concurrent tests were conducted
		iterations := 100
		var successCount, failureCount int32

		for i := 0; i < iterations; i++ {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("testIteration", str.ToString(i))

			msgList := []test.Msg{{
				MetaData:   metaData,
				MsgType:    "TEST_CONCURRENT",
				Data:       `{"test":"concurrency"}`,
				AfterSleep: time.Millisecond * 50,
			}}

			nodeCallback := test.NodeAndCallback{
				Node:          node,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if relationType == types.Success {
						// There are 2 Success nodes, which should satisfy the matchNum=2 condition
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&failureCount, 1)
					}
				},
			}

			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		}

		// Wait for all tests to be completed
		time.Sleep(time.Millisecond * 200)

		// Verification result: All should be Success because there are 2 Success nodes satisfying matchNum=2
		//t.Logf("Concurrent test results: Success = %d, Failure = %d, Total = %d",
		//	atomic.LoadInt32(&successCount), atomic.LoadInt32(&failureCount), iterations)

		assert.Equal(t, int32(iterations), atomic.LoadInt32(&successCount), "所有测试应该返回Success")
		assert.Equal(t, int32(0), atomic.LoadInt32(&failureCount), "不应该有Failure结果")
	})

	t.Run("Concurrent Insufficient Match Race Condition", func(t *testing.T) {
		// Create a GroupActionNode, requiring matching 3 Success Nodes (but only 2 Success Nodes)
		node, err := test.CreateAndInitNode("groupAction", types.Configuration{
			"matchRelationType": types.Success,
			"matchNum":          3,                            // Requires 3 Successes
			"nodeIds":           "success1,success2,failure1", // Only 2 successes
		}, Registry)
		assert.Nil(t, err)

		// Create child nodes
		successNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, action.Registry)
		successNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, action.Registry)
		failureNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, action.Registry)

		childrenNodes := map[string]types.Node{
			"success1": successNode1,
			"success2": successNode2,
			"failure1": failureNode1,
		}

		// Multiple concurrent tests were conducted
		iterations := 100
		var successCount, failureCount int32

		for i := 0; i < iterations; i++ {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("testIteration", str.ToString(i))

			msgList := []test.Msg{{
				MetaData:   metaData,
				MsgType:    "TEST_CONCURRENT",
				Data:       `{"test":"insufficient_match"}`,
				AfterSleep: time.Millisecond * 50,
			}}

			nodeCallback := test.NodeAndCallback{
				Node:          node,
				MsgList:       msgList,
				ChildrenNodes: childrenNodes,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if relationType == types.Success {
						atomic.AddInt32(&successCount, 1)
					} else {
						// There are only 2 Success nodes, and matchNum=3 is not met, so Failure should be returned
						atomic.AddInt32(&failureCount, 1)
					}
				},
			}

			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		}

		// Wait for all tests to be completed
		time.Sleep(time.Millisecond * 200)

		// Verification result: It should all be Failures, because only 2 Successes do not satisfy matchNum=3
		//t.Logf("Insufficient match test results: Success = %d, Failure = %d, Total = %d",
		//	atomic.LoadInt32(&successCount), atomic.LoadInt32(&failureCount), iterations)

		assert.Equal(t, int32(0), atomic.LoadInt32(&successCount), "不应该有Success结果")
		assert.Equal(t, int32(iterations), atomic.LoadInt32(&failureCount), "所有测试应该返回Failure")
	})
}

// Fixes the TestGroupActionNodeTimeoutRaceCondition condition for testing timeouts
func TestGroupActionNodeTimeoutRaceCondition(t *testing.T) {
	t.Skip("Skip the complex timeout test for now and use the simplified version")
}

// TestGroupActionNodeTimeoutSimple simplifies timeout testing
func TestGroupActionNodeTimeoutSimple(t *testing.T) {
	// Get the initial goroutine quantity
	initialGoroutines := runtime.NumGoroutine()

	// Create a simple timeout test
	action.Functions.Register("timeoutTestFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Simulates slow processing, but checks context cancellation
		for i := 0; i < 30; i++ { // 3 seconds total time
			select {
			case <-ctx.GetContext().Done():
				// context cancels and returns directly
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		ctx.TellSuccess(msg)
	})

	node := &GroupActionNode{}
	err := node.Init(types.NewConfig(), map[string]interface{}{
		"matchRelationType": types.Success,
		"matchNum":          1,
		"nodeIds":           []string{"test1"},
		"timeout":           1, // Timeout of 1 second
	})
	assert.Nil(t, err)

	// Create a simple test context
	testCtx := test.NewExtendedTestRuleContextWithChannel()
	// Set up the node processor to simulate timeout behavior
	testCtx.SetNodeHandler("test1", func(msg types.RuleMsg) (string, error) {
		// Simulates slow processing, but checks context cancellation
		for i := 0; i < 30; i++ { // 3 seconds total time
			select {
			case <-testCtx.GetContext().Done():
				// context cancels and returns directly
				return "", testCtx.GetContext().Err()
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		return types.Success, nil
	})

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{}`)

	// Perform the test
	start := time.Now()
	node.OnMsg(testCtx, msg)
	duration := time.Since(start)

	// Verification timeouts work as expected
	assert.True(t, duration >= 1*time.Second && duration < 1500*time.Millisecond,
		"Expected timeout around 1 second, got %v", duration)

	// Verify the results received
	select {
	case result := <-testCtx.GetResultsChannel():
		assert.Equal(t, "Failure", result.RelationType, "Should receive Failure on timeout")
		assert.NotNil(t, result.Err, "Should receive timeout error")
		t.Logf("Received the expected timeout result: %s, err: %v", result.RelationType, result.Err)
	case <-time.After(100 * time.Millisecond):
		t.Error("Should receive a result")
	}

	// Wait for all goroutines to complete
	time.Sleep(2 * time.Second)

	// Forced GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	goroutineIncrease := finalGoroutines - initialGoroutines

	assert.True(t, goroutineIncrease <= 3,
		"Expected goroutine increase <= 3, got %d (from %d to %d)",
		goroutineIncrease, initialGoroutines, finalGoroutines)
}

// createSimpleTestContext creates a simple test context, now using ExtendedTestRuleContext
// Maintain backward compatibility
func createSimpleTestContext(onEnd func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string)) *test.ExtendedTestRuleContext {
	ctx := test.NewExtendedTestRuleContextWithChannel()
	// Set up the node processor to simulate timeout behavior
	ctx.SetNodeHandler("timeout", func(msg types.RuleMsg) (string, error) {
		return "timeout", context.DeadlineExceeded
	})
	return ctx
}

// TestResult now uses the definition in the test package
