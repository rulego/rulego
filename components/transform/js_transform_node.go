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

// 规则链节点配置示例：
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
	// JsTransformDefaultScript 默认的JS脚本，直接返回原始消息内容
	JsTransformDefaultScript = "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
	// JsTransformType 组件类型标识符
	JsTransformType = "jsTransform"
	// JsTransformFuncTemplate JS函数模板，用于包装用户脚本，新增dataType参数
	JsTransformFuncTemplate = "function Transform(msg, metadata, msgType, dataType) { %s }"
	// JsTransformFuncName JS引擎中执行的函数名称
	JsTransformFuncName = "Transform"
)

// JsTransformReturnFormatErr JS脚本返回值格式错误，期望返回map类型
// 正确格式：return {'msg':msg,'metadata':metadata,'msgType':msgType}
var JsTransformReturnFormatErr = errors.New("return the value is not a map")

func init() {
	Registry.Add(&JsTransformNode{})
}

// JsTransformNodeConfiguration JS转换节点配置结构
type JsTransformNodeConfiguration struct {
	// JsScript 用户自定义的JavaScript脚本内容
	// 用于对消息的msg、metadata、msgType、dataType进行转换和增强
	// 脚本会被包装成完整函数：function Transform(msg, metadata, msgType, dataType) { ${JsScript} }
	// 必须返回格式：return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};
	// 其中dataType字段是可选的，如果返回则会更新消息的数据类型
	JsScript string
}

// JsTransformNode JavaScript消息转换节点
// 使用JavaScript脚本对消息的metadata、msg、msgType或dataType进行转换处理
//
// JavaScript函数接收4个参数：
//   - msg: 消息的payload数据（JSON类型会解析为对象，BINARY类型为Uint8Array，其他为字符串）
//   - metadata: 消息的元数据
//   - msgType: 消息的类型
//   - dataType: 消息的数据类型（JSON、TEXT、BINARY等）
//
// 返回结构必须为：return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};
// 其中dataType字段是可选的，如果返回则会更新消息的数据类型
// msg字段支持返回Uint8Array或数字数组，会自动转换为二进制数据
// 脚本执行成功时，消息发送到Success链；执行失败时，发送到Failure链
type JsTransformNode struct {
	// Config 节点配置信息
	Config JsTransformNodeConfiguration
	// jsEngine JavaScript执行引擎实例
	jsEngine types.JsEngine
	// passThrough 是否启用直通模式（跳过JS脚本执行，直接转发消息）
	passThrough bool
}

// Type 返回组件类型标识符
func (x *JsTransformNode) Type() string {
	return JsTransformType
}

// New 创建新的JS转换节点实例，使用默认配置
func (x *JsTransformNode) New() types.Node {
	return &JsTransformNode{Config: JsTransformNodeConfiguration{
		JsScript: JsTransformDefaultScript,
	}}
}

// Init 初始化节点，解析配置并设置执行模式
func (x *JsTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// 解析节点配置
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	// 检查是否启用直通模式（默认脚本或空脚本时跳过JS执行）
	script := strings.TrimSpace(x.Config.JsScript)
	if script == "" || script == JsTransformDefaultScript {
		x.passThrough = true
		return nil
	}

	// 非直通模式：初始化JavaScript执行引擎
	jsScript := fmt.Sprintf(JsTransformFuncTemplate, x.Config.JsScript)
	x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	return err
}

// OnMsg 处理接收到的消息
func (x *JsTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 直通模式：跳过JS脚本执行，直接转发原始消息
	if x.passThrough {
		ctx.TellNext(msg, types.Success)
		return
	}

	// 准备传递给JS脚本的数据
	data := base.NodeUtils.PrepareJsData(msg)

	// 执行JavaScript脚本进行消息转换
	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// 调用JS引擎，传递msg、metadata、msgType和dataType四个参数
	out, err := x.jsEngine.Execute(ctx, JsTransformFuncName, data, metadataValues, msg.Type, msg.DataType)
	if err != nil {
		// JS执行失败，发送到Failure链
		ctx.TellFailure(msg, err)
		return
	}

	// 处理JS脚本的执行结果
	x.processJsResult(ctx, msg, out)
}

// processJsResult 处理JavaScript脚本的执行结果并更新消息
func (x *JsTransformNode) processJsResult(ctx types.RuleContext, msg types.RuleMsg, out interface{}) {
	// 验证返回值格式，必须是map类型
	formatData, ok := out.(map[string]interface{})
	if !ok {
		ctx.TellFailure(msg, JsTransformReturnFormatErr)
		return
	}

	// 更新数据类型（如果JS脚本中修改了dataType）
	if formatDataType, ok := formatData[types.DataTypeKey]; ok {
		if dataTypeStr := str.ToString(formatDataType); dataTypeStr != "" {
			msg.DataType = types.DataType(dataTypeStr)
		}
	}

	// 更新消息类型（如果JS脚本中修改了msgType）
	if formatMsgType, ok := formatData[types.MsgTypeKey]; ok {
		msg.Type = str.ToString(formatMsgType)
	}

	// 更新消息元数据（如果JS脚本中修改了metadata）
	if formatMetaData, ok := formatData[types.MetadataKey]; ok {
		msg.Metadata.ReplaceAll(str.ToStringMapString(formatMetaData))
	}

	// 更新消息数据（如果JS脚本中修改了msg）
	if formatMsgData, ok := formatData[types.MsgKey]; ok {
		// 检查是否是字节数组类型（JavaScript Uint8Array转换后的结果）
		if byteData, isByteSlice := formatMsgData.([]byte); isByteSlice {
			// 直接设置二进制数据
			msg.SetBytes(byteData)
		} else if byteData, isByteArray := formatMsgData.([]interface{}); isByteArray {
			// 处理JavaScript数组转换为[]interface{}的情况 - 尝试转换为字节数组
			bytes := make([]byte, len(byteData))
			isValidByteArray := true
			for i, v := range byteData {
				if byteVal, ok := v.(float64); ok {
					bytes[i] = byte(byteVal)
				} else if byteVal, ok := v.(int64); ok {
					bytes[i] = byte(byteVal)
				} else if byteVal, ok := v.(int); ok {
					bytes[i] = byte(byteVal)
				} else {
					// 数组元素不是数字，直接转字符串处理
					isValidByteArray = false
					break
				}
			}

			if isValidByteArray {
				// 设置转换后的字节数据
				msg.SetBytes(bytes)
			} else {
				// 直接转字符串处理
				if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
					msg.SetData(newValue)
				} else {
					ctx.TellFailure(msg, err)
					return
				}
			}
		} else {
			// 普通数据类型，转换为字符串
			if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
				// 使用msg.SetData方法设置数据
				msg.SetData(newValue)
			} else {
				// 数据转换失败，发送到Failure链
				ctx.TellFailure(msg, err)
				return
			}
		}
	}

	// 处理成功，发送转换后的消息到Success链
	ctx.TellNext(msg, types.Success)
}

// Destroy 销毁节点，释放JavaScript引擎资源
func (x *JsTransformNode) Destroy() {
	if x.jsEngine != nil {
		x.jsEngine.Stop()
	}
}
