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

// ParseCrossNodeDependencies parses cross-node dependencies in the rule chain and returns a list of node IDs for each node dependencies (only nodes defined in the rule chain)
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
				// Only add the node IDs actually defined in the rule chain
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

// GetReferencedNodeIds Retrieves a list of all node IDs referenced and defined in the rule chain (deduplication)
// GetReferencedNodeIds gets all referenced node IDs that are defined in the rule chain (deduplicated)
func GetReferencedNodeIds(def types.RuleChain) []string {
	referencedNodeSet := make(map[string]bool)

	for _, node := range def.Metadata.Nodes {
		referencedNodes := ExtractReferencedNodeIds(node.Configuration)
		for _, nodeId := range referencedNodes {
			// Only add the node IDs actually defined in the rule chain
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

// IsNodeIdDefined checks whether a given nodeId is included in the node definition of the rule chain
// IsNodeIdDefined checks if the given nodeId is defined in the rule chain nodes
func IsNodeIdDefined(def types.RuleChain, nodeId string) bool {
	for _, node := range def.Metadata.Nodes {
		if node.Id == nodeId {
			return true
		}
	}
	return false
}

// ExtractReferencedNodeIds Extract a list of referenced node IDs from the node configuration (supports nested fields)
// ExtractReferencedNodeIds extracts referenced node IDs from node configuration (supports nested fields)
func ExtractReferencedNodeIds(configuration types.Configuration) []string {
	var nodeIds []string
	uniqueNodeIds := make(map[string]bool)

	for _, value := range configuration {
		extractNodeIdsFromValue(value, uniqueNodeIds, &nodeIds)
	}

	return nodeIds
}

// extractNodeIdsFromValue Recursively extracts node references from any type of value
// extractNodeIdsFromValue recursively extracts node references from values of any type
func extractNodeIdsFromValue(value interface{}, uniqueNodeIds map[string]bool, nodeIds *[]string) {
	switch v := value.(type) {
	case string:
		// Extracts node references from strings, supporting ${nodeId.msg.xx} and nodeId.msg.xx formats
		// Extract node references from string, supports ${nodeId.msg.xx} and nodeId.msg.xx formats
		extractedNodes := ExtractNodeReferencesFromExpression(v)
		for _, nodeId := range extractedNodes {
			if !uniqueNodeIds[nodeId] {
				uniqueNodeIds[nodeId] = true
				*nodeIds = append(*nodeIds, nodeId)
			}
		}
	case map[string]interface{}:
		// Recurrent processing of map types
		// Recursively process map type
		for _, mapValue := range v {
			extractNodeIdsFromValue(mapValue, uniqueNodeIds, nodeIds)
		}
	case []interface{}:
		// Recursively handles slice types
		// Recursively process slice type
		for _, sliceValue := range v {
			extractNodeIdsFromValue(sliceValue, uniqueNodeIds, nodeIds)
		}
	case types.Configuration:
		// Recursively handles the Configuration type
		// Recursively process Configuration type
		for _, configValue := range v {
			extractNodeIdsFromValue(configValue, uniqueNodeIds, nodeIds)
		}
	// For other types (int, bool, float, etc.), node references are not included and are directly ignored
	// For other types (int, bool, float, etc.), no node references, ignore
	default:
		// No other types are dealt with
		// Do not process other types
	}
}

// BuiltinVars lists built-in variables that should not be identified as node IDs
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

// ExtractNodeReferencesFromExpression extracts node references from the expression content
// ExtractNodeReferencesFromExpression extracts node references from expression content
func ExtractNodeReferencesFromExpression(expression string) []string {
	var nodeIds []string
	uniqueNodeIds := make(map[string]bool)

	// Use regular expressions to match node references
	// Use regex to match node references
	// Match the patterns nodeId.data, nodeId.msg, nodeId.metadata, nodeId.id, nodeId.ts, nodeId.dataType, nodeId.global, nodeId.vars, and make sure the preceding is not a dot number
	// nodeId supports letters, numbers, underscores, centerlines, and slashes
	// Match nodeId.data, nodeId.msg, nodeId.metadata, etc. patterns, ensuring not preceded by a dot
	// nodeId supports letters, numbers, underscores, hyphens and slashes
	nodeRefRegex := regexp.MustCompile(`(?:^|[^a-zA-Z0-9_./-])([a-zA-Z0-9_\-/]+)\.(data|msg|metadata|id|ts|dataType|global|vars)(?:[^a-zA-Z0-9_]|$)`)
	matches := nodeRefRegex.FindAllStringSubmatch(expression, -1)

	for _, match := range matches {
		if len(match) > 1 {
			nodeId := match[1]
			// Excluding built-in variables, only handling genuine cross-node references
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

// ParseVars parses variables in the rule chain
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
					//The script is parsed using {varPrefix}.xx
					vars = str.ParseVars(varPrefix, strV)
				} else {
					//Parsing via ${{varPrefix}.xx}
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

// IsFlowNode determines whether it is a sub-rule chain
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
