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

package schema

import (
	"fmt"
)

// JSONSchema defines the structure of the JSON Schema
type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]FieldSchema `json:"properties"`
	Required   []string               `json:"required"`
}

// CheckFieldIsRequired checks whether the field is in the Required list
func (s JSONSchema) CheckFieldIsRequired(fieldName string) bool {
	for _, requiredField := range s.Required {
		if requiredField == fieldName {
			return true
		}
	}
	return false
}

// FieldSchema defines the schema for individual fields
type FieldSchema struct {
	Type        string                 `json:"type"`        //Type
	Title       string                 `json:"title"`       //Title
	Description string                 `json:"description"` //Description
	Default     interface{}            `json:"default"`     //Default values
	Properties  map[string]FieldSchema `json:"properties"`  // Nested fields
	Required    []string               `json:"required"`    // Required list of nested fields
	Component   map[string]interface{} `json:"component"`   //Front-end form component configuration
}

// Data defines the structure of JSON data
type Data struct {
	Properties map[string]interface{} `json:"properties"`
}

// validateData verifies whether JSON data complies with the JSON Schema
func validateData(data map[string]interface{}, schema JSONSchema) error {
	// Check the required field
	for _, field := range schema.Required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Check the type of each field
	for fieldName, fieldSchema := range schema.Properties {
		if value, ok := data[fieldName]; ok {
			if err := validateFieldType(value, fieldSchema.Type); err != nil {
				return fmt.Errorf("field %s: %v", fieldName, err)
			}
		}
	}

	return nil
}

// validateFieldType Verifies whether the field type matches the Schema definition
func validateFieldType(value interface{}, fieldType string) error {
	switch fieldType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer":
		if _, ok := value.(float64); !ok { // Integers in JSON are usually parsed as float64
			return fmt.Errorf("expected integer, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	default:
		return fmt.Errorf("unsupported type: %s", fieldType)
	}
	return nil
}
