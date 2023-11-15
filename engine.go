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

package rulego

import (
	"context"
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"sync/atomic"
	"time"
)

// DefaultRuleContext 默认规则引擎消息处理上下文
type DefaultRuleContext struct {
	//id     string
	//用于不同组件共享信号量和数据的上下文
	context context.Context
	config  types.Config
	//根规则链上下文
	ruleChainCtx *RuleChainCtx
	//上一个节点上下文
	from types.NodeCtx
	//当前节点上下文
	self types.NodeCtx
	//是否是第一个节点
	isFirst bool
	//协程池
	pool types.Pool
	//规则链分支处理结束回调函数
	onEnd types.OnEndFunc
	//当前节点下未执行完成的子节点数量
	waitingCount int32
	//父ruleContext
	parentRuleCtx *DefaultRuleContext
	//所有子节点处理完成事件，只执行一次
	onAllNodeCompleted func()
	//子规则链池
	ruleChainPool *RuleGo
	//tellNext的拦截器，再触发下一个节点前调用
	interceptor func(msg types.RuleMsg, err error, relationTypes ...string)
}

//NewRuleContext 创建一个默认规则引擎消息处理上下文实例
func NewRuleContext(context context.Context, config types.Config, ruleChainCtx *RuleChainCtx, from types.NodeCtx, self types.NodeCtx, pool types.Pool, onEnd types.OnEndFunc, ruleChainPool *RuleGo) *DefaultRuleContext {
	return &DefaultRuleContext{
		context:       context,
		config:        config,
		ruleChainCtx:  ruleChainCtx,
		from:          from,
		self:          self,
		isFirst:       from == nil,
		pool:          pool,
		onEnd:         onEnd,
		ruleChainPool: ruleChainPool,
	}
}

//NewNextNodeRuleContext 创建下一个节点的规则引擎消息处理上下文实例RuleContext
func (ctx *DefaultRuleContext) NewNextNodeRuleContext(nextNode types.NodeCtx) *DefaultRuleContext {
	return &DefaultRuleContext{
		config:        ctx.config,
		ruleChainCtx:  ctx.ruleChainCtx,
		from:          ctx.self,
		self:          nextNode,
		pool:          ctx.pool,
		onEnd:         ctx.onEnd,
		context:       ctx.GetContext(),
		parentRuleCtx: ctx,
		interceptor:   ctx.interceptor,
	}
}

func (ctx *DefaultRuleContext) TellSuccess(msg types.RuleMsg) {
	ctx.tell(msg, nil, types.Success)
}
func (ctx *DefaultRuleContext) TellFailure(msg types.RuleMsg, err error) {
	ctx.tell(msg, err, types.Failure)
}
func (ctx *DefaultRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	ctx.tell(msg, nil, relationTypes...)
}
func (ctx *DefaultRuleContext) TellSelf(msg types.RuleMsg, delayMs int64) {
	time.AfterFunc(time.Millisecond*time.Duration(delayMs), func() {
		_ = ctx.self.OnMsg(ctx, msg)
	})
}
func (ctx *DefaultRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *DefaultRuleContext) GetSelfId() string {
	return ctx.self.GetNodeId().Id
}

func (ctx *DefaultRuleContext) Config() types.Config {
	return ctx.config
}

func (ctx *DefaultRuleContext) SetEndFunc(onEndFunc types.OnEndFunc) types.RuleContext {
	ctx.onEnd = onEndFunc
	return ctx
}

func (ctx *DefaultRuleContext) GetEndFunc() types.OnEndFunc {
	return ctx.onEnd
}

func (ctx *DefaultRuleContext) SetContext(c context.Context) types.RuleContext {
	ctx.context = c
	return ctx
}

func (ctx *DefaultRuleContext) GetContext() context.Context {
	return ctx.context
}

func (ctx *DefaultRuleContext) SetAllCompletedFunc(f func()) types.RuleContext {
	ctx.onAllNodeCompleted = f
	return ctx
}

func (ctx *DefaultRuleContext) SubmitTack(task func()) {
	if ctx.pool != nil {
		if err := ctx.pool.Submit(task); err != nil {
			ctx.config.Logger.Printf("SubmitTack error:%s", err)
		}
	} else {
		go task()
	}
}

