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

package filter

import (
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
)

// init 注册FieldFilterNode组件
// init registers the FieldFilterNode component with the default registry.
func init() {
	Registry.Add(&FieldFilterNode{})
}

// FieldFilterNodeConfiguration FieldFilterNode配置结构
// FieldFilterNodeConfiguration defines the configuration structure for the FieldFilterNode component.
type FieldFilterNodeConfiguration struct {
	// CheckAllKeys 决定字段检查逻辑
	// CheckAllKeys determines the field checking logic:
	//   - true: All specified fields must exist for message to pass filter
	//   - false: Any specified field existing will pass the filter
	CheckAllKeys bool

	// DataNames 指定要在消息数据中检查的字段名称，多个字段名称用逗号分隔
	// DataNames specifies the field names to check in message data.
	// Multiple field names should be separated by commas.
	// Only applicable when message data type is JSON.
	//
	// Example: "temperature,humidity,pressure"
	DataNames string

	// MetadataNames 指定要在消息元数据中检查的字段名称，多个字段名称用逗号分隔
	// MetadataNames specifies the field names to check in message metadata.
	// Multiple field names should be separated by commas.
	//
	// Example: "deviceId,location,timestamp"
	MetadataNames string
}

// FieldFilterNode 根据消息数据和元数据中指定字段的存在来过滤消息的过滤组件
// FieldFilterNode filters messages based on the existence of specified fields in message data and metadata.
//
// 核心算法：
// Core Algorithm:
// 1. 解析JSON格式的消息数据（如果指定了DataNames）- Parse JSON message data if DataNames specified
// 2. 检查指定字段在消息数据中的存在性 - Check field existence in message data
// 3. 检查指定字段在元数据中的存在性 - Check field existence in metadata
// 4. 根据CheckAllKeys配置应用ALL/ANY逻辑 - Apply ALL/ANY logic based on CheckAllKeys configuration
//
// 验证逻辑 - Validation logic:
//   - CheckAllKeys=true: 所有指定字段都必须存在 - All specified fields must exist
//   - CheckAllKeys=false: 至少一个指定字段必须存在 - At least one specified field must exist
//   - 空字段列表在验证中被忽略 - Empty field lists are ignored in validation
//
// 字段规范 - Field specification:
//   - DataNames: 逗号分隔的JSON数据字段 - Comma-separated JSON data fields
//   - MetadataNames: 逗号分隔的元数据字段 - Comma-separated metadata fields
type FieldFilterNode struct {
	// Config 字段过滤器配置
	// Config holds the field filter configuration
	Config FieldFilterNodeConfiguration

	// DataNamesList 要检查的数据字段名称列表
	// DataNamesList contains the parsed list of data field names to check
	DataNamesList []string

	// MetadataNamesList 要检查的元数据字段名称列表
	// MetadataNamesList contains the parsed list of metadata field names to check
	MetadataNamesList []string
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *FieldFilterNode) Type() string {
	return "fieldFilter"
}

// New 创建新实例
// New creates a new instance.
func (x *FieldFilterNode) New() types.Node {
	return &FieldFilterNode{}
}

// Init 初始化组件，解析逗号分隔的字段名称
// Init initializes the component.
func (x *FieldFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	x.DataNamesList = filterEmptyStrings(strings.Split(x.Config.DataNames, ","))
	x.MetadataNamesList = filterEmptyStrings(strings.Split(x.Config.MetadataNames, ","))
	return err
}

// OnMsg 处理消息，通过检查数据和元数据中的字段存在来验证消息
// OnMsg processes incoming messages by checking field existence in data and metadata.
func (x *FieldFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var dataMap = make(map[string]interface{})
	if msg.DataType == types.JSON {
		if err := json.Unmarshal([]byte(msg.GetData()), &dataMap); err != nil {
			ctx.TellFailure(msg, err)
			return
		}
	}

	if x.Config.CheckAllKeys {
		if x.checkAllKeysMetadata(msg.Metadata) && x.checkAllKeysData(dataMap) {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	} else {
		if x.checkAtLeastOneMetadata(msg.Metadata) || x.checkAtLeastOneData(dataMap) {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *FieldFilterNode) Destroy() {
}

// checkAllKeysMetadata 验证所有指定的元数据字段都存在
// checkAllKeysMetadata validates that all specified metadata fields exist.
func (x *FieldFilterNode) checkAllKeysMetadata(metadata *types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if !metadata.Has(item) {
			return false
		}
	}
	return true
}

// checkAllKeysData 验证所有指定的数据字段都存在
// checkAllKeysData validates that all specified data fields exist.
func (x *FieldFilterNode) checkAllKeysData(data map[string]interface{}) bool {
	for _, item := range x.DataNamesList {
		if data == nil {
			return false
		}
		if _, ok := data[item]; !ok {
			return false
		}
	}
	return true
}

// checkAtLeastOneMetadata 验证至少一个指定的元数据字段存在
// checkAtLeastOneMetadata validates that at least one specified metadata field exists.
func (x *FieldFilterNode) checkAtLeastOneMetadata(metadata *types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if metadata.Has(item) {
			return true
		}
	}
	return false
}

// filterEmptyStrings 过滤掉字符串切片中的空字符串
// filterEmptyStrings filters out empty strings from a string slice.
func filterEmptyStrings(strs []string) []string {
	var result []string
	for _, str := range strs {
		if strings.TrimSpace(str) != "" {
			result = append(result, strings.TrimSpace(str))
		}
	}
	return result
}

// checkAtLeastOneData 验证至少一个指定的数据字段存在
// checkAtLeastOneData validates that at least one specified data field exists.
func (x *FieldFilterNode) checkAtLeastOneData(data map[string]interface{}) bool {
	for _, item := range x.DataNamesList {
		if data == nil {
			return false
		}
		if _, ok := data[item]; ok {
			return true
		}
	}
	return false
}
