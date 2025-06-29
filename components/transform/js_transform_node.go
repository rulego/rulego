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
	// JsTransformDefaultScript 默认JS脚本，直接返回原始消息
	JsTransformDefaultScript = "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
	// JsTransformType 组件类型
	JsTransformType = "jsTransform"
	// JsTransformFuncTemplate JS函数模板
	JsTransformFuncTemplate = "function Transform(msg, metadata, msgType, dataType) { %s }"
	// JsTransformFuncName JS函数名
	JsTransformFuncName = "Transform"
)

// JsTransformReturnFormatErr JS脚本返回值必须是map类型
var JsTransformReturnFormatErr = errors.New("return the value is not a map")

func init() {
	Registry.Add(&JsTransformNode{})
}

// JsTransformNodeConfiguration JS转换节点配置
// JsTransformNodeConfiguration defines the configuration for JsTransformNode.
type JsTransformNodeConfiguration struct {
	// JsScript JavaScript脚本，用于转换消息
	// JsScript JavaScript script for message transformation.
	// 函数参数：msg, metadata, msgType, dataType
	// Function parameters: msg, metadata, msgType, dataType
	// 必须返回：{'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType}
	// Must return: {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType}
	// 支持修改消息的任意字段，dataType字段可选
	// Supports modifying any message fields, dataType field is optional
	JsScript string
}

// JsTransformNode JavaScript消息转换节点，使用JavaScript脚本对消息进行转换处理
// JsTransformNode is a JavaScript message transformation component that processes messages using JavaScript scripts.
//
// 脚本环境：
// Script Environment:
//   - 函数签名：function Transform(msg, metadata, msgType, dataType) - Function signature
//   - 输入参数：消息数据、元数据映射、消息类型、数据类型 - Input params: message data, metadata map, message type, data type
//   - 返回格式：{'msg':newMsg,'metadata':newMetadata,'msgType':newType,'dataType':newDataType} - Return format
//   - 可选字段：dataType字段可省略，保持原值 - Optional fields: dataType can be omitted to keep original
//
// 内置变量：
// Built-in Variables:
//   - $ctx: 上下文对象，提供缓存操作 - Context object providing cache operations
//   - global: 全局配置属性 - Global configuration properties
//   - vars: 规则链变量 - Rule chain variables
//   - UDF函数: 用户自定义函数 - User-defined functions
//
// 缓存操作示例：
// Cache Operation Examples:
//
//	let cache = $ctx.ChainCache(); // 获取规则链级别缓存 - Get chain-level cache
//	// let cache = $ctx.GlobalCache(); // 获取全局级别缓存 - Get global-level cache
//	cache.Set("key", "value"); // 设置缓存，永不过期 - Set cache, never expires
//	cache.Set("key2", "value2", "10m"); // 设置缓存，10分钟后过期 - Set cache, expires in 10 minutes
//	let value = cache.Get("key"); // 获取缓存 - Get cache value
//	let exists = cache.Has("key"); // 判断缓存是否存在 - Check if cache exists
//	cache.Delete("key"); // 删除缓存 - Delete cache
//	let values = cache.GetByPrefix("prefix_"); // 获取前缀匹配的缓存 - Get caches by prefix
//	cache.DeleteByPrefix("prefix_"); // 删除前缀匹配的缓存 - Delete caches by prefix
//
// 配置示例 - Configuration example:
//
//	{
//	  "jsScript": "msg.temperature = msg.temperature * 9/5 + 32; metadata.unit = 'Fahrenheit'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//	}
//
// 使用场景 - Use cases:
//   - 数据格式转换：JSON字段重组、单位换算 - Data format conversion: JSON field reorganization, unit conversion
//   - 消息富化：添加计算字段、时间戳、标识符 - Message enrichment: add calculated fields, timestamps, identifiers
//   - 条件转换：基于消息内容的动态转换逻辑 - Conditional transformation: dynamic conversion logic based on message content
//   - 协议适配：不同数据协议间的消息转换 - Protocol adaptation: message conversion between different data protocols
type JsTransformNode struct {
	// Config 节点配置
	// Config holds the node configuration
	Config JsTransformNodeConfiguration
	// jsEngine JavaScript执行引擎
	// jsEngine JavaScript execution engine
	jsEngine types.JsEngine
	// passThrough 直通模式，跳过JS执行
	// passThrough direct pass-through mode, skip JS execution
	passThrough bool
}

