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

package str

import (
	"github.com/rulego/rulego/test/assert"
	"reflect"
	"testing"
)

func TestSprintfDict(t *testing.T) {
	// 创建一个字典
	dict := map[string]string{
		"name": "Alice",
		"age":  "18",
	}
	// 使用SprintfDict来格式化字符串
	s := SprintfDict("Hello, ${name}. You are ${age} years old.", dict)
	assert.Equal(t, "Hello, Alice. You are 18 years old.", s)
}

func TestSprintfVar(t *testing.T) {
	// 创建一个字典
	dict := map[string]string{
		"name": "Alice",
		"age":  "18",
	}
	// 使用SprintfDict来格式化字符串
	s := SprintfVar("Hello, ${global.name}. You are ${global.age} years old.", "global.", dict)
	assert.Equal(t, "Hello, Alice. You are 18 years old.", s)
}

func TestToString(t *testing.T) {
	var x interface{}
	x = 123 // 赋值为整数
	assert.Equal(t, "123", ToString(x))
	x = "this is test"
	assert.Equal(t, "this is test", ToString(x))
	x = []byte("this is test")
	assert.Equal(t, "this is test", ToString(x))

	x = User{Username: "lala", Age: 25}
	assert.Equal(t, "{\"Username\":\"lala\",\"Age\":25,\"Address\":{\"Detail\":\"\"}}", ToString(x))

	x = map[string]string{
		"name": "lala",
	}
	assert.Equal(t, "{\"name\":\"lala\"}", ToString(x))

}
func TestToStringMapString(t *testing.T) {
	var x interface{}
	x = map[string]interface{}{
		"name": "lala",
		"age":  5,
		"user": User{},
	}
	strMap := ToStringMapString(x)
	ageV := strMap["age"]
	ageType := reflect.TypeOf(&ageV).Elem()
	assert.Equal(t, reflect.String, ageType.Kind())

	strMap2 := ToStringMapString(strMap)
	ageV = strMap2["age"]
	ageType = reflect.TypeOf(&ageV).Elem()
	assert.Equal(t, reflect.String, ageType.Kind())
}

type User struct {
	Username string
	Age      int
	Address  Address
}
type Address struct {
	Detail string
}
