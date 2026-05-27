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

package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
)

// ClientType WebSocket客户端组件类型
const ClientType = types.EndpointTypePrefix + "ws_client"

// ClientEndpoint 别名
type ClientEndpoint = WsClient

// WsClientConfig WebSocket客户端配置
type WsClientConfig struct {
	// WebSocket服务器地址，格式 ws://host:port/path 或 wss://host:port/path
	Server string `json:"server" label:"Server URL" desc:"WebSocket server URL, format: ws://host:port/path or wss://host:port/path" required:"true"`

	// 连接时携带的自定义Header
	Headers map[string]string `json:"headers" label:"Headers" desc:"Custom headers to send during connection"`

	// 断线重连间隔，单位为秒，默认5，0表示不重连
	ReconnectInterval int `json:"reconnectInterval" label:"Reconnect Interval" desc:"Reconnect interval in seconds, default 5, 0 means no reconnect"`

	// 心跳发送间隔，单位为秒，0表示不发送
	// Heartbeat send interval in seconds, 0 means no heartbeat
	// 默认发送WebSocket Ping帧；如果设置了HeartbeatData，则发送TextMessage
	// Default sends WebSocket Ping frame; if HeartbeatData is set, sends TextMessage instead
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval" desc:"Heartbeat send interval in seconds, 0 means no heartbeat"`

	// 心跳包内容，仅在HeartbeatInterval > 0时生效
	// Heartbeat packet content, only effective when HeartbeatInterval > 0
	// 空字符串(默认): 发送WebSocket协议级Ping帧
	// Empty string (default): sends WebSocket protocol-level Ping frame
	// 非空字符串: 通过TextMessage发送自定义心跳内容
	// Non-empty string: sends custom heartbeat content via TextMessage
	HeartbeatData string `json:"heartbeatData" label:"Heartbeat Data" desc:"Heartbeat packet content. Empty: send Ping frame. Non-empty: send custom text message"`

	// 是否接收TextMessage，默认true
	AllowText bool `json:"allowText" label:"Allow Text" desc:"Allow receiving TextMessage, default true"`

	// 是否接收BinaryMessage，默认true
	AllowBinary bool `json:"allowBinary" label:"Allow Binary" desc:"Allow receiving BinaryMessage, default true"`
}

// WsClientRequestMessage WebSocket客户端请求消息
type WsClientRequestMessage struct {
	headers     textproto.MIMEHeader
	messageType int
	body        []byte
	msg         *types.RuleMsg
	err         error
	from        string
}

func (r *WsClientRequestMessage) Body() []byte {
	return r.body
}

func (r *WsClientRequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *WsClientRequestMessage) From() string {
	return r.from
}

func (r *WsClientRequestMessage) GetParam(key string) string {
	if r.msg != nil {
		return r.msg.Metadata.GetValue(key)
	}
	return ""
}

func (r *WsClientRequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *WsClientRequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := types.JSON
		if r.messageType == websocket.BinaryMessage {
			dataType = types.BINARY
		}
		ruleMsg := types.NewMsg(0, r.From(), dataType, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

func (r *WsClientRequestMessage) SetStatusCode(statusCode int) {}

func (r *WsClientRequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *WsClientRequestMessage) SetError(err error) {
	r.err = err
}

func (r *WsClientRequestMessage) GetError() error {
	return r.err
}

// WsClientResponseMessage WebSocket客户端响应消息，用于发送数据到服务端
type WsClientResponseMessage struct {
	headers     textproto.MIMEHeader
	messageType int
	log         func(format string, v ...interface{})
	conn        *websocket.Conn
	body        []byte
	from        string
	msg         *types.RuleMsg
	err         error
	mu          sync.RWMutex
	writeMu     *sync.Mutex
}

func (r *WsClientResponseMessage) Body() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.body
}

func (r *WsClientResponseMessage) Headers() textproto.MIMEHeader {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *WsClientResponseMessage) From() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.from
}

func (r *WsClientResponseMessage) GetParam(key string) string {
	return ""
}

func (r *WsClientResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}

