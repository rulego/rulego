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
	"context"
	"errors"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/dsl"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/schema"
	"github.com/rulego/rulego/utils/str"
)

// ErrRuleEnginePoolNil rule engine pool is nil
var ErrRuleEnginePoolNil = errors.New("rule engine pool is nil")

// ErrDSLEmpty dsl is empty
var ErrDSLEmpty = errors.New("dsl is empty")

// DynamicNode dynamically defines node components through a chain of subrule
// ruleChain.id: Define the component type
// ruleChain.name: Define the component label
// ruleChain.additionalInfo.category: Defines component classification
// ruleChain.additionalInfo.icon: Defines the component icon
// ruleChain.additionalInfo.description: Defines the component description
// ruleChain.additionalInfo.inputSchema: Uses JSON Schema to define input parameters for components (component parameter configuration)
// ruleChain.additionalInfo.relationTypes: Defines the type of relationship that allows the next node to connect
// Components obtain configuration parameters via ${vars.xx}
// Example:
// Define components via DSL:
// dynamicNode := NewDynamicNode("fahrenheit", `
//
//			 {
//			 "ruleChain": {
//			   "id": "fahrenheit",
//			   "name": "华氏温度转换",
//			   "debugMode": false,
//			   "root": false,
//			   "additionalInfo": {
//			     "layoutX": 720,
//			     "layoutY": 260,
//		         "description":"this is a description",
//			     "relationTypes":["Success","Failure"],
//			     "inputSchema": {
//			       "type": "object",
//			       "properties": {
//			         "scaleFactor": {
//			           "type": "number",
//	                  "title": "换算系数",
//	                  "default": 1.8
//			         }
//			       },
//			       "required": ["scaleFactor"]
//			     }
//
//			   }
//			 },
//			 "metadata": {
//			   "firstNodeIndex": 0,
//			   "nodes": [
//			     {
//			       "id": "s2",
//			       "type": "jsTransform",
//			       "name": "摄氏温度转华氏温度",
//			       "debugMode": true,
//			       "configuration": {
//			         "jsScript": "var newMsg={'temperature': msg.temperature*vars.scaleFactor+32};\n return {'msg':newMsg,'metadata':metadata,'msgType':msgType};"
//			       }
//			     }
//			   ],
//			   "connections": [
//			     {
//			     }
//			   ]
//			 }
//			}
//
//		`)
//		Register the component
//		Registry.Register(dynamicNode)
type DynamicNode struct {
	//ComponentType
	ComponentType string
	//DSL sub-rule chain DSL
	Dsl string
	//Instantiated node configuration
	instantiatedConfig types.Configuration
	//Instantiate the rule engine
	ruleEngine types.RuleEngine
}

func NewDynamicNode(componentType, componentDsl string) *DynamicNode {
	return &DynamicNode{
		ComponentType: componentType,
		Dsl:           componentDsl,
	}
}

// Type returns the component type
func (x *DynamicNode) Type() string {
	return x.ComponentType
}

func (x *DynamicNode) New() types.Node {
	return &DynamicNode{
		ComponentType: x.ComponentType,
		Dsl:           x.Dsl,
	}
}

// Init initializes the component
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

	//Copy component configuration and rule chain vars to the current component's defined vars
	newComponentDef := x.copyVars(componentDef, chainCtx.Definition(), configuration)
	newComponentDsl, err := ruleConfig.Parser.EncodeRuleChain(newComponentDef)
	if err != nil {
		return err
	}

	//Dynamically initialize the sub-rule chain
	x.ruleEngine, err = NewRuleEngine(newChainId, newComponentDsl, WithConfig(ruleConfig))
	return err
}

// OnMsg processes a message
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

// Destroy releases resources
func (x *DynamicNode) Destroy() {
	if x.ruleEngine != nil {
		x.ruleEngine.Stop(context.Background())
	}
}

// Def component definition
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
		// Obtain relationship types
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

	// Obtain the input parameter definition
	inputSchemaMap := ruleChain.RuleChain.AdditionalInfo["inputSchema"]
	var fields types.ComponentFormFieldList
	if inputSchemaMap != nil {
		var inputSchema schema.JSONSchema
		_ = maps.Map2Struct(inputSchemaMap, &inputSchema)

		// Get a list of fields and sort them
		var fieldNames []string
		for name := range inputSchema.Properties {
			fieldNames = append(fieldNames, name)
		}

		for _, name := range fieldNames {
			fieldMap := inputSchema.Properties[name]
			field := x.processField(name, fieldMap, inputSchema)
			fields = append(fields, field)
		}

	} else {
		fields = x.processFieldAuto(ruleChain)
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
		ComponentKind: types.ComponentKindDynamic,
	}
	return componentForm
}

// processFieldAuto processes automatically generated fields. Generation rule: extract the ${vars.xx} variable
func (x *DynamicNode) processFieldAuto(def types.RuleChain) types.ComponentFormFieldList {
	var fields types.ComponentFormFieldList
	// Find all matching variables
	var vars = dsl.ParseVars(types.Vars, def)
	for _, item := range vars {
		var rules []map[string]interface{}
		var required = true
		rules = []map[string]interface{}{
			{
				"required": true,
				"message":  "This field is required",
			},
		}
		field := types.ComponentFormField{
			Name:         item,
			Label:        item,
			Type:         "string",
			DefaultValue: "${vars." + item + "}",
			Fields:       nil,
			Rules:        rules,
			Required:     required,
		}
		fields = append(fields, field)
	}

	return fields
}

// processField handles individual fields and supports nested fields
func (x *DynamicNode) processField(name string, fieldMap schema.FieldSchema, parentSchema schema.JSONSchema) types.ComponentFormField {
	var rules []map[string]interface{}
	var required = false
	if parentSchema.CheckFieldIsRequired(name) {
		rules = []map[string]interface{}{
			{
				"required": true,
				"message":  "This field is required",
			},
		}
		required = true
	}

	field := types.ComponentFormField{
		Name:         name,
		Label:        fieldMap.Title,
		Type:         fieldMap.Type,
		DefaultValue: fieldMap.Default,
		Fields:       nil,
		Rules:        rules,
		Desc:         fieldMap.Description,
		Required:     required,
	}

	if fieldMap.Type == "object" && fieldMap.Properties != nil {
		// Get a list of subfields and sort them
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
