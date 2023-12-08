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

package reflect

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
	"reflect"
	"strings"
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

	if strings.Contains(strings.ToLower(componentForm.Label), "filter") {
		relationTypes = []string{types.True, types.False, types.Failure}
	} else if strings.Contains(strings.ToLower(componentForm.Label), "switch") {
		relationTypes = []string{}
	}
	componentForm.RelationTypes = &relationTypes
	//如果实现ComponentDefGetter接口，使用接口定义的代替
	if componentDefGetter, ok := component.(types.ComponentDefGetter); ok {
		componentForm = coverComponentForm(componentDefGetter, componentForm)
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
			var defaultValue interface{}
			if configValue.Field(i).CanInterface() {
				defaultValue = configValue.Field(i).Interface()
			}
			label := field.Tag.Get("label")
			desc := field.Tag.Get("desc")
			validate := field.Tag.Get("validate")
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

			fields = append(fields,
				types.ComponentFormField{
					Name:         str.ToLowerFirst(field.Name),
					Type:         typeName,
					DefaultValue: defaultValue,
					Label:        label,
					Desc:         desc,
					Validate:     validate,
					Fields:       subFields,
				})
		}
	}
	return fields
}
