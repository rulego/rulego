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

// Package websocket provides a WebSocket endpoint implementation for the RuleGo framework.
// It allows creating WebSocket servers that can receive and process incoming WebSocket messages,
// routing them to appropriate rule chains or components for further processing.
//
// Key components in this package include:
// - Endpoint (alias Websocket): Implements the WebSocket server and message handling
// - RequestMessage: Represents an incoming WebSocket message
// - ResponseMessage: Represents the WebSocket message to be sent back
//
// The WebSocket endpoint supports dynamic routing configuration, allowing users to
// define message patterns and their corresponding rule chain or component destinations.
// It also provides flexibility in handling different WebSocket message types and formats.
//
// This package integrates with the broader RuleGo ecosystem, enabling seamless
// data flow from WebSocket messages to rule processing and back to WebSocket responses.
package websocket

import (
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/runtime"
	"github.com/rulego/rulego/utils/str"
)

// Type returns the component type
const Type = types.EndpointTypePrefix + "ws"

// Endpoint alias
type Endpoint = Websocket

// RequestMessage websocket requests messages
type RequestMessage struct {
	//ws message type TextMessage=1 / BinaryMessage=2
	messageType int
	request     *http.Request
	body        []byte
	//Path parameters
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
		//The default specification is JSON format. If it is not this type, please modify it in the process function
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

// ResponseMessage websocket responds to messages
type ResponseMessage struct {
	headers textproto.MIMEHeader
	//ws message type: TextMessage/BinaryMessage
	messageType int
	log         func(format string, v ...interface{})
	request     *http.Request
	sender      *wsSender // Shares wsSender (the same lock as session addressing push), replacing bare conn
	body        []byte
	to          string
	msg         *types.RuleMsg
	err         error
	locker      sync.RWMutex
}

func (r *ResponseMessage) Body() []byte {
	r.locker.RLock()
	defer r.locker.RUnlock()
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
	r.locker.Lock()
	defer r.locker.Unlock()
	r.msg = msg
}

func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.locker.RLock()
	defer r.locker.RUnlock()
	return r.msg
}

// SetStatusCode does not provide a status code
func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.locker.Lock()
	defer r.locker.Unlock()

	r.body = body
	if r.sender != nil {
		if r.messageType == 0 {
			r.messageType = websocket.TextMessage
		}
		// Write via wsSender with locking (sharing the lock with addressed push); Set r.err directly instead of SetError (the latter repeatedly adds a locker deadlock)
		if err := r.sender.SendWithType(body, r.messageType); err != nil {
			r.err = err
		}
	}
}

func (r *ResponseMessage) SetError(err error) {
	r.locker.Lock()
	defer r.locker.Unlock()
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	r.locker.RLock()
	defer r.locker.RUnlock()
	return r.err
}

// Config Websocket service configuration
// Config is the configuration of the ws endpoint, embedded in rest.Config(HTTP server) + ws-exclusive session field.
// Embed rest.Config (squash tiling) allows both reflect forms and Map2Struct to overwrite HTTP fields and session fields simultaneously.
type Config struct {
	rest.Config `json:",squash" mapstructure:",squash"`
	SessionKey  interface{} `json:"sessionKey" label:"Session Key" desc:"会话寻址键，留空用 RemoteAddr。支持 ${} 表达式"`
	SessionTTL  int         `json:"sessionTTL" label:"Session TTL" desc:"会话空闲超时(秒)，<=0 用默认 1800"`
}

// Websocket receives endpoints
type Websocket struct {
	*rest.Rest
	//Configuration
	Config   Config
	Upgrader websocket.Upgrader

	// Session addressing: embedded registry supports wsSend to actively push to connected WS clients by pressing a key
	impl.DefaultSessionRegistry
	// sessionKey extraction rules (rest.Config does not have this field and is stored separately). For example, ${msg.deviceId} / ${metadata.device}
	keyResolver *impl.SessionKeyResolver
}

// Type returns the component type
func (ws *Websocket) Type() string {
	return Type
}

// Category returns the component category
func (ws *Websocket) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (ws *Websocket) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "WebSocket server endpoint: upgrades HTTP requests (GET) to WebSocket and processes incoming frames; shares HTTP server config (server/cert/cors/timeouts) with the rest endpoint",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Path",
					Desc:     "WebSocket upgrade path, e.g. /api/ws (method is fixed to GET)",
					Required: true,
				},
			},
		},
	}
}

func (ws *Websocket) New() types.Node {
	return &Websocket{Config: Config{Config: rest.Config{Server: ":6334", AllowCors: true}, SessionTTL: 1800}}
}

// Init initializes the component
func (ws *Websocket) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &ws.Config)
	if err != nil {
		return err
	}
	ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return ws.Config.AllowCors // All cross-origin requests are allowed
	}
	if ws.Config.SessionTTL <= 0 {
		ws.Config.SessionTTL = 1800 // <=0 Default 30 minutes
	}
	ws.keyResolver = impl.NewSessionKeyResolver(ws.Config.SessionKey)
	ws.Rest = &rest.Rest{}
	if err = ws.Rest.Init(ruleConfig, configuration); err != nil {
		return err
	}
	return err
}

func (ws *Websocket) Id() string {
	return ws.Config.Server
}

// GetInstance returns its own *Websocket, overwriting *rest.Rest.GetInstance (which returns *Rest), for ref:// to fetch instances for session addressing.
func (ws *Websocket) GetInstance() (interface{}, error) { return ws, nil }

