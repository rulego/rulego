/*
 * Copyright 2025 The RuleGo Authors.
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

package common

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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/components/filter"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

// init 注册SwitchNode组件
// init registers the SwitchNode component with the default registry.
func init() {
	filter.Registry.Add(&SwitchNode{})
}

// SwitchNodeConfiguration SwitchNode配置结构
type SwitchNodeConfiguration struct {
	// Cases 包含路由决策的条件表达式列表
	// 按顺序评估，第一个匹配的 case 决定路由。无匹配走 Default。
	Cases []Case `json:"cases" label:"Cases" desc:"Condition-expression pairs evaluated in order. First match determines route. then value is the connection type" required:"true"`
}

// Case 表示消息路由的单个条件-动作对
type Case struct {
	// Case is the expression to evaluate. Available variables: msg, metadata, type
	Case string `json:"case" label:"Condition" desc:"Boolean expression, e.g. msg.temperature > 50. Variables: msg, metadata, type" required:"true"`
	// Then is the connection type name when this case matches
	Then string `json:"then" label:"Then" desc:"Connection type name when matched, corresponds to connections type" required:"true"`
}

// SwitchNode 基于表达式评估提供条件消息路由的过滤组件
// SwitchNode provides conditional message routing based on expression evaluation.
//
// 核心算法：
// Core Algorithm:
// 1. 初始化时编译所有case表达式为优化程序 - Compile all case expressions to optimized programs during initialization
// 2. 按顺序评估每个case表达式 - Evaluate each case expression sequentially
// 3. 第一个评估为true的case决定路由 - First case that evaluates to true determines routing
// 4. 无匹配时路由到"Default"关系 - Route to "Default" relation if no matches
//
// 评估逻辑 - Evaluation logic:
//   - 按配置中出现的顺序评估case - Cases evaluated in configuration order
//   - 在第一个成功匹配时停止评估 - Evaluation stops at first successful match
//   - 布尔true结果触发路由到case关系 - Boolean true result triggers routing to case relation
//   - 无匹配导致路由到默认关系 - No matches result in routing to default relation
//
// 表达式语言特性 - Expression language features:
//   - 算术运算符：+, -, *, /, % - Arithmetic operators
//   - 比较运算符：==, !=, <, <=, >, >= - Comparison operators
//   - 逻辑运算符：&&, ||, ! - Logical operators
//   - 字符串操作：contains, startsWith, endsWith - String operations
//   - 数学函数：abs, ceil, floor, round - Mathematical functions
//
// 性能优化 - Performance optimization:
//   - 表达式在初始化期间编译一次 - Expressions compiled once during initialization
//   - 早期终止减少不必要的评估 - Early termination reduces unnecessary evaluations
//   - 按概率排序case以获得最佳性能 - Order cases by probability for optimal performance
type SwitchNode struct {
	// Config 开关节点配置
	// Config holds the switch node configuration
	Config SwitchNodeConfiguration

	// Cases 用于高效评估的编译case程序
	// Cases contains the compiled case programs for efficient evaluation
	Cases []*caseProgram
}

// caseProgram 表示编译的case表达式及其目标关系
// caseProgram represents a compiled case expression with its target relation.
type caseProgram struct {
	// relationType 此case的目标关系名称
	// relationType is the target relation name for this case
	relationType string

	// template 用于评估的编译模板
	template el.Template
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *SwitchNode) Type() string {
	return "switch"
}

// New 创建新实例
// New creates a new instance.
func (x *SwitchNode) New() types.Node {
	return &SwitchNode{Config: SwitchNodeConfiguration{
		Cases: []Case{
			{Case: "msg.temperature>=20 && msg.temperature<=50", Then: "Case1"},
			{Case: "msg.temperature>50", Then: "Case2"},
		},
	}}
}

// Init 初始化组件，编译所有case表达式
// Init initializes the component.
func (x *SwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		x.Cases = nil
		for _, item := range x.Config.Cases {
			if template, err := el.NewExprTemplate(item.Case); err != nil {
				return err
			} else {
				x.Cases = append(x.Cases, &caseProgram{
					relationType: item.Then,
					template:     template,
				})
			}
		}
	}
	return err
}

// OnMsg 处理消息，按顺序评估case表达式并路由到第一个匹配的case或默认关系
// OnMsg processes incoming messages by evaluating case expressions sequentially.
func (x *SwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	for _, p := range x.Cases {
		if out, err := p.template.Execute(evn); err != nil {
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
	ctx.TellNext(msg, types.DefaultRelationType)
}

// Desc returns the component description
func (x *SwitchNode) Desc() string {
	return "Exclusive conditional routing. Evaluates cases in order, first match determines route. then value is the connection type. Unmatched goes to Default"
}

// Destroy 清理资源
func (x *SwitchNode) Destroy() {
}
