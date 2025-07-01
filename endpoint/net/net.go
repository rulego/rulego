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

// Package net provides a network endpoint implementation for the RuleGo framework.
// It allows creating TCP/UDP servers that can receive and process incoming network messages,
// routing them to appropriate rule chains or components for further processing.
//
// Key components in this package include:
// - Endpoint (alias Net): Implements the network server and message handling
// - RequestMessage: Represents an incoming network message
// - ResponseMessage: Represents the network message to be sent back
//
// The network endpoint supports dynamic routing configuration, allowing users to
// define message patterns and their corresponding rule chain or component destinations.
// It also provides flexibility in handling different network protocols and message formats.
//
// This package integrates with the broader RuleGo ecosystem, enabling seamless
// data flow from network messages to rule processing and back to network responses.
package net

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/rulego/rulego/utils/runtime"
)

const (
	// Type 组件类型
	Type = types.EndpointTypePrefix + "net"
	// RemoteAddrKey 远程地址键
	RemoteAddrKey = "remoteAddr"
	// PingData 心跳数据
	PingData = "ping"
	// MatchAll 匹配所有数据
	MatchAll = "*"
	// BufferSize 假设缓冲区大小为1024字节
	BufferSize = 1024
	// LineBreak JSON消息行分隔符
	LineBreak = "\n"

	// DefaultMaxPacketSize 默认最大数据包大小(64KB)
	DefaultMaxPacketSize = 65536

	// 协议常量
	ProtocolTCP        = "tcp"
	ProtocolTCP4       = "tcp4"
	ProtocolTCP6       = "tcp6"
	ProtocolUDP        = "udp"
	ProtocolUDP4       = "udp4"
	ProtocolUDP6       = "udp6"
	ProtocolUnix       = "unix"
	ProtocolUnixPacket = "unixpacket"

	// 编码模式常量
	EncodeHex    = "hex"
	EncodeBase64 = "base64"

	// 十六进制前缀常量
	HexPrefix   = "0x"
	HexPrefixUp = "0X"

	// PacketMode解析用常量
	BigEndianSuffix      = "_be"
	IncludesPrefixSuffix = "_inc"

	// 特殊路由匹配常量
	RouteMatchDotStar = ".*"
)

// Endpoint 别名
type Endpoint = Net

// RequestMessage 请求消息
type RequestMessage struct {
	headers  textproto.MIMEHeader
	conn     net.Conn
	body     []byte
	msg      *types.RuleMsg
	err      error
	from     string
	dataType types.DataType // 添加数据类型字段
}

func (r *RequestMessage) Body() []byte {
	return r.body
}

func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	if r.conn != nil {
		r.headers.Set(RemoteAddrKey, r.From())
	}
	return r.headers
}

// From 返回客户端Addr
func (r RequestMessage) From() string {
	return r.from
}

func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