func (r *WsClientResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

func (r *WsClientResponseMessage) SetStatusCode(statusCode int) {}

func (r *WsClientResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.body = body
	if r.conn != nil && r.writeMu != nil {
		mt := r.messageType
		if mt == 0 {
			mt = websocket.TextMessage
		}
		r.writeMu.Lock()
		if err := r.conn.WriteMessage(mt, body); err != nil {
			r.err = err
		}
		r.writeMu.Unlock()
	}
}

func (r *WsClientResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *WsClientResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

// WsClient WebSocket客户端端点，连接到远程WebSocket服务器并接收数据
type WsClient struct {
	impl.BaseEndpoint
	Config     WsClientConfig
	RuleConfig types.Config
	conn       *websocket.Conn
	routers    map[string]endpoint.Router
	closed     int32
	mu         sync.RWMutex
	writeMu    sync.Mutex
	OnEvent    func(event string, params ...interface{})
	// OnDial 在每次建立连接前调用，可用于动态设置鉴权Header
	// Called before each connection attempt, can be used to dynamically set auth headers
	// 参数 header: 已包含配置中的静态Headers，可就地修改
	OnDial func(header http.Header) error
	// OnHeartbeat 自定义心跳发送回调，覆盖默认的心跳发送逻辑
	// Custom heartbeat send callback, overrides the default heartbeat logic
	// 如果设置了此回调，HeartbeatData配置将被忽略，完全由回调决定发送内容
	// If this callback is set, HeartbeatData config is ignored, the callback fully controls what to send
	//
	// 参数 / Parameters:
	//   - conn: 当前WebSocket连接 / Current WebSocket connection
	// 返回值 / Returns:
	//   - error: 非nil时停止心跳协程 / Non-nil stops the heartbeat goroutine
	OnHeartbeat func(conn *websocket.Conn) error
}

// Type 组件类型
func (c *WsClient) Type() string {
	return ClientType
}

// Category returns the component category
func (c *WsClient) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (c *WsClient) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "WebSocket client endpoint for connecting to remote WebSocket servers",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Path",
					Desc:     "Route identifier for data routing",
					Required: true,
				},
			},
		},
	}
}

// New 创建默认实例
func (c *WsClient) New() types.Node {
	return &WsClient{
		Config: WsClientConfig{
			ReconnectInterval: 5,
			AllowText:         true,
			AllowBinary:       true,
		},
	}
}

// Init 初始化
func (c *WsClient) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &c.Config); err != nil {
		return err
	}
	// 设置默认值（Map2Struct 使用 ZeroFields:true，会覆盖为零值）
	if c.Config.ReconnectInterval <= 0 {
		c.Config.ReconnectInterval = 5
	}
	// AllowText/AllowBinary 默认应该为true，仅当配置中明确设为false时才为false
	// 由于ZeroFields，未配置时会变成false，需要通过检查配置原始值来判断
	// 简单处理：如果配置中没有明确设置，则默认true
	if configuration != nil {
		if _, ok := configuration["allowText"]; !ok {
			c.Config.AllowText = true
		}
		if _, ok := configuration["allowBinary"]; !ok {
			c.Config.AllowBinary = true
		}
	} else {
		c.Config.AllowText = true
		c.Config.AllowBinary = true
	}
	c.RuleConfig = ruleConfig
	c.Logger = ruleConfig.Logger
	return nil
}

// Destroy 销毁
func (c *WsClient) Destroy() {
	_ = c.Close()
}

// Close 关闭连接
func (c *WsClient) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.writeMu.Lock()
		err := c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.writeMu.Unlock()
		_ = c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Id 返回服务器地址作为标识
func (c *WsClient) Id() string {
	return c.Config.Server
}

// AddRouter 添加路由
func (c *WsClient) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	c.CheckAndSetRouterId(router)
	c.Lock()
	defer c.Unlock()
	if c.routers == nil {
		c.routers = make(map[string]endpoint.Router)
	}
	if _, ok := c.routers[router.GetId()]; ok {
		return router.GetId(), fmt.Errorf("duplicate router %s", router.GetFrom().ToString())
	}
	c.routers[router.GetId()] = router
	return router.GetId(), nil
}

// RemoveRouter 移除路由
func (c *WsClient) RemoveRouter(routerId string, params ...interface{}) error {
	c.Lock()
	defer c.Unlock()
	if c.routers != nil {
		if _, ok := c.routers[routerId]; ok {
			delete(c.routers, routerId)
		} else {
			return fmt.Errorf("router: %s not found", routerId)
		}
	}
	return nil
}

