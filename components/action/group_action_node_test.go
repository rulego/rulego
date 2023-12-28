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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"testing"
	"time"
)

func TestGroupFilterNode(t *testing.T) {
	var targetNodeType = "groupAction"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &GroupActionNode{}, types.Configuration{
			"allMatches": true,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"allMatches": false,
		}, types.Configuration{
			"allMatches": false,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{
			"allMatches": true,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {

		//测试函数
		Functions.Register("groupActionTest1", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test1", time.Now().String())
			msg.Data = `{"addValue":"addFromTest1"}`
			ctx.TellSuccess(msg)
		})

		Functions.Register("groupActionTest2", func(ctx types.RuleContext, msg types.RuleMsg) {
			msg.Metadata.PutValue("test2", time.Now().String())
			msg.Data = `{"addValue":"addFromTest2"}`
			ctx.TellSuccess(msg)
		})

		Functions.Register("groupActionTestFailure", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(time.Millisecond * 100)
			ctx.TellFailure(msg, errors.New("test error"))
		})

		groupFilterNode1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": false,
			"nodeIds":    "node1,node2,node3,noFoundId",
			"timeout":    10,
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": true,
			"nodeIds":    "node1,node2",
		}, Registry)

		assert.Nil(t, err)

		groupFilterNode3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": false,
			"nodeIds":    "node1,node2,node3,noFoundId",
		}, Registry)

		groupFilterNode4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"nodeIds": "node1,node2",
		}, Registry)

		groupFilterNode5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": true,
			"nodeIds":    "node1,node2,node3,noFoundId",
		}, Registry)

		groupFilterNode6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": true,
			"nodeIds":    "",
		}, Registry)

		groupFilterNode7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"allMatches": false,
			"nodeIds":    "node3,node4",
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
					_ = json.Unmarshal([]byte(msg.Data), &result)
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
