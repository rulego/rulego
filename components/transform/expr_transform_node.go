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
//	"id": "s1",
//	"type": "exprTransform",
//	"name": "表达式转换",
//	"debugMode": false,
//		"configuration": {
//			"mapping": {
//			"name":        "upper(msg.name)",
//			"tmp":         "msg.temperature",
//			"alarm":       "msg.temperature>50",
//			"productType": "metaData.productType"
//		}
//	}
//}
import (
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"strings"
)

func init() {
	Registry.Add(&ExprTransformNode{})
}

// ExprTransformNodeConfiguration 节点配置
type ExprTransformNodeConfiguration struct {
	//转换表达式，转换结果替换到msg 转到下一个节点。
	Expr string
	//多个字段转换表达式，格式(字段:转换表达式)，多个转换结果转换成json字符串转到下一个节点。如果Mapping和Expr同时存在，优先使用Expr
	Mapping map[string]string
}

// ExprTransformNode 使用expr表达式转换或者创建新的msg
// 如果config.Expr有值，则把转换结果替换到msg 转到下一个节点
// 如果config.Mapping有值，则把多个字段转换结果转换成json替换到msg 转到下一个节点
// 如果Mapping和Expr同时存在，优先使用config.Expr
// 多个字段转换msg结构如下：
//
//	{
//	  fieldKey1:fieldValue1
//	  fieldKey2:fieldValue2
//	}
//
// fieldValue 可以使用 expr 从当前的msg或者metadata中获取值，例如:
//
//	"configuration": {
//		"mapping": {
//		"name":        "upper(msg.name)",
//		"tmp":         "msg.temperature",
//		"alarm":       "msg.temperature>50",
//		"productType": "metaData.productType",
//	}
//
// 通过`id`变量访问消息id
// 通过`ts`变量访问消息时间戳
// 通过`data`变量访问消息原始数据
// 通过`msg`变量访问转换后消息体，如果消息的dataType是json类型，可以通过 `msg.XX`方式访问msg的字段。例如:`msg.temperature > 50;`
// 通过`metadata`变量访问消息元数据。例如 `metadata.customerName`
// 通过`type`变量访问消息类型
// 通过`dataType`变量访问数据类型
type ExprTransformNode struct {
	//节点配置
	Config         ExprTransformNodeConfiguration
	program        *vm.Program
	programMapping map[string]*vm.Program
}

// Type 组件类型
func (x *ExprTransformNode) Type() string {
	return "exprTransform"
}

func (x *ExprTransformNode) New() types.Node {
	return &ExprTransformNode{Config: ExprTransformNodeConfiguration{}}
}

// Init 初始化
func (x *ExprTransformNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if exprV := strings.TrimSpace(x.Config.Expr); exprV != "" {
			if program, err := expr.Compile(exprV, expr.AllowUndefinedVariables()); err != nil {
				return err
			} else {
				x.program = program
			}
		} else {
			x.programMapping = make(map[string]*vm.Program)
			for k, v := range x.Config.Mapping {
				if program, err := expr.Compile(v, expr.AllowUndefinedVariables()); err != nil {
					return err
				} else {
					x.programMapping[k] = program
				}
			}
		}

	}
	return err
}

// OnMsg 处理消息
func (x *ExprTransformNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn, err := components.NodeUtils.GetEvn(ctx, msg)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	var result interface{}
	var exprVm = vm.VM{}
	if x.program != nil {
		if out, err := exprVm.Run(x.program, evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			result = out
		}
	} else {
		mapResult := make(map[string]interface{})
		for fieldName, program := range x.programMapping {
			if out, err := exprVm.Run(program, evn); err != nil {
				ctx.TellFailure(msg, err)
				return
			} else {
				mapResult[fieldName] = out
			}
		}
		result = mapResult
		msg.DataType = types.JSON
	}

	if newValue, err := str.ToStringMaybeErr(result); err == nil {
		msg.Data = newValue
		ctx.TellSuccess(msg)
	} else {
		ctx.TellFailure(msg, err)
	}

}

// Destroy 销毁
func (x *ExprTransformNode) Destroy() {
}
