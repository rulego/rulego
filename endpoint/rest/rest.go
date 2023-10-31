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

package rest

import (
	"context"
	"errors"
	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"strings"
)

const (
	ContentTypeKey  = "Content-Type"
	JsonContextType = "application/json"
)

//RequestMessage http请求消息
type RequestMessage struct {
	request *http.Request
	body    []byte
	//路径参数
	Params httprouter.Params
	msg    *types.RuleMsg
	err    error
}

func (r *RequestMessage) Body() []byte {
	if r.body == nil {
		defer func() {
			if r.request.Body != nil {
				_ = r.request.Body.Close()
			}
		}()
		entry, _ := ioutil.ReadAll(r.request.Body)
		r.body = entry
	}
	return r.body
}
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	return textproto.MIMEHeader(r.request.Header)
}

func (r RequestMessage) From() string {
	return r.request.URL.String()
}

func (r *RequestMessage) GetParam(key string) string {
	return r.request.FormValue(key)
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := types.TEXT
		if contentType := r.Headers().Get(ContentTypeKey); contentType == JsonContextType {
			dataType = types.JSON
		}
		ruleMsg := types.NewMsg(0, r.From(), dataType, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *RequestMessage) SetError(err error) {
	r.err = err
}

func (r *RequestMessage) GetError() error {
	return r.err
}

func (r *RequestMessage) Request() *http.Request {
	return r.request
}

//ResponseMessage http响应消息
type ResponseMessage struct {
	request  *http.Request
	response http.ResponseWriter
	body     []byte
	to       string
	msg      *types.RuleMsg
	err      error
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	return textproto.MIMEHeader(r.response.Header())
}

func (r *ResponseMessage) From() string {
	return r.request.URL.String()
}

func (r *ResponseMessage) GetParam(key string) string {
	return r.request.FormValue(key)
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
	r.response.WriteHeader(statusCode)
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	_, _ = r.response.Write(body)
}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}

func (r *ResponseMessage) Response() http.ResponseWriter {
	return r.response
}

//Config Rest 服务配置
type Config struct {
	Server      string
	CertFile    string
	CertKeyFile string
}

//Rest 接收端端点
type Rest struct {
	endpoint.BaseEndpoint
	//配置
	Config     Config
	RuleConfig types.Config
	//http路由器
	router *httprouter.Router
	server *http.Server
}

//Type 组件类型
func (rest *Rest) Type() string {
	return "http"
}

func (rest *Rest) New() types.Node {
	return &Rest{}
}

//Init 初始化
func (rest *Rest) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &rest.Config)
	rest.RuleConfig = ruleConfig
	return err
}

//Destroy 销毁
func (rest *Rest) Destroy() {
	_ = rest.Close()
}

func (rest *Rest) Close() error {
	if nil != rest.server {
		return rest.server.Shutdown(context.Background())
	}
	return nil
}

func (rest *Rest) Id() string {
	return rest.Config.Server
}

func (rest *Rest) AddRouterWithParams(router *endpoint.Router, params ...interface{}) (string, error) {
	if len(params) <= 0 {
		return "", errors.New("need to specify HTTP method")
	} else if router == nil {
		return "", errors.New("router can not nil")
	} else {
		for _, item := range params {
			rest.AddRouter(str.ToString(item), router)
		}
		return router.GetFrom().From, nil
	}
}

func (rest *Rest) RemoveRouterWithParams(routerId string, params ...interface{}) error {
	if len(params) <= 0 {
		return errors.New("need to specify HTTP method")
	} else {
		for _, item := range params {
			if router, ok := rest.RouterStorage[rest.routerKey(str.ToString(item), routerId)]; ok {
				router.Disable(true)
			}
		}
		return nil
	}
}

func (rest *Rest) Start() error {
	var err error
	rest.server = &http.Server{Addr: rest.Config.Server, Handler: rest.router}
	if rest.Config.CertKeyFile != "" && rest.Config.CertFile != "" {
		rest.Printf("starting server with TLS on :%s", rest.Config.Server)
		err = rest.server.ListenAndServeTLS(rest.Config.CertFile, rest.Config.CertKeyFile)
	} else {
		rest.Printf("starting server on :%s", rest.Config.Server)
		err = rest.server.ListenAndServe()
	}
	return err

}

// AddRouter 注册1个或者多个路由
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
func (rest *Rest) AddRouter(method string, routers ...*endpoint.Router) *Rest {
	method = strings.ToUpper(method)
	rest.Lock()
	defer rest.Unlock()
	if rest.router == nil {
		rest.router = httprouter.New()
	}

	if rest.RouterStorage == nil {
		rest.RouterStorage = make(map[string]*endpoint.Router)
	}
	for _, item := range routers {
		key := rest.routerKey(method, item.FromToString())
		if old, ok := rest.RouterStorage[key]; ok {
			//已经存储则，把路由设置可用
			old.Disable(false)
		} else {
			//存储路由
			rest.RouterStorage[key] = item
			//添加到http路由器
			rest.router.Handle(method, item.FromToString(), rest.handler(item))
		}
	}

	return rest
}

func (rest *Rest) GET(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodGet, routers...)
	return rest
}

func (rest *Rest) HEAD(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodHead, routers...)
	return rest
}

func (rest *Rest) OPTIONS(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodOptions, routers...)
	return rest
}

func (rest *Rest) POST(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPost, routers...)
	return rest
}

func (rest *Rest) PUT(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPut, routers...)
	return rest
}

func (rest *Rest) PATCH(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPatch, routers...)
	return rest
}

func (rest *Rest) DELETE(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodDelete, routers...)
	return rest
}

func (rest *Rest) GlobalOPTIONS(handler http.Handler) *Rest {
	if rest.router == nil {
		rest.router = httprouter.New()
		rest.router.GlobalOPTIONS = handler
	} else {
		rest.router.GlobalOPTIONS = handler
	}
	return rest
}

func (rest *Rest) Router() *httprouter.Router {
	return rest.router
}

func (rest *Rest) Stop() {

}

func (rest *Rest) routerKey(method string, from string) string {
	return method + " " + from
}
func (rest *Rest) handler(router *endpoint.Router) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				rest.Printf("rest handler err :%v", e)
			}
		}()
		if router.IsDisable() {
			http.NotFound(w, r)
			//w.WriteHeader(http.NotFound())
			return
		}
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: r,
				Params:  params,
			},
			Out: &ResponseMessage{
				request:  r,
				response: w,
			}}

		msg := exchange.In.GetMsg()
		//把路径参数放到msg元数据中
		for _, param := range params {
			msg.Metadata.PutValue(param.Key, param.Value)
		}

		//把url?参数放到msg元数据中
		for key, value := range r.URL.Query() {
			if len(value) > 1 {
				msg.Metadata.PutValue(key, str.ToString(value))
			} else {
				msg.Metadata.PutValue(key, value[0])
			}

		}
		rest.DoProcess(router, exchange)
	}
}

func (rest *Rest) Printf(format string, v ...interface{}) {
	if rest.RuleConfig.Logger != nil {
		rest.RuleConfig.Logger.Printf(format, v...)
	}
}
