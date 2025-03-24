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

package engine

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/schema"
	"github.com/rulego/rulego/utils/str"
	"strings"
)

// ErrRuleEnginePoolNil rule engine pool is nil
var ErrRuleEnginePoolNil = errors.New("rule engine pool is nil")

// ErrDSLEmpty dsl is empty
var ErrDSLEmpty = errors.New("dsl is empty")

// DynamicNode 通过子规则链动态定义节点组件
// ruleChain.id: 定义组件类型
// ruleChain.name: 定义组件label
// ruleChain.additionalInfo.category: 定义组件分类
// ruleChain.additionalInfo.icon: 定义组件图标
// ruleChain.additionalInfo.description: 定义组件描述
// ruleChain.additionalInfo.inputSchema: 使用JSON Schema 定义组件的输入参数(组件参数配置)
// ruleChain.additionalInfo.relationTypes: 定义和下一个节点允许连接关系类型
// 组件通过 ${vars.xx} 方式获取组件配置参数
// 使用示例：
// 通过dsl定义组件：
// dynamicNode := NewDynamicNode("fahrenheit", `
//
//		 {
//		 "ruleChain": {
//		   "id": "fahrenheit",
//		   "name": "华氏温度转换",
//		   "debugMode": false,
//		   "root": false,
//		   "additionalInfo": {
//		     "layoutX": 720,
//		     "layoutY": 260,
//	         "description":"this is a description",
//		     "relationTypes":["Success","Failure"],
//		     "inputSchema": {
//		       "type": "object",
//		       "properties": {
//		         "appKey": {
//		           "type": "string"
//		         }
//		       },
//		       "required": ["appKey"]
//		     }
//
//		   }
//		 },
//		 "metadata": {
//		   "firstNodeIndex": 0,
//		   "nodes": [
//		     {
//		       "id": "s2",
//		       "type": "jsTransform",
//		       "name": "摄氏温度转华氏温度",
//		       "debugMode": true,
//		       "configuration": {
//		         "jsScript": "var newMsg={'appKey':'${vars.appKey}','temperature': msg.temperature*(9/5)+32};\n return {'msg':newMsg,'metadata':metadata,'msgType':msgType};"
//		       }
//		     }
//		   ],
//		   "connections": [
//		     {
//		     }
//		   ]
//		 }
//		}
//
//	`)
//	注册组件
//	Registry.Register(dynamicNode)
type DynamicNode struct {
	//ComponentType 组件类型
	ComponentType string
	//Dsl 子规则链 DSL
	Dsl string
	//实例化的节点配置
	instantiatedConfig types.Configuration
	//实例化规则引擎
	ruleEngine types.RuleEngine
}

func NewDynamicNode(componentType, componentDsl string) *DynamicNode {
	return &DynamicNode{
		ComponentType: componentType,
		Dsl:           componentDsl,
	}
}

// Type 组件类型
func (x *DynamicNode) Type() string {
	return x.ComponentType
}

func (x *DynamicNode) New() types.Node {
	return &DynamicNode{
		ComponentType: x.ComponentType,
		Dsl:           x.Dsl,
	}
}

// Init 初始化
func (x *DynamicNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	chainCtx := base.NodeUtils.GetChainCtx(configuration)
	if chainCtx == nil {
		return ErrRuleEnginePoolNil
	}
	if x.Dsl == "" {
		return ErrDSLEmpty
	}
	err := maps.Map2Struct(configuration, &x.instantiatedConfig)
	if err != nil {
		return err
	}
	rootChainId := chainCtx.GetNodeId().Id
	self := base.NodeUtils.GetSelfDefinition(configuration)
	newChainId := rootChainId + "#" + self.Id
	componentDef, err := ruleConfig.Parser.DecodeRuleChain([]byte(x.Dsl))
	if err != nil {
		return err
	}

	//把组件配置和跟规则链vars复制到当前组件定义的vars
	newComponentDef := x.copyVars(componentDef, chainCtx.Definition(), configuration)
	newComponentDsl, err := ruleConfig.Parser.EncodeRuleChain(newComponentDef)
	if err != nil {
		return err
	}

	//动态初始化子规则链
	x.ruleEngine, err = NewRuleEngine(newChainId, newComponentDsl)
	return err
}

// OnMsg 处理消息
func (x *DynamicNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	if x.ruleEngine == nil {
		ctx.TellFailure(msg, errors.New("rule engine is nil"))
		return
	}
	x.ruleEngine.OnMsg(msg, types.WithContext(ctx.GetContext()),
		types.WithOnEnd(func(nodeCtx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			if err != nil {
				ctx.TellFailure(onEndMsg, err)
			} else {
				ctx.TellNext(onEndMsg, relationType)
			}
		}))
}

// Destroy 销毁
func (x *DynamicNode) Destroy() {
	if x.ruleEngine != nil {
		x.ruleEngine.Stop()
	}
}

