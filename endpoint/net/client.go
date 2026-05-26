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
 * WITHOUT WARRANTIES OR  CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package net

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
)

// ClientType 客户端组件类型
const ClientType = types.EndpointTypePrefix + "net_client"

// ClientEndpoint 别名
type ClientEndpoint = NetClient

// ClientConfig NET客户端配置
// 用于配置TCP/UDP客户端连接远程服务器的参数
// Configuration for TCP/UDP client to connect to a remote server
type ClientConfig struct {
	// 通信协议，支持以下值：
	// Communication protocol, supports the following values:
	// - "tcp"(默认): TCP IPv4/IPv6自适应 / TCP IPv4/IPv6 auto-detection
	// - "tcp4": 仅TCP IPv4 / TCP IPv4 only
	// - "tcp6": 仅TCP IPv6 / TCP IPv6 only
	// - "udp": UDP IPv4/IPv6自适应 / UDP IPv4/IPv6 auto-detection
	// - "udp4": 仅UDP IPv4 / UDP IPv4 only
	// - "udp6": 仅UDP IPv6 / UDP IPv6 only
	// - "unix": Unix域套接字 / Unix domain socket
	// - "unixpacket": Unix域套接字(数据包模式) / Unix domain socket (packet mode)
	Protocol string `json:"protocol" label:"Protocol" desc:"Network protocol: tcp, tcp4, tcp6, udp, udp4, udp6, unix, unixpacket. Default: tcp"`

	// 远程服务器地址，格式为host:port
	// Remote server address, format: host:port
	// 示例 / Examples: "192.168.1.100:8080", "127.0.0.1:1883", "[::1]:8080"
	Server string `json:"server" label:"Server Address" desc:"Remote server address, format: host:port" required:"true"`

	// 连接超时时间，单位为秒，默认5
	// Connection timeout in seconds, default 5
	ConnectTimeout int `json:"connectTimeout" label:"Connect Timeout" desc:"Connection timeout in seconds, default 5"`

	// 读取超时时间，单位为秒，0表示不设置超时
	// Read timeout in seconds, 0 means no timeout
	ReadTimeout int `json:"readTimeout" label:"Read Timeout" desc:"Read timeout in seconds, 0 means no timeout"`

	// 断线重连间隔，单位为秒，默认5，0表示不重连
	// Reconnection interval in seconds, default 5, 0 means no reconnection
	ReconnectInterval int `json:"reconnectInterval" label:"Reconnect Interval" desc:"Reconnection interval in seconds, default 5, 0 means no reconnect"`

	// 数据编解码方式：
	// Data encoding/decoding method:
	// - "hex": 将接收到的二进制数据编码为十六进制字符串，消息数据类型为TEXT
	// - "base64": 将接收到的二进制数据编码为Base64字符串，消息数据类型为TEXT
	// - 其他值(默认): 保持原始二进制数据不变，消息数据类型为BINARY
	Encode string `json:"encode" label:"Encode" desc:"Data encoding: hex (hex string), base64 (base64 string), other (default binary)"`

	// 数据包分割模式：
	// Packet splitting mode:
	// - "line"(默认): 按行分割，以\n或\r\n作为分隔符
	// - "fixed": 固定长度分割，需配合PacketSize使用
	// - "delimiter": 自定义分隔符分割，需配合Delimiter使用
	// - "length_prefix_le": 长度前缀小端序，长度不包含前缀
	// - "length_prefix_be": 长度前缀大端序，长度不包含前缀
	// - "length_prefix_le_inc": 长度前缀小端序，长度包含前缀
	// - "length_prefix_be_inc": 长度前缀大端序，长度包含前缀
	PacketMode string `json:"packetMode" label:"Packet Mode" desc:"Packet splitting mode: line, fixed, delimiter, length_prefix_le, length_prefix_be, length_prefix_le_inc, length_prefix_be_inc"`

	// 数据包大小配置（根据PacketMode含义不同）
	// Packet size configuration (meaning varies by PacketMode)
	// - fixed模式：固定数据包的字节数 / fixed mode: fixed packet byte count
	// - length_prefix*模式：长度前缀的字节数（1-4字节）/ length_prefix* mode: length prefix byte count (1-4 bytes)
	// - 其他模式：此字段无效 / other modes: this field is invalid
	PacketSize int `json:"packetSize" label:"Packet Size" desc:"Packet size configuration (meaning varies by PacketMode)"`

	// 自定义分隔符，仅在PacketMode为"delimiter"时生效
	// Custom delimiter, only effective when PacketMode is "delimiter"
	// 支持普通字符串或十六进制格式（如"0x0D0A"表示\r\n）
	Delimiter string `json:"delimiter" label:"Delimiter" desc:"Custom delimiter, only effective when PacketMode is delimiter. Supports hex format like 0x0D0A"`

	// 最大数据包大小，防止恶意或异常的大数据包，默认64KB
	// Maximum packet size to prevent malicious or abnormal large packets, default 64KB
	MaxPacketSize int `json:"maxPacketSize" label:"Max Packet Size" desc:"Maximum packet size to prevent malicious packets, default 64KB"`

	// 心跳发送间隔，单位为秒，0表示不发送心跳
	// Heartbeat send interval in seconds, 0 means no heartbeat
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval" desc:"Heartbeat send interval in seconds, 0 means no heartbeat"`

	// 心跳包内容，仅在HeartbeatInterval > 0时生效
	// Heartbeat packet content, only effective when HeartbeatInterval > 0
	// 支持普通字符串和十六进制格式（如"0x0D0A"表示\r\n），默认"ping\n"
	HeartbeatData string `json:"heartbeatData" label:"Heartbeat Data" desc:"Heartbeat packet content. Supports hex format like 0x0D0A. Default: ping\\n"`
}

