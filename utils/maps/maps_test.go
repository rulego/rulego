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
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
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
