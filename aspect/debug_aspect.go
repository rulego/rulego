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

package aspect

import (
	"github.com/rulego/rulego/api/types"
)

var (
	// Compile-time check Debug implements types.BeforeAspect.
	_ types.BeforeAspect = (*Debug)(nil)
	// Compile-time check Debug implements types.AfterAspect.
	_ types.AfterAspect = (*Debug)(nil)
)

// Debug 节点debug日志切面
type Debug struct {
}

func (aspect *Debug) Order() int {
	return 900
}

// PointCut 切入点 所有节点都会执行
func (aspect *Debug) PointCut(ctx types.RuleContext, msg types.RuleMsg, relationType string) bool {
	return true
}

func (aspect *Debug) Before(ctx types.RuleContext, msg types.RuleMsg, relationType string) types.RuleMsg {
	//异步记录In日志
	aspect.onDebug(ctx, types.In, msg, relationType, nil)
	return msg
}

func (aspect *Debug) After(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	//异步记录Out日志
	aspect.onDebug(ctx, types.Out, msg, relationType, err)
	return msg
}

func (aspect *Debug) onDebug(ctx types.RuleContext, flowType string, msg types.RuleMsg, relationType string, err error) {
	ctx.SubmitTack(func() {
		//异步记录日志
		if ctx.Self() != nil && ctx.Self().IsDebugMode() {
			var chainId = ""
			if ctx.RuleChain() != nil {
				chainId = ctx.RuleChain().GetNodeId().Id
			}
			ctx.Config().OnDebug(chainId, flowType, ctx.Self().GetNodeId().Id, msg.Copy(), relationType, err)
		}
	})
}