// ClientRequestMessage 客户端请求消息
type ClientRequestMessage struct {
	headers  textproto.MIMEHeader
	body     []byte
	msg      *types.RuleMsg
	err      error
	from     string
	dataType types.DataType
}

func (r *ClientRequestMessage) Body() []byte {
	return r.body
}

func (r *ClientRequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	r.headers.Set(RemoteAddrKey, r.from)
	return r.headers
}

func (r *ClientRequestMessage) From() string {
	return r.from
}

func (r *ClientRequestMessage) GetParam(key string) string {
	return ""
}

func (r *ClientRequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *ClientRequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		dataType := r.dataType
		if dataType == "" {
			dataType = types.BINARY
		}
		var ruleMsg types.RuleMsg
		if dataType == types.BINARY {
			ruleMsg = types.NewMsgFromBytes(0, r.From(), dataType, types.NewMetadata(), r.Body())
		} else {
			ruleMsg = types.NewMsg(0, r.From(), dataType, types.NewMetadata(), string(r.Body()))
		}
		r.msg = &ruleMsg
	}
	return r.msg
}

func (r *ClientRequestMessage) SetStatusCode(statusCode int) {}

func (r *ClientRequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *ClientRequestMessage) SetError(err error) {
	r.err = err
}

func (r *ClientRequestMessage) GetError() error {
	return r.err
}

// ClientResponseMessage 客户端响应消息，用于通过连接发送数据到服务端
type ClientResponseMessage struct {
	headers textproto.MIMEHeader
	conn    net.Conn
	log     func(format string, v ...interface{})
	body    []byte
	from    string
	msg     *types.RuleMsg
	err     error
	mu      sync.RWMutex
}

func (r *ClientResponseMessage) Body() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.body
}

func (r *ClientResponseMessage) Headers() textproto.MIMEHeader {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	r.headers.Set(RemoteAddrKey, r.from)
	return r.headers
}

func (r *ClientResponseMessage) From() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.from
}

func (r *ClientResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ClientResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}