//TellFlow 执行子规则链，ruleChainId 规则链ID
//onEndFunc 子规则链链分支执行完的回调，并返回该链执行结果，如果同时触发多个分支链，则会调用多次
//onAllNodeCompleted 所以节点执行完之后的回调，无结果返回
//如果找不到规则链，并把消息通过`Failure`关系发送到下一个节点
func (ctx *DefaultRuleContext) TellFlow(msg types.RuleMsg, chainId string, onEndFunc types.OnEndFunc, onAllNodeCompleted func()) {
	if e, ok := ctx.GetRuleChainPool().Get(chainId); ok {
		e.OnMsgWithOptions(msg, types.WithEndFunc(onEndFunc), types.WithOnAllNodeCompleted(onAllNodeCompleted))
	} else {
		ctx.TellFailure(msg, fmt.Errorf("ruleChain id=%s not found", chainId))
	}
}

//SetRuleChainPool 设置子规则链池
func (ctx *DefaultRuleContext) SetRuleChainPool(ruleChainPool *RuleGo) {
	ctx.ruleChainPool = ruleChainPool
}

//GetRuleChainPool 获取子规则链池
func (ctx *DefaultRuleContext) GetRuleChainPool() *RuleGo {
	if ctx.ruleChainPool == nil {
		return DefaultRuleGo
	} else {
		return ctx.ruleChainPool
	}
}

//SetOnAllNodeCompleted 设置所有节点执行完回调
func (ctx *DefaultRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
}

//ExecuteNode 独立执行某个节点，通过callback获取节点执行情况，用于节点分组类节点控制执行某个节点
func (ctx *DefaultRuleContext) ExecuteNode(chanCtx context.Context, nodeId string, msg types.RuleMsg, callback func(msg types.RuleMsg, err error, relationTypes ...string)) {
	if nodeCtx, ok := ctx.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: nodeId}); ok {
		rootCtxCopy := NewRuleContext(chanCtx, ctx.config, ctx.ruleChainCtx, nil, nodeCtx, ctx.pool, nil, ctx.ruleChainPool)
		rootCtxCopy.interceptor = callback
		if err := nodeCtx.OnMsg(rootCtxCopy, msg); err != nil {
			callback(msg, err, types.Failure)
		}
	} else {
		callback(msg, errors.New("node id not found nodeId="+nodeId), types.Failure)
	}
}

//增加一个待执行子节点
func (ctx *DefaultRuleContext) childReady() {
	atomic.AddInt32(&ctx.waitingCount, 1)
}

//减少一个待执行子节点
//如果返回数量0，表示该分支链条已经都执行完成，递归父节点，直到所有节点都处理完，则触发onAllNodeCompleted事件。
func (ctx *DefaultRuleContext) childDone() {
	if atomic.AddInt32(&ctx.waitingCount, -1) <= 0 {
		//该节点已经执行完成，通知父节点
		if ctx.parentRuleCtx != nil {
			ctx.parentRuleCtx.childDone()
		}
		//完成回调
		if ctx.onAllNodeCompleted != nil {
			ctx.onAllNodeCompleted()
		}
	}
}

// getNextNodes 获取当前节点指定关系的子节点
func (ctx *DefaultRuleContext) getNextNodes(relationType string) ([]types.NodeCtx, bool) {
	if ctx.ruleChainCtx == nil || ctx.self == nil {
		return nil, false
	}
	return ctx.ruleChainCtx.GetNextNodes(ctx.self.GetNodeId(), relationType)
}

func (ctx *DefaultRuleContext) onDebug(flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if ctx.config.OnDebug != nil {
		var chainId = ""
		if ctx.ruleChainCtx != nil {
			chainId = ctx.ruleChainCtx.Id.Id
		}
		ctx.config.OnDebug(chainId, flowType, nodeId, msg.Copy(), relationType, err)
	}
}

func (ctx *DefaultRuleContext) tell(msg types.RuleMsg, err error, relationTypes ...string) {
	msgCopy := msg.Copy()
	if ctx.isFirst {
		ctx.SubmitTack(func() {
			if ctx.self != nil {
				ctx.tellNext(msgCopy, ctx.self)
			} else {
				ctx.doOnEnd(msgCopy, err)
			}
		})
	} else {
		//回调
		if ctx.interceptor != nil {
			ctx.interceptor(msgCopy, err, relationTypes...)
		}
		for _, relationType := range relationTypes {
			if ctx.self != nil && ctx.self.IsDebugMode() {
				//记录调试信息
				ctx.SubmitTack(func() {
					ctx.onDebug(types.Out, ctx.GetSelfId(), msgCopy, relationType, err)
				})
			}

			if nodes, ok := ctx.getNextNodes(relationType); ok {
				for _, item := range nodes {
					tmp := item
					ctx.SubmitTack(func() {
						ctx.tellNext(msg.Copy(), tmp)
					})
				}
			} else {
				ctx.doOnEnd(msgCopy, err)
			}
		}
	}

}

