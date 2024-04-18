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
	"io"
	"net/http"
	"net/textproto"
	"strings"
)

const (
	ContentTypeKey  = "Content-Type"
	JsonContextType = "application/json"
)

// Type 组件类型
const Type = "http"

// Endpoint 别名
type Endpoint = Rest

// 注册组件
func init() {
	_ = endpoint.Registry.Register(&Endpoint{})
}

// RequestMessage http请求消息
type RequestMessage struct {
	request *http.Request
	body    []byte
	//路径参数
	Params httprouter.Params
	msg    *types.RuleMsg
	err    error
}

func (r *RequestMessage) Body() []byte {
	if r.body == nil && r.request != nil {
		defer func() {
			if r.request.Body != nil {
				_ = r.request.Body.Close()
			}
		}()
		entry, _ := io.ReadAll(r.request.Body)
		r.body = entry
	}
	return r.body
}

func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.request == nil {
		return nil
	}
	return textproto.MIMEHeader(r.request.Header)
}

func (r RequestMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.URL.String()
}

func (r *RequestMessage) GetParam(key string) string {
	if r.request == nil {
		return ""
	}
	if v := r.Params.ByName(key); v == "" {
		return r.request.FormValue(key)
	} else {
		return v
	}
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

// ResponseMessage http响应消息
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
	if r.response == nil {
		return nil
	}
	return textproto.MIMEHeader(r.response.Header())
}

func (r *ResponseMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.URL.String()
}

func (r *ResponseMessage) GetParam(key string) string {
	if r.request == nil {
		return ""
	}
	return r.request.FormValue(key)
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
	if r.response != nil {
		r.response.WriteHeader(statusCode)
	}
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	if r.response != nil {
		_, _ = r.response.Write(body)
	}
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

// Config Rest 服务配置
type Config struct {
	Server      string
	CertFile    string
	CertKeyFile string
}

// Rest 接收端端点
type Rest struct {
	endpoint.BaseEndpoint
	//配置
	Config     Config
	RuleConfig types.Config
	Server     *http.Server
	//http路由器
	router *httprouter.Router
	//事件回调函数
	OnEventFunc endpoint.OnEvent
}

// Type 组件类型
func (rest *Rest) Type() string {
	return Type
}

func (rest *Rest) New() types.Node {
	return &Rest{}
}

// Init 初始化
func (rest *Rest) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &rest.Config)
	rest.RuleConfig = ruleConfig
	return err
}

// Destroy 销毁
func (rest *Rest) Destroy() {
	_ = rest.Close()
}

func (rest *Rest) Close() error {
	if nil != rest.Server {
		return rest.Server.Shutdown(context.Background())
	}
	return nil
}

func (rest *Rest) Id() string {
	return rest.Config.Server
}

func (rest *Rest) AddRouter(router *endpoint.Router, params ...interface{}) (string, error) {
	if len(params) <= 0 {
		return "", errors.New("need to specify HTTP method")
	} else if router == nil {
		return "", errors.New("router can not nil")
	} else {
		for _, item := range params {
			rest.addRouter(str.ToString(item), router)
		}
		return router.GetFrom().From, nil
	}
}

func (rest *Rest) RemoveRouter(routerId string, params ...interface{}) error {
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
	if rest.router == nil {
		rest.router = httprouter.New()
	}
	var err error
	rest.Server = &http.Server{Addr: rest.Config.Server, Handler: rest.router}
	if rest.OnEventFunc != nil {
		rest.OnEventFunc(endpoint.EventInitServer, rest)
	}
	if rest.Config.CertKeyFile != "" && rest.Config.CertFile != "" {
		rest.Printf("starting rest server with TLS on :%s", rest.Config.Server)
		err = rest.Server.ListenAndServeTLS(rest.Config.CertFile, rest.Config.CertKeyFile)
	} else {
		rest.Printf("starting rest server on :%s", rest.Config.Server)
		err = rest.Server.ListenAndServe()
	}
	return err

}

// addRouter 注册1个或者多个路由
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
func (rest *Rest) addRouter(method string, routers ...*endpoint.Router) *Rest {
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
	rest.addRouter(http.MethodGet, routers...)
	return rest
}

func (rest *Rest) HEAD(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodHead, routers...)
	return rest
}

func (rest *Rest) OPTIONS(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodOptions, routers...)
	return rest
}

func (rest *Rest) POST(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodPost, routers...)
	return rest
}

func (rest *Rest) PUT(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodPut, routers...)
	return rest
}

func (rest *Rest) PATCH(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodPatch, routers...)
	return rest
}

func (rest *Rest) DELETE(routers ...*endpoint.Router) *Rest {
	rest.addRouter(http.MethodDelete, routers...)
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
