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

// ClientType is the type of client component
const ClientType = types.EndpointTypePrefix + "net_client"

// ClientEndpoint alias
type ClientEndpoint = NetClient

// ClientConfig NET client configuration
// Used to configure the parameters for TCP/UDP clients to connect to remote servers
// Configuration for TCP/UDP client to connect to a remote server
type ClientConfig struct {
	// Communication protocol, supports the following values:
	// Communication protocol, supports the following values:
	// - "tcp" (default): TCP IPv4/IPv6 adaptive / TCP IPv4/IPv6 auto-detection
	// - "tcp4": TCP IPv4 only / TCP IPv4 only
	// - "tcp6": TCP IPv6 only / TCP IPv6 only
	// - "udp": UDP IPv4/IPv6 adaptive / UDP IPv4/IPv6 auto-detection
	// - "udp4": UDP IPv4 only / UDP IPv4 only
	// - "udp6": UDP IPv6 only / UDP IPv6 only
	// - "unix": Unix domain socket / Unix domain socket
	// - "unixpacket": Unix domain socket (packet mode) / Unix domain socket (packet mode)
	Protocol string `json:"protocol" label:"Protocol" desc:"Network protocol: tcp, tcp4, tcp6, udp, udp4, udp6, unix, unixpacket. Default: tcp"`

	// Remote server address, format: host:port
	// Remote server address, format: host:port
	// Examples: "192.168.1.100:8080", "127.0.0.1:1883", "[::1]:8080"
	Server string `json:"server" label:"Server Address" desc:"Remote server address, format: host:port" required:"true"`

	// Connection timeout, measured in seconds, default 5
	// Connection timeout in seconds, default 5
	ConnectTimeout int `json:"connectTimeout" label:"Connect Timeout" desc:"Connection timeout in seconds, default 5"`

	// Read the timeout time, measured in seconds; 0 means no timeout is set
	// Read timeout in seconds, 0 means no timeout
	ReadTimeout int `json:"readTimeout" label:"Read Timeout" desc:"Read timeout in seconds, 0 means no timeout"`

	// Disconnection interval, measured in seconds, default 5.0 means no reconnection
	// Reconnection interval in seconds, default 5, 0 means no reconnection
	ReconnectInterval int `json:"reconnectInterval" label:"Reconnect Interval" desc:"Reconnection interval in seconds, default 5, 0 means no reconnect"`

	// Data encoding and decoding methods:
	// Data encoding/decoding method:
	// - "hex": Encodes the received binary data into a hexadecimal string, with the message data type as TEXT
	// - "base64": Encodes the received binary data into a Base64 string, with the message data type as TEXT
	// - Other values (default): Keeps the original binary data unchanged, and the message data type is BINARY
	Encode string `json:"encode" label:"Encode" desc:"Data encoding: hex (hex string), base64 (base64 string), other (default binary)"`

	// Packet splitting mode:
	// Packet splitting mode:
	// - "line" (default): Splits by line, using \n or \r\n as the separator
	// - "fixed": Fixed-length split, must be used with PacketSize
	// - "delimiter": Custom separator split, must be used with Delimiter
	// - "length_prefix_le": Length prefix small end order; length does not contain prefixes
	// - "length_prefix_be": Length prefixes are large terminal order; length does not contain prefixes
	// - "length_prefix_le_inc": Length prefix, small-end sequence, length contains the prefix
	// - "length_prefix_be_inc": Length prefix large end order, length contains the prefix
	PacketMode string `json:"packetMode" label:"Packet Mode" desc:"Packet splitting mode: line, fixed, delimiter, length_prefix_le, length_prefix_be, length_prefix_le_inc, length_prefix_be_inc"`

	// Packet size configuration (depending on the meaning of PacketMode)
	// Packet size configuration (meaning varies by PacketMode)
	// - Fixed mode: Fixed packet byte count / Fixed mode: Fixed packet byte count
	// - length_prefix* mode: number of bytes with length prefix (1-4 bytes) / length_prefix* mode: length prefix byte count (1-4 bytes)
	// - Other modes: This field is invalid / Other modes: This field is invalid
	PacketSize int `json:"packetSize" label:"Packet Size" desc:"Packet size configuration (meaning varies by PacketMode)"`

	// Custom delimiter, effective only when PacketMode is "delimiter"
	// Custom delimiter, only effective when PacketMode is "delimiter"
	// Supports standard strings or hexadecimal format (e.g., "0x0D0A" means \r\n)
	Delimiter string `json:"delimiter" label:"Delimiter" desc:"Custom delimiter, only effective when PacketMode is delimiter. Supports hex format like 0x0D0A"`

	// Maximum packet size to prevent malicious or abnormal large packets, default 64KB
	// Maximum packet size to prevent malicious or abnormal large packets, default 64KB
	MaxPacketSize int `json:"maxPacketSize" label:"Max Packet Size" desc:"Maximum packet size to prevent malicious packets, default 64KB"`

	// Heartbeat transmission interval, measured in seconds, 0 means no heartbeat is sent
	// Heartbeat send interval in seconds, 0 means no heartbeat
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval" desc:"Heartbeat send interval in seconds, 0 means no heartbeat"`

	// Heartbeat Pack content, effective only at HeartbeatInterval > 0
	// Heartbeat packet content, only effective when HeartbeatInterval > 0
	// Supports standard strings and hexadecimal formats (such as "0x0D0A" representing \r\n), default "ping\n"
	HeartbeatData string `json:"heartbeatData" label:"Heartbeat Data" desc:"Heartbeat packet content. Supports hex format like 0x0D0A. Default: ping\\n"`
}

// ClientRequestMessage
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

// ClientResponseMessage: A client-side response message used to send data to the server via a connection
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

