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

// Package endpoint /**

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	pathKey = "_path"
)

var ChainNotFoundErr = errors.New("chain not found error")

type Endpoint interface {
	//Node 继承node
	types.Node
	//Id 类型标识
	Id() string
	//Start 启动服务
	Start() error
	//AddInterceptors 添加全局拦截器
	AddInterceptors(interceptors ...Process)
	//AddRouter 添加路由，指定参数
	//params 为路由额外参数
	//返回路由ID，路由ID一般是from值，但某些Endpoint允许from值重复，Endpoint会返回新的路由ID，
	//路由ID用于路由删除
	AddRouter(router *Router, params ...interface{}) (string, error)
	//RemoveRouter 删除路由，指定参数
	//params 为路由额外参数
	//routerId:路由ID
	RemoveRouter(routerId string, params ...interface{}) error
}

// Message 接收端点数据抽象接口
// 不同输入源数据统一接口
type Message interface {
	//Body message body
	Body() []byte
	Headers() textproto.MIMEHeader
	From() string
	//GetParam http.Request#FormValue
	GetParam(key string) string
	//SetMsg set RuleMsg
	SetMsg(msg *types.RuleMsg)
	//GetMsg 把接收数据转换成 RuleMsg
	GetMsg() *types.RuleMsg
	//SetStatusCode 响应 code
	SetStatusCode(statusCode int)
	//SetBody 响应 body
	SetBody(body []byte)
	//SetError 设置错误
	SetError(err error)
	//GetError 获取错误
	GetError() error
}

// Exchange 包含in 和out message
type Exchange struct {
	//入数据
	In Message
	//出数据
	Out Message
}

// Process 处理函数
// true:执行下一个处理器，否则不执行
type Process func(router *Router, exchange *Exchange) bool

// From from端
type From struct {
	//Config 配置
	Config types.Configuration
	//Router router指针
	Router *Router
	//来源路径
	From string
	//消息处理拦截器
	processList []Process
	//流转目标路径，例如"chain:{chainId}"，则是交给规则引擎处理数据
	to *To
}

func (f *From) ToString() string {
	return f.From
}

// Transform from端转换msg
func (f *From) Transform(transform Process) *From {
	f.processList = append(f.processList, transform)
	return f
}

// Process from端处理msg
func (f *From) Process(process Process) *From {
	f.processList = append(f.processList, process)
	return f
}

// GetProcessList 获取from端处理器列表
func (f *From) GetProcessList() []Process {
	return f.processList
}

