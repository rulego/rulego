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

// GetEnv retrieves environment variables and metadata
func (ctx *DefaultRuleContext) getEnv(msg types.RuleMsg, useMetadata bool, nodeIds ...string) map[string]interface{} {
	// Pre-partitioning combined with appropriately sized maps reduces expansion costs
	capacity := 9 // Basic field count: id, ts, data, msgType, dataType, msg, metadata
	if msg.Metadata != nil && useMetadata {
		// Estimate the number of key-value pairs in metadata
		capacity += 8 // Estimates of common metadata quantities
	}

	evn := make(map[string]interface{}, capacity)

	if ctx.Config().Properties != nil {
		evn[types.Global] = ctx.Config().Properties.Values()
	}
	if ctx.ruleChainCtx != nil {
		evn[types.Vars] = ctx.ruleChainCtx.vars
	}

	// Set up the basic field
	evn[types.IdKey] = msg.Id
	evn[types.TsKey] = msg.Ts
	evn[types.DataKey] = msg.GetData()
	evn[types.MsgTypeKey] = msg.Type
	evn[types.DataTypeKey] = msg.DataType

	// Optimized JSON data processing
	if msg.DataType == types.JSON {
		if jsonData, err := msg.GetJsonData(); err == nil {
			evn[types.MsgKey] = jsonData
		} else {
			evn[types.MsgKey] = msg.GetData()
		}
	} else {
		evn[types.MsgKey] = msg.GetData()
	}

	// Processing metadata - optimized with zero-copy ForEach
	if msg.Metadata != nil {
		if useMetadata {
			// Use zero copy ForEach to add metadata key-value pairs to environment variables
			msg.Metadata.ForEach(func(k, v string) bool {
				evn[k] = v
				return true // continue iteration
			})
		}
		evn[types.MetadataKey] = msg.Metadata.GetReadOnlyValues()
	}

	return evn
}

