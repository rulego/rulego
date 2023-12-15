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

package filter

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestGroupFilterNode(t *testing.T) {
	var targetNodeType = "groupFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &GroupFilterNode{}, types.Configuration{
			"allMatches": false,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"allMatches": true,
		}, types.Configuration{
			"allMatches": true,
		}, Registry)
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
