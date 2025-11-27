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

// TestInclusiveNode 验证包容分支组件的行为：多匹配、单匹配、默认分支与失败场景
func TestInclusiveNode(t *testing.T) {
	var targetNodeType = "inclusive"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &InclusiveNode{}, types.Configuration{}, filter.Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"cases": []Case{
				{Case: "msg.temperature > 50", Then: "case1"},
				{Case: "msg.name == \"test1\"", Then: "case2"},
			},
		}, types.Configuration{
			"cases": []Case{
				{Case: "msg.temperature > 50", Then: "case1"},
				{Case: "msg.name == \"test1\"", Then: "case2"},
			},
		}, filter.Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"cases": []map[string]string{
				{"case": "msg.temperature > 50", "then": "case1"},
				{"case": "metadata.productType == \"test\"", "then": "case2"},
				{"case": "msg.temperature > 30 && msg.humidity > 20", "then": "case3"},
			},
		}, filter.Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"cases": []map[string]string{
				{"then": "case4", "case": "msg.temperature2 > 50"},
			},
		}, filter.Registry)
		assert.Nil(t, err)

		msgMulti := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType": "test2",
				"expect":      "multi",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msgSingle := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType": "test",
				"expect":      "single",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":20,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msgDefault := test.Msg{
			MetaData: types.BuildMetadata(map[string]string{
				"productType": "test2",
				"expect":      "default",
			}),
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":10,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}

		seen := map[string]int{}

		var nodeList = []test.NodeAndCallback{
			{
				Node: node1,
				MsgList: []test.Msg{
					msgMulti, msgSingle, msgDefault,
				},
				Callback: func(m types.RuleMsg, relationType string, err error) {
					assert.Nil(t, err)
					expect := m.Metadata.GetValue("expect")
					switch expect {
					case "multi":
						if relationType == "case1" || relationType == "case3" {
							seen[relationType]++
						} else {
							assert.True(t, false)
						}
					case "single":
						assert.Equal(t, "case2", relationType)
					case "default":
						assert.Equal(t, types.DefaultRelationType, relationType)
					}
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msgMulti},
				Callback: func(m types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}

		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

		time.Sleep(time.Millisecond * 50)
		assert.Equal(t, 1, seen["case1"]) // 多匹配应路由到 case1
		assert.Equal(t, 1, seen["case3"]) // 多匹配应路由到 case3
	})
}
