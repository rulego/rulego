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
	"fmt"
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/cache"
)

var _ types.RuleContext = (*NodeTestRuleContext)(nil)

// NodeTestRuleContext
// A context created temporarily for testing single nodes
// Multiple nodes cannot be combined into a chain
// Callback processing results
type NodeTestRuleContext struct {
	context  context.Context
	config   types.Config
	callback func(msg types.RuleMsg, relationType string, err error)
	self     types.Node
	selfId   string
	//All child nodes handle the completion event and execute it only once
	onAllNodeCompleted func()
	onEndFunc          types.OnEndFunc
	childrenNodes      sync.Map
	out                types.RuleMsg
	globalCache        types.Cache
	chainCache         types.Cache
	mutex              sync.RWMutex // Add mutex for thread safety
}

func (ctx *NodeTestRuleContext) GlobalCache() types.Cache {
	return ctx.globalCache
}

func (ctx *NodeTestRuleContext) ChainCache() types.Cache {
	return ctx.chainCache
}

func NewRuleContext(config types.Config, callback func(msg types.RuleMsg, relationType string, err error)) types.RuleContext {
	globalCache := cache.NewMemoryCache(time.Minute * 5)
	return &NodeTestRuleContext{
		context:     context.TODO(),
		config:      config,
		callback:    callback,
		globalCache: globalCache,
		chainCache:  cache.NewNamespaceCache(globalCache, "test"),
	}
}

func NewRuleContextFull(config types.Config, self types.Node, childrenNodes map[string]types.Node, callback func(msg types.RuleMsg, relationType string, err error)) types.RuleContext {
	ctx := &NodeTestRuleContext{
		config:      config,
		self:        self,
		callback:    callback,
		context:     context.TODO(),
		globalCache: config.Cache,
		chainCache:  cache.NewNamespaceCache(config.Cache, "test"),
	}
	for k, v := range childrenNodes {
		ctx.childrenNodes.Store(k, v)
	}
	return ctx
}

func (ctx *NodeTestRuleContext) TellSuccess(msg types.RuleMsg) {
	ctx.mutex.RLock()
	callback := ctx.callback
	onEndFunc := ctx.onEndFunc
	ctx.mutex.RUnlock()

	if callback != nil {
		callback(msg, types.Success, nil)
	}
	if onEndFunc != nil {
		onEndFunc(ctx, msg, nil, types.Success)
	}
}

func (ctx *NodeTestRuleContext) TellFailure(msg types.RuleMsg, err error) {
	ctx.mutex.RLock()
	callback := ctx.callback
	onEndFunc := ctx.onEndFunc
	ctx.mutex.RUnlock()

	if callback != nil {
		callback(msg, types.Failure, err)
	}
	if onEndFunc != nil {
		onEndFunc(ctx, msg, err, types.Failure)
	}
}

