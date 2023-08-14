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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"io/ioutil"
	"log"
	"net/http"
	"net/textproto"
	"sync"
)

const (
	ContentTypeKey  = "Content-Type"
	JsonContextType = "application/json"
)

//RequestMessage http请求消息
type RequestMessage struct {
	request *http.Request
	body    []byte
	msg     *types.RuleMsg
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
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), "")
		if contentType := r.Headers().Get(ContentTypeKey); contentType == JsonContextType {
			//如果Content-Type是application/json 类型，把body复制到msg.Data
			ruleMsg.Data = string(r.Body())
		}

		r.msg = &ruleMsg
	}
	return r.msg
}
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
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

func (r *ResponseMessage) Response() http.ResponseWriter {
	return r.response
}

//Config Rest 服务配置
type Config struct {
	Addr        string
	CertFile    string
	CertKeyFile string
}

//Rest 接收端端点
type Rest struct {
	Config  Config
	Routers map[string]*endpoint.Router
	sync.RWMutex
}

func (r *Rest) Start() error {
	r.RLock()
	for _, router := range r.Routers {
		form := router.GetFrom()
		if form != nil {
			http.Handle(form.ToString(), r.handler(router))
		}
	}
	r.RUnlock()
	var err error
	if r.Config.CertKeyFile != "" && r.Config.CertFile != "" {
		log.Printf("starting server with TLS on :%s", r.Config.Addr)
		err = http.ListenAndServeTLS(r.Config.Addr, r.Config.CertFile, r.Config.CertKeyFile, nil)
	} else {
		log.Printf("starting server on :%s", r.Config.Addr)
		err = http.ListenAndServe(r.Config.Addr, nil)
	}
	return err

}

//AddRouter 添加路由
func (r *Rest) AddRouter(routers ...*endpoint.Router) *Rest {
	r.Lock()
	defer r.Unlock()
	if r.Routers == nil {
		r.Routers = make(map[string]*endpoint.Router)
	}
	for _, router := range routers {
		r.Routers[router.FromToString()] = router
	}
	return r
}

func (r *Rest) Stop() {

}

func (r *Rest) handler(router *endpoint.Router) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				log.Printf("rest handler err :%v", e)
			}
		}()

		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: r,
			},
			Out: &ResponseMessage{
				request:  r,
				response: w,
			}}

		if fromFlow := router.GetFrom(); fromFlow != nil {
			fromFlow.ExecuteProcess(exchange)
		}

		if router.ToHandler != nil {
			router.ToHandler(r.Context(), router, exchange)
		}
	})
}
