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

// VarPrefix template variable prefix
const VarPrefix = "${"

// VarSuffix template variable suffix
const VarSuffix = "}"

func init() {
	//Set random seeds
	rand.Seed(time.Now().UnixNano())
}

// Regular expressions match ${aa} or ${aa.bb}
// Precompiled template variable regular expressions improve performance
var tplVarRegex = regexp.MustCompile(`\$\{ *([^}]+) *\}`)

// ExecuteTemplate replaces the ${} variable in the string template
// original is a string containing a placeholder for a variable in the form ${key}. Supports multi-level variables such as: ${key.subKey}
// Example: ExecuteTemplate("Hello,${name}",map[string]string{"name":"Alice"}). return "Hello,Alice!".
// If the variable is not matched, it is kept as is
// Deprecated: Use github.com/rulego/rulego/utils/el.NewTemplate instead.
// This function will be removed in a future version.
func ExecuteTemplate(original string, dict map[string]interface{}) string {
	// Quick check: If there are no template variables in the string, return it directly
	if !strings.Contains(original, "${") {
		return original
	}

	// Replace with precompiled regular expressions
	return tplVarRegex.ReplaceAllStringFunc(original, func(s string) string {
		// Key Extraction (Optimization: Reduces Duplicate Regular Matching)
		start := strings.Index(s, "{") + 1
		end := strings.LastIndex(s, "}")
		if start <= 0 || end <= start {
			return s
		}

		key := strings.TrimSpace(s[start:end])
		v := maps.Get(dict, key)
		if v == nil {
			return s
		}
		return ToString(v)
	})
}

// SprintfDict formats strings based on pattern and dict.
// SprintfDict replaces the ${} variable in the string template
// original is a string containing a placeholder for a variable in the form ${key}. Multi-level variables are not supported.
// Example: SprintfDict("Hello,${name}",map[string]string{"name":"Alice"}). return "Hello,Alice!".
// If the variable is not matched, it is kept as is
func SprintfDict(original string, dict map[string]string) string {
	// Replace with regular expressions
	replaced := tplVarRegex.ReplaceAllStringFunc(original, func(s string) string {
		// Extract key names
		matches := tplVarRegex.FindStringSubmatch(s)
		if len(matches) < 2 {
			return s // If no match is found, the original string is returned
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

// RandomStr creates random characters of specified length
func RandomStr(num int) string {
	var builder strings.Builder
	for i := 0; i < num; i++ {
		builder.WriteByte(randomStrOptions[rand.Intn(randomStrOptionsLen)])
	}
	return builder.String()
}

// The value of ToString input is converted to a string, ignoring the error
func ToString(input interface{}) string {
	v, _ := ToStringMaybeErr(input)
	return v
}

// ToStringMaybeErr Input value is converted to a string
func ToStringMaybeErr(input interface{}) (string, error) {
	if input == nil {
		return "", nil
	}

	// Optimized type conversion reduces reflection and memory allocation
	switch v := input.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.Itoa(int(v)), nil
	case int16:
		return strconv.Itoa(int(v)), nil
	case int32:
		return strconv.Itoa(int(v)), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case []byte:
		return string(v), nil
	case fmt.Stringer:
		return v.String(), nil
	case error:
		return v.Error(), nil
	case map[interface{}]interface{}:
		// Directly create temporary maps for type conversion
		stringMap := make(map[string]interface{})
		for key, value := range v {
			stringMap[ToString(key)] = value
		}
		if data, err := json.Marshal(stringMap); err == nil {
			return string(data), nil
		} else {
			return "", err
		}
	default:
		// For other types, use JSON serialization directly
		if data, err := json.Marshal(input); err == nil {
			return string(data), nil
		} else {
			return "", err
		}
	}
}

// ToStringMapString converts interface type to map[string]string type
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

// CheckHasVar checks whether a string has placeholders
func CheckHasVar(str string) bool {
	return strings.Contains(str, VarPrefix) && strings.Contains(str, VarSuffix)
}

// ConvertDollarPlaceholder converts to postgres-style placeholder
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

// ToLowerFirst: Convert the initial letter to lowercase
func ToLowerFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// ParseVarsWithBraces parses variables in the string and returns the variable name slices, for example: ${vars.name} -> [name]
func ParseVarsWithBraces(varPrefix, str string) []string {
	var regexpCompile = regexp.MustCompile(`\$\{` + varPrefix + `\.([^\}]+)\}`)
	// Find all matching variables
	return parseVars(regexpCompile.FindAllStringSubmatch(str, -1))
}

// ParseVars parses variables in a string and returns a variable name slice, for example: vars.name -> [name]
func ParseVars(varPrefix, str string) []string {
	var regexpCompile = regexp.MustCompile(varPrefix + `\.(\w+)`)
	// Find all matching variables
	return parseVars(regexpCompile.FindAllStringSubmatch(str, -1))
}

func parseVars(matches [][]string) []string {
	var vars = make(map[string]struct{})
	for _, match := range matches {
		// match[1] is the variable name after removing ${vars.}
		vars[match[1]] = struct{}{}
	}
	// Convert variable names in map to slices
	var result []string
	for varName := range vars {
		result = append(result, varName)
	}
	return result
}

// Contains checks whether the slice contains elements
func Contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
