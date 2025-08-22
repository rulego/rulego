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

	"github.com/rulego/rulego/utils/js"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

const (
	// JsFilterFuncName JS函数名
	JsFilterFuncName = "Filter"
	// JsFilterType JsFilter组件类型
	JsFilterType = "jsFilter"
	// JsFilterFuncTemplate JS函数模板
	JsFilterFuncTemplate = "function Filter(msg, metadata, msgType, dataType) { %s }"
)

// init 注册JsFilterNode组件
func init() {
	Registry.Add(&JsFilterNode{})
}

// JsFilterNodeConfiguration JsFilterNode配置结构
type JsFilterNodeConfiguration struct {
	// JsScript JavaScript脚本，用于评估过滤条件
	// 函数参数：msg, metadata, msgType, dataType
	// 必须返回布尔值：true通过过滤，false不通过
	//
	// 内置变量：
	//   - $ctx: 上下文对象，提供缓存操作
	//   - global: 全局配置属性
	//   - vars: 规则链变量
	//   - UDF函数: 用户自定义函数
	//
	// 示例: "return msg.temperature > 25.0;"
	JsScript string `json:"jsScript"`
}

// JsFilterNode 使用JavaScript评估布尔条件的过滤器节点
type JsFilterNode struct {
	// Config 节点配置
	Config JsFilterNodeConfiguration

	// jsEngine JavaScript执行引擎
	jsEngine types.JsEngine
}

// Type 返回组件类型
func (x *JsFilterNode) Type() string {
	return JsFilterType
}

// New 创建新实例
func (x *JsFilterNode) New() types.Node {
	return &JsFilterNode{Config: JsFilterNodeConfiguration{
		JsScript: "return msg.temperature > 50;",
	}}
}

// Init 初始化节点
func (x *JsFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsFilterFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	return err
}

// OnMsg 处理消息，执行JavaScript过滤条件
func (x *JsFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 准备传递给JS脚本的数据
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

// Destroy 清理资源
func (x *JsFilterNode) Destroy() {
	if x.jsEngine != nil {
		x.jsEngine.Stop()
	}
}
