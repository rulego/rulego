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
	"errors"
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/rulego/rulego/utils/cache"

	"github.com/rulego/rulego/api/types/metrics"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/aspect"
	"github.com/rulego/rulego/builtin/funcs"
)

// Ensuring RuleEngine implements types.RuleEngine interface.
var _ types.RuleEngine = (*RuleEngine)(nil)

var ErrDisabled = errors.New("the rule chain has been disabled")

// BuiltinsAspects holds a list of built-in aspects for the rule engine.
var BuiltinsAspects = []types.Aspect{&aspect.Validator{}, &aspect.Debug{}, &aspect.MetricsAspect{}, &aspect.RunSnapshotAspect{}}

// aspectsHolder holds the aspects for atomic access
type aspectsHolder struct {
	startAspects     []types.StartAspect
	endAspects       []types.EndAspect
	completedAspects []types.CompletedAspect
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
	// aspectsPtr provides high-performance atomic access to aspects
	aspectsPtr unsafe.Pointer
	// initialized indicates whether the rule engine has been initialized.
	initialized bool
	// Aspects is a list of AOP (Aspect-Oriented Programming) aspects.
	Aspects   types.AspectList
	OnUpdated func(chainId, nodeId string, dsl []byte)
}

// NewRuleEngine creates a new RuleEngine instance with the given ID and definition.
// It applies the provided RuleEngineOptions during the creation process.
func NewRuleEngine(id string, def []byte, opts ...types.RuleEngineOption) (*RuleEngine, error) {
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
	e.Aspects = types.NewAspectList(aspects)
}

func (e *RuleEngine) SetRuleEnginePool(ruleChainPool types.RuleEnginePool) {
	e.ruleChainPool = ruleChainPool
	if e.rootRuleChainCtx != nil {
		e.rootRuleChainCtx.SetRuleEnginePool(ruleChainPool)
	}
}

func (e *RuleEngine) GetAspects() types.AspectList {
	// 返回一个副本以避免数据竞争
	if e.rootRuleChainCtx != nil {
		return e.rootRuleChainCtx.GetAspects()
	}
	return e.Aspects
}

func (e *RuleEngine) Reload(opts ...types.RuleEngineOption) error {
	return e.ReloadSelf(e.DSL(), opts...)
}

func (e *RuleEngine) initBuiltinsAspects() {
	var newAspectsSlice []types.Aspect
	//初始化内置切面
	if e.Aspects.Len() == 0 {
		for _, builtinsAspect := range BuiltinsAspects {
			newAspectsSlice = append(newAspectsSlice, builtinsAspect.New())
		}
	} else {
		for _, item := range e.Aspects.Aspects() {
			newAspectsSlice = append(newAspectsSlice, item.New())
		}

		for _, builtinsAspect := range BuiltinsAspects {
			found := false
			for _, item := range newAspectsSlice {
				//判断是否是相同类型
				if reflect.TypeOf(item) == reflect.TypeOf(builtinsAspect) {
					found = true
					break
				}
			}
			if !found {
				newAspectsSlice = append(newAspectsSlice, builtinsAspect.New())
			}
		}
	}
	e.Aspects = types.NewAspectList(newAspectsSlice)
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
		if e.Aspects.Len() == 0 {
			e.initBuiltinsAspects()
		}
		e.rootRuleChainCtx.config = e.Config
		e.rootRuleChainCtx.SetAspects(e.Aspects)
		//更新规则链
		err = e.rootRuleChainCtx.ReloadSelf(dsl)
		//设置子规则链池
		e.rootRuleChainCtx.SetRuleEnginePool(e.ruleChainPool)
		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, e.id, dsl)
		}
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
	holder := &aspectsHolder{startAspects: startAspects, endAspects: endAspects, completedAspects: completedAspects}
	atomic.StorePointer(&e.aspectsPtr, unsafe.Pointer(holder))
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
		err := e.rootRuleChainCtx.ReloadChild(types.RuleNodeId{Id: ruleNodeId}, dsl)
		if err == nil && e.OnUpdated != nil {
			e.OnUpdated(e.id, ruleNodeId, e.DSL())
		}
		return err
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
	// 清理实例缓存
	if e.Config.Cache != nil && e.rootRuleChainCtx != nil {
		_ = e.Config.Cache.DeleteByPrefix(e.rootRuleChainCtx.GetNodeId().Id + types.NamespaceSeparator)
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
	for _, aop := range e.Aspects.Aspects() {
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
// It executes aspects and triggers any custom callback functions.
func (e *RuleEngine) doOnAllNodeCompleted(rootCtxCopy *DefaultRuleContext, msg types.RuleMsg, customFunc func()) {
	// Execute aspects upon completion of all nodes.
	e.onAllNodeCompleted(rootCtxCopy, msg)

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
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.startAspects {
			if aop.PointCut(ctx, msg, "") {
				if err != nil {
					return msg, err
				}
				msg, err = aop.Start(ctx, msg)
			}
		}
	}
	return msg, err
}

// onEnd executes the list of end aspects when a branch of the rule chain ends.
func (e *RuleEngine) onEnd(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) types.RuleMsg {
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.endAspects {
			if aop.PointCut(ctx, msg, relationType) {
				msg = aop.End(ctx, msg, err, relationType)
			}
		}
	}
	return msg
}

// onAllNodeCompleted executes the list of completed aspects after all branches of the rule chain have ended.
func (e *RuleEngine) onAllNodeCompleted(ctx types.RuleContext, msg types.RuleMsg) types.RuleMsg {
	if aspects := e.getAspectsHolder(); aspects != nil {
		for _, aop := range aspects.completedAspects {
			if aop.PointCut(ctx, msg, "") {
				msg = aop.Completed(ctx, msg)
			}
		}
	}
	return msg
}

// getAspectsHolder safely retrieves the aspects holder with high performance
func (e *RuleEngine) getAspectsHolder() *aspectsHolder {
	ptr := atomic.LoadPointer(&e.aspectsPtr)
	if ptr == nil {
		return nil
	}
	return (*aspectsHolder)(ptr)
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
	if c.Cache == nil {
		c.Cache = cache.DefaultCache
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
