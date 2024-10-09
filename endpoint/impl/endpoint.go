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

// Package impl provides the implementation of the endpoint module.
// It includes structures and methods for handling endpoints, routers,
// and message processing in the RuleGo framework.
//
// This package contains the core components for building and managing
// endpoints, which are used to receive and process incoming data from
// various sources (e.g., HTTP, MQTT, Kafka) before passing it to the
// rule engine for further processing.
//
// Key components in this package include:
// - From: Represents the source of incoming data
// - To: Represents the destination for processed data
// - Router: Manages the routing of messages between From and To
// - DynamicEndpoint: Implements a configurable and updatable endpoint
//
// The implementation in this package allows for flexible configuration
// and dynamic updates of endpoints, enabling RuleGo to adapt to various
// input sources and protocols while maintaining a consistent interface
// for data processing and rule execution.
package impl

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/str"
)

const (
	pathKey = "_path"
	//分割标志如：{chainId}#{nodeId}
	pathSplitFlag = ":"
)

var _ endpoint.From = (*From)(nil)

// From from端
type From struct {
	//Config 配置
	Config types.Configuration
	//Router router指针
	Router *Router
	//来源路径
	From string
	//消息处理拦截器
	processList []endpoint.Process
	//流转目标路径，例如"chain:{chainId}"，则是交给规则引擎处理数据
	to *To
}

func (f *From) ToString() string {
	return f.From
}

// Transform from端转换msg
func (f *From) Transform(transform endpoint.Process) endpoint.From {
	f.processList = append(f.processList, transform)
	return f
}

// Process from端处理msg
func (f *From) Process(process endpoint.Process) endpoint.From {
	f.processList = append(f.processList, process)
	return f
}

// GetProcessList 获取from端处理器列表
func (f *From) GetProcessList() []endpoint.Process {
	return f.processList
}

// ExecuteProcess 执行处理函数
// true:执行To端逻辑，否则不执行
func (f *From) ExecuteProcess(router endpoint.Router, exchange *endpoint.Exchange) bool {
	result := true
	for _, process := range f.GetProcessList() {
		if !process(router, exchange) {
			result = false
			break
		}
	}
	return result
}

// To To端
// 参数是组件路径，格式{executorType}:{path} executorType：执行器组件类型，path:组件路径
// 如：chain:{chainId} 执行rulego中注册的规则链
// chain:{chainId}#{nodeId} 执行rulego中注册的规则链，并指定开始节点
// component:{nodeType} 执行在config.ComponentsRegistry 中注册的组件
// 可在DefaultExecutorFactory中注册自定义执行器组件类型
// componentConfigs 组件配置参数
func (f *From) To(to string, configs ...types.Configuration) endpoint.To {
	var toConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			toConfig[k] = v
		}
	}
	f.to = &To{Router: f.Router, To: to, Config: toConfig}
	//路径中是否有变量，如：chain:${userId}
	if strings.Contains(to, "${") && strings.Contains(to, "}") {
		f.to.HasVars = true
	}
	//获取To执行器类型
	executorType := strings.Split(to, pathSplitFlag)[0]

	//获取To执行器
	if executor, ok := DefaultExecutorFactory.New(executorType); ok {
		if f.to.HasVars && !executor.IsPathSupportVar() {
			panic(fmt.Errorf("executor=%s, path not support variables", executorType))
		}
		f.to.ToPath = strings.TrimSpace(to[len(executorType)+1:])
		toConfig[pathKey] = f.to.ToPath
		//初始化组件
		err := executor.Init(f.Router.Config, toConfig)
		if err != nil {
			panic(err)
		}
		f.to.executor = executor
	} else {
		f.to.executor = &ChainExecutor{}
		f.to.ToPath = to
	}
	return f.to
}

func (f *From) GetTo() endpoint.To {
	if f.to == nil {
		return nil
	}
	return f.to
}

// ToComponent to组件
// 参数是types.Node类型组件
func (f *From) ToComponent(node types.Node) endpoint.To {
	component := &ComponentExecutor{component: node, config: f.Router.Config}
	f.to = &To{Router: f.Router, To: node.Type(), ToPath: node.Type()}
	f.to.executor = component
	return f.to
}

// End 结束返回*Router
func (f *From) End() endpoint.Router {
	return f.Router
}

// To to端
type To struct {
	//toPath是否有占位符变量
	HasVars bool
	//Config to组件配置
	Config types.Configuration
	//Router router指针
	Router *Router
	//流转目标路径，例如"chain:{chainId}"，则是交给规则引擎处理数据
	To string
	//去掉to执行器标记的路径
	ToPath string
	//消息处理拦截器
	processList []endpoint.Process
	//目标处理器，默认是规则链处理
	executor endpoint.Executor
	//等待规则链/组件执行结束，并恢复到父进程，同步得到规则链结果。
	//用于需要等待规则链执行结果，并且要保留父进程的场景，否则不需要设置该字段。例如：http的响应。
	wait bool
	//规则上下文配置，如果配置了	`types.WithOnEnd` 需要接管`ChainExecutor`结果响应逻辑
	opts []types.RuleContextOption
}