// GetMsg returns the RuleMsg associated with this request.
// If no message exists, creates a new one with the request data.
//
// GetMsg 返回与此请求关联的 RuleMsg。
// 如果不存在消息，则使用请求数据创建新消息。
//
// Data Type Handling:
// 数据类型处理：
//
// By default, all network data is treated as BINARY type to preserve data integrity.
// This ensures that binary protocols, raw sensor data, and any byte sequences are
// handled correctly without character encoding issues.
//
// 默认情况下，所有网络数据都被视为 BINARY 类型以保持数据完整性。
// 这确保二进制协议、原始传感器数据和任何字节序列都能正确处理，不会出现字符编码问题。
//
// Changing Data Type with Processors:
// 使用处理器更改数据类型：
//
// The data type can be changed using built-in processors to optimize downstream
// component processing. Use processors in router configuration:
// 可以使用内置处理器更改数据类型以优化下游组件处理。在路由配置中使用处理器：
//
//	router := impl.NewRouter().From("").
//	  Process("setJsonDataType").   // Changes to JSON type
//	  To("chain:jsonProcessor").End()
//
// Available data type processors:
// 可用的数据类型处理器：
//   - setJsonDataType: For JSON protocols and REST APIs
//     用于 JSON 协议和 REST API
//   - setTextDataType: For text-based protocols like HTTP, SMTP, etc.
//     用于基于文本的协议，如 HTTP、SMTP 等
//   - setBinaryDataType: For binary protocols (default, explicit setting)
//     用于二进制协议（默认，显式设置）
//
// Protocol-Specific Recommendations:
// 协议特定建议：
//   - IoT sensors: Keep BINARY for raw data integrity
//     物联网传感器：保持 BINARY 以确保原始数据完整性
//   - JSON APIs: Use setJsonDataType processor
//     JSON API：使用 setJsonDataType 处理器
//   - Text protocols: Use setTextDataType processor
//     文本协议：使用 setTextDataType 处理器
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		// 使用实际的数据类型，如果未设置则默认为BINARY（网络数据默认为二进制类型）
		// Use the actual data type, default to BINARY if not set (network data defaults to binary type)
		dataType := r.dataType
		if dataType == "" {
			dataType = types.BINARY
		}

		// 根据数据类型决定如何创建消息
		// Decide how to create the message based on data type
		var ruleMsg types.RuleMsg
		if dataType == types.BINARY {
			// 对于二进制数据，使用 NewMsgFromBytes 避免字符串转换
			// For binary data, use NewMsgFromBytes to avoid string conversion
			ruleMsg = types.NewMsgFromBytes(0, r.From(), dataType, types.NewMetadata(), r.Body())
		} else {
			// 对于文本和JSON数据，可以安全地转换为字符串
			// For text and JSON data, it's safe to convert to string
			ruleMsg = types.NewMsg(0, r.From(), dataType, types.NewMetadata(), string(r.Body()))
		}
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode 不提供设置响应状态码
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

func (r *RequestMessage) Conn() net.Conn {
	return r.conn
}

// ResponseMessage 响应消息
type ResponseMessage struct {
	headers textproto.MIMEHeader
	conn    net.Conn
	log     func(format string, v ...interface{})
	body    []byte
	msg     *types.RuleMsg
	err     error
	udpAddr *net.UDPAddr
	from    string
	mu      sync.RWMutex
}

func (r *ResponseMessage) Body() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	if r.conn != nil {
		r.headers.Set(RemoteAddrKey, r.from)
	}
	return r.headers
}

func (r *ResponseMessage) From() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.from
}

func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.msg != nil && r.msg.GetDataType() == types.JSON {
		// 检查JSON数据是否以换行符结尾，如果没有则添加
		if len(body) > 0 && !strings.HasSuffix(string(body), LineBreak) {
			body = append(body, LineBreak...)
		}
		r.body = body
	} else {
		r.body = body
	}
	if r.conn == nil {
		r.err = errors.New("write err: conn is nil")
		return
	}
	if r.udpAddr != nil {
		if udpConn, ok := r.conn.(*net.UDPConn); ok {
			if _, err := udpConn.WriteToUDP(body, r.udpAddr); err != nil {
				r.err = err
			}
		} else {
			r.err = errors.New("write err: conn is not udp")
		}
	} else {
		if _, err := r.conn.Write(body); err != nil {
			r.err = err
		}
	}
}

func (r *ResponseMessage) SetError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

