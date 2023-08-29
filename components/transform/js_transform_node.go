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

//规则链节点配置示例：
//{
//        "id": "s2",
//        "type": "jsTransform",
//        "name": "转换",
//        "debugMode": false,
//        "configuration": {
//          "jsScript": "metadata['test']='test02';\n metadata['index']=52;\n msgType='TEST_MSG_TYPE2';\n  msg['aa']=66; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//        }
//      }
import (
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/js"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	string2 "github.com/rulego/rulego/utils/str"
)

func init() {
	Registry.Add(&JsTransformNode{})
}

//JsTransformNodeConfiguration 节点配置
type JsTransformNodeConfiguration struct {
	//JsScript 配置函数体脚本内容
	//对msg、metadata、msgType 进行转换、增强
	//完整脚本函数：
	//function Transform(msg, metadata, msgType) { ${JsScript} }
	//return {'msg':msg,'metadata':metadata,'msgType':msgType};
	JsScript string
}

//JsTransformNode 使用JavaScript更改消息metadata，msg或msgType
//JavaScript 函数接收3个参数：
//metadata:是消息的 metadata
//msg:是消息的payload
//msgType:是消息的 type
//法返回结构:return {'msg':msg,'metadata':metadata,'msgType':msgType};
//脚本执行成功，发送信息到`Success`链, 否则发到`Failure`链。
type JsTransformNode struct {
	config   JsTransformNodeConfiguration
	jsEngine types.JsEngine
}

//Type 组件类型
func (x *JsTransformNode) Type() string {
	return "jsTransform"
}

func (x *JsTransformNode) New() types.Node {
	return &JsTransformNode{}
}

//Init 初始化
func (x *JsTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.config)
	if err == nil {
		jsScript := fmt.Sprintf("function Transform(msg, metadata, msgType) { %s }", x.config.JsScript)
		x.jsEngine = js.NewGojaJsEngine(ruleConfig, jsScript, nil)
	}
	return err
}

//OnMsg 处理消息
func (x *JsTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	var data interface{} = msg.Data
	if msg.DataType == types.JSON {
		var dataMap interface{}
		if err := json.Unmarshal([]byte(msg.Data), &dataMap); err == nil {
			data = dataMap
		}
	}

	out, err := x.jsEngine.Execute("Transform", data, msg.Metadata.Values(), msg.Type)

	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		formatData, ok := out.(map[string]interface{})
		if ok {
			var errResult error
			if formatMsgType, ok := formatData[types.MsgTypeKey]; ok {
				msg.Type = string2.ToString(formatMsgType)
			}

			if formatMetaData, ok := formatData[types.MetadataKey]; ok {
				msg.Metadata = types.BuildMetadata(string2.ToStringMapString(formatMetaData))
			}

			if formatMsgData, ok := formatData[types.MsgKey]; ok {
				msg.Data = string2.ToString(formatMsgData)
			}

			//ctx.Config().Logger.Printf("jsTransform用时：%s", time.Since(start))
			if errResult == nil {
				ctx.TellNext(msg, types.Success)
			} else {
				ctx.TellNext(msg, types.Failure)
			}

		}
	}

	return err
}

//Destroy 销毁
func (x *JsTransformNode) Destroy() {
	x.jsEngine.Stop()
}