// Def 组件定义
func (x *DynamicNode) Def() types.ComponentForm {
	var componentForm types.ComponentForm
	var ruleChain types.RuleChain
	_ = json.Unmarshal([]byte(x.Dsl), &ruleChain)
	var icon = "custom-node"
	var category = "custom"
	var description string
	var version string
	var relationTypes = []string{types.Success, types.Failure}
	if ruleChain.RuleChain.AdditionalInfo != nil {
		if v := str.ToString(ruleChain.RuleChain.AdditionalInfo["icon"]); v != "" {
			icon = v
		}
		if v := str.ToString(ruleChain.RuleChain.AdditionalInfo["category"]); v != "" {
			category = v
		}
		if v := str.ToString(ruleChain.RuleChain.AdditionalInfo["description"]); v != "" {
			description = v
		}
		if v := str.ToString(ruleChain.RuleChain.AdditionalInfo["version"]); v != "" {
			version = v
		}
		// 获取关系类型
		relationTypesValue := ruleChain.RuleChain.AdditionalInfo["relationTypes"]
		if relationTypesValue != nil {
			if v, ok := relationTypesValue.([]string); ok && len(v) > 0 {
				relationTypes = v
			} else if v, ok := relationTypesValue.(string); ok {
				if v := strings.Split(v, ","); len(v) > 0 {
					relationTypes = v
				}
			}
		}
	}

	// 获取输入参数定义
	inputSchemaMap := ruleChain.RuleChain.AdditionalInfo["inputSchema"]
	var inputSchema schema.JSONSchema
	var fields types.ComponentFormFieldList
	if inputSchemaMap != nil {
		_ = maps.Map2Struct(inputSchemaMap, &inputSchema)

		// 获取字段列表并排序
		var fieldNames []string
		for name := range inputSchema.Properties {
			fieldNames = append(fieldNames, name)
		}

		for _, name := range fieldNames {
			fieldMap := inputSchema.Properties[name]
			field := x.processField(name, fieldMap, inputSchema)
			fields = append(fields, field)
		}

	}
	componentForm = types.ComponentForm{
		Type:          x.ComponentType,
		Category:      category,
		Label:         ruleChain.RuleChain.Name,
		Desc:          description,
		Icon:          icon,
		Fields:        fields,
		RelationTypes: &relationTypes,
		Version:       version,
	}
	return componentForm
}

// processField 处理单个字段，支持嵌套字段
func (x *DynamicNode) processField(name string, fieldMap schema.FieldSchema, parentSchema schema.JSONSchema) types.ComponentFormField {
	var rules []map[string]interface{}
	if parentSchema.CheckFieldIsRequired(name) {
		rules = []map[string]interface{}{
			{
				"required": true,
				"message":  "This field is required",
			},
		}
	}

	field := types.ComponentFormField{
		Name:         name,
		Label:        fieldMap.Title,
		Type:         fieldMap.Type,
		DefaultValue: fieldMap.Default,
		Fields:       nil,
		Rules:        rules,
		Desc:         fieldMap.Description,
	}

	if fieldMap.Type == "object" && fieldMap.Properties != nil {
		// 获取子字段列表并排序
		var nestedFieldNames []string
		for nestedName := range fieldMap.Properties {
			nestedFieldNames = append(nestedFieldNames, nestedName)
		}

		for _, nestedName := range nestedFieldNames {
			nestedFieldMap := fieldMap.Properties[nestedName]
			nestedField := x.processField(nestedName, nestedFieldMap, schema.JSONSchema{
				Required: fieldMap.Required,
			})
			field.Fields = append(field.Fields, nestedField)
		}
	}

	return field
}
func (x *DynamicNode) copyVars(targetRuleChain types.RuleChain, fromRootChain *types.RuleChain, fromNodeConfig types.Configuration) types.RuleChain {
	var varsMap map[string]interface{}
	if vars, ok := targetRuleChain.RuleChain.Configuration[types.Vars]; ok {
		if v, ok := vars.(map[string]interface{}); ok {
			varsMap = v
		} else {
			varsMap = make(map[string]interface{})
		}
	} else {
		varsMap = make(map[string]interface{})
	}

	if fromRootChain != nil {
		if fromRootVars, ok := fromRootChain.RuleChain.Configuration[types.Vars]; ok {
			if fromRootVarsMap, ok := fromRootVars.(map[string]interface{}); ok {
				for k, v := range fromRootVarsMap {
					varsMap[k] = v
				}
			}
		}
	}

	for k, v := range fromNodeConfig {
		if strings.HasPrefix(k, "$") {
			continue
		}
		varsMap[k] = v
	}
	if targetRuleChain.RuleChain.Configuration == nil {
		targetRuleChain.RuleChain.Configuration = make(types.Configuration)
	}
	targetRuleChain.RuleChain.Configuration[types.Vars] = varsMap
	return targetRuleChain
}
