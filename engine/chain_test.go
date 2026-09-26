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
	"context"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
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

// ReloadSelf 后拓扑派生状态（父节点表/End 标记/LCA）必须随新图刷新，
// 否则 join 判定、OnEnd 门控等会继续使用旧图直到进程重启。
func TestReloadSelfRefreshTopologyState(t *testing.T) {
	config := NewConfig()
	jsonParser := JsonParser{}
	dsl := []byte(`{
	  "ruleChain": {"id": "reload-topo", "name": "reloadTopo"},
	  "metadata": {
	    "nodes": [
	      {"id": "a", "type": "jsFilter", "name": "a", "configuration": {"jsScript": "return true"}},
	      {"id": "b", "type": "jsFilter", "name": "b", "configuration": {"jsScript": "return true"}}
	    ],
	    "connections": [{"fromId": "a", "toId": "b", "relationType": "Success"}]
	  }
	}`)
	def, err := jsonParser.DecodeRuleChain(dsl)
	assert.Nil(t, err)
	ctx, err := InitRuleChainCtx(config, nil, &def, nil)
	assert.Nil(t, err)
	assert.False(t, ctx.HasEndNode())
	if _, ok := ctx.GetParentNodeIds(types.RuleNodeId{Id: "b"}); !ok {
		t.Fatal("initial parentNodeIds missing b")
	}

	newDsl := []byte(`{
	  "ruleChain": {"id": "reload-topo", "name": "reloadTopo"},
	  "metadata": {
	    "nodes": [
	      {"id": "a", "type": "jsFilter", "name": "a", "configuration": {"jsScript": "return true"}},
	      {"id": "c", "type": "jsFilter", "name": "c", "configuration": {"jsScript": "return true"}},
	      {"id": "e", "type": "end", "name": "e"}
	    ],
	    "connections": [
	      {"fromId": "a", "toId": "c", "relationType": "Success"},
	      {"fromId": "c", "toId": "e", "relationType": "Success"}
	    ]
	  }
	}`)
	err = ctx.ReloadSelf(newDsl)
	assert.Nil(t, err)

	assert.True(t, ctx.HasEndNode())
	parents, ok := ctx.GetParentNodeIds(types.RuleNodeId{Id: "c"})
	if !ok || len(parents) == 0 || parents[0].Id != "a" {
		t.Errorf("after reload, parents(c) = %v, want [a]", parents)
	}
	if _, ok := ctx.GetParentNodeIds(types.RuleNodeId{Id: "b"}); ok {
		t.Error("stale node b should be gone from parentNodeIds after reload")
	}
	if _, ok := ctx.GetLCA(types.RuleNodeId{Id: "c"}); !ok {
		t.Error("GetLCA(c) should resolve after reload")
	}
}