// ToStringByDict 转换路径中的变量，并返回最终字符串
func (t *To) ToStringByDict(dict map[string]string) string {
	if t.HasVars {
		return str.SprintfDict(t.ToPath, dict)
	}
	return t.ToPath
}

func (t *To) ToString() string {
	return t.ToPath
}

// Execute 执行To端逻辑
func (t *To) Execute(ctx context.Context, exchange *endpoint.Exchange) {
	if t.executor != nil {
		t.executor.Execute(ctx, t.Router, exchange)
	}
}

// Transform 执行To端逻辑 后转换，如果规则链有多个结束点，则会执行多次
func (t *To) Transform(transform endpoint.Process) endpoint.To {
	t.processList = append(t.processList, transform)
	return t
}

// Process 执行To端逻辑 后处理，如果规则链有多个结束点，则会执行多次
func (t *To) Process(process endpoint.Process) endpoint.To {
	t.processList = append(t.processList, process)
	return t
}

// Wait 等待规则链/组件执行结束，并恢复到父进程。同步得到规则链结果。
// 用于需要等待规则链执行结果，并且要保留父进程的场景，否则不需要设置该字段。例如：http的响应。
func (t *To) Wait() endpoint.To {
	t.wait = true
	return t
}

func (t *To) SetWait(wait bool) endpoint.To {
	t.wait = wait
	return t
}
func (t *To) IsWait() bool {
	return t.wait
}

// SetOpts 规则上下文配置
func (t *To) SetOpts(opts ...types.RuleContextOption) endpoint.To {
	t.opts = opts
	return t
}

func (t *To) GetOpts() []types.RuleContextOption {
	return t.opts
}

// GetProcessList 获取执行To端逻辑 处理器
func (t *To) GetProcessList() []endpoint.Process {
	return t.processList
}

// End 结束返回*Router
func (t *To) End() endpoint.Router {
	return t.Router
}

// Router 路由，抽象不同输入源数据路由
// 把消息从输入端（From），经过转换（Transform）成RuleMsg结构，或者处理Process，然后交给规则链处理（To）
// 或者 把消息从输入端（From），经过转换（Transform），然后处理响应（Process）
// 用法：
// http endpoint
// endpoint.NewRouter().From("/api/v1/msg/").Transform().To("chain:xx")
// endpoint.NewRouter().From("/api/v1/msg/").Transform().Process().To("chain:xx")
// endpoint.NewRouter().From("/api/v1/msg/").Transform().Process().To("component:nodeType")
// endpoint.NewRouter().From("/api/v1/msg/").Transform().Process()
// mqtt endpoint
// endpoint.NewRouter().From("#").Transform().Process().To("chain:xx")
// endpoint.NewRouter().From("topic").Transform().Process().To("chain:xx")
type Router struct {
	//创建上下文回调函数
	ContextFunc func(ctx context.Context, exchange *endpoint.Exchange) context.Context
	//Config ruleEngine Config
	Config types.Config
	id     string
	//输入
	from *From
	//规则链池，默认使用rulego.DefaultPool
	RuleGo types.RuleEnginePool
	//动态获取规则链池函数
	ruleGoFunc func(exchange *endpoint.Exchange) types.RuleEnginePool
	//是否不可用 1:不可用;0:可以
	disable uint32
	//路由定义，如果没设置会返回nil
	def *types.RouterDsl
	//配置参数
	params []interface{}
}

// RouterOption 选项函数
type RouterOption = endpoint.RouterOption

//// WithRuleGoFunc 动态获取规则链池函数
//func WithRuleGoFunc(f func(exchange *endpoint.Exchange) types.RuleEnginePool) RouterOption {
//	return func(re *Router) error {
//		re.ruleGoFunc = f
//		return nil
//	}
//}
//
//// WithRuleGo 更改规则链池，默认使用rulego.DefaultPool
//func WithRuleGo(RuleGo types.RuleEnginePool) RouterOption {
//	return func(re *Router) error {
//		re.RuleGo = RuleGo
//		return nil
//	}
//}
//
//// WithRuleConfig 更改规则引擎配置
//func WithRuleConfig(config types.Config) RouterOption {
//	return func(re *Router) error {
//		re.Config = config
//		return nil
//	}
//}
//
//func WithContextFunc(ctx func(ctx context.Context, exchange *endpoint.Exchange) context.Context) RouterOption {
//	return func(re *Router) error {
//		re.ContextFunc = ctx
//		return nil
//	}
//}

