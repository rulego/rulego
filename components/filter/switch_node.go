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
//        "id": "s1",
//        "type": "switch",
//        "name": "switch",
//        "debugMode": false,
//        "configuration": {
//         "cases": [
//           {"case": "msg.temperature > 50", "then": "case1"}
//         ]
//        }
//      }
import (
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

func init() {
	Registry.Add(&SwitchNode{})
}

// SwitchNodeConfiguration 节点配置
type SwitchNodeConfiguration struct {
	// Cases 条件表达式列表，依次匹配case表达式，如果匹配成功，则终止匹配，把消息转发到对应的路由链，
	// 如果匹配不到，则转发到默认的"Default"链
	Cases []Case
}

type Case struct {
	// Case 条件表达式
	// 表达式允许使用以下变量:
	// 通过`id`变量访问消息id
	// 通过`ts`变量访问消息时间戳
	// 通过`data`变量访问消息原始数据
	// 通过`msg`变量访问转换后消息体，如果消息的dataType是json类型，可以通过 `msg.XX`方式访问msg的字段。例如:`msg.temperature > 50;`
	// 通过`metadata`变量访问消息元数据。例如 `metadata.customerName`
	// 通过`type`变量访问消息类型
	// 通过`dataType`变量访问数据类型
	Case string `json:"case"`
	// Then 路由关系，把消息转发到对应的路由链
	Then string `json:"then"`
}

// SwitchNode 依次匹配case表达式，如果匹配成功，则终止匹配，把消息转发到对应的路由链，如果匹配不到，则转发到默认的"Default"链
// 如果表达式执行失败则发送到`Failure`链
type SwitchNode struct {
	//节点配置
	Config SwitchNodeConfiguration
	Cases  []*caseProgram
}

type caseProgram struct {
	relationType string
	program      *vm.Program
}

// Type 组件类型
func (x *SwitchNode) Type() string {
	return "switch"
}
func (x *SwitchNode) New() types.Node {
	return &SwitchNode{Config: SwitchNodeConfiguration{
		Cases: []Case{
			{Case: "msg.temperature>=20 && msg.temperature<=50", Then: "Case1"},
			{Case: "msg.temperature>50", Then: "Case2"},
		},
	}}
}

// Init 初始化
func (x *SwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		x.Cases = nil
		for _, item := range x.Config.Cases {
			if program, err := expr.Compile(item.Case, expr.AllowUndefinedVariables(), expr.AsBool()); err == nil {
				x.Cases = append(x.Cases, &caseProgram{
					relationType: item.Then,
					program:      program,
				})
			}
		}
	}
	return err
}

// OnMsg 处理消息
func (x *SwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	for _, p := range x.Cases {
		if out, err := vm.Run(p.program, evn); err != nil {
			ctx.TellFailure(msg, err)
			return
		} else {
			if result, ok := out.(bool); ok && result {
				ctx.TellNext(msg, p.relationType)
				return
			}
		}
	}
	//没匹配到，默认转发到Default链
	ctx.TellNext(msg, KeyDefaultRelationType)
}

// Destroy 销毁
func (x *SwitchNode) Destroy() {
}