func (ctx *DefaultRuleContext) tellNext(msg types.RuleMsg, nextNode types.NodeCtx) {

	nextCtx := ctx.NewNextNodeRuleContext(nextNode)
	//增加一个待执行的子节点
	ctx.childReady()
	defer func() {
		//捕捉异常
		if e := recover(); e != nil {
			if nextCtx.self != nil && nextCtx.self.IsDebugMode() {
				//记录异常信息
				ctx.onDebug(types.In, nextCtx.GetSelfId(), msg, "", fmt.Errorf("%v", e))
			}
			ctx.childDone()
		}
	}()
	if nextCtx.self != nil && nextCtx.self.IsDebugMode() {
		//记录调试信息
		ctx.onDebug(types.In, nextCtx.GetSelfId(), msg, "", nil)
	}
	if err := nextNode.OnMsg(nextCtx, msg); err != nil {
		ctx.config.Logger.Printf("tellNext error.node type:%s error: %s", nextCtx.self.Type(), err)
	}
}

//规则链执行完成回调函数
func (ctx *DefaultRuleContext) doOnEnd(msg types.RuleMsg, err error) {
	//全局回调
	//通过`Config.OnEnd`设置
	if ctx.config.OnEnd != nil {
		ctx.SubmitTack(func() {
			ctx.config.OnEnd(msg, err)
		})
	}
	//单条消息的context回调
	//通过OnMsgWithEndFunc(msg, endFunc)设置
	if ctx.onEnd != nil {
		ctx.SubmitTack(func() {
			ctx.onEnd(ctx, msg, err)
			ctx.childDone()
		})
	} else {
		ctx.childDone()
	}

}

// RuleEngine 规则引擎
//每个规则引擎实例只有一个根规则链，如果没设置规则链则无法处理数据
type RuleEngine struct {
	//规则引擎实例标识
	Id string
	//配置
	Config types.Config
	//子规则链池
	RuleChainPool *RuleGo
	//根规则链
	rootRuleChainCtx *RuleChainCtx
}

// RuleEngineOption is a function type that modifies the RuleEngine.
type RuleEngineOption func(*RuleEngine) error

func newRuleEngine(id string, def []byte, opts ...RuleEngineOption) (*RuleEngine, error) {
	if len(def) == 0 {
		return nil, errors.New("def can not nil")
	}
	// Create a new RuleEngine with the Id
	ruleEngine := &RuleEngine{
		Id:            id,
		Config:        NewConfig(),
		RuleChainPool: DefaultRuleGo,
	}
	err := ruleEngine.ReloadSelf(def, opts...)
	if err == nil && ruleEngine.rootRuleChainCtx != nil {
		if id != "" {
			ruleEngine.rootRuleChainCtx.Id = types.RuleNodeId{Id: id, Type: types.CHAIN}
		} else {
			//使用规则链ID
			ruleEngine.Id = ruleEngine.rootRuleChainCtx.Id.Id
		}

	}

	return ruleEngine, err
}

// ReloadSelf 重新加载规则链
func (e *RuleEngine) ReloadSelf(def []byte, opts ...RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	for _, opt := range opts {
		_ = opt(e)
	}
	//初始化
	if ctx, err := e.Config.Parser.DecodeRuleChain(e.Config, def); err == nil {
		if e.Initialized() {
			e.Stop()
		}
		if e.rootRuleChainCtx != nil {
			ctx.(*RuleChainCtx).Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx.(*RuleChainCtx)
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleChainPool(e.RuleChainPool)

		return nil
	} else {
		return err
	}
}

// ReloadChild 更新根规则链或者其下某个节点
//如果ruleNodeId为空更新根规则链，否则更新指定的子节点
//dsl 根规则链/子节点配置
func (e *RuleEngine) ReloadChild(ruleNodeId string, dsl []byte) error {
	if len(dsl) == 0 {
		return errors.New("dsl can not empty")
	} else if e.rootRuleChainCtx == nil {
		return errors.New("ReloadNode error.RuleEngine not initialized")
	} else if ruleNodeId == "" {
		//更新根规则链
		return e.ReloadSelf(dsl)
	} else {
		//更新根规则链子节点
		return e.rootRuleChainCtx.ReloadChild(types.RuleNodeId{Id: ruleNodeId}, dsl)
	}
}

//DSL 获取根规则链配置
func (e *RuleEngine) DSL() []byte {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.DSL()
	} else {
		return nil
	}
}

