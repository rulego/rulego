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
	"github.com/mitchellh/mapstructure"
	"strings"
)

// Map2Struct Decode takes an input structure and uses reflection to translate it to
// the output structure. output must be a pointer to a map or struct.
func Map2Struct(input interface{}, output interface{}) error {
	cfg := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc()),
		WeaklyTypedInput: true,
		Metadata:         nil,
		Result:           &output,
	}
	if d, err := mapstructure.NewDecoder(cfg); err != nil {
		return err
	} else if err := d.Decode(input); err != nil {
		return err
	}
	return nil
}

// Get 获取map中的字段，支持嵌套结构获取，例如fieldName.subFieldName.xx
// 嵌套类型必须是map[string]interface{}
// 如果字段不存在，返回nil
func Get(input interface{}, fieldName string) interface{} {
	// 按照"."分割fieldName
	fields := strings.Split(fieldName, ".")
	var result interface{}
	result = input
	// 遍历每个子字段
	for _, field := range fields {
		if mapValue, ok := result.(map[string]interface{}); ok {
			if v, ok := mapValue[field]; ok {
				result = v
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return result
}
