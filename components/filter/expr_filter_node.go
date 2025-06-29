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
//        "type": "exprFilter",
//        "name": "表达式过滤器",
//        "debugMode": false,
//        "configuration": {
//          "expr": "msg.temperature > 50"
//        }
//      }
import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

// init 注册ExprFilterNode组件
// init registers the ExprFilterNode component with the default registry.
func init() {
	Registry.Add(&ExprFilterNode{})
}

// ExprFilterNodeConfiguration ExprFilterNode配置结构
// ExprFilterNodeConfiguration defines the configuration structure for the ExprFilterNode component.
type ExprFilterNodeConfiguration struct {
	// Expr 用于过滤评估的表达式，必须返回布尔值
	// Expr contains the expression to evaluate for filtering.
	// The expression has access to the following variables:
	//   - id: Message ID (string)
	//   - ts: Message timestamp (int64)
	//   - data: Original message data (string)
	//   - msg: Parsed message body (object for JSON, string otherwise)
	//   - metadata: Message metadata (object with key-value pairs)
	//   - type: Message type (string)
	//   - dataType: Message data type (string)
	//
	// The expression must evaluate to a boolean value:
	//   - true: Message passes the filter (routed to "True" relation)
	//   - false: Message fails the filter (routed to "False" relation)
	//
	// Example expressions:
	// 表达式示例：
	//   - "msg.temperature > 50"
	//   - "metadata.deviceType == 'sensor' && msg.value > 100"
	//   - "type == 'TELEMETRY' && data contains 'alarm'"
	//   - "ts > 1640995200 && msg.status == 'active'"
	Expr string
}

// ExprFilterNode 使用expr-lang表达式进行布尔评估来过滤消息的过滤组件
// ExprFilterNode filters messages using expr-lang expressions for boolean evaluation.
//
// 核心算法：
// Core Algorithm:
// 1. 初始化时编译表达式为优化的程序 - Compile expression to optimized program during initialization
// 2. 准备消息评估环境（id、ts、data、msg、metadata等）- Prepare message evaluation environment
// 3. 执行编译的表达式程序 - Execute compiled expression program
// 4. 根据布尔结果路由消息到True/False关系 - Route message to True/False relation based on boolean result
//
// 表达式语言特性 - Expression language features:
//   - 算术运算符：+, -, *, /, % - Arithmetic operators
//   - 比较运算符：==, !=, <, <=, >, >= - Comparison operators
//   - 逻辑运算符：&&, ||, ! - Logical operators
//   - 字符串操作：contains, startsWith, endsWith - String operations
//   - 数学函数：abs, ceil, floor, round - Mathematical functions
type ExprFilterNode struct {
	// Config 表达式过滤器配置
	// Config holds the expression filter configuration
	Config ExprFilterNodeConfiguration

	// program 用于高效执行的编译表达式
	// program is the compiled expression for efficient execution
	program *vm.Program
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *ExprFilterNode) Type() string {
	return "exprFilter"
}

// New 创建新实例
// New creates a new instance.
func (x *ExprFilterNode) New() types.Node {
	return &ExprFilterNode{Config: ExprFilterNodeConfiguration{
		Expr: "",
	}}
}

// Init 初始化组件，验证并编译表达式
// Init initializes the component.
func (x *ExprFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if strings.TrimSpace(x.Config.Expr) == "" {
		return fmt.Errorf("expr can not be empty")
	}
	if err == nil {
		x.program, err = expr.Compile(x.Config.Expr, expr.AllowUndefinedVariables())
	}
	return err
}

// OnMsg 处理消息，通过评估编译的表达式来过滤消息
// OnMsg processes incoming messages by evaluating the compiled expression.
func (x *ExprFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	if out, err := vm.Run(x.program, evn); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if result, ok := out.(bool); ok && result {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *ExprFilterNode) Destroy() {
}
