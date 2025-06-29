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

//规则链节点配置示例：
//{
//        "id": "s1",
//        "type": "comment",
//        "name": "this a comment",
//        "debugMode": false,
//        "configuration": {
//        },
//       "additionalInfo": {
//          "description": "this a comment",
//          "layoutX": 540,
//          "layoutY": 260
//        },
//  }
import (
	"github.com/rulego/rulego/api/types"
)

// init 注册CommentNode组件
// init registers the CommentNode component with the default registry.
func init() {
	Registry.Add(&CommentNode{})
}

// CommentNodeConfiguration CommentNode配置结构
// CommentNodeConfiguration defines the configuration structure for the CommentNode component.
type CommentNodeConfiguration struct {
	// 注释节点不需要配置字段
	// No configuration fields required for comment nodes
}

// CommentNode 注释组件，用于规则链的可视化编辑器显示节点注释信息，不处理消息，直通传递消息
// CommentNode is a visualization and documentation component that passes messages through unchanged.
type CommentNode struct {
}

// Type 返回组件类型
// Type returns the component type identifier.
func (x *CommentNode) Type() string {
	return "comment"
}

// New 创建新实例
// New creates a new instance.
func (x *CommentNode) New() types.Node {
	return &CommentNode{}
}

// Init 初始化组件
// Init initializes the component.
func (x *CommentNode) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

// OnMsg 直通传递消息
// OnMsg forwards messages unchanged.
func (x *CommentNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy cleans up resources.
func (x *CommentNode) Destroy() {
}