// Config endpoint组件的配置
// Configuration for the NET endpoint component that creates TCP/UDP servers
// for receiving and processing network messages through the RuleGo framework.
//
// Config NET端点组件的配置，用于创建 TCP/UDP 服务器
// 通过 RuleGo 框架接收和处理网络消息。
//
// Data Type Processing:
// 数据类型处理：
//
// By default, all incoming network data is treated as BINARY type. This can be
// changed using built-in processors from the processor package:
// 默认情况下，所有传入的网络数据都被视为 BINARY 类型。这可以通过
// processor 包中的内置处理器进行更改：
//
//   - processor.InBuiltins.Get("setJsonDataType"): Sets data type to JSON
//     设置数据类型为 JSON
//   - processor.InBuiltins.Get("setTextDataType"): Sets data type to TEXT
//     设置数据类型为 TEXT
//   - processor.InBuiltins.Get("setBinaryDataType"): Sets data type to BINARY
//     设置数据类型为 BINARY
//
// Packet Splitting Modes:
// 数据包分割模式：
//
// The endpoint supports multiple packet splitting strategies to handle different
// network protocols and data formats:
// 端点支持多种数据包分割策略以处理不同的网络协议和数据格式：
//
//   - "line": Split by newline characters (\n or \r\n) - default mode
//     按换行符分割（\n 或 \r\n）- 默认模式
//   - "fixed": Split by fixed byte length
//     按固定字节长度分割
//   - "delimiter": Split by custom delimiter (supports hex format)
//     按自定义分隔符分割（支持十六进制格式）
//   - "length_prefix_*": Split by length prefix with various endianness options
//     按长度前缀分割，支持各种字节序选项
//
// Router Configuration Best Practices:
// 路由配置最佳实践：
//
// It's recommended to use a single default router that matches all messages,
// and handle routing logic within rule chains for better maintainability:
// 建议使用匹配所有消息的单个默认路由，并在规则链中处理路由逻辑以获得更好的可维护性：
//
//	router := impl.NewRouter().From("").To("chain:main").End()
//	ep.AddRouter(router)
//
// Advanced routing with processors can be configured like:
// 可以配置带处理器的高级路由，如：
//
//	router := impl.NewRouter().From("").
//		Process("setJsonDataType").
//		To("chain:jsonProcessor").End()
type Config struct {
	// 通信协议，可以是tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。默认tcp协议
	// Network protocol: tcp, udp, ip4:1, ip6:ipv6-icmp, ip6:58, unix, unixgram, and other protocol types supported by the net package. Default: tcp
	Protocol string

	// 服务器的地址，格式为host:port
	// Server address in host:port format
	Server string

	// 读取超时，用于设置读取数据的超时时间，单位为秒，可以为0表示不设置超时
	// Read timeout for setting data read timeout in seconds, can be 0 for no timeout
	ReadTimeout int

	// 编解码 转16进制字符串(hex)、转base64字符串(base64)、其他
	// ⚠️  该字段计划在未来版本中弃用，建议在规则链中使用 jsTransform 等组件处理数据编码
	// ⚠️  规则链（如 jsTransform、luaTransform）具备更强的二进制数据处理能力
	// ⚠️  This field is planned for deprecation in future versions. Use jsTransform or other components in rule chains for data encoding
	// ⚠️  Rule chains (like jsTransform, luaTransform) have stronger binary data processing capabilities
	Encode string

	// 数据包分割模式：
	// Packet splitting mode:
	// "line": 按行分割（默认模式，以\n或\r\n分割）/ Split by line (default mode, split by \n or \r\n)
	// "fixed": 固定长度分割 / Fixed length splitting
	// "delimiter": 自定义分隔符分割 / Custom delimiter splitting
	// "length_prefix_le": 长度前缀小端序，长度不包含前缀 / Length prefix little endian, length excludes prefix
	// "length_prefix_be": 长度前缀大端序，长度不包含前缀 / Length prefix big endian, length excludes prefix
	// "length_prefix_le_inc": 长度前缀小端序，长度包含前缀 / Length prefix little endian, length includes prefix
	// "length_prefix_be_inc": 长度前缀大端序，长度包含前缀 / Length prefix big endian, length includes prefix
	PacketMode string `json:"packetMode"`

	// PacketSize 数据包大小配置（根据PacketMode含义不同）
	// PacketSize configuration (meaning varies by PacketMode)
	// - fixed模式：固定数据包的字节数 / fixed mode: fixed packet byte count
	// - length_prefix*模式：长度前缀的字节数（1-4字节）/ length_prefix* mode: length prefix byte count (1-4 bytes)
	// - 其他模式：此字段无效 / other modes: this field is invalid
	PacketSize int `json:"packetSize"`

	// 自定义分隔符模式：分隔符字节序列（支持十六进制格式如"0x0A"表示\n）
	// Custom delimiter mode: delimiter byte sequence (supports hex format like "0x0A" for \n)
	Delimiter string `json:"delimiter"`

	// 最大数据包大小，防止恶意数据包，默认64KB
	// Maximum packet size to prevent malicious packets, default 64KB
	MaxPacketSize int `json:"maxPacketSize"`
}

