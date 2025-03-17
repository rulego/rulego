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

// JSONSchema 定义了 JSON Schema 的结构
type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]FieldSchema `json:"properties"`
	Required   []string               `json:"required"`
}

// CheckFieldIsRequired 检查字段是否在 Required 列表中
func (s JSONSchema) CheckFieldIsRequired(fieldName string) bool {
	for _, requiredField := range s.Required {
		if requiredField == fieldName {
			return true
		}
	}
	return false
}

// FieldSchema 定义了单个字段的 Schema
type FieldSchema struct {
	Type        string                 `json:"type"`        //类型
	Title       string                 `json:"title"`       //标题
	Description string                 `json:"description"` //描述
	Default     interface{}            `json:"default"`     //默认值
	Properties  map[string]FieldSchema `json:"properties"`  // 嵌套字段
	Required    []string               `json:"required"`    // 嵌套字段的必填列表
	Component   map[string]interface{} `json:"component"`   //前端表单组件配置
}

// Data 定义了 JSON 数据的结构
type Data struct {
	Properties map[string]interface{} `json:"properties"`
}

// validateData 验证 JSON 数据是否符合 JSON Schema
func validateData(data map[string]interface{}, schema JSONSchema) error {
	// 检查 required 字段
	for _, field := range schema.Required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// 检查每个字段的类型
	for fieldName, fieldSchema := range schema.Properties {
		if value, ok := data[fieldName]; ok {
			if err := validateFieldType(value, fieldSchema.Type); err != nil {
				return fmt.Errorf("field %s: %v", fieldName, err)
			}
		}
	}

	return nil
}

// validateFieldType 验证字段的类型是否符合 Schema 定义
func validateFieldType(value interface{}, fieldType string) error {
	switch fieldType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer":
		if _, ok := value.(float64); !ok { // JSON 中的整数通常被解析为 float64
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
