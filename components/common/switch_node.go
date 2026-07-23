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

//Example of rule chain node configuration:
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

// init registers the SwitchNode component
// init registers the SwitchNode component with the default registry.
func init() {
	filter.Registry.Add(&SwitchNode{})
}

// SwitchNodeConfiguration SwitchNode configuration structure
type SwitchNodeConfiguration struct {
	// Cases contains a list of conditional expressions for routing decisions
	// Sequential evaluation, and the first matching case determines the route. No match goes to default.
	Cases []Case `json:"cases" label:"Cases" desc:"Condition-expression pairs evaluated in order. First match determines route. then value is the connection type" required:"true"`
}

// Case represents a single condition-action pair for message routing
type Case struct {
	// Case is the expression to evaluate. Available variables: msg, metadata, type
	Case string `json:"case" label:"Condition" desc:"Boolean expression, e.g. msg.temperature > 50. Variables: msg, metadata, type" required:"true"`
	// Then is the connection type name when this case matches
	Then string `json:"then" label:"Then" desc:"Connection type name when matched, corresponds to connections type" required:"true"`
}

// SwitchNode provides filtering components for conditional message routing based on expression evaluation
// SwitchNode provides conditional message routing based on expression evaluation.
//
// Core algorithm:
// Core Algorithm:
// 1. Compile all case expressions to optimize programs during initialization
// 2. Evaluate each case expression sequentially
// 3. First case that evaluates to true determines routing
// 4. Route to "Default" relation if there are no matches - Route to "Default" relation if there are no matches
//
// Evaluation logic:
//   - Cases evaluated in configuration order - Cases evaluated in configuration order
//   - Evaluation stops at first successful match
//   - Boolean true result triggers routing to case relation
//   - No matches result in routing to default relation
//
// Expression language features:
//   - Arithmetic operators: +, -, *, /, % - Arithmetic operators
//   - Comparison operators: ==,!=, <, <=, >, >= - Comparison operators
//   - Logical operator: &&, ||,! - Logical operators
//   - String operations: contains, startsWith, endsWith - String operations
//   - Mathematical functions: abs, ceil, floor, round - Mathematical functions
//
// Performance optimization:
//   - Expressions compiled once during initialization
//   - Early termination reduces unnecessary evaluations
//   - Order cases by probability for optimal performance
type SwitchNode struct {
	// Config switch node configuration
	// Config holds the switch node configuration
	Config SwitchNodeConfiguration

	// Cases: Compiled case programs for efficient evaluation
	// Cases contains the compiled case programs for efficient evaluation
	Cases []*caseProgram
}

// caseProgram represents the compiled case expression and its target relationship
// caseProgram represents a compiled case expression with its target relation.
type caseProgram struct {
	// relationType The name of the target relationship in this case
	// relationType is the target relation name for this case
	relationType string

	// template: A compilation template used for evaluation
	template el.Template
}

// Type returns the component type
// Type returns the component type identifier.
func (x *SwitchNode) Type() string {
	return "switch"
}

// New creates an instance
// New creates a new instance.
func (x *SwitchNode) New() types.Node {
	return &SwitchNode{Config: SwitchNodeConfiguration{
		Cases: []Case{
			{Case: "msg.temperature>=20 && msg.temperature<=50", Then: "Case1"},
			{Case: "msg.temperature>50", Then: "Case2"},
		},
	}}
}

// Init initializes the component and compiles all case expressions
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

// OnMsg processes messages, evaluates case expressions in order, and routes them to the first matched case or default relationship
// OnMsg processes incoming messages by evaluating case expressions sequentially.
func (x *SwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	for _, p := range x.Cases {
		if out, err := p.template.Execute(evn); err != nil {
			// Evaluation failure is considered a mismatch, and the case is skipped to continue evaluation; Ultimately, Default is the backup plan.
			ctx.Config().Logger.Debugf("switch node [%s] case [%s] evaluation skipped: %v", ctx.GetSelfId(), p.relationType, err)
			continue
		} else {
			if result, ok := out.(bool); ok && result {
				ctx.TellNext(msg, p.relationType)
				return
			}
		}
	}
	//If no match, the default forwarding is to the Default chain
	ctx.TellNext(msg, types.DefaultRelationType)
}

// Desc returns the component description
func (x *SwitchNode) Desc() string {
	return "Exclusive conditional routing. Evaluates cases in order, first match determines route. then value is the connection type. Unmatched goes to Default"
}

// Destroy to clean up resources
func (x *SwitchNode) Destroy() {
}