// SendToTarget implements types.TargetSender: Push to connected WS clients by target addressing.
// target:userId/deviceId/*(Broadcast)/Empty(Broadcast).
// Allows ref:// of nodes like.NET to address WS Endpoint across protocols (with *net.Net.SendToTarget).
func (ws *Websocket) SendToTarget(target string, data []byte) (sent, failed int, err error) {
	sessions := ws.Lookup(target)
	if len(sessions) == 0 {
		return 0, 0, fmt.Errorf("no session matched target=%q", target)
	}
	var firstErr error
	for _, s := range sessions {
		if e := s.Sender.Send(data); e != nil {
			failed++
			if firstErr == nil {
				firstErr = e
			}
		} else {
			sent++
			s.Touch()
		}
	}
	return sent, failed, firstErr
}

// Destroy: Cleans the session registry (as a backup, all connections are normally disconnected and Removed) + shut down the HTTP server.
func (ws *Websocket) Destroy() {
	ws.StopSweeping()
	ws.Clear()
	ws.Rest.Destroy()
}

func (ws *Websocket) AddRouter(router endpoint.Router, params ...interface{}) (id string, err error) {
	if router == nil {
		return "", errors.New("router can not nil")
	} else {
		defer func() {
			if e := recover(); e != nil {
				err = fmt.Errorf("addRouter err :%v", e)
			}
		}()
		ws.addRouter(router)
		return router.GetId(), err
	}
}

func (ws *Websocket) RemoveRouter(routerId string, params ...interface{}) error {
	routerId = strings.TrimSpace(routerId)
	ws.Lock()
	defer ws.Unlock()
	if ws.RouterStorage != nil {
		if router, ok := ws.RouterStorage[routerId]; ok && !router.IsDisable() {
			router.Disable(true)
			return nil
		} else {
			return fmt.Errorf("router: %s not found", routerId)
		}
	}
	return nil
}
func (ws *Websocket) Printf(format string, v ...interface{}) {
	if ws.RuleConfig.Logger != nil {
		ws.RuleConfig.Logger.Printf(format, v...)
	}
}

func (ws *Websocket) Start() error {
	if ws.OnEvent != nil {
		ws.OnEvent(endpoint.EventInitServer, ws.Rest.Server)
	}
	ws.Upgrader.CheckOrigin = func(r *http.Request) bool {
		return ws.Config.AllowCors // All cross-origin requests are allowed
	}
	if ws.Rest.Started() {
		return nil
	}
	if err := ws.Rest.Start(); err != nil {
		return err
	}
	// Init has guaranteed SessionTTL>0 (<=0 normalized to the default 1800), so no further judgment is needed here
	ttl := time.Duration(ws.Config.SessionTTL) * time.Second
	ws.StartSweeping(ttl, ttl/2)
	return nil
}

// addRouter registers one or more routes
func (ws *Websocket) addRouter(routers ...endpoint.Router) *Websocket {
	ws.Lock()
	defer ws.Unlock()

	if ws.RouterStorage == nil {
		ws.RouterStorage = make(map[string]endpoint.Router)
	}
	for _, item := range routers {
		item.SetParams("GET")
		ws.CheckAndSetRouterId(item)
		//Store the route
		ws.RouterStorage[item.GetId()] = item
		//Add to the HTTP router
		ws.Router().Handle("GET", item.FromToString(), ws.handler(item))
	}

	return ws
}

func (ws *Websocket) handler(router endpoint.Router) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		if router.IsDisable() {
			http.NotFound(w, r)
			return
		}
		c, err := ws.Upgrader.Upgrade(w, r, nil)
		if err != nil {
			ws.Printf("Websocket handler upgrade: %v", err)
			return
		}
		// Shared wsSender: Shares locks for writeback and addressing, preventing crossover of concurrent WriteMessage frames
		sender := &wsSender{conn: c}
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
				sender:  sender,
			}}
		if ws.OnEvent != nil {
			ws.OnEvent(endpoint.EventConnect, connectExchange)
		}

		// Connection establishment: Create and register a session (default Key = RemoteAddr)
		session := endpoint.NewSession(r.RemoteAddr, sender)
		ws.Add(session)

		defer func() {
			ws.Remove(session.Key()) // Disconnection: Cancel session (before c.Close)
			_ = c.Close()
			//Capture anomalies
			if e := recover(); e != nil {
				if ws.OnEvent != nil {
					ws.OnEvent(endpoint.EventDisconnect, connectExchange)
				}
				ws.Printf("ws endpoint handler err :\n%v", runtime.Stack())
			}
		}()

		for {
			mt, message, err := c.ReadMessage()
			session.Touch()
			if err != nil {
				if ws.OnEvent != nil {
					ws.OnEvent(endpoint.EventDisconnect, connectExchange, w, r, params)
				}
				break
			}

			if router.IsDisable() {
				if ws.OnEvent != nil {
					ws.OnEvent(endpoint.EventDisconnect, connectExchange, w, r, params)
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
					sender:      sender,
					messageType: mt,
				}}

			msg := exchange.In.GetMsg()
			//Put the path parameter into the msg metadata
			for _, param := range params {
				msg.Metadata.PutValue(param.Key, param.Value)
			}

			msg.Metadata.PutValue("messageType", strconv.Itoa(mt))

			//Place the url? parameter into the msg metadata
			for key, value := range r.URL.Query() {
				if len(value) > 1 {
					msg.Metadata.PutValue(key, str.ToString(value))
				} else {
					msg.Metadata.PutValue(key, value[0])
				}

			}

			// sessionKey extraction: only performed before resolution (skipped after keyResolved), using SessionKeyResolver(${} expression)
			if !session.IsResolved() && ws.keyResolver != nil {
				if key := ws.keyResolver.Resolve(*msg, message); key != "" {
					ws.Rekey(session, key)
				}
			}

			ws.DoProcess(r.Context(), router, exchange)
		}
	}
}
