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

package external

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

// EndSign terminator
const EndSign = '\n'

// PingData ping content
var PingData = []byte("ping\n")

// Register the node
func init() {
	Registry.Add(&NetNode{})
}

// Configuration of the NetNodeConfiguration component
type NetNodeConfiguration struct {
	Protocol          string `json:"protocol" label:"Protocol" desc:"Network protocol: tcp, udp, default is tcp"`
	Server            string `json:"server" label:"Server" desc:"Server address, format: host:port" required:"true" ref:"primary"`
	ConnectTimeout    int    `json:"connectTimeout" label:"Connect Timeout (s)" desc:"Connection timeout in seconds"`
	HeartbeatInterval int    `json:"heartbeatInterval" label:"Heartbeat Interval (s)" desc:"Heartbeat interval in seconds"`
	// Target: Addressing the target. It only takes effect when the server is ref:// (pointing to the server endpoint).
	// Supports rulego ${} expressions, such as ${metadata.deviceId}; The value is IP / deviceId / *(broadcast) / empty.
	Target string `json:"target" label:"Target" desc:"Addressing target when server is ref://. Supports ${metadata.xxx}; IP/deviceId/* (broadcast)"`
}

// NetNode provides network protocol communication capabilities for sending messages over various protocols.
// It supports TCP, UDP, IP, Unix sockets, and other protocols supported by Go's net package,
// with automatic heartbeat, reconnection, and connection lifecycle management.
//
// NetNode provides network protocol communication capabilities for sending messages through various protocols.
// Supports TCP, UDP, IP, Unix sockets, and other protocols supported by Go Net packages,
// Features automatic heartbeat, reconnection, and connection lifecycle management functions.
//
// Configuration:
// Configuration:
//
//	{
//		"protocol": "tcp",              // Network protocol
//		"server": "192.168.1.100:8080", // Server address
//		"connectTimeout": 30,           // Connection timeout in seconds
//		"heartbeatInterval": 60         // Heartbeat interval in seconds (0=disabled)
//	}
//
// Supported Protocols:
// Supported protocols:
//
//   - "tcp": TCP protocol for reliable, connection-oriented communication
//     TCP protocol is used for reliable connection-oriented communication
//   - "udp": UDP protocol for fast, connectionless communication
//     UDP protocol for fast connectionless communication
//   - "ip4:1", "ip6:ipv6-icmp", "ip6:58": Raw IP protocols
//     Original IP protocol
//   - "unix", "unixgram": Unix domain sockets for local communication
//     Unix domain sockets for local communication
//   - Any protocol supported by Go's net.Dial function
//     Any protocol supported by the Go net.Dial function
//
// Smart Data Type Handling:
// Intelligent Data Type Processing:
//
// The component intelligently handles different data types:
// Components intelligently handle different data types:
//   - BINARY: Uses GetBytes(), sends raw bytes without terminator
//     Binary: Use GetBytes() to send the original byte without adding a terminator
//   - JSON/TEXT: Uses GetData(), appends newline terminator ('\n')
//     JSON/text: Use GetData() to add a newline terminator ('\n')
//
// Message Format:
// Message format:
//
// For non-binary data, messages are sent with an automatic newline terminator ('\n') appended.
// Binary data is sent as-is without any modifications.
// This ensures proper message framing while preserving binary data integrity.
//
// For non-binary data, a newline terminator ('\n') is automatically added when the message is sent.
// Binary data is sent as is, without any modifications.
// This ensures the correct message frame while maintaining the integrity of binary data.
//
// Connection Management:
// Connection Management:
//
// The component implements:
// Component implementation:
//   - Automatic connection establishment and reconnection
//   - Configurable heartbeat with ping mechanism
//   - Connection pooling through SharedNode pattern
//   - Graceful connection cleanup on destroy
//
// Heartbeat Mechanism:
// Heartbeat mechanism:
//
// When heartbeatInterval > 0, the component sends periodic "ping\n" messages
// to maintain connection liveness. Failed heartbeats trigger automatic reconnection.
//
// When heartbeatInterval > 0, the component sends periodic "ping\n" messages to maintain connection activity.
// A failed heartbeat will trigger automatic reconnection.
//
// Error Handling and Reconnection:
// Error handling and reconnection:
//
// The component includes robust error handling with automatic reconnection on:
// Components include powerful error handling and automatically reconnect in the following situations:
//   - Connection timeouts
//   - Network errors during message sending
//   - Heartbeat failures
//   - Server disconnections
//
// Thread Safety:
// Thread safety:
//
// The component uses atomic operations and mutex locks to ensure safe concurrent
// access across multiple rule chain executions.
//
// Components use atomic operations and mutexes to ensure secure concurrent access between multiple rule chains.
//
// Output Relations:
// Output relationships:
//
//   - Success: Message sent successfully
//   - Failure: Network error or connection failure
//
// Usage Examples:
// Example:
//
//	// TCP client for sending JSON telemetry data
//	TCP client used to send JSON telemetry data
//	{
//		"id": "tcpSender",
//		"type": "net",
//		"configuration": {
//			"protocol": "tcp",
//			"server": "telemetry.example.com:9999",
//			"connectTimeout": 30,
//			"heartbeatInterval": 60
//		}
//	}
//
//	// UDP client for sending binary data (no terminator added)
//	UDP client for sending binary data (without terminator)
//	{
//		"id": "udpBinarySender",
//		"type": "net",
//		"configuration": {
//			"protocol": "udp",
//			"server": "binary.example.com:8888",
//			"connectTimeout": 10,
//			"heartbeatInterval": 0
//		}
//	}
type NetNode struct {
	base.SharedNode[net.Conn]
	// Node configuration
	Config NetNodeConfiguration
	// ruleGo configuration
	ruleConfig types.Config
	// target parser (a precompiled template when Init), used in ref:// mode
	targetResolver *base.TargetResolver
	// Create a heartbeat timer to regularly send heartbeat messages; 0 can indicate no heartbeat
	heartbeatTimer *time.Timer
	//Heartbeat interval
	heartbeatDuration time.Duration
	// Check if the connection has been disconnected, 0: No port; 1: Port
	disconnected int32
	//Number of disconnections
	disconnectedCount int32
}

