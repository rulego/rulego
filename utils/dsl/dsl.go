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
	"regexp"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
)

const FieldNameScript = "script"

// ParseCrossNodeDependencies 解析规则链中的跨节点依赖关系，返回每个节点依赖的节点ID列表（仅包含在规则链中定义的节点）
// ParseCrossNodeDependencies parses cross-node dependencies in the rule chain and returns dependent node IDs for each node (only includes nodes defined in the rule chain)
func ParseCrossNodeDependencies(def types.RuleChain) map[string][]string {
	dependencies := make(map[string][]string)

	for _, node := range def.Metadata.Nodes {
		referencedNodes := ExtractReferencedNodeIds(node.Configuration)
		if len(referencedNodes) > 0 {
			// Remove duplicates and filter only defined nodes
			uniqueNodes := make([]string, 0, len(referencedNodes))
			seen := make(map[string]bool)
			for _, nodeId := range referencedNodes {
				// 只添加在规则链中实际定义的节点ID
				// Only add node IDs that are actually defined in the rule chain
				if !seen[nodeId] && IsNodeIdDefined(def, nodeId) {
					seen[nodeId] = true
					uniqueNodes = append(uniqueNodes, nodeId)
				}
			}
			if len(uniqueNodes) > 0 {
				dependencies[node.Id] = uniqueNodes
			}
		}
	}

	return dependencies
}

// GetReferencedNodeIds 获取规则链中所有被引用且在规则链中定义的节点ID列表（去重）
// GetReferencedNodeIds gets all referenced node IDs that are defined in the rule chain (deduplicated)
func GetReferencedNodeIds(def types.RuleChain) []string {
	referencedNodeSet := make(map[string]bool)

	for _, node := range def.Metadata.Nodes {
		referencedNodes := ExtractReferencedNodeIds(node.Configuration)
		for _, nodeId := range referencedNodes {
			// 只添加在规则链中实际定义的节点ID
			// Only add node IDs that are actually defined in the rule chain
			if IsNodeIdDefined(def, nodeId) {
				referencedNodeSet[nodeId] = true
			}
		}
	}

	// Convert set to slice
	referencedNodeIds := make([]string, 0, len(referencedNodeSet))
	for nodeId := range referencedNodeSet {
		referencedNodeIds = append(referencedNodeIds, nodeId)
	}

	return referencedNodeIds
}

// IsNodeIdDefined 检查给定的nodeId是否在规则链的节点定义中
// IsNodeIdDefined checks if the given nodeId is defined in the rule chain nodes
func IsNodeIdDefined(def types.RuleChain, nodeId string) bool {
	for _, node := range def.Metadata.Nodes {
		if node.Id == nodeId {
			return true
		}
	}
	return false
}

// ExtractReferencedNodeIds 从节点配置中提取被引用的节点ID，支持复杂的expr-lang表达式
// ExtractReferencedNodeIds extracts referenced node IDs from node configuration, supports complex expr-lang expressions
func ExtractReferencedNodeIds(configuration types.Configuration) []string {
	var nodeIds []string
	uniqueNodeIds := make(map[string]bool)

	for _, fieldValue := range configuration {
		if strV, ok := fieldValue.(string); ok {
			// 提取所有${...}表达式内容
			// Extract all ${...} expression content
			expressionRegex := regexp.MustCompile(`\$\{([^}]+)\}`)
			expressionMatches := expressionRegex.FindAllStringSubmatch(strV, -1)

			for _, exprMatch := range expressionMatches {
				if len(exprMatch) > 1 {
					expressionContent := exprMatch[1]
					// 从表达式内容中提取节点引用
					// Extract node references from expression content
					extractedNodes := ExtractNodeReferencesFromExpression(expressionContent)
					for _, nodeId := range extractedNodes {
						if !uniqueNodeIds[nodeId] {
							uniqueNodeIds[nodeId] = true
							nodeIds = append(nodeIds, nodeId)
						}
					}
				}
			}
		}
	}

	return nodeIds
}

