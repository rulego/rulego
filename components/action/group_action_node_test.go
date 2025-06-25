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

package action

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

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

		//测试函数
		Functions.Register("groupActionTest1", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test1", time.Now().String())
			msg.SetData(`{"addValue":"addFromTest1"}`)
			ctx.TellSuccess(msg)
		})

		Functions.Register("groupActionTest2", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test2", time.Now().String())
			msg.SetData(`{"addValue":"addFromTest2"}`)
			ctx.TellSuccess(msg)
		})

		Functions.Register("groupActionTestFailure", func(ctx types.RuleContext, msg types.RuleMsg) {
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
		}, Registry)

		node2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTest2",
		}, Registry)
		node3, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTestFailure",
		}, Registry)
		node4, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "notFound",
		}, Registry)

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
}

// TestGroupActionConcurrencySafety 测试 GroupActionNode 的并发安全性
func TestGroupActionConcurrencySafety(t *testing.T) {
	t.Run("Concurrent Match Count Race Condition", func(t *testing.T) {
		// 注册测试用的函数
		Functions.Register("testConcurrentSuccess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 1) // 模拟处理时间
			ctx.TellSuccess(msg)
		})

		Functions.Register("testConcurrentFailure", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 2) // 模拟处理时间
			ctx.TellFailure(msg, errors.New("test failure"))
		})

		// 创建 GroupActionNode，要求匹配2个Success
		node, err := test.CreateAndInitNode("groupAction", types.Configuration{
			"matchRelationType": types.Success,
			"matchNum":          2,
			"nodeIds":           "success1,success2,failure1,failure2",
		}, Registry)
		assert.Nil(t, err)

		// 创建子节点
		successNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, Registry)
		successNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, Registry)
		failureNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, Registry)
		failureNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, Registry)

		childrenNodes := map[string]types.Node{
			"success1": successNode1,
			"success2": successNode2,
			"failure1": failureNode1,
			"failure2": failureNode2,
		}

		// 进行多次并发测试
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
						// 有2个Success节点，应该满足matchNum=2的条件
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&failureCount, 1)
					}
				},
			}

			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		}

		// 等待所有测试完成
		time.Sleep(time.Millisecond * 200)

		// 验证结果：应该都是Success，因为有2个Success节点满足matchNum=2
		//t.Logf("并发测试结果: Success=%d, Failure=%d, Total=%d",
		//	atomic.LoadInt32(&successCount), atomic.LoadInt32(&failureCount), iterations)

		assert.Equal(t, int32(iterations), atomic.LoadInt32(&successCount), "所有测试应该返回Success")
		assert.Equal(t, int32(0), atomic.LoadInt32(&failureCount), "不应该有Failure结果")
	})

	t.Run("Concurrent Insufficient Match Race Condition", func(t *testing.T) {
		// 创建 GroupActionNode，要求匹配3个Success（但只有2个Success节点）
		node, err := test.CreateAndInitNode("groupAction", types.Configuration{
			"matchRelationType": types.Success,
			"matchNum":          3,                            // 要求3个Success
			"nodeIds":           "success1,success2,failure1", // 只有2个Success
		}, Registry)
		assert.Nil(t, err)

		// 创建子节点
		successNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, Registry)
		successNode2, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentSuccess",
		}, Registry)
		failureNode1, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "testConcurrentFailure",
		}, Registry)

		childrenNodes := map[string]types.Node{
			"success1": successNode1,
			"success2": successNode2,
			"failure1": failureNode1,
		}

		// 进行多次并发测试
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
						// 只有2个Success节点，不满足matchNum=3，应该返回Failure
						atomic.AddInt32(&failureCount, 1)
					}
				},
			}

			test.NodeOnMsgWithChildren(t, nodeCallback.Node, nodeCallback.MsgList, nodeCallback.ChildrenNodes, nodeCallback.Callback)
		}

		// 等待所有测试完成
		time.Sleep(time.Millisecond * 200)

		// 验证结果：应该都是Failure，因为只有2个Success不满足matchNum=3
		//t.Logf("不足匹配测试结果: Success=%d, Failure=%d, Total=%d",
		//	atomic.LoadInt32(&successCount), atomic.LoadInt32(&failureCount), iterations)

		assert.Equal(t, int32(0), atomic.LoadInt32(&successCount), "不应该有Success结果")
		assert.Equal(t, int32(iterations), atomic.LoadInt32(&failureCount), "所有测试应该返回Failure")
	})
}
