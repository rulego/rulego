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
	// Type returns the component type
	Type = types.EndpointTypePrefix + "net"
	// RemoteAddrKey Remote address key
	RemoteAddrKey = "remoteAddr"
	// PingData heartbeat data
	PingData = "ping"
	// MatchAll matches all data
	MatchAll = "*"
	// BufferSize assumes a buffer size of 1024 bytes
	BufferSize = 1024
	// LineBreak JSON message line separator
	LineBreak = "\n"

	// DefaultMaxPacketSizeDefault maximum packet size (64KB)
	DefaultMaxPacketSize = 65536

	// DefaultSessionTTL Default session idle TTL (seconds, 30 minutes). 0=Disabled.
	DefaultSessionTTL = 1800

	// Protocol constant
	ProtocolTCP        = "tcp"
	ProtocolTCP4       = "tcp4"
	ProtocolTCP6       = "tcp6"
	ProtocolUDP        = "udp"
	ProtocolUDP4       = "udp4"
	ProtocolUDP6       = "udp6"
	ProtocolUnix       = "unix"
	ProtocolUnixPacket = "unixpacket"

	// Encoding mode constants
	EncodeHex    = "hex"
	EncodeBase64 = "base64"
	// Data type explanation (non-encoded: do not change bytes, only set dataType to allow downstream to process as text/JSON)
	EncodeText = "text"
	EncodeJson = "json"

	// Hexadecimal prefix constant
	HexPrefix   = "0x"
	HexPrefixUp = "0X"

	// PacketMode uses constants for parsing
	BigEndianSuffix      = "_be"
	IncludesPrefixSuffix = "_inc"

	// Special route matching constants
	RouteMatchDotStar = ".*"
)

// Endpoint alias
type Endpoint = Net