// RegexpRouter 正则表达式路由
type RegexpRouter struct {
	//路由ID
	id string
	//路由
	router endpoint.Router
	//正则表达式
	regexp *regexp.Regexp
	//路由匹配选项
	matchOptions *RouterMatchOptions
}

// RouterMatchOptions 路由匹配选项
type RouterMatchOptions struct {
	// 匹配原始数据而非编码数据
	MatchRawData bool `json:"matchRawData"`
	// 数据类型过滤器：TEXT, BINARY, JSON 等
	DataTypeFilter string `json:"dataTypeFilter"`
	// 最小数据长度
	MinDataLength int `json:"minDataLength"`
	// 最大数据长度
	MaxDataLength int `json:"maxDataLength"`
}

// Net net endpoint组件
// 支持通过正则表达式把匹配的消息路由到指定路由
//
// 路由使用建议：
// ⚠️  不建议使用多路由功能，推荐只添加一个默认路由（使用空字符串、"*" 或 ".*" 匹配所有消息）
// ⚠️  将路由逻辑放在规则链中处理，这样更灵活且便于维护
// ⚠️  多路由匹配功能在未来版本中可能会被弃用
//
// 推荐用法：
//
//	router := impl.NewRouter().From("").To("chain:main").End()
//	ep.AddRouter(router)
//
// 不推荐用法：
//
//	router1 := impl.NewRouter().From("^sensor.*").To("chain:sensor").End()
//	router2 := impl.NewRouter().From("^device.*").To("chain:device").End()
//	// 应该在规则链中使用 msgTypeSwitch 或 jsFilter 等组件进行路由
type Net struct {
	// 嵌入endpoint.BaseEndpoint，继承其方法
	impl.BaseEndpoint
	// 配置
	Config Config
	// rulego配置
	RuleConfig types.Config
	// 服务器监听器对象
	listener net.Listener
	// udp conn
	udpConn *net.UDPConn
	// 路由映射表
	routers map[string]*RegexpRouter
	closed  int32 // 使用int32类型支持原子操作，0表示未关闭，1表示已关闭
}

// Type 组件类型
func (ep *Net) Type() string {
	return Type
}

func (ep *Net) New() types.Node {
	return &Net{
		Config: Config{
			Protocol:      ProtocolTCP,
			ReadTimeout:   60,
			Server:        ":6335",
			PacketMode:    PacketModeLine.String(), // 默认按行分割保持向后兼容
			PacketSize:    2,
			Encode:        "none",
			MaxPacketSize: DefaultMaxPacketSize, // 默认64KB最大包大小
		},
	}
}

// Init 初始化
func (ep *Net) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// 将配置转换为EndpointConfiguration结构体
	err := maps.Map2Struct(configuration, &ep.Config)
	if ep.Config.Protocol == "" {
		ep.Config.Protocol = ProtocolTCP
	}
	if ep.Config.PacketMode == "" {
		ep.Config.PacketMode = PacketModeLine.String()
	}
	if ep.Config.MaxPacketSize <= 0 {
		ep.Config.MaxPacketSize = DefaultMaxPacketSize
	}
	ep.RuleConfig = ruleConfig
	return err
}

