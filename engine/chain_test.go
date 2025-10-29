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
	"os"
	"strings"
	"testing"
)

func TestChainCtx(t *testing.T) {

	ruleChainDef := types.RuleChain{}

	t.Run("New", func(t *testing.T) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				assert.Equal(t, "not support this method", fmt.Sprintf("%s", e))
			}
		}()
		ruleChainDef.Metadata.RuleChainConnections = []types.RuleChainConnection{
			{
				FromId: "s1",
				ToId:   "s2",
				Type:   types.True,
			},
		}
		ctx, _ := InitRuleChainCtx(NewConfig(), nil, &ruleChainDef, nil)
		ctx.New()
	})

	t.Run("Init", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), nil, &ruleChainDef, nil)
		newRuleChainDef := types.RuleChain{}
		err := ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Nil(t, err)

		newRuleChainDef = types.RuleChain{}
		ruleNode := types.RuleNode{Type: "notFound"}
		newRuleChainDef.Metadata.Nodes = append(newRuleChainDef.Metadata.Nodes, &ruleNode)
		err = ctx.Init(NewConfig(), types.Configuration{"selfDefinition": &newRuleChainDef})
		assert.Equal(t, "nodeType:notFound for id:node0 new error:component not found. componentType=notFound", err.Error())
	})

	t.Run("ReloadChildNotFound", func(t *testing.T) {
		ctx, _ := InitRuleChainCtx(NewConfig(), nil, &ruleChainDef, nil)
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
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)
		ruleChainCtx, _ := InitRuleChainCtx(config, nil, &def, nil)
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
		maps.Map2Struct(nodeCtx.(*RuleNodeCtx).Node, &output)
		nodeStr := str.ToString(output)
		assert.True(t, strings.Contains(nodeStr, "127.0.0.1"))
		ruleChainFile = strings.Replace(ruleChainFile, "127.0.0.1", "192.168.1.1", -1)
		err = ruleChainCtx.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, err)
		nodeCtx = ruleChainCtx.nodes[types.RuleNodeId{Id: "s1"}]
		maps.Map2Struct(nodeCtx.(*RuleNodeCtx).Node, &output)
		nodeStr = str.ToString(output)
		assert.True(t, strings.Contains(nodeStr, "192.168.1.1"))
	})

}