func (ctx *NodeTestRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	ctx.mutex.RLock()
	callback := ctx.callback
	onEndFunc := ctx.onEndFunc
	ctx.mutex.RUnlock()

	for _, relationType := range relationTypes {
		if callback != nil {
			callback(msg, relationType, nil)
		}
		if onEndFunc != nil {
			onEndFunc(ctx, msg, nil, relationType)
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
func (ctx *NodeTestRuleContext) TellNextOrElse(msg types.RuleMsg, defaultRelationType string, relationTypes ...string) {
	ctx.TellNext(msg, relationTypes...)
}
func (ctx *NodeTestRuleContext) NewMsg(msgType string, metaData *types.Metadata, data string) types.RuleMsg {
	return types.NewMsg(0, msgType, types.JSON, metaData, data)
}
func (ctx *NodeTestRuleContext) GetSelfId() string {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
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
	ctx.SubmitTask(task)
}
func (ctx *NodeTestRuleContext) SubmitTask(task func()) {
	go task()
}

func (ctx *NodeTestRuleContext) SetEndFunc(onEndFunc types.OnEndFunc) types.RuleContext {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	ctx.onEndFunc = onEndFunc
	return ctx
}

func (ctx *NodeTestRuleContext) GetEndFunc() types.OnEndFunc {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	return ctx.onEndFunc
}

func (ctx *NodeTestRuleContext) SetContext(c context.Context) types.RuleContext {
	ctx.context = c
	return ctx
}

func (ctx *NodeTestRuleContext) GetContext() context.Context {
	return ctx.context
}

func (ctx *NodeTestRuleContext) TellFlow(chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) {
	for _, opt := range opts {
		opt(ctx)
	}
	if chainId == "" {
		if ctx.onEndFunc != nil {
			ctx.onEndFunc(ctx, msg, errors.New("chainId can not nil"), types.Failure)
		}

	} else if chainId == "notfound" {
		if ctx.onEndFunc != nil {
			ctx.onEndFunc(ctx, msg, fmt.Errorf("ruleChain id=%s not found", chainId), types.Failure)
		}
		if ctx.onAllNodeCompleted != nil {
			ctx.onAllNodeCompleted()
		}
	} else if chainId == "toTrue" {
		if ctx.onEndFunc != nil {
			ctx.onEndFunc(ctx, msg, nil, types.True)
		}
		if ctx.onAllNodeCompleted != nil {
			ctx.onAllNodeCompleted()
		}
	} else {
		if ctx.onEndFunc != nil {
			ctx.onEndFunc(ctx, msg, nil, types.Success)
		}
		if ctx.onAllNodeCompleted != nil {
			ctx.onAllNodeCompleted()
		}
	}
}

// TellNode independently executes a specific node and obtains its execution status through callback. It is used for grouping nodes to control the execution of a specific node
func (ctx *NodeTestRuleContext) TellNode(context context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, callback types.OnEndFunc, onAllNodeCompleted func()) {
	if v, ok := ctx.childrenNodes.Load(nodeId); ok {
		// Threads safely set selfId
		ctx.mutex.Lock()
		ctx.selfId = nodeId
		ctx.mutex.Unlock()

		subCtx := NewRuleContext(ctx.config, func(msg types.RuleMsg, relationType string, err error) {
			if callback != nil {
				callback(ctx, msg, err, relationType)
			}

			if onAllNodeCompleted != nil {
				onAllNodeCompleted()
			}
		})

		v.(types.Node).OnMsg(subCtx, msg)
	} else {
		if callback != nil {
			callback(ctx, msg, fmt.Errorf("node id=%s not found", nodeId), types.Failure)
		}
		if onAllNodeCompleted != nil {
			onAllNodeCompleted()
		}
	}
}

// TellChainNode independently executes a node and obtains the node's execution status through callback. It is used for grouping nodes to control and execute a specific node
func (ctx *NodeTestRuleContext) TellChainNode(context context.Context, chainId string, nodeId string, msg types.RuleMsg, skipTellNext bool, callback types.OnEndFunc, onAllNodeCompleted func()) {
	ctx.TellNode(context, nodeId, msg, skipTellNext, callback, onAllNodeCompleted)
}

// SetOnAllNodeCompleted sets the callback after all nodes have executed
func (ctx *NodeTestRuleContext) SetOnAllNodeCompleted(onAllNodeCompleted func()) {
	ctx.onAllNodeCompleted = onAllNodeCompleted
}

func (ctx *NodeTestRuleContext) DoOnEnd(msg types.RuleMsg, err error, relationType string) {

}

// SetCallbackFunc sets the callback function
func (ctx *NodeTestRuleContext) SetCallbackFunc(functionName string, f interface{}) {

}

// GetCallbackFunc gets the callback function
func (ctx *NodeTestRuleContext) GetCallbackFunc(functionName string) interface{} {
	return nil
}

// OnDebug calls the configured OnDebug callback function
func (ctx *NodeTestRuleContext) OnDebug(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
}

func (ctx *NodeTestRuleContext) SetExecuteNodes(nodes ...types.NodeRequest) {

}

func (ctx *NodeTestRuleContext) TellCollect(msg types.RuleMsg, callback func(msgList []types.WrapperMsg)) bool {
	callback(nil)
	return true
}

func (ctx *NodeTestRuleContext) GetOut() types.RuleMsg {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	return ctx.out
}

func (ctx *NodeTestRuleContext) GetRelationTypes() []string {
	return nil
}

// setOut safely sets the out field
func (ctx *NodeTestRuleContext) setOut(msg types.RuleMsg) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	ctx.out = msg
}

func (ctx *NodeTestRuleContext) GetErr() error {
	return nil
}

func (ctx *NodeTestRuleContext) TellStream(msg types.RuleMsg) {
	ctx.TellNext(msg, types.Stream)
}

// GetEnv retrieves environment variables and metadata
func (ctx *NodeTestRuleContext) GetEnv(msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	// Create the environment variable map
	envVars := make(map[string]interface{})

	// Set the base environment variable
	envVars["id"] = msg.GetId()
	envVars["ts"] = msg.GetTs()
	envVars["data"] = msg.GetData()
	envVars["msgType"] = msg.GetType()
	envVars["type"] = msg.GetType()
	envVars["dataType"] = string(msg.GetDataType())
	// Use GetJsonData() to avoid repeated JSON parsing
	if msg.DataType == types.JSON {
		if jsonData, err := msg.GetJsonData(); err == nil {
			envVars[types.MsgKey] = jsonData
		} else {
			// Parsing fails, using raw data
			envVars[types.MsgKey] = msg.GetData()
		}
	} else {
		// If it is not a JSON type, use the raw data directly
		envVars[types.MsgKey] = msg.GetData()
	}
	// Optimized metadata processing
	if msg.Metadata != nil {
		if useMetadata {
			// Traverse the metadata and add key-value pairs to environment variables - use zero-copy ForEach
			msg.Metadata.ForEach(func(k, v string) bool {
				envVars[k] = v
				return true // continue iteration
			})
		}
		envVars[types.MetadataKey] = msg.Metadata.Values()
	}

	return envVars
}

// GetNodeRuleMsg retrieves the complete message information of the node (cross-node value selection is not currently supported in the test context)
// GetNodeRuleMsg retrieves the complete RuleMsg of a node (not supported in test context)
func (ctx *NodeTestRuleContext) GetNodeRuleMsg(nodeId string) (types.RuleMsg, bool) {
	return types.RuleMsg{}, false
}

func (ctx *NodeTestRuleContext) SetDebugMode(debugMode bool) {}

func (ctx *NodeTestRuleContext) SetSkipTellNext(skip bool) {}

// ExtendedTestRuleContext extends the test context to support result collection and node processor settings
// Can replace SimpleTestContext and MockRuleContext
type ExtendedTestRuleContext struct {
	*NodeTestRuleContext
	nodeHandlers map[string]func(msg types.RuleMsg) (string, error)
	results      []string
	resultsChan  chan TestResult
	handlerMutex sync.RWMutex
}

// TestResult test result structure
type TestResult struct {
	RelationType string
	Err          error
}

// NewExtendedTestRuleContext creates an extended test context
// Used to replace SimpleTestContext and MockRuleContext
func NewExtendedTestRuleContext(config types.Config, callback func(msg types.RuleMsg, relationType string, err error)) *ExtendedTestRuleContext {
	baseCtx := NewRuleContext(config, callback).(*NodeTestRuleContext)
	return &ExtendedTestRuleContext{
		NodeTestRuleContext: baseCtx,
		nodeHandlers:        make(map[string]func(msg types.RuleMsg) (string, error)),
		results:             make([]string, 0),
		resultsChan:         make(chan TestResult, 10),
	}
}

// NewExtendedTestRuleContextWithChannel creates an extended test context with a result channel
// Mainly used to replace SimpleTestContext
func NewExtendedTestRuleContextWithChannel() *ExtendedTestRuleContext {
	config := types.NewConfig()
	baseCtx := NewRuleContext(config, nil).(*NodeTestRuleContext)
	return &ExtendedTestRuleContext{
		NodeTestRuleContext: baseCtx,
		nodeHandlers:        make(map[string]func(msg types.RuleMsg) (string, error)),
		results:             make([]string, 0),
		resultsChan:         make(chan TestResult, 10),
	}
}

// SetNodeHandler sets up the node processor and is used to simulate node behavior
// SetNodeHandler method that replaces MockRuleContext
func (ctx *ExtendedTestRuleContext) SetNodeHandler(nodeId string, handler func(msg types.RuleMsg) (string, error)) {
	ctx.handlerMutex.Lock()
	defer ctx.handlerMutex.Unlock()
	ctx.nodeHandlers[nodeId] = handler
}

// GetResults retrieves the collected results
// Replace MockRuleContext with the GetResults method
func (ctx *ExtendedTestRuleContext) GetResults() []string {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	results := make([]string, len(ctx.results))
	copy(results, ctx.results)
	return results
}

// GetResultsChannel to obtain the results channel
// The results channel used to replace SimpleTestContext
func (ctx *ExtendedTestRuleContext) GetResultsChannel() <-chan TestResult {
	return ctx.resultsChan
}

// TellNode overrides the TellNode method to support node processors
func (ctx *ExtendedTestRuleContext) TellNode(context context.Context, nodeId string, msg types.RuleMsg, skipTellNext bool, callback types.OnEndFunc, onAllNodeCompleted func()) {
	ctx.handlerMutex.RLock()
	handler, hasHandler := ctx.nodeHandlers[nodeId]
	ctx.handlerMutex.RUnlock()

	if hasHandler {
		// Using custom processors (simulating node behavior)
		go func() {
			relationType, err := handler(msg)
			if callback != nil {
				callback(ctx, msg, err, relationType)
			}
			if onAllNodeCompleted != nil {
				onAllNodeCompleted()
			}
		}()
	} else {
		// Use the original TellNode logic
		ctx.NodeTestRuleContext.TellNode(context, nodeId, msg, skipTellNext, callback, onAllNodeCompleted)
	}
}

// TellNext rewrote to support results collection
func (ctx *ExtendedTestRuleContext) TellNext(msg types.RuleMsg, relationTypes ...string) {
	// Call the original logic
	ctx.NodeTestRuleContext.TellNext(msg, relationTypes...)

	// Collect the results
	if len(relationTypes) > 0 {
		ctx.mutex.Lock()
		ctx.results = append(ctx.results, relationTypes[0])
		ctx.mutex.Unlock()

		// Send to the results channel
		select {
		case ctx.resultsChan <- TestResult{RelationType: relationTypes[0], Err: nil}:
		default:
		}
	}
}

// TellSuccess rewrote to support result collection
func (ctx *ExtendedTestRuleContext) TellSuccess(msg types.RuleMsg) {
	// Call the original logic
	ctx.NodeTestRuleContext.TellSuccess(msg)

	// Collect the results
	ctx.mutex.Lock()
	ctx.results = append(ctx.results, "Success")
	ctx.mutex.Unlock()

	// Send to the results channel
	select {
	case ctx.resultsChan <- TestResult{RelationType: "Success", Err: nil}:
	default:
	}
}

// TellFailure overrides to support result collection
func (ctx *ExtendedTestRuleContext) TellFailure(msg types.RuleMsg, err error) {
	// Call the original logic
	ctx.NodeTestRuleContext.TellFailure(msg, err)

	// Collect the results
	ctx.mutex.Lock()
	ctx.results = append(ctx.results, "Failure")
	ctx.mutex.Unlock()

	// Send to the results channel
	select {
	case ctx.resultsChan <- TestResult{RelationType: "Failure", Err: err}:
	default:
	}
}
