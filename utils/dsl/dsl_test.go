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

	// Test case 6: Function expressions with node references
	// 测试包含函数表达式的节点引用
	config6 := types.Configuration{
		"param1": "${upper(node1.msg.name)}",
		"param2": "${lower(node2.data.value)}",
		"param3": "${trim(config_node.metadata.description)}",
		"param4": "${substring(service_node.msg.content, 0, 10)}",
	}
	referencedNodes6 := ExtractReferencedNodeIds(config6)
	assert.Equal(t, 4, len(referencedNodes6))
	assert.True(t, contains(referencedNodes6, "node1"))
	assert.True(t, contains(referencedNodes6, "node2"))
	assert.True(t, contains(referencedNodes6, "config_node"))
	assert.True(t, contains(referencedNodes6, "service_node"))

	// Test case 7: Mixed function expressions and regular references
	// 测试混合函数表达式和常规引用
	config7 := types.Configuration{
		"script": "var name = ${upper(node1.msg.name)}; var value = ${node2.data.value}; return name + value;",
		"url":    "http://localhost:8080/${lower(api_node.metadata.endpoint)}/${node3.data.id}",
	}
	referencedNodes7 := ExtractReferencedNodeIds(config7)
	assert.Equal(t, 4, len(referencedNodes7))
	assert.True(t, contains(referencedNodes7, "node1"))
	assert.True(t, contains(referencedNodes7, "node2"))
	assert.True(t, contains(referencedNodes7, "api_node"))
	assert.True(t, contains(referencedNodes7, "node3"))

	// Test case 8: Function expressions with nested properties
	// 测试包含嵌套属性的函数表达式
	config8 := types.Configuration{
		"param1": "${format(node1.data.user.name)}",
		"param2": "${validate(node2.metadata.config.rules)}",
	}
	referencedNodes8 := ExtractReferencedNodeIds(config8)
	assert.Equal(t, 2, len(referencedNodes8))
	assert.True(t, contains(referencedNodes8, "node1"))
	assert.True(t, contains(referencedNodes8, "node2"))

	// Test case 9: Function expressions should not match built-in variables
	// 测试函数表达式不应匹配内置变量
	config9 := types.Configuration{
		"param1": "${upper(msg.name)}",
		"param2": "${lower(metadata.type)}",
		"param3": "${format(global.config)}",
		"param4": "${trim(vars.description)}",
	}
	referencedNodes9 := ExtractReferencedNodeIds(config9)
	assert.Equal(t, 0, len(referencedNodes9)) // Should not extract built-in variables

	// Test case 10: Complex expr-lang expressions with nested functions
	// 测试复杂的expr-lang表达式，包含嵌套函数
	config10 := types.Configuration{
		"param1": "${split(lower(node1.data.Name), ' ')}",
		"param2": "${join(upper(node2.data.items), ',')}",
		"param3": "${substring(trim(config_node.metadata.description), 0, 10)}",
	}
	referencedNodes10 := ExtractReferencedNodeIds(config10)
	assert.Equal(t, 3, len(referencedNodes10))
	assert.True(t, contains(referencedNodes10, "node1"))
	assert.True(t, contains(referencedNodes10, "node2"))
	assert.True(t, contains(referencedNodes10, "config_node"))

	// Test case 11: Complex conditional and mathematical expressions
	// 测试复杂的条件和数学表达式
	config11 := types.Configuration{
		"condition1": "${node1.data.value > 100 && node2.metadata.status == 'active'}",
		"condition2": "${len(processor_node.msg.items) > 0 ? validator_node.data.result : 'default'}",
		"calculation": "${(node1.data.price * node2.data.quantity) + service_node.metadata.tax}",
	}
	referencedNodes11 := ExtractReferencedNodeIds(config11)
	assert.Equal(t, 5, len(referencedNodes11))
	assert.True(t, contains(referencedNodes11, "node1"))
	assert.True(t, contains(referencedNodes11, "node2"))
	assert.True(t, contains(referencedNodes11, "processor_node"))
	assert.True(t, contains(referencedNodes11, "validator_node"))
	assert.True(t, contains(referencedNodes11, "service_node"))

	// Test case 12: Array/slice operations and nested property access
	// 测试数组/切片操作和嵌套属性访问
	config12 := types.Configuration{
		"nested": "${api_node.data.response.user.profile.name}",
		"array_op": "${node1.data.items[0].name + node2.msg.values[node3.data.index]}",
		"complex": "${transform_node.metadata.config.rules[0].condition && filter_node.data.results[1].status}",
	}
	referencedNodes12 := ExtractReferencedNodeIds(config12)
	assert.Equal(t, 6, len(referencedNodes12))
	assert.True(t, contains(referencedNodes12, "api_node"))
	assert.True(t, contains(referencedNodes12, "node1"))
	assert.True(t, contains(referencedNodes12, "node2"))
	assert.True(t, contains(referencedNodes12, "node3"))
	assert.True(t, contains(referencedNodes12, "transform_node"))
	assert.True(t, contains(referencedNodes12, "filter_node"))

	// Test case 13: Mixed complex expressions in single string
	// 测试单个字符串中的混合复杂表达式
	config13 := types.Configuration{
		"script": "var name = ${upper(node1.msg.name)}; var items = ${split(node2.data.list, ',')}; return ${node3.data.value > 0 ? processor_node.metadata.result : 'empty'};",
	}
	referencedNodes13 := ExtractReferencedNodeIds(config13)
	assert.Equal(t, 4, len(referencedNodes13))
	assert.True(t, contains(referencedNodes13, "node1"))
	assert.True(t, contains(referencedNodes13, "node2"))
	assert.True(t, contains(referencedNodes13, "node3"))
	assert.True(t, contains(referencedNodes13, "processor_node"))
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
