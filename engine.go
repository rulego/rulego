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
	"github.com/rulego/rulego/aspect"
	"sync"
	"sync/atomic"
	"time"
)

var _ types.RuleContext = (*DefaultRuleContext)(nil)

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
	//是否已经执行了onAllNodeCompleted函数
	onAllNodeCompletedDone int32
	//子规则链池
	ruleChainPool *RuleGo
	//是否跳过执行子节点，默认是不跳过
	skipTellNext bool
	//环绕切面列表
	aroundAspects []types.AroundAspect
	//前置切面列表
	beforeAspects []types.BeforeAspect
	//后置切面列表
	afterAspects []types.AfterAspect
	//运行时快照
	runSnapshot *RunSnapshot
}

// NewRuleContext 创建一个默认规则引擎消息处理上下文实例
func NewRuleContext(context context.Context, config types.Config, ruleChainCtx *RuleChainCtx, from types.NodeCtx, self types.NodeCtx, pool types.Pool, onEnd types.OnEndFunc, ruleChainPool *RuleGo) *DefaultRuleContext {
	aroundAspects, beforeAspects, afterAspects := config.GetNodeAspects()
	//添加before日志切面
	beforeAspects = append(beforeAspects, &aspect.Debug{})
	//添加after日志切面
	afterAspects = append(afterAspects, &aspect.Debug{})
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
		aroundAspects: aroundAspects,
		beforeAspects: beforeAspects,
		afterAspects:  afterAspects,
	}
}

type RunSnapshot struct {
	msgId                    string
	chainCtx                 *RuleChainCtx
	startTs                  int64
	onRuleChainCompletedFunc func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot)
	onNodeCompletedFunc      func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog)
	// Logs 每个节点的日志
	logs map[string]*types.RuleNodeRunLog
	//onDebugCustomFunc 自定义debug回调
	onDebugCustomFunc func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error)
	lock              sync.RWMutex
}

func NewRunSnapshot(msgId string, chainCtx *RuleChainCtx, startTs int64) *RunSnapshot {
	runSnapshot := &RunSnapshot{
		msgId:    msgId,
		chainCtx: chainCtx,
		startTs:  startTs,
	}
	runSnapshot.logs = make(map[string]*types.RuleNodeRunLog)
	return runSnapshot
}

func (r *RunSnapshot) needCollectRunSnapshot() bool {
	return r.onRuleChainCompletedFunc != nil || r.onNodeCompletedFunc != nil
}

func (r *RunSnapshot) collectRunSnapshot(ctx types.RuleContext, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if !r.needCollectRunSnapshot() {
		return
	}
	r.lock.RLock()
	nodeLog, ok := r.logs[nodeId]
	r.lock.RUnlock()
	if !ok {
		nodeLog = &types.RuleNodeRunLog{
			Id: nodeId,
		}
		r.lock.Lock()
		r.logs[nodeId] = nodeLog
		r.lock.Unlock()
	}
	if flowType == types.In {
		nodeLog.InMsg = msg
		nodeLog.StartTs = time.Now().UnixMilli()
	}
	if flowType == types.Out {
		nodeLog.OutMsg = msg
		nodeLog.RelationType = relationType
		if err != nil {
			nodeLog.Err = err.Error()
		}
		nodeLog.EndTs = time.Now().UnixMilli()
		if r.onNodeCompletedFunc != nil {
			r.onNodeCompletedFunc(ctx, *nodeLog)
		}
	}
	if flowType == types.Log {
		nodeLog.LogItems = append(nodeLog.LogItems, msg.Data)
	}
}

func (r *RunSnapshot) onDebugCustom(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if r.onDebugCustomFunc != nil {
		r.onDebugCustomFunc(ruleChainId, flowType, nodeId, msg, relationType, err)
	}
}

func (r *RunSnapshot) createRuleChainRunLog(endTs int64) types.RuleChainRunSnapshot {
	var logs []types.RuleNodeRunLog
	for _, item := range r.logs {
		logs = append(logs, *item)
	}
	ruleChainRunLog := types.RuleChainRunSnapshot{
		RuleChain: *r.chainCtx.SelfDefinition,
		Id:        r.msgId,
		StartTs:   r.startTs,
		EndTs:     endTs,
		Logs:      logs,
	}
	return ruleChainRunLog

}

