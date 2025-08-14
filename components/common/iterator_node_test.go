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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestIteratorNode(t *testing.T) {
	
	Registry.Add(&IteratorNode{})

	var targetNodeType = "iterator"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &IteratorNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"fieldName": " items ",
			"jsScript":  " \nreturn true; ",
		}, types.Configuration{
			"fieldName": "items",
			"jsScript":  "return true;",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"fieldName": "",
			"jsScript":  "",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {

		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"fieldName": "items",
		}, Registry)
		assert.Nil(t, err)

		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"fieldName": "notFoundItems",
		}, Registry)
		assert.Nil(t, err)

		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "return metadata.dataType=='map'||index%2==0",
		}, Registry)
		assert.Nil(t, err)

		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "return aa",
		}, Registry)
		assert.Nil(t, err)

		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"fieldName": "items.value",
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		data1 := "{\"humidity\":90,\"temperature\":41}"
		data2 := "{\"humidity\":70,\"temperature\":66}"
		msgList := []test.Msg{
			{
				MetaData: types.BuildMetadata(map[string]string{
					"dataType": "map",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       data1,
				AfterSleep: time.Millisecond * 20,
			},
			{
				MetaData: types.BuildMetadata(map[string]string{
					"dataType": "array",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "[" + data1 + "," + data2 + "]",
				AfterSleep: time.Millisecond * 20,
			},
			{
				MetaData: types.BuildMetadata(map[string]string{
					"dataType": "mapAndArray",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "{\"humidity\":90,\"temperature\":41,\"items\":" + "[" + data1 + "," + data2 + "]" + "}",
				AfterSleep: time.Millisecond * 20,
			},
			{
				MetaData: types.BuildMetadata(map[string]string{
					"dataType": "string",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "aaaa",
				AfterSleep: time.Millisecond * 20,
			},
		}
		msgList2 := []test.Msg{
			{
				MetaData: types.BuildMetadata(map[string]string{
					"dataType": "mapAndArray",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "{\"humidity\":90,\"temperature\":41,\"items\":{" + "\"value\":[" + data1 + "," + data2 + "]" + "} }",
				AfterSleep: time.Millisecond * 20,
			},
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if msg.Metadata.GetValue("dataType") == "string" {
						assert.Equal(t, types.Failure, relationType)
						assert.Equal(t, "value is not array or {key:value} type", err.Error())
					} else if msg.Metadata.GetValue("dataType") == "map" {
						if types.True == relationType {
							assert.True(t, "41" == msg.GetData() || "90" == msg.GetData())
						} else if types.Success == relationType {
							assert.Equal(t, data1, msg.GetData())
						}
					} else if msg.Metadata.GetValue("dataType") == "array" {
						if types.True == relationType {
							assert.True(t, data1 == msg.GetData() || data2 == msg.GetData())
						} else if types.Success == relationType {
							assert.Equal(t, "["+data1+","+data2+"]", msg.GetData())
						}
					}

				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if msg.Metadata.GetValue("dataType") == "map" || msg.Metadata.GetValue("dataType") == "array" {
						assert.Equal(t, types.Failure, relationType)
						assert.Equal(t, "field=items not found", err.Error())
					} else {
						if types.True == relationType {
							assert.True(t, data1 == msg.GetData() || data2 == msg.GetData())
						} else if types.Success == relationType {
							assert.Equal(t, "{\"humidity\":90,\"temperature\":41,\"items\":"+"["+data1+","+data2+"]"+"}", msg.GetData())
						}
					}
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
					assert.Equal(t, "field=notFoundItems not found", err.Error())
				},
			},
			{
				Node:    node4,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if msg.Metadata.GetValue("dataType") == "array" {
						if types.True == relationType {
							assert.Equal(t, data1, msg.GetData())
						} else if types.False == relationType {
							assert.Equal(t, data2, msg.GetData())
						} else {
							assert.Equal(t, "["+data1+","+data2+"]", msg.GetData())
						}
					} else if msg.Metadata.GetValue("dataType") == "map" {
						if types.Success == relationType {
							assert.Equal(t, data1, msg.GetData())
						}
					} else if msg.Metadata.GetValue("dataType") == "mapAndArray" {
						if types.Success == relationType {
							assert.Equal(t, "{\"humidity\":90,\"temperature\":41,\"items\":"+"["+data1+","+data2+"]"+"}", msg.GetData())
						}
					}
				},
			},
			{
				Node:    node5,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node6,
				MsgList: msgList2,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					if types.True == relationType {
						assert.True(t, data1 == msg.GetData() || data2 == msg.GetData())
					} else if types.Success == relationType {
						assert.Equal(t, "{\"humidity\":90,\"temperature\":41,\"items\":{"+"\"value\":["+data1+","+data2+"]"+"} }", msg.GetData())
					}
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)

	})
}
