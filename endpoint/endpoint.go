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
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
	"net/textproto"
	"strings"
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
	if toHandler, ok := ToHandlerRegistry[toHandlerType]; ok {
		f.router.ToHandler = toHandler
		f.to.toPath = strings.TrimSpace(to[len(toHandlerType)+1:])
	} else {
		f.router.ToHandler = RuleChainHandler
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
}

func (t *To) ToString() string {
	return t.toPath
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
	//目标处理器，默认是规则链处理
	ToHandler ToHandler
}

func NewRouter() *Router {
	return &Router{}
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

//ToHandler 目标处理器
type ToHandler func(ctx context.Context, router *Router, exchange *Exchange)

//RuleChainHandler 规则链处理器
var RuleChainHandler = func(ctx context.Context, router *Router, exchange *Exchange) {
	fromFlow := router.GetFrom()
	if fromFlow == nil {
		return
	}
	inMsg := exchange.In.GetMsg()
	if toFlow := fromFlow.GetTo(); toFlow != nil && inMsg != nil {
		toChainId := toFlow.toPath
		if toFlow.HasVars {
			toChainId = str.SprintfDict(toFlow.toPath, inMsg.Metadata.Values())
		}
		if ruleEngine, ok := rulego.Get(toChainId); ok {
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

//ToHandlerRegistry 处理器注册
var ToHandlerRegistry = map[string]ToHandler{
	"chain": RuleChainHandler,
}
