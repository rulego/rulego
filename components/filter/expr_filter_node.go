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

//Example of rule chain node configuration:
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

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
)

// init registers the ExprFilterNode component
// init registers the ExprFilterNode component with the default registry.
func init() {
	Registry.Add(&ExprFilterNode{})
}

// ExprFilterNodeConfiguration ExprFilterNode configuration structure
// ExprFilterNodeConfiguration defines the configuration structure for the ExprFilterNode component.
type ExprFilterNodeConfiguration struct {
	// Expr is the expression to evaluate for filtering. Must return a boolean.
	// Available variables: id, ts, data, msg, metadata, type, dataType
	Expr string `json:"expr" label:"Expression" desc:"Boolean expression for filtering. Available: id, ts, data, msg, metadata, type, dataType. Example: msg.temperature > 50" required:"true"`
}

// ExprFilterNode uses expr-lang expressions for boolean evaluation to filter out the filtering component of messages
// ExprFilterNode filters messages using expr-lang expressions for boolean evaluation.
//
// Core algorithm:
// Core Algorithm:
// 1. Compile expression to optimized program during initialization
// 2. Prepare the message evaluation environment (id, ts, data, msg, metadata, etc.) - Prepare the message evaluation environment
// 3. Execute compiled expression program
// 4. Route message to True/False relation based on boolean result - Route message to True/False relation based on boolean result
//
// Expression language features:
//   - Arithmetic operators: +, -, *, /, % - Arithmetic operators
//   - Comparison operators: ==,!=, <, <=, >, >= - Comparison operators
//   - Logical operator: &&, ||,! - Logical operators
//   - String operations: contains, startsWith, endsWith - String operations
//   - Mathematical functions: abs, ceil, floor, round - Mathematical functions
type ExprFilterNode struct {
	// Config Expression Filter Configuration
	// Config holds the expression filter configuration
	Config ExprFilterNodeConfiguration

	// exprTemplate executes the compiled expression template
	// exprTemplate is the compiled expression template for execution
	exprTemplate el.Template
}

// Type returns the component type
// Type returns the component type identifier.
func (x *ExprFilterNode) Type() string {
	return "exprFilter"
}

// New creates an instance
// New creates a new instance.
func (x *ExprFilterNode) New() types.Node {
	return &ExprFilterNode{Config: ExprFilterNodeConfiguration{
		Expr: "",
	}}
}

// Init initializes components, verifies and compiles expressions
// Init initializes the component.
func (x *ExprFilterNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if strings.TrimSpace(x.Config.Expr) == "" {
		return fmt.Errorf("expr can not be empty")
	}
	if err == nil {
		if template, err := el.NewExprTemplate(x.Config.Expr); err != nil {
			return fmt.Errorf("failed to create expression template: %w", err)
		} else {
			x.exprTemplate = template
		}
	}
	return err
}

// OnMsg processes messages by evaluating compiled expressions to filter messages
// OnMsg processes incoming messages by evaluating the compiled expression.
func (x *ExprFilterNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	if out, err := x.exprTemplate.Execute(evn); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		if result, ok := out.(bool); ok && result {
			ctx.TellNext(msg, types.True)
		} else {
			ctx.TellNext(msg, types.False)
		}
	}
}

// Desc returns the component description
func (x *ExprFilterNode) Desc() string {
	return "Filter messages using expr-lang expressions. Expression must return boolean. Routes to True/False. Variables: id, ts, data, msg, metadata, type, dataType"
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *ExprFilterNode) Destroy() {
}
