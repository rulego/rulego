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

package test

import (
	"context"
	"errors"
	"github.com/rulego/rulego/api/types"
	"sync"
	"time"
)

var _ types.RuleContext = (*NodeTestRuleContext)(nil)

// NodeTestRuleContext
// 只为测试单节点，临时创建的上下文
// 无法把多个节点组成链式
// callback 回调处理结果
type NodeTestRuleContext struct {
	context  context.Context
	config   types.Config
	callback func(msg types.RuleMsg, relationType string, err error)
	self     types.Node
	selfId   string
	//所有子节点处理完成事件，只执行一次
	onAllNodeCompleted func()
	onEndFunc          types.OnEndFunc
	childrenNodes      sync.Map
}

func NewRuleContext(config types.Config, callback func(msg types.RuleMsg, relationType string, err error)) types.RuleContext {
	return &NodeTestRuleContext{
		context:  context.TODO(),
		config:   config,
		callback: callback,
	}
}

func NewRuleContextFull(config types.Config, self types.Node, childrenNodes map[string]types.Node, callback func(msg types.RuleMsg, relationType string, err error)) types.RuleContext {
	ctx := &NodeTestRuleContext{
		config:   config,
		self:     self,
		callback: callback,
		context:  context.TODO(),
	}
	for k, v := range childrenNodes {
		ctx.childrenNodes.Store(k, v)
	}
	return ctx
}

func (ctx *NodeTestRuleContext) TellSuccess(msg types.RuleMsg) {
	ctx.callback(msg, types.Success, nil)
	if ctx.onEndFunc != nil {
		ctx.onEndFunc(ctx, msg, nil, types.Success)
	}
}
func (ctx *NodeTestRuleContext) TellFailure(msg types.RuleMsg, err error) {
	ctx.callback(msg, types.Failure, err)
	if ctx.onEndFunc != nil {
		ctx.onEndFunc(ctx, msg, err, types.Failure)
	}
}
func (ctx *NodeTestRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	for _, relationType := range relationTypes {
		ctx.callback(msg, relationType, nil)
		if ctx.onEndFunc != nil {
			ctx.onEndFunc(ctx, msg, nil, relationType)
		}
	}

}
func (ctx *NodeTestRuleContext) TellSelf(msg types.RuleMsg, delayMs int64) {
	time.AfterFunc(time.Millisecond*time.Duration(delayMs), func() {
		if ctx.self != nil {
			ctx.self.OnMsg(ctx, msg)
		}
	})
}
func (ctx *NodeTestRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *NodeTestRuleContext) GetSelfId() string {
	return ctx.selfId
}
func (ctx *NodeTestRuleContext) Self() types.NodeCtx {
	return nil
}

func (ctx *NodeTestRuleContext) From() types.NodeCtx {
	return nil
}
func (ctx *NodeTestRuleContext) RuleChain() types.NodeCtx {
	return nil
}
func (ctx *NodeTestRuleContext) Config() types.Config {
	return ctx.config
}

func (ctx *NodeTestRuleContext) SubmitTack(task func()) {
	go task()
}

func (ctx *NodeTestRuleContext) SetEndFunc(onEndFunc types.OnEndFunc) types.RuleContext {
	ctx.onEndFunc = onEndFunc
	return ctx
}

func (ctx *NodeTestRuleContext) GetEndFunc() types.OnEndFunc {
	return ctx.onEndFunc
}

func (ctx *NodeTestRuleContext) SetContext(c context.Context) types.RuleContext {
	ctx.context = c
	return ctx
}

func (ctx *NodeTestRuleContext) GetContext() context.Context {
	return ctx.context
}

func (ctx *NodeTestRuleContext) TellFlow(msg types.RuleMsg, chainId string, endFunc types.OnEndFunc, onAllNodeCompleted func()) {

	if chainId == "" {
		endFunc(ctx, msg, errors.New("chainId can not nil"), types.Failure)
	} else {
		endFunc(ctx, msg, nil, types.Success)
		onAllNodeCompleted()
	}
}

// SetOnAllNodeCompleted 设置所有节点执行完回调
func (ctx *NodeTestRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
}

// ExecuteNode 独立执行某个节点，通过callback获取节点执行情况，用于节点分组类节点控制执行某个节点
func (ctx *NodeTestRuleContext) ExecuteNode(context context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, callback types.OnEndFunc) {
	if v, ok := ctx.childrenNodes.Load(nodeId); ok {
		ctx.selfId = nodeId
		subCtx := NewRuleContext(ctx.config, func(msg types.RuleMsg, relationType string, err error) {
			callback(ctx, msg, err, relationType)
		})

		v.(types.Node).OnMsg(subCtx, msg)
	} else {
		callback(ctx, msg, errors.New("not found nodeId="+nodeId), types.Failure)
	}
}

func (ctx *NodeTestRuleContext) DoOnEnd(msg types.RuleMsg, err error, relationType string) {

}

// SetCallbackFunc 设置回调函数
func (ctx *NodeTestRuleContext) SetCallbackFunc(functionName string, f interface{}) {

}

// GetCallbackFunc 获取回调函数
func (ctx *NodeTestRuleContext) GetCallbackFunc(functionName string) interface{} {
	return nil
}

// OnDebug 调用配置的OnDebug回调函数
func (ctx *NodeTestRuleContext) OnDebug(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
}