func (r *ClientResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

func (r *ClientResponseMessage) SetStatusCode(statusCode int) {}

func (r *ClientResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.msg != nil && r.msg.GetDataType() == types.JSON {
		if len(body) > 0 && !strings.HasSuffix(string(body), LineBreak) {
			body = append(body, LineBreak...)
		}
	}
	r.body = body
	if r.conn == nil {
		r.err = errors.New("write err: conn is nil")
		return
	}
	if _, err := r.conn.Write(body); err != nil {
		r.err = err
	}
}

func (r *ClientResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *ClientResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

// NetClient NET客户端端点，作为主动连接方连接到远程TCP/UDP服务器并接收数据
// NetClient is a NET client endpoint that actively connects to a remote TCP/UDP server and receives data.
//
// 工作流程 / Workflow:
//  1. 调用 Start() 连接到远程服务器 / Call Start() to connect to the remote server
//  2. 通过路由规则处理接收到的数据 / Process received data through router rules
//  3. 支持通过路由处理器向服务端发送响应数据 / Support sending response data to the server via router processors
//  4. 连接断开时自动重连（如果配置了ReconnectInterval）/ Auto-reconnect on disconnection (if ReconnectInterval is configured)
//
// 使用示例 / Usage example:
//
//	config := engine.NewConfig()
//	client := &NetClient{}
//	client.Init(config, types.Configuration{
//	    "server":            "192.168.1.100:8080",
//	    "protocol":          "tcp",
//	    "packetMode":        "line",
//	    "reconnectInterval": 5,
//	})
//	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
//	    // 处理接收到的数据 / Process received data
//	    return true
//	}).End()
//	client.AddRouter(router)
//	client.Start()
type NetClient struct {
	impl.BaseEndpoint
	// Config 客户端配置 / Client configuration
	Config ClientConfig
	// RuleConfig 规则引擎配置 / Rule engine configuration
	RuleConfig types.Config
	conn       net.Conn
	routers    map[string]*RegexpRouter
	closed     int32
	mu         sync.RWMutex
	// OnEvent 连接状态事件回调函数
	// Connection state event callback function
	// 支持的事件 / Supported events:
	//   - endpoint.EventConnect: 连接成功时触发，参数为 net.Conn / Triggered on successful connection, parameter is net.Conn
	//   - endpoint.EventDisconnect: 连接断开时触发 / Triggered on disconnection
	OnEvent func(event string, params ...interface{})
	// OnHeartbeat 自定义心跳发送回调，覆盖默认的心跳发送逻辑
	// Custom heartbeat send callback, overrides the default heartbeat logic
	// 如果设置了此回调，HeartbeatData配置将被忽略，完全由回调决定发送内容
	// If this callback is set, HeartbeatData config is ignored, the callback fully controls what to send
	//
	// 参数 / Parameters:
	//   - conn: 当前TCP/UDP连接 / Current TCP/UDP connection
	// 返回值 / Returns:
	//   - error: 非nil时停止心跳协程 / Non-nil stops the heartbeat goroutine
	OnHeartbeat func(conn net.Conn) error
}

// Type 返回组件类型标识 "endpoint/net_client"
// Returns the component type identifier "endpoint/net_client"
func (c *NetClient) Type() string {
	return ClientType
}

// Category returns the component category
func (c *NetClient) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (c *NetClient) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "TCP/UDP network client endpoint for connecting to remote servers",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Route Pattern",
					Desc:     "Regex pattern to match incoming data, use * to match all",
					Required: true,
				},
			},
		},
	}
}

// New 创建默认配置的NetClient实例，用于组件注册表创建新实例
// Creates a NetClient instance with default configuration, used by the component registry to create new instances
func (c *NetClient) New() types.Node {
	return &NetClient{
		Config: ClientConfig{
			Protocol:          ProtocolTCP,
			ConnectTimeout:    5,
			ReadTimeout:       0,
			ReconnectInterval: 5,
			PacketMode:        PacketModeLine.String(),
			PacketSize:        2,
			Encode:            "none",
			MaxPacketSize:     DefaultMaxPacketSize,
		},
	}
}

// Init 使用规则引擎配置和组件配置初始化客户端
// Initializes the client with rule engine configuration and component configuration
//
// 参数 / Parameters:
//   - ruleConfig: 规则引擎配置 / Rule engine configuration
//   - configuration: 组件配置键值对 / Component configuration key-value pairs
func (c *NetClient) Init(ruleConfig types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &c.Config); err != nil {
		return err
	}
	if c.Config.Protocol == "" {
		c.Config.Protocol = ProtocolTCP
	}
	if c.Config.PacketMode == "" {
		c.Config.PacketMode = PacketModeLine.String()
	}
	if c.Config.MaxPacketSize <= 0 {
		c.Config.MaxPacketSize = DefaultMaxPacketSize
	}
	if c.Config.ConnectTimeout <= 0 {
		c.Config.ConnectTimeout = 5
	}
	c.RuleConfig = ruleConfig
	c.Logger = ruleConfig.Logger
	return nil
}

// Destroy 销毁客户端，关闭连接并释放资源
// Destroys the client, closes the connection and releases resources
func (c *NetClient) Destroy() {
	_ = c.Close()
}

// Close 关闭客户端连接，停止所有读取和心跳协程
// Closes the client connection, stops all reading and heartbeat goroutines
func (c *NetClient) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Id 返回服务器地址作为端点唯一标识
// Returns the server address as the endpoint unique identifier
func (c *NetClient) Id() string {
	return c.Config.Server
}

