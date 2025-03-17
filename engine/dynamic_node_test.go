/*
 * Copyright 2024 The RuleGo Authors.
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

package engine

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestDynamicNodeTest(t *testing.T) {

	var dsl = `
	 {
	 "ruleChain": {
	   "id": "fahrenheit",
	   "name": "华氏温度转换",
	   "debugMode": false,
	   "root": false,
	   "additionalInfo": {
	     "layoutX": 720,
	     "layoutY": 260,
	     "relationTypes":["Success","Failure"],
         "description":"this is a description",
	     "inputSchema": {
	       "type": "object",
	       "properties": {
	         "appKey": {
	           "type": "string",
	           "order": 1
	         },
	         "age": {
	           "type": "number",
               "order": 2
	         }
	       },
	       "required": ["appKey"]
	     }
	
	   }
	 },
	 "metadata": {
	   "firstNodeIndex": 0,
	   "nodes": [
	     {
	       "id": "s2",
	       "type": "jsTransform",
	       "name": "摄氏温度转华氏温度",
	       "debugMode": true,
	       "configuration": {
	         "jsScript": "var newMsg={'appKey':'${vars.appKey}','temperature': msg.temperature*(9/5)+32};\n return {'msg':newMsg,'metadata':metadata,'msgType':msgType};"
	       }
	     }
	   ],
	   "connections": [
	     {
	     }
	   ]
	 }
	}
`
	componentType := "fahrenheit"
	t.Run("RegistryDynamicNode", func(t *testing.T) {
		dynamicNode := NewDynamicNode(componentType, dsl)
		err := Registry.Register(dynamicNode)
		assert.Nil(t, err)
		node, ok := Registry.GetComponents()[componentType]
		assert.True(t, ok)
		assert.Equal(t, node.Type(), componentType)

		componentType = componentType + "02"
		//error
		dynamicNode = NewDynamicNode(componentType, "")

		err = Registry.Register(dynamicNode)
		assert.Nil(t, err)
		node, ok = Registry.GetComponents()[componentType]
		err = node.Init(NewConfig(), types.Configuration{
			"appKey": "123456",
			types.NodeConfigurationKeyChainCtx: &RuleChainCtx{
				Id: types.RuleNodeId{Id: "test_5001"},
			},
			types.NodeConfigurationKeySelfDefinition: types.RuleNode{
				Id: "s1",
			},
		})
		assert.NotNil(t, err)
	})

	t.Run("RunDynamicNode", func(t *testing.T) {
		componentType = componentType + "03"
		dynamicNode := NewDynamicNode(componentType, dsl)
		err := Registry.Register(dynamicNode)
		assert.Nil(t, err)
		node, ok := Registry.GetComponents()[componentType]
		assert.True(t, ok)
		assert.Equal(t, node.Type(), componentType)

		node1 := node.New()
		err = node1.Init(NewConfig(), types.Configuration{
			"appKey": "123456",
			types.NodeConfigurationKeyChainCtx: &RuleChainCtx{
				Id: types.RuleNodeId{Id: "test_5001"},
			},
			types.NodeConfigurationKeySelfDefinition: types.RuleNode{
				Id: "s1",
			},
		})
		assert.Nil(t, err)

		node2 := node.New()
		err = node2.Init(NewConfig(), types.Configuration{
			"appKey": "789012",
			types.NodeConfigurationKeyChainCtx: &RuleChainCtx{
				Id: types.RuleNodeId{Id: "test_5001"},
				SelfDefinition: &types.RuleChain{
					RuleChain: types.RuleChainBaseInfo{
						Configuration: types.Configuration{
							types.Vars: map[string]interface{}{
								"aa": "test",
							},
						},
					},
				},
			},
			types.NodeConfigurationKeySelfDefinition: types.RuleNode{
				Id: "s1",
			},
		})
		assert.Nil(t, err)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			},
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, `{"appKey":"123456","temperature":140}`, msg.Data)
					assert.Equal(t, types.Success, relationType)
				},
			}, {
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, `{"appKey":"789012","temperature":140}`, msg.Data)
					assert.Equal(t, types.Success, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		node1.Destroy()
		node2.Destroy()
	})

}
