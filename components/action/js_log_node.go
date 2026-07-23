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

package action

//Example of rule chain node configuration:
//{
//        "id": "s2",
//        "type": "log",
//        "name": "记录日志",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "return 'Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);"
//        }
//  }
import (
	"errors"
	"fmt"

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

const (
	// JsLogFuncName: JavaScript function name
	JsLogFuncName = "ToString"

	// JsLogFuncTemplate JavaScript function template
	JsLogFuncTemplate = "function ToString(msg, metadata, msgType, dataType) { %s }"
)

// The JsLogReturnFormatErr JavaScript script must return a string
var JsLogReturnFormatErr = errors.New("return the value is not a string")

// init registers the LogNode component
func init() {
	Registry.Add(&LogNode{})
}

// LogNodeConfiguration: LogNode configuration structure
type LogNodeConfiguration struct {
	// JsScript is the JavaScript script for formatting log messages.
	// Must return a string. Parameters: msg, metadata, msgType, dataType.
	JsScript string `json:"jsScript" label:"Log Script" desc:"JavaScript script to format log message. Must return a string. Params: msg, metadata, msgType, dataType" required:"true"`
}

// LogNode is a log node formatted with JavaScript and records messages
type LogNode struct {
	// Config defines the node configuration
	Config LogNodeConfiguration

	// jsEngine JavaScript execution engine
	jsEngine types.JsEngine

	// Logger
	logger types.Logger
}

// Type returns the component type
func (x *LogNode) Type() string {
	return "log"
}

// New creates an instance
func (x *LogNode) New() types.Node {
	return &LogNode{Config: LogNodeConfiguration{
		JsScript: `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
	}}
}

// Init initializes the node
func (x *LogNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsLogFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	x.logger = ruleConfig.Logger
	return err
}

// OnMsg handles messages, executes JavaScript scripts to format them, and logs
func (x *LogNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// Prepare data to be passed to the JS script
	data := base.NodeUtils.GetDataByType(msg, true)

	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// Execute JavaScript scripts
	out, err := x.jsEngine.Execute(ctx, JsLogFuncName, data, metadataValues, msg.Type, msg.DataType)
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if formatData, ok := out.(string); ok {
			x.logger.Printf("%s", formatData)
			ctx.TellSuccess(msg)
		} else {
			ctx.TellFailure(msg, JsLogReturnFormatErr)
		}
	}
}

// Destroy to clean up resources
func (x *LogNode) Destroy() {
	x.jsEngine.Stop()
}

// Desc returns the component description
func (x *LogNode) Desc() string {
	return "Format and log messages using JavaScript. Script must return a string. Params: msg, metadata, msgType, dataType. Routes to Success/Failure"
}