func (r *RunSnapshot) onRuleChainCompleted(ctx types.RuleContext) {
	if r.onRuleChainCompletedFunc != nil {
		r.onRuleChainCompletedFunc(ctx, r.createRuleChainRunLog(time.Now().UnixMilli()))
	}
}

// NewNextNodeRuleContext 创建下一个节点的规则引擎消息处理上下文实例RuleContext
func (ctx *DefaultRuleContext) NewNextNodeRuleContext(nextNode types.NodeCtx) *DefaultRuleContext {
	return &DefaultRuleContext{
		config:        ctx.config,
		ruleChainCtx:  ctx.ruleChainCtx,
		from:          ctx.self,
		self:          nextNode,
		pool:          ctx.pool,
		onEnd:         ctx.onEnd,
		ruleChainPool: ctx.ruleChainPool,
		context:       ctx.GetContext(),
		parentRuleCtx: ctx,
		skipTellNext:  ctx.skipTellNext,
		aroundAspects: ctx.aroundAspects,
		beforeAspects: ctx.beforeAspects,
		afterAspects:  ctx.afterAspects,
		runSnapshot:   ctx.runSnapshot,
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
		ctx.self.OnMsg(ctx, msg)
	})
}
func (ctx *DefaultRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *DefaultRuleContext) GetSelfId() string {
	return ctx.self.GetNodeId().Id
}
func (ctx *DefaultRuleContext) Self() types.NodeCtx {
	return ctx.self
}

func (ctx *DefaultRuleContext) From() types.NodeCtx {
	return ctx.from
}
func (ctx *DefaultRuleContext) RuleChain() types.NodeCtx {
	return ctx.ruleChainCtx
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

// TellFlow 执行子规则链，ruleChainId 规则链ID
// onEndFunc 子规则链链分支执行完的回调，并返回该链执行结果，如果同时触发多个分支链，则会调用多次
// onAllNodeCompleted 所以节点执行完之后的回调，无结果返回
// 如果找不到规则链，并把消息通过`Failure`关系发送到下一个节点
func (ctx *DefaultRuleContext) TellFlow(msg types.RuleMsg, chainId string, onEndFunc types.OnEndFunc, onAllNodeCompleted func()) {
	if e, ok := ctx.GetRuleChainPool().Get(chainId); ok {
		e.OnMsg(msg, types.WithOnEnd(onEndFunc), types.WithOnAllNodeCompleted(onAllNodeCompleted))
	} else {
		ctx.TellFailure(msg, fmt.Errorf("ruleChain id=%s not found", chainId))
	}
}

// SetRuleChainPool 设置子规则链池
func (ctx *DefaultRuleContext) SetRuleChainPool(ruleChainPool *RuleGo) {
	ctx.ruleChainPool = ruleChainPool
}

// GetRuleChainPool 获取子规则链池
func (ctx *DefaultRuleContext) GetRuleChainPool() *RuleGo {
	if ctx.ruleChainPool == nil {
		return DefaultRuleGo
	} else {
		return ctx.ruleChainPool
	}
}

// SetOnAllNodeCompleted 设置所有节点执行完回调
func (ctx *DefaultRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
}

// ExecuteNode 从指定节点开始执行，如果 skipTellNext=true 则只执行当前节点，不通知下一个节点。
// onEnd 查看获得最终执行结果
func (ctx *DefaultRuleContext) ExecuteNode(chanCtx context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc) {
	if nodeCtx, ok := ctx.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: nodeId}); ok {
		rootCtxCopy := NewRuleContext(chanCtx, ctx.config, ctx.ruleChainCtx, nil, nodeCtx, ctx.pool, nil, ctx.ruleChainPool)
		rootCtxCopy.onEnd = onEnd
		//只执行当前节点
		rootCtxCopy.skipTellNext = skipTellNext
		rootCtxCopy.tell(msg, nil, "")
	} else {
		onEnd(ctx, msg, errors.New("node id not found nodeId="+nodeId), types.Failure)
	}
}

// DoOnEnd  结束规则链分支执行，触发 OnEnd 回调函数
func (ctx *DefaultRuleContext) DoOnEnd(msg types.RuleMsg, err error, relationType string) {
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
			ctx.onEnd(ctx, msg, err, relationType)
			ctx.childDone()
		})
	} else {
		ctx.childDone()
	}

}

