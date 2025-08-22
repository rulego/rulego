/*
 * Copyright 2025 The RuleGo Authors.
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

package dsl

import (
	"strings"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
)

func TestIsFlowNode(t *testing.T) {
	ruleChain := types.RuleChain{
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{
				{
					Id:   "node1",
					Type: "flow",
				},
				{
					Id:   "node2",
					Type: "jsTransform",
				},
			},
		},
	}

	assert.True(t, IsFlowNode(ruleChain, "node1"))
	assert.False(t, IsFlowNode(ruleChain, "node2"))
	assert.False(t, IsFlowNode(ruleChain, "nonExistentNode"))

	// Test with empty nodes
	emptyRuleChain := types.RuleChain{
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{},
		},
	}
	assert.False(t, IsFlowNode(emptyRuleChain, "node1"))
}

func TestParseVars(t *testing.T) {
	inputData := `{
		"ruleChain": {
			"id": "fahrenheit",
			"name": "华氏温度转换",
			"debugMode": true,
			"root": true,
			"disabled": false,
			"additionalInfo": {
				"createTime": "2025/03/12 10:08:57",
				"description": "华氏温度转换",
				"height": 40,
				"layoutX": "280",
				"layoutY": "280",
				"updateTime": "2025/03/28 23:53:31",
				"username": "admin",
				"width": 240
			}
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s2",
					"additionalInfo": {
						"layoutX": 590,
						"layoutY": 270
					},
					"type": "jsTransform",
					"name": "摄氏温度转华氏温度",
					"debugMode": true,
					"configuration": {
						"jsScript": "var newMsg={'temperature': msg.temperature*msg.scaleFactor+32};\n return {'msg':newMsg,'metadata':metadata,'msgType':msgType};"
					}
				},
                {
					"id": "s2",
					"type": "dbClient",
					"name": "db query",
					"debugMode": true,
					"configuration": {
						"sql": "${msg.sql}"
					}
				}

			],
			"connections": []
		}
	}`

	var ruleChain types.RuleChain
	err := json.Unmarshal([]byte(inputData), &ruleChain)
	if err != nil {
		t.Fatalf("Failed to unmarshal input data: %v", err)
	}

	actualOutput := ParseVars(types.MsgKey, ruleChain)
	assert.True(t, strings.Contains(strings.Join(actualOutput, ","), "scaleFactor"))
	assert.True(t, strings.Contains(strings.Join(actualOutput, ","), "sql"))
}

// TestExtractReferencedNodeIds 测试从节点配置中提取被引用的节点ID
// TestExtractReferencedNodeIds tests extracting referenced node IDs from node configuration
func TestExtractReferencedNodeIds(t *testing.T) {
	// Test case 1: Basic cross-node references
	config1 := types.Configuration{
		"functionName": "${config_node.metadata.targetFunction}",
		"param1":       "${node1.data.value}",
		"param2":       "${node2.msg.content}",
	}
	referencedNodes1 := ExtractReferencedNodeIds(config1)
	assert.Equal(t, 3, len(referencedNodes1))
	assert.True(t, contains(referencedNodes1, "config_node"))
	assert.True(t, contains(referencedNodes1, "node1"))
	assert.True(t, contains(referencedNodes1, "node2"))

	// Test case 2: No cross-node references
	config2 := types.Configuration{
		"param1": "static_value",
		"param2": "${msg.data}",
		"param3": "${metadata.key}",
	}
	referencedNodes2 := ExtractReferencedNodeIds(config2)
	assert.Equal(t, 0, len(referencedNodes2))

	// Test case 3: Complex nested references
	config3 := types.Configuration{
		"script": "var result = ${node1.data.value} + ${node2.metadata.config.nested};",
		"url":    "http://localhost:8080/api/${service_node.data.endpoint}",
	}
	referencedNodes3 := ExtractReferencedNodeIds(config3)
	assert.Equal(t, 3, len(referencedNodes3))
	assert.True(t, contains(referencedNodes3, "node1"))
	assert.True(t, contains(referencedNodes3, "node2"))
	assert.True(t, contains(referencedNodes3, "service_node"))

	// Test case 4: Duplicate node references
	config4 := types.Configuration{
		"param1": "${node1.data.value1}",
		"param2": "${node1.metadata.value2}",
		"param3": "${node1.msg.value3}",
	}
	referencedNodes4 := ExtractReferencedNodeIds(config4)
	assert.Equal(t, 1, len(referencedNodes4))
	assert.True(t, contains(referencedNodes4, "node1"))

	// Test case 5: Non-string configuration values
	config5 := types.Configuration{
		"timeout": 5000,
		"enabled": true,
		"config":  map[string]interface{}{"key": "${node1.data.value}"},
	}
	referencedNodes5 := ExtractReferencedNodeIds(config5)
	assert.Equal(t, 0, len(referencedNodes5)) // map values are not processed
}

// TestParseCrossNodeDependencies 测试解析规则链中的跨节点依赖关系
// TestParseCrossNodeDependencies tests parsing cross-node dependencies in rule chain
func TestParseCrossNodeDependencies(t *testing.T) {
	// Create test rule chain
	ruleChain := types.RuleChain{
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{
				{
					Id:   "config_node",
					Type: "jsTransform",
					Configuration: types.Configuration{
						"jsScript": "metadata.targetFunction = 'processData'; return {msg: msg, metadata: metadata, msgType: msgType};",
					},
				},
				{
					Id:   "dynamic_function_node",
					Type: "functions",
					Configuration: types.Configuration{
						"functionName": "${config_node.metadata.targetFunction}",
					},
				},
				{
					Id:   "validator_node",
					Type: "jsFilter",
					Configuration: types.Configuration{
						"jsScript": "return ${dynamic_function_node.data.status} === 'processed';",
					},
				},
				{
					Id:   "independent_node",
					Type: "log",
					Configuration: types.Configuration{
						"jsScript": "console.log(msg.data);",
					},
				},
			},
		},
	}

	dependencies := ParseCrossNodeDependencies(ruleChain)

	// dynamic_function_node depends on config_node
	assert.Equal(t, 1, len(dependencies["dynamic_function_node"]))
	assert.True(t, contains(dependencies["dynamic_function_node"], "config_node"))

	// validator_node depends on dynamic_function_node
	assert.Equal(t, 1, len(dependencies["validator_node"]))
	assert.True(t, contains(dependencies["validator_node"], "dynamic_function_node"))

	// config_node and independent_node have no dependencies
	_, hasConfigDeps := dependencies["config_node"]
	_, hasIndependentDeps := dependencies["independent_node"]
	assert.False(t, hasConfigDeps)
	assert.False(t, hasIndependentDeps)
}

// TestGetReferencedNodeIds 测试获取规则链中所有被引用的节点ID
// TestGetReferencedNodeIds tests getting all referenced node IDs in rule chain
func TestGetReferencedNodeIds(t *testing.T) {
	// Create test rule chain
	ruleChain := types.RuleChain{
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{
				{
					Id:   "node1",
					Type: "jsTransform",
					Configuration: types.Configuration{
						"param": "static_value",
					},
				},
				{
					Id:   "node2",
					Type: "functions",
					Configuration: types.Configuration{
						"functionName": "${node1.metadata.func}",
						"param1":       "${node3.data.value}", // node3 not defined in chain
					},
				},
				{
					Id:   "node4",
					Type: "jsFilter",
					Configuration: types.Configuration{
						"condition": "${node1.data.status} === 'ok' && ${node2.metadata.result} === true",
					},
				},
			},
		},
	}

	referencedNodeIds := GetReferencedNodeIds(ruleChain)

	// Should only include nodes that are defined in the rule chain
	assert.Equal(t, 2, len(referencedNodeIds))
	assert.True(t, contains(referencedNodeIds, "node1"))
	assert.True(t, contains(referencedNodeIds, "node2"))
	// node3 should not be included as it's not defined in the rule chain
	assert.False(t, contains(referencedNodeIds, "node3"))
}

// TestIsNodeIdDefined 测试检查节点ID是否在规则链中定义
// TestIsNodeIdDefined tests checking if node ID is defined in rule chain
func TestIsNodeIdDefined(t *testing.T) {
	ruleChain := types.RuleChain{
		Metadata: types.RuleMetadata{
			Nodes: []*types.RuleNode{
				{Id: "node1", Type: "jsTransform"},
				{Id: "node2", Type: "functions"},
				{Id: "node3", Type: "log"},
			},
		},
	}

	assert.True(t, IsNodeIdDefined(ruleChain, "node1"))
	assert.True(t, IsNodeIdDefined(ruleChain, "node2"))
	assert.True(t, IsNodeIdDefined(ruleChain, "node3"))
	assert.False(t, IsNodeIdDefined(ruleChain, "node4"))
	assert.False(t, IsNodeIdDefined(ruleChain, "nonexistent"))
	assert.False(t, IsNodeIdDefined(ruleChain, ""))
}

// contains 辅助函数，检查字符串切片是否包含指定字符串
// contains helper function to check if string slice contains specified string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