// RequestMessage
type RequestMessage struct {
	headers  textproto.MIMEHeader
	conn     net.Conn
	body     []byte
	msg      *types.RuleMsg
	err      error
	from     string
	dataType types.DataType // Add a data type field
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

// From returns client Addr
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
// GetMsg returns the RuleMsg associated with this request.
// If the message does not exist, a new message is created using the request data.
//
// Data Type Handling:
// Data type handling:
//
// By default, all network data is treated as BINARY type to preserve data integrity.
// This ensures that binary protocols, raw sensor data, and any byte sequences are
// handled correctly without character encoding issues.
//
// By default, all network data is considered BINARY type to maintain data integrity.
// This ensures that binary protocols, raw sensor data, and any byte sequences are handled correctly without character encoding issues.
//
// Changing Data Type with Processors:
// Change data type using the processor:
//
// The data type can be changed using built-in processors to optimize downstream
// component processing. Use processors in router configuration:
// The built-in processor can be used to change data types to optimize downstream component processing. Using processors in routing configurations:
//
//	router := impl.NewRouter().From("").
//	  Process("setJsonDataType").   // Changes to JSON type
//	  To("chain:jsonProcessor").End()
//
// Available data type processors:
// Available data types processors:
//   - setJsonDataType: For JSON protocols and REST APIs
//     Used for JSON protocol and REST API
//   - setTextDataType: For text-based protocols like HTTP, SMTP, etc.
//     Used for text-based protocols such as HTTP, SMTP, etc
//   - setBinaryDataType: For binary protocols (default, explicit setting)
//     Used for binary protocol (default, explicit settings)
//
// Protocol-Specific Recommendations:
// Protocol-specific recommendations:
//   - IoT sensors: Keep BINARY for raw data integrity
//     IoT sensors: Maintain BINARY to ensure the integrity of raw data
//   - JSON APIs: Use setJsonDataType processor
//     JSON API: uses the setJsonDataType processor
//   - Text protocols: Use setTextDataType processor
//     Text Protocol: Uses the setTextDataType processor
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		// Uses the actual data type; if not set, default is BINARY (network data defaults to binary type)
		// Use the actual data type, default to BINARY if not set (network data defaults to binary type)
		dataType := r.dataType
		if dataType == "" {
			dataType = types.BINARY
		}

		// Decide how to create messages based on the data type
		// Decide how to create the message based on data type
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

// SetStatusCode does not provide a response status code
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

// ResponseMessage
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
		// Check if the JSON data ends with a line new; if not, add it
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

// Config endpoint component configuration
// Configuration for the NET endpoint component that creates TCP/UDP servers
// for receiving and processing network messages through the RuleGo framework.
//
// Config.NET endpoint component configuration for creating TCP/UDP servers
// Receive and process network messages through the RuleGo framework.
//
// Data Type Processing:
// Data type handling:
//
// By default, all incoming network data is treated as BINARY type. This can be
// changed using built-in processors from the processor package:
// By default, all incoming network data is considered BINARY type. This can be done
// Changes to the built-in processor package include:
//
//   - processor.InBuiltins.Get("setJsonDataType"): Sets data type to JSON
//     Set the data type to JSON
//   - processor.InBuiltins.Get("setTextDataType"): Sets data type to TEXT
//     Set the data type to TEXT
//   - processor.InBuiltins.Get("setBinaryDataType"): Sets data type to BINARY
//     Set the data type to BINARY
//
// Packet Splitting Modes:
// Packet splitting mode:
//
// The endpoint supports multiple packet splitting strategies to handle different
// network protocols and data formats:
// Endpoints support multiple packet segmentation strategies to handle different network protocols and data formats:
//
//   - "line": Split by newline characters (\n or \r\n) - default mode
//     Split by line break (\n or \r\n) - default mode
//   - "fixed": Split by fixed byte length
//     Split by fixed byte length
//   - "delimiter": Split by custom delimiter (supports hex format)
//     Splitting by custom separator (supports hexadecimal format)
//   - "length_prefix_*": Split by length prefix with various endianness options
//     Split by length prefix, supporting various byte order options
//
// Router Configuration Best Practices:
// Best Practices for Route Configuration:
//
// It's recommended to use a single default router that matches all messages,
// and handle routing logic within rule chains for better maintainability:
// It is recommended to use a single default route that matches all messages and handle routing logic in the rule chain for better maintainability:
//
//	router := impl.NewRouter().From("").To("chain:main").End()
//	ep.AddRouter(router)
//
// Advanced routing with processors can be configured like:
// Advanced routers with processors can be configured, such as:
//
//	router := impl.NewRouter().From("").
//		Process("setJsonDataType").
//		To("chain:jsonProcessor").End()
type Config struct {
	// Network protocol: tcp, udp, ip4:1, ip6:ipv6-icmp, ip6:58, unix, unixgram, and other protocol types supported by the net package. Default: tcp
	Protocol string `json:"protocol" label:"Protocol" desc:"Network protocol: tcp, udp, ip4:1, ip6:ipv6-icmp, ip6:58, unix, unixgram. Default: tcp"`

	// Server address in host:port format
	Server string `json:"server" label:"Server Address" desc:"Listen address, format host:port or :port, e.g. 0.0.0.0:6335 or :6335" required:"true"`

	// Read timeout for setting data read timeout in seconds, can be 0 for no timeout
	ReadTimeout int `json:"readTimeout" label:"Read Timeout" desc:"Read timeout in seconds, 0 for no timeout"`

	// Encode Data Encoding/Type: hex/base64 Encoded bytes as strings; text/string set dataType=TEXT, json set dataType=JSON (does not encode bytes, making it easier for editors and downstream readability)
	Encode string `json:"encode" label:"Encode" desc:"hex/base64 encode bytes to string; text/string set dataType=TEXT, json set dataType=JSON (keep bytes, readable)."`

	// Packet splitting mode:
	// "line": Split by line (default mode, split by \n or \r\n)
	// "fixed": Fixed length splitting
	// "delimiter": Custom delimiter splitting
	// "length_prefix_le": Length prefix little endian, length excludes prefix
	// "length_prefix_be": Length prefix big endian, length excludes prefix
	// "length_prefix_le_inc": Length prefix little endian, length includes prefix
	// "length_prefix_be_inc": Length prefix big endian, length includes prefix
	PacketMode string `json:"packetMode" label:"Packet Mode" desc:"Packet splitting mode: line, fixed, delimiter, length_prefix_le, length_prefix_be, length_prefix_le_inc, length_prefix_be_inc"`

	// PacketSize configuration (meaning varies by PacketMode)
	// - fixed mode: fixed packet byte count
	// - length_prefix* mode: length prefix byte count (1-4 bytes)
	// - other modes: this field is invalid
	PacketSize int `json:"packetSize" label:"Packet Size" desc:"Packet size configuration (meaning varies by PacketMode)"`

	// Custom delimiter mode: delimiter byte sequence (supports hex format like "0x0A" for \n)
	Delimiter string `json:"delimiter" label:"Delimiter" desc:"Custom delimiter byte sequence (supports hex format like 0x0A for \\n)"`

	// Maximum packet size to prevent malicious packets, default 64KB
	MaxPacketSize int `json:"maxPacketSize" label:"Max Packet Size" desc:"Maximum packet size to prevent malicious packets, default 64KB"`

	// SessionKey declares a sessionKey extraction rule (rulego ${} expression, supports array multiple candidates). Leave blank = Use RemoteAddr.
	// For example: ${msg.deviceId} / ${msg.header.sn} / ${hex(data[4:14])} / ${reFind("ID:([a-zA-Z0-9_]+)", data)}
	// Note: Expressions like reFind containing string parameters must be in double quotes (el engines do not recognize single quotes);
	//       Avoid using \w (which is illegal escape in expr strings), and instead use [a-zA-Z0-9_]
	SessionKey interface{} `json:"sessionKey"`

	// SessionTTL session idle TTL (seconds). If idle times out, the connection is closed, prompting the client to reconnect and re-extract the sessionKey. <=0 uses the default 1800 (30 minutes).
	SessionTTL int `json:"sessionTTL" label:"Session TTL" desc:"Session idle timeout in seconds, <=0 uses default 1800 (30min)"`
}

// RegexpRouter is a regular expression for routing
type RegexpRouter struct {
	//Route ID
	id string
	//Route
	router endpoint.Router
	//Regular expression
	regexp *regexp.Regexp
	//Route matching options
	matchOptions *RouterMatchOptions
}

// RouterMatchOptions Route matching options
type RouterMatchOptions struct {
	// Match raw data rather than encoded data
	MatchRawData bool `json:"matchRawData"`
	// Data type filters: TEXT, BINARY, JSON, etc
	DataTypeFilter string `json:"dataTypeFilter"`
	// Minimum data length
	MinDataLength int `json:"minDataLength"`
	// Maximum data length
	MaxDataLength int `json:"maxDataLength"`
}

// Match checks whether the data matches routing rules
func (r *RegexpRouter) Match(rawData, encodedData []byte, exchange *endpoint.Exchange) bool {
	opts := r.matchOptions
	if opts == nil {
		return r.regexp == nil || r.regexp.Match(encodedData)
	}

	dataLen := len(rawData)
	if opts.MinDataLength > 0 && dataLen < opts.MinDataLength {
		return false
	}
	if opts.MaxDataLength > 0 && dataLen > opts.MaxDataLength {
		return false
	}

	if opts.DataTypeFilter != "" {
		msg := exchange.In.GetMsg()
		if strings.ToUpper(opts.DataTypeFilter) != strings.ToUpper(string(msg.GetDataType())) {
			return false
		}
	}

	var dataToMatch []byte
	if opts.MatchRawData {
		dataToMatch = rawData
	} else {
		dataToMatch = encodedData
	}

	return r.regexp == nil || r.regexp.Match(dataToMatch)
}

// encodeData encodes the data
func encodeData(src []byte, encode string) ([]byte, types.DataType) {
	switch strings.ToLower(encode) {
	case EncodeHex:
		encoded := make([]byte, hex.EncodedLen(len(src)))
		hex.Encode(encoded, src)
		return encoded, types.TEXT
	case EncodeBase64:
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
		base64.StdEncoding.Encode(encoded, src)
		return encoded, types.TEXT
	case EncodeText, "string":
		// No encoded bytes, only set dataType=TEXT:data interpreted by text string (editor debugging IN readable ASCII)
		return src, types.TEXT
	case EncodeJson:
		// No byte encoded, only set dataType=JSON: data interpreted by JSON (downstream jsTransform can parse directly)
		return src, types.JSON
	default:
		return src, types.BINARY
	}
}

// Net net endpoint component
// Supports routing matching messages to specified routes via regular expressions
//
// Routing usage recommendations:
// ⚠️ It is not recommended to use multi-routing functionality; it is recommended to add only one default route (using an empty string, "*" or ".*" to match all messages).
// ⚠️ Routing logic is processed within the rule chain, making it more flexible and easier to maintain
// ⚠️ The multi-route matching feature may be deprecated in future versions
//
// Recommended usage:
//
//	router := impl.NewRouter().From("").To("chain:main").End()
//	ep.AddRouter(router)
//
// Recommended usage:
//
//	router1 := impl.NewRouter().From("^sensor.*").To("chain:sensor").End()
//	router2 := impl.NewRouter().From("^device.*").To("chain:device").End()
//	Components like msgTypeSwitch or jsFilter should be used in the rule chain for routing
type Net struct {
	// Embedding endpoint.BaseEndpoint inherits its method
	impl.BaseEndpoint
	// Configuration
	Config Config
	// rulego configuration
	RuleConfig types.Config
	// Server listener object
	listener net.Listener
	// udp conn
	udpConn *net.UDPConn
	// Route mapping table
	routers map[string]*RegexpRouter
	closed  int32        // Using the int32 type supports atomic operations; 0 means not closed, 1 means closed
	mu      sync.RWMutex // Protect concurrent access between listeners and udpConn

	// Embedded session registry, supports proactive push to connected clients by key
	impl.DefaultSessionRegistry
	// sessionKey extractor (constructed when init), and RemoteAddr when nil is used
	keyResolver *impl.SessionKeyResolver
}

// Type returns the component type
func (ep *Net) Type() string {
	return Type
}

// Category returns the component category
func (ep *Net) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (ep *Net) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "TCP/UDP network server endpoint for receiving and processing network data",
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

func (ep *Net) New() types.Node {
	return &Net{
		Config: Config{
			Protocol:      ProtocolTCP,
			ReadTimeout:   60,
			Server:        ":6335",
			PacketMode:    PacketModeLine.String(), // By default, it is divided by row
			PacketSize:    2,
			Encode:        "none",
			MaxPacketSize: DefaultMaxPacketSize, // The default maximum package size is 64KB
			SessionTTL:    DefaultSessionTTL,
		},
	}
}

// Init initializes the component
func (ep *Net) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Converts the configuration into the EndpointConfiguration structure
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
	ep.Logger = ruleConfig.Logger
	ep.keyResolver = impl.NewSessionKeyResolver(ep.Config.SessionKey)
	return err
}