func (ctx *DefaultRuleContext) SetCallbackFunc(functionName string, f interface{}) {
	if ctx.runSnapshot != nil {
		switch functionName {
		case types.CallbackFuncOnRuleChainCompleted:
			if targetFunc, ok := f.(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot)); ok {
				ctx.runSnapshot.onRuleChainCompletedFunc = targetFunc
			}
		case types.CallbackFuncOnNodeCompleted:
			if targetFunc, ok := f.(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog)); ok {
				ctx.runSnapshot.onNodeCompletedFunc = targetFunc
			}
		case types.CallbackFuncDebug:
			if targetFunc, ok := f.(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error)); ok {
				ctx.runSnapshot.onDebugCustomFunc = targetFunc
			}
		}
	}
}

func (ctx *DefaultRuleContext) GetCallbackFunc(functionName string) interface{} {
	if ctx.runSnapshot != nil {
		switch functionName {
		case types.CallbackFuncOnRuleChainCompleted:
			return ctx.runSnapshot.onRuleChainCompletedFunc
		case types.CallbackFuncOnNodeCompleted:
			return ctx.runSnapshot.onNodeCompletedFunc
		case types.CallbackFuncDebug:
			return ctx.runSnapshot.onDebugCustomFunc
		default:
			return nil
		}
	}
	return nil
}

func (ctx *DefaultRuleContext) OnDebug(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	msgCopy := msg.Copy()
	if ctx.Self() != nil && ctx.Self().IsDebugMode() {
		//异步记录日志
		ctx.SubmitTack(func() {
			if ctx.config.OnDebug != nil {
				ctx.config.OnDebug(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
			}
			if ctx.runSnapshot != nil {
				ctx.runSnapshot.onDebugCustom(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
			}
		})
	}
	if ctx.runSnapshot != nil {
		//记录快照
		ctx.runSnapshot.collectRunSnapshot(ctx, flowType, nodeId, msgCopy, relationType, err)
	}

}

// 增加一个待执行子节点
func (ctx *DefaultRuleContext) childReady() {
	atomic.AddInt32(&ctx.waitingCount, 1)
}

// 减少一个待执行子节点
// 如果返回数量0，表示该分支链条已经都执行完成，递归父节点，直到所有节点都处理完，则触发onAllNodeCompleted事件。
func (ctx *DefaultRuleContext) childDone() {
	if atomic.AddInt32(&ctx.waitingCount, -1) <= 0 {
		//该节点已经执行完成，通知父节点
		if ctx.parentRuleCtx != nil {
			ctx.parentRuleCtx.childDone()
		}

		//完成回调
		if ctx.onAllNodeCompleted != nil && atomic.LoadInt32(&ctx.onAllNodeCompletedDone) != 1 {
			atomic.StoreInt32(&ctx.onAllNodeCompletedDone, 1)
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

// tellFirst 执行第一个节点
func (ctx *DefaultRuleContext) tellFirst(msg types.RuleMsg, err error, relationTypes ...string) {
	msgCopy := msg.Copy()
	ctx.SubmitTack(func() {
		if ctx.self != nil {
			ctx.tellNext(msgCopy, ctx.self, "")
		} else {
			ctx.DoOnEnd(msgCopy, err, "")
		}
	})
}

// tellNext 通知执行子节点，如果是当前第一个节点则执行当前节点
func (ctx *DefaultRuleContext) tell(msg types.RuleMsg, err error, relationTypes ...string) {
	//msgCopy := msg.Copy()
	if ctx.isFirst {
		ctx.tellFirst(msg, err, relationTypes...)
	} else {
		if relationTypes == nil {
			//找不到子节点，则执行结束回调
			ctx.DoOnEnd(msg, err, "")
		} else {
			for _, relationType := range relationTypes {
				//执行After aop
				msg = ctx.executeAfterAop(msg, err, relationType)
				//根据relationType查找子节点列表
				if nodes, ok := ctx.getNextNodes(relationType); ok && !ctx.skipTellNext {
					for _, item := range nodes {
						tmp := item
						//增加一个待执行的子节点
						ctx.childReady()
						msgCopy := msg.Copy()
						//通知执行子节点
						ctx.SubmitTack(func() {
							ctx.tellNext(msgCopy, tmp, relationType)
						})
					}
				} else {
					//找不到子节点，则执行结束回调
					ctx.DoOnEnd(msg, err, relationType)
				}
			}
		}
	}
}

// 执行下一个节点
func (ctx *DefaultRuleContext) tellNext(msg types.RuleMsg, nextNode types.NodeCtx, relationType string) {

	defer func() {
		//捕捉异常
		if e := recover(); e != nil {
			//执行After aop
			msg = ctx.executeAfterAop(msg, fmt.Errorf("%v", e), relationType)
			ctx.childDone()
		}
	}()

	nextCtx := ctx.NewNextNodeRuleContext(nextNode)

	//环绕aop
	if !nextCtx.executeAroundAop(msg, relationType) {
		return
	}
	// AroundAop 已经执行节点OnMsg逻辑，不在执行下面的逻辑

	nextNode.OnMsg(nextCtx, msg)
}

// 执行环绕aop
// 返回值true: 继续执行下一个节点，否则不执行
func (ctx *DefaultRuleContext) executeAroundAop(msg types.RuleMsg, relationType string) bool {
	// before aop
	for _, aop := range ctx.beforeAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.Before(ctx, msg, relationType)
		}
	}

	tellNext := true
	//是否已经执行了tellNext逻辑
	//如果 AroundAspect 已经执行了tellNext逻辑，则引擎不再执行tellNext逻辑
	showTellNext := false
	for _, aop := range ctx.aroundAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg, showTellNext = aop.Around(ctx, msg, relationType)
			if !showTellNext {
				tellNext = false
			}
		}
	}
	return tellNext
}

// 执行After aop
func (ctx *DefaultRuleContext) executeAfterAop(msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	// after aop
	for _, aop := range ctx.afterAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.After(ctx, msg, err, relationType)
		}
	}
	return msg
}

