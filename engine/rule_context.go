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

package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/cache"
)

// Ensuring DefaultRuleContext implements types.RuleContext interface.
var _ types.RuleContext = (*DefaultRuleContext)(nil)

// GetEnv 获取环境变量和元数据
func (ctx *DefaultRuleContext) GetEnv(msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// 预分配合适大小的map，减少扩容开销
	capacity := 7 // 基础字段数量：id, ts, data, msgType, dataType, msg, metadata

	// 预先获取metadata，避免重复调用Values()
	var metadataValues map[string]string
	if msg.Metadata != nil {
		metadataValues = msg.Metadata.Values()
		if useMetadata {
			capacity += len(metadataValues)
		}
	}

	evn := make(map[string]interface{}, capacity)

	// 设置基础字段
	evn[types.IdKey] = msg.Id
	evn[types.TsKey] = msg.Ts
	evn[types.DataKey] = msg.GetData()
	evn[types.MsgTypeKey] = msg.Type
	evn[types.DataTypeKey] = msg.DataType

	// 优化JSON数据处理
	if msg.DataType == types.JSON {
		if jsonData, err := msg.GetDataAsJson(); err == nil {
			evn[types.MsgKey] = jsonData
		} else {
			evn[types.MsgKey] = msg.GetData()
		}
	} else {
		evn[types.MsgKey] = msg.GetData()
	}

	// 处理metadata，只调用一次Values()
	if metadataValues != nil {
		if useMetadata {
			// 将metadata键值对添加到环境变量中
			for k, v := range metadataValues {
				evn[k] = v
			}
		}
		// 总是设置metadata字段
		evn[types.MetadataKey] = metadataValues
	}

	return evn
}

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
	err        error
	chainCache types.Cache
}

func (ctx *DefaultRuleContext) GlobalCache() types.Cache {
	return ctx.config.Cache
}

func (ctx *DefaultRuleContext) ChainCache() types.Cache {
	return ctx.chainCache
}

// NewRuleContext creates a new instance of the default rule engine message processing context.
func NewRuleContext(context context.Context, config types.Config, ruleChainCtx *RuleChainCtx, from types.NodeCtx, self types.NodeCtx, pool types.Pool, onEnd types.OnEndFunc, ruleChainPool types.RuleEnginePool) *DefaultRuleContext {
	var chainId string
	// Initialize aspects list.
	var aspects types.AspectList
	if ruleChainCtx != nil {
		aspects = ruleChainCtx.aspects
		chainId = ruleChainCtx.GetNodeId().Id
	}
	// If no aspects are defined, use built-in aspects.
	if len(aspects) == 0 {
		for _, builtinsAspect := range BuiltinsAspects {
			aspects = append(aspects, builtinsAspect.New())
		}
	}
	// Get node-specific aspects.
	aroundAspects, beforeAspects, afterAspects := aspects.GetNodeAspects()
	var chainCache types.Cache
	if chainId != "" {
		chainCache = cache.NewNamespaceCache(config.Cache, chainId+types.NamespaceSeparator)
	}
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
		chainCache:    chainCache,
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
		nodeLog.LogItems = append(nodeLog.LogItems, msg.GetData())
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
	// Create a new context directly instead of using object pool to avoid data races
	nextCtx := &DefaultRuleContext{
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
		chainCache:    ctx.chainCache,
	}

	return nextCtx
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
	var errStr string
	if ctx.GetErr() != nil {
		errStr = ctx.GetErr().Error()
	}
	if ctx.observer.addInMsg(selfNodeId, fromId, msg, errStr) {
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

func (ctx *DefaultRuleContext) NewMsg(msgType string, metaData *types.Metadata, data string) types.RuleMsg {
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

// Deprecated: Use Flow SubmitTask instead.
func (ctx *DefaultRuleContext) SubmitTack(task func()) {
	ctx.SubmitTask(task)
}

func (ctx *DefaultRuleContext) SubmitTask(task func()) {
	if ctx.pool != nil {
		// 在提交任务前捕获需要的值，避免并发访问
		logger := ctx.config.Logger
		if err := ctx.pool.Submit(task); err != nil {
			logger.Printf("SubmitTask error:%s", err)
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
	// 拷贝msg
	safeMsgCopy := msg.Copy()
	// 确保Metadata不为nil，避免空指针异常
	if safeMsgCopy.Metadata == nil {
		safeMsgCopy.SetMetadata(types.NewMetadata())
	}

	// 在提交异步任务前捕获需要的值，避免并发访问
	configOnEnd := ctx.config.OnEnd
	contextOnEnd := ctx.onEnd

	//全局回调
	//通过`Config.OnEnd`设置
	if configOnEnd != nil {
		ctx.SubmitTask(func() {
			configOnEnd(safeMsgCopy, err)
		})
	}
	//单条消息的context回调
	//通过OnMsgWithEndFunc(msg, endFunc)设置
	if contextOnEnd != nil {
		ctx.SubmitTask(func() {
			contextOnEnd(ctx, safeMsgCopy, err, relationType)
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
	// 在方法开始时就缓存runSnapshot引用，避免并发竞态条件
	runSnapshot := ctx.runSnapshot
	if ctx.IsDebugMode() {
		// 在提交异步任务前捕获需要的值，避免并发访问
		onDebugFunc := ctx.config.OnDebug

		//异步记录日志
		ctx.SubmitTask(func() {
			if onDebugFunc != nil {
				onDebugFunc(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
			}
			if runSnapshot != nil {
				runSnapshot.onDebugCustom(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
			}
		})
	}
	if runSnapshot != nil {
		//记录快照
		runSnapshot.collectRunSnapshot(ctx, flowType, nodeId, msgCopy, relationType, err)
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

			// 在进行任何异步操作前捕获需要的值，避免对象池重用导致的数据竞争
			parentRuleCtx := ctx.parentRuleCtx
			selfId := ctx.GetSelfId()
			var parentSelfId string
			if parentRuleCtx != nil {
				parentSelfId = parentRuleCtx.GetSelfId()
			}
			observer := ctx.observer
			onAllNodeCompleted := ctx.onAllNodeCompleted

			//该节点已经执行完成，通知父节点
			if parentRuleCtx != nil {
				parentRuleCtx.childDone()
			}
			if parentRuleCtx == nil || selfId != parentSelfId {
				//记录当前节点执行完成
				observer.executedNode(selfId)
			}
			//完成回调
			if onAllNodeCompleted != nil {
				onAllNodeCompleted()
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
	var relationType string
	if len(relationTypes) > 0 {
		relationType = relationTypes[0]
	}
	if ctx.self != nil {
		msgCopy := msg.Copy()
		ctx.SubmitTask(func() {
			ctx.tellNext(msgCopy, ctx.self, relationType)
		})
	} else {
		ctx.DoOnEnd(msg, err, relationType)
	}
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
						//为每个子节点创建独立的消息副本，避免并发竞态条件
						msgCopy := msg.Copy()
						//通知执行子节点
						ctx.SubmitTask(func() {
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
		// 如果AroundAspect阻止了执行，需要调用childDone来平衡之前的childReady
		ctx.childDone()
		return
	}
	// AroundAop 已经执行节点OnMsg逻辑，不在执行下面的逻辑

	nextNode.OnMsg(nextCtx, msg)
}