// Destroy 销毁
func (ep *Net) Destroy() {
	_ = ep.Close()
}

func (ep *Net) Close() error {
	atomic.StoreInt32(&ep.closed, 1)
	if ep.listener != nil {
		err := ep.listener.Close()
		ep.listener = nil
		return err
	}
	if ep.udpConn != nil {
		err := ep.udpConn.Close()
		ep.udpConn = nil
		return err
	}
	return nil
}

func (ep *Net) Id() string {
	return ep.Config.Server
}

// AddRouter 添加路由规则
//
// ⚠️  不建议使用多个路由，推荐只添加一个默认路由匹配所有消息
// ⚠️  路由表达式支持特殊值：空字符串("")、"*" 或 ".*" 将匹配所有数据
// ⚠️  建议将复杂的路由逻辑放在规则链中处理
//
// 参数：
//   - router: 路由规则
//   - params: 可选参数，第一个参数可以是 *RouterMatchOptions 用于高级匹配
//
// 返回：
//   - 路由ID和错误信息
func (ep *Net) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	} else {
		expr := router.GetFrom().ToString()
		//允许空expr，表示匹配所有
		var regexpV *regexp.Regexp
		// 特殊路由表达式不创建正则表达式，在matchesRouter中通过regexp==nil判断
		if expr != "" && expr != MatchAll && expr != RouteMatchDotStar {
			//编译表达式
			if re, err := regexp.Compile(expr); err != nil {
				return "", err
			} else {
				regexpV = re
			}
		}

		// 解析路由匹配选项
		var matchOptions *RouterMatchOptions
		if len(params) > 0 {
			if opts, ok := params[0].(*RouterMatchOptions); ok {
				matchOptions = opts
			}
		}

		ep.CheckAndSetRouterId(router)
		ep.Lock()
		defer ep.Unlock()
		if ep.routers == nil {
			ep.routers = make(map[string]*RegexpRouter)
		}
		if _, ok := ep.routers[router.GetId()]; ok {
			return router.GetId(), fmt.Errorf("duplicate router %s", expr)
		} else {
			ep.routers[router.GetId()] = &RegexpRouter{
				router:       router,
				regexp:       regexpV,
				matchOptions: matchOptions,
			}
			return router.GetId(), nil
		}

	}
}

func (ep *Net) RemoveRouter(routerId string, params ...interface{}) error {
	ep.Lock()
	defer ep.Unlock()
	if ep.routers != nil {
		if _, ok := ep.routers[routerId]; ok {
			delete(ep.routers, routerId)
		} else {
			return fmt.Errorf("router: %s not found", routerId)
		}
	}
	return nil
}
func (ep *Net) Start() error {
	var err error
	// 根据配置的协议和地址，创建一个服务器监听器
	switch ep.Config.Protocol {

	case ProtocolTCP, ProtocolTCP4, ProtocolTCP6, ProtocolUnix, ProtocolUnixPacket:
		ep.listener, err = net.Listen(ep.Config.Protocol, ep.Config.Server)
		if err != nil {
			return err
		}
		ep.Printf("started TCP server on %s", ep.Config.Server)
		go ep.acceptTCPConnections()
	case ProtocolUDP, ProtocolUDP4, ProtocolUDP6:
		err = ep.listenUDP()
		if err != nil {
			return err
		}
		ep.Printf("started UDP server on %s", ep.Config.Server)
		h := UDPHandler{
			endpoint: ep,
			config:   ep.Config,
		}
		ep.submitTask(h.handler)
	default:
		return fmt.Errorf("unsupported protocol: %s", ep.Config.Protocol)
	}
	return nil
}

func (ep *Net) listenUDP() error {
	udpAddr, err := net.ResolveUDPAddr(ep.Config.Protocol, ep.Config.Server)
	if err != nil {
		return err
	}
	ep.udpConn, err = net.ListenUDP(ep.Config.Protocol, udpAddr)
	if err != nil {
		return err
	}
	return nil
}

