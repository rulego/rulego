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
//        "id": "s2",
//        "type": "msgTypeSwitch",
//        "name": "消息路由"
//      }
import (
	"github.com/rulego/rulego/api/types"
)

// init 注册MsgTypeSwitchNode组件
// init registers the MsgTypeSwitchNode component with the default registry.
func init() {
	Registry.Add(&MsgTypeSwitchNode{})
}

// MsgTypeSwitchNode 根据消息类型将消息路由到不同的输出链的过滤组件
// MsgTypeSwitchNode routes messages to different output chains based on their message type.
//
// 核心算法：
// Core Algorithm:
// 1. 从传入消息提取消息类型 - Extract message type from incoming message
// 2. 尝试路由到匹配消息类型的关系 - Attempt to route to relation matching message type
// 3. 如果不存在匹配关系，路由到默认关系 - If no matching relation exists, route to default relation
//
// 路由逻辑 - Routing logic:
//   - 主要：路由到匹配消息类型的关系 - Primary: Route to relation matching message type
//   - 回退：路由到配置的默认关系 - Fallback: Route to configured default relation
//   - 关系名称区分大小写 - Relation names are case-sensitive
//
// 配置选项 - Configuration options:
//   - 全局属性"defaultRelationType"：自定义默认关系名称 - Global property "defaultRelationType": Custom default relation name
//   - 如果未配置，使用"Default"作为回退关系 - If not configured, uses "Default" as the fallback relation
//
// 使用场景 - Use cases:
//   - 按类型分类消息 - Message categorization by type
//   - 特定类型的处理工作流 - Type-specific processing workflows
//   - 消息过滤和路由 - Message filtering and routing
//
// 路由示例 - Routing examples:
//   - 消息类型"ALARM"->路由到"ALARM"关系 - Message type "ALARM" -> Routes to "ALARM" relation
//   - 消息类型"TELEMETRY"->路由到"TELEMETRY"关系 - Message type "TELEMETRY" -> Routes to "TELEMETRY" relation
//   - 未知类型->路由到"Default"关系 - Unknown type -> Routes to "Default" relation
type MsgTypeSwitchNode struct {
	// defaultRelationType 存储未匹配消息类型的配置默认关系名称
	// defaultRelationType stores the configured default relation name for unmatched message types
	defaultRelationType string
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *MsgTypeSwitchNode) Type() string {
	return "msgTypeSwitch"
}

// New 创建新实例
// New creates a new instance.
func (x *MsgTypeSwitchNode) New() types.Node {
	return &MsgTypeSwitchNode{}
}

// Init 初始化组件，从全局属性配置默认关系类型名称
// Init initializes the component.
func (x *MsgTypeSwitchNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if v := ruleConfig.Properties.GetValue(types.DefaultRelationTypeKey); v != "" {
		x.defaultRelationType = v
	} else {
		x.defaultRelationType = types.DefaultRelationType
	}
	return nil
}

// OnMsg 处理消息，根据消息类型路由到匹配的关系或默认关系
// OnMsg processes incoming messages by routing them based on their message type.
func (x *MsgTypeSwitchNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellNextOrElse(msg, x.defaultRelationType, msg.Type)
}

// Destroy 清理资源
// Destroy cleans up resources.
func (x *MsgTypeSwitchNode) Destroy() {
}