// Type returns the component type
func (x *NetNode) Type() string {
	return "net"
}

func (x *NetNode) New() types.Node {
	return &NetNode{Config: NetNodeConfiguration{
		Protocol:          "tcp",
		ConnectTimeout:    60,
		HeartbeatInterval: 60,
	}}
}

// Init initializes the component
func (x *NetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.ruleConfig = ruleConfig
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	// Set the default value
	x.setDefaultConfig()
	x.targetResolver = base.NewTargetResolver(x.Config.Target)
	x.heartbeatDuration = time.Duration(x.Config.HeartbeatInterval) * time.Second
	return x.SharedNode.InitWithClose(ruleConfig, x.Type(), x.Config.Server, ruleConfig.NodeClientInitNow, x.initConnect, func(conn net.Conn) error {
		// Cleanup callback function: Close the connection and clean up the related status
		x.onDisconnect()
		return conn.Close()
	})
}

// OnMsg: The server pushes addresses at ref://, otherwise it will dial out.
func (x *NetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data []byte

	// Intelligent processing based on data type
	if msg.GetDataType() == types.BINARY {
		// Binary data: Directly obtains a byte array without adding terminators
		data = msg.GetBytes()
	} else {
		// Text or JSON data: Retrieves the string and adds a newline terminator
		strData := msg.GetData()
		data = []byte(strData)
		data = append(data, EndSign)
	}

	// Mode determination: ref:// Points to server endpoint → Addresses push; Otherwise, exit the station dial
	if x.IsFromPool() {
		x.onSendToEndpoint(ctx, msg, data)
		return
	}
	x.onWrite(ctx, msg, data)
}

// onSendToEndpoint ref:// mode: Parse the target instance and push by target address.
// Parsing order: Resources on the same chain prioritize → NodePool reverts (all follow sync.Map, unlocked read).
// TargetSender → SendToTarget addressing; net.Conn → Direct Write (compatible with shared outbound connections).
func (x *NetNode) onSendToEndpoint(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	target := x.targetResolver.Resolve(ctx, msg)
	if target == "" && !x.targetResolver.IsEmpty() && x.targetResolver.Literal() != "*" {
		ctx.TellFailure(msg, fmt.Errorf("target %q resolved to empty", x.targetResolver.Literal()))
		return
	}
	inst, found := base.ResolveRefEndpoint(ctx, x.ruleConfig.NodePool, x.InstanceId)
	if !found {
		ctx.TellFailure(msg, fmt.Errorf("ref://%s not found in chain or node pool", x.InstanceId))
		return
	}
	switch v := inst.(type) {
	case types.TargetSender:
		sent, failed, err := v.SendToTarget(target, data)
		if err != nil && sent == 0 {
			ctx.TellFailure(msg, err)
			return
		}
		if failed > 0 {
			x.Printf("partial delivery: %d/%d failed for target=%q", failed, sent+failed, target)
		}
		ctx.TellSuccess(msg)
	case net.Conn:
		// ref:// Borrow a shared connection from another outbound NetNode (not a local connection to this node).
		// The borrower is only responsible for a single write; failure is called TellFailure; The health of the connection/reconnection is determined by the holder
		// (formerly NetNode's heartbeat/tryReconnect) management; this node does not take over its lifecycle.
		if _, err := v.Write(data); err != nil {
			ctx.TellFailure(msg, err)
			return
		}
		ctx.TellSuccess(msg)
	default:
		ctx.TellFailure(msg, fmt.Errorf("ref://%s type %T does not support addressing", x.InstanceId, inst))
	}
}