func TestGetLCA(t *testing.T) {
	config := NewConfig()

	t.Run("NodeNotExists", func(t *testing.T) {
		// 测试节点不存在的情况
		ruleChainDef := types.RuleChain{}
		ctx, _ := InitRuleChainCtx(config, nil, &ruleChainDef, nil)

		nonExistentNodeId := types.RuleNodeId{Id: "nonexistent", Type: types.NODE}
		lca, found := ctx.GetLCA(nonExistentNodeId)
		assert.False(t, found)
		assert.Equal(t, types.RuleNodeId{}, lca)
	})

	t.Run("NoParents", func(t *testing.T) {
		// 测试没有父节点的情况
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_chain",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "node1",
						"type": "jsFilter",
						"name": "Node 1"
					}
				],
				"connections": []
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		nodeId := types.RuleNodeId{Id: "node1", Type: types.NODE}
		lca, found := ctx.GetLCA(nodeId)
		assert.False(t, found)
		assert.Equal(t, types.RuleNodeId{}, lca)
	})

	t.Run("SingleParent", func(t *testing.T) {
		// 测试单个父节点的情况
		ruleChainFile := `{
	"ruleChain": {
		"id": "InVGNEN0NOnN",
		"name": "测试join",
		"debugMode": true,
		"root": true,
		"disabled": false,
		"additionalInfo": {
			"createTime": "2025/10/27 16:09:02",
			"description": "",
			"layoutX": "274",
			"layoutY": "279",
			"noDefaultInput": false,
			"updateTime": "2025/10/29 18:27:25",
			"username": "admin"
		}
	},
	"metadata": {
		"endpoints": [],
		"nodes": [
			{
				"id": "node_13",
				"additionalInfo": {
					"layoutX": 473,
					"layoutY": 279
				},
				"type": "switch",
				"name": "条件分支",
				"debugMode": false,
				"configuration": {
					"cases": [
						{
							"case": "msg.temperature>=20 && msg.temperature<=50",
							"then": "Case1"
						},
						{
							"case": "msg.temperature>50",
							"then": "Case2"
						}
					]
				}
			},
			{
				"id": "node_4",
				"additionalInfo": {
					"layoutX": 702,
					"layoutY": 204
				},
				"type": "jsTransform",
				"name": "js转换1",
				"debugMode": false,
				"configuration": {
					"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
				}
			},
			{
				"id": "node_7",
				"additionalInfo": {
					"layoutX": 1080,
					"layoutY": 331
				},
				"type": "join",
				"name": "合并",
				"debugMode": false,
				"configuration": {
					"timeout": 0
				}
			},
			{
				"id": "node_14",
				"additionalInfo": {
					"layoutX": 938,
					"layoutY": 200
				},
				"type": "jsTransform",
				"name": "js转换1",
				"debugMode": false,
				"configuration": {
					"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
				}
			}
		],
		"connections": [
			{
				"fromId": "node_13",
				"toId": "node_4",
				"type": "Case1"
			},
			{
				"fromId": "node_4",
				"toId": "node_14",
				"type": "Success"
			},
			{
				"fromId": "node_14",
				"toId": "node_7",
				"type": "Success"
			},
			{
				"fromId": "node_13",
				"toId": "node_7",
				"type": "Case2"
			}
		]
	}
}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		childNodeId := types.RuleNodeId{Id: "node_7", Type: types.NODE}
		lca, found := ctx.GetLCA(childNodeId)
		assert.True(t, found)
		assert.Equal(t, "node_13", lca.Id)
	})

	t.Run("MultipleParents", func(t *testing.T) {
		// 测试多个父节点的情况
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_chain",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "root",
						"type": "jsFilter",
						"name": "Root Node"
					},
					{
						"id": "parent1",
						"type": "jsFilter",
						"name": "Parent 1"
					},
					{
						"id": "parent2",
						"type": "jsFilter",
						"name": "Parent 2"
					},
					{
						"id": "child",
						"type": "jsFilter",
						"name": "Child Node"
					}
				],
				"connections": [
					{
						"fromId": "root",
						"toId": "parent1",
						"type": "True"
					},
					{
						"fromId": "root",
						"toId": "parent2",
						"type": "False"
					},
					{
						"fromId": "parent1",
						"toId": "child",
						"type": "True"
					},
					{
						"fromId": "parent2",
						"toId": "child",
						"type": "True"
					}
				]
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		childNodeId := types.RuleNodeId{Id: "child", Type: types.NODE}
		lca, found := ctx.GetLCA(childNodeId)
		assert.True(t, found)
		assert.Equal(t, "root", lca.Id)
	})

	t.Run("CacheFunctionality", func(t *testing.T) {
		// 测试缓存功能
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_chain",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "root",
						"type": "jsFilter",
						"name": "Root Node"
					},
					{
						"id": "parent1",
						"type": "jsFilter",
						"name": "Parent 1"
					},
					{
						"id": "parent2",
						"type": "jsFilter",
						"name": "Parent 2"
					},
					{
						"id": "child",
						"type": "jsFilter",
						"name": "Child Node"
					}
				],
				"connections": [
					{
						"fromId": "root",
						"toId": "parent1",
						"type": "True"
					},
					{
						"fromId": "root",
						"toId": "parent2",
						"type": "False"
					},
					{
						"fromId": "parent1",
						"toId": "child",
						"type": "True"
					},
					{
						"fromId": "parent2",
						"toId": "child",
						"type": "True"
					}
				]
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		childNodeId := types.RuleNodeId{Id: "child", Type: types.NODE}

		// 第一次调用，应该计算并缓存结果
		lca1, found1 := ctx.GetLCA(childNodeId)
		assert.True(t, found1)
		assert.Equal(t, "root", lca1.Id)

		// 第二次调用，应该从缓存中获取结果
		lca2, found2 := ctx.GetLCA(childNodeId)
		assert.True(t, found2)
		assert.Equal(t, "root", lca2.Id)
		assert.Equal(t, lca1, lca2)

		// 验证缓存功能正常工作（通过多次调用验证一致性）
		for i := 0; i < 5; i++ {
			lcaTest, foundTest := ctx.GetLCA(childNodeId)
			assert.True(t, foundTest)
			assert.Equal(t, "root", lcaTest.Id)
			assert.Equal(t, lca1, lcaTest)
		}
	})

	t.Run("ComplexHierarchy", func(t *testing.T) {
		// 测试复杂层级结构
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_chain",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "root",
						"type": "jsFilter",
						"name": "Root Node"
					},
					{
						"id": "level1_a",
						"type": "jsFilter",
						"name": "Level 1 A"
					},
					{
						"id": "level1_b",
						"type": "jsFilter",
						"name": "Level 1 B"
					},
					{
						"id": "level2_a",
						"type": "jsFilter",
						"name": "Level 2 A"
					},
					{
						"id": "level2_b",
						"type": "jsFilter",
						"name": "Level 2 B"
					},
					{
						"id": "level2_c",
						"type": "jsFilter",
						"name": "Level 2 C"
					},
					{
						"id": "target",
						"type": "jsFilter",
						"name": "Target Node"
					}
				],
				"connections": [
					{
						"fromId": "root",
						"toId": "level1_a",
						"type": "True"
					},
					{
						"fromId": "root",
						"toId": "level1_b",
						"type": "False"
					},
					{
						"fromId": "level1_a",
						"toId": "level2_a",
						"type": "True"
					},
					{
						"fromId": "level1_a",
						"toId": "level2_b",
						"type": "False"
					},
					{
						"fromId": "level1_b",
						"toId": "level2_c",
						"type": "True"
					},
					{
						"fromId": "level2_a",
						"toId": "target",
						"type": "True"
					},
					{
						"fromId": "level2_b",
						"toId": "target",
						"type": "True"
					},
					{
						"fromId": "level2_c",
						"toId": "target",
						"type": "True"
					}
				]
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		targetNodeId := types.RuleNodeId{Id: "target", Type: types.NODE}
		lca, found := ctx.GetLCA(targetNodeId)
		assert.True(t, found)
		assert.Equal(t, "root", lca.Id)
	})

	t.Run("NoCommonAncestor", func(t *testing.T) {
		// 测试没有共同祖先的情况
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_chain",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "isolated1",
						"type": "jsFilter",
						"name": "Isolated 1"
					},
					{
						"id": "isolated2",
						"type": "jsFilter",
						"name": "Isolated 2"
					},
					{
						"id": "child",
						"type": "jsFilter",
						"name": "Child Node"
					}
				],
				"connections": [
					{
						"fromId": "isolated1",
						"toId": "child",
						"type": "True"
					},
					{
						"fromId": "isolated2",
						"toId": "child",
						"type": "True"
					}
				]
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		childNodeId := types.RuleNodeId{Id: "child", Type: types.NODE}
		lca, found := ctx.GetLCA(childNodeId)
		assert.False(t, found)
		assert.Equal(t, types.RuleNodeId{}, lca)
	})

	t.Run("SingleParentJoin", func(t *testing.T) {
		// 测试单父节点Join的情况
		ruleChainFile := `{
			"ruleChain": {
				"id": "singleParentJoin",
				"name": "单父节点Join测试"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "node_transform",
						"type": "jsTransform",
						"name": "Transform",
						"configuration": {
							"jsScript": "msg.single='single_value'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "node_join",
						"type": "join",
						"name": "SingleJoin",
						"configuration": {}
					}
				],
				"connections": [
					{
						"fromId": "node_transform",
						"toId": "node_join",
						"type": "Success"
					}
				]
			}
		}`

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		// 测试 node_join 的 LCA
		joinNodeId := types.RuleNodeId{Id: "node_join", Type: types.NODE}
		// 测试 GetLCA - 应该返回父节点 node_transform 本身
		lca, hasLCA := ctx.GetLCA(joinNodeId)
		assert.True(t, hasLCA)
		assert.Equal(t, "node_transform", lca.Id)
	})

	t.Run("ConditionalBranchJoin", func(t *testing.T) {
		// 测试条件分支Join的情况
		// node_2 (switch) 有两个输出：Case1 -> node_6 -> node_4, Case2 -> node_4
		// node_4 的 LCA 应该是 node_2

		// 从文件加载规则链配置
		ruleChainFile, err := os.ReadFile("../testdata/rule/test_conditional_branch_join.json")
		assert.Nil(t, err)

		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain(ruleChainFile)
		assert.Nil(t, err)

		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		joinNodeId := types.RuleNodeId{Id: "node_4", Type: types.NODE}
		// 测试 node_4 的 LCA - 应该返回 node_2 - with debug output
		lca, hasLCA := ctx.GetLCA(joinNodeId)
		assert.True(t, hasLCA)
		assert.Equal(t, "node_2", lca.Id, "node_4 的 LCA 应该是 node_2，因为 node_2 是两个父节点路径的共同祖先")

	})

	t.Run("ComplexJoinCase", func(t *testing.T) {
		// Load DSL from external file
		dslBytes, err := os.ReadFile("../testdata/rule/test_complex_join.json")
		assert.Nil(t, err)

		// Parse the JSON to rule chain definition
		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain(dslBytes)
		assert.Nil(t, err)

		// Create rule chain context with the parsed definition
		ctx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)

		// Test parent nodes for node_7 (should have node_14 and node_5)
		joinNodeId := types.RuleNodeId{Id: "node_7", Type: types.NODE}
		lca, hasLCA := ctx.GetLCA(joinNodeId)
		assert.True(t, hasLCA, "node_7 应该有 LCA")
		assert.Equal(t, "node_13", lca.Id, "node_7 的 LCA 应该是 node_13，因为所有路径都经过 node_13")

	})
}
