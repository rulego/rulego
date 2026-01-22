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

package maps

import (
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

type User struct {
	Username string
	Age      int
	Address  Address
	Hobbies  []string
}

type Address struct {
	Detail string
}

func TestMap2Struct(t *testing.T) {
	m := make(map[string]interface{})
	m["userName"] = "lala"
	m["Age"] = float64(5)
	m["Address"] = Address{"test"}
	m["Hobbies"] = []string{"c"}
	var user User
	user.Hobbies = []string{"a", "b"}
	_ = Map2Struct(m, &user)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, 5, user.Age)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, "test", user.Address.Detail)
	assert.Equal(t, 1, len(user.Hobbies))

	// Test with time.Duration string
	type Config struct {
		Timeout time.Duration
	}
	configMap := map[string]interface{}{
		"Timeout": "5s",
	}
	var cfg Config
	err := Map2Struct(configMap, &cfg)
	assert.Nil(t, err)
	assert.Equal(t, 5*time.Second, cfg.Timeout)

	// Test with invalid time.Duration string
	configMapInvalid := map[string]interface{}{
		"Timeout": "5invalid",
	}
	var cfgInvalid Config
	err = Map2Struct(configMapInvalid, &cfgInvalid)
	assert.NotNil(t, err)

	// Test with non-pointer output
	var userNonPointer User
	err = Map2Struct(m, userNonPointer) // Pass non-pointer
	assert.NotNil(t, err)               // Expect error

	// Test with nil input
	var userNilInput User
	err = Map2Struct(nil, &userNilInput)
	assert.Nil(t, err) // mapstructure might not error on nil input, but result in zero struct
	assert.Equal(t, "", userNilInput.Username)

	// Test with input that is not a map
	var userNotMapInput User
	err = Map2Struct("not a map", &userNotMapInput)
	assert.NotNil(t, err)
}

// TestGet 测试Get函数
func TestGet(t *testing.T) {
	// 定义一个map，包含嵌套结构
	value := map[string]interface{}{
		"name": "Alice",
		"age":  25,
		"address": map[string]interface{}{
			"city":    "Beijing",
			"country": "China",
			"detail":  nil,
		},
		"friends": []string{"Bob", "Charlie"},
	}
	// 定义一些测试用例，包含字段名和期望的值
	cases := []struct {
		fieldName string
		expected  interface{}
	}{
		{"name", "Alice"},
		{"age", 25},
		{"address.city", "Beijing"},
		{"address.country", "China"},
		{"address.detail", nil},
		{"address.detail.x", nil},
		{"friends", []string{"Bob", "Charlie"}},
		{"hobbies", nil},
		{"address.zipcode", nil},
	}
	// 遍历每个测试用例，调用GetFieldValue函数，断言结果是否与期望的值相等
	for _, c := range cases {
		actual := Get(value, c.fieldName)
		assert.Equal(t, c.expected, actual)
	}

	value2 := map[string]string{
		"name": "Alice",
	}
	// 定义一些测试用例，包含字段名和期望的值
	cases2 := []struct {
		fieldName string
		expected  interface{}
	}{
		{"name", "Alice"},
	}
	// 遍历每个测试用例，调用GetFieldValue函数，断言结果是否与期望的值相等
	for _, c := range cases2 {
		actual := Get(value2, c.fieldName)
		assert.Equal(t, c.expected, actual)
	}

	// Test with non-map input
	assert.Nil(t, Get("not a map", "field"))

	// Test with empty field name
	assert.Equal(t, nil, Get(value, ""))

	// Test with field name containing only dots
	assert.Nil(t, Get(value, "..."))

	// Test with map[string]interface{} containing non-string key
	// Get function expects map[string]interface{} or map[string]string.
	// If we have map[interface{}]interface{}, it won't be processed correctly by current Get.
	// This is a limitation of the current Get implementation rather than a bug to fix here.
	// We'll test that it returns nil as expected for such cases if a field is accessed.
	mapWithIntKey := map[interface{}]interface{}{
		1: "one",
	}
	assert.Nil(t, Get(mapWithIntKey, "1"))
}

func TestMap2StructWithJsonTag(t *testing.T) {
	// Case 3: key matches json tag, but field name is completely different
	// This is the real test case.
	type ComplexJsonTagStruct struct {
		MyField string `json:"completely_different_name"`
	}
	m3 := map[string]interface{}{
		"completely_different_name": "value3",
	}
	var s3 ComplexJsonTagStruct
	_ = Map2Struct(m3, &s3)

	// 如果 Map2Struct 不支持 json tag，这里应该失败（为空）
	// 如果支持，这里应该是 value3
	assert.Equal(t, "value3", s3.MyField)
}

func TestMap2StructWithoutJsonTag(t *testing.T) {
	type NoTagStruct struct {
		SimpleField string
		CamelCase   string
	}

	// Case 1: Exact match
	m1 := map[string]interface{}{
		"SimpleField": "value1",
		"CamelCase":   "value2",
	}
	var s1 NoTagStruct
	_ = Map2Struct(m1, &s1)
	assert.Equal(t, "value1", s1.SimpleField)
	assert.Equal(t, "value2", s1.CamelCase)

	// Case 2: Case insensitive match
	m2 := map[string]interface{}{
		"simpleField": "value1",
		"camelCase":   "value2",
	}
	var s2 NoTagStruct
	_ = Map2Struct(m2, &s2)
	assert.Equal(t, "value1", s2.SimpleField)
	assert.Equal(t, "value2", s2.CamelCase)
}

func TestMap2StructSquash(t *testing.T) {
	type Base struct {
		BaseField string `json:"base_field"`
	}

	// Test json squash
	type SquashedStructJson struct {
		Base  `json:",squash"`
		Other string `json:"other"`
	}

	input := map[string]interface{}{
		"base_field": "base_value",
		"other":      "other_value",
	}

	var s1 SquashedStructJson
	err := Map2Struct(input, &s1)
	assert.Nil(t, err)
	assert.Equal(t, "base_value", s1.BaseField)
	assert.Equal(t, "other_value", s1.Other)
}
