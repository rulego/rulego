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

// init registers the CommentNode component
// init registers the CommentNode component with the default registry.
func init() {
	Registry.Add(&CommentNode{})
}

// CommentNodeConfiguration CommentNode configuration structure
// CommentNodeConfiguration defines the configuration structure for the CommentNode component.
type CommentNodeConfiguration struct {
	// Annotation nodes do not require field configuration
	// No configuration fields required for comment nodes
}

// CommentNode comment component, used as a visual editor for rule chains to display node comment information, does not process messages, and passes messages directly
// CommentNode is a visualization and documentation component that passes messages through unchanged.
type CommentNode struct {
}

// Type returns the component type
// Type returns the component type identifier.
func (x *CommentNode) Type() string {
	return "comment"
}

// New creates an instance
// New creates a new instance.
func (x *CommentNode) New() types.Node {
	return &CommentNode{}
}

// Init initializes the component
// Init initializes the component.
func (x *CommentNode) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

// OnMsg passes messages directly
// OnMsg forwards messages unchanged.
func (x *CommentNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellSuccess(msg)
}

// Destroy cleans up resources.
func (x *CommentNode) Destroy() {
}

// Def returns the component form definition
func (x *CommentNode) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc:          "Comment node for visual editor annotations. Passes messages through unchanged",
		RelationTypes: &[]string{types.Success},
	}
}