// ExecuteProcess 执行处理函数
// true:执行To端逻辑，否则不执行
func (f *From) ExecuteProcess(router *Router, exchange *Exchange) bool {
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
// component:{nodeType} 执行在config.ComponentsRegistry 中注册的组件
// 可在DefaultExecutorFactory中注册自定义执行器组件类型
// componentConfigs 组件配置参数
func (f *From) To(to string, configs ...types.Configuration) *To {
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
	executorType := strings.Split(to, ":")[0]

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

func (f *From) GetTo() *To {
	return f.to
}

// ToComponent to组件
// 参数是types.Node类型组件
func (f *From) ToComponent(node types.Node) *To {
	component := &ComponentExecutor{component: node, config: f.Router.Config}
	f.to = &To{Router: f.Router, To: node.Type(), ToPath: node.Type()}
	f.to.executor = component
	return f.to
}

// End 结束返回*Router
func (f *From) End() *Router {
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
	processList []Process
	//目标处理器，默认是规则链处理
	executor Executor
	//等待规则链/组件执行结束，并恢复到父进程，同步得到规则链结果。
	//用于需要等待规则链执行结果，并且要保留父进程的场景，否则不需要设置该字段。例如：http的响应。
	wait bool
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
func (t *To) Execute(ctx context.Context, exchange *Exchange) {
	if t.executor != nil {
		t.executor.Execute(ctx, t.Router, exchange)
	}
}

// Transform 执行To端逻辑 后转换，如果规则链有多个结束点，则会执行多次
func (t *To) Transform(transform Process) *To {
	t.processList = append(t.processList, transform)
	return t
}

// Process 执行To端逻辑 后处理，如果规则链有多个结束点，则会执行多次
func (t *To) Process(process Process) *To {
	t.processList = append(t.processList, process)
	return t
}

// Wait 等待规则链/组件执行结束，并恢复到父进程。同步得到规则链结果。
// 用于需要等待规则链执行结果，并且要保留父进程的场景，否则不需要设置该字段。例如：http的响应。
func (t *To) Wait() *To {
	t.wait = true
	return t
}

// GetProcessList 获取执行To端逻辑 处理器
func (t *To) GetProcessList() []Process {
	return t.processList
}

// End 结束返回*Router
func (t *To) End() *Router {
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
	//输入
	from *From
	//规则链池，默认使用rulego.DefaultRuleGo
	RuleGo *rulego.RuleGo
	//Config ruleEngine Config
	Config types.Config
	//是否不可用 1:不可用;0:可以
	disable uint32
}

// RouterOption 选项函数
type RouterOption func(*Router) error

// WithRuleGo 更改规则链池，默认使用rulego.DefaultRuleGo
func WithRuleGo(ruleGo *rulego.RuleGo) RouterOption {
	return func(re *Router) error {
		re.RuleGo = ruleGo
		return nil
	}
}

// WithRuleConfig 更改规则引擎配置
func WithRuleConfig(config types.Config) RouterOption {
	return func(re *Router) error {
		re.Config = config
		return nil
	}
}

// NewRouter 创建新的路由
func NewRouter(opts ...RouterOption) *Router {
	router := &Router{RuleGo: rulego.DefaultRuleGo, Config: rulego.NewConfig()}
	// 设置选项值
	for _, opt := range opts {
		_ = opt(router)
	}
	return router
}

func (r *Router) FromToString() string {
	if r.from == nil {
		return ""
	} else {
		return r.from.ToString()
	}
}

func (r *Router) From(from string, configs ...types.Configuration) *From {
	var fromConfig = make(types.Configuration)
	for _, item := range configs {
		for k, v := range item {
			fromConfig[k] = v
		}
	}
	r.from = &From{Router: r, From: from, Config: fromConfig}

	return r.from
}

func (r *Router) GetFrom() *From {
	return r.from
}

// Disable 设置状态 true:不可用，false:可以
func (r *Router) Disable(disable bool) *Router {
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

// BaseEndpoint 基础端点
// 实现全局拦截器基础方法
type BaseEndpoint struct {
	//全局拦截器
	interceptors []Process
	//endpoint 路由存储器
	RouterStorage map[string]*Router
	sync.RWMutex
}

func (e *BaseEndpoint) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	panic("not support this method")
}

// AddInterceptors 添加全局拦截器
func (e *BaseEndpoint) AddInterceptors(interceptors ...Process) {
	e.interceptors = append(e.interceptors, interceptors...)
}

func (e *BaseEndpoint) DoProcess(router *Router, exchange *Exchange) {
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
		router.GetFrom().GetTo().Execute(context.TODO(), exchange)
	}
}

func (e *BaseEndpoint) baseAddRouter(from string, params ...interface{}) error {
	e.Lock()
	defer e.Unlock()
	if e.RouterStorage != nil {
		delete(e.RouterStorage, from)
	}
	return nil
}

// Executor to端执行器
type Executor interface {
	//New 创建新的实例
	New() Executor
	//IsPathSupportVar to路径是否支持${}变量方式，默认不支持
	IsPathSupportVar() bool
	//Init 初始化
	Init(config types.Config, configuration types.Configuration) error
	//Execute 执行逻辑
	Execute(ctx context.Context, router *Router, exchange *Exchange)
}

// ExecutorFactory to端执行器工厂
type ExecutorFactory struct {
	sync.RWMutex
	executors map[string]Executor
}

// Register 注册to端执行器
func (r *ExecutorFactory) Register(name string, executor Executor) {
	r.Lock()
	r.Unlock()
	if r.executors == nil {
		r.executors = make(map[string]Executor)
	}
	r.executors[name] = executor
}

// New 根据类型创建to端执行器实例
func (r *ExecutorFactory) New(name string) (Executor, bool) {
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

func (ce *ChainExecutor) New() Executor {

	return &ChainExecutor{}
}

// IsPathSupportVar to路径允许带变量
func (ce *ChainExecutor) IsPathSupportVar() bool {
	return true
}

func (ce *ChainExecutor) Init(_ types.Config, _ types.Configuration) error {
	return nil
}

func (ce *ChainExecutor) Execute(ctx context.Context, router *Router, exchange *Exchange) {
	fromFlow := router.GetFrom()
	if fromFlow == nil {
		return
	}
	inMsg := exchange.In.GetMsg()
	if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
		toChainId := toFlow.ToStringByDict(inMsg.Metadata.Values())

		//查找规则链，并执行
		if ruleEngine, ok := router.RuleGo.Get(toChainId); ok {
			//监听结束回调函数
			endFunc := types.WithEndFunc(func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
			if toFlow.wait {
				//同步
				ruleEngine.OnMsgAndWait(*inMsg, types.WithContext(ctx), endFunc)
			} else {
				//异步
				ruleEngine.OnMsg(*inMsg, types.WithContext(ctx), endFunc)
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

func (ce *ComponentExecutor) New() Executor {
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
	nodeType := configuration.GetToString(pathKey)
	node, err := config.ComponentsRegistry.NewNode(nodeType)
	if err == nil {
		ce.component = node
		err = ce.component.Init(config, configuration)
	}
	return err
}

func (ce *ComponentExecutor) Execute(ctx context.Context, router *Router, exchange *Exchange) {
	if ce.component != nil {
		fromFlow := router.GetFrom()
		if fromFlow == nil {
			return
		}

		inMsg := exchange.In.GetMsg()
		if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
			//初始化的空上下文
			ruleCtx := rulego.NewRuleContext(ctx, ce.config, nil, nil, nil, ce.config.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error) {
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
			}, rulego.DefaultRuleGo)

			//执行组件逻辑
			ce.component.OnMsg(ruleCtx, *inMsg)
			if toFlow.wait {
				c := make(chan struct{})
				ruleCtx.SetAllCompletedFunc(func() {
					close(c)
				})
				//等待执行结束
				<-c
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
