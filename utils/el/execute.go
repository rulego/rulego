/*
 * Copyright 2026 The RuleGo Authors.
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

package el

import (
	"regexp"
	"strings"

	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// tplVarRegex 匹配 ${aa} 或 ${aa.bb} 占位符，预编译提高性能
var tplVarRegex = regexp.MustCompile(`\$\{ *([^}]+) *\}`)

// ExecuteTemplate 替换字符串模板中的${}变量，支持多级变量如：${key.subKey}
// Example: ExecuteTemplate("Hello,${name}",map[string]any{"name":"Alice"}) return "Hello,Alice".
// 如果没匹配到变量，则保留原样。
// 适用于偶发调用的一次性渲染；高频路径应使用 NewTemplate 预解析后复用。
func ExecuteTemplate(tmpl string, vars map[string]any) string {
	// 快速检查：如果字符串中没有模板变量，直接返回
	if !strings.Contains(tmpl, "${") {
		return tmpl
	}

	// 使用预编译的正则表达式进行替换
	return tplVarRegex.ReplaceAllStringFunc(tmpl, func(s string) string {
		// 提取键名
		start := strings.Index(s, "{") + 1
		end := strings.LastIndex(s, "}")
		if start <= 0 || end <= start {
			return s
		}

		key := strings.TrimSpace(s[start:end])
		v := maps.Get(vars, key)
		if v == nil {
			return s
		}
		return str.ToString(v)
	})
}
