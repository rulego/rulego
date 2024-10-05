/*
 * Copyright 2024 The RuleGo Authors.
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
	"strings"
	"sync"
)

// TemplateFuncMap 内置模板函数
var TemplateFuncMap funcMap

// UdfMap 内置Js用户函数
var UdfMap funcMap

func init() {
	TemplateFuncMap.Register("escape", func(s string) string {
		var replacer = strings.NewReplacer(
			"\\", "\\\\", // 反斜杠
			"\"", "\\\"", // 双引号
			"\n", "\\n", // 换行符
			"\r", "\\r", // 回车符
			"\t", "\\t", // 制表符
		)
		return replacer.Replace(s)
	})
}

type funcMap struct {
	v map[string]any
	sync.RWMutex
}

func (x *funcMap) Register(name string, value any) {
	x.Lock()
	defer x.Unlock()
	if x.v == nil {
		x.v = make(map[string]any)
	}
	x.v[name] = value
}

func (x *funcMap) RegisterAll(values map[string]any) {
	x.Lock()
	defer x.Unlock()
	if x.v == nil {
		x.v = make(map[string]any)
	}
	for k, v := range values {
		x.v[k] = v
	}
}

func (x *funcMap) UnRegister(name string) {
	x.Lock()
	defer x.Unlock()
	if x.v != nil {
		delete(x.v, name)
	}
}

func (x *funcMap) Get(name string) (any, bool) {
	x.RLock()
	defer x.RUnlock()
	if x.v != nil {
		f, ok := x.v[name]
		return f, ok
	}
	return nil, false
}

func (x *funcMap) GetAll() map[string]any {
	x.RLock()
	defer x.RUnlock()
	if x.v == nil {
		return nil
	}
	cp := make(map[string]any)
	for k, v := range x.v {
		cp[k] = v
	}
	return cp
}

func (x *funcMap) Names() []string {
	x.RLock()
	defer x.RUnlock()
	var keys = make([]string, 0, len(x.v))
	for k := range x.v {
		keys = append(keys, k)
	}
	return keys
}
