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

package transform

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestJsTransformNode(t *testing.T) {
	var targetNodeType = "jsTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &JsTransformNode{}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "metadata['test']='addFromJs';msgType='MSG_TYPE_MODIFY_BY_JS';return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return true`,
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
					Data:       "{\"name\":\"lala\"}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if node.(*JsTransformNode).Config.JsScript == `return true` {
					assert.Equal(t, JsTransformReturnFormatErr.Error(), err2.Error())
				} else if node.(*JsTransformNode).Config.JsScript == `return a` {
					assert.NotNil(t, err2)
				} else {
					assert.Equal(t, "test", msg.Metadata.GetValue("productType"))
					assert.Equal(t, "addFromJs", msg.Metadata.GetValue("test"))
					assert.Equal(t, "MSG_TYPE_MODIFY_BY_JS", msg.Type)
				}

			})
		}
	})
	t.Run("OnMsgError", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "msg['add']=5+msg['test'];return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node1, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Failure, relationType)
		})
	})
}