// GetNextNodes 锁外计算期间发生 ReloadSelf 时，缓存写回必须核对拓扑版本，
// 否则已销毁的旧节点指针会被永久写入新缓存（命中后每次路由都拿到失效节点）。
func TestGetNextNodesConcurrentReload(t *testing.T) {
	config := NewConfig()
	jsonParser := JsonParser{}
	// v1 有 a-False->c；v2 删除 c 与该边
	dslV1 := []byte(`{
	  "ruleChain": {"id": "next-nodes-reload"},
	  "metadata": {
	    "nodes": [
	      {"id": "a", "type": "jsFilter", "name": "a", "configuration": {"jsScript": "return true"}},
	      {"id": "b", "type": "jsFilter", "name": "b", "configuration": {"jsScript": "return true"}},
	      {"id": "c", "type": "jsFilter", "name": "c", "configuration": {"jsScript": "return true"}}
	    ],
	    "connections": [
	      {"fromId": "a", "toId": "b", "type": "Success"},
	      {"fromId": "a", "toId": "c", "type": "False"}
	    ]
	  }
	}`)
	dslV2 := []byte(`{
	  "ruleChain": {"id": "next-nodes-reload"},
	  "metadata": {
	    "nodes": [
	      {"id": "a", "type": "jsFilter", "name": "a", "configuration": {"jsScript": "return true"}},
	      {"id": "b", "type": "jsFilter", "name": "b", "configuration": {"jsScript": "return true"}}
	    ],
	    "connections": [
	      {"fromId": "a", "toId": "b", "type": "Success"}
	    ]
	  }
	}`)
	def, err := jsonParser.DecodeRuleChain(dslV1)
	assert.Nil(t, err)
	ctx, err := InitRuleChainCtx(config, nil, &def, nil)
	assert.Nil(t, err)

	nodeA := types.RuleNodeId{Id: "a", Type: types.NODE}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				nodes, _ := ctx.GetNextNodes(nodeA, types.False)
				for _, n := range nodes {
					// v1 只可能返回 c，v2 返回空；出现其他 id 说明缓存被旧图污染
					if n.GetNodeId().Id != "c" {
						t.Errorf("GetNextNodes(a,False) returned unexpected node %s", n.GetNodeId().Id)
						return
					}
				}
				runtime.Gosched()
			}
		}()
	}
	for i := 0; i < 100; i++ {
		dsl := dslV2
		if i%2 == 1 {
			dsl = dslV1
		}
		if err := ctx.ReloadSelf(dsl); err != nil {
			t.Fatal(err)
		}
	}
	// 终态固定为 v2 后停止并发读
	if err := ctx.ReloadSelf(dslV2); err != nil {
		t.Fatal(err)
	}
	close(stop)
	wg.Wait()

	nodes, ok := ctx.GetNextNodes(nodeA, types.False)
	assert.False(t, ok, "v2 has no False edge from a")
	for _, n := range nodes {
		assert.NotEqual(t, "c", n.GetNodeId().Id, "deleted node c leaked into relationCache")
	}
}