// GetInstance returns itself, satisfying types.SharedNode, used by ref:// to fetch *Net from NodePool for session addressing.
func (ep *Net) GetInstance() (interface{}, error) { return ep, nil }

// addr returns the actual listening address (e.g., server=":0" to the real address after random port binding). Returns empty strings without monitoring.
func (ep *Net) Addr() string {
	ep.mu.RLock()
	defer ep.mu.RUnlock()
	if ep.listener != nil {
		return ep.listener.Addr().String()
	}
	if ep.udpConn != nil {
		return ep.udpConn.LocalAddr().String()
	}
	return ""
}

// Destroy releases resources
func (ep *Net) Destroy() {
	ep.StopSweeping() // Stop TTL scanning first to prevent race against Clear
	_ = ep.Close()
	ep.Clear() // Bottom-line cleanup of session registry
}

// SendToTarget implements types.TargetSender: Send data by target address.
// target: IP / deviceId / *(broadcast) / empty (broadcast).
// Return (number of successes, number of failures, first error); Missing any session returns err.
//
// Address the embedded DefaultSessionRegistry.Lookup (sync.Map Range, unlocked),
// Zero allocation (reusing parameter data). Allows net nodes ref:// same-chain/shared pool addressing push push multiplex.
func (ep *Net) SendToTarget(target string, data []byte) (sent, failed int, err error) {
	sessions := ep.Lookup(target)
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

// Close: Close the network endpoint
func (ep *Net) Close() error {
	atomic.StoreInt32(&ep.closed, 1)

	ep.mu.Lock()
	defer ep.mu.Unlock()

	var err error
	if ep.listener != nil {
		err = ep.listener.Close()
		ep.listener = nil
	}
	if ep.udpConn != nil {
		udpErr := ep.udpConn.Close()
		ep.udpConn = nil
		if err == nil {
			err = udpErr
		}
	}
	return err
}

func (ep *Net) Id() string {
	return ep.Config.Server
}

// AddRouter adds routing rules
//
// ⚠️ It is not recommended to use multiple routes; it is recommended to add only one default route to match all messages
// ⚠️ Routing expressions support special values: empty strings (""), "*", or ".*" will match all data
// ⚠️ It is recommended to place complex routing logic within the rule chain
//
// Parameters:
//   - router: routing rules
//   - params: Optional parameters, the first can be *RouterMatchOptions for advanced matching
//
// Returns:
//   - Routing ID and error messages
func (ep *Net) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	} else {
		expr := router.GetFrom().ToString()
		//Allow empty expr, indicating matching all items
		var regexpV *regexp.Regexp
		// Special routing expressions do not create regular expressions; in matchesRouter, they are determined by regexp==nil
		if expr != "" && expr != MatchAll && expr != RouteMatchDotStar {
			//Compiling expressions
			if re, err := regexp.Compile(expr); err != nil {
				return "", err
			} else {
				regexpV = re
			}
		}

		// Parse routing matching options
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

// Start the Net endpoint
func (ep *Net) Start() error {
	var err error
	// Create a server listener based on the configured protocol and address
	switch ep.Config.Protocol {

	case ProtocolTCP, ProtocolTCP4, ProtocolTCP6, ProtocolUnix, ProtocolUnixPacket:
		listener, err := net.Listen(ep.Config.Protocol, ep.Config.Server)
		if err != nil {
			return err
		}

		ep.mu.Lock()
		ep.listener = listener
		ep.mu.Unlock()

		ep.Printf("started TCP server on %s", ep.Config.Server)
		go ep.acceptTCPConnections()
		ttl := time.Duration(ep.Config.SessionTTL) * time.Second
		if ttl <= 0 {
			ttl = time.Duration(DefaultSessionTTL) * time.Second
		}
		ep.StartSweeping(ttl, ttl/2)
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
		if err := ep.submitTask(h.handler); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported protocol: %s", ep.Config.Protocol)
	}
	return nil
}

// listenUDP starts UDP monitoring
func (ep *Net) listenUDP() error {
	udpAddr, err := net.ResolveUDPAddr(ep.Config.Protocol, ep.Config.Server)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP(ep.Config.Protocol, udpAddr)
	if err != nil {
		return err
	}

	ep.mu.Lock()
	ep.udpConn = udpConn
	ep.mu.Unlock()

	return nil
}

func (ep *Net) acceptTCPConnections() {
	// Loop to accept connection requests from clients
	for {
		// Check if it is closed to avoid data contention
		if atomic.LoadInt32(&ep.closed) == 1 {
			ep.Printf("net endpoint stop")
			return
		}

		// Retrieves listener references to avoid accessing nil pointers during the Close() process
		ep.mu.RLock()
		listener := ep.listener
		ep.mu.RUnlock()

		if listener == nil {
			ep.Printf("net endpoint stop - listener is nil")
			return
		}

		// Obtain a client connection from the listener, returning the connection object and error information
		conn, err := listener.Accept()
		if err != nil {
			if opError, ok := err.(*net.OpError); ok && opError.Err == net.ErrClosed {
				ep.Printf("net endpoint stop")
				return
				//return endpoint.ErrServerStopped
			} else {
				ep.Printf("accept: %v", err)
				continue
			}
		}

		// Check the closed status again to prevent it from being closed during Accept().
		if atomic.LoadInt32(&ep.closed) == 1 {
			_ = conn.Close()
			ep.Printf("net endpoint stop - closing accepted connection")
			return
		}

		// Print the client-side connection information
		//ep.Printf("new connection from:", conn.RemoteAddr().String())
		h := TcpHandler{
			endpoint: ep,
			conn:     conn,
			config:   ep.Config,
		}
		// Start a coroutine to handle client connections; If submit fails, close conn to prevent FD leaks
		if err := ep.submitTask(h.handler); err != nil {
			_ = conn.Close()
		}
	}
}

func (ep *Net) submitTask(fn func()) error {
	if ep.RuleConfig.Pool != nil {
		if err := ep.RuleConfig.Pool.Submit(fn); err != nil {
			ep.Printf("submit task err: %v", err)
			return err
		}
	} else {
		go fn()
	}
	return nil
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
	// Client connection to objects
	conn net.Conn
	// Create a read-out timer to set the timeout for reading data; 0 can indicate no timeout
	readTimeoutTimer *time.Timer
	//Read data configuration
	config Config
	// Packet splitter
	splitter PacketSplitter
	// The session of the current connection (created and registered to the registry when the connection is established)
	session *endpoint.Session
}

func (x *TcpHandler) handler() {
	defer func() {
		// Disconnection: Cancel the session (before conn.Close, ensure the registry does not retain closed connections)
		if x.session != nil {
			x.endpoint.Remove(x.session.Key())
		}
		_ = x.conn.Close()
		//Capture anomalies
		if e := recover(); e != nil {
			x.endpoint.Printf("net endpoint handler err :\n%v", runtime.Stack())
		}
	}()

	// Connection establishment: Create and register a session (default Key = RemoteAddr)
	from0 := ""
	if x.conn.RemoteAddr() != nil {
		from0 = x.conn.RemoteAddr().String()
	}
	x.session = endpoint.NewSession(from0, &connSender{conn: x.conn})
	x.endpoint.Add(x.session)

	// Create a packet splitter
	splitter, err := CreatePacketSplitter(x.endpoint.Config)
	if err != nil {
		x.endpoint.Printf("failed to create packet splitter: %v", err)
		return
	}
	x.splitter = splitter

	readTimeoutDuration := time.Duration(x.endpoint.Config.ReadTimeout+5) * time.Second
	//Read timeout, disconnect the connection
	x.readTimeoutTimer = time.AfterFunc(readTimeoutDuration, func() {
		if x.endpoint.Config.ReadTimeout > 0 {
			x.onDisconnect()
		}
	})
	// Create a buffer reader to read data sent by the client
	reader := bufio.NewReader(x.conn)
	// Loop to read data sent by the client
	for {
		// Set read timeout
		if x.endpoint.Config.ReadTimeout > 0 {
			err := x.conn.SetReadDeadline(time.Now().Add(readTimeoutDuration))
			if err != nil {
				x.onDisconnect()
				break
			}
		}

		// Data is read using a packet splitter
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
		//Reset the read-out timer
		if x.endpoint.Config.ReadTimeout > 0 {
			x.readTimeoutTimer.Reset(readTimeoutDuration)
		}
		if x.session != nil {
			x.session.Touch() // Each frame refreshes lastSeen (including heartbeat frames), TTL is kept alive
		}
		if string(data) == PingData {
			continue
		}
		// Encoding processing
		encodedMessage, dataType := encodeData(data, x.endpoint.Config.Encode)

		from := ""
		if x.conn.RemoteAddr() != nil {
			from = x.conn.RemoteAddr().String()
		}
		// Create an exchange object to store input and output messages
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				conn:     x.conn,
				body:     encodedMessage,
				from:     from,
				dataType: dataType, // Set the correct data type
			},
			Out: &ResponseMessage{
				log: func(format string, v ...interface{}) {
					x.endpoint.Printf(format, v...)
				},
				conn: x.conn,
				from: from,
			}}

		msg := exchange.In.GetMsg()
		// Place the client-side connection address into the MSG metadata
		msg.Metadata.PutValue(RemoteAddrKey, from)

		// sessionKey extraction: only performed before determination (skipped after keyResolved), using SessionKeyResolver(rulego ${} expression)
		if x.session != nil && !x.session.IsResolved() && x.endpoint.keyResolver != nil {
			if key := x.endpoint.keyResolver.Resolve(*msg, data); key != "" {
				x.endpoint.Rekey(x.session, key) // Atomic key change: Deregister the old Key + SetKey + register the new Key
			}
		}

		// Matching the matching routes and processing messages
		x.endpoint.RLock()
		snapshot := make([]*RegexpRouter, 0, len(x.endpoint.routers))
		for _, v := range x.endpoint.routers {
			snapshot = append(snapshot, v)
		}
		x.endpoint.RUnlock()
		for _, v := range snapshot {
			if v.Match(data, encodedMessage, exchange) {
				x.endpoint.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}

}

func (x *TcpHandler) onDisconnect() {
	if x.conn != nil {
		_ = x.conn.Close()
	}
	if x.readTimeoutTimer != nil {
		x.readTimeoutTimer.Stop()
	}
	if x.conn.RemoteAddr() != nil {
		x.endpoint.Printf("onDisconnect: %s", x.conn.RemoteAddr().String())
	}
}

type UDPHandler struct {
	endpoint *Net
	// Create a read-out timer to set the timeout for reading data; 0 can indicate no timeout
	readTimeoutTimer *time.Timer
	//Read data configuration
	config Config
}

func (x *UDPHandler) handler() {
	// UDP uses the maximum package size configured but not less than the original BufferSize
	bufferSize := x.endpoint.Config.MaxPacketSize
	if bufferSize < BufferSize {
		bufferSize = BufferSize
	}
	buffer := make([]byte, bufferSize)

	for {
		if atomic.LoadInt32(&x.endpoint.closed) == 1 {
			break
		}

		x.endpoint.mu.RLock()
		udpConn := x.endpoint.udpConn
		x.endpoint.mu.RUnlock()

		if udpConn == nil {
			break
		}

		n, addr, err := udpConn.ReadFromUDP(buffer)
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

		// Check the package size limits
		if len(msgBuffer) > x.endpoint.Config.MaxPacketSize {
			x.endpoint.Printf("UDP packet too large: %d > %d from %s", len(msgBuffer), x.endpoint.Config.MaxPacketSize, addr)
			continue
		}

		from := ""
		if addr != nil {
			from = addr.String()
		}
		// Encoding processing
		encodedMessage, dataType := encodeData(msgBuffer, x.endpoint.Config.Encode)

		// Create an exchange object to store input and output messages
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				conn:     x.endpoint.udpConn,
				body:     encodedMessage,
				from:     from,
				dataType: dataType, // Set the correct data type
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
		// Place the client-side connection address into the MSG metadata
		msg.Metadata.PutValue(RemoteAddrKey, from)

		// Matching the matching routes and processing messages
		x.endpoint.RLock()
		snapshot := make([]*RegexpRouter, 0, len(x.endpoint.routers))
		for _, v := range x.endpoint.routers {
			snapshot = append(snapshot, v)
		}
		x.endpoint.RUnlock()
		for _, v := range snapshot {
			if v.Match(msgBuffer, encodedMessage, exchange) {
				x.endpoint.DoProcess(context.Background(), v.router, exchange)
			}
		}
	}
}
