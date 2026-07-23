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

package transform

// Example of rule chain node configuration:
// {
//   "id": "s2",
//   "type": "jsTransform",
//   "name": "转换",
//   "debugMode": false,
//   "configuration": {
//     "jsScript": "metadata['test']='test02';\n metadata['index']=52;\n msgType='TEST_MSG_TYPE2';\n if(dataType==='BINARY'){var newBytes=new Uint8Array(4);newBytes[0]=1;newBytes[1]=2;newBytes[2]=3;newBytes[3]=4;return {'msg':newBytes,'metadata':metadata,'msgType':msgType,'dataType':'BINARY'};} else {msg['aa']=66; return {'msg':msg,'metadata':metadata,'msgType':msgType};}"
//   }
// }
import (
	"errors"
	"fmt"
	"strings"

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

const (
	// JsTransformDefaultScript is the default JS script that returns the original message directly
	JsTransformDefaultScript = "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
	// JsTransformType component type
	JsTransformType = "jsTransform"
	// JsTransformFuncTemplate JS function template
	JsTransformFuncTemplate = "function Transform(msg, metadata, msgType, dataType) { %s }"
	// JsTransformFuncName JS function name
	JsTransformFuncName = "Transform"
)

// JsTransformReturnFormatErr indicates that the JS script did not return a map.
var JsTransformReturnFormatErr = errors.New("return the value is not a map")

func init() {
	Registry.Add(&JsTransformNode{})
}

// JsTransformNodeConfiguration JS converts node configuration
// JsTransformNodeConfiguration defines the configuration for JsTransformNode.
type JsTransformNodeConfiguration struct {
	// JsScript is the JavaScript script for message transformation.
	// Must return: {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType}
	JsScript string `json:"jsScript" label:"Transform Script" desc:"JavaScript script, must return {'msg':msg,'metadata':metadata,'msgType':msgType}. Params: msg, metadata, msgType, dataType" required:"true"`
}

// JsTransformNode is a JavaScript message conversion node that uses JavaScript scripts to process message transformation
// JsTransformNode is a JavaScript message transformation component that processes messages using JavaScript scripts.
//
// Script environment:
// Script Environment:
//   - Function signature: function transform(msg, metadata, msgType, dataType) - Function signature
//   - Input params: message data, metadata map, message type, data type
//   - Return format: {'msg':newMsg,'metadata':newMetadata,'msgType':newType,'dataType':newDataType} - Return format
//   - Optional fields: dataType fields can be omitted to keep original - Optional fields: dataType can be omitted to keep original
//
// Built-in variables:
// Built-in Variables:
//   - $ctx: Context object providing cache operations - Context object providing cache operations
//   - global: Global configuration properties
//   - vars: Rule chain variables
//   - UDF functions: User-defined functions
//
// Sample cache operation:
// Cache Operation Examples:
//
//	let cache = $ctx.ChainCache(); Get chain-level cache
//	// let cache = $ctx.GlobalCache(); Get global-level cache - Get global-level cache
//	cache.Set("key", "value"); Set cache, never expire
//	cache.Set("key2", "value2", "10m"); Set cache, expires in 10 minutes - Set cache, expires in 10 minutes
//	let value = cache.Get("key"); Get cache value - Get cache value
//	let exists = cache.Has("key"); Check if cache exists
//	cache.Delete("key"); Delete cache - Delete cache
//	let values = cache.GetByPrefix("prefix_"); Get caches by prefix matching - Get caches by prefix
//	cache.DeleteByPrefix("prefix_"); Delete caches by prefix matching caches - Delete caches by prefix
//
// Configuration example:
//
//	{
//	  "jsScript": "msg.temperature = msg.temperature * 9/5 + 32; metadata.unit = 'Fahrenheit'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//	}
//
// Use cases:
//   - Data format conversion: JSON field reorganization, unit conversion
//   - Message enrichment: add calculated fields, timestamps, identifiers
//   - Conditional transformation: Dynamic conversion logic based on message content
//   - Protocol adaptation: message conversion between different data protocols
type JsTransformNode struct {
	// Config defines the node configuration
	// Config holds the node configuration
	Config JsTransformNodeConfiguration
	// jsEngine JavaScript execution engine
	// jsEngine JavaScript execution engine
	jsEngine types.JsEngine
	// passThrough mode skips JS execution
	// passThrough direct pass-through mode, skip JS execution
	passThrough bool
}

// Type returns the component type
// Type returns the component type.
func (x *JsTransformNode) Type() string {
	return JsTransformType
}

// New creates an instance
// New creates a new instance.
func (x *JsTransformNode) New() types.Node {
	return &JsTransformNode{Config: JsTransformNodeConfiguration{
		JsScript: JsTransformDefaultScript,
	}}
}

// Init initializes the node
// Init initializes the node.
func (x *JsTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	// Check whether passthrough mode is enabled
	script := strings.TrimSpace(x.Config.JsScript)
	if script == "" || script == JsTransformDefaultScript {
		x.passThrough = true
		return nil
	}

	// Initialize the JavaScript execution engine
	jsScript := fmt.Sprintf(JsTransformFuncTemplate, x.Config.JsScript)
	x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	return err
}

// OnMsg processes messages and uses JavaScript scripts to transform message content
// OnMsg processes messages using JavaScript script for message transformation.
func (x *JsTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// Direct Mode: Direct forwarding
	if x.passThrough {
		ctx.TellNext(msg, types.Success)
		return
	}

	// Prepare to pass the data to the JS script. Since JavaScript can modify the data, it will affect the original data, so a copy is needed
	data := base.NodeUtils.GetDataByType(msg, false)

	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// Execute JavaScript scripts
	out, err := x.jsEngine.Execute(ctx, JsTransformFuncName, data, metadataValues, msg.Type, string(msg.DataType))
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	// Handle the execution results
	x.processJsResult(ctx, msg, out)
}

// processJsResult handles the result of JavaScript execution
// processJsResult processes JavaScript execution results.
func (x *JsTransformNode) processJsResult(ctx types.RuleContext, msg types.RuleMsg, out interface{}) {
	// Verify the return value format
	formatData, ok := out.(map[string]interface{})
	if !ok {
		ctx.TellFailure(msg, JsTransformReturnFormatErr)
		return
	}

	// Update data types
	if formatDataType, ok := formatData[types.DataTypeKey]; ok {
		if dataTypeStr := str.ToString(formatDataType); dataTypeStr != "" {
			msg.DataType = types.DataType(dataTypeStr)
		}
	}

	// Update message type
	if formatMsgType, ok := formatData[types.MsgTypeKey]; ok {
		msg.Type = str.ToString(formatMsgType)
	}

	// Update metadata
	if formatMetaData, ok := formatData[types.MetadataKey]; ok {
		msg.Metadata.ReplaceAll(str.ToStringMapString(formatMetaData))
	}

	// Update message data
	if formatMsgData, ok := formatData[types.MsgKey]; ok {
		// Processing byte arrays
		if byteData, isByteSlice := formatMsgData.([]byte); isByteSlice {
			msg.SetBytes(byteData)
		} else if byteData, isByteArray := formatMsgData.([]interface{}); isByteArray {
			// Try converting it to a byte array
			bytes := make([]byte, len(byteData))
			isValidByteArray := true
			for i, v := range byteData {
				var byteVal float64
				var isNumber bool

				if val, ok := v.(float64); ok {
					byteVal = val
					isNumber = true
				} else if val, ok := v.(int64); ok {
					byteVal = float64(val)
					isNumber = true
				} else if val, ok := v.(int); ok {
					byteVal = float64(val)
					isNumber = true
				}

				if isNumber {
					// Border checks
					if byteVal < 0 || byteVal > 255 || byteVal != float64(int(byteVal)) {
						ctx.TellFailure(msg, fmt.Errorf("byte array element at index %d has invalid value %v: must be integer in range 0-255", i, byteVal))
						return
					}
					bytes[i] = byte(byteVal)
				} else {
					isValidByteArray = false
					break
				}
			}

			if isValidByteArray {
				msg.SetBytes(bytes)
			} else {
				// String conversion processing
				if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
					msg.SetData(newValue)
				} else {
					ctx.TellFailure(msg, err)
					return
				}
			}
		} else {
			// Ordinary data types
			if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
				msg.SetData(newValue)
			} else {
				ctx.TellFailure(msg, err)
				return
			}
		}
	}

	// Send to the Success chain
	ctx.TellNext(msg, types.Success)
}

// Desc returns the component description
func (x *JsTransformNode) Desc() string {
	return "Transform messages using JavaScript. Must return {'msg':msg,'metadata':metadata,'msgType':msgType}. Params: msg, metadata, msgType, dataType."
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *JsTransformNode) Destroy() {
	if x.jsEngine != nil {
		x.jsEngine.Stop()
	}
}
