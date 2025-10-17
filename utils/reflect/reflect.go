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

// Package reflect provides utility functions for reflection-based operations.
// It includes functions for extracting component configurations, generating
// component forms, and working with struct fields.
//
// This package is particularly useful for introspecting and manipulating
// RuleGo components at runtime, allowing for dynamic configuration and
// form generation based on the structure of component types.
//
// Key features:
// - GetComponentForm: Generates a form structure for a given component
// - GetComponentConfig: Extracts configuration information from a component
// - GetFields: Retrieves field information from struct types
// - SetField: Sets field values in structs using reflection
//
// The functions in this package are designed to work with the RuleGo
// component system, providing flexibility and ease of use when dealing
// with various component types and their configurations.
package reflect

import (
	"github.com/rulego/rulego/api/types/endpoint"
	"reflect"
	"strconv"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
)

// GetComponentForm 获取组件的表单结构
func GetComponentForm(component types.Node) types.ComponentForm {
	var componentForm types.ComponentForm

	t, configField, configValue := GetComponentConfig(component)
	componentForm.Label = t.Name()
	componentForm.Type = component.Type()
	componentForm.Category = strings.Replace(t.PkgPath(), "github.com/rulego/rulego/components/", "", -1)
	componentForm.Category = strings.Replace(componentForm.Category, "github.com/rulego/rulego-components/", "", -1)
	componentForm.Fields = GetFields(configField, configValue)
	var relationTypes = []string{types.Success, types.Failure}
	componentForm.ComponentKind = types.ComponentKindNative
	if component.Type() == "iterator" {
		relationTypes = []string{types.True, types.False, types.Success, types.Failure}
	} else if strings.Contains(strings.ToLower(componentForm.Label), "filter") {
		relationTypes = []string{types.True, types.False, types.Failure}
	} else if strings.Contains(strings.ToLower(componentForm.Label), "switch") {
		relationTypes = []string{}
	} else if _, ok := component.(endpoint.Endpoint); ok {
		relationTypes = []string{}
		componentForm.ComponentKind = types.ComponentKindEndpoint
	}
	componentForm.RelationTypes = &relationTypes
	//如果实现ComponentDefGetter接口，使用接口定义的代替
	if componentDefGetter, ok := component.(types.ComponentDefGetter); ok {
		componentForm = coverComponentForm(componentDefGetter, componentForm)
	}
	if categoryGetter, ok := component.(types.CategoryGetter); ok {
		componentForm.Category = categoryGetter.Category()
	}
	if descGetter, ok := component.(types.DescGetter); ok {
		componentForm.Desc = descGetter.Desc()
	}
	return componentForm
}

// 使用ComponentDefGetter接口定义的覆盖
func coverComponentForm(from types.ComponentDefGetter, toComponentForm types.ComponentForm) types.ComponentForm {
	def := from.Def()
	if def.Type != "" {
		toComponentForm.Type = def.Type
	}
	if def.Category != "" {
		toComponentForm.Category = def.Category
	}
	if len(def.Fields) != 0 {
		toComponentForm.Fields = def.Fields
	}
	if def.Label != "" {
		toComponentForm.Label = def.Label
	}
	if def.Desc != "" {
		toComponentForm.Desc = def.Desc
	}
	if def.RelationTypes != nil {
		toComponentForm.RelationTypes = def.RelationTypes
	}
	if def.Version != "" {
		toComponentForm.Version = def.Version
	}
	if def.ComponentKind != "" {
		toComponentForm.ComponentKind = def.ComponentKind
	}
	if def.Icon != "" {
		toComponentForm.Icon = def.Icon
	}
	toComponentForm.Disabled = def.Disabled

	return toComponentForm
}

