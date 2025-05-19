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
	}
	if d, err := mapstructure.NewDecoder(cfg); err != nil {
		return err
	} else if err := d.Decode(input); err != nil {
		return err
	}
	return nil
}

// Get 获取map中的字段，支持嵌套结构获取，例如fieldName.subFieldName.xx
// 嵌套类型必须是map[string]interface{}
// 如果字段不存在，返回nil
func Get(input interface{}, fieldName string) interface{} {
	// 按照"."分割fieldName
	fields := strings.Split(fieldName, ".")
	var result interface{}
	result = input

	// 遍历每个子字段
	for _, field := range fields {
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
				return nil
			}
		default:
			return nil
		}
	}
	return result
}
