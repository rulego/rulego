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

// init registers the FieldFilterNode component
// init registers the FieldFilterNode component with the default registry.
func init() {
	Registry.Add(&FieldFilterNode{})
}

// FieldFilterNodeConfiguration FieldFilterNode configuration structure
// FieldFilterNodeConfiguration defines the configuration structure for the FieldFilterNode component.
type FieldFilterNodeConfiguration struct {
	// CheckAllKeys determines field checking logic.
	// true: all specified fields must exist; false: any field existing is sufficient.
	CheckAllKeys bool `json:"checkAllKeys" label:"Check All Keys" desc:"true=ALL fields must exist, false=ANY field exists is sufficient"`

	// DataNames specifies comma-separated field names to check in message data (JSON only).
	DataNames string `json:"dataNames" label:"Data Fields" desc:"Comma-separated field names to check in message data (JSON only)"`

	// MetadataNames specifies comma-separated field names to check in message metadata.
	MetadataNames string `json:"metadataNames" label:"Metadata Fields" desc:"Comma-separated field names to check in message metadata"`
}

// FieldFilterNode filters messages based on the message data and the presence of specified fields in the metadata
// FieldFilterNode filters messages based on the existence of specified fields in message data and metadata.
//
// Core algorithm:
// Core Algorithm:
// 1. Parse message data in JSON format (if DataNames are specified) - Parse JSON message data if DataNames specified
// 2. Check the existence of specified fields in message data - Check field existence in message data
// 3. Check the existence of a specified field in metadata - Check field existence in metadata
// 4. Apply ALL/ANY logic based on CheckAllKeys configuration
//
// Validation logic:
//   - CheckAllKeys=true: All specified fields must exist
//   - CheckAllKeys=false: At least one specified field must exist
//   - Empty field lists are ignored in validation
//
// Field specification:
//   - DataNames: Comma-separated JSON data fields
//   - MetadataNames: Comma-separated metadata fields
type FieldFilterNode struct {
	// Config field filter configuration
	// Config holds the field filter configuration
	Config FieldFilterNodeConfiguration

	// DataNamesList The list of data field names to check
	// DataNamesList contains the parsed list of data field names to check
	DataNamesList []string

	// MetadataNamesList The list of metadata field names to check
	// MetadataNamesList contains the parsed list of metadata field names to check
	MetadataNamesList []string
}

// Type returns the component type
// Type returns the component type identifier.
func (x *FieldFilterNode) Type() string {
	return "fieldFilter"
}

// New creates an instance
// New creates a new instance.
func (x *FieldFilterNode) New() types.Node {
	return &FieldFilterNode{}
}

// Init initializes the component and parses the comma-separated field names
// Init initializes the component.
func (x *FieldFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	x.DataNamesList = filterEmptyStrings(strings.Split(x.Config.DataNames, ","))
	x.MetadataNamesList = filterEmptyStrings(strings.Split(x.Config.MetadataNames, ","))
	return err
}

// OnMsg processes messages by verifying the presence of fields in the data and metadata
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

// Desc returns the component description
func (x *FieldFilterNode) Desc() string {
	return "Filter messages by checking existence of specified fields in message data (JSON) and metadata. checkAllKeys controls AND/OR logic. Routes to True/False"
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *FieldFilterNode) Destroy() {
}

// checkAllKeysMetadata verifies that all specified metadata fields exist
// checkAllKeysMetadata validates that all specified metadata fields exist.
func (x *FieldFilterNode) checkAllKeysMetadata(metadata *types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if !metadata.Has(item) {
			return false
		}
	}
	return true
}

// checkAllKeysData verifies that all specified data fields exist
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

// checkAtLeastOneMetadata verifies the existence of at least one specified metadata field
// checkAtLeastOneMetadata validates that at least one specified metadata field exists.
func (x *FieldFilterNode) checkAtLeastOneMetadata(metadata *types.Metadata) bool {
	for _, item := range x.MetadataNamesList {
		if metadata.Has(item) {
			return true
		}
	}
	return false
}

// filterEmptyStrings filters out empty strings in the string slice
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

// checkAtLeastOneData verifies the existence of at least one specified data field
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