func (ep *Net) acceptTCPConnections() {
	// 循环接受客户端的连接请求
	for {
		// 检查是否已关闭，避免数据竞争
		if atomic.LoadInt32(&ep.closed) == 1 {
			ep.Printf("net endpoint stop")
			return
		}

		// 获取监听器引用，避免在Close()过程中访问nil指针
		listener := ep.listener
		if listener == nil {
			ep.Printf("net endpoint stop - listener is nil")
			return
		}

		// 从监听器中获取一个客户端连接，返回连接对象和错误信息
		conn, err := listener.Accept()
		if err != nil {
			if opError, ok := err.(*net.OpError); ok && opError.Err == net.ErrClosed {
				ep.Printf("net endpoint stop")
				return
				//return endpoint.ErrServerStopped
			} else {
				ep.Printf("accept:", err)
				continue
			}
		}

		// 再次检查关闭状态，防止在Accept()期间被关闭
		if atomic.LoadInt32(&ep.closed) == 1 {
			_ = conn.Close()
			ep.Printf("net endpoint stop - closing accepted connection")
			return
		}

		// 打印客户端连接的信息
		//ep.Printf("new connection from:", conn.RemoteAddr().String())
		h := TcpHandler{
			endpoint: ep,
			conn:     conn,
			config:   ep.Config,
		}
		// 启动一个协端处理客户端连接
		ep.submitTask(h.handler)
		//go ep.handler(conn)
	}
}

func (ep *Net) submitTask(fn func()) {
	if ep.RuleConfig.Pool != nil {
		err := ep.RuleConfig.Pool.Submit(fn)
		if err != nil {
			ep.Printf("redis consumer handler err :%v", err)
		}
	} else {
		go fn()
	}
}

func (ep *Net) Printf(format string, v ...interface{}) {
	if ep.RuleConfig.Logger != nil {
		ep.RuleConfig.Logger.Printf(format, v...)
	}
}

// encode 对数据进行编码处理并返回编码后的数据和对应的数据类型
// ⚠️  该方法计划在未来版本中弃用，建议在规则链中处理数据编码
func (ep *Net) encode(src []byte) ([]byte, types.DataType) {
	// 编码处理
	var encodedMessage []byte
	var dataType types.DataType

	switch strings.ToLower(ep.Config.Encode) {
	case EncodeHex:
		encodedMessage = make([]byte, hex.EncodedLen(len(src)))
		hex.Encode(encodedMessage, src)
		dataType = types.TEXT // 十六进制编码后为文本
	case EncodeBase64:
		encodedMessage = make([]byte, base64.StdEncoding.EncodedLen(len(src)))
		base64.StdEncoding.Encode(encodedMessage, src)
		dataType = types.TEXT // Base64编码后为文本
	default:
		encodedMessage = src
		// 网络数据默认为二进制类型
		dataType = types.BINARY
	}
	return encodedMessage, dataType
}

func (ep *Net) handler(conn net.Conn) {
	h := TcpHandler{
		endpoint: ep,
		conn:     conn,
	}
	h.handler()
}

type TcpHandler struct {
	endpoint *Net
	// 客户端连接对象
	conn net.Conn
	// 创建一个读取超时定时器，用于设置读取数据的超时时间，可以为0表示不设置超时
	readTimeoutTimer *time.Timer
	//读取数据配置
	config Config
	// 数据包分割器
	splitter PacketSplitter
}

