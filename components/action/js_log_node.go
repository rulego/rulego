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
	// JsLogFuncTemplate JavaScript函数模板，新增dataType参数
	JsLogFuncTemplate = "function ToString(msg, metadata, msgType, dataType) { %s }"
)

// JsLogReturnFormatErr 如果脚本返回值不是string错误
var JsLogReturnFormatErr = errors.New("return the value is not a string")

// 注册节点
func init() {
	Registry.Add(&LogNode{})
}

// LogNodeConfiguration 节点配置
type LogNodeConfiguration struct {
	//JsScript 只配置函数体脚本内容，对消息进行格式化，脚本返回值string
	//例如
	//return 'Incoming message:\\n' + JSON.stringify(msg) + '\\nIncoming metadata:\\n' + JSON.stringify(metadata);
	//完整脚本函数：
	//"function ToString(msg, metadata, msgType, dataType) { ${JsScript} }"
	//JavaScript函数接收4个参数：
	//- msg: 消息的payload数据（JSON类型会解析为对象，BINARY类型为Uint8Array，其他为字符串）
	//- metadata: 消息的元数据
	//- msgType: 消息的类型
	//- dataType: 消息的数据类型（JSON、TEXT、BINARY等）
	//脚本返回值string
	JsScript string
}

// LogNode 使用JS脚本将传入消息转换为字符串，并将最终值记录到日志文件中
// 使用`types.Config.Logger`记录日志
// 消息体可以通过`msg`变量访问，如果消息的dataType是JSON类型，可以通过 `msg.XX`方式访问msg的字段。例如:`return msg.temperature > 50;`
// BINARY类型的消息体会作为Uint8Array传递给JavaScript
// 消息元数据可以通过`metadata`变量访问。例如 `metadata.customerName === 'Lala';`
// 消息类型可以通过`msgType`变量访问.
// 消息数据类型可以通过`dataType`变量访问.
// 脚本执行成功，发送信息到`Success`链, 否则发到`Failure`链。
type LogNode struct {
	//节点配置
	Config LogNodeConfiguration
	//js脚本引擎
	jsEngine types.JsEngine
	//日志记录器
	logger types.Logger
}

// Type 组件类型
func (x *LogNode) Type() string {
	return "log"
}

func (x *LogNode) New() types.Node {
	return &LogNode{Config: LogNodeConfiguration{
		JsScript: `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
	}}
}

// Init 初始化
func (x *LogNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		jsScript := fmt.Sprintf(JsLogFuncTemplate, x.Config.JsScript)
		x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	}
	x.logger = ruleConfig.Logger
	return err
}

// OnMsg 处理消息
func (x *LogNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 准备传递给JS脚本的数据，支持多种数据类型
	data := base.NodeUtils.PrepareJsData(msg)

	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// 执行JavaScript脚本，传递msg、metadata、msgType和dataType四个参数
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

// Destroy 销毁
func (x *LogNode) Destroy() {
	x.jsEngine.Stop()
}
