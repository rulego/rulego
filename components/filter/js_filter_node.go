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
//        "type": "jsFilter",
//        "name": "过滤",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "return msg.temperature > 50;"
//        }
//      }
import (
	"fmt"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/components/js"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
)

const (
	// JsFilterFuncName JS函数名
	JsFilterFuncName = "Filter"
	// JsFilterType JsFilter组件类型
	JsFilterType = "jsFilter"
	// JsFilterFuncTemplate JS函数模板
	JsFilterFuncTemplate = "function Filter(msg, metadata, msgType) { %s }"
)

func init() {
	Registry.Add(&JsFilterNode{})
}

// JsFilterNodeConfiguration 节点配置
type JsFilterNodeConfiguration struct {
	//JsScript 配置函数体脚本内容
	// 使用js脚本进行过滤
	//完整脚本函数：
	//function Filter(msg, metadata, msgType) { ${JsScript} }
	//return bool
	JsScript string
}

// JsFilterNode 使用js脚本过滤传入信息
// 如果 `True`发送信息到`True`链, `False`发到`False`链。
// 如果 脚本执行失败则发送到`Failure`链
// 消息体可以通过`msg`变量访问，如果消息的dataType是json类型，可以通过 `msg.XX`方式访问msg的字段。例如:`return msg.temperature > 50;`
// 消息元数据可以通过`metadata`变量访问。例如 `metadata.customerName === 'Lala';`
// 消息类型可以通过`msgType`变量访问.
type JsFilterNode struct {
	//节点配置
	Config   JsFilterNodeConfiguration
	jsEngine types.JsEngine
}

// Type 组件类型
func (x *JsFilterNode) Type() string {
	return JsFilterType
}

func (x *JsFilterNode) New() types.Node {
	return &JsFilterNode{Config: JsFilterNodeConfiguration{
		JsScript: "return msg.temperature > 50;",
	}}
}

// Init 初始化
func (x *JsFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsFilterFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	return err
}

// OnMsg 处理消息
func (x *JsFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data interface{} = msg.Data
	if msg.DataType == types.JSON {
		var dataMap interface{}
		if err := json.Unmarshal([]byte(msg.Data), &dataMap); err == nil {
			data = dataMap
		}
	}

	out, err := x.jsEngine.Execute(ctx, JsFilterFuncName, data, msg.Metadata.Values(), msg.Type)
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

// Destroy 销毁
func (x *JsFilterNode) Destroy() {
	x.jsEngine.Stop()
}
