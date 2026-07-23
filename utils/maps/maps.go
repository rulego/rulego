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

// Get to get fields in map or struct, supporting nested structures such as fieldName.subFieldName.xx
// Supported types: map[string]interface{}, map[string]string, structs (access fields via reflection)
// Field matching priority: JSON tag > Field name (case-insensitive)
// If the field does not exist, return nil
func Get(input interface{}, fieldName string) interface{} {
	// According to the "." Split fieldName
	fields := strings.Split(fieldName, ".")
	var result interface{}
	result = input

	// Traverse each subfield
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
				// Fallback: Try to use the remaining part as the full key lookup (supports multi-level keys for flat storage)
				// For example: "llm.providers.default.base_url" is stored in the map, and when accessing "llm.providers.default.base_url"
				// First, try nested access to map["llm"]["providers"]..., and if failed, fallback to map["llm.providers.default.base_url"]
				remainingKey := strings.Join(fields[i:], ".")
				if val, ok := v[remainingKey]; ok {
					return val
				}
				return nil
			}
		default:
			// Try to access the structure field by reflecting
			val := getStructField(result, field)
			if val == nil {
				return nil
			}
			result = val
		}
	}
	return result
}

// getStructField obtains the structure field value by reflecting
// Supports matching JSON tags with field names (case-insensitive)
func getStructField(obj interface{}, fieldName string) interface{} {
	if obj == nil {
		return nil
	}

	val := reflect.ValueOf(obj)
	// Handles pointer types
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	// Only handle structure types
	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	// Prioritize matching JSON tags, then match field names (case-insensitive)
	fieldNameLower := strings.ToLower(fieldName)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Check the JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			// Handle the json:"name,omitempty" format
			jsonName := strings.Split(jsonTag, ",")[0]
			if jsonName == fieldName || jsonName == fieldNameLower {
				fieldVal := val.Field(i)
				if fieldVal.CanInterface() {
					return fieldVal.Interface()
				}
			}
		}

		// Check field names (case-insensitive)
		if strings.ToLower(field.Name) == fieldNameLower {
			fieldVal := val.Field(i)
			if fieldVal.CanInterface() {
				return fieldVal.Interface()
			}
		}
	}

	return nil
}
