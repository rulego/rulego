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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"strconv"
	"testing"
	"time"
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
			if rangeType == "rangeNum" {
				item := msg.Metadata.GetValue(KeyLoopItem)
				i, _ := strconv.Atoi(index)
				itemVal, _ := strconv.Atoi(item)
				assert.Equal(t, i, itemVal-1)
			} else if rangeType == "rangeObject" {
				key := msg.Metadata.GetValue(KeyLoopKey)
				assert.True(t, key != "")
			} else {
				if index == "0" {
					assert.Equal(t, data1, msg.Data)
				} else if index == "1" {
					assert.Equal(t, data2, msg.Data)
				}
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
		}
		for _, item := range nodeList {

			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, childrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)

	})
}