// AddRouter 添加路由规则，用于匹配和处理接收到的数据
// Adds a router rule for matching and processing received data
//
// 路由匹配逻辑 / Router matching logic:
//   - 路由的From表达式作为正则表达式与接收到的数据进行匹配
//     The router's From expression is used as a regex to match against received data
//   - 空字符串""、"*"、".*" 匹配所有数据
//     Empty string "", "*", ".*" match all data
//
// 可选参数 / Optional parameters:
//   - *RouterMatchOptions: 路由匹配选项，支持数据长度过滤、数据类型过滤、原始数据匹配等
//     *RouterMatchOptions: Router match options, supports data length filter, data type filter, raw data matching, etc
func (c *NetClient) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	expr := router.GetFrom().ToString()
	var regexpV *regexp.Regexp
	if expr != "" && expr != MatchAll && expr != RouteMatchDotStar {
		if re, err := regexp.Compile(expr); err != nil {
			return "", err
		} else {
			regexpV = re
		}
	}

	var matchOptions *RouterMatchOptions
	if len(params) > 0 {
		if opts, ok := params[0].(*RouterMatchOptions); ok {
			matchOptions = opts
		}
	}

	c.CheckAndSetRouterId(router)
	c.Lock()
	defer c.Unlock()
	if c.routers == nil {
		c.routers = make(map[string]*RegexpRouter)
	}
	if _, ok := c.routers[router.GetId()]; ok {
		return router.GetId(), fmt.Errorf("duplicate router %s", expr)
	}
	c.routers[router.GetId()] = &RegexpRouter{
		router:       router,
		regexp:       regexpV,
		matchOptions: matchOptions,
	}
	return router.GetId(), nil
}

