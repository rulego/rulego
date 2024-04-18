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

package websocket

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"log"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"
)

// Type 组件类型
const Type = "ws"

// Endpoint 别名
type Endpoint = Websocket

// 注册组件
func init() {
	_ = endpoint.Registry.Register(&Endpoint{})
}

// RequestMessage websocket请求消息
type RequestMessage struct {
	//ws消息类型 TextMessage=1/BinaryMessage=2
	messageType int
	request     *http.Request
	body        []byte
	//路径参数
	Params httprouter.Params
	msg    *types.RuleMsg
	err    error
}

func (r *RequestMessage) Body() []byte {
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
		//默认指定是JSON格式，如果不是该类型，请在process函数中修改
		dataType := types.JSON
		if r.messageType == websocket.BinaryMessage {
			dataType = types.BINARY
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

// ResponseMessage websocket响应消息
type ResponseMessage struct {
	headers textproto.MIMEHeader
	//ws消息类型 TextMessage/BinaryMessage
	messageType int
	log         func(format string, v ...interface{})
	request     *http.Request
	conn        *websocket.Conn
	body        []byte
	to          string
	msg         *types.RuleMsg
	err         error
	locker      sync.RWMutex
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
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

// SetStatusCode 不提供设置状态码
func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	if r.conn != nil {
		if r.messageType == 0 {
			r.messageType = websocket.TextMessage
		}
		// 在写入之前加锁
		r.locker.Lock()
		defer r.locker.Unlock()
		err := r.conn.WriteMessage(r.messageType, body)
		if err != nil {
			log.Println("write:", err)
		}
	}
}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}

// Config Websocket 服务配置
type Config struct {
	Server      string
	CertFile    string
	CertKeyFile string
}

// Websocket 接收端端点
type Websocket struct {
	endpoint.BaseEndpoint
	//配置
	Config       Config
	RuleConfig   types.Config
	OnEventFunc  endpoint.OnEvent
	RestEndpoint *rest.Rest
	Upgrader     websocket.Upgrader
	server       *http.Server
	//http路由器
	router *httprouter.Router
}

// Type 组件类型
func (ws *Websocket) Type() string {
	return Type
}

func (ws *Websocket) New() types.Node {
	return &Websocket{}
}

// Init 初始化
func (ws *Websocket) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &ws.Config)
	ws.RuleConfig = ruleConfig
	return err
}

// Destroy 销毁
func (ws *Websocket) Destroy() {
	_ = ws.Close()
}

func (ws *Websocket) Close() error {
	if nil != ws.server {
		return ws.server.Shutdown(context.Background())
	}
	return nil
}

func (ws *Websocket) Id() string {
	return ws.Config.Server
}

func (ws *Websocket) AddRouter(router *endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	} else {
		ws.addRouter(router)
		return router.GetFrom().From, nil
	}
}

func (ws *Websocket) RemoveRouter(routerId string, params ...interface{}) error {
	if router, ok := ws.RouterStorage[ws.routerKey("GET", routerId)]; ok {
		router.Disable(true)
	}
	return nil
}

func (ws *Websocket) Start() error {
	var err error
	//已经初始化
	if ws.RestEndpoint != nil {
		if ws.OnEventFunc != nil {
			ws.OnEventFunc(endpoint.EventInitServer, ws.RestEndpoint.Server)
		}
		return nil
	}
	if ws.router == nil {
		ws.router = httprouter.New()
	}
	ws.server = &http.Server{Addr: ws.Config.Server, Handler: ws.router}
	if ws.OnEventFunc != nil {
		ws.OnEventFunc(endpoint.EventInitServer, ws.server)
	}
	if ws.Config.CertKeyFile != "" && ws.Config.CertFile != "" {
		ws.Printf("starting ws server with TLS on :%s", ws.Config.Server)
		err = ws.server.ListenAndServeTLS(ws.Config.CertFile, ws.Config.CertKeyFile)
	} else {
		ws.Printf("starting ws server on :%s", ws.Config.Server)
		err = ws.server.ListenAndServe()
	}
	return err

}

// addRouter 注册1个或者多个路由
func (ws *Websocket) addRouter(routers ...*endpoint.Router) *Websocket {
	ws.Lock()
	defer ws.Unlock()
	if ws.router == nil {
		ws.router = httprouter.New()
	}

	if ws.RouterStorage == nil {
		ws.RouterStorage = make(map[string]*endpoint.Router)
	}
	for _, item := range routers {
		key := ws.routerKey("GET", item.FromToString())
		if old, ok := ws.RouterStorage[key]; ok {
			//已经存储则，把路由设置可用
			old.Disable(false)
		} else {
			//存储路由
			ws.RouterStorage[key] = item
			//添加到http路由器
			if ws.RestEndpoint != nil {
				ws.RestEndpoint.Router().Handle("GET", item.FromToString(), ws.handler(item))
			} else {
				ws.router.Handle("GET", item.FromToString(), ws.handler(item))
			}
		}
	}

	return ws
}

func (ws *Websocket) Router() *httprouter.Router {
	return ws.router
}

func (ws *Websocket) routerKey(method string, from string) string {
	return method + " " + from
}
func (ws *Websocket) handler(router *endpoint.Router) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		c, err := ws.Upgrader.Upgrade(w, r, nil)
		if err != nil {
			ws.Printf("upgrade:", err)
			return
		}
		connectExchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: r,
				Params:  params,
				body:    nil,
			},
			Out: &ResponseMessage{
				log: func(format string, v ...interface{}) {
					ws.Printf(format, v...)
				},
				request: r,
				conn:    c,
			}}
		if ws.OnEventFunc != nil {
			ws.OnEventFunc(endpoint.EventConnect, connectExchange)
		}
		defer func() {
			_ = c.Close()
			//捕捉异常
			if e := recover(); e != nil {
				if ws.OnEventFunc != nil {
					ws.OnEventFunc(endpoint.EventDisconnect, connectExchange)
				}
				ws.Printf("ws handler err :%v", e)
			}
		}()

		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				if ws.OnEventFunc != nil {
					ws.OnEventFunc(endpoint.EventDisconnect, w, r, params)
				}
				break
			}

			if router.IsDisable() {
				if ws.OnEventFunc != nil {
					ws.OnEventFunc(endpoint.EventDisconnect, w, r, params)
				}
				http.NotFound(w, r)
				break
			}
			if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
				continue
			}
			//ws.Printf("recv:", string(message))
			exchange := &endpoint.Exchange{
				In: &RequestMessage{
					request:     r,
					Params:      params,
					body:        message,
					messageType: mt,
				},
				Out: &ResponseMessage{
					log: func(format string, v ...interface{}) {
						ws.Printf(format, v...)
					},
					request:     r,
					conn:        c,
					messageType: mt,
				}}

			msg := exchange.In.GetMsg()
			//把路径参数放到msg元数据中
			for _, param := range params {
				msg.Metadata.PutValue(param.Key, param.Value)
			}

			msg.Metadata.PutValue("messageType", strconv.Itoa(mt))

			//把url?参数放到msg元数据中
			for key, value := range r.URL.Query() {
				if len(value) > 1 {
					msg.Metadata.PutValue(key, str.ToString(value))
				} else {
					msg.Metadata.PutValue(key, value[0])
				}

			}
			ws.DoProcess(router, exchange)
		}

	}
}

func (ws *Websocket) Printf(format string, v ...interface{}) {
	if ws.RuleConfig.Logger != nil {
		ws.RuleConfig.Logger.Printf(format, v...)
	}
}
