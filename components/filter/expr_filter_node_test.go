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

func TestExprFilterNode(t *testing.T) {
	var targetNodeType = "exprFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExprFilterNode{}, types.Configuration{
			"expr": "",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50",
		}, types.Configuration{
			"expr": "msg.temperature > 50",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"expr": "",
		}, types.Configuration{
			"expr": "",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50 && msg.humidity > 20",
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": `upper(msg.name)=='AA'`,
		}, Registry)

		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "temperature==nil?true:false",
		}, Registry)

		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg)=='AA'",
		}, Registry)
		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])=='A'",
		}, Registry)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msg1 := test.Msg{
			MetaData:   metaData,
			DataType:   types.TEXT,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "AA",
			AfterSleep: time.Millisecond * 200,
		}
		msg2 := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg3 := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"bb\",\"temperature\":40}",
			AfterSleep: time.Millisecond * 200,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node1,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},

			{
				Node:    node3,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node3,
				MsgList: []test.Msg{msg3},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.False, relationType)
				},
			},
			{
				Node:    node4,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node5,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node6,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)
	})
}
