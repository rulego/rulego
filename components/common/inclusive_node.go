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

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/components/filter"
)

// init registers the InclusiveNode component
// Init registers the InclusiveNode component with the default registry.
func init() {
	filter.Registry.Add(&InclusiveNode{})
}

// InclusiveNode is a filtering component that routes all matching branches simultaneously based on expression evaluation
// InclusiveNode embeds SwitchNode to reuse initialization and configuration, and overrides routing behavior.
type InclusiveNode struct {
	SwitchNode
}

// Type returns the component type identifier
// Type returns the component type identifier.
func (x *InclusiveNode) Type() string {
	return "inclusive"
}

// New creates a component instance
// New creates a new component instance with default demo cases.
func (x *InclusiveNode) New() types.Node {
	return &InclusiveNode{SwitchNode: SwitchNode{Config: SwitchNodeConfiguration{
		Cases: []Case{
			{Case: "msg.temperature>=20 && msg.temperature<=50", Then: "Case1"},
			{Case: "msg.temperature>50", Then: "Case2"},
		},
	}}}
}

// Init initializes components and compiles all case expressions
// Init initializes the component by compiling all case expressions into programs.
// Init reuses the initialization logic of SwitchNode
// Init reuses SwitchNode.Init for compiling case expressions.
func (x *InclusiveNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return x.SwitchNode.Init(ruleConfig, configuration)
}

// OnMsg handles messages: evaluates all cases and routes messages to all matching relationships; If there is no match, routing is done to the default relationship
// OnMsg processes the incoming message by evaluating all case expressions.
// It routes to each relation whose expression evaluates to true. If none matches, routes to Default.
func (x *InclusiveNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn := base.NodeUtils.GetEvn(ctx, msg)

	matched := false
	var relationTypes []string
	for _, p := range x.Cases {
		out, err := p.template.Execute(evn)
		if err != nil {
			ctx.TellFailure(msg, err)
			return
		}
		if result, ok := out.(bool); ok && result {
			matched = true
			relationTypes = append(relationTypes, p.relationType)
		}
	}
	if matched {
		ctx.TellNext(msg, relationTypes...)
	} else {
		ctx.TellNext(msg, types.DefaultRelationType)
	}
}

// Destroy to clean up resources
// Destroy cleans up resources.
func (x *InclusiveNode) Destroy() {
}

// Desc returns the component description
func (x *InclusiveNode) Desc() string {
	return "Inclusive gateway that evaluates all cases and routes to ALL matching branches simultaneously. Unmatched goes to Default. Same config as switch node"
}