// BuiltinVars 内置变量列表，这些不应该被识别为节点ID
// Built-in variables list, these should not be recognized as node IDs
var BuiltinVars = map[string]bool{
	"msg":      true,
	"metadata": true,
	"msgType":  true,
	"global":   true,
	"vars":     true,
	"len":      true,
	"string":   true,
	"int":      true,
	"float":    true,
	"bool":     true,
	"true":     true,
	"false":    true,
}

// ExtractNodeReferencesFromExpression 从表达式内容中提取节点引用
// ExtractNodeReferencesFromExpression extracts node references from expression content
func ExtractNodeReferencesFromExpression(expression string) []string {
	var nodeIds []string
	uniqueNodeIds := make(map[string]bool)

	// 使用精确的正则表达式来匹配节点引用
	// Use precise regex to match node references
	// 只匹配 nodeId.data, nodeId.msg, nodeId.metadata 这三种模式
	// Only match nodeId.data, nodeId.msg, nodeId.metadata patterns
	nodeRefRegex := regexp.MustCompile(`\b([a-zA-Z][a-zA-Z0-9_]*)\.(data|msg|metadata)\b`)
	matches := nodeRefRegex.FindAllStringSubmatch(expression, -1)

	for _, match := range matches {
		if len(match) > 1 {
			nodeId := match[1]
			// 排除内置变量，只处理真正的跨节点引用
			// Exclude built-in variables, only process real cross-node references
			if !BuiltinVars[nodeId] {
				if !uniqueNodeIds[nodeId] {
					uniqueNodeIds[nodeId] = true
					nodeIds = append(nodeIds, nodeId)
				}
			}
		}
	}

	return nodeIds
}

// ParseVars 解析规则链中的变量
func ParseVars(varPrefix string, def types.RuleChain, includeNodeId ...string) []string {
	var mergeVars = make(map[string]struct{})
	includeNodeIdLen := len(includeNodeId)
	for _, node := range def.Metadata.Nodes {
		if includeNodeIdLen > 0 && !str.Contains(includeNodeId, node.Id) {
			continue
		}
		for fieldName, fieldValue := range node.Configuration {
			if strV, ok := fieldValue.(string); ok {
				var vars []string
				if strings.Contains(strings.ToLower(fieldName), FieldNameScript) {
					//脚本通过 {varPrefix}.xx 方式解析
					vars = str.ParseVars(varPrefix, strV)
				} else {
					//通过 ${{varPrefix}.xx} 方式解析
					vars = str.ParseVarsWithBraces(varPrefix, strV)
				}
				for _, v := range vars {
					mergeVars[v] = struct{}{}
				}
			}
		}
	}
	var result []string
	for varName := range mergeVars {
		result = append(result, varName)
	}
	return result
}

// IsFlowNode 判断是否是子规则链
func IsFlowNode(def types.RuleChain, nodeId string) bool {
	for _, node := range def.Metadata.Nodes {
		if node.Id == nodeId && node.Type == "flow" {
			return true
		}
	}
	return false
}

// ProcessVariables replaces placeholders in the node configuration with global and chain-specific variables.
func ProcessVariables(config types.Config, ruleChainDef types.RuleChain, from types.Configuration) types.Configuration {
	to := make(types.Configuration)
	env := GetInitNodeEnv(config, ruleChainDef)
	for key, value := range from {
		if strV, ok := value.(string); ok {
			to[key] = str.ExecuteTemplate(strV, env)
		} else {
			to[key] = value
		}
	}

	return to
}

func GetInitNodeEnv(config types.Config, ruleChainDef types.RuleChain) map[string]interface{} {
	varsEnv := ruleChainDef.RuleChain.Configuration[types.Vars]
	globalEnv := make(map[string]string)

	if config.Properties != nil {
		globalEnv = config.Properties.Values()
	}
	env := map[string]interface{}{
		types.Global: globalEnv,
		types.Vars:   varsEnv,
	}
	return env
}
