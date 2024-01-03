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
)

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}

func TestMap2Struct(t *testing.T) {
	m := make(map[string]interface{})
	m["userName"] = "lala"
	m["Age"] = float64(5)
	m["Address"] = Address{"test"}
	var user User
	_ = Map2Struct(m, &user)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, 5, user.Age)
	assert.Equal(t, "lala", user.Username)
	assert.Equal(t, "test", user.Address.Detail)
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
		{"friends", []string{"Bob", "Charlie"}},
		{"hobbies", nil},
		{"address.zipcode", nil},
	}
	// 遍历每个测试用例，调用GetFieldValue函数，断言结果是否与期望的值相等
	for _, c := range cases {
		actual := Get(value, c.fieldName)
		assert.Equal(t, c.expected, actual)
	}
}
