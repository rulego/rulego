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
	"encoding/json"
	"fmt"
	"testing"
)

// 测试用例结构体
type TestCase struct {
	Name     string
	Data     string
	Schema   string
	Expected error
}

func TestValidateData(t *testing.T) {
	testCases := []TestCase{
		{
			Name: "Valid Data",
			Data: `{
				"name": "John Doe",
				"age": 30,
				"is_student": false,
				"scores": [85, 90, 78],
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}`,
			Schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"},
					"is_student": {"type": "boolean"},
					"scores": {"type": "array"},
					"address": {"type": "object"}
				},
				"required": ["name", "age"]
			}`,
			Expected: nil,
		},
		{
			Name: "Missing Required Field",
			Data: `{
				"age": 30,
				"is_student": false,
				"scores": [85, 90, 78],
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}`,
			Schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"},
					"is_student": {"type": "boolean"},
					"scores": {"type": "array"},
					"address": {"type": "object"}
				},
				"required": ["name", "age"]
			}`,
			Expected: fmt.Errorf("missing required field: name"),
		},
		{
			Name: "Type Mismatch",
			Data: `{
				"name": "John Doe",
				"age": "thirty",
				"is_student": false,
				"scores": [85, 90, 78],
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}`,
			Schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"},
					"is_student": {"type": "boolean"},
					"scores": {"type": "array"},
					"address": {"type": "object"}
				},
				"required": ["name", "age"]
			}`,
			Expected: fmt.Errorf("field age: expected integer, got string"),
		},
		{
			Name: "Extra Fields",
			Data: `{
				"name": "John Doe",
				"age": 30,
				"is_student": false,
				"scores": [85, 90, 78],
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				},
				"extra_field": "some value"
			}`,
			Schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"},
					"is_student": {"type": "boolean"},
					"scores": {"type": "array"},
					"address": {"type": "object"}
				},
				"required": ["name", "age"]
			}`,
			Expected: nil, // Extra fields are allowed by default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var data map[string]interface{}
			err := json.Unmarshal([]byte(tc.Data), &data)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON data: %v", err)
			}

			var schema JSONSchema
			err = json.Unmarshal([]byte(tc.Schema), &schema)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON Schema: %v", err)
			}

			err = validateData(data, schema)
			if tc.Expected == nil && err == nil {
				return // Both are nil, test passes
			}
			if tc.Expected == nil && err != nil {
				t.Errorf("Expected no error, got %v", err)
				return
			}
			if tc.Expected != nil && err == nil {
				t.Errorf("Expected error %v, got no error", tc.Expected)
				return
			}
			if tc.Expected != nil && err != nil {
				if tc.Expected.Error() != err.Error() {
					t.Errorf("Expected error %v, got %v", tc.Expected, err)
				}
			}
		})
	}
}
