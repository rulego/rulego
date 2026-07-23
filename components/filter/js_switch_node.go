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
//        "type": "jsSwitch",
//        "name": "脚本路由",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "return ['one','two'];"
//        }
//      }
import (
	"errors"
	"fmt"

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// The JsSwitchReturnFormatErr JavaScript script must return an array
var JsSwitchReturnFormatErr = errors.New("return the value is not an array")

// init registers the JsSwitchNode component
func init() {
	Registry.Add(&JsSwitchNode{})
}

// JsSwitchNodeConfiguration
type JsSwitchNodeConfiguration struct {
	// JsScript JavaScript script to determine message routing paths
	// Function parameters: msg, metadata, msgType, dataType
	// Must return a string array of routing relation types
	//
	// Built-in variables:
	//   - $ctx: context object for cache operations
	//   - global: global configuration properties
	//   - vars: rule chain variables
	//   - UDF functions: user-defined functions
	//
	// Example: "return ['route1', 'route2'];"
	JsScript string `json:"jsScript" label:"Switch Script" desc:"JavaScript script that returns an array of routing relation types. Example: return ['route1','route2'];" required:"true"`
}

// JsSwitchNode uses JavaScript to determine the switch node for message routing paths
type JsSwitchNode struct {
	// Config defines the node configuration
	Config JsSwitchNodeConfiguration

	// jsEngine JavaScript execution engine
	jsEngine types.JsEngine

	// defaultRelationType The default relationship type
	defaultRelationType string
}

// Type returns the component type
func (x *JsSwitchNode) Type() string {
	return "jsSwitch"
}

// New creates an instance
func (x *JsSwitchNode) New() types.Node {
	return &JsSwitchNode{Config: JsSwitchNodeConfiguration{
		JsScript: `return ['msgType1','msgType2'];`,
	}}
}

// Init initializes the node
func (x *JsSwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf("function Switch(msg, metadata, msgType, dataType) { %s }", x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
		if v := ruleConfig.Properties.GetValue(types.DefaultRelationTypeKey); v != "" {
			x.defaultRelationType = v
		} else {
			x.defaultRelationType = types.DefaultRelationType
		}
	}
	return err
}

// OnMsg processes messages and executes JavaScript scripts to determine routing paths
func (x *JsSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// Prepare data to be passed to the JS script
	data := base.NodeUtils.GetDataByType(msg, true)

	out, err := x.jsEngine.Execute(ctx, "Switch", data, msg.Metadata.Values(), msg.Type, msg.DataType)

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if formatData, ok := out.([]interface{}); ok {
			for _, relationType := range formatData {
				ctx.TellNextOrElse(msg, x.defaultRelationType, str.ToString(relationType))
			}
		} else {
			ctx.TellFailure(msg, JsSwitchReturnFormatErr)
		}
	}
}

// Desc returns the component description
func (x *JsSwitchNode) Desc() string {
	return "Use JavaScript to determine message routing paths. The script must return an array of relation type strings. Available variables: msg, metadata, msgType, dataType"
}

// Destroy to clean up resources
func (x *JsSwitchNode) Destroy() {
	x.jsEngine.Stop()
}