func (x *TcpHandler) handler() {
	defer func() {
		_ = x.conn.Close()
		//捕捉异常
		if e := recover(); e != nil {
			x.endpoint.Printf("net endpoint handler err :\n%v", runtime.Stack())
		}
	}()

	// 创建数据包分割器
	splitter, err := CreatePacketSplitter(x.endpoint.Config)
	if err != nil {
		x.endpoint.Printf("failed to create packet splitter: %v", err)
		return
	}
	x.splitter = splitter

	readTimeoutDuration := time.Duration(x.endpoint.Config.ReadTimeout+5) * time.Second
	//读超时，断开连接
	x.readTimeoutTimer = time.AfterFunc(readTimeoutDuration, func() {
		if x.endpoint.Config.ReadTimeout > 0 {
			x.onDisconnect()
		}
	})
	// 创建一个缓冲读取器，用于读取客户端发送的数据
	reader := bufio.NewReader(x.conn)
	// 循环读取客户端发送的数据
	for {
		// 设置读取超时
		if x.endpoint.Config.ReadTimeout > 0 {
			err := x.conn.SetReadDeadline(time.Now().Add(readTimeoutDuration))
			if err != nil {
				x.onDisconnect()
				break
			}
		}

		// 使用数据包分割器读取数据
		data, err := x.splitter.ReadPacket(reader)

		if err != nil && err.Error() != os.ErrDeadlineExceeded.Error() {
			if e, ok := err.(*net.OpError); ok {
				if e.Err != os.ErrDeadlineExceeded {
					x.onDisconnect()
					break
				} else {
					continue
				}
			} else {
				x.onDisconnect()
				break
			}
		}
		//重置读超时定时器
		if x.endpoint.Config.ReadTimeout > 0 {
			x.readTimeoutTimer.Reset(readTimeoutDuration)
		}
		if string(data) == PingData {
			continue
		}
		// 编码处理
		encodedMessage, dataType := x.endpoint.encode(data)

		from := ""
		if x.conn.RemoteAddr() != nil {
			from = x.conn.RemoteAddr().String()
		}
		// 创建一个交换对象，用于存储输入和输出的消息
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				conn:     x.conn,
				body:     encodedMessage,
				from:     from,
				dataType: dataType, // 设置正确的数据类型
			},
			Out: &ResponseMessage{
				log: func(format string, v ...interface{}) {
					x.endpoint.Printf(format, v...)
				},
				conn: x.conn,
				from: from,
			}}

		msg := exchange.In.GetMsg()
		// 把客户端连接的地址放到msg元数据中
		msg.Metadata.PutValue(RemoteAddrKey, from)

		// 匹配符合的路由，处理消息
		for _, v := range x.endpoint.routers {
			if x.matchesRouter(v, data, encodedMessage, exchange) {
				x.endpoint.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}

}

// matchesRouter 检查数据是否匹配指定的路由
func (x *TcpHandler) matchesRouter(router *RegexpRouter, rawData, encodedData []byte, exchange *endpoint.Exchange) bool {
	// 获取匹配选项
	opts := router.matchOptions
	if opts == nil {
		// 如果没有正则表达式，表示匹配所有数据（特殊路由）
		if router.regexp == nil {
			return true
		}
		// 使用正则匹配逻辑
		return router.regexp.Match(encodedData)
	}

	// 数据长度检查
	dataLen := len(rawData)
	if opts.MinDataLength > 0 && dataLen < opts.MinDataLength {
		return false
	}
	if opts.MaxDataLength > 0 && dataLen > opts.MaxDataLength {
		return false
	}

	// 数据类型过滤
	if opts.DataTypeFilter != "" {
		msg := exchange.In.GetMsg()
		if strings.ToUpper(opts.DataTypeFilter) != strings.ToUpper(string(msg.GetDataType())) {
			return false
		}
	}

	// 选择匹配的数据：原始数据或编码数据
	var dataToMatch []byte
	if opts.MatchRawData {
		dataToMatch = rawData
	} else {
		dataToMatch = encodedData
	}

	// 正则表达式匹配
	return router.regexp == nil || router.regexp.Match(dataToMatch)
}

func (x *TcpHandler) onDisconnect() {
	if x.conn != nil {
		_ = x.conn.Close()
	}
	if x.readTimeoutTimer != nil {
		x.readTimeoutTimer.Stop()
	}
	if x.conn.RemoteAddr() != nil {
		x.endpoint.Printf("onDisconnect:" + x.conn.RemoteAddr().String())
	}
}

