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

func TestJsSwitchNode(t *testing.T) {
	var targetNodeType = "jsSwitch"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &JsSwitchNode{}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("InitNodeDefault", func(t *testing.T) {
		config := types.NewConfig()
		node := JsSwitchNode{}
		err := node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, KeyDefaultRelationType)

		config.Properties.PutValue(KeyOtherRelationTypeName, "Default")
		err = node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, "Default")
	})
	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "return ['one','two'];",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return 1`,
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return a`,
		}, Registry)

		var nodeList = []types.Node{node1, node2, node3}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "AA",
					AfterSleep: time.Millisecond * 200,
				},
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature\":60}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if node.(*JsSwitchNode).Config.JsScript == `return 1` {
					assert.Equal(t, JsSwitchReturnFormatErr.Error(), err2.Error())
				} else if node.(*JsSwitchNode).Config.JsScript == `return a` {
					assert.NotNil(t, err2)
				} else {
					assert.True(t, relationType == "one" || relationType == "two")
				}

			})
		}
	})
}
