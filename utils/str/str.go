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

// Package str provides utility functions for string manipulation and processing.
// It includes functions for template execution, string formatting, and various
// string operations commonly used in the RuleGo project.
// Key features:
// - ExecuteTemplate: Replaces ${} variables in string templates
// - SprintfDict: Formats strings using a dictionary for variable substitution
// - ToString: Converts various types to string representations
// - Random string generation functions
// - String manipulation utilities (e.g., TrimQuotes, IsEmpty)
//
// This package is designed to simplify string-related operations throughout
// the RuleGo codebase, providing a consistent and efficient way to handle
// string processing tasks.
package str

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
)

const varPatternLeft = "${"
const varPatternRight = "}"

func init() {
	//设置随机种子
	rand.Seed(time.Now().UnixNano())
}

// 正则表达式匹配 ${aa} 或 ${aa.bb}
var tplVarRegex = regexp.MustCompile(`\$\{ *([^}]+) *\}`)

// ExecuteTemplate 替换字符串模板中的${}变量
// original是一个字符串，包含${key}形式的变量占位符。支持多级变量如：${key.subKey}
// Example: ExecuteTemplate("Hello,${name}",map[string]string{"name":"Alice"}). return "Hello,Alice!".
// 如果没匹配到变量，则保留原样
func ExecuteTemplate(original string, dict map[string]interface{}) string {
	// 使用正则表达式进行替换
	replaced := tplVarRegex.ReplaceAllStringFunc(original, func(s string) string {
		// 提取键名
		matches := tplVarRegex.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s // 如果没有匹配到，返回原字符串
		}
		v := maps.Get(dict, strings.TrimSpace(matches[1]))
		if v == nil {
			return s
		} else {
			return ToString(v)
		}
	})
	return replaced
}

// SprintfDict 根据pattern和dict格式化字符串。
// SprintfDict 替换字符串模板中的${}变量
// original是一个字符串，包含${key}形式的变量占位符。不支持多级变量。
// Example: SprintfDict("Hello,${name}",map[string]string{"name":"Alice"}). return "Hello,Alice!".
// 如果没匹配到变量，则保留原样
func SprintfDict(original string, dict map[string]string) string {
	// 使用正则表达式进行替换
	replaced := tplVarRegex.ReplaceAllStringFunc(original, func(s string) string {
		// 提取键名
		matches := tplVarRegex.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s // 如果没有匹配到，返回原字符串
		}
		result, ok := dict[strings.TrimSpace(matches[1])]
		if !ok {
			return s
		} else {
			return result
		}
	})
	return replaced
}

const randomStrOptions = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const randomStrOptionsLen = len(randomStrOptions)

// RandomStr 创建指定长度的随机字符
func RandomStr(num int) string {
	var builder strings.Builder
	for i := 0; i < num; i++ {
		builder.WriteByte(randomStrOptions[rand.Intn(randomStrOptionsLen)])
	}
	return builder.String()
}

// ToString input的值转成字符串,忽略错误
func ToString(input interface{}) string {
	v, _ := ToStringMaybeErr(input)
	return v
}

// ToStringMaybeErr input的值转成字符串
func ToStringMaybeErr(input interface{}) (string, error) {
	if input == nil {
		return "", nil
	}
	switch v := input.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		ft := input.(float64)
		return strconv.FormatFloat(ft, 'f', -1, 64), nil
	case float32:
		ft := input.(float32)
		return strconv.FormatFloat(float64(ft), 'f', -1, 32), nil
	case int:
		return strconv.Itoa(v), nil
	case uint:
		return strconv.Itoa(int(v)), nil
	case int8:
		return strconv.Itoa(int(v)), nil
	case uint8:
		return strconv.Itoa(int(v)), nil
	case int16:
		return strconv.Itoa(int(v)), nil
	case uint16:
		return strconv.Itoa(int(v)), nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case uint32:
		return strconv.Itoa(int(v)), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case []byte:
		return string(v), nil
	case fmt.Stringer:
		return v.String(), nil
	case error:
		return v.Error(), nil
	case map[interface{}]interface{}:
		// 转换为 map[string]interface{}
		convertedInput := make(map[string]interface{})
		for k, value := range v {
			convertedInput[fmt.Sprintf("%v", k)] = value
		}
		if newValue, err := json.Marshal(convertedInput); err == nil {
			return string(newValue), nil
		} else {
			return "", err
		}
	default:
		if newValue, err := json.Marshal(input); err == nil {
			return string(newValue), nil
		} else {
			return "", err
		}
	}
}

// ToStringMapString 把interface类型 转 map[string]string类型
func ToStringMapString(input interface{}) map[string]string {
	var output = map[string]string{}

	switch v := input.(type) {
	case map[string]string:
		return v
	case map[string]interface{}:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case map[interface{}]string:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case map[interface{}]interface{}:
		for k, val := range v {
			output[ToString(k)] = ToString(val)
		}
		return output
	case string:
		_ = json.Unmarshal([]byte(v), &output)
		return output
	default:
		return output
	}
}

// CheckHasVar 检查字符串是否有占位符
func CheckHasVar(str string) bool {
	return strings.Contains(str, "${") && strings.Contains(str, "}")
}

// ConvertDollarPlaceholder 转postgres风格占位符
func ConvertDollarPlaceholder(sql, dbType string) string {
	if dbType == "postgres" {
		n := 1
		for strings.Contains(sql, "?") {
			sql = strings.Replace(sql, "?", fmt.Sprintf("$%d", n), 1)
			n++
		}
	}
	return sql
}

// RemoveBraces A function that takes a string with ${} and returns a string without them
func RemoveBraces(s string) string {
	// Create a new empty string
	result := ""
	// Loop through each character in the input string
	for i := 0; i < len(s); i++ {
		// Get the current character
		c := s[i]
		// If the character is $, check the next character
		if c == '$' && i+1 < len(s) {
			// If the next character is {, skip it and move to the next one
			if s[i+1] == '{' {
				i++
				continue
			}
		}
		// If the character is }, skip it and move to the next one
		if c == '}' {
			continue
		}
		// If the character is a space, skip it and move to the next one
		if c == ' ' {
			continue
		}
		// Otherwise, append the character to the result string
		result += string(c)
	}
	// Return the result string
	return result
}

// ToLowerFirst 首字母转小写
func ToLowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// ParseVarsWithBraces 解析字符串中的变量，返回变量名切片，例如：${vars.name} -> [name]
func ParseVarsWithBraces(varPrefix, str string) []string {
	var regexpCompile = regexp.MustCompile(`\$\{` + varPrefix + `\.([^\}]+)\}`)
	// 找到所有匹配的变量
	return parseVars(regexpCompile.FindAllStringSubmatch(str, -1))
}

// ParseVars 解析字符串中的变量，返回变量名切片，例如：vars.name -> [name]
func ParseVars(varPrefix, str string) []string {
	var regexpCompile = regexp.MustCompile(varPrefix + `\.(\w+)`)
	// 找到所有匹配的变量
	return parseVars(regexpCompile.FindAllStringSubmatch(str, -1))
}

func parseVars(matches [][]string) []string {
	var vars = make(map[string]struct{})
	for _, match := range matches {
		// match[1] 是去掉 ${vars.} 后的变量名
		vars[match[1]] = struct{}{}
	}
	// 将 map 中的变量名转换为切片
	var result []string
	for varName := range vars {
		result = append(result, varName)
	}
	return result
}

// Contains 检查切片中是否包含元素
func Contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