// RemoveRouter 根据路由ID移除已注册的路由规则
// Removes a registered router rule by router ID
func (c *NetClient) RemoveRouter(routerId string, params ...interface{}) error {
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

// Printf 输出日志到规则引擎的日志器
// Outputs log messages to the rule engine's logger


// Start 启动客户端，连接到远程服务器并开始接收数据
// Starts the client, connects to the remote server and begins receiving data
// 连接成功后，根据协议类型启动对应的读取循环协程（TCP使用流式读取，UDP使用数据报读取）
// After successful connection, starts the corresponding read loop goroutine based on protocol type
// (TCP uses streaming read, UDP uses datagram read)
func (c *NetClient) Start() error {
	return c.connect()
}

// connect 建立连接并开始读取数据
func (c *NetClient) connect() error {
	conn, err := net.DialTimeout(c.Config.Protocol, c.Config.Server,
		time.Duration(c.Config.ConnectTimeout)*time.Second)
	if err != nil {
		return fmt.Errorf("net client connect to %s failed: %w", c.Config.Server, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.Printf("net client connected to %s", c.Config.Server)

	if c.OnEvent != nil {
		c.OnEvent(endpoint.EventConnect, c.conn)
	}

	// 只支持TCP协议的流式读取；UDP客户端使用不同的模式
	switch c.Config.Protocol {
	case ProtocolTCP, ProtocolTCP4, ProtocolTCP6, ProtocolUnix, ProtocolUnixPacket:
		go c.readLoop(conn)
	case ProtocolUDP, ProtocolUDP4, ProtocolUDP6:
		go c.readLoopUDP(conn)
	}

	// 启动心跳
	if c.Config.HeartbeatInterval > 0 {
		go c.heartbeatLoop(conn)
	}

	return nil
}

// readLoop TCP读取循环
func (c *NetClient) readLoop(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		if e := recover(); e != nil {
			c.Printf("net client readLoop panic: %v", e)
		}
	}()

	splitter, err := CreatePacketSplitter(Config{
		PacketMode:    c.Config.PacketMode,
		PacketSize:    c.Config.PacketSize,
		Delimiter:     c.Config.Delimiter,
		MaxPacketSize: c.Config.MaxPacketSize,
	})
	if err != nil {
		c.Printf("net client failed to create packet splitter: %v", err)
		return
	}

	readTimeoutDuration := time.Duration(c.Config.ReadTimeout+5) * time.Second

	reader := bufio.NewReader(conn)

	for {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		if c.Config.ReadTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(readTimeoutDuration)); err != nil {
				c.tryReconnect()
				return
			}
		}

		data, err := splitter.ReadPacket(reader)
		if err != nil {
			if c.isClosedConn(err) {
				if atomic.LoadInt32(&c.closed) == 1 {
					return
				}
				c.tryReconnect()
				return
			}
			continue
		}

		if string(data) == PingData {
			continue
		}

		encodedMessage, dataType := encodeData(data, c.Config.Encode)
		from := ""
		if conn.RemoteAddr() != nil {
			from = conn.RemoteAddr().String()
		}

		exchange := &endpoint.Exchange{
			In: &ClientRequestMessage{
				body:     encodedMessage,
				from:     from,
				dataType: dataType,
			},
			Out: &ClientResponseMessage{
				log: func(format string, v ...interface{}) {
					c.Printf(format, v...)
				},
				conn: conn,
				from: from,
			},
		}

		msg := exchange.In.GetMsg()
		msg.Metadata.PutValue(RemoteAddrKey, from)

		c.RLock()
		snapshot := make([]*RegexpRouter, 0, len(c.routers))
		for _, v := range c.routers {
			snapshot = append(snapshot, v)
		}
		c.RUnlock()
		for _, v := range snapshot {
			if v.Match(data, encodedMessage, exchange) {
				c.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}
}

// readLoopUDP UDP读取循环
func (c *NetClient) readLoopUDP(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		if e := recover(); e != nil {
			c.Printf("net client readLoopUDP panic: %v", e)
		}
	}()

	bufferSize := c.Config.MaxPacketSize
	if bufferSize < BufferSize {
		bufferSize = BufferSize
	}
	buffer := make([]byte, bufferSize)

	for {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		n, err := conn.Read(buffer)
		if err != nil {
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}
			c.Printf("net client UDP read error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		msgBuffer := buffer[:n]
		if string(msgBuffer) == PingData {
			continue
		}

		encodedMessage, dataType := encodeData(msgBuffer, c.Config.Encode)
		from := ""
		if conn.RemoteAddr() != nil {
			from = conn.RemoteAddr().String()
		}

		exchange := &endpoint.Exchange{
			In: &ClientRequestMessage{
				body:     encodedMessage,
				from:     from,
				dataType: dataType,
			},
			Out: &ClientResponseMessage{
				log: func(format string, v ...interface{}) {
					c.Printf(format, v...)
				},
				conn: conn,
				from: from,
			},
		}

		msg := exchange.In.GetMsg()
		msg.Metadata.PutValue(RemoteAddrKey, from)

		c.RLock()
		snapshot := make([]*RegexpRouter, 0, len(c.routers))
		for _, v := range c.routers {
			snapshot = append(snapshot, v)
		}
		c.RUnlock()
		for _, v := range snapshot {
			if v.Match(msgBuffer, encodedMessage, exchange) {
				c.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}
}

// heartbeatLoop 心跳发送
func (c *NetClient) heartbeatLoop(conn net.Conn) {
	ticker := time.NewTicker(time.Duration(c.Config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	// 解析心跳数据：优先使用自定义回调，其次使用配置的HeartbeatData，最后使用默认值
	heartbeatData := c.resolveHeartbeatData()

	for range ticker.C {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}
		c.mu.RLock()
		currentConn := c.conn
		c.mu.RUnlock()

		if currentConn != nil {
			var err error
			if c.OnHeartbeat != nil {
				err = c.OnHeartbeat(currentConn)
			} else {
				_, err = currentConn.Write(heartbeatData)
			}
			if err != nil {
				c.Printf("net client heartbeat send failed: %v", err)
				return
			}
		}
	}
}

// resolveHeartbeatData 解析心跳数据内容
func (c *NetClient) resolveHeartbeatData() []byte {
	data := c.Config.HeartbeatData
	if data == "" {
		return []byte(PingData + LineBreak)
	}
	// 支持十六进制格式，如 "0x0D0A"
	decoded, err := decodeHexIfMatch(data)
	if err == nil && decoded != nil {
		return decoded
	}
	return []byte(data)
}

// decodeHexIfMatch 如果字符串以"0x"开头，尝试解码为十六进制字节
func decodeHexIfMatch(s string) ([]byte, error) {
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		return nil, nil
	}
	hexStr := s[2:]
	if len(hexStr)%2 != 0 {
		hexStr = "0" + hexStr
	}
	return hex.DecodeString(hexStr)
}

// tryReconnect 尝试重连
func (c *NetClient) tryReconnect() {
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
		c.Printf("net client attempting to reconnect to %s in %d seconds...", c.Config.Server, c.Config.ReconnectInterval)
		time.Sleep(time.Duration(c.Config.ReconnectInterval) * time.Second)

		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		if err := c.connect(); err != nil {
			c.Printf("net client reconnect failed: %v", err)
			continue
		}
		return
	}
}


// isClosedConn 判断是否是连接关闭错误
func (c *NetClient) isClosedConn(err error) bool {
	if err == io.EOF {
		return true
	}
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err == net.ErrClosed || opErr.Timeout()
	}
	return err.Error() == os.ErrDeadlineExceeded.Error() ||
		strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe")
}

// Send 通过当前连接发送原始字节数据到远程服务器
// Sends raw byte data to the remote server through the current connection
// 注意：此方法不是线程安全的，避免与路由处理器的响应写入并发调用
// Note: This method is not thread-safe, avoid concurrent calls with router processor response writes
func (c *NetClient) Send(data []byte) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("not connected")
	}
	_, err := conn.Write(data)
	return err
}