// RuleEngine 规则引擎
// 每个规则引擎实例只有一个根规则链，如果没设置规则链则无法处理数据
type RuleEngine struct {
	//规则引擎实例标识
	Id string
	//配置
	Config types.Config
	//子规则链池
	RuleChainPool *RuleGo
	//根规则链
	rootRuleChainCtx *RuleChainCtx
	//规则链执行开始前置切面列表
	startAspects []types.StartAspect
	//规则链分支链执行结束切面列表
	endAspects []types.EndAspect
	//规则链执行完成切面列表
	completedAspects []types.CompletedAspect
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
	//设置切面列表
	startAspects, endAspects, completedAspects := ruleEngine.Config.GetChainAspects()
	ruleEngine.startAspects = startAspects
	ruleEngine.endAspects = endAspects
	ruleEngine.completedAspects = completedAspects

	return ruleEngine, err
}

// ReloadSelf 重新加载规则链
func (e *RuleEngine) ReloadSelf(def []byte, opts ...RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	for _, opt := range opts {
		_ = opt(e)
	}
	if e.Initialized() {
		e.rootRuleChainCtx.Config = e.Config
		//更新规则链
		err := e.rootRuleChainCtx.ReloadSelf(def)
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleChainPool(e.RuleChainPool)
		return err
	} else {
		//初始化
		if ctx, err := e.Config.Parser.DecodeRuleChain(e.Config, def); err == nil {
			if e.rootRuleChainCtx != nil {
				ctx.(*RuleChainCtx).Id = e.rootRuleChainCtx.Id
			}
			e.rootRuleChainCtx = ctx.(*RuleChainCtx)
			//设置子规则链池
			e.rootRuleChainCtx.SetRuleChainPool(e.RuleChainPool)

			//执行创建切面逻辑
			createdAspects, _, _ := e.Config.GetEngineAspects()
			for _, aop := range createdAspects {
				aop.OnCreated(e.rootRuleChainCtx)
			}

			return nil
		} else {
			return err
		}
	}

}

// ReloadChild 更新根规则链或者其下某个节点
// 如果ruleNodeId为空更新根规则链，否则更新指定的子节点
// dsl 根规则链/子节点配置
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

// DSL 获取根规则链配置
func (e *RuleEngine) DSL() []byte {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.DSL()
	} else {
		return nil
	}
}