type UDPHandler struct {
	endpoint *Net
	// 创建一个读取超时定时器，用于设置读取数据的超时时间，可以为0表示不设置超时
	readTimeoutTimer *time.Timer
	//读取数据配置
	config Config
}

func (x *UDPHandler) handler() {
	// UDP使用配置的最大包大小，但不小于原来的BufferSize
	bufferSize := x.endpoint.Config.MaxPacketSize
	if bufferSize < BufferSize {
		bufferSize = BufferSize
	}
	buffer := make([]byte, bufferSize)

	for {
		if x.endpoint.udpConn == nil || atomic.LoadInt32(&x.endpoint.closed) == 1 {
			break
		}
		n, addr, err := x.endpoint.udpConn.ReadFromUDP(buffer)
		if err != nil {
			time.Sleep(time.Second)
			if atomic.LoadInt32(&x.endpoint.closed) == 1 {
				break
			}
			err = x.endpoint.listenUDP()
			if err != nil {
				x.endpoint.Printf("Error listenUDP: %v", err)
				time.Sleep(time.Second)
			}
			continue
		}

		msgBuffer := buffer[:n]
		if string(msgBuffer) == PingData {
			continue
		}

		// 检查包大小限制
		if len(msgBuffer) > x.endpoint.Config.MaxPacketSize {
			x.endpoint.Printf("UDP packet too large: %d > %d from %s", len(msgBuffer), x.endpoint.Config.MaxPacketSize, addr)
			continue
		}

		from := ""
		if addr != nil {
			from = addr.String()
		}
		// 编码处理
		encodedMessage, dataType := x.endpoint.encode(msgBuffer)

		// 创建一个交换对象，用于存储输入和输出的消息
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				conn:     x.endpoint.udpConn,
				body:     encodedMessage,
				from:     from,
				dataType: dataType, // 设置正确的数据类型
			},
			Out: &ResponseMessage{
				log: func(format string, v ...interface{}) {
					x.endpoint.Printf(format, v...)
				},
				conn:    x.endpoint.udpConn,
				udpAddr: addr,
				from:    from,
			}}

		msg := exchange.In.GetMsg()
		// 把客户端连接的地址放到msg元数据中
		msg.Metadata.PutValue(RemoteAddrKey, from)

		// 匹配符合的路由，处理消息
		for _, v := range x.endpoint.routers {
			if x.matchesRouter(v, msgBuffer, encodedMessage, exchange) {
				x.endpoint.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}
}

// matchesRouter 检查数据是否匹配指定的路由（UDP版本）
func (x *UDPHandler) matchesRouter(router *RegexpRouter, rawData, encodedData []byte, exchange *endpoint.Exchange) bool {
	// 获取匹配选项
	opts := router.matchOptions
	if opts == nil {
		// 如果没有正则表达式，表示匹配所有数据（特殊路由）
		if router.regexp == nil {
			return true
		}
		// 使用正则匹配逻辑（向后兼容）
		return router.regexp.Match(encodedData)
	}

	// 数据长度检查
	dataLen := len(rawData)
	if opts.MinDataLength > 0 && dataLen < opts.MinDataLength {
		return false
	}
	if opts.MaxDataLength > 0 && dataLen > opts.MaxDataLength {
		return false
	}

	// 数据类型过滤
	if opts.DataTypeFilter != "" {
		msg := exchange.In.GetMsg()
		if strings.ToUpper(opts.DataTypeFilter) != strings.ToUpper(string(msg.GetDataType())) {
			return false
		}
	}

	// 选择匹配的数据：原始数据或编码数据
	var dataToMatch []byte
	if opts.MatchRawData {
		dataToMatch = rawData
	} else {
		dataToMatch = encodedData
	}

	// 正则表达式匹配
	return router.regexp == nil || router.regexp.Match(dataToMatch)
}
