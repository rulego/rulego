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
	"reflect"
	"strconv"
	"strings"

	"github.com/rulego/rulego/api/types/endpoint"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
)

// GetComponentForm to get the form structure of the component
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
	//If implementing the ComponentDefGetter interface, use the interface definition instead
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

// Overrides defined using the ComponentDefGetter interface
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
	if def.RouterForm != nil {
		toComponentForm.RouterForm = def.RouterForm
	}
	toComponentForm.Disabled = def.Disabled

	return toComponentForm
}

// GetComponentConfig retrieves the component configuration field and default values
func GetComponentConfig(component types.Node) (reflect.Type, reflect.StructField, reflect.Value) {
	//component = component.New()
	t := reflect.TypeOf(component)
	if t.Kind() == reflect.Ptr {
		t = t.Elem() // Dereference pointers to retrieve the value pointed
	}

	var configField reflect.StructField
	var ok bool
	var configValue reflect.Value
	if configField, ok = t.FieldByName("config"); !ok {
		if configField, ok = t.FieldByName("Config"); ok {
			v := reflect.ValueOf(component)
			if v.Kind() == reflect.Ptr {
				v = v.Elem() // Dereference pointers to retrieve the value pointed
			}
			configValue = v.FieldByName("Config")
		}
	} else {
		v := reflect.ValueOf(component)
		if v.Kind() == reflect.Ptr {
			v = v.Elem() // Dereference pointers to retrieve the value pointed
		}
		configValue = v.FieldByName("config")
	}
	return t, configField, configValue
}

// GetFields to get the component config field
func GetFields(configField reflect.StructField, configValue reflect.Value) []types.ComponentFormField {
	var fields []types.ComponentFormField
	if configField.Type != nil {
		for i := 0; i < configField.Type.NumField(); i++ {
			field := configField.Type.Field(i)

			// Skip private fields (lowercase)
			if !field.IsExported() {
				continue
			}

			// Check the JSON tag; skip it if it is "-"
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			// Check if squash is needed
			// Prioritize obtaining it from the JSON tag; if the JSON tag is not available, then get it from the MapStructure tag
			tag := field.Tag.Get("json")
			if !strings.Contains(tag, "squash") {
				tag = field.Tag.Get("mapstructure")
			}
			if field.Anonymous && field.Type.Kind() == reflect.Struct && strings.Contains(tag, "squash") {
				var embeddedValue reflect.Value
				if configValue.IsValid() {
					embeddedValue = configValue.Field(i)
				}
				embeddedFields := GetFields(field, embeddedValue)
				fields = append(fields, embeddedFields...)
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
				//If the field type is a structure, then recursively call the GetFields function, passing in the field's type and value objects to obtain information about the subfields
				subFields = GetFields(field, configValue.Field(i))
			}
			var rules []map[string]interface{}
			if required {
				rules = append(rules, map[string]interface{}{
					"required": true,
					"message":  "This field is required",
				})
			}

			// Obtain the validation rule configuration from the rules tab
			rulesTag := field.Tag.Get("rules")
			if rulesTag != "" {
				// Parse the rules tag in JSON format
				// For example: rules:"[{\"required\":true,\"message\":\"Required field\"},{\"min\":1,\"message\":\"Minimum value is 1\"}]"
				var tagRules []map[string]interface{}
				if err := json.Unmarshal([]byte(rulesTag), &tagRules); err == nil {
					// If parsing successfully, add the rules from the tags to the existing rules
					rules = append(rules, tagRules...)
				}
			}
			// Prioritize getting field names from the JSON tag
			fieldName := jsonTag
			if fieldName == "" {
				fieldName = str.ToLowerFirst(field.Name)
			} else {
				// Handling options in JSON tags, such as "name,omitempty"
				if commaIndex := strings.Index(fieldName, ","); commaIndex != -1 {
					fieldName = fieldName[:commaIndex]
				}
			}

			// Retrieves UI component configuration from the component tag
			var component map[string]interface{}
			componentTag := field.Tag.Get("component")
			if componentTag != "" {
				// Parsing component tags in JSON format
				// For example: component:"{\"type\":\"select\",\"filterable\":true,\"options\":[{\"label\":\"mysql\",\"value\":\"mysql\"}]}"
				_ = json.Unmarshal([]byte(componentTag), &component)
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
					Ref:          field.Tag.Get("ref"),
				})
		}
	}
	return fields
}
