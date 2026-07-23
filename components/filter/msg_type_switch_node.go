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
//        "id": "s2",
//        "type": "msgTypeSwitch",
//        "name": "消息路由"
//      }
import (
	"github.com/rulego/rulego/api/types"
)

// init registers the MsgTypeSwitchNode component
// init registers the MsgTypeSwitchNode component with the default registry.
func init() {
	Registry.Add(&MsgTypeSwitchNode{})
}

// MsgTypeSwitchNode routes messages to different output chains based on message type through filtering components
// MsgTypeSwitchNode routes messages to different output chains based on their message type.
//
// Core algorithm:
// Core Algorithm:
// 1. Extract message type from incoming message
// 2. Attempt to route to relation to matching message type
// 3. If no matching relation exists, route to default relation
//
// Routing logic:
//   - Primary: Route to relation matching message type
//   - Fallback: Route to configured default relation
//   - Relationship names are case-sensitive
//
// Configuration options:
//   - Global property "defaultRelationType": Custom default relation name
//   - If not configured, use "Default" as the fallback relation - If not configured, use "Default" as the fallback relation
//
// Use cases:
//   - Message categorization by type
//   - Type-specific processing workflows
//   - Message filtering and routing
//
// Routing examples:
//   - Message type "ALARM" - > routing to the "ALARM" relationship - Message type "ALARM" -> Routes to "ALARM" relation
//   - Message type "TELEMETRY" - > routing to "TELEMETRY" relationship - Message type "TELEMETRY" -> Routes to "TELEMETRY" relation
//   - Unknown type -> routing to "Default" relation - Unknown type -> Routes to "Default" relation
type MsgTypeSwitchNode struct {
	// defaultRelationType stores the default relationship name for configurations that do not match the message type
	// defaultRelationType stores the configured default relation name for unmatched message types
	defaultRelationType string
}

// Type returns the component type
// Type returns the component type identifier.
func (x *MsgTypeSwitchNode) Type() string {
	return "msgTypeSwitch"
}

// New creates an instance
// New creates a new instance.
func (x *MsgTypeSwitchNode) New() types.Node {
	return &MsgTypeSwitchNode{}
}

// Init initializes the component and configures the default relationship type name from global properties
// Init initializes the component.
func (x *MsgTypeSwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if v := ruleConfig.Properties.GetValue(types.DefaultRelationTypeKey); v != "" {
		x.defaultRelationType = v
	} else {
		x.defaultRelationType = types.DefaultRelationType
	}
	return nil
}

// OnMsg handles messages by routing them to matching relationships or default relationships based on message types
// OnMsg processes incoming messages by routing them based on their message type.
func (x *MsgTypeSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellNextOrElse(msg, x.defaultRelationType, msg.Type)
}

// Def returns the component form definition
func (x *MsgTypeSwitchNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Route messages to connections matching their msgType. No configuration needed. Unmatched goes to Default",
		RelationTypes: &[]string{},
	}
}

// Destroy to clean up resources
func (x *MsgTypeSwitchNode) Destroy() {
}
