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

// ClientType WebSocket client component type
const ClientType = types.EndpointTypePrefix + "ws_client"

// ClientEndpoint alias
type ClientEndpoint = WsClient

// WsClientConfig WebSocket client configuration
type WsClientConfig struct {
	// WebSocket server address, formatted as ws://host:port/path or wss://host:port/path
	Server string `json:"server" label:"Server URL" desc:"WebSocket server URL, format: ws://host:port/path or wss://host:port/path" required:"true"`

	// Custom header carried when connecting
	Headers map[string]string `json:"headers" label:"Headers" desc:"Custom headers to send during connection"`

	// Disconnection interval, measured in seconds, default 5.0 means no reconnection
	ReconnectInterval int `json:"reconnectInterval" label:"Reconnect Interval" desc:"Reconnect interval in seconds, default 5, 0 means no reconnect"`

	// Heartbeat sending interval, measured in seconds, 0 means no transmission
	// Heartbeat send interval in seconds, 0 means no heartbeat
	// By default, WebSocket Ping frames are sent; If HeartbeatData is set, a TextMessage is sent
	// Default sends WebSocket Ping frame; if HeartbeatData is set, sends TextMessage instead
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval" desc:"Heartbeat send interval in seconds, 0 means no heartbeat"`

	// Heartbeat Pack content, effective only at HeartbeatInterval > 0
	// Heartbeat packet content, only effective when HeartbeatInterval > 0
	// Null string (default): Transmits WebSocket protocol-level ping frames
	// Empty string (default): sends WebSocket protocol-level Ping frame
	// Non-empty string: Send custom heartbeat content via TextMessage
	// Non-empty string: sends custom heartbeat content via TextMessage
	HeartbeatData string `json:"heartbeatData" label:"Heartbeat Data" desc:"Heartbeat packet content. Empty: send Ping frame. Non-empty: send custom text message"`

	// Whether to receive TextMessages is true by default
	AllowText bool `json:"allowText" label:"Allow Text" desc:"Allow receiving TextMessage, default true"`

	// Whether to receive BinaryMessage is true by default
	AllowBinary bool `json:"allowBinary" label:"Allow Binary" desc:"Allow receiving BinaryMessage, default true"`
}

// WsClientRequestMessage: WebSocket client request message
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

// WsClientResponseMessage: WebSocket client response messages, used to send data to the server
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

// WsClient WebSocket client endpoint connects to a remote WebSocket server and receives data
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
	// OnDial is called before each connection is established and can be used to dynamically set the authentication header
	// Called before each connection attempt, can be used to dynamically set auth headers
	// Parameter header: Includes the configured static headers and can be modified locally
	OnDial func(header http.Header) error
	// OnHeartbeat customizes heartbeat send callbacks to override the default heartbeat sending logic
	// Custom heartbeat send callback, overrides the default heartbeat logic
	// If this callback is set, the HeartbeatData configuration will be ignored, and the content sent will be determined entirely by the callback
	// If this callback is set, HeartbeatData config is ignored, the callback fully controls what to send
	//
	// Parameters / Parameters:
	//   - conn: Current WebSocket connection / Current WebSocket connection
	// Returns:
	//   - error: Non-nil stops the heartbeat goroutine
	OnHeartbeat func(conn *websocket.Conn) error
}

// Type returns the component type
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
					Desc:     "Route key only; no matching is performed — every received frame is delivered to all routers, e.g. default",
					Required: true,
				},
			},
		},
	}
}

// New creates the default instance
func (c *WsClient) New() types.Node {
	return &WsClient{
		Config: WsClientConfig{
			ReconnectInterval: 5,
			AllowText:         true,
			AllowBinary:       true,
		},
	}
}

// Init initializes the component
func (c *WsClient) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &c.Config); err != nil {
		return err
	}
	// Set the default value (Map2Struct uses ZeroFields:true to overwrite the zero value)
	if c.Config.ReconnectInterval <= 0 {
		c.Config.ReconnectInterval = 5
	}
	// AllowText/AllowBinary should be true by default, only false if explicitly set to false in the configuration
	// Because of ZeroFields, it will appear false when not configured. You need to check the original configuration value to determine this
	// Simple handling: If there is no explicit setting in the configuration, default is true
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

// Destroy releases resources
func (c *WsClient) Destroy() {
	_ = c.Close()
}

// Close Close closes the connection
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

// Id returns the server address as an identifier
func (c *WsClient) Id() string {
	return c.Config.Server
}

// AddRouter adds a route
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

// RemoveRouter removes the route
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

// Printf log output

// Start connects to a remote WebSocket server
func (c *WsClient) Start() error {
	return c.connect()
}

// connect: establish a connection and start reading data
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

// readLoop reads the loop
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

		// Filter message types
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

// heartbeatLoop Heartbeat sends
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

// tryReconnect attempts to reconnect
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

// Send: Send text data to the server
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

// SendBinary sends binary data to the server
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
