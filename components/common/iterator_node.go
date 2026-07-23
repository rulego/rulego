/*
 * Copyright 2025 The RuleGo Authors.
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

package common

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// init registers the IteratorNode component
// init registers the IteratorNode component with the default registry.
func init() {
	//Registry.Add(&IteratorNode{}) // Deprecated, replaced with a for component
}

// IteratorNodeConfiguration IteratorNode configuration structure
// IteratorNodeConfiguration defines the configuration structure for the IteratorNode component.
type IteratorNodeConfiguration struct {
	// FieldName is the field to iterate over. Supports dot notation for nested access.
	// If empty, iterates over the entire message.
	FieldName string `json:"fieldName" label:"Field Name" desc:"Field to iterate over. Supports dot notation (e.g. items.value). Empty=iterate entire message"`
	// JsScript is optional JavaScript filter. Function: ItemFilter(item, index, metadata) -> boolean.
	JsScript string `json:"jsScript" label:"Filter Script" desc:"Optional JS filter: function ItemFilter(item, index, metadata). Returns boolean for True/False routing"`
}

// IteratorNode is an action component that traverses an array or object in message data
// IteratorNode is an action component that iterates over arrays or objects in message data.
//
// Deprecation Notice:
//   - This component is deprecated, use ForNode for better performance and features
//
// Core algorithm:
// Core Algorithm:
// 1. Extract and parse data to iterate (supports JSON) - Extract and parse data to iterate (supports JSON)
// 2. Extract specific fields by FieldName or use entire messages - Extract specific field by FieldName or use entire message
// 3. Iterate over each element in an array or object
// 4. Apply JavaScript filter to each element (if configured)
// 5. Route to True/False relations based on filter results
// 6. After traversal is complete, send the original message via Success relation after iteration
//
// Supported data types:
//   - []interface{}: Arrays with numeric indices
//   - map[string]interface{}: Objects, using string keys - Objects with string keys
//
// JavaScript filter function signature:
//   - function ItemFilter(item, index, metadata) -> boolean
type IteratorNode struct {
	// Config defines the node configuration
	// Config holds the node configuration including field name and JavaScript filter
	Config IteratorNodeConfiguration

	// jsEngine JavaScript engine example
	// jsEngine holds the JavaScript engine instance for item filtering
	jsEngine types.JsEngine
}

// Type returns the component type
// Type returns the component type identifier.
func (x *IteratorNode) Type() string {
	return "iterator"
}

// New creates an instance
// New creates a new instance.
func (x *IteratorNode) New() types.Node {
	return &IteratorNode{Config: IteratorNodeConfiguration{}}
}

// Init initializes the component
// Init initializes the component.
func (x *IteratorNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	x.Config.JsScript = strings.TrimSpace(x.Config.JsScript)
	x.Config.FieldName = strings.TrimSpace(x.Config.FieldName)
	if err == nil && x.Config.JsScript != "" {
		jsScript := fmt.Sprintf("function ItemFilter(item,index,metadata) { %s }", x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	return err
}

// OnMsg processes messages, traverses specified fields or entire messages, and applies JavaScript filters
// OnMsg processes incoming messages by iterating over the specified field or entire message.
func (x *IteratorNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data interface{} = msg.GetData()
	if msg.DataType == types.JSON {
		var dataMap interface{}
		if err := json.Unmarshal([]byte(msg.GetData()), &dataMap); err == nil {
			data = dataMap
		}
	}

	// Traverse the specified field
	if x.Config.FieldName != "" {
		data = maps.Get(data, x.Config.FieldName)
		if data == nil {
			ctx.TellFailure(msg, errors.New("field="+x.Config.FieldName+" not found"))
			return
		}
	}

	if arrayValue, ok := data.([]interface{}); ok {
		oldMsg := msg.Copy()
		for index, item := range arrayValue {
			if err := x.executeItem(ctx, msg, item, index); err != nil {
				//An error interrupts traversal occurs
				return
			}
		}
		ctx.TellSuccess(oldMsg)
	} else if mapValue, ok := data.(map[string]interface{}); ok {
		oldMsg := msg.Copy()
		for k, item := range mapValue {
			if err := x.executeItem(ctx, msg, item, k); err != nil {
				//An error interrupts traversal occurs
				return
			}
		}
		ctx.TellSuccess(oldMsg)
	} else {
		ctx.TellFailure(msg, errors.New("value is not array or {key:value} type"))
	}
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *IteratorNode) Destroy() {
	// No resources to clean
	// No resources to clean up
}

// executeItem handles each traversal item, applies JavaScript filters, and routes them
// executeItem processes each individual item during iteration.
func (x *IteratorNode) executeItem(ctx types.RuleContext, msg types.RuleMsg, item interface{}, index interface{}) error {
	if x.jsEngine != nil {
		// Using zero-copy GetReadOnlyValues, the JS engine only reads metadata
		if out, err := x.jsEngine.Execute(ctx, "ItemFilter", item, index, msg.Metadata.GetReadOnlyValues()); err != nil {
			ctx.TellFailure(msg, err)
			//An error interrupts traversal occurs
			return err
		} else if formatData, ok := out.(bool); ok && formatData {
			msg.SetData(str.ToString(item))
			ctx.TellNext(msg, types.True)
		} else {
			msg.SetData(str.ToString(item))
			ctx.TellNext(msg, types.False)
		}
	} else {
		msg.SetData(str.ToString(item))
		ctx.TellNext(msg, types.True)
	}
	return nil
}

// Def returns the component form definition
func (x *IteratorNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Deprecated: use for node. Iterate over arrays/objects with optional JS filter. Routes each item to True/False, completes via Success",
		RelationTypes: &[]string{types.Success, types.Failure, types.True, types.False},
	}
}
