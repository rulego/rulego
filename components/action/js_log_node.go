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

//规则链节点配置示例：
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
	// JsLogFuncName JavaScript函数名
	JsLogFuncName = "ToString"

	// JsLogFuncTemplate JavaScript函数模板
	JsLogFuncTemplate = "function ToString(msg, metadata, msgType, dataType) { %s }"
)

// JsLogReturnFormatErr JavaScript脚本必须返回字符串
var JsLogReturnFormatErr = errors.New("return the value is not a string")

// init 注册LogNode组件
func init() {
	Registry.Add(&LogNode{})
}

// LogNodeConfiguration LogNode配置结构
type LogNodeConfiguration struct {
	// JsScript JavaScript脚本，用于格式化日志消息
	// 函数参数：msg, metadata, msgType, dataType
	// 必须返回字符串用于日志记录
	//
	// 内置变量：
	//   - $ctx: 上下文对象，提供缓存操作
	//   - global: 全局配置属性
	//   - vars: 规则链变量
	//   - UDF函数: 用户自定义函数
	//
	// 示例: "return '[' + msgType + '] ' + JSON.stringify(msg);"
	JsScript string `json:"jsScript"`
}

// LogNode 使用JavaScript格式化并记录消息的日志节点
type LogNode struct {
	// Config 节点配置
	Config LogNodeConfiguration

	// jsEngine JavaScript执行引擎
	jsEngine types.JsEngine

	// logger 日志记录器
	logger types.Logger
}

// Type 返回组件类型
func (x *LogNode) Type() string {
	return "log"
}

// New 创建新实例
func (x *LogNode) New() types.Node {
	return &LogNode{Config: LogNodeConfiguration{
		JsScript: `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
	}}
}

// Init 初始化节点
func (x *LogNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsLogFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	x.logger = ruleConfig.Logger
	return err
}

// OnMsg 处理消息，执行JavaScript脚本格式化并记录日志
func (x *LogNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 准备传递给JS脚本的数据
	data := base.NodeUtils.GetDataByType(msg, true)

	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// 执行JavaScript脚本
	out, err := x.jsEngine.Execute(ctx, JsLogFuncName, data, metadataValues, msg.Type, msg.DataType)
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if formatData, ok := out.(string); ok {
			x.logger.Printf(formatData)
			ctx.TellSuccess(msg)
		} else {
			ctx.TellFailure(msg, JsLogReturnFormatErr)
		}
	}
}

// Destroy 清理资源
func (x *LogNode) Destroy() {
	x.jsEngine.Stop()
}
