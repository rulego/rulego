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
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestForNode(t *testing.T) {
	var targetNodeType = "for"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ForNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"range": "msg.items ",
			"do":    "s1",
		}, types.Configuration{
			"range": "msg.items",
			"do":    "s1",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"range": "1..3",
			"do":    "s3",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		data1 := "{\"humidity\":90,\"temperature\":41}"
		data2 := "{\"humidity\":70,\"temperature\":66}"
		//测试函数
		Functions.Register("groupActionTest1", func(ctx types.RuleContext, msg types.RuleMsg) {
			index := msg.Metadata.GetValue(KeyLoopIndex)
			msg.Metadata.PutValue("add"+index, "value"+index)
			rangeType := msg.Metadata.GetValue("rangeType")
			mode := msg.Metadata.GetValue("mode")
			if rangeType == "rangeNum" {
				item := msg.Metadata.GetValue(KeyLoopItem)
				i, _ := strconv.Atoi(index)
				itemVal, _ := strconv.Atoi(item)
				assert.Equal(t, i, itemVal-1)
			} else if rangeType == "rangeObject" {
				key := msg.Metadata.GetValue(KeyLoopKey)
				assert.True(t, key != "")
			} else if mode == "2" {
				if index == "0" && rangeType == "rangeNum" {
					assert.Equal(t, data1, msg.Data)
				} else if index == "1" {
					assert.Equal(t, "0", msg.Data)
				}
			} else {
				if index == "0" {
					assert.Equal(t, data1, msg.Data)
				} else if index == "1" {
					assert.Equal(t, data2, msg.Data)
				}
			}
			if mode == "2" {
				msg.Data = index
			}
			ctx.TellSuccess(msg)
		})
		childrenNode1, err := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "groupActionTest1",
		}, Registry)
		childrenNodes := map[string]types.Node{
			"node1": childrenNode1,
		}

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "",
		}, Registry)
		assert.NotNil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "notfound",
		}, Registry)
		assert.Nil(t, err)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "chain:notfound",
		}, Registry)
		assert.Nil(t, err)

		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "node1",
		}, Registry)
		assert.Nil(t, err)
		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "1..5",
			"do":    "node1",
		}, Registry)
		assert.Nil(t, err)
		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "",
			"do":    "node1",
		}, Registry)
		assert.Nil(t, err)

		node7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "node1",
		}, Registry)
		node8, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "node1",
			"mode":  MergeValues,
		}, Registry)
		node9, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "msg.items",
			"do":    "node1",
			"mode":  ReplaceValues,
		}, Registry)

		node10, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "1..3",
			"do":    "node1",
			"mode":  ReplaceValues,
		}, Registry)
		node11, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "",
			"do":    "node1",
			"mode":  ReplaceValues,
		}, Registry)

		node12, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"range": "",
			"do":    "node1",
			"mode":  AsyncProcess,
		}, Registry)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		msgList := []test.Msg{
			{
				MetaData: types.BuildMetadata(map[string]string{
					//"rangeType": "",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
				AfterSleep: time.Millisecond * 20,
			},
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, fmt.Sprintf("node id=%s not found", "notfound"), err.Error())
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, fmt.Sprintf("ruleChain id=%s not found", "notfound"), err.Error())
				},
			},
			{
				Node:    node4,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					//fmt.Println(msg)
				},
			},
			{
				Node: node5,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"rangeType": "rangeNum",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					//fmt.Println(msg)
				},
			},
			{
				Node: node6,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"rangeType": "rangeObject",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					//fmt.Println(msg)
				},
			},
			{
				Node: node7,
				MsgList: []test.Msg{
					{
						MetaData:   types.BuildMetadata(map[string]string{}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "{\"humidity\":90,\"temperature\":41,\"items\":"+"["+data1+","+data2+"]"+"}", msg.Data)
				},
			},
			{
				Node: node8,
				MsgList: []test.Msg{
					{
						MetaData:   types.BuildMetadata(map[string]string{}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "["+data1+","+data2+"]", msg.Data)
				},
			},
			{
				Node: node9,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"mode": "2",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "1", msg.Data)
				},
			},
			{
				Node: node10,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"mode": "2",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "2", msg.Data)
				},
			},
			{
				Node: node11,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"mode":      "2",
							"rangeType": "rangeObject",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "2", msg.Data)
				},
			},
			{
				Node: node12,
				MsgList: []test.Msg{
					{
						MetaData: types.BuildMetadata(map[string]string{
							"mode":      "3",
							"rangeType": "rangeObject",
						}),
						MsgType:    "ACTIVITY_EVENT1",
						Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
						AfterSleep: time.Millisecond * 20,
					},
				},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "{\"humidity\":90,\"temperature\":41,\"items\":"+"["+data1+","+data2+"]"+"}", msg.Data)
				},
			},
		}
		for _, item := range nodeList {

			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, childrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)

	})
}