// GetComponentConfig 获取组件配置字段和默认值
func GetComponentConfig(component types.Node) (reflect.Type, reflect.StructField, reflect.Value) {
	//component = component.New()
	t := reflect.TypeOf(component)
	if t.Kind() == reflect.Ptr {
		t = t.Elem() // 解引用指针，获取指向的值
	}

	var configField reflect.StructField
	var ok bool
	var configValue reflect.Value
	if configField, ok = t.FieldByName("config"); !ok {
		if configField, ok = t.FieldByName("Config"); ok {
			v := reflect.ValueOf(component)
			if v.Kind() == reflect.Ptr {
				v = v.Elem() // 解引用指针，获取指向的值
			}
			configValue = v.FieldByName("Config")
		}
	} else {
		v := reflect.ValueOf(component)
		if v.Kind() == reflect.Ptr {
			v = v.Elem() // 解引用指针，获取指向的值
		}
		configValue = v.FieldByName("config")
	}
	return t, configField, configValue
}

// GetFields 获取组件config字段
func GetFields(configField reflect.StructField, configValue reflect.Value) []types.ComponentFormField {
	var fields []types.ComponentFormField
	if configField.Type != nil {
		for i := 0; i < configField.Type.NumField(); i++ {
			field := configField.Type.Field(i)

			// 跳过私有字段（首字母小写）
			if !field.IsExported() {
				continue
			}

			// 检查json标签，如果是"-"则跳过
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			var defaultValue interface{}
			if configValue.Field(i).CanInterface() {
				defaultValue = configValue.Field(i).Interface()
			}
			label := field.Tag.Get("label")
			desc := field.Tag.Get("desc")
			validate := field.Tag.Get("validate")
			required, _ := strconv.ParseBool(field.Tag.Get("required"))
			typeName := field.Type.Name()
			var subFields []types.ComponentFormField
			if field.Type.Kind() == reflect.Map {
				typeName = "map"
			} else if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
				typeName = "array"
			} else if field.Type.Kind() == reflect.Struct {
				typeName = "struct"
				//如果字段类型是结构体，那么递归调用 GetFields 函数，传入字段的类型对象和值对象，获取子字段的信息
				subFields = GetFields(field, configValue.Field(i))
			}
			var rules []map[string]interface{}
			if required {
				rules = append(rules, map[string]interface{}{
					"required": true,
					"message":  "This field is required",
				})
			}
			
			// 从rules标签获取验证规则配置
			rulesTag := field.Tag.Get("rules")
			if rulesTag != "" {
				// 解析JSON格式的rules标签
				// 例如: rules:"[{\"required\":true,\"message\":\"必填字段\"},{\"min\":1,\"message\":\"最小值为1\"}]"
				var tagRules []map[string]interface{}
				if err := json.Unmarshal([]byte(rulesTag), &tagRules); err == nil {
					// 如果解析成功，将标签中的规则添加到现有规则中
					rules = append(rules, tagRules...)
				}
			}
			// 优先从json标签获取字段名
			fieldName := jsonTag
			if fieldName == "" {
				fieldName = str.ToLowerFirst(field.Name)
			} else {
				// 处理json标签中的选项，如 "name,omitempty"
				if commaIndex := strings.Index(fieldName, ","); commaIndex != -1 {
					fieldName = fieldName[:commaIndex]
				}
			}

			// 从component标签获取UI组件配置
			var component map[string]interface{}
			componentTag := field.Tag.Get("component")
			if componentTag != "" {
				// 解析JSON格式的component标签
				// 例如: component:"{\"type\":\"select\",\"filterable\":true,\"options\":[{\"label\":\"mysql\",\"value\":\"mysql\"}]}"
				_=json.Unmarshal([]byte(componentTag), &component)
			}

			fields = append(fields,
				types.ComponentFormField{
					Name:         fieldName,
					Type:         typeName,
					DefaultValue: defaultValue,
					Label:        label,
					Desc:         desc,
					Rules:        rules,
					Validate:     validate,
					Fields:       subFields,
					Component:    component,
				})
		}
	}
	return fields
}
