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

//规则链节点配置示例：
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

// JsSwitchReturnFormatErr JavaScript脚本必须返回数组
var JsSwitchReturnFormatErr = errors.New("return the value is not an array")

// init 注册JsSwitchNode组件
func init() {
	Registry.Add(&JsSwitchNode{})
}

// JsSwitchNodeConfiguration JsSwitchNode配置结构
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

// JsSwitchNode 使用JavaScript确定消息路由路径的开关节点
type JsSwitchNode struct {
	// Config 节点配置
	Config JsSwitchNodeConfiguration

	// jsEngine JavaScript执行引擎
	jsEngine types.JsEngine

	// defaultRelationType 默认关系类型
	defaultRelationType string
}

// Type 返回组件类型
func (x *JsSwitchNode) Type() string {
	return "jsSwitch"
}

// New 创建新实例
func (x *JsSwitchNode) New() types.Node {
	return &JsSwitchNode{Config: JsSwitchNodeConfiguration{
		JsScript: `return ['msgType1','msgType2'];`,
	}}
}

// Init 初始化节点
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

// OnMsg 处理消息，执行JavaScript脚本确定路由路径
func (x *JsSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 准备传递给JS脚本的数据
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

// Destroy 清理资源
func (x *JsSwitchNode) Destroy() {
	x.jsEngine.Stop()
}
