/*
 * Copyright 2024 The RuleGo Authors.
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

// Package engine provides the core functionality for the RuleGo rule engine.
// It includes implementations for rule contexts, rule engines, and related components
// that enable the execution and management of rule chains.
//
// The engine package is responsible for:
// - Defining and managing rule contexts (DefaultRuleContext)
// - Implementing the main rule engine (RuleEngine)
// - Handling rule chain execution and flow control
// - Managing built-in aspects and extensions
// - Providing utilities for rule processing and message handling
//
// This package is central to the RuleGo framework, offering the primary mechanisms
// for rule-based processing and decision making in various applications.
package engine

import (
	"context"
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types/metrics"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/builtin/funcs"
)

// Ensuring DefaultRuleContext implements types.RuleContext interface.
var _ types.RuleContext = (*DefaultRuleContext)(nil)

// Ensuring RuleEngine implements types.RuleEngine interface.
var _ types.RuleEngine = (*RuleEngine)(nil)

var ErrDisabled = errors.New("the rule chain has been disabled")

// BuiltinsAspects holds a list of built-in aspects for the rule engine.
var BuiltinsAspects = []types.Aspect{&aspect.Debug{}, &aspect.MetricsAspect{}}

// ContextObserver tracks the execution state of nodes in the rule chain.
type ContextObserver struct {
	// Map of executed nodes
	executedNodes sync.Map
	// Map of input messages for each node
	nodeInMsgList map[string][]types.WrapperMsg
	// Map of callbacks for node completion events
	nodeDoneEvent map[string]joinNodeCallback
	sync.RWMutex
}

// joinNodeCallback represents a callback function for when a join node completes.
type joinNodeCallback struct {
	joinNodeId string
	parentIds  []string
	callback   func([]types.WrapperMsg)
}

// addInMsg adds an input message for a specific join node.
func (c *ContextObserver) addInMsg(joinNodeId, fromId string, msg types.RuleMsg, errStr string) bool {
	c.Lock()
	defer c.Unlock()
	if c.nodeInMsgList == nil {
		c.nodeInMsgList = make(map[string][]types.WrapperMsg)
	}
	if list, ok := c.nodeInMsgList[joinNodeId]; ok {
		list = append(list, types.WrapperMsg{
			Msg:    msg,
			Err:    errStr,
			NodeId: fromId,
		})
		c.nodeInMsgList[joinNodeId] = list
		return true
	} else {
		c.nodeInMsgList[joinNodeId] = []types.WrapperMsg{
			{
				Msg:    msg,
				Err:    errStr,
				NodeId: fromId,
			},
		}
		return false
	}
}

// getInMsgList retrieves the list of input messages for a specific join node.
func (c *ContextObserver) getInMsgList(joinNodeId string) []types.WrapperMsg {
	if c.nodeInMsgList == nil {
		return nil
	}
	c.RLock()
	defer c.RUnlock()
	return c.nodeInMsgList[joinNodeId]
}

// registerNodeDoneEvent registers a callback for when a join node completes.
func (c *ContextObserver) registerNodeDoneEvent(joinNodeId string, parentIds []string, callback func([]types.WrapperMsg)) {
	c.Lock()
	defer c.Unlock()
	if c.nodeDoneEvent == nil {
		c.nodeDoneEvent = make(map[string]joinNodeCallback)
	}
	c.nodeDoneEvent[joinNodeId] = joinNodeCallback{
		joinNodeId: joinNodeId,
		parentIds:  parentIds,
		callback:   callback,
	}
}

// checkNodesDone checks if all specified nodes have completed execution.
func (c *ContextObserver) checkNodesDone(nodeIds ...string) bool {
	for _, nodeId := range nodeIds {
		if _, ok := c.executedNodes.Load(nodeId); !ok {
			return false
		}
	}
	return true
}

// executedNode marks a node as executed and checks for any completed join nodes.
func (c *ContextObserver) executedNode(nodeId string) {
	c.executedNodes.Store(nodeId, true)
	c.checkAndTrigger()
}

// checkAndTrigger checks for completed join nodes and triggers their callbacks.
func (c *ContextObserver) checkAndTrigger() {
	if c.nodeDoneEvent != nil {
		c.Lock()
		defer c.Unlock()
		for _, item := range c.nodeDoneEvent {
			if c.checkNodesDone(item.parentIds...) {
				delete(c.nodeDoneEvent, item.joinNodeId)
				item.callback(c.nodeInMsgList[item.joinNodeId])
			}
		}
	}
}

// DefaultRuleContext is the default context for message processing in the rule engine.
type DefaultRuleContext struct {
	// Context for sharing semaphores and data across different components.
	context context.Context
	// Configuration settings for the rule engine.
	config types.Config
	// Context of the root rule chain.
	ruleChainCtx *RuleChainCtx
	// Context of the previous node.
	from types.NodeCtx
	// Context of the current node.
	self types.NodeCtx
	// Indicates if this is the first node in the chain.
	isFirst bool
	// Goroutine pool for concurrent execution.
	pool types.Pool
	// Callback function for when the rule chain branch processing ends.
	onEnd types.OnEndFunc
	// Count of child nodes that have not yet completed execution.
	waitingCount int32
	// Parent rule context.
	parentRuleCtx *DefaultRuleContext
	// Event that triggers once when all child nodes have completed, executed only once.
	onAllNodeCompleted func()
	// Indicates if the onAllNodeCompleted function has been executed.
	onAllNodeCompletedDone int32
	// Pool for sub-rule chains.
	ruleChainPool types.RuleEnginePool
	// Indicates whether to skip executing child nodes, default is false.
	skipTellNext bool
	// List of aspects.
	aspects types.AspectList
	// List of around aspects.
	aroundAspects []types.AroundAspect
	// List of before aspects.
	beforeAspects []types.BeforeAspect
	// List of after aspects.
	afterAspects []types.AfterAspect
	// Runtime snapshot for debugging and logging.
	runSnapshot *RunSnapshot
	observer    *ContextObserver
	// first node relationType
	relationTypes []string
	// OUT msg
	out types.RuleMsg
	// IN or OUT err
	err error
}

// NewRuleContext creates a new instance of the default rule engine message processing context.
func NewRuleContext(context context.Context, config types.Config, ruleChainCtx *RuleChainCtx, from types.NodeCtx, self types.NodeCtx, pool types.Pool, onEnd types.OnEndFunc, ruleChainPool types.RuleEnginePool) *DefaultRuleContext {
	// Initialize aspects list.
	var aspects types.AspectList
	if ruleChainCtx != nil {
		aspects = ruleChainCtx.aspects
	}
	// If no aspects are defined, use built-in aspects.
	if len(aspects) == 0 {
		for _, builtinsAspect := range BuiltinsAspects {
			aspects = append(aspects, builtinsAspect.New())
		}
	}
	// Get node-specific aspects.
	aroundAspects, beforeAspects, afterAspects := aspects.GetNodeAspects()
	// Return a new DefaultRuleContext populated with the provided parameters and aspects.
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
		aspects:       aspects,
		aroundAspects: aroundAspects,
		beforeAspects: beforeAspects,
		afterAspects:  afterAspects,
		observer:      &ContextObserver{},
	}
}

// RunSnapshot holds the state and logs for a rule chain execution.
type RunSnapshot struct {
	// Unique identifier for the message being processed.
	msgId string
	// Context of the rule chain being executed.
	chainCtx *RuleChainCtx
	// Timestamp marking the start of execution.
	startTs int64
	// Callback function for when the rule chain execution is completed.
	onRuleChainCompletedFunc func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot)
	// Callback function for when a node execution is completed.
	onNodeCompletedFunc func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog)
	// Logs for each node's execution.
	logs map[string]*types.RuleNodeRunLog
	// Custom debug callback function.
	onDebugCustomFunc func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error)
	// Lock for synchronizing access to logs.
	lock sync.RWMutex
}

// NewRunSnapshot creates a new instance of RunSnapshot with the given parameters.
func NewRunSnapshot(msgId string, chainCtx *RuleChainCtx, startTs int64) *RunSnapshot {
	runSnapshot := &RunSnapshot{
		msgId:    msgId,
		chainCtx: chainCtx,
		startTs:  startTs,
	}
	// Initialize the logs map.
	runSnapshot.logs = make(map[string]*types.RuleNodeRunLog)
	return runSnapshot
}

// needCollectRunSnapshot determines if there is a need to collect a snapshot of the rule chain execution.
func (r *RunSnapshot) needCollectRunSnapshot() bool {
	return r.onRuleChainCompletedFunc != nil || r.onNodeCompletedFunc != nil
}

// collectRunSnapshot collects a snapshot of the rule node's execution state.
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
	// If the flow type is 'In', update the log with the incoming message and timestamp.
	if flowType == types.In {
		nodeLog.InMsg = msg
		nodeLog.StartTs = time.Now().UnixMilli()
	}
	// If the flow type is 'Out', update the log with the outgoing message, relation type, and timestamp.
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
	// If the flow type is 'Log', append the log item to the node's log items.
	if flowType == types.Log {
		nodeLog.LogItems = append(nodeLog.LogItems, msg.Data)
	}
}

// onDebugCustom invokes the custom debug function with the provided parameters.
func (r *RunSnapshot) onDebugCustom(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
	if r.onDebugCustomFunc != nil {
		r.onDebugCustomFunc(ruleChainId, flowType, nodeId, msg, relationType, err)
	}
}

// createRuleChainRunLog creates a log of the entire rule chain's execution.
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

// onRuleChainCompleted is called when the rule chain execution is completed.
func (r *RunSnapshot) onRuleChainCompleted(ctx types.RuleContext) {
	if r.onRuleChainCompletedFunc != nil {
		r.onRuleChainCompletedFunc(ctx, r.createRuleChainRunLog(time.Now().UnixMilli()))
	}
}

// NewNextNodeRuleContext creates a new instance of RuleContext for the next node in the rule engine.
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
		observer:      ctx.observer,
		err:           ctx.err,
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

func (ctx *DefaultRuleContext) TellNextOrElse(msg types.RuleMsg, defaultRelationType string, relationTypes ...string) {
	ctx.tellOrElse(msg, nil, defaultRelationType, relationTypes...)
}

func (ctx *DefaultRuleContext) TellCollect(msg types.RuleMsg, callback func(msgList []types.WrapperMsg)) bool {

	selfNodeId := ctx.GetSelfId()
	fromId := ""
	if ctx.from != nil {
		fromId = ctx.from.GetNodeId().Id
	}
	if ctx.observer.addInMsg(selfNodeId, fromId, msg, "") {
		//因为已经存在一条合并链，则当前链提前通知父节点
		if ctx.parentRuleCtx != nil {
			ctx.parentRuleCtx.childDone()
		}
		return false
	} else {
		var parentIds []string
		if nodes, ok := ctx.ruleChainCtx.GetParentNodeIds(ctx.self.GetNodeId()); ok {
			for _, nodeId := range nodes {
				parentIds = append(parentIds, nodeId.Id)
			}
		}
		ctx.observer.registerNodeDoneEvent(selfNodeId, parentIds, func(inMsgList []types.WrapperMsg) {
			callback(inMsgList)
		})
		if ctx.from != nil {
			ctx.observer.executedNode(ctx.from.GetNodeId().Id)
		}
		return true
	}

}

func (ctx *DefaultRuleContext) NewMsg(msgType string, metaData types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}

func (ctx *DefaultRuleContext) GetSelfId() string {
	if ctx.self == nil {
		return ""
	}
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
// onAllNodeCompleted 所以节点执行完触发，无结果返回
// 如果找不到规则链，并把消息通过`Failure`关系发送到下一个节点
func (ctx *DefaultRuleContext) TellFlow(chanCtx context.Context, ruleChainId string, msg types.RuleMsg, onEndFunc types.OnEndFunc, onAllNodeCompleted func()) {
	if e, ok := ctx.GetRuleChainPool().Get(ruleChainId); ok {
		e.OnMsg(msg, types.WithOnEnd(onEndFunc), types.WithContext(chanCtx), types.WithOnAllNodeCompleted(onAllNodeCompleted))
	} else {
		ctx.TellFailure(msg, fmt.Errorf("ruleChain id=%s not found", ruleChainId))
	}
}

// TellNode 从指定节点开始执行，如果 skipTellNext=true 则只执行当前节点，不通知下一个节点。
// onEnd 查看获得最终执行结果
// onAllNodeCompleted 所以节点执行完触发，无结果返回
func (ctx *DefaultRuleContext) TellNode(chanCtx context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc, onAllNodeCompleted func()) {
	if nodeCtx, ok := ctx.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: nodeId}); ok {
		rootCtxCopy := NewRuleContext(chanCtx, ctx.config, ctx.ruleChainCtx, nil, nodeCtx, ctx.pool, onEnd, ctx.ruleChainPool)
		rootCtxCopy.onAllNodeCompleted = onAllNodeCompleted
		//Whether to only execute the current node
		rootCtxCopy.skipTellNext = skipTellNext
		rootCtxCopy.tell(msg, nil, "")
	} else {
		if onEnd != nil {
			onEnd(ctx, msg, fmt.Errorf("node id=%s not found", nodeId), types.Failure)
		}
		if onAllNodeCompleted != nil {
			onAllNodeCompleted()
		}
	}
}

func (ctx *DefaultRuleContext) TellChainNode(chanCtx context.Context, ruleChainId, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc, onAllNodeCompleted func()) {
	// Tell current chain node
	if ruleChainId == "" || (ctx.ruleChainCtx != nil && ctx.ruleChainCtx.Id.Id == ruleChainId) {
		ctx.TellNode(chanCtx, nodeId, msg, skipTellNext, onEnd, onAllNodeCompleted)
	} else {
		// Tell other chain node
		ctx.tellOtherChainNode(chanCtx, ruleChainId, nodeId, msg, skipTellNext, onEnd, onAllNodeCompleted)
	}
}

func (ctx *DefaultRuleContext) tellOtherChainNode(chanCtx context.Context, ruleChainId, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc, onAllNodeCompleted func()) {
	if e, ok := ctx.GetRuleChainPool().Get(ruleChainId); ok {
		rootCtx := e.RootRuleContext()
		if rootCtx == nil {
			if onEnd != nil {
				onEnd(ctx, msg, fmt.Errorf("ruleChain id=%s root rule context is nil", ruleChainId), types.Failure)
			}
			if onAllNodeCompleted != nil {
				onAllNodeCompleted()
			}
			return
		}
		rootCtx.TellNode(chanCtx, nodeId, msg, skipTellNext, onEnd, onAllNodeCompleted)
	} else {
		if onEnd != nil {
			onEnd(ctx, msg, fmt.Errorf("ruleChain id=%s not found", ruleChainId), types.Failure)
		}
		if onAllNodeCompleted != nil {
			onAllNodeCompleted()
		}
	}
}

// SetRuleChainPool 设置子规则链池
func (ctx *DefaultRuleContext) SetRuleChainPool(ruleChainPool types.RuleEnginePool) {
	ctx.ruleChainPool = ruleChainPool
}

// GetRuleChainPool 获取子规则链池
func (ctx *DefaultRuleContext) GetRuleChainPool() types.RuleEnginePool {
	if ctx.ruleChainPool == nil {
		return DefaultPool
	} else {
		return ctx.ruleChainPool
	}
}

// SetOnAllNodeCompleted 设置所有节点执行完回调
func (ctx *DefaultRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
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
	if ctx.IsDebugMode() {
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

func (ctx *DefaultRuleContext) SetExecuteNode(nodeId string, relationTypes ...string) {
	//如果relationTypes为空，则执行当前节点
	ctx.isFirst = len(relationTypes) == 0
	//否则通过relationTypes查找子节点执行
	ctx.relationTypes = relationTypes
	if node, ok := ctx.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: nodeId}); ok {
		ctx.self = node
	} else {
		ctx.err = fmt.Errorf("SetExecuteNode node id=%s not found", nodeId)
	}
}

func (ctx *DefaultRuleContext) GetOut() types.RuleMsg {
	return ctx.out
}

func (ctx *DefaultRuleContext) GetErr() error {
	return ctx.err
}

// IsDebugMode 是否调试模式，优先使用规则链指定的调试模式
func (ctx *DefaultRuleContext) IsDebugMode() bool {
	if ctx.ruleChainCtx.IsDebugMode() {
		return true
	}
	return ctx.Self() != nil && ctx.Self().IsDebugMode()
}

// 增加一个待执行子节点
func (ctx *DefaultRuleContext) childReady() {
	atomic.AddInt32(&ctx.waitingCount, 1)
}

// 减少一个待执行子节点
// 如果返回数量0，表示该分支链条已经都执行完成，递归父节点，直到所有节点都处理完，则触发onAllNodeCompleted事件。
func (ctx *DefaultRuleContext) childDone() {
	if atomic.AddInt32(&ctx.waitingCount, -1) <= 0 {
		if atomic.CompareAndSwapInt32(&ctx.onAllNodeCompletedDone, 0, 1) {

			//该节点已经执行完成，通知父节点
			if ctx.parentRuleCtx != nil {
				ctx.parentRuleCtx.childDone()
			}
			if ctx.parentRuleCtx == nil || ctx.GetSelfId() != ctx.parentRuleCtx.GetSelfId() {
				//记录当前节点执行完成
				ctx.observer.executedNode(ctx.GetSelfId())
			}
			//完成回调
			if ctx.onAllNodeCompleted != nil {
				ctx.onAllNodeCompleted()
			}
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

// tellSelf 执行自身节点
func (ctx *DefaultRuleContext) tellSelf(msg types.RuleMsg, err error, relationTypes ...string) {
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
	ctx.tellOrElse(msg, err, "", relationTypes...)
}

// tellNext 通知执行子节点，如果是当前第一个节点则执行当前节点
// 如果找不到relationTypes对应的节点，而且defaultRelationType非默认值，则通过defaultRelationType查找节点
func (ctx *DefaultRuleContext) tellOrElse(msg types.RuleMsg, err error, defaultRelationType string, relationTypes ...string) {
	ctx.out = msg
	ctx.err = err
	//msgCopy := msg.Copy()
	if ctx.isFirst {
		ctx.tellSelf(msg, err, relationTypes...)
	} else {
		if relationTypes == nil {
			//找不到子节点，则执行结束回调
			ctx.DoOnEnd(msg, err, "")
		} else {
			for _, relationType := range relationTypes {
				//执行After aop
				msg = ctx.executeAfterAop(msg, err, relationType)
				var ok = false
				var nodes []types.NodeCtx
				//根据relationType查找子节点列表
				nodes, ok = ctx.getNextNodes(relationType)
				//根据默认关系查找节点
				if defaultRelationType != "" && (!ok || len(nodes) == 0) && !ctx.skipTellNext {
					nodes, ok = ctx.getNextNodes(defaultRelationType)
				}
				if ok && !ctx.skipTellNext {
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

// RuleEngine is the core structure for a rule engine instance.
// Each RuleEngine instance has only one root rule chain, and it cannot process data without a set rule chain.
type RuleEngine struct {
	// Config is the configuration for the rule engine.
	Config types.Config
	// ruleChainPool is a pool of rule engine.
	ruleChainPool types.RuleEnginePool
	// id is the unique identifier for the rule engine instance.
	id string
	// rootRuleChainCtx is the context of the root rule chain.
	rootRuleChainCtx *RuleChainCtx
	// startAspects is a list of aspects that are applied before the execution of the rule chain starts.
	startAspects []types.StartAspect
	// endAspects is a list of aspects that are applied when a branch of the rule chain ends.
	endAspects []types.EndAspect
	// completedAspects is a list of aspects that are applied when the rule chain execution is completed.
	completedAspects []types.CompletedAspect
	// initialized indicates whether the rule engine has been initialized.
	initialized bool
	// Aspects is a list of AOP (Aspect-Oriented Programming) aspects.
	Aspects types.AspectList
}

// newRuleEngine creates a new RuleEngine instance with the given ID and definition.
// It applies the provided RuleEngineOptions during the creation process.
func newRuleEngine(id string, def []byte, opts ...types.RuleEngineOption) (*RuleEngine, error) {
	if len(def) == 0 {
		return nil, errors.New("def can not nil")
	}
	// Create a new RuleEngine with the Id
	ruleEngine := &RuleEngine{
		id:            id,
		Config:        NewConfig(),
		ruleChainPool: DefaultPool,
	}
	err := ruleEngine.ReloadSelf(def, opts...)
	if err == nil && ruleEngine.rootRuleChainCtx != nil {
		if id != "" {
			ruleEngine.rootRuleChainCtx.Id = types.RuleNodeId{Id: id, Type: types.CHAIN}
		} else {
			// Use the rule chain ID if no ID is provided.
			ruleEngine.id = ruleEngine.rootRuleChainCtx.Id.Id
		}
	}

	return ruleEngine, err
}

func (e *RuleEngine) Id() string {
	return e.id
}
func (e *RuleEngine) SetConfig(config types.Config) {
	e.Config = config
}

func (e *RuleEngine) SetAspects(aspects ...types.Aspect) {
	e.Aspects = aspects
}

func (e *RuleEngine) SetRuleEnginePool(ruleChainPool types.RuleEnginePool) {
	e.ruleChainPool = ruleChainPool
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.SetRuleEnginePool(ruleChainPool)
	}
}

func (e *RuleEngine) GetAspects() types.AspectList {
	return e.Aspects
}

func (e *RuleEngine) Reload(opts ...types.RuleEngineOption) error {
	return e.ReloadSelf(e.DSL(), opts...)
}

func (e *RuleEngine) initBuiltinsAspects() {
	var newAspects types.AspectList
	//初始化内置切面
	if len(e.Aspects) == 0 {
		for _, builtinsAspect := range BuiltinsAspects {
			newAspects = append(newAspects, builtinsAspect.New())
		}
	} else {
		for _, item := range e.Aspects {
			newAspects = append(newAspects, item.New())
		}

		for _, builtinsAspect := range BuiltinsAspects {
			found := false
			for _, item := range newAspects {
				//判断是否是相同类型
				if reflect.TypeOf(item) == reflect.TypeOf(builtinsAspect) {
					found = true
					break
				}
			}
			if !found {
				newAspects = append(newAspects, builtinsAspect.New())
			}
		}
	}
	e.Aspects = newAspects
}

func (e *RuleEngine) initChain(def types.RuleChain) error {
	if def.RuleChain.Disabled {
		return ErrDisabled
	}
	if ctx, err := InitRuleChainCtx(e.Config, e.Aspects, &def); err == nil {
		if e.rootRuleChainCtx != nil {
			ctx.Id = e.rootRuleChainCtx.Id
		}
		e.rootRuleChainCtx = ctx
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
		//执行创建切面逻辑
		_, _, createdAspects, _, _ := e.Aspects.GetEngineAspects()
		for _, aop := range createdAspects {
			if err := aop.OnCreated(e.rootRuleChainCtx); err != nil {
				return err
			}
		}
		e.initialized = true
		return nil
	} else {
		return err
	}
}

// ReloadSelf 重新加载规则链
func (e *RuleEngine) ReloadSelf(dsl []byte, opts ...types.RuleEngineOption) error {
	// Apply the options to the RuleEngine.
	for _, opt := range opts {
		_ = opt(e)
	}
	var err error
	if e.Initialized() {
		//初始化内置切面
		if len(e.Aspects) == 0 {
			e.initBuiltinsAspects()
		}
		e.rootRuleChainCtx.config = e.Config
		e.rootRuleChainCtx.SetAspects(e.Aspects)
		//更新规则链
		err = e.rootRuleChainCtx.ReloadSelf(dsl)
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
	} else {
		//初始化内置切面
		e.initBuiltinsAspects()
		var rootRuleChainDef types.RuleChain
		//初始化
		if rootRuleChainDef, err = e.Config.Parser.DecodeRuleChain(dsl); err == nil {
			err = e.initChain(rootRuleChainDef)
		} else {
			return err
		}
	}
	// Set the aspect lists.
	startAspects, endAspects, completedAspects := e.Aspects.GetChainAspects()
	e.startAspects = startAspects
	e.endAspects = endAspects
	e.completedAspects = completedAspects
	return err
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

func (e *RuleEngine) Definition() types.RuleChain {
	if e.rootRuleChainCtx != nil {
		return *e.rootRuleChainCtx.SelfDefinition
	} else {
		return types.RuleChain{}
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
	return e.initialized && e.rootRuleChainCtx != nil
}

// RootRuleChainCtx 获取根规则链
func (e *RuleEngine) RootRuleChainCtx() types.ChainCtx {
	return e.rootRuleChainCtx
}

func (e *RuleEngine) Stop() {
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.Destroy()
	}
	e.initialized = false
}

// OnMsg asynchronously processes a message using the rule engine.
// It accepts optional RuleContextOption parameters to customize the execution context.
func (e *RuleEngine) OnMsg(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// OnMsgAndWait synchronously processes a message using the rule engine and waits for all nodes in the rule chain to complete before returning.
func (e *RuleEngine) OnMsgAndWait(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, true, opts...)
}

// RootRuleContext returns the root rule context.
func (e *RuleEngine) RootRuleContext() types.RuleContext {
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.rootRuleContext
	}
	return nil
}

func (e *RuleEngine) GetMetrics() *metrics.EngineMetrics {
	for _, aop := range e.Aspects {
		if metricsAspect, ok := aop.(*aspect.MetricsAspect); ok {
			return metricsAspect.GetMetrics()
		}
	}
	return nil
}

// OnMsgWithEndFunc is a deprecated method that asynchronously processes a message using the rule engine.
// The endFunc callback is used to obtain the results after the rule chain execution is complete.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use OnMsg instead.
func (e *RuleEngine) OnMsgWithEndFunc(msg types.RuleMsg, endFunc types.OnEndFunc) {
	e.OnMsg(msg, types.WithOnEnd(endFunc))
}

// OnMsgWithOptions is a deprecated method that asynchronously processes a message using the rule engine.
// It allows carrying context options and an end callback option.
// The context is used for sharing data between different component instances.
// The endFunc callback is used to obtain the results after the rule chain execution is complete.
// Note: If the rule chain has multiple endpoints, the callback function will be executed multiple times.
// Deprecated: Use OnMsg instead.
func (e *RuleEngine) OnMsgWithOptions(msg types.RuleMsg, opts ...types.RuleContextOption) {
	e.onMsgAndWait(msg, false, opts...)
}

// doOnAllNodeCompleted handles the completion of all nodes within the rule chain.
// It executes aspects, completes the run snapshot, and triggers any custom callback functions.
func (e *RuleEngine) doOnAllNodeCompleted(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, customFunc func()) {
	// Execute aspects upon completion of all nodes.
	e.onAllNodeCompleted(rootCtxCopy, msg)

	// Complete the run snapshot if it exists.
	if rootCtxCopy.runSnapshot != nil {
		rootCtxCopy.runSnapshot.onRuleChainCompleted(rootCtxCopy)
	}
	// Trigger custom callback if provided.
	if customFunc != nil {
		customFunc()
	}
}

// onErrHandler handles the scenario where the rule chain has no nodes or fails to process the message.
// It logs an error and triggers the end-of-chain callbacks.
func (e *RuleEngine) onErrHandler(msg types.RuleMsg, rootCtxCopy *DefaultRuleContext, err error) {
	// Trigger the configured OnEnd callback with the error.
	if rootCtxCopy.config.OnEnd != nil {
		rootCtxCopy.config.OnEnd(msg, err)
	}
	// Trigger the onEnd callback with the error and Failure relation type.
	if rootCtxCopy.onEnd != nil {
		rootCtxCopy.onEnd(rootCtxCopy, msg, err, types.Failure)
	}
	// Execute the onAllNodeCompleted callback if it exists.
	if rootCtxCopy.onAllNodeCompleted != nil {
		rootCtxCopy.onAllNodeCompleted()
	}
}

// onMsgAndWait processes a message through the rule engine, optionally waiting for all nodes to complete.
// It applies any provided RuleContextOptions to customize the execution context.
func (e *RuleEngine) onMsgAndWait(msg types.RuleMsg, wait bool, opts ...types.RuleContextOption) {
	if e.rootRuleChainCtx != nil {
		// Create a copy of the root context for processing the message.
		rootCtx := e.rootRuleChainCtx.rootRuleContext.(*DefaultRuleContext)
		rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, e.ruleChainPool)
		rootCtxCopy.isFirst = rootCtx.isFirst
		rootCtxCopy.runSnapshot = NewRunSnapshot(msg.Id, rootCtxCopy.ruleChainCtx, time.Now().UnixMilli())
		// Apply the provided options to the context copy.
		for _, opt := range opts {
			opt(rootCtxCopy)
		}
		// Handle the case where the rule chain has no nodes.
		if rootCtxCopy.ruleChainCtx.isEmpty {
			e.onErrHandler(msg, rootCtxCopy, errors.New("the rule chain has no nodes"))
			return
		}
		if rootCtxCopy.err != nil {
			e.onErrHandler(msg, rootCtxCopy, rootCtxCopy.err)
			return
		}
		var err error
		// Execute start aspects and update the message accordingly.
		msg, err = e.onStart(rootCtxCopy, msg)
		if err != nil {
			e.onErrHandler(msg, rootCtxCopy, err)
			return
		}
		// Set up a custom end callback function.
		customOnEndFunc := rootCtxCopy.onEnd
		rootCtxCopy.onEnd = func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// Execute end aspects and update the message accordingly.
			msg = e.onEnd(rootCtxCopy, msg, err, relationType)
			// Trigger the custom end callback if provided.
			if customOnEndFunc != nil {
				customOnEndFunc(ctx, msg, err, relationType)
			}

		}
		// Set up a custom function to be called upon completion of all nodes.
		customFunc := rootCtxCopy.onAllNodeCompleted
		// If waiting is required, set up a channel to synchronize the completion.
		if wait {
			c := make(chan struct{})
			rootCtxCopy.onAllNodeCompleted = func() {
				defer close(c)
				// Execute the completion handling function.
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			// Process the message through the rule chain.
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
			// Block until all nodes have completed.
			<-c
		} else {
			// If not waiting, simply set the completion handling function.
			rootCtxCopy.onAllNodeCompleted = func() {
				e.doOnAllNodeCompleted(rootCtxCopy, msg, customFunc)
			}
			// Process the message through the rule chain.
			rootCtxCopy.TellNext(msg, rootCtxCopy.relationTypes...)
		}

	} else {
		// Log an error if the rule engine is not initialized or the root rule chain is not defined.
		e.Config.Logger.Printf("onMsg error.RuleEngine not initialized")
	}
}

// onStart executes the list of start aspects before the rule chain begins processing a message.
func (e *RuleEngine) onStart(ctx types.RuleContext, msg types.RuleMsg) (types.RuleMsg, error) {
	var err error
	for _, aop := range e.startAspects {
		if aop.PointCut(ctx, msg, "") {
			if err != nil {
				return msg, err
			}
			msg, err = aop.Start(ctx, msg)
		}
	}
	return msg, err
}

// onEnd executes the list of end aspects when a branch of the rule chain ends.
func (e *RuleEngine) onEnd(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	for _, aop := range e.endAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.End(ctx, msg, err, relationType)
		}
	}
	return msg
}

// onAllNodeCompleted executes the list of completed aspects after all branches of the rule chain have ended.
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
	// register all udfs
	for name, f := range funcs.ScriptFunc.GetAll() {
		c.RegisterUdf(name, f)
	}
	return c
}

// WithConfig is an option that sets the Config of the RuleEngine.
func WithConfig(config types.Config) types.RuleEngineOption {
	return func(re types.RuleEngine) error {
		re.SetConfig(config)
		return nil
	}
}