// The NetClient NET client endpoint acts as an active connector to connect to the remote TCP/UDP server and receive data
// NetClient is a NET client endpoint that actively connects to a remote TCP/UDP server and receives data.
//
// Workflow:
//  1. Call Start() to connect to the remote server
//  2. Process received data through router rules
//  3. Support sending response data to the server via router processors
//  4. Auto-reconnect on disconnection (if ReconnectInterval is configured) / Auto-reconnect on disconnection (if ReconnectInterval is configured)
//
// Usage example:
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
//	    Process received data
//	    return true
//	}).End()
//	client.AddRouter(router)
//	client.Start()
type NetClient struct {
	impl.BaseEndpoint
	// Config Client configuration
	Config ClientConfig
	// RuleConfig Rule engine configuration / Rule engine configuration
	RuleConfig types.Config
	conn       net.Conn
	routers    map[string]*RegexpRouter
	closed     int32
	mu         sync.RWMutex
	// OnEvent connection status event callback function
	// Connection state event callback function
	// Supported events:
	//   - endpoint.EventConnect: Triggered when connection is successful, with the parameter net.Conn / Triggered on successful connection, parameter is net. Conn
	//   - endpoint.EventDisconnect: Triggered on disconnection
	OnEvent func(event string, params ...interface{})
	// OnHeartbeat customizes heartbeat send callbacks to override the default heartbeat sending logic
	// Custom heartbeat send callback, overrides the default heartbeat logic
	// If this callback is set, the HeartbeatData configuration will be ignored, and the content sent will be determined entirely by the callback
	// If this callback is set, HeartbeatData config is ignored, the callback fully controls what to send
	//
	// Parameters / Parameters:
	//   - conn: Current TCP/UDP connection / Current TCP/UDP connection
	// Returns:
	//   - error: Non-nil stops the heartbeat goroutine
	OnHeartbeat func(conn net.Conn) error
}

// Type returns component type identification "endpoint/net_client"
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
					Desc:     "Regex applied to incoming data to select a router; empty / * / .* matches all (recommended: single default router)",
					Required: true,
				},
			},
		},
	}
}

// New creates a NetClient instance with the default configuration to create a new instance in the component registry
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

// Init initializes the client using the rule engine configuration and component configuration
// Initializes the client with rule engine configuration and component configuration
//
// Parameters / Parameters:
//   - ruleConfig: Rule engine configuration / Rule engine configuration
//   - configuration: Component configuration key-value pairs
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

// Destroy: Destroy the client, close the connection, and release resources
// Destroys the client, closes the connection and releases resources
func (c *NetClient) Destroy() {
	_ = c.Close()
}

// Close: Close closes client connections, stops all reads and heartbeat coroutines
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

// Id: Returns the server address as a unique endpoint identifier
// Returns the server address as the endpoint unique identifier
func (c *NetClient) Id() string {
	return c.Config.Server
}

// AddRouter adds routing rules to match and process received data
// Adds a router rule for matching and processing received data
//
// Router matching logic:
//   - The routing From expression matches the received data as a regular expression
//     The router's From expression is used as a regex to match against received data
//   - The empty strings """*", ".*" match all data
//     Empty string "", "*", ".*" match all data
//
// Optional parameters:
//   - *RouterMatchOptions: Routing matching options, supporting data length filtering, data type filtering, raw data matching, etc
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

// RemoveRouter removes registered routing rules based on the routing ID
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

// printf outputs logs to the logger of the rule engine
// Outputs log messages to the rule engine's logger

// Start the client, connect to the remote server, and start receiving data
// Starts the client, connects to the remote server and begins receiving data
// After a successful connection, the corresponding read loop coroutine is launched according to the protocol type (TCP uses streaming read, UDP uses datagram read).
// After successful connection, starts the corresponding read loop goroutine based on protocol type
// (TCP uses streaming read, UDP uses datagram read)
func (c *NetClient) Start() error {
	return c.connect()
}

// connect: establish a connection and start reading data
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

	// Only supports streaming reads for TCP protocol; UDP clients use different modes
	switch c.Config.Protocol {
	case ProtocolTCP, ProtocolTCP4, ProtocolTCP6, ProtocolUnix, ProtocolUnixPacket:
		go c.readLoop(conn)
	case ProtocolUDP, ProtocolUDP4, ProtocolUDP6:
		go c.readLoopUDP(conn)
	}

	// Start your heartbeat
	if c.Config.HeartbeatInterval > 0 {
		go c.heartbeatLoop(conn)
	}

	return nil
}

// readLoop TCP read loop
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

// readLoopUDP UDP read/loop
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

// heartbeatLoop Heartbeat sends
func (c *NetClient) heartbeatLoop(conn net.Conn) {
	ticker := time.NewTicker(time.Duration(c.Config.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	// Parsing heartbeat data: prioritize using custom callbacks, then use the configured HeartbeatData, and finally use the default values
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

// resolveHeartbeatData parses the heartbeat data content
func (c *NetClient) resolveHeartbeatData() []byte {
	data := c.Config.HeartbeatData
	if data == "" {
		return []byte(PingData + LineBreak)
	}
	// Supports hexadecimal formats, such as "0x0D0A"
	decoded, err := decodeHexIfMatch(data)
	if err == nil && decoded != nil {
		return decoded
	}
	return []byte(data)
}

// decodeHexIfMatch If the string starts with "0x", try to decode it to hexadecimal bytes
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

// tryReconnect attempts to reconnect
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

// isClosedConn checks whether it is a connection closure error
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

// Send: sends raw byte data to the remote server through the current connection
// Sends raw byte data to the remote server through the current connection
// Note: This method is not thread-safe and should avoid concurrent calls written to the routing processor's response
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
