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
	"github.com/rulego/rulego/components/filter"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestSwitchNode(t *testing.T) {
	var targetNodeType = "switch"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &SwitchNode{}, types.Configuration{}, filter.Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"cases": []Case{
				{
					Case: "msg.temperature > 50",
					Then: "case1",
				},
				{
					Case: "msg.name == \"test1\"",
					Then: "case2",
				},
			},
		}, types.Configuration{
			"cases": []Case{
				{
					Case: "msg.temperature > 50",
					Then: "case1",
				},
				{
					Case: "msg.name == \"test1\"",
					Then: "case2",
				},
			},
		}, filter.Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"cases": []map[string]string{
				{"case": "msg.temperature > 50", "then": "case1"},
				{"case": "metadata.productType == \"test\"", "then": "case2"},
				{"case": "msg.temperature > 30 && msg.humidity > 20", "then": "case3"},
				{"case": "msg.temperature2!=nil &&msg.temperature2 > 50", "then": "case4"},
			},
		}, filter.Registry)
		assert.Nil(t, err)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"cases": []map[string]string{
				{"then": "case4", "case": "msg.temperature2 > 50"},
			},
		}, filter.Registry)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msg1 := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType":  "test2",
				"relationType": "case1",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg2 := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType":  "test",
				"relationType": "case2",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":20,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg3 := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType":  "test2",
				"relationType": "case3",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":40,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg4 := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType":  "test2",
				"relationType": types.DefaultRelationType,
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":10,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg1, msg2, msg3, msg4},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, msg.Metadata.GetValue("relationType"), relationType)
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
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
