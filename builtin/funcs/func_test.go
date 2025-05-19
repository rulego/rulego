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

package funcs

import (
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func TestBuiltinFunc(t *testing.T) {
	t.Run("TestEscapeFunc", func(t *testing.T) {
		escapeFunc, ok := TemplateFunc.Get("escape")
		assert.True(t, ok)
		fn, ok := escapeFunc.(func(string) string)
		assert.True(t, ok)

		assert.Equal(t, "hello\\\\world", fn("hello\\world"))
		assert.Equal(t, "hello\\\"world\\\"", fn("hello\"world\""))
		assert.Equal(t, "hello\\nworld", fn("hello\nworld"))
		assert.Equal(t, "hello\\rworld", fn("hello\rworld"))
		assert.Equal(t, "hello\\tworld", fn("hello\tworld"))
		assert.Equal(t, "complex\\\\\\\"\\n\\r\\tstring", fn("complex\\\"\n\r\tstring"))
	})

	t.Run("TestTemplateFuncMap", func(t *testing.T) {
		TemplateFunc.RegisterAll(map[string]any{
			"test": func(a int) int {
				return a + 1
			},
		})
		TemplateFunc.Register("test2", func(a int) int {
			return a + 1
		})
		cp := TemplateFunc.GetAll()
		_, ok := cp["test"]
		assert.True(t, ok)
		_, ok = cp["test2"]
		assert.True(t, ok)

		TemplateFunc.UnRegister("test")
		_, ok = TemplateFunc.Get("test")
		assert.False(t, ok)

		_, ok = TemplateFunc.Get("test2")
		assert.True(t, ok)

		TemplateFunc.UnRegister("test2")
		_, ok = TemplateFunc.Get("test2")
		assert.False(t, ok)
	})

	t.Run("TestUdfFuncMap", func(t *testing.T) {
		ScriptFunc.RegisterAll(map[string]any{
			"test": func(a int) int {
				return a + 1
			},
		})
		ScriptFunc.Register("test2", func(a int) int {
			return a + 1
		})
		cp := ScriptFunc.GetAll()
		_, ok := cp["test"]
		assert.True(t, ok)
		_, ok = cp["test2"]
		assert.True(t, ok)

		names := ScriptFunc.Names()
		var num = 0
		for range cp {
			num++
		}
		assert.Equal(t, num, len(names))
		var checkName = false
		for _, name := range names {
			if name == "test" || name == "test2" {
				checkName = true
			}
		}
		assert.True(t, checkName)

		ScriptFunc.UnRegister("test")
		_, ok = ScriptFunc.Get("test")
		assert.False(t, ok)

		_, ok = ScriptFunc.Get("test2")
		assert.True(t, ok)

		ScriptFunc.UnRegister("test2")
		_, ok = ScriptFunc.Get("test2")
		assert.False(t, ok)
	})

	t.Run("TestFuncMap", func(t *testing.T) {
		var testMap = NewFuncMap[string]()
		testMap.RegisterAll(map[string]string{
			"test1": "test1",
			"test2": "test2",
		})
		for k, v := range testMap.GetAll() {
			assert.Equal(t, k, v)
		}
		assert.Equal(t, len(testMap.Names()), 2)
	})
}