// NewRouter 创建新的路由
func NewRouter(opts ...RouterOption) endpoint.Router {
	router := &Router{RuleGo: engine.DefaultPool, Config: engine.NewConfig()}
	// 设置选项值
	for _, opt := range opts {
		_ = opt(router)
	}
	return router
}

func (r *Router) SetConfig(config types.Config) {
	r.Config = config
}

func (r *Router) SetRuleEnginePool(pool types.RuleEnginePool) {
	r.RuleGo = pool
}

func (r *Router) SetRuleEnginePoolFunc(f func(exchange *endpoint.Exchange) types.RuleEnginePool) {
	r.ruleGoFunc = f
}

func (r *Router) SetContextFunc(f func(ctx context.Context, exchange *endpoint.Exchange) context.Context) {
	r.ContextFunc = f
}

func (r *Router) GetContextFunc() func(ctx context.Context, exchange *endpoint.Exchange) context.Context {
	return r.ContextFunc
}

func (r *Router) SetDefinition(def *types.RouterDsl) {
	r.def = def
}

// Definition 返回路由定义，如果没设置会返回nil
func (r *Router) Definition() *types.RouterDsl {
	return r.def
}

func (r *Router) SetId(id string) endpoint.Router {
	r.id = id
	return r
}

func (r *Router) GetId() string {
	return r.id
}
func (r *Router) FromToString() string {
	if r.from == nil {
		return ""
	} else {
		return r.from.ToString()
	}
}

func (r *Router) From(from string, configs ...types.Configuration) endpoint.From {
	var fromConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			fromConfig[k] = v
		}
	}
	r.from = &From{Router: r, From: from, Config: fromConfig}

	return r.from
}

func (r *Router) GetFrom() endpoint.From {
	if r.from == nil {
		return nil
	}
	return r.from
}

func (r *Router) GetRuleGo(exchange *endpoint.Exchange) types.RuleEnginePool {
	if r.ruleGoFunc != nil {
		return r.ruleGoFunc(exchange)
	} else {
		return r.RuleGo
	}
}

// Disable 设置状态 true:不可用，false:可以
func (r *Router) Disable(disable bool) endpoint.Router {
	if disable {
		atomic.StoreUint32(&r.disable, 1)
	} else {
		atomic.StoreUint32(&r.disable, 0)
	}
	return r
}

// IsDisable 是否是不可用状态 true:不可用，false:可以
func (r *Router) IsDisable() bool {
	return atomic.LoadUint32(&r.disable) == 1
}

func (r *Router) SetParams(args ...interface{}) {
	r.params = args
}

func (r *Router) GetParams() []interface{} {
	return r.params
}

// BaseEndpoint 基础端点
// 实现全局拦截器基础方法
type BaseEndpoint struct {
	//endpoint 路由存储器
	RouterStorage map[string]endpoint.Router
	OnEvent       endpoint.OnEvent
	//全局拦截器
	interceptors []endpoint.Process
	sync.RWMutex
}

func (e *BaseEndpoint) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	panic("not support this method")
}

func (e *BaseEndpoint) SetOnEvent(onEvent endpoint.OnEvent) {
	e.OnEvent = onEvent
}

// AddInterceptors 添加全局拦截器
func (e *BaseEndpoint) AddInterceptors(interceptors ...endpoint.Process) {
	e.interceptors = append(e.interceptors, interceptors...)
}

func (e *BaseEndpoint) DoProcess(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	//创建上下文
	ctx := e.createContext(baseCtx, router, exchange)
	for _, item := range e.interceptors {
		//执行全局拦截器
		if !item(router, exchange) {
			return
		}
	}
	//执行from端逻辑
	if fromFlow := router.GetFrom(); fromFlow != nil {
		if !fromFlow.ExecuteProcess(router, exchange) {
			return
		}
	}
	//执行to端逻辑
	if router.GetFrom() != nil && router.GetFrom().GetTo() != nil {
		router.GetFrom().GetTo().Execute(ctx, exchange)
	}
}

func (e *BaseEndpoint) createContext(baseCtx context.Context, router endpoint.Router, exchange *endpoint.Exchange) context.Context {
	if router.GetContextFunc() != nil {
		if ctx := router.GetContextFunc()(baseCtx, exchange); ctx == nil {
			panic("ContextFunc returned nil")
		} else {
			exchange.Context = ctx
			return ctx
		}
	} else if baseCtx != nil {
		return baseCtx
	} else {
		return context.Background()
	}

}

func (e *BaseEndpoint) CheckAndSetRouterId(router endpoint.Router) string {
	if router.GetId() == "" {
		router.SetId(router.FromToString())
	}
	return router.GetId()
}

func (e *BaseEndpoint) Destroy() {
	e.interceptors = nil
	e.RouterStorage = make(map[string]endpoint.Router)
}

