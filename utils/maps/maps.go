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

// Package maps provides utility functions for working with maps and structs.
// It includes functions for converting between maps and structs, as well as
// retrieving nested values from maps using dot notation.
//
// This package is particularly useful when dealing with dynamic data structures
// or when working with configuration data that needs to be converted between
// different formats.
//
// Key features:
// - Map2Struct: Converts a map to a struct using reflection
// - Get: Retrieves nested values from maps using dot notation
// - Support for weakly typed input when converting maps to structs
// - Handling of time.Duration conversions from string representations
//
// Usage example:
//
//	input := map[string]interface{}{
//		"name": "John Doe",
//		"age":  30,
//		"address": map[string]interface{}{
//			"street": "123 Main St",
//			"city":   "Anytown",
//		},
//	}
//
//	// Retrieve a nested value
//	city := maps.Get(input, "address.city")
//
//	// Convert map to struct
//	type Person struct {
//		Name    string
//		Age     int
//		Address struct {
//			Street string
//			City   string
//		}
//	}
//	var person Person
//	err := maps.Map2Struct(input, &person)
package maps

import (
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
)

// Map2Struct Decode takes an input structure and uses reflection to translate it to
// the output structure. output must be a pointer to a map or struct.
func Map2Struct(input interface{}, output interface{}) error {
	cfg := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
		),
		WeaklyTypedInput: true,
		Metadata:         nil,
		Result:           output,
		ZeroFields:       true,
		TagName:          "json",
	}
	if d, err := mapstructure.NewDecoder(cfg); err != nil {
		return err
	} else if err := d.Decode(input); err != nil {
		return err
	}
	return nil
}

// Get 获取map或struct中的字段，支持嵌套结构获取，例如fieldName.subFieldName.xx
// 支持的类型：map[string]interface{}、map[string]string、结构体（通过反射访问字段）
// 字段匹配优先级：JSON tag > 字段名（不区分大小写）
// 如果字段不存在，返回nil
func Get(input interface{}, fieldName string) interface{} {
	// 按照"."分割fieldName
	fields := strings.Split(fieldName, ".")
	var result interface{}
	result = input

	// 遍历每个子字段
	for i, field := range fields {
		switch v := result.(type) {
		case map[string]interface{}:
			if val, ok := v[field]; ok {
				result = val
			} else {
				return nil
			}
		case map[string]string:
			if val, ok := v[field]; ok {
				result = val
			} else {
				// Fallback: 尝试用剩余部分作为完整 key 查找（支持扁平存储的多级 key）
				// 例如：map 中存储了 "llm.providers.default.base_url"，访问 "llm.providers.default.base_url" 时
				// 先尝试嵌套访问 map["llm"]["providers"]...，失败后 fallback 到 map["llm.providers.default.base_url"]
				remainingKey := strings.Join(fields[i:], ".")
				if val, ok := v[remainingKey]; ok {
					return val
				}
				return nil
			}
		default:
			// 尝试通过反射访问结构体字段
			val := getStructField(result, field)
			if val == nil {
				return nil
			}
			result = val
		}
	}
	return result
}

// getStructField 通过反射获取结构体字段值
// 支持 JSON tag 和字段名匹配（不区分大小写）
func getStructField(obj interface{}, fieldName string) interface{} {
	if obj == nil {
		return nil
	}

	val := reflect.ValueOf(obj)
	// 处理指针类型
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	// 只处理结构体类型
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	// 优先匹配 JSON tag，然后匹配字段名（不区分大小写）
	fieldNameLower := strings.ToLower(fieldName)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// 检查 JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			// 处理 json:"name,omitempty" 格式
			jsonName := strings.Split(jsonTag, ",")[0]
			if jsonName == fieldName || jsonName == fieldNameLower {
				fieldVal := val.Field(i)
				if fieldVal.CanInterface() {
					return fieldVal.Interface()
				}
			}
		}

		// 检查字段名（不区分大小写）
		if strings.ToLower(field.Name) == fieldNameLower {
			fieldVal := val.Field(i)
			if fieldVal.CanInterface() {
				return fieldVal.Interface()
			}
		}
	}

	return nil
}
