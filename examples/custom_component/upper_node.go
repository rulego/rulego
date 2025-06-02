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

package main

import (
	"context"
	"github.com/rulego/rulego/api/types"
	"strings"
)

// UpperNode A plugin that converts the message data to uppercase
type UpperNode struct{}

func (n *UpperNode) Type() string {
	return "test/upper"
}
func (n *UpperNode) New() types.Node {
	return &UpperNode{}
}
func (n *UpperNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *UpperNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.SetData(strings.ToUpper(msg.GetData()))
	v := ctx.GetContext().Value(shareKey)
	if v != nil {
		msg.Metadata.PutValue(shareKey, v.(string))
	}
	//增加新的共享数据
	modifyCtx := context.WithValue(ctx.GetContext(), addShareKey, addShareValue)
	ctx.SetContext(modifyCtx)
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
}

func (n *UpperNode) Destroy() {
	// Do some cleanup work
}