// Type 返回组件类型
// Type returns the component type.
func (x *JsTransformNode) Type() string {
	return JsTransformType
}

// New 创建新实例
// New creates a new instance.
func (x *JsTransformNode) New() types.Node {
	return &JsTransformNode{Config: JsTransformNodeConfiguration{
		JsScript: JsTransformDefaultScript,
	}}
}

// Init 初始化节点
// Init initializes the node.
func (x *JsTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err != nil {
		return err
	}

	// 检查是否启用直通模式
	script := strings.TrimSpace(x.Config.JsScript)
	if script == "" || script == JsTransformDefaultScript {
		x.passThrough = true
		return nil
	}

	// 初始化JavaScript执行引擎
	jsScript := fmt.Sprintf(JsTransformFuncTemplate, x.Config.JsScript)
	x.jsEngine, err = js.NewGojaJsEngine(ruleConfig, jsScript, base.NodeUtils.GetVars(configuration))
	return err
}

// OnMsg 处理消息，使用JavaScript脚本转换消息内容
// OnMsg processes messages using JavaScript script for message transformation.
func (x *JsTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 直通模式：直接转发
	if x.passThrough {
		ctx.TellNext(msg, types.Success)
		return
	}

	// 准备传递给JS脚本的数据
	data := base.NodeUtils.PrepareJsData(msg)

	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
	} else {
		metadataValues = make(map[string]string)
	}

	// 执行JavaScript脚本
	out, err := x.jsEngine.Execute(ctx, JsTransformFuncName, data, metadataValues, msg.Type, msg.DataType)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	// 处理执行结果
	x.processJsResult(ctx, msg, out)
}

// processJsResult 处理JavaScript执行结果
// processJsResult processes JavaScript execution results.
func (x *JsTransformNode) processJsResult(ctx types.RuleContext, msg types.RuleMsg, out interface{}) {
	// 验证返回值格式
	formatData, ok := out.(map[string]interface{})
	if !ok {
		ctx.TellFailure(msg, JsTransformReturnFormatErr)
		return
	}

	// 更新数据类型
	if formatDataType, ok := formatData[types.DataTypeKey]; ok {
		if dataTypeStr := str.ToString(formatDataType); dataTypeStr != "" {
			msg.DataType = types.DataType(dataTypeStr)
		}
	}

	// 更新消息类型
	if formatMsgType, ok := formatData[types.MsgTypeKey]; ok {
		msg.Type = str.ToString(formatMsgType)
	}

	// 更新元数据
	if formatMetaData, ok := formatData[types.MetadataKey]; ok {
		msg.Metadata.ReplaceAll(str.ToStringMapString(formatMetaData))
	}

	// 更新消息数据
	if formatMsgData, ok := formatData[types.MsgKey]; ok {
		// 处理字节数组
		if byteData, isByteSlice := formatMsgData.([]byte); isByteSlice {
			msg.SetBytes(byteData)
		} else if byteData, isByteArray := formatMsgData.([]interface{}); isByteArray {
			// 尝试转换为字节数组
			bytes := make([]byte, len(byteData))
			isValidByteArray := true
			for i, v := range byteData {
				var byteVal float64
				var isNumber bool

				if val, ok := v.(float64); ok {
					byteVal = val
					isNumber = true
				} else if val, ok := v.(int64); ok {
					byteVal = float64(val)
					isNumber = true
				} else if val, ok := v.(int); ok {
					byteVal = float64(val)
					isNumber = true
				}

				if isNumber {
					// 边界检查
					if byteVal < 0 || byteVal > 255 || byteVal != float64(int(byteVal)) {
						ctx.TellFailure(msg, fmt.Errorf("byte array element at index %d has invalid value %v: must be integer in range 0-255", i, byteVal))
						return
					}
					bytes[i] = byte(byteVal)
				} else {
					isValidByteArray = false
					break
				}
			}

			if isValidByteArray {
				msg.SetBytes(bytes)
			} else {
				// 转字符串处理
				if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
					msg.SetData(newValue)
				} else {
					ctx.TellFailure(msg, err)
					return
				}
			}
		} else {
			// 普通数据类型
			if newValue, err := str.ToStringMaybeErr(formatMsgData); err == nil {
				msg.SetData(newValue)
			} else {
				ctx.TellFailure(msg, err)
				return
			}
		}
	}

	// 发送到Success链
	ctx.TellNext(msg, types.Success)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *JsTransformNode) Destroy() {
	if x.jsEngine != nil {
		x.jsEngine.Stop()
	}
}
