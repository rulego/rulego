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

func TestFieldFilterNode(t *testing.T) {
	var targetNodeType = "fieldFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &FieldFilterNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("Check", func(t *testing.T) {
		node, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"checkAllKeys":  false,
			"dataNames":     "temperature,temp",
			"metadataNames": "productType,name,location",
		}, Registry)
		assert.False(t, node.(*FieldFilterNode).checkAllKeysData(nil))
		assert.False(t, node.(*FieldFilterNode).checkAllKeysData(map[string]interface{}{
			"temp": 40,
		}))
		assert.False(t, node.(*FieldFilterNode).checkAtLeastOneData(nil))
		assert.True(t, node.(*FieldFilterNode).checkAtLeastOneData(map[string]interface{}{
			"temp": 40,
		}))
	})

	t.Run("OnMsg1", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"checkAllKeys":  true,
			"dataNames":     "temperature",
			"metadataNames": "productType,name",
		}, Registry)
		assert.Nil(t, err)

		var nodeList = []types.Node{node1}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "A001")
			metaData.PutValue("name", "A001")

			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature\":40}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				assert.Equal(t, types.True, relationType)
			})
		}
	})

	t.Run("OnMsg2", func(t *testing.T) {

		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"checkAllKeys":  true,
			"dataNames":     "temperature",
			"metadataNames": "productType,name,location",
		}, Registry)

		var nodeList = []types.Node{node2}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "A001")
			metaData.PutValue("name", "A001")

			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature\":40}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				assert.Equal(t, types.False, relationType)
			})
		}
	})

	t.Run("OnMsg3", func(t *testing.T) {
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"checkAllKeys":  false,
			"dataNames":     "temperature",
			"metadataNames": "productType,name,location",
		}, Registry)

		var nodeList = []types.Node{node3}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "A001")
			metaData.PutValue("name", "A001")

			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature2\":40}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				assert.Equal(t, types.True, relationType)
			})
		}
	})

	t.Run("OnMsg4", func(t *testing.T) {
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"checkAllKeys":  false,
			"dataNames":     "temperature",
			"metadataNames": "productType,name,location",
		}, Registry)

		var nodeList = []types.Node{node3}

		for _, node := range nodeList {
			var msgList = []test.Msg{
				{
					MetaData:   types.BuildMetadata(make(map[string]string)),
					MsgType:    "ACTIVITY_EVENT",
					Data:       "aa",
					AfterSleep: time.Millisecond * 200,
				},
				{
					MetaData:   types.BuildMetadata(make(map[string]string)),
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature2\":40}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if msg.GetData() == "aa" {
					assert.Equal(t, types.Failure, relationType)
				} else {
					assert.Equal(t, types.False, relationType)
				}
			})
		}
	})
}
