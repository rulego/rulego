/*
 * Copyright 2023 The RuleGo Authors.
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

package transform

//Example of rule chain node configuration:
//{
//	"id": "s1",
//	"type": "exprTransform",
//	"name": "表达式转换",
//	"debugMode": false,
//		"configuration": {
//			"mapping": {
//			"name":        "upper(msg.name)",
//			"tmp":         "msg.temperature",
//			"alarm":       "msg.temperature>50",
//			"productType": "metaData.productType"
//		}
//	}
//}
import (
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

func init() {
	Registry.Add(&ExprTransformNode{})
}

// ExprTransformNodeConfiguration
type ExprTransformNodeConfiguration struct {
	// Expr is a single expression. Result replaces msg data. Takes priority over Mapping.
	Expr string `json:"expr" label:"Expression" desc:"Single expression to transform msg. Result replaces msg data. Takes priority over mapping"`
	// Mapping is a field-to-expression map. Results become JSON. Used when Expr is empty.
	Mapping map[string]string `json:"mapping" label:"Mapping" desc:"Field-to-expression map, e.g. {\"name\":\"upper(msg.name)\"}. Used when expr is empty"`
}

// ExprTransformNode uses expr expressions to convert or create new msg
// If config.Expr has a value, replace the conversion result with msg and proceed to the next node
// If config.Mapping has a value, then convert multiple fields into JSON and replace them with msg to the next node
// If both Mapping and Expr exist together, prioritize using config.Expr
// The structure of multiple field conversion msg is as follows:
//
//	{
//	  fieldKey1:fieldValue1
//	  fieldKey2:fieldValue2
//	}
//
// fieldValue can be obtained using expr from the current msg or metadata, for example:
//
//	"configuration": {
//		"mapping": {
//		"name":        "upper(msg.name)",
//		"tmp":         "msg.temperature",
//		"alarm":       "msg.temperature>50",
//		"productType": "metaData.productType",
//	}
//
// Access the message `id` via the 'id' variable
// Access message timestamps via the `ts` variable
// Access the original message `data` through the 'data' variable
// Access the transformed message body via the `msg` variable. If the message's dataType is of JSON type, you can use `msg.XX` to access the msg field. For example: `msg.temperature > 50;` `
// Access message `metadata` through the 'metadata' variable. For example, `metadata.customerName`
// Access message `type`s via the 'type' variable
// Access data types via the `dataType` variable
type ExprTransformNode struct {
	//Node configuration
	Config          ExprTransformNodeConfiguration
	exprTemplate    el.Template
	templateMapping map[string]el.Template
}

// Type returns the component type
func (x *ExprTransformNode) Type() string {
	return "exprTransform"
}

func (x *ExprTransformNode) New() types.Node {
	return &ExprTransformNode{Config: ExprTransformNodeConfiguration{}}
}

// Init initializes the component
func (x *ExprTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if exprV := strings.TrimSpace(x.Config.Expr); exprV != "" {
			if template, err := el.NewExprTemplate(exprV); err != nil {
				return err
			} else {
				x.exprTemplate = template
			}
		} else {
			x.templateMapping = make(map[string]el.Template)
			for k, v := range x.Config.Mapping {
				if template, err := el.NewExprTemplate(v); err != nil {
					return err
				} else {
					x.templateMapping[k] = template
				}
			}
		}

	}
	return err
}

// OnMsg processes a message
func (x *ExprTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)
	var result interface{}
	if x.exprTemplate != nil {
		if out, err := x.exprTemplate.Execute(evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			result = out
		}
	} else {
		mapResult := make(map[string]interface{})
		for fieldName, template := range x.templateMapping {
			if out, err := template.Execute(evn); err != nil {
				ctx.TellFailure(msg, err)
				return
			} else {
				mapResult[fieldName] = out
			}
		}
		result = mapResult
		msg.DataType = types.JSON
	}

	if newValue, err := str.ToStringMaybeErr(result); err == nil {
		msg.SetData(newValue)
		ctx.TellSuccess(msg)
	} else {
		ctx.TellFailure(msg, err)
	}

}

// Desc returns the component description
func (x *ExprTransformNode) Desc() string {
	return "Transform messages using expr-lang. Single expr replaces msg, or mapping creates multi-field JSON. Variables: id, ts, data, msg, metadata, type, dataType. Routes to Success/Failure"
}

// Destroy releases resources
func (x *ExprTransformNode) Destroy() {
}
