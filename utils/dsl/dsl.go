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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
	"strings"
)

const FieldNameScript = "script"

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
