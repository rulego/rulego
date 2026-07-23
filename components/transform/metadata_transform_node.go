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
//	"type": "metadataTransform",
//	"name": "元数据转换",
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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

func init() {
	Registry.Add(&MetadataTransformNode{})
}

// MetadataTransformNodeConfiguration
type MetadataTransformNodeConfiguration struct {
	// Mapping is a field-to-expression map for metadata transformation.
	Mapping map[string]string `json:"mapping" label:"Mapping" desc:"Field-to-expression map, e.g. {\"temperature\":\"msg.temperature\"}" required:"true"`
	// IsNew: true creates new metadata, false updates existing keys.
	IsNew bool `json:"isNew" label:"Is New" desc:"true=create new metadata structure, false=update existing keys"`
}

// MetadataTransformNode uses expr expressions to transform or create new metadata
// then convert multiple fields to replace the metadata corresponding keys (if isNew=true, create a new metadata struct), and proceed to the next node
// The conversion structure is as follows:
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
type MetadataTransformNode struct {
	//Node configuration
	Config          MetadataTransformNodeConfiguration
	templateMapping map[string]el.Template
}

// Type returns the component type
func (x *MetadataTransformNode) Type() string {
	return "metadataTransform"
}

func (x *MetadataTransformNode) New() types.Node {
	return &MetadataTransformNode{Config: MetadataTransformNodeConfiguration{
		Mapping: map[string]string{
			"temperature": "msg.temperature",
		},
	}}
}

// Init initializes the component
func (x *MetadataTransformNode) Init(_ types.Config, configuration types.Configuration) error {
	//Delete the default configuration
	x.Config.Mapping = map[string]string{}
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		x.templateMapping = make(map[string]el.Template)
		for k, v := range x.Config.Mapping {
			if template, err := el.NewExprTemplate(v); err != nil {
				return err
			} else {
				x.templateMapping[k] = template
			}
		}
	}
	return err
}

// OnMsg processes a message
func (x *MetadataTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)
	mapResult := make(map[string]string)
	for fieldName, template := range x.templateMapping {
		if out, err := template.Execute(evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			mapResult[fieldName] = str.ToString(out)
		}
	}
	if x.Config.IsNew {
		msg.Metadata.ReplaceAll(mapResult)
	} else {
		for k, v := range mapResult {
			msg.Metadata.PutValue(k, v)
		}
	}
	ctx.TellSuccess(msg)
}

// Desc returns the component description
func (x *MetadataTransformNode) Desc() string {
	return "Transform message metadata using expr-lang mapping. isNew=true replaces all metadata, false updates existing keys. Variables: id, ts, data, msg, metadata, type, dataType"
}

// Destroy releases resources
func (x *MetadataTransformNode) Destroy() {
}
