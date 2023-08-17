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

package endpoint

import (
	"context"
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
	"net/textproto"
	"strings"
	"sync"
)

//Message 接收端点数据抽象接口
//不同输入源数据统一接口
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
}

//Exchange 包含in 和out message
type Exchange struct {
	In  Message
	Out Message
}

//Process 处理函数
//true:执行下一个处理器，否则不执行
type Process func(exchange *Exchange) bool

//From 来源路由
type From struct {
	router *Router
	//来源路径
	from string
	//消息处理拦截器
	processList []Process
	//xx:前缀的方式标记目的地，例如"chain:{chainId}"，则是交给规则引擎处理数据
	to *To
}

func (f *From) ToString() string {
	return f.from
}

func (f *From) From(from string) *From {
	f.from = from
	return f
}

func (f *From) Transform(transform Process) *From {
	f.processList = append(f.processList, transform)
	return f
}

func (f *From) Process(process Process) *From {
	f.processList = append(f.processList, process)
	return f
}

func (f *From) GetProcessList() []Process {
	return f.processList
}

//ExecuteProcess 执行处理函数
//true:执行下一个逻辑，否则不执行
func (f *From) ExecuteProcess(exchange *Exchange) bool {
	result := true
	for _, process := range f.GetProcessList() {
		if !process(exchange) {
			result = false
			break
		}
	}
	return result
}

func (f *From) To(to string) *To {
	f.to = &To{router: f.router, to: to}

	toHandlerType := strings.Split(to, ":")[0]

	if executor, ok := DefaultExecutorFactory.New(toHandlerType); ok {
		f.to.executor = executor
		f.to.toPath = strings.TrimSpace(to[len(toHandlerType)+1:])
	} else {
		f.to.executor = &ChainExecutor{}
		f.to.toPath = to
	}
	if strings.Contains(to, "${") && strings.Contains(to, "}") {
		f.to.HasVars = true
	}
	return f.to
}

func (f *From) GetTo() *To {
	return f.to
}

func (f *From) ToComponent(node types.Node) *To {
	component := &ComponentExecutor{component: node, config: f.router.config}
	f.to = &To{router: f.router, to: node.Type(), toPath: node.Type()}
	f.to.executor = component

	return f.to
}

func (f *From) End() *Router {
	return f.router
}

//To 目的地路由
type To struct {
	router *Router
	//xx:前缀的方式标记目的地，例如"chain:{chainId}"，则是交给规则引擎处理数据
	to string
	//去掉标记前缀后的路径
	toPath string
	//消息处理拦截器
	processList []Process
	//是否有占位符变量
	HasVars bool
	//目标处理器，默认是规则链处理
	executor Executor
}

func (t *To) ToStringByDict(dict map[string]string) string {
	if t.HasVars {
		return str.SprintfDict(t.toPath, dict)
	}
	return t.toPath
}

func (t *To) ToString() string {
	return t.toPath
}

//Execute 执行
func (t *To) Execute(ctx context.Context, exchange *Exchange) {
	if t.executor != nil {
		t.executor.Execute(ctx, t.router, exchange)
	}
}

func (t *To) Transform(transform Process) *To {
	t.processList = append(t.processList, transform)
	return t
}

func (t *To) Process(process Process) *To {
	t.processList = append(t.processList, process)
	return t
}

func (t *To) GetProcessList() []Process {
	return t.processList
}

func (t *To) End() *Router {
	return t.router
}

//Router 路由，抽象不同输入源数据路由
//把消息从输入端（From），经过转换（Transform）成RuleMsg结构，或者处理Process，然后交给规则链处理（To）
//或者 把消息从输入端（From），经过转换（Transform），然后处理响应（Process）
//用法：
//http endpoint
// endpoint.NewRouter().From("/api/v1/msg/").Transform().To("chain:xx")
// endpoint.NewRouter().From("/api/v1/msg/").Transform().Process().To("chain:xx")
// endpoint.NewRouter().From("/api/v1/msg/").Transform().Process()
//mqtt endpoint
// endpoint.NewRouter().From("#").Transform().Process().To("chain:xx")
// endpoint.NewRouter().From("topic").Transform().Process().To("chain:xx")
type Router struct {
	//输入
	from *From
	//规则链池，默认使用rulego.DefaultRuleGo
	ruleGo *rulego.RuleGo
	config types.Config
}

//RouterOption 选项函数
type RouterOption func(*Router) error

//WithRuleGo 更改规则链池，默认使用rulego.DefaultRuleGo
func WithRuleGo(ruleGo *rulego.RuleGo) RouterOption {
	return func(re *Router) error {
		re.ruleGo = ruleGo
		return nil
	}
}

func WithRuleConfig(config types.Config) RouterOption {
	return func(re *Router) error {
		re.config = config
		return nil
	}
}
func NewRouter(opts ...RouterOption) *Router {
	router := &Router{ruleGo: rulego.DefaultRuleGo, config: rulego.NewConfig()}
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

func (r *Router) From(from string) *From {
	r.from = &From{router: r, from: from}
	return r.from
}

func (r *Router) GetFrom() *From {
	return r.from
}

type Executor interface {
	New() Executor
	Init(config types.Config, configuration types.Configuration) error
	Execute(ctx context.Context, router *Router, exchange *Exchange)
}

//ExecutorFactory 执行器工厂
type ExecutorFactory struct {
	sync.RWMutex
	executors map[string]Executor
}

func (r *ExecutorFactory) Register(name string, executor Executor) {
	r.Lock()
	r.Unlock()
	if r.executors == nil {
		r.executors = make(map[string]Executor)
	}
	r.executors[name] = executor
}

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

//ChainExecutor 规则链执行器
type ChainExecutor struct {
}

func (ce *ChainExecutor) New() Executor {

	return &ChainExecutor{}
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
		if ruleEngine, ok := router.ruleGo.Get(toChainId); ok {
			ruleEngine.OnMsgWithOptions(*inMsg, types.WithContext(ctx),
				types.WithEndFunc(func(msg types.RuleMsg, err error) {
					exchange.Out.SetMsg(&msg)
					for _, process := range toFlow.GetProcessList() {
						if !process(exchange) {
							break
						}
					}
				}))
		}

	}
}

//ComponentExecutor node组件执行器
type ComponentExecutor struct {
	component types.Node
	config    types.Config
}

func (ce *ComponentExecutor) New() Executor {
	return &ComponentExecutor{}
}

func (ce *ComponentExecutor) Init(config types.Config, configuration types.Configuration) error {
	ce.config = config
	if configuration == nil {
		return fmt.Errorf("nodeType can't empty")
	}
	nodeType := configuration.GetToString("nodeType")
	node, err := config.ComponentsRegistry.NewNode(nodeType)
	if err == nil {
		ce.component = node
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
			ruleCtx := rulego.NewRuleContext(ce.config, nil, nil, nil, ce.config.Pool, func(msg types.RuleMsg, err error) {
				exchange.Out.SetMsg(&msg)
				for _, process := range toFlow.GetProcessList() {
					if !process(exchange) {
						break
					}
				}
			}, ctx)

			//执行组件逻辑
			_ = ce.component.OnMsg(ruleCtx, *inMsg)
		}
	}
}

var DefaultExecutorFactory = new(ExecutorFactory)

//注册默认执行器
func init() {
	DefaultExecutorFactory.Register("chain", &ChainExecutor{})
	DefaultExecutorFactory.Register("component", &ComponentExecutor{})
}