// TestForNodeMetadataAccuracy 测试for节点metadata的准确性
func TestForNodeMetadataAccuracy(t *testing.T) {
	// 创建for节点
	node := &ForNode{}
	config := types.NewConfig()
	nodeConfig := types.Configuration{
		"range": "msg.items",
		"do":    "testNode", // 使用节点ID而不是表达式
		"mode":  0,          // DoNotProcess模式
	}
	err := node.Init(config, nodeConfig)
	assert.Nil(t, err)

	// 创建测试消息
	testData := `{"items": ["item1", "item2", "item3"]}`
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, types.NewMetadata(), testData)
	msg.Metadata.PutValue("original_key", "original_value")
	msg.Metadata.PutValue("test_counter", "0")

	// 用于收集所有回调结果
	var results []types.RuleMsg
	var mu sync.Mutex

	// 创建一个简单的子节点用于测试
	testNode := &CommentNode{}
	testNodeConfig := types.Configuration{"comment": "test"}
	testNode.Init(config, testNodeConfig)

	// 创建带子节点的规则上下文
	childrenNodes := map[string]types.Node{
		"testNode": testNode,
	}
	ctx := test.NewRuleContextFull(config, node, childrenNodes, func(msg types.RuleMsg, relationType string, err error) {
		mu.Lock()
		defer mu.Unlock()
		results = append(results, msg)

		// 验证metadata是否保持完整
		metadata := msg.Metadata.Values()
		assert.Equal(t, "original_value", metadata["original_key"])
		assert.Equal(t, "0", metadata["test_counter"])

		//t.Logf("Callback %d - RelationType: %s, Data: %s", len(results), relationType, msg.Data)
		//t.Logf("Callback %d - Metadata: %v", len(results), metadata)
	})

	// 执行节点
	node.OnMsg(ctx, msg)

	// 等待执行完成
	time.Sleep(time.Millisecond * 200)

	// 验证结果
	mu.Lock()
	resultCount := len(results)
	finalMsg := results[0] // for节点在DoNotProcess模式下只返回一个最终结果
	mu.Unlock()

	// 在DoNotProcess模式下，for节点处理完所有元素后返回原始消息
	assert.Equal(t, 1, resultCount, "Expected 1 final result in DoNotProcess mode")
	assert.Equal(t, testData, finalMsg.Data, "Expected original data to be preserved")

	// 验证原始metadata保持完整
	finalMetadata := finalMsg.Metadata.Values()
	assert.Equal(t, "original_value", finalMetadata["original_key"])
	assert.Equal(t, "0", finalMetadata["test_counter"])

	//t.Logf("TestForNodeMetadataAccuracy completed - verified metadata accuracy in for node")
}

// TestForNodeMetadataWithMergeMode 测试for节点在合并模式下的metadata准确性
func TestForNodeMetadataWithMergeMode(t *testing.T) {
	// 创建for节点
	node := &ForNode{}
	config := types.NewConfig()
	nodeConfig := types.Configuration{
		"range": "msg.items",
		"do":    "transformNode",
		"mode":  1, // MergeValues模式
	}
	err := node.Init(config, nodeConfig)
	assert.Nil(t, err)

	// 创建测试消息
	testData := `{"items": ["item1", "item2", "item3"]}`
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, types.NewMetadata(), testData)
	msg.Metadata.PutValue("original_key", "original_value")
	msg.Metadata.PutValue("process_count", "0")

	// 用于收集所有回调结果
	var results []types.RuleMsg
	var mu sync.Mutex

	// 创建一个转换节点用于测试，它会修改数据并添加metadata
	transformNode := &CommentNode{}
	transformNodeConfig := types.Configuration{"comment": "transform"}
	transformNode.Init(config, transformNodeConfig)

	// 创建带子节点的规则上下文
	childrenNodes := map[string]types.Node{
		"transformNode": transformNode,
	}
	ctx := test.NewRuleContextFull(config, node, childrenNodes, func(msg types.RuleMsg, relationType string, err error) {
		mu.Lock()
		defer mu.Unlock()
		results = append(results, msg)

		// 验证metadata是否保持完整
		metadata := msg.Metadata.Values()
		assert.Equal(t, "original_value", metadata["original_key"])
		assert.Equal(t, "0", metadata["process_count"])

		// 验证for循环的最终状态metadata（应该是最后一次迭代的值）
		assert.Equal(t, "2", metadata["_loopIndex"])    // 最后一个索引是2
		assert.Equal(t, "item3", metadata["_loopItem"]) // 最后一个项目是item3

		t.Logf("Final result - RelationType: %s, Data: %s", relationType, msg.Data)
		t.Logf("Final metadata: %v", metadata)
	})

	// 执行节点
	node.OnMsg(ctx, msg)

	// 等待执行完成
	time.Sleep(time.Millisecond * 200)

	// 验证结果
	mu.Lock()
	resultCount := len(results)
	finalMsg := results[0]
	mu.Unlock()

	// 在MergeValues模式下，for节点处理完所有元素后返回合并结果
	assert.Equal(t, 1, resultCount, "Expected 1 final result in MergeValues mode")

	// 验证原始metadata保持完整
	finalMetadata := finalMsg.Metadata.Values()
	assert.Equal(t, "original_value", finalMetadata["original_key"])
	assert.Equal(t, "0", finalMetadata["process_count"])

	// 验证for循环metadata反映了最后一次迭代的状态
	assert.Equal(t, "2", finalMetadata["_loopIndex"])
	assert.Equal(t, "item3", finalMetadata["_loopItem"])

	t.Logf("TestForNodeMetadataWithMergeMode completed - verified metadata accuracy with merge mode")
}
