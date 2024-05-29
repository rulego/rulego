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
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strings"
	"testing"
)

func TestChainCtx(t *testing.T) {

	ruleChainDef := types.RuleChain{}

	t.Run("New", func(t *testing.T) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				assert.Equal(t, "not support this func", fmt.Sprintf("%s", e))
			}
		}()
		ruleChainDef.Metadata.RuleChainConnections = []types.RuleChainConnection{
			{
				FromId: "s1",
				ToId:   "s2",
				Type:   types.True,
			},
		}
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		ctx.New()
	})

	t.Run("Init", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		newRuleChainDef := types.RuleChain{}
		err := ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Nil(t, err)

		newRuleChainDef = types.RuleChain{}
		ruleNode := types.RuleNode{Type: "notFound"}
		newRuleChainDef.Metadata.Nodes = append(newRuleChainDef.Metadata.Nodes, &ruleNode)
		err = ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Equal(t, "component not found. componentType=notFound", err.Error())
	})

	t.Run("ReloadChildNotFound", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), &ruleChainDef)
		newRuleChainDef := types.RuleChain{}
		err := ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Nil(t, err)
		err = ctx.ReloadChild(types.RuleNodeId{}, []byte(""))
		assert.Nil(t, err)
	})

	t.Run("ChildParser", func(t *testing.T) {
		var ruleChainFile = `{
          "ruleChain": {
            "id": "test01",
            "name": "testRuleChain01",
            "debugMode": true,
            "root": true,
             "configuration": {
                 "vars": {
						"ip":"127.0.0.1"
					},
  				  "decryptSecrets": {
						"bb":"xx"
					}
                }
          },
           "metadata": {
            "firstNodeIndex": 0,
            "nodes": [
              {
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "groupFilter",
                "name": "分组",
                "debugMode": true,
                "configuration": {
                  "nodeIds": "${vars.ip}"
                }
              }
              
            ],
            "connections": [
            ]
          }
        }`
		config := NewConfig()
		jsonParser := JsonParser{}
		chainNode, err := jsonParser.DecodeRuleChain(config, []byte(ruleChainFile))
		assert.Nil(t, err)
		ruleChainCtx, _ := chainNode.(*RuleChainCtx)
		nodeDsl := []byte(`
 			{
                "id": "s1",
                "additionalInfo": {
                  "description": "",
                  "layoutX": 0,
                  "layoutY": 0
                },
                "type": "groupFilter",
                "name": "分组",
                "debugMode": true,
                "configuration": {
                  "nodeIds": "${vars.ip}"
                }
              }
`)
		nodeCtx := ruleChainCtx.nodes[types.RuleNodeId{Id: "s1"}]
		err = nodeCtx.ReloadSelf(nodeDsl)
		assert.Nil(t, err)
		var output = make(map[string]interface{})
		maps.Map2Struct(nodeCtx, output)
		nodeStr := str.ToString(output["Node"])
		assert.True(t, strings.Contains(nodeStr, "127.0.0.1"))
		ruleChainFile = strings.Replace(ruleChainFile, "127.0.0.1", "192.168.1.1", -1)
		err = ruleChainCtx.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, err)
		nodeCtx = ruleChainCtx.nodes[types.RuleNodeId{Id: "s1"}]
		maps.Map2Struct(nodeCtx, output)
		nodeStr = str.ToString(output["Node"])
		assert.True(t, strings.Contains(nodeStr, "192.168.1.1"))
	})

}
