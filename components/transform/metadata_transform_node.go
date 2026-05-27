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

//规则链节点配置示例：
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

// MetadataTransformNodeConfiguration 节点配置
type MetadataTransformNodeConfiguration struct {
	// Mapping is a field-to-expression map for metadata transformation.
	Mapping map[string]string `json:"mapping" label:"Mapping" desc:"Field-to-expression map, e.g. {\"temperature\":\"msg.temperature\"}" required:"true"`
	// IsNew: true creates new metadata, false updates existing keys.
	IsNew bool `json:"isNew" label:"Is New" desc:"true=create new metadata structure, false=update existing keys"`
}

// MetadataTransformNode 使用expr表达式转换或者创建新的元数据
// 则把多个字段转换结果替换元数据对应key（如果isNew=true，则创建新的元数据结构体），转到下一个节点
// 转换结构如下：
//
//	{
//	  fieldKey1:fieldValue1
//	  fieldKey2:fieldValue2
//	}
//
// fieldValue 可以使用 expr 从当前的msg或者metadata中获取值，例如:
//
//	"configuration": {
//		"mapping": {
//		"name":        "upper(msg.name)",
//		"tmp":         "msg.temperature",
//		"alarm":       "msg.temperature>50",
//		"productType": "metaData.productType",
//	}
//
// 通过`id`变量访问消息id
// 通过`ts`变量访问消息时间戳
// 通过`data`变量访问消息原始数据
// 通过`msg`变量访问转换后消息体，如果消息的dataType是json类型，可以通过 `msg.XX`方式访问msg的字段。例如:`msg.temperature > 50;`
// 通过`metadata`变量访问消息元数据。例如 `metadata.customerName`
// 通过`type`变量访问消息类型
// 通过`dataType`变量访问数据类型
type MetadataTransformNode struct {
	//节点配置
	Config         MetadataTransformNodeConfiguration
	templateMapping map[string]el.Template
}

// Type 组件类型
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

// Init 初始化
func (x *MetadataTransformNode) Init(_ types.Config, configuration types.Configuration) error {
	//删除默认配置
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

// OnMsg 处理消息
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

// Destroy 销毁
func (x *MetadataTransformNode) Destroy() {
}
