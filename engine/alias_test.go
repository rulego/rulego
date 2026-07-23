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
	// Register aliases for test components
	err := Registry.RegisterAlias("jsFilter", "js_filter", "javascriptFilter")
	assert.Nil(t, err)

	err = Registry.RegisterAlias("jsTransform", "js_transform", "javascriptTransform")
	assert.Nil(t, err)

	t.Run("ChainWithAliasNodeType", func(t *testing.T) {
		// Create a rule chain using aliases as node types
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

		// Create a rule chain context using aliases
		ruleChainCtx, err := InitRuleChainCtx(config, nil, &def, nil)
		assert.Nil(t, err)
		assert.NotNil(t, ruleChainCtx)

		// Validator nodes are correctly created (using aliases)
		node1Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node1"}]
		assert.NotNil(t, node1Ctx)
		// SelfDefinition retains the type name from the original configuration
		assert.Equal(t, "js_filter", node1Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		node2Ctx := ruleChainCtx.nodes[types.RuleNodeId{Id: "node2"}]
		assert.NotNil(t, node2Ctx)
		assert.Equal(t, "js_transform", node2Ctx.(*RuleNodeCtx).SelfDefinition.Type)

		// Verify that the underlying node instance is the correct master type
		assert.Equal(t, "jsFilter", node1Ctx.Type())
		assert.Equal(t, "jsTransform", node2Ctx.Type())
	})

	t.Run("ChainWithMultipleAliases", func(t *testing.T) {
		// Test using different aliases within the same rule chain
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

		// All nodes should be created correctly
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}])
		assert.NotNil(t, ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}])

		// Verify that all nodes are of the correct primary type
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter1"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter2"}].Type())
		assert.Equal(t, "jsFilter", ruleChainCtx.nodes[types.RuleNodeId{Id: "filter3"}].Type())
	})

	t.Run("NodeWithAlias", func(t *testing.T) {
		// Separate tests using aliases to create nodes
		selfDefinition := types.RuleNode{
			Id:   "test_node",
			Type: "js_filter", // Use aliases
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)
		// SelfDefinition retains the original configuration
		assert.Equal(t, "js_filter", ctx.SelfDefinition.Type)
		// But the underlying node is the correct primary type
		assert.Equal(t, "jsFilter", ctx.Type())
	})

	t.Run("NewNodeWithAlias", func(t *testing.T) {
		// Test creating new node instances using aliases
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
		// Create a temporary alias
		err := Registry.RegisterAlias("log", "logAlias")
		assert.Nil(t, err)

		// Creating nodes with aliases should be successful
		selfDefinition := types.RuleNode{
			Id:   "log_node",
			Type: "logAlias",
		}
		ctx, err := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.Nil(t, err)
		assert.NotNil(t, ctx)

		// Delete aliases
		err = Registry.Unregister("logAlias")
		assert.Nil(t, err)

		// Creating a node with a deleted alias should fail
		_, err = InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		assert.NotNil(t, err)
	})

	// Cleanup: Delete test aliases
	defer func() {
		Registry.Unregister("js_filter")
		Registry.Unregister("javascriptFilter")
		Registry.Unregister("js_transform")
		Registry.Unregister("javascriptTransform")
	}()
}
