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

//Example of rule chain node configuration:
//{
//        "id": "s2",
//        "type": "jsFilter",
//        "name": "过滤",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "return msg.temperature > 50;"
//        }
//      }
import (
	"fmt"

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

const (
	// JsFilterFuncName: JS function name
	JsFilterFuncName = "Filter"
	// JsFilterType The type of the JsFilter component
	JsFilterType = "jsFilter"
	// JsFilterFuncTemplate JS function template
	JsFilterFuncTemplate = "function Filter(msg, metadata, msgType, dataType) { %s }"
)

// init registers the JsFilterNode component
func init() {
	Registry.Add(&JsFilterNode{})
}

// JsFilterNodeConfiguration The configuration structure of JsFilterNode
type JsFilterNodeConfiguration struct {
	// JsScript JavaScript script for evaluating filter conditions
	// Function parameters: msg, metadata, msgType, dataType
	// A boolean value must be returned: true passes filtering, false does not
	//
	// Built-in variables:
	//   - $ctx: Context object, provides caching operations
	//   - global: Global configuration properties
	//   - vars: Rules chain variables
	//   - UDF function: User-defined function
	//
	// Example: "return msg.temperature > 25.0;"
	JsScript string `json:"jsScript" label:"Filter Script" desc:"JavaScript expression that returns true to pass, false to reject. Available variables: msg (message body), metadata, msgType (message type)" required:"true"`
}

// JsFilterNode uses JavaScript to evaluate the filter node for boolean conditions
type JsFilterNode struct {
	// Config defines the node configuration
	Config JsFilterNodeConfiguration

	// jsEngine JavaScript execution engine
	jsEngine types.JsEngine
}

// Type returns the component type
func (x *JsFilterNode) Type() string {
	return JsFilterType
}

// New creates an instance
func (x *JsFilterNode) New() types.Node {
	return &JsFilterNode{Config: JsFilterNodeConfiguration{
		JsScript: "return msg.temperature > 50;",
	}}
}

// Init initializes the node
func (x *JsFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsFilterFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	return err
}

// OnMsg processes messages and executes JavaScript filtering conditions
func (x *JsFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// Prepare data to be passed to the JS script
	data := base.NodeUtils.GetDataByType(msg, true)

	out, err := x.jsEngine.Execute(ctx, JsFilterFuncName, data, msg.Metadata.Values(), msg.Type, string(msg.DataType))
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if formatData, ok := out.(bool); ok && formatData {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Desc returns the component description
func (x *JsFilterNode) Desc() string {
	return "Filter messages using a JavaScript expression. Returns true routes to True, false routes to False. Available variables: msg (message body), metadata, msgType, dataType"
}

// Destroy to clean up resources
func (x *JsFilterNode) Destroy() {
	if x.jsEngine != nil {
		x.jsEngine.Stop()
	}
}