// NodeDSL 获取规则链节点配置
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

// RootRuleChainCtx 获取根规则链
func (e *RuleEngine) RootRuleChainCtx() *RuleChainCtx {
	return e.rootRuleChainCtx
}

func (e *RuleEngine) Stop() {
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.Destroy()
	}
}

// OnMsg 把消息交给规则引擎处理，异步执行
// 提供可选参数types.RuleContextOption
func (e *RuleEngine) OnMsg(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// OnMsgAndWait 把消息交给规则引擎处理，同步执行
// 等规则链所有节点执行完后返回
func (e *RuleEngine) OnMsgAndWait(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, true, opts...)
}

// OnMsgWithEndFunc 把消息交给规则引擎处理，异步执行
// endFunc 用于数据经过规则链执行完的回调，用于获取规则链处理结果数据。注意：如果规则链有多个结束点，回调函数则会执行多次
// Deprecated
// 使用OnMsg代替
func (e *RuleEngine) OnMsgWithEndFunc(msg types.RuleMsg, endFunc types.OnEndFunc) {
	e.OnMsg(msg, types.WithOnEnd(endFunc))
}

// OnMsgWithOptions 把消息交给规则引擎处理，异步执行
// 可以携带context选项和结束回调选项
// context 用于不同组件实例数据共享
// endFunc 用于数据经过规则链执行完的回调，用于获取规则链处理结果数据。注意：如果规则链有多个结束点，回调函数则会执行多次
// Deprecated
// 使用OnMsg代替
func (e *RuleEngine) OnMsgWithOptions(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

func (e *RuleEngine) doOnAllNodeCompleted(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, customFunc func()) {
	//执行切面
	e.onAllNodeCompleted(rootCtxCopy, msg)

	if rootCtxCopy.runSnapshot != nil {
		//等待所有日志执行完
		rootCtxCopy.runSnapshot.onRuleChainCompleted(rootCtxCopy)
	}
	//触发自定义回调
	if customFunc != nil {
		customFunc()
	}

}

func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	if e.rootRuleChainCtx != nil {
		rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
		rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.RuleChainPool)
		rootCtxCopy.isFirst = rootCtx.isFirst
		rootCtxCopy.runSnapshot = NewRunSnapshot(msg.Id, rootCtxCopy.ruleChainCtx, time.Now().UnixMilli())
		for _, opt := range opts {
			opt(rootCtxCopy)
		}

		msg = e.onStart(rootCtxCopy, msg)

		//用户自定义结束回调
		customOnEndFunc := rootCtxCopy.onEnd
		rootCtxCopy.onEnd = func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			msg = e.onEnd(rootCtxCopy, msg, err, relationType)
			if customOnEndFunc != nil {
				customOnEndFunc(ctx, msg, err, relationType)
			}

		}

		customFunc := rootCtxCopy.onAllNodeCompleted
		//同步方式调用，等规则链都执行完，才返回
		if wait {
			c := make(chan struct{})
			rootCtxCopy.onAllNodeCompleted = func() {
				defer close(c)
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			//执行规则链
			rootCtxCopy.TellNext(msg)
			//阻塞
			<-c
		} else {
			rootCtxCopy.onAllNodeCompleted = func() {
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			//执行规则链
			rootCtxCopy.TellNext(msg)
		}

	} else {
		//沒有定义根则链或者没初始化
		e.Config.Logger.Printf("onMsg error.RuleEngine not initialized")
	}
}

// 执行规则链执行开始切面列表
func (e *RuleEngine) onStart(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	for _, aop := range e.startAspects {
		if aop.PointCut(ctx, msg, "") {
			msg = aop.Start(ctx, msg)
		}
	}

	return msg
}

// 执行规则链分支链执行结束切面列表
func (e *RuleEngine) onEnd(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	for _, aop := range e.endAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.End(ctx, msg, err, relationType)
		}
	}
	return msg
}

// 执行规则链所有分支链执行结束切面列表
func (e *RuleEngine) onAllNodeCompleted(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	for _, aop := range e.completedAspects {
		if aop.PointCut(ctx, msg, "") {
			msg = aop.Completed(ctx, msg)
		}
	}
	return msg
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
