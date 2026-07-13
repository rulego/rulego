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
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

func TestAliasIntegration(t *testing.T) {
	// 为测试组件注册别名
	err := Registry.RegisterAlias("jsFilter", "js_filter", "javascriptFilter")
	assert.Nil(t, err)

	err = Registry.RegisterAlias("jsTransform", "js_transform", "javascriptTransform")
	assert.Nil(t, err)

	t.Run("ChainWithAliasNodeType", func(t *testing.T) {
		// 使用别名作为节点类型创建规则链
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_alias_chain",
				"name": "Test Alias Chain",
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "node1",
						"type": "js_filter",
						"name": "Filter Node",
						"configuration": {
							"jsScript": "return msg.temperature > 20;"
						}
					},
					{
						"id": "node2",
						"type": "js_transform",
						"name": "Transform Node",
						"configuration": {
							"jsScript": "msg.filtered = true; return msg;"
						}
					}
				],
				"connections": [
					{
						"fromId": "node1",
						"toId": "node2",
						"type": "True"
					}
				]
			}
		}`

		config := NewConfig()
		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		// 使用别名创建规则链上下文
		ruleChainCtx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ruleChainCtx)

		// 验证节点被正确创建（使用别名）
		node1Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node1"}]
		assert.NotNil(t, node1Ctx)
		// SelfDefinition 保留原始配置中的类型名
		assert.Equal(t, "js_filter", node1Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		node2Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node2"}]
		assert.NotNil(t, node2Ctx)
		assert.Equal(t, "js_transform", node2Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		// 验证底层节点实例是正确的主类型
		assert.Equal(t, "jsFilter", node1Ctx.Type())
		assert.Equal(t, "jsTransform", node2Ctx.Type())
	})

	t.Run("ChainWithMultipleAliases", func(t *testing.T) {
		// 测试同一个规则链中使用不同别名
		ruleChainFile := `{
			"ruleChain": {
				"id": "test_multi_alias",
				"name": "Test Multi Alias Chain",
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "filter1",
						"type": "jsFilter",
						"name": "Original Name Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					},
					{
						"id": "filter2",
						"type": "js_filter",
						"name": "Underscore Alias Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					},
					{
						"id": "filter3",
						"type": "javascriptFilter",
						"name": "Full Name Alias Filter",
						"configuration": {
							"jsScript": "return true;"
						}
					}
				],
				"connections": []
			}
		}`

		config := NewConfig()
		jsonParser := JsonParser{}
		def, err := jsonParser.DecodeRuleChain([]byte(ruleChainFile))
		assert.Nil(t, err)

		ruleChainCtx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ruleChainCtx)

		// 所有节点都应该被正确创建
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}])

		// 验证所有节点都是正确的主类型
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}].Type())
	})

	t.Run("NodeWithAlias", func(t *testing.T) {
		// 单独测试使用别名创建节点
		selfDefinition := types.RuleNode{
			Id:   "test_node",
			Type: "js_filter", // 使用别名
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)
		// SelfDefinition 保留原始配置
		assert.Equal(t, "js_filter", ctx.SelfDefinition.Type)
		// 但底层节点是正确的主类型
		assert.Equal(t, "jsFilter", ctx.Type())
	})

	t.Run("NewNodeWithAlias", func(t *testing.T) {
		// 测试通过别名创建新节点实例
		node, err := Registry.NewNode("js_filter")
		assert.Nil(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, "jsFilter", node.Type())

		node, err = Registry.NewNode("javascriptTransform")
		assert.Nil(t, err)
		assert.NotNil(t, node)
		assert.Equal(t, "jsTransform", node.Type())
	})

	t.Run("UnregisterAliasAffectsChain", func(t *testing.T) {
		// 创建一个临时别名
		err := Registry.RegisterAlias("log", "logAlias")
		assert.Nil(t, err)

		// 使用别名创建节点应该成功
		selfDefinition := types.RuleNode{
			Id:   "log_node",
			Type: "logAlias",
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)

		// 删除别名
		err = Registry.Unregister("logAlias")
		assert.Nil(t, err)

		// 使用已删除的别名创建节点应该失败
		_, err = InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.NotNil(t, err)
	})

	// 清理：删除测试别名
	defer func() {
		Registry.Unregister("js_filter")
		Registry.Unregister("javascriptFilter")
		Registry.Unregister("js_transform")
		Registry.Unregister("javascriptTransform")
	}()
}
