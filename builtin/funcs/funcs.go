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
	"text/template"
)

var TemplateFuncMap = template.FuncMap{}

func init() {
	TemplateFuncMap["escape"] = func(s string) string {
		var replacer = strings.NewReplacer(
			"\\", "\\\\", // 反斜杠
			"\"", "\\\"", // 双引号
			"\n", "\\n", // 换行符
			"\r", "\\r", // 回车符
			"\t", "\\t", // 制表符
		)
		return replacer.Replace(s)
	}
}