// Destroy releases resources
func (x *NetNode) Destroy() {
	_ = x.SharedNode.Close()
}

func (x *NetNode) Printf(format string, v ...interface{}) {
	x.ruleConfig.Logger.Printf(format, v...)
}

// The initConnect method is simplified
func (x *NetNode) initConnect() (net.Conn, error) {
	conn, err := net.DialTimeout(x.Config.Protocol, x.Config.Server, time.Duration(x.Config.ConnectTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	x.setDisconnected(false)
	if x.heartbeatDuration != 0 {
		// Initialize the heartbeat timer
		if x.heartbeatTimer == nil {
			x.heartbeatTimer = time.AfterFunc(x.heartbeatDuration, func() {
				x.onPing()
			})
		} else {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		}
	}
	return conn, nil
}

// Repeatedly connected
func (x *NetNode) tryReconnect() {
	// ref:// Endpoint mode does not reconnect (non-outbound connection)
	if x.IsFromPool() {
		return
	}
	// Attempting to obtain a new connection via SharedNode (triggering reinitialization)
	if conn, err := x.SharedNode.GetSafely(); err != nil {
		// Try again after 5 seconds
		x.heartbeatTimer.Reset(5 * time.Second)
	} else {
		x.setDisconnected(false)
		x.Printf("Reconnected to: %s", conn.RemoteAddr().String())
		// After successful reconnection, the interval is reset to normal
		x.heartbeatTimer.Reset(x.heartbeatDuration)
	}
}

func (x *NetNode) onPing() {
	// ref:// endpoint mode: no heartbeat (server-side addressing, not outbound connection; and SharedNode.GetSafely type assertion fails)
	if x.IsFromPool() {
		return
	}
	// If the connection has already been disconnected, try reconnecting
	if x.isDisconnected() {
		x.tryReconnect()
		return
	}
	// Sending heartbeats
	if conn, err := x.SharedNode.GetSafely(); err == nil {
		if _, err := conn.Write(PingData); err != nil {
			x.Printf("Ping failed: %v", err)
			x.setDisconnected(true)
			x.tryReconnect()
		} else {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		}
	}
}

func (x *NetNode) onWrite(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	// Send data to the server
	if conn, err := x.SharedNode.GetSafely(); err != nil {
		ctx.TellFailure(msg, err)
	} else if _, err := conn.Write(data); err != nil {
		if atomic.LoadInt32(&x.disconnectedCount) == 0 {
			x.setDisconnected(true)
			//Try again
			x.onWrite(ctx, msg, data)
		} else {
			x.setDisconnected(true)
			ctx.TellFailure(msg, err)
		}
	} else {
		//Reset the heartbeat sending interval
		if x.heartbeatTimer != nil {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		}
		//Send to the next node
		ctx.TellSuccess(msg)
	}
}

func (x *NetNode) onDisconnect() {
	// Stop the heartbeat timer
	if x.heartbeatTimer != nil {
		x.heartbeatTimer.Stop()
	}
	x.setDisconnected(true)
}

func (x *NetNode) isDisconnected() bool {
	return atomic.LoadInt32(&x.disconnected) == 1
}

func (x *NetNode) setDisconnected(disconnected bool) {
	if disconnected {
		atomic.AddInt32(&x.disconnectedCount, 1)
		atomic.StoreInt32(&x.disconnected, 1)
	} else {
		atomic.StoreInt32(&x.disconnectedCount, 0)
		atomic.StoreInt32(&x.disconnected, 0)
	}
}

// Default value settings
func (x *NetNode) setDefaultConfig() {
	if x.Config.Protocol == "" {
		x.Config.Protocol = "tcp"
	}
	if x.Config.ConnectTimeout <= 0 {
		x.Config.ConnectTimeout = 60
	}
	if x.Config.HeartbeatInterval < 0 {
		x.Config.HeartbeatInterval = 60
	}
}

// Desc returns the component description
func (x *NetNode) Desc() string {
	return "Network protocol communication (TCP, UDP, Unix sockets etc.) with heartbeat and auto-reconnection. Binary data sent raw, text/JSON appends newline. Routes to Success/Failure"
}
