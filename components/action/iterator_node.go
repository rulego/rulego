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

import (
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/js"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strings"
)

func init() {
	Registry.Add(&IteratorNode{})
}

// IteratorNodeConfiguration 节点配置
type IteratorNodeConfiguration struct {
	// 遍历字段名称，如果空，遍历整个msg，支持嵌套方式获取msg字段值，例如items.value、items
	FieldName string
	// 过滤item js脚本，可选，默认为空，匹配所有item
	// function ItemFilter(item,index,metadata)
	// item: 当前遍历的item
	// index: 如果是数组，则表示当前遍历的index，如果是map，则表示当前遍历的key
	JsScript string
}

// IteratorNode 遍历msg或者msg中指定字段item值到下一个节点，遍历字段值必须是`数组`或者`{key:value}`类型
// 如果item满足JsScript，则会把item数据通过`True`链发到下一个节点，否则通过`False`链发到下一个节点
// 如果找不到指定字段、js脚本执行失败或者遍历的对象不是`数组`或者`{key:value}`，则会把错误信息通过Failure链发到下一个节点
// 遍历结束后，通过Success链把原始msg发送到下一个节点
type IteratorNode struct {
	//节点配置
	Config   IteratorNodeConfiguration
	jsEngine types.JsEngine
}

// Type 组件类型
func (x *IteratorNode) Type() string {
	return "iterator"
}

func (x *IteratorNode) New() types.Node {
	return &IteratorNode{Config: IteratorNodeConfiguration{}}
}

// Init 初始化
func (x *IteratorNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	x.Config.JsScript = strings.TrimSpace(x.Config.JsScript)
	x.Config.FieldName = strings.TrimSpace(x.Config.FieldName)
	if err == nil && x.Config.JsScript != "" {
		jsScript := fmt.Sprintf("function ItemFilter(item,index,metadata) { %s }", x.Config.JsScript)
		x.jsEngine = js.NewGojaJsEngine(ruleConfig, jsScript, nil)
	}
	return err
}

// OnMsg 处理消息
func (x *IteratorNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data interface{} = msg.Data
	if msg.DataType == types.JSON {
		var dataMap interface{}
		if err := json.Unmarshal([]byte(msg.Data), &dataMap); err == nil {
			data = dataMap
		}
	}

	// 遍历指定字段
	if x.Config.FieldName != "" {
		data = maps.Get(data, x.Config.FieldName)
		if data == nil {
			ctx.TellFailure(msg, errors.New("field="+x.Config.FieldName+" not found"))
			return
		}
	}

	if arrayValue, ok := data.([]interface{}); ok {
		for index, item := range arrayValue {
			if err := x.executeItem(ctx, msg, item, index); err != nil {
				//出现错误中断遍历
				return
			}
		}
		ctx.TellSuccess(msg)
	} else if mapValue, ok := data.(map[string]interface{}); ok {
		for k, item := range mapValue {
			if err := x.executeItem(ctx, msg, item, k); err != nil {
				//出现错误中断遍历
				return
			}
		}
		ctx.TellSuccess(msg)
	} else {
		ctx.TellFailure(msg, errors.New("value is not array or {key:value} type"))
	}
}

// Destroy 销毁
func (x *IteratorNode) Destroy() {
}

// 处理每条item
func (x *IteratorNode) executeItem(ctx types.RuleContext, msg types.RuleMsg, item interface{}, index interface{}) error {
	if x.jsEngine != nil {
		if out, err := x.jsEngine.Execute("ItemFilter", item, index, msg.Metadata.Values()); err != nil {
			ctx.TellFailure(msg, err)
			//出现错误中断遍历
			return err
		} else if formatData, ok := out.(bool); ok && formatData {
			msg.Data = str.ToString(item)
			ctx.TellNext(msg, types.True)
		} else {
			msg.Data = str.ToString(item)
			ctx.TellNext(msg, types.False)
		}
	} else {
		msg.Data = str.ToString(item)
		ctx.TellNext(msg, types.True)
	}
	return nil
}