// GetEnv retrieves environment variables and metadata, supporting cross-node values
// msg: Current news
// useMetadata: Whether metadata is included
// Returns a context map containing cross-node data, formatted as nodeId.msg.xx and nodeId.metadata.xx
func (ctx *DefaultRuleContext) GetEnv(msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// Obtain basic environment variables and metadata
	baseContext := ctx.getEnv(msg, useMetadata)

	if ctx.nodeOutputCache == nil {
		return baseContext
	}

	// Determine the list of node IDs to be accessed
	var targetNodeIds []string
	// Automatically retrieves the list of dependent node IDs for the current node
	if ctx.ruleChainCtx != nil {
		currentNodeId := ctx.GetSelfId()
		targetNodeIds = ctx.ruleChainCtx.GetNodeDependencies(currentNodeId)
	}

	// Add cross-node data for each node ID
	for _, nodeId := range targetNodeIds {
		if nodeMsg, found := ctx.nodeOutputCache.GetNodeRuleMsg(nodeId); found {
			baseContext[nodeId] = ctx.getEnv(nodeMsg, false)
		}
	}

	return baseContext
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
func (c *ContextObserver) registerNodeDoneEvent(joinNodeId, lcaNodeId string, callback func([]types.WrapperMsg)) {
	c.Lock()
	if c.nodeDoneEvent == nil {
		c.nodeDoneEvent = make(map[string]joinNodeCallback)
	}
	c.nodeDoneEvent[joinNodeId] = joinNodeCallback{
		lcaNodeId:  lcaNodeId,
		joinNodeId: joinNodeId,
		callback:   callback,
	}
	c.Unlock()
	c.checkAndTrigger()
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
	c.Lock()
	defer c.Unlock()

	if c.nodeDoneEvent != nil {
		for joinNodeId, item := range c.nodeDoneEvent {
			if c.checkNodesDone(item.lcaNodeId) {
				delete(c.nodeDoneEvent, joinNodeId)
				// Retrieve the message list and trigger a callback
				msgList := c.nodeInMsgList[joinNodeId]
				if msgList == nil {
					msgList = []types.WrapperMsg{}
				}
				// Directly executes callbacks to maintain the original synchronization behavior
				item.callback(msgList)
			}
		}
	}
}

// joinNodeCallback represents a callback function for when a join node completes.
type joinNodeCallback struct {
	lcaNodeId  string //joinNodeId node is the most recent common ancestor node
	joinNodeId string
	callback   func([]types.WrapperMsg)
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
	// Observer for join nodes - Delayed initialization
	observer *ContextObserver
	// first node relationType
	relationTypes []string
	// OUT msg
	out types.RuleMsg
	// IN or OUT err
	err        error
	chainCache types.Cache
	// nodeOutputCache node output cache, used to retrieve values across nodes
	nodeOutputCache *NodeOutputCache
	//Does the chain have termination nodes?
	hasEndNode bool
	// restoreNodeInfo restores the execution node information
	restoreNodeInfo *RestoreNodeInfo
	// debugModeOverride runtime debug according to message override mode, with priority above chain/node DSL configuration
	// 0 = inheritance chain or node configuration, 1 = forced open, -1 = forced close
	debugModeOverride int32
}

// RestoreNodeInfo restores executable node information
type RestoreNodeInfo struct {
	// NodeRequests Restore the list of executed node requests
	NodeRequests []types.NodeRequest
}

func (ctx *DefaultRuleContext) GlobalCache() types.Cache {
	return ctx.config.Cache
}

func (ctx *DefaultRuleContext) ChainCache() types.Cache {
	return ctx.chainCache
}

// GetNodeOutputCache Obtains the node's output cache
// GetNodeOutputCache returns the node output cache
func (ctx *DefaultRuleContext) GetNodeOutputCache() *NodeOutputCache {
	return ctx.nodeOutputCache
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
	hasEndNode := false
	if ruleChainCtx != nil {
		hasEndNode = ruleChainCtx.HasEndNode()
	}
	// Return a new DefaultRuleContext populated with the provided parameters and aspects.
	return &DefaultRuleContext{
		context:         context,
		config:          config,
		ruleChainCtx:    ruleChainCtx,
		from:            from,
		self:            self,
		isFirst:         from == nil,
		pool:            pool,
		onEnd:           onEnd,
		ruleChainPool:   ruleChainPool,
		aspects:         aspects,
		aroundAspects:   aroundAspects,
		beforeAspects:   beforeAspects,
		afterAspects:    afterAspects,
		observer:        &ContextObserver{},
		chainCache:      chainCache,
		nodeOutputCache: &NodeOutputCache{},
		hasEndNode:      hasEndNode,
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
	r.lock.Lock()
	nodeLog, ok := r.logs[nodeId]
	if !ok {
		nodeLog = &types.RuleNodeRunLog{
			Id: nodeId,
		}
		r.logs[nodeId] = nodeLog
	}
	// If the flow type is 'In', update the log with the incoming message and timestamp.
	if flowType == types.In {
		nodeLog.InMsg = msg
		nodeLog.StartTs = time.Now().UnixMilli()
	}
	// If the flow type is 'Out', update the log with the outgoing message, relation type, and timestamp.
	var logCopy types.RuleNodeRunLog
	triggerCallback := false
	if flowType == types.Out {
		nodeLog.OutMsg = msg
		nodeLog.RelationType = relationType
		if err != nil {
			nodeLog.Err = err.Error()
		}
		nodeLog.EndTs = time.Now().UnixMilli()
		if r.onNodeCompletedFunc != nil {
			logCopy = *nodeLog
			triggerCallback = true
		}
	}
	// If the flow type is 'Log', append the log item to the node's log items.
	if flowType == types.Log {
		nodeLog.LogItems = append(nodeLog.LogItems, msg.GetData())
	}
	r.lock.Unlock()

	if triggerCallback {
		r.onNodeCompletedFunc(ctx, logCopy)
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
// Predefine commonly used relationship types in singleton slice to avoid duplicate allocation
// Pre-defined singleton slices for common relation types to avoid repeated allocations
var (
	successRelationTypes = []string{types.Success}
	failureRelationTypes = []string{types.Failure}
	trueRelationTypes    = []string{types.True}
	falseRelationTypes   = []string{types.False}
)

// NewNextNodeRuleContext creates the rule context for the next node
// NewNextNodeRuleContext creates a rule context for the next node
func (ctx *DefaultRuleContext) NewNextNodeRuleContext(nextNode types.NodeCtx) *DefaultRuleContext {
	// Create a new context directly instead of using object pool to avoid data races
	// However, it reuses immutable shared states to reduce memory overhead
	nextCtx := &DefaultRuleContext{
		config:        ctx.config,       // Shared configuration, immutable
		ruleChainCtx:  ctx.ruleChainCtx, // Shared rule chain context
		from:          ctx.self,
		self:          nextNode,
		pool:          ctx.pool, // Shared coroutine pool
		onEnd:         ctx.onEnd,
		ruleChainPool: ctx.ruleChainPool, // Shared rule chain pool
		context:       ctx.context,       // Directly reuse context to avoid calling GetContext()
		parentRuleCtx: ctx,
		skipTellNext:  ctx.skipTellNext,

		// Shared faceted lists do not change at runtime
		aroundAspects: ctx.aroundAspects,
		beforeAspects: ctx.beforeAspects,
		afterAspects:  ctx.afterAspects,

		// Shared runtime state
		runSnapshot: ctx.runSnapshot,
		// Subcontexts share observers
		observer:        ctx.observer, // Shared observer instances
		err:             ctx.err,
		chainCache:      ctx.chainCache,      // Shared cache
		nodeOutputCache: ctx.nodeOutputCache, // Shared node output cache

		relationTypes: make([]string, 1),
		hasEndNode:    ctx.hasEndNode,
	}
	// Inherit debugModeOverride (1=ForceEnabled, -1=ForceClose, 0=Use node default values)
	if v := atomic.LoadInt32(&ctx.debugModeOverride); v != 0 {
		atomic.StoreInt32(&nextCtx.debugModeOverride, v)
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
		//Notify the current node to the common ancestor that this branch chain has completed execution.
		if ctx.parentRuleCtx != nil {
			ctx.parentRuleCtx.childDoneWithoutCallback()
		}
		return false
	} else {
		//Notify the current node to the common ancestor that this branch chain has completed execution.
		if ctx.parentRuleCtx != nil {
			ctx.parentRuleCtx.childDoneWithoutCallback()
		}
		lcaNodeId := ""
		if ctx.ruleChainCtx != nil && ctx.self != nil {
			// Obtain LCA nodes
			if lcaNode, ok := ctx.ruleChainCtx.GetLCA(ctx.self.GetNodeId()); ok {
				lcaNodeId = lcaNode.Id
			}
		}

		ctx.observer.registerNodeDoneEvent(selfNodeId, lcaNodeId, func(inMsgList []types.WrapperMsg) {
			callback(inMsgList)
		})
		// Note: Do not call executedNode(fromId)
		// LCA nodes should be marked as completed when the waitingCount zeros through the childDoneWithoutCallback process
		// Instead of marking it right away here. Otherwise, when the fork node connects directly to the join node,
		// Forks are marked as complete before all child nodes finish, causing join to trigger callbacks too early.
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
		// Capture the required values before submitting tasks, avoiding concurrent access
		logger := ctx.config.Logger
		if err := ctx.pool.Submit(task); err != nil {
			logger.Printf("SubmitTask error:%s, fallback to goroutine", err)
			// If the working pool fails to commit, it falls back to directly creating a goroutine
			// This ensures tasks are not lost and avoids deadlocks caused by counter mismatches
			go task()
		}
	} else {
		go task()
	}
}

// TellFlow executes subchain rules, ruleChainId, rule chainID
// The callback after the onEndFunc sub-rule chain branch is executed and returns the execution result of that chain. If multiple branch chains are triggered simultaneously, it will be called multiple times
// onAllNodeCompleted So the node is triggered after execution, but returns with no result
// If the rule chain cannot be found, the message is sent to the next node through a `Failure` relationship
func (ctx *DefaultRuleContext) TellFlow(ruleChainId string, msg types.RuleMsg, opts ...types.RuleContextOption) {
	if e, ok := ctx.GetRuleChainPool().Get(ruleChainId); ok {
		// Inheriting parent chain debugging mode: During parent chain debugging, the subchain automatically enters debugging, causing subchain nodes to also generate debug logs
		if ctx.IsDebugMode() {
			opts = append([]types.RuleContextOption{types.WithDebugMode(true)}, opts...)
		}
		e.OnMsg(msg, opts...)
	} else {
		ctx.TellFailure(msg, fmt.Errorf("ruleChain id=%s not found", ruleChainId))
	}
}

// TellNode starts executing from the specified node. If skipTellNext=true, only the current node is executed, and the next node is not notified.
// onEnd View to obtain the final execution result
// onAllNodeCompleted So the node is triggered after execution, but returns with no result
func (ctx *DefaultRuleContext) TellNode(chanCtx context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, onEnd types.OnEndFunc, onAllNodeCompleted func()) {
	startId := types.RuleNodeId{Id: nodeId}
	if nodeCtx, ok := ctx.ruleChainCtx.GetNodeById(startId); ok {
		rootCtxCopy := NewRuleContext(chanCtx, ctx.config, ctx.ruleChainCtx, nil, nodeCtx, ctx.pool, onEnd, ctx.ruleChainPool)
		rootCtxCopy.onAllNodeCompleted = onAllNodeCompleted
		//Whether to only execute the current node
		rootCtxCopy.skipTellNext = skipTellNext
		if skipTellNext {
			//If only one node is executed, there definitely is no termination node (it itself is termination)
			rootCtxCopy.hasEndNode = false
		} else if ctx.ruleChainCtx != nil {
			rootCtxCopy.hasEndNode = ctx.ruleChainCtx.HasEndDescendant(startId)
		}

		if ctx.GetNodeOutputCache() != nil {
			rootCtxCopy.nodeOutputCache = ctx.GetNodeOutputCache()
		}

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

// SetRuleChainPool sets up the sub-rule chain pool
func (ctx *DefaultRuleContext) SetRuleChainPool(ruleChainPool types.RuleEnginePool) {
	ctx.ruleChainPool = ruleChainPool
}

// GetRuleChainPool obtains the sub-rule chain pool
func (ctx *DefaultRuleContext) GetRuleChainPool() types.RuleEnginePool {
	if ctx.ruleChainPool == nil {
		return DefaultPool
	} else {
		return ctx.ruleChainPool
	}
}

// SetOnAllNodeCompleted sets the callback after all nodes have executed
func (ctx *DefaultRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
}

func (ctx *DefaultRuleContext) HasEndNode() bool {
	return ctx.hasEndNode
}

// DoOnEnd ends the execution of the rule chain branch, triggering the OnEnd callback function
func (ctx *DefaultRuleContext) DoOnEnd(msg types.RuleMsg, err error, relationType string) {
	configOnEnd := ctx.config.OnEnd
	contextOnEnd := ctx.onEnd

	needsCopy := configOnEnd != nil || contextOnEnd != nil

	var msgToUse types.RuleMsg
	if needsCopy {
		// Copy MSG
		msgToUse = msg.Copy()
		// Ensure the metadata is not nil, and avoid empty pointer exceptions
		if msgToUse.Metadata == nil {
			msgToUse.SetMetadata(types.NewMetadata())
		}
	} else {
		msgToUse = msg
	}

	// If a termination node is configured, only the termination node can trigger the callback; If no terminal node is configured, all nodes can be triggered
	isEndNode := ctx.self != nil && ctx.self.Type() == types.NodeTypeEnd
	if configOnEnd != nil || contextOnEnd != nil {
		if relationType == types.Stream {
			// Whether a callback has been triggered
			shouldTrigger := ctx.ruleChainCtx == nil || !ctx.HasEndNode() || isEndNode || (ctx.config.OnEndWithFailure && relationType == types.Failure)
			//A global pullback
			//Set it through `Config.OnEnd`
			if configOnEnd != nil && shouldTrigger {
				configOnEnd(ctx, msgToUse, err, relationType)
			}
			// types.withOnEnd settings
			if contextOnEnd != nil && shouldTrigger {
				contextOnEnd(ctx, msgToUse, err, relationType)
			}
			if msg.GetMetadata().Has(types.KeyStreamStart) {
				ctx.childDone()
			}
		} else {
			ctx.SubmitTask(func() {
				// Whether a callback has been triggered
				shouldTrigger := ctx.ruleChainCtx == nil || !ctx.HasEndNode() || isEndNode || (ctx.config.OnEndWithFailure && relationType == types.Failure)
				//A global pullback
				//Set it through `Config.OnEnd`
				if configOnEnd != nil && shouldTrigger {
					configOnEnd(ctx, msgToUse, err, relationType)
				}
				// types.withOnEnd settings
				if contextOnEnd != nil && shouldTrigger {
					contextOnEnd(ctx, msgToUse, err, relationType)
				}
				ctx.childDone()
			})
		}

	} else {
		ctx.childDone()
	}
	if isEndNode {
		// Execute AfterAop
		msg = ctx.executeAfterAop(msg, err, relationType)
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
	// Cache runSnapshot references at the start of the method to avoid concurrent race conditions
	runSnapshot := ctx.runSnapshot

	// Smart copy optimization: Messages are only copied when truly needed
	needsAsyncDebug := ctx.IsDebugMode() && ctx.config.OnDebug != nil
	needsSnapshotDebug := ctx.IsDebugMode() && runSnapshot != nil && runSnapshot.onDebugCustomFunc != nil
	needsSnapshot := runSnapshot != nil && runSnapshot.needCollectRunSnapshot()

	// Copies are only created when a copy is truly needed
	var msgCopy types.RuleMsg
	if needsAsyncDebug || needsSnapshotDebug || needsSnapshot {
		msgCopy = msg.Copy()
	}

	if ctx.IsDebugMode() {
		// Capture the required values before submitting asynchronous tasks, avoiding concurrent access
		onDebugFunc := ctx.config.OnDebug

		//Asynchronously log logs
		if needsAsyncDebug || needsSnapshotDebug {
			ctx.SubmitTask(func() {
				if onDebugFunc != nil {
					onDebugFunc(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
				}
				if runSnapshot != nil {
					runSnapshot.onDebugCustom(ruleChainId, flowType, nodeId, msgCopy, relationType, err)
				}
			})
		}
	}
	if runSnapshot != nil {
		//Record snapshots
		runSnapshot.collectRunSnapshot(ctx, flowType, nodeId, msgCopy, relationType, err)
	}

}

// SetExecuteNodes sets the execution node
// You can set up one or more nodes to resume execution or specify the starting node
func (ctx *DefaultRuleContext) SetExecuteNodes(nodes ...types.NodeRequest) {
	if len(nodes) == 1 {
		// Check whether it is a search pattern or includes relation type
		// If RelationTypes is not nil, it is considered to be Lookup Child Node mode
		// If RelationTypes is not nil, it is considered as finding child nodes mode.
		if nodes[0].RelationTypes == nil {
			nodeId := nodes[0].NodeId
			// Execute the current node mode
			ctx.isFirst = true
			ctx.relationTypes = nil
			if node, ok := ctx.ruleChainCtx.GetNodeById(types.RuleNodeId{Id: nodeId}); ok {
				ctx.self = node
			} else {
				ctx.err = fmt.Errorf("SetExecuteNodes node id=%s not found", nodeId)
			}
			// Empty restoreNodeInfo to ensure the TellNext path is used
			ctx.restoreNodeInfo = nil
			return
		}
	}

	ctx.restoreNodeInfo = &RestoreNodeInfo{
		NodeRequests: nodes,
	}
}

// GetRelationTypes retrieves the current input node execution relationship
func (ctx *DefaultRuleContext) GetRelationTypes() []string {
	return ctx.relationTypes
}

func (ctx *DefaultRuleContext) GetOut() types.RuleMsg {
	return ctx.out
}

func (ctx *DefaultRuleContext) GetErr() error {
	return ctx.err
}

// IsDebugMode checks whether the mode is being debugged. debugModeOverride: 1=On, -1=Off, 0=Use node default values
func (ctx *DefaultRuleContext) IsDebugMode() bool {
	v := atomic.LoadInt32(&ctx.debugModeOverride)
	if v == 1 {
		return true
	}
	if v == -1 {
		return false
	}
	return ctx.Self() != nil && ctx.Self().IsDebugMode()
}

// SetDebugMode sets the debug mode override for per-message
func (ctx *DefaultRuleContext) SetDebugMode(debugMode bool) {
	if debugMode {
		atomic.StoreInt32(&ctx.debugModeOverride, 1)
	} else {
		atomic.StoreInt32(&ctx.debugModeOverride, -1)
	}
}

// SetSkipTellNext sets whether to skip propagating to successor nodes.
func (ctx *DefaultRuleContext) SetSkipTellNext(skip bool) {
	ctx.skipTellNext = skip
	if skip {
		ctx.hasEndNode = false
	}
}

// Add a child node to be executed
func (ctx *DefaultRuleContext) childReady(msg types.RuleMsg, relationType string) {
	if relationType != types.Stream || (relationType == types.Stream && msg.GetMetadata().Has(types.KeyStreamStart)) {
		atomic.AddInt32(&ctx.waitingCount, 1)
	}
}

// Reduces one pending child node
// If the return count is 0, it means the branch chain has completed execution and returns to the parent node until all nodes have processed it, triggering the onAllNodeCompleted event.
func (ctx *DefaultRuleContext) childDone() {
	if atomic.AddInt32(&ctx.waitingCount, -1) <= 0 {
		if atomic.CompareAndSwapInt32(&ctx.onAllNodeCompletedDone, 0, 1) {
			// Capture the required values before any asynchronous operation to avoid concurrency issues
			parentRuleCtx := ctx.parentRuleCtx
			selfId := ctx.GetSelfId()
			var parentSelfId string
			if parentRuleCtx != nil {
				parentSelfId = parentRuleCtx.GetSelfId()
			}
			observer := ctx.observer
			onAllNodeCompleted := ctx.onAllNodeCompleted

			//The node has completed execution, notify the parent node
			if parentRuleCtx != nil {
				parentRuleCtx.childDone()
			}

			// Node execution completion is only recorded when the observer exists (usually in the join node scenario)
			if observer != nil && (parentRuleCtx == nil || selfId != parentSelfId) {
				//Records the completion of execution on the current node
				observer.executedNode(selfId)
			}
			//The pullback was completed
			if onAllNodeCompleted != nil {
				onAllNodeCompleted()
			}
		}
	}
}

// childDoneWithoutCallback notifies the parent node that a child node has completed execution, but does not trigger the onAllNodeCompleted callback event
//
// Differences from the childDone() method:
// 1. childDone(): When a child node finishes execution, the onAllNodeCompleted callback is triggered, suitable for normal node completion scenarios
// 2. childDoneWithoutCallback(): No callback is triggered upon child node execution, dedicated for aggregating data from multiple branch chains
//
// Usage scenarios:
// - Used together with the TellCollect method, querying whether the common ancestor of the parent node of the node has completed all branches up to the current aggregated node
// - Status tracking for multi-branch aggregation scenarios, avoiding premature triggering of completion events
func (ctx *DefaultRuleContext) childDoneWithoutCallback() {
	if atomic.AddInt32(&ctx.waitingCount, -1) <= 0 {
		//if atomic.CompareAndSwapInt32(&ctx.onAllNodeCompletedDone, 0, 1) {

		// Capture the required values before any asynchronous operation to avoid concurrency issues
		parentRuleCtx := ctx.parentRuleCtx
		selfId := ctx.GetSelfId()
		var parentSelfId string
		if parentRuleCtx != nil {
			parentSelfId = parentRuleCtx.GetSelfId()
		}
		observer := ctx.observer

		//When a child node completes execution, notify the parent node
		if parentRuleCtx != nil {
			parentRuleCtx.childDoneWithoutCallback()
		}

		// Node execution completion is only recorded when the observer exists (usually in the join node scenario)
		if observer != nil && (parentRuleCtx == nil || selfId != parentSelfId) {
			//Records the completion of execution on the current node
			observer.executedNode(selfId)
		}
	}
}

// getNextNodes gets the child node of the current node's specified relationship
func (ctx *DefaultRuleContext) getNextNodes(relationType string) ([]types.NodeCtx, bool) {
	if ctx.ruleChainCtx == nil || ctx.self == nil {
		return nil, false
	}
	return ctx.ruleChainCtx.GetNextNodes(ctx.self.GetNodeId(), relationType)
}

// tellSelf executes its own node
func (ctx *DefaultRuleContext) tellSelf(msg types.RuleMsg, err error, relationTypes ...string) {
	var relationType string
	if len(relationTypes) > 0 {
		relationType = relationTypes[0]
	}
	if ctx.self != nil {
		// Asynchronous execution requires copying to ensure thread safety
		// Note: You cannot simply optimize based on node type, as other concurrent branches may modify messages
		msgCopy := msg.Copy()
		if relationType == types.Stream {
			ctx.tellNext(msgCopy, ctx.self, relationType)
		} else {
			ctx.SubmitTask(func() {
				ctx.tellNext(msgCopy, ctx.self, relationType)
			})
		}
	} else {
		ctx.DoOnEnd(msg, err, relationType)
	}
}

// tellNext notifies the executing child node; if it is the first node currently, it executes the current node
func (ctx *DefaultRuleContext) tell(msg types.RuleMsg, err error, relationTypes ...string) {
	ctx.tellOrElse(msg, err, "", relationTypes...)
}

// tellNext notifies the executing child node; if it is the first node currently, it executes the current node
// If the node corresponding to relationTypes cannot be found and defaultRelationType is not a default value, use defaultRelation Type to find the node
func (ctx *DefaultRuleContext) tellOrElse(msg types.RuleMsg, err error, defaultRelationType string, relationTypes ...string) {
	ctx.out = msg
	ctx.err = err
	if ctx.isFirst {
		ctx.tellSelf(msg, err, relationTypes...)
	} else {
		if relationTypes == nil {
			//If no child node is found, the execution ends, and the callback ends
			ctx.DoOnEnd(msg, err, "")
		} else {

			relationTypeLen := len(relationTypes)

			for _, relationType := range relationTypes {
				//Create local replicas to avoid data contention caused by closure capture loop variables
				rt := relationType
				//Execute After Aop
				msg = ctx.executeAfterAop(msg, err, rt)
				var ok = false
				var nodes []types.NodeCtx
				//Find the list of child nodes based on relationType
				nodes, ok = ctx.getNextNodes(rt)
				//Find nodes based on default relationships
				if defaultRelationType != "" && (!ok || len(nodes) == 0) && !ctx.skipTellNext {
					nodes, ok = ctx.getNextNodes(defaultRelationType)
				}
				if ok && !ctx.skipTellNext {
					// Copying is only needed when there are multiple child nodes or multiple relationships in parallel
					needsCopy := len(nodes) > 1 || relationTypeLen > 0
					for _, item := range nodes {
						tmp := item
						//Add a child node to be executed
						ctx.childReady(msg, rt)

						var msgToPass types.RuleMsg
						if needsCopy {
							//Except for one node and multiple parallel relationships, all nodes create copies
							msgToPass = msg.Copy()
						} else {
							//The unique node can directly use the original message
							msgToPass = msg
						}

						//Notify the execution child node
						if rt == types.Stream {
							//To ensure the order of the flow blocks,
							ctx.tellNext(msgToPass, tmp, rt)
						} else {
							ctx.SubmitTask(func() {
								ctx.tellNext(msgToPass, tmp, rt)
							})
						}
					}
				} else {
					//Calling DoOnEnd will reduce childDone() by 1 to the waitingCount, so childReady and childDone appear as a pair
					ctx.childReady(msg, relationType)
					//If no child node is found, the execution ends, and the callback ends
					ctx.DoOnEnd(msg, err, relationType)
				}
			}
		}
	}
}

// Perform Surround AOP
// Return true: Continue executing the next node; otherwise, it will not be executed
func (ctx *DefaultRuleContext) executeAroundAop(msg types.RuleMsg, relationType string) bool {
	// before aop
	for _, aop := range ctx.beforeAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.Before(ctx, msg, relationType)
		}
	}

	tellNext := true
	//Has tellNext logic already been executed?
	//If AroundAspect has already executed tellNext logic, the engine will no longer execute tellNext logic
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

// Execute After Aop
func (ctx *DefaultRuleContext) executeAfterAop(msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	// after aop
	for _, aop := range ctx.afterAspects {
		if aop.PointCut(ctx, msg, relationType) {
			msg = aop.After(ctx, msg, err, relationType)
		}
	}
	return msg
}

// Execute the next node
func (ctx *DefaultRuleContext) tellNext(msg types.RuleMsg, nextNode types.NodeCtx, relationType string) {

	defer func() {
		//Capture anomalies
		if e := recover(); e != nil {
			//Execute After Aop
			msg = ctx.executeAfterAop(msg, fmt.Errorf("%v", e), relationType)
			ctx.childDone()
		}
	}()

	// Unified context check whether it has been canceled (elegant shutdown)
	// Unified check for context cancellation (graceful shutdown)
	if ctx.GetContext() != nil {
		select {
		case <-ctx.GetContext().Done():
			// The context has been canceled, processing stopped, and a failure was notified
			// Context cancelled, stop processing and notify failure
			// Use DoOnEnd to ensure proper triggering of end callbacks and reduction in active message counts
			// DoOnEnd internally calls childDone(), so there is no need to call it again here
			ctx.DoOnEnd(msg, fmt.Errorf("processing cancelled: %w", ctx.GetContext().Err()), types.Failure)
			return
		default:
			// The context is normal, so continue processing
			// Context is normal, continue processing
		}
	}

	// Before executing the next node, the output of the current node is stored in the cache
	// Store current node output to cache before executing next node
	ctx.StoreNodeOutput(ctx.GetSelfId(), msg)

	nextCtx := ctx.NewNextNodeRuleContext(nextNode)

	//Surrounding AOP
	if !nextCtx.executeAroundAop(msg, relationType) {
		// If AroundAspect blocks execution, childDone needs to be called to balance the previous childReady
		ctx.childDone()
		return
	}
	// AroundAop has already executed node OnMsg logic and is not executing the following logic
	ctx.setRelationType(nextCtx, relationType)
	nextNode.OnMsg(nextCtx, msg)
}

// setRelationType optimizes the assignment of the relationship type using predefined singletons or reusing allocated slice
// setRelationType optimizes relation type assignment using predefined singletons or reusing allocated slices
func (ctx *DefaultRuleContext) setRelationType(nextCtx *DefaultRuleContext, relationType string) {
	// For common relationship types, use predefined singleton slice to avoid memory allocation
	// For common relation types, use predefined singleton slices to avoid memory allocation
	switch relationType {
	case types.Success:
		nextCtx.relationTypes = successRelationTypes
	case types.Failure:
		nextCtx.relationTypes = failureRelationTypes
	case types.True:
		nextCtx.relationTypes = trueRelationTypes
	case types.False:
		nextCtx.relationTypes = falseRelationTypes
	default:
		// For custom relationship types, reuse the allocated slice
		// For custom relation types, reuse the pre-allocated slice
		nextCtx.relationTypes[0] = relationType
	}
}

// GetNodeRuleMsg retrieves the complete RuleMsg of a specific executed node by nodeId
// IMPORTANT: Node dependency must be established beforehand to successfully retrieve data
//
// Dependency establishment methods:
// 1. Using FetchNodeOutputNode component (automatic)
// 2. Manually calling chainCtx.AddNodeDependency(currentNodeId, targetNodeId)
// 3. Node configuration contains references to other nodes. e.g. ${nodeId.msg.xx} (auto-detected)
func (ctx *DefaultRuleContext) GetNodeRuleMsg(nodeId string) (types.RuleMsg, bool) {
	// Retrieves the target node's RuleMsg from the node output cache
	// Only node outputs with established dependencies will be cached
	// Retrieve target node's RuleMsg from node output cache
	// Only outputs from nodes with established dependencies are cached
	if ruleMsg, ok := ctx.nodeOutputCache.GetNodeRuleMsg(nodeId); ok {
		return ruleMsg, true
	}
	// Target node output not found, possible reasons:
	// 1. Node not yet executed
	// 2. Dependency not established
	// 3. Node execution failed
	return types.RuleMsg{}, false
}

// StoreNodeOutput: Store nodes and output them to the cache, used to retrieve values across nodes
// Caching is only performed under the following circumstances:
// 1. Node Output Cache is enabled in the configuration (EnableNodeOutputCache = true)
// 2. Or cross-node value retrieval usage has been detected (enabled via EnableCrossNodeAccess())
// Parameters:
//   - nodeId: Node ID
//   - msg: Rule message
func (ctx *DefaultRuleContext) StoreNodeOutput(nodeId string, msg types.RuleMsg) {
	ctx.nodeOutputCache.StoreNodeOutput(nodeId, msg)
}
