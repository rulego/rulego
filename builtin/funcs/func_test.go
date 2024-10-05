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
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestBuiltinFunc(t *testing.T) {

	t.Run("TestTemplateFuncMap", func(t *testing.T) {
		TemplateFuncMap.RegisterAll(map[string]any{
			"test": func(a int) int {
				return a + 1
			},
		})
		TemplateFuncMap.Register("test2", func(a int) int {
			return a + 1
		})
		cp := TemplateFuncMap.GetAll()
		_, ok := cp["test"]
		assert.True(t, ok)
		_, ok = cp["test2"]
		assert.True(t, ok)

		TemplateFuncMap.UnRegister("test")
		_, ok = TemplateFuncMap.Get("test")
		assert.False(t, ok)

		_, ok = TemplateFuncMap.Get("test2")
		assert.True(t, ok)

		TemplateFuncMap.UnRegister("test2")
		_, ok = TemplateFuncMap.Get("test2")
		assert.False(t, ok)
	})

	t.Run("TestUdfFuncMap", func(t *testing.T) {
		UdfMap.RegisterAll(map[string]any{
			"test": func(a int) int {
				return a + 1
			},
		})
		UdfMap.Register("test2", func(a int) int {
			return a + 1
		})
		cp := UdfMap.GetAll()
		_, ok := cp["test"]
		assert.True(t, ok)
		_, ok = cp["test2"]
		assert.True(t, ok)

		names := UdfMap.Names()
		assert.Equal(t, len(UdfMap.GetAll()), len(names))
		var checkName = false
		for _, name := range names {
			if name == "test" || name == "test2" {
				checkName = true
			}
		}
		assert.True(t, checkName)

		UdfMap.UnRegister("test")
		_, ok = UdfMap.Get("test")
		assert.False(t, ok)

		_, ok = UdfMap.Get("test2")
		assert.True(t, ok)

		UdfMap.UnRegister("test2")
		_, ok = UdfMap.Get("test2")
		assert.False(t, ok)
	})

}