//NodeDSL 获取规则链节点配置
func (e *RuleEngine) NodeDSL(chainId types.RuleNodeId, childNodeId types.RuleNodeId) []byte {
	if e.rootRuleChainCtx != nil {
		if chainId.Id == "" {
			if node, ok := e.rootRuleChainCtx.GetNodeById(childNodeId); ok {
				return node.DSL()
			}
		} else {
			if node, ok := e.rootRuleChainCtx.GetNodeById(chainId); ok {
				if childNode, ok := node.GetNodeById(childNodeId); ok {
					return childNode.DSL()
				}
			}
		}
	}
	return nil
}

func (e *RuleEngine) Initialized() bool {
	return e.rootRuleChainCtx != nil
}

//RootRuleChainCtx 获取根规则链
func (e *RuleEngine) RootRuleChainCtx() *RuleChainCtx {
	return e.rootRuleChainCtx
}

func (e *RuleEngine) Stop() {
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.Destroy()
		e.rootRuleChainCtx = nil
	}
}

// OnMsg 把消息交给规则引擎处理，异步执行
//根据规则链节点配置和连接关系处理消息
func (e *RuleEngine) OnMsg(msg types.RuleMsg) {
	e.OnMsgWithOptions(msg)
}

// OnMsgWithEndFunc 把消息交给规则引擎处理，异步执行
//endFunc 用于数据经过规则链执行完的回调，用于获取规则链处理结果数据。注意：如果规则链有多个结束点，回调函数则会执行多次
func (e *RuleEngine) OnMsgWithEndFunc(msg types.RuleMsg, endFunc types.OnEndFunc) {
	e.OnMsgWithOptions(msg, types.WithEndFunc(endFunc))
}

// OnMsgWithOptions 把消息交给规则引擎处理，异步执行
//可以携带context选项和结束回调选项
//context 用于不同组件实例数据共享
//endFunc 用于数据经过规则链执行完的回调，用于获取规则链处理结果数据。注意：如果规则链有多个结束点，回调函数则会执行多次
func (e *RuleEngine) OnMsgWithOptions(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// OnMsgAndWait 把消息交给规则引擎处理，同步执行，等规则链所有节点执行完，返回
func (e *RuleEngine) OnMsgAndWait(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, true, opts...)
}

func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	if e.rootRuleChainCtx != nil {
		rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
		rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.RuleChainPool)
		rootCtxCopy.isFirst = rootCtx.isFirst
		for _, opt := range opts {
			opt(rootCtxCopy)
		}

		rootCtxCopy.TellNext(msg)

		//同步方式调用，等规则链都执行完，才返回
		if wait {
			customFunc := rootCtxCopy.onAllNodeCompleted
			c := make(chan struct{})
			rootCtxCopy.onAllNodeCompleted = func() {
				close(c)
				if customFunc != nil {
					//触发自定义回调
					customFunc()
				}
			}
			<-c
		}

	} else {
		//沒有定义根则链或者没初始化
		e.Config.Logger.Printf("onMsg error.RuleEngine not initialized")
	}
}

// NewConfig creates a new Config and applies the options.
func NewConfig(opts ...types.Option) types.Config {
	c := types.NewConfig(opts...)
	if c.Parser == nil {
		c.Parser = &JsonParser{}
	}
	if c.ComponentsRegistry == nil {
		c.ComponentsRegistry = Registry
	}
	return c
}

// WithConfig is an option that sets the Config of the RuleEngine.
func WithConfig(config types.Config) RuleEngineOption {
	return func(re *RuleEngine) error {
		re.Config = config
		return nil
	}
}

//WithRuleChainPool 子规则链池
func WithRuleChainPool(ruleChainPool *RuleGo) RuleEngineOption {
	return func(re *RuleEngine) error {
		re.RuleChainPool = ruleChainPool
		return nil
	}
}
