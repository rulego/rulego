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
	"github.com/rulego/rulego/api/types"
	"time"
)

//TimeNode A plugin that adds a timestamp to the message metadata
type TimeNode struct{}

func (n *TimeNode) Type() string {
	return "test/time"
}

func (n *TimeNode) New() types.Node {
	return &TimeNode{}
}

func (n *TimeNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Do some initialization work
	return nil
}

func (n *TimeNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	msg.Metadata.PutValue("timestamp", time.Now().Format(time.RFC3339))
	v1 := ctx.GetContext().Value(shareKey)
	if v1 != nil {
		msg.Metadata.PutValue(shareKey, v1.(string))
	}
	v2 := ctx.GetContext().Value(addShareKey)
	if v2 != nil {
		msg.Metadata.PutValue(addShareKey, v2.(string))
	}
	// Send the modified message to the next node
	ctx.TellSuccess(msg)
	return nil
}

func (n *TimeNode) Destroy() {
	// Do some cleanup work
}
