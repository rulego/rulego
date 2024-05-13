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

func TestMsgTypeSwitchNode(t *testing.T) {
	var targetNodeType = "msgTypeSwitch"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &MsgTypeSwitchNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("InitNodeDefault", func(t *testing.T) {
		config := types.NewConfig()
		node := MsgTypeSwitchNode{}
		err := node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, KeyDefaultRelationType)

		config.Properties.PutValue(KeyOtherRelationTypeName, "Default")
		err = node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, "Default")
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)

		var nodeList = []types.Node{node1}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT1",
					Data:       "AA",
					AfterSleep: time.Millisecond * 200,
				},
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT2",
					Data:       "{\"temperature\":60}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				assert.Equal(t, msg.Type, relationType)
			})
		}
	})
}