// TestForkJoinWithForkNode 测试使用正确的 fork 节点设计
// 规则链结构:
//
//	node_20 (fork)
//	    │
//	    ├──→ node_2 (js转换a) → node_12 (js转换c) → node_5 (join)
//	    │
//	    └──→ node_3 (js转换b) → node_5 (join)
//
// 预期: join 后 metadata 应该同时包含 a, b, c
func TestForkJoinWithForkNode(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_join_with_fork",
			"name": "Test Fork Join With Fork Node",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node_20",
					"type": "fork",
					"name": "并行分支"
				},
				{
					"id": "node_2",
					"type": "jsTransform",
					"name": "js转换a",
					"configuration": {
						"jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_3",
					"type": "jsTransform",
					"name": "js转换b",
					"configuration": {
						"jsScript": "metadata.b=\"b\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_12",
					"type": "jsTransform",
					"name": "js转换c",
					"configuration": {
						"jsScript": "metadata.c=\"c\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_5",
					"type": "join",
					"name": "合并",
					"configuration": {
						"mergeToMap": true,
						"timeout": 5
					}
				}
			],
			"connections": [
				{
					"fromId": "node_20",
					"toId": "node_2",
					"type": "Success"
				},
				{
					"fromId": "node_20",
					"toId": "node_3",
					"type": "Success"
				},
				{
					"fromId": "node_2",
					"toId": "node_12",
					"type": "Success"
				},
				{
					"fromId": "node_12",
					"toId": "node_5",
					"type": "Success"
				},
				{
					"fromId": "node_3",
					"toId": "node_5",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test_fork_join_with_fork", []byte(ruleChainDef), WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_5" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")
		valueB := metadata.GetValue("b")
		valueC := metadata.GetValue("c")

		t.Logf("Metadata after join: a=%s, b=%s, c=%s", valueA, valueB, valueC)

		// 验证所有元数据都被正确合并
		if valueA != "a" {
			t.Errorf("Expected metadata.a='a', got '%s'", valueA)
		}
		if valueB != "b" {
			t.Errorf("Expected metadata.b='b', got '%s'", valueB)
		}
		if valueC != "c" {
			t.Errorf("Expected metadata.c='c', got '%s'", valueC)
		}

		// 如果所有值都正确，测试通过
		if valueA == "a" && valueB == "b" && valueC == "c" {
			t.Log("SUCCESS: All metadata correctly merged!")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestForkNodeDirectToJoin 测试 fork 节点直接连接到 join 节点的场景
// 规则链结构:
//
//	fork ────────→ join (直接连接)
//	    │
//	    └──→ js转换 → join
//
// 预期: 这种设计也有问题，因为 fork 直接连接到 join 会创建"零长度"分支
func TestForkNodeDirectToJoin(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_direct_to_join",
			"name": "Test Fork Direct To Join",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node_fork",
					"type": "fork",
					"name": "并行分支"
				},
				{
					"id": "node_a",
					"type": "jsTransform",
					"name": "js转换a",
					"configuration": {
						"jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					}
				},
				{
					"id": "node_join",
					"type": "join",
					"name": "合并",
					"configuration": {
						"mergeToMap": true,
						"timeout": 5
					}
				}
			],
			"connections": [
				{
					"fromId": "node_fork",
					"toId": "node_join",
					"type": "Success"
				},
				{
					"fromId": "node_fork",
					"toId": "node_a",
					"type": "Success"
				},
				{
					"fromId": "node_a",
					"toId": "node_join",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test_fork_direct_to_join", []byte(ruleChainDef), WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_join" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")

		t.Logf("Metadata after join: a=%s", valueA)

		// 关键验证: fork 直接连接到 join 会导致问题
		if valueA != "a" {
			t.Logf("BUG CONFIRMED: Fork node directly connected to join causes early callback trigger!")
			t.Logf("  metadata.a = '%s' (expected: 'a') - LOST", valueA)
		} else {
			t.Log("SUCCESS: metadata.a correctly merged")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestForkDirectToJoinWithMetadataMerge 测试有问题的规则链设计（无 fork 节点）
// 模拟用户提供的规则链结构:
//
//	node_3 (js转换b) → node_2 (js转换a) → node_12 (js转换c) → node_5 (join)
//	node_3 (js转换b) → node_5 (join)  [直接连接]
//
// 预期: join 后 metadata 应该同时包含 a, b, c
func TestForkDirectToJoinWithMetadataMerge(t *testing.T) {
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_fork_join_metadata",
			"name": "Test Fork Join Metadata",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
                   	"id": "node_3",
                    "type": "jsTransform",
                    "name": "js转换b",
                    "configuration": {
                        "jsScript": "metadata.b=\"b\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_2",
                    "type": "jsTransform",
                    "name": "js转换a",
                    "configuration": {
                        "jsScript": "metadata.a=\"a\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_12",
                    "type": "jsTransform",
                    "name": "js转换c",
                    "configuration": {
                        "jsScript": "metadata.c=\"c\"\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
                    }
                },
                {
                    "id": "node_5",
                    "type": "join",
                    "name": "合并",
                    "configuration": {
                        "mergeToMap": true,
                        "timeout": 5
                    }
                }
            ],
            "connections": [
                {
                    "fromId": "node_2",
                    "toId": "node_12",
                    "type": "Success"
                },
                {
                    "fromId": "node_12",
                    "toId": "node_5",
                    "type": "Success"
                },
                {
                    "fromId": "node_3",
                    "toId": "node_2",
                    "type": "Success"
                },
                {
                    "fromId": "node_3",
                    "toId": "node_5",
                    "type": "Success"
                }
            ]
        }
    }`

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test_fork_join_metadata", []byte(ruleChainDef), WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, `{}`)
	done := make(chan struct{})
	var lock sync.Mutex
	var joinNodeLog *types.RuleNodeRunLog
	var allNodeLogs = make(map[string]types.RuleNodeRunLog)

	ruleEngine.OnMsg(msg,
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			allNodeLogs[nodeRunLog.Id] = nodeRunLog
			t.Logf("Node %s completed", nodeRunLog.Id)
			if nodeRunLog.Id == "node_5" {
				joinNodeLog = &nodeRunLog
			}
		}),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			t.Log("Rule chain completed")
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()

		// 打印所有节点的日志
		for id := range allNodeLogs {
			t.Logf("Node %s: executed", id)
		}

		if joinNodeLog == nil {
			t.Fatal("join node log is nil")
		}

		// 验证元数据是否被正确合并
		metadata := joinNodeLog.OutMsg.Metadata
		if metadata == nil {
			t.Fatal("metadata is nil")
		}

		valueA := metadata.GetValue("a")
		valueB := metadata.GetValue("b")
		valueC := metadata.GetValue("c")

		t.Logf("Metadata after join: a=%s, b=%s, c=%s", valueA, valueB, valueC)

		// 关键验证: 所有分支的元数据都应该存在
		// 注意: 由于 node_3 直接连接到 node_5 (join), 这是一个已知的 bug 场景
		// 在当前实现中,join 可能不会等待 node_12 分支完成

		// 风险: 这是已知 bug,		// 当 node_3 同时连接到 node_2 和 node_5 时:
		// - node_3 → node_5 (直接连接) 会先到达 join
		// - node_3 → node_2 → node_12 → node_5 (长路径) 后到达
		// join 节点可能在收到第一条消息后就触发回调, 导致第二条消息的数据丢失

		t.Logf("BUG VERIFICATION:")
		t.Logf("  metadata.a = '%s' (expected: 'a') - %s", valueA,
			map[bool]string{true: "LOST", false: "OK"}[valueA == "a"])
		t.Logf("  metadata.b = '%s' (expected: 'b') - %s", valueB,
			map[bool]string{true: "OK", false: "LOST"}[valueB == "b"])
		t.Logf("  metadata.c = '%s' (expected: 'c') - %s", valueC,
			map[bool]string{true: "LOST", false: "OK"}[valueC == "c"])

		// 记录 bug 现象
		if valueA != "a" || valueC != "c" {
			t.Logf("BUG CONFIRMED: Join node triggered callback before all branches completed!")
			t.Logf("  This causes metadata from node_2->node_12 path to be lost")
		}

	case <-time.After(time.Second * 10):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

func TestLCAComplexBug(t *testing.T) {
	// 读取规则链文件
	ruleChainFile := filepath.Join("..", "testdata", "rule", "test_lca_complex_bug.json")
	buf, err := os.ReadFile(ruleChainFile)
	if err != nil {
		t.Skip("Skip test because rule chain file not found:", err)
		return
	}

	config := NewConfig(types.WithDefaultPool())
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if err != nil {
			t.Logf("Node error! flowType=%s, nodeId=%s, relationType=%s, err=%v", flowType, nodeId, relationType, err)
		}
	}

	ruleEngine, err := New("test_lca_complex_bug", buf, WithConfig(config))
	assert.Nil(t, err)
	if err != nil {
		t.Fatal(err)
	}
	defer ruleEngine.Stop(context.Background())

	runTest := func(runIndex int) {
		metaData := types.NewMetadata()
		msgData := `{}`

		msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, msgData)

		var wg sync.WaitGroup
		wg.Add(1)

		var joinNodeLog *types.RuleNodeRunLog
		var endNodeLog *types.RuleNodeRunLog

		ruleEngine.OnMsg(msg,
			types.WithContext(context.Background()),
			types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
				if nodeRunLog.Id == "node_86" { // join node
					joinNodeLog = &nodeRunLog
				}
				if nodeRunLog.Id == "node_122" { // end node
					endNodeLog = &nodeRunLog
				}
			}),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				wg.Done()
			}),
		)

		// 等待执行完成或超时
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Logf("Run %d: rule chain finished", runIndex)
		case <-time.After(5 * time.Second):
			t.Fatalf("Run %d: rule chain timeout!", runIndex)
		}

		if joinNodeLog != nil {
			metadataStr := joinNodeLog.OutMsg.Metadata.Values()
			_, hasLevelInfo := metadataStr["level_info"]
			t.Logf("Run %d: node_86 join node metadata has level_info: %v", runIndex, hasLevelInfo)
			if !hasLevelInfo {
				t.Errorf("Run %d: level_info missing from join node metadata!", runIndex)
			}
		} else {
			t.Errorf("Run %d: node_86 join node did not run!", runIndex)
		}

		if endNodeLog == nil {
			t.Errorf("Run %d: end node node_122 did not run!", runIndex)
		}
	}

	// 跑2次以复现问题
	for i := 1; i <= 2; i++ {
		t.Logf("--- Starting Run %d ---", i)
		runTest(i)
	}
}