// Printf 日志输出


// Start 连接到远程WebSocket服务器
func (c *WsClient) Start() error {
	return c.connect()
}

// connect 建立连接并开始读取数据
func (c *WsClient) connect() error {
	header := http.Header{}
	for k, v := range c.Config.Headers {
		header.Set(k, v)
	}

	if c.OnDial != nil {
		if err := c.OnDial(header); err != nil {
			return fmt.Errorf("ws client OnDial callback failed: %w", err)
		}
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.Config.Server, header)
	if err != nil {
		return fmt.Errorf("ws client connect to %s failed: %w", c.Config.Server, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.Printf("ws client connected to %s", c.Config.Server)

	if c.OnEvent != nil {
		c.OnEvent(endpoint.EventConnect, conn)
	}

	go c.readLoop(conn)

	if c.Config.HeartbeatInterval > 0 {
		go c.heartbeatLoop(conn)
	}

	return nil
}

// readLoop 读取循环
func (c *WsClient) readLoop(conn *websocket.Conn) {
	defer func() {
		_ = conn.Close()
		if e := recover(); e != nil {
			c.Printf("ws client readLoop panic: %v", e)
		}
	}()

	for {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		mt, message, err := conn.ReadMessage()
		if err != nil {
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}
			c.Printf("ws client read error: %v", err)
			c.tryReconnect()
			return
		}

		// 过滤消息类型
		if mt == websocket.TextMessage && !c.Config.AllowText {
			continue
		}
		if mt == websocket.BinaryMessage && !c.Config.AllowBinary {
			continue
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}

		from := c.Config.Server

		exchange := &endpoint.Exchange{
			In: &WsClientRequestMessage{
				messageType: mt,
				body:        message,
				from:        from,
			},
			Out: &WsClientResponseMessage{
				log: func(format string, v ...interface{}) {
					c.Printf(format, v...)
				},
				conn:        conn,
				messageType: mt,
				from:        from,
				writeMu:     &c.writeMu,
			},
		}

		msg := exchange.In.GetMsg()
		msg.Metadata.PutValue("messageType", strconv.Itoa(mt))

		c.RLock()
		routerSnapshot := make([]endpoint.Router, 0, len(c.routers))
		for _, r := range c.routers {
			routerSnapshot = append(routerSnapshot, r)
		}
		c.RUnlock()
		for _, router := range routerSnapshot {
			c.DoProcess(context.Background(), router, exchange)
		}
	}
}

// heartbeatLoop 心跳发送
func (c *WsClient) heartbeatLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(time.Duration(c.Config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}
		c.mu.RLock()
		currentConn := c.conn
		c.mu.RUnlock()

		if currentConn != nil {
			var err error
			c.writeMu.Lock()
			if c.OnHeartbeat != nil {
				err = c.OnHeartbeat(currentConn)
			} else if c.Config.HeartbeatData != "" {
				err = currentConn.WriteMessage(websocket.TextMessage, []byte(c.Config.HeartbeatData))
			} else {
				err = currentConn.WriteMessage(websocket.PingMessage, nil)
			}
			c.writeMu.Unlock()
			if err != nil {
				c.Printf("ws client heartbeat send failed: %v", err)
				return
			}
		}
	}
}

// tryReconnect 尝试重连
func (c *WsClient) tryReconnect() {
	if c.Config.ReconnectInterval <= 0 {
		return
	}

	if c.OnEvent != nil {
		c.OnEvent(endpoint.EventDisconnect)
	}

	for {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}
		c.Printf("ws client attempting to reconnect to %s in %d seconds...", c.Config.Server, c.Config.ReconnectInterval)
		time.Sleep(time.Duration(c.Config.ReconnectInterval) * time.Second)

		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		if err := c.connect(); err != nil {
			c.Printf("ws client reconnect failed: %v", err)
			continue
		}
		return
	}
}

// Send 发送文本数据到服务器
func (c *WsClient) Send(data []byte) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

// SendBinary 发送二进制数据到服务器
func (c *WsClient) SendBinary(data []byte) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.BinaryMessage, data)
}