// ExecutorFactory to端执行器工厂
type ExecutorFactory struct {
	sync.RWMutex
	executors map[string]endpoint.Executor
}

// Register 注册to端执行器
func (r *ExecutorFactory) Register(name string, executor endpoint.Executor) {
	r.Lock()
	defer r.Unlock()
	if r.executors == nil {
		r.executors = make(map[string]endpoint.Executor)
	}
	r.executors[name] = executor
}

// New 根据类型创建to端执行器实例
func (r *ExecutorFactory) New(name string) (endpoint.Executor, bool) {
	r.RLock()
	r.RUnlock()
	h, ok := r.executors[name]
	if ok {
		return h.New(), true
	} else {
		return nil, false
	}
}

// ChainExecutor 规则链执行器
type ChainExecutor struct {
}

func (ce *ChainExecutor) New() endpoint.Executor {

	return &ChainExecutor{}
}

// IsPathSupportVar to路径允许带变量
func (ce *ChainExecutor) IsPathSupportVar() bool {
	return true
}

func (ce *ChainExecutor) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

func (ce *ChainExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	fromFlow := router.GetFrom()
	if fromFlow == nil {
		return
	}
	inMsg := exchange.In.GetMsg()
	if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
		toChainId := toFlow.ToStringByDict(inMsg.Metadata.Values())
		tos := strings.Split(toChainId, pathSplitFlag)
		toChainId = tos[0]
		//查找规则链，并执行
		if ruleEngine, ok := router.GetRuleGo(exchange).Get(toChainId); ok {
			opts := toFlow.GetOpts()
			//监听结束回调函数
			endFunc := types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					exchange.Out.SetError(err)
				} else {
					exchange.Out.SetMsg(&msg)
				}

				for _, process := range toFlow.GetProcessList() {
					if !process(router, exchange) {
						break
					}
				}
			})
			opts = append(opts, types.WithContext(ctx))
			if len(tos) > 1 {
				opts = append(opts, types.WithStartNode(tos[1]))
			}
			opts = append(opts, endFunc)

			if toFlow.IsWait() {
				//同步
				ruleEngine.OnMsgAndWait(*inMsg, opts...)
			} else {
				//异步
				ruleEngine.OnMsg(*inMsg, opts...)
			}
		} else {
			//找不到规则链返回错误
			for _, process := range toFlow.GetProcessList() {
				exchange.Out.SetError(fmt.Errorf("chainId=%s not found error", toChainId))
				if !process(router, exchange) {
					break
				}
			}
		}

	}
}

// ComponentExecutor node组件执行器
type ComponentExecutor struct {
	component types.Node
	config    types.Config
}

func (ce *ComponentExecutor) New() endpoint.Executor {
	return &ComponentExecutor{}
}

// IsPathSupportVar to路径不允许带变量
func (ce *ComponentExecutor) IsPathSupportVar() bool {
	return false
}

func (ce *ComponentExecutor) Init(config types.Config, configuration types.Configuration) error {
	ce.config = config
	if configuration == nil {
		return fmt.Errorf("nodeType can't empty")
	}
	var nodeType = ""
	if v, ok := configuration[pathKey]; ok {
		nodeType = str.ToString(v)
	}
	node, err := config.ComponentsRegistry.NewNode(nodeType)
	if err == nil {
		ce.component = node
		err = ce.component.Init(config, configuration)
	}
	return err
}

func (ce *ComponentExecutor) Execute(ctx context.Context, router endpoint.Router, exchange *endpoint.Exchange) {
	if ce.component != nil {
		fromFlow := router.GetFrom()
		if fromFlow == nil {
			return
		}

		inMsg := exchange.In.GetMsg()
		if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
			//初始化的空上下文
			ruleCtx := engine.NewRuleContext(ctx, ce.config, nil, nil, nil, ce.config.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					exchange.Out.SetError(err)
				} else {
					exchange.Out.SetMsg(&msg)
				}
				for _, process := range toFlow.GetProcessList() {
					if !process(router, exchange) {
						break
					}
				}
			}, engine.DefaultPool)

			if toFlow.IsWait() {
				c := make(chan struct{})
				ruleCtx.SetOnAllNodeCompleted(func() {
					close(c)
				})
				//执行组件逻辑
				ce.component.OnMsg(ruleCtx, *inMsg)
				//等待执行结束
				<-c
			} else {
				//执行组件逻辑
				ce.component.OnMsg(ruleCtx, *inMsg)
			}
		}
	}
}

// DefaultExecutorFactory 默认to端执行器注册器
var DefaultExecutorFactory = new(ExecutorFactory)

// 注册默认执行器
func init() {
	DefaultExecutorFactory.Register("chain", &ChainExecutor{})
	DefaultExecutorFactory.Register("component", &ComponentExecutor{})
}
