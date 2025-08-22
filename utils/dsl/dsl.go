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

// extractReferencedNodeIds 从节点配置中提取被引用的节点ID
// extractReferencedNodeIds extracts referenced node IDs from node configuration
func ExtractReferencedNodeIds(configuration types.Configuration) []string {
	var nodeIds []string
	uniqueNodeIds := make(map[string]bool)

	for _, fieldValue := range configuration {
		if strV, ok := fieldValue.(string); ok {
			// 解析模板语法中的跨节点变量引用: ${nodeId.data.xxx}, ${nodeId.metadata.xxx}, ${nodeId.msg.xxx}
			re := regexp.MustCompile(`\$\{([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)(?:\.[^}]*)?\}`)
			matches := re.FindAllStringSubmatch(strV, -1)

			for _, match := range matches {
				if len(match) > 1 {
					nodeId := match[1]
					// 排除内置变量，只处理真正的跨节点引用
					// Exclude built-in variables, only process real cross-node references
					if nodeId != "msg" && nodeId != "metadata" && nodeId != "msgType" && nodeId != "global" && nodeId != "vars" {
						// 确保这是一个跨节点引用（第二个捕获组是字段名）
						// Ensure this is a cross-node reference (second capture group is field name)
						if len(match) > 2 && match[2] != "" {
							if !uniqueNodeIds[nodeId] {
								uniqueNodeIds[nodeId] = true
								nodeIds = append(nodeIds, nodeId)
							}
						}
					}
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
