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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/components/js"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// JsSwitchReturnFormatErr 如果脚本返回不是数组错误
var JsSwitchReturnFormatErr = errors.New("return the value is not an array")

func init() {
	Registry.Add(&JsSwitchNode{})
}

// JsSwitchNodeConfiguration 节点配置
type JsSwitchNodeConfiguration struct {
	JsScript string
}

// JsSwitchNode 节点执行已配置的JS脚本。脚本应返回消息应路由到的下一个链名称的数组。
// 如果数组为空-消息不路由到下一个节点。
// 消息体可以通过`msg`变量访问，如果消息的dataType是json类型，可以通过 `msg.XX`方式访问msg的字段。例如:`msg.temperature > 50;`
// 消息元数据可以通过`metadata`变量访问。例如 `metadata.customerName === 'Lala';`
// 消息类型可以通过`msgType`变量访问.
type JsSwitchNode struct {
	//节点配置
	Config              JsSwitchNodeConfiguration
	jsEngine            types.JsEngine
	defaultRelationType string
}

// Type 组件类型
func (x *JsSwitchNode) Type() string {
	return "jsSwitch"
}
func (x *JsSwitchNode) New() types.Node {
	return &JsSwitchNode{Config: JsSwitchNodeConfiguration{
		JsScript: `return ['msgType1','msgType2'];`,
	}}
}

// Init 初始化
func (x *JsSwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf("function Switch(msg, metadata, msgType) { %s }", x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
		if v := ruleConfig.Properties.GetValue(KeyOtherRelationTypeName); v != "" {
			x.defaultRelationType = v
		} else {
			x.defaultRelationType = KeyDefaultRelationType
		}
	}
	return err
}

// OnMsg 处理消息
func (x *JsSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {

	var data interface{} = msg.Data
	if msg.DataType == types.JSON {
		var dataMap = make(map[string]interface{})
		if err := json.Unmarshal([]byte(msg.Data), &dataMap); err == nil {
			data = dataMap
		}
	}

	out, err := x.jsEngine.Execute("Switch", data, msg.Metadata.Values(), msg.Type)

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

// Destroy 销毁
func (x *JsSwitchNode) Destroy() {
	x.jsEngine.Stop()
}
