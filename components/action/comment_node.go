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

// 注册节点
func init() {
	Registry.Add(&CommentNode{})
}

// CommentNodeConfiguration 节点配置
type CommentNodeConfiguration struct {
}

// CommentNode 用于可视化注释节点，不参与流程逻辑
type CommentNode struct {
}

// Type 组件类型
func (x *CommentNode) Type() string {
	return "comment"
}

func (x *CommentNode) New() types.Node {
	return &CommentNode{}
}

// Init 初始化
func (x *CommentNode) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

// OnMsg 处理消息
func (x *CommentNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy 销毁
func (x *CommentNode) Destroy() {
}
