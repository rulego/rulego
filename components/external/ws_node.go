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

package external

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

// writeWait timeout for a single write control frame (ping).
const writeWait = 10 * time.Second

// reconnectDelay The interval between retries after a failed dial-up
const reconnectDelay = 5 * time.Second

func init() {
	Registry.Add(&WsNode{})
}

// WsNodeConfiguration ws Client node configuration
type WsNodeConfiguration struct {
	// Server: Simply enter ws://host:port to dial out the server; Or ref:// points to endpoint/ws addressed push
	Server string `json:"server" label:"Server" desc:"ws://host:port 拨号发送；或 ref:// 指向 endpoint/ws 寻址推送" required:"true" ref:"primary"`
	// Target: Addressed only when the server is ref://. IP/deviceId/* (Broadcast); Supports ${metadata.xxx}
	Target string `json:"target" label:"Target" desc:"仅 ref:// 时生效。IP/deviceId/* (broadcast); supports ${metadata.xxx}"`

	// Below are outbound client configurations (only server is set to ws://wss:// that is, effective in dial-up mode; ignored in ref:// mode)
	// Headers: HTTP request headers during handshakes, such as Authorization
	Headers map[string]string `json:"headers" label:"Headers" desc:"Handshake HTTP headers, e.g. Authorization"`
	// Subprotocol Sec-WebSocket-Protocol subprotocols (dial-up mode), such as mqtt / ocpp1.6. Leaving the space empty without negotiation
	Subprotocol string `json:"subprotocol" label:"Subprotocol" desc:"Sec-WebSocket-Protocol, e.g. mqtt / ocpp1.6. Leave empty to skip"`
	// ConnectTimeout dial-up handshake timeout (seconds), 0 is the default
	ConnectTimeout int `json:"connectTimeout" label:"Connect Timeout (s)" desc:"Dial handshake timeout in seconds, 0=default"`
	// InsecureSkipVerify: Skips TLS certificate validation (wss:// self-visa document scenario), consistent with restApiCall
	InsecureSkipVerify bool `json:"insecureSkipVerify" label:"Skip TLS Verify" desc:"Set to true to skip HTTPS certificate verification"`
	// MessageType Message type: text/binary (default)
	MessageType string `json:"messageType" label:"Message Type" desc:"text or binary (default text)"`
	// HeartbeatInterval (seconds). >0 Regularly send PingMessages to keep them alive, automatically reconnect when disconnected; 0 = Heartbeat and reconnection disabled
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval (s)" desc:"Heartbeat interval in seconds. 0=disable heartbeat and reconnect"`
}

// WsNode is a WebSocket client node, symmetrical to NetNode: outbound dial-up (ws://) + ref:// addressed push (endpoint/ws).
type WsNode struct {
	base.SharedNode[*websocket.Conn]
	Config     WsNodeConfiguration
	ruleConfig types.Config
	// target parser (a precompiled template when Init), used in ref:// mode
	targetResolver *base.TargetResolver

	// The following are only for outbound mode (server is not ref://).
	// mu protects business WriteMessage (Gorilla's WriteMessage does not support concurrent writes;
	//   WriteControl can be sent concurrently with WriteMessage, so ping does not hold this lock)
	mu sync.Mutex
	// timerMu protects heartbeatTimer field read/write (initConnect/onPing/tryReconnect/Destroy may cause concurrent access)
	timerMu        sync.Mutex
	heartbeatTimer *time.Timer
	heartbeatDur   time.Duration
	disconnected   int32 // 0 = Normal 1 = Disconnected and awaiting reconnection
	reconnecting   int32 // CAS prevents reentry of heavy connections
	closed         int32 // Destroy is set to 1 to prevent reconnection/heartbeat
}

func (x *WsNode) Type() string {
	return "ws"
}

func (x *WsNode) Category() string {
	return "external"
}

func (x *WsNode) New() types.Node {
	return &WsNode{Config: WsNodeConfiguration{
		InsecureSkipVerify: false,
		MessageType:        "text",
		HeartbeatInterval:  60,
	}}
}

func (x *WsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.ruleConfig = ruleConfig
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	x.setDefaultConfig()
	x.targetResolver = base.NewTargetResolver(x.Config.Target)
	x.heartbeatDur = time.Duration(x.Config.HeartbeatInterval) * time.Second
	return x.SharedNode.InitWithClose(ruleConfig, x.Type(), x.Config.Server, ruleConfig.NodeClientInitNow,
		x.initConnect,
		func(conn *websocket.Conn) error {
			return conn.Close()
		})
}

func (x *WsNode) setDefaultConfig() {
	if x.Config.MessageType == "" {
		x.Config.MessageType = "text"
	}
	if x.Config.HeartbeatInterval < 0 {
		x.Config.HeartbeatInterval = 60
	}
}

// initConnect outbound dial-up: Connect to the remote WS server, applying Headers/Subprotocols/ConnectTimeout/InsecureSkipVerify.
// Execution only occurs when the server is not ref:// (ref:// run via NodePool). After dialing successfully, start reading loop and heartbeat.
func (x *WsNode) initConnect() (*websocket.Conn, error) {
	dialer := *websocket.DefaultDialer // Copying to avoid contaminating the entire situation
	if x.Config.ConnectTimeout > 0 {
		dialer.HandshakeTimeout = time.Duration(x.Config.ConnectTimeout) * time.Second
	}
	if x.Config.InsecureSkipVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if x.Config.Subprotocol != "" {
		dialer.Subprotocols = []string{x.Config.Subprotocol}
	}
	header := http.Header{}
	for k, v := range x.Config.Headers {
		header.Set(k, v)
	}
	c, _, err := dialer.Dial(x.Config.Server, header)
	if err != nil {
		return nil, err
	}
	x.setDisconnected(false)
	// The read loop only starts when the heartbeat is enabled and not destroyed: consumes the pong returned by the peer and triggers reconnection when the connection is disconnected
	if x.heartbeatDur > 0 && atomic.LoadInt32(&x.closed) == 0 {
		go x.readLoop(c)
		x.resetHeartbeat(x.heartbeatDur)
	}
	return c, nil
}

// readLoop continuously reads to process the peer pong control frame (WsNode sends but does not receive, discarding business messages). Read error to trigger reconnection.
func (x *WsNode) readLoop(c *websocket.Conn) {
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if atomic.LoadInt32(&x.closed) == 1 {
				return // Disable triggered by Destroy and exit normally
			}
			x.setDisconnected(true)
			x.tryReconnect()
			return
		}
	}
}

// OnMsg server pushes addressing at ref://, otherwise outbound WriteMessage.
func (x *WsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	data := msg.GetBytes()
	if x.IsFromPool() {
		x.onSendToEndpoint(ctx, msg, data)
		return
	}
	x.onWrite(ctx, msg, data)
}

// onWrite outbound: Connects via SharedNode with ws and locks WriteMessage to send.
func (x *WsNode) onWrite(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	conn, err := x.SharedNode.GetSafely()
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	mt := websocket.TextMessage
	if x.Config.MessageType == "binary" || msg.GetDataType() == types.BINARY {
		mt = websocket.BinaryMessage
	}
	x.mu.Lock()
	err = conn.WriteMessage(mt, data)
	x.mu.Unlock()
	if err != nil {
		// Write failure: Connection is broken
		if x.heartbeatDur > 0 {
			// Activate heartbeat: mark disconnect and trigger automatic reconnection
			if !x.isDisconnected() {
				x.setDisconnected(true)
				x.tryReconnect()
			}
		} else {
			// Disable heartbeat: Reset SharedNode cache, next time GetSafely redials (self-healing, does not start reconnection timer)
			_ = x.SharedNode.Close()
		}
		ctx.TellFailure(msg, err)
		return
	}
	ctx.TellSuccess(msg)
}

// onSendToEndpoint addressing push: resolveRefEndpoint (same-chain priority→NodePool) → TargetSender.SendToTarget.
// Isomorphic with NetNode.onSendToEndpoint and reuses SendToTarget's already encapsulated Lookup+ traversal + statistical logic to avoid behavioral splitting.
func (x *WsNode) onSendToEndpoint(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	target := x.targetResolver.Resolve(ctx, msg)
	// Expression parsing is empty (non-explicit *) and does not broadcast silently to avoid mismatches and full push
	if target == "" && !x.targetResolver.IsEmpty() && x.targetResolver.Literal() != "*" {
		ctx.TellFailure(msg, fmt.Errorf("ws: target %q resolved to empty", x.targetResolver.Literal()))
		return
	}
	inst, found := base.ResolveRefEndpoint(ctx, x.ruleConfig.NodePool, x.InstanceId)
	if !found {
		ctx.TellFailure(msg, fmt.Errorf("ws: ref://%s not found in chain or node pool", x.InstanceId))
		return
	}
	sender, ok := inst.(types.TargetSender)
	if !ok {
		ctx.TellFailure(msg, fmt.Errorf("ws: ref://%s type %T does not support addressing", x.InstanceId, inst))
		return
	}
	sent, failed, err := sender.SendToTarget(target, data)
	if err != nil && sent == 0 {
		ctx.TellFailure(msg, err)
		return
	}
	if failed > 0 {
		x.Printf("ws partial delivery: %d/%d failed for target=%q", failed, sent+failed, target)
	}
	ctx.TellSuccess(msg)
}

// onPing heartbeat: Send a PingMessage to keep it alive. If disconnected, reconnect instead.
func (x *WsNode) onPing() {
	if x.IsFromPool() || atomic.LoadInt32(&x.closed) == 1 {
		return
	}
	if x.isDisconnected() {
		x.tryReconnect()
		return
	}
	conn, err := x.SharedNode.GetSafely()
	if err != nil || conn == nil {
		x.setDisconnected(true)
		x.tryReconnect()
		return
	}
	// WriteControl can be paralleled with WriteMessage without needing to hold mu
	if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
		x.Printf("ws ping failed: %v", err)
		x.setDisconnected(true)
		x.tryReconnect()
		return
	}
	x.resetHeartbeat(x.heartbeatDur)
}

// tryReconnect closes the old connection (resets the SharedNode cache) and then redials.
// CAS ensures that only one reconnection is running at any given time; If the dial fails, onPing will retry with a delay.
func (x *WsNode) tryReconnect() {
	if x.IsFromPool() || atomic.LoadInt32(&x.closed) == 1 {
		return
	}
	if !atomic.CompareAndSwapInt32(&x.reconnecting, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&x.reconnecting, 0)
	// Secondary check closed: Situations where CAS is passed before Destroy is done, to avoid redialing after destruction
	if atomic.LoadInt32(&x.closed) == 1 {
		return
	}

	// Close triggers CloseFunc(conn.Close), causing the old readLoop to fail and exit the ReadMessage as a result;
	// At the same time, reset clientInitialized to enable the next GetSafely redial
	_ = x.SharedNode.Close()
	if _, err := x.SharedNode.GetSafely(); err != nil {
		// initConnect dial-up failed: After a delay, onPing tries again
		x.resetHeartbeat(reconnectDelay)
	} else {
		// Dial-up successful: initConnect has setDisconnected(false) and started a new readLoop
		x.Printf("ws reconnected to %s", x.Config.Server)
		x.resetHeartbeat(x.heartbeatDur)
	}
}

func (x *WsNode) Destroy() {
	atomic.StoreInt32(&x.closed, 1)
	x.stopHeartbeat()
	_ = x.SharedNode.Close()
}

// resetHeartbeat Reset the heartbeat timer to the specified interval.
func (x *WsNode) resetHeartbeat(d time.Duration) {
	x.timerMu.Lock()
	defer x.timerMu.Unlock()
	if x.heartbeatTimer == nil {
		x.heartbeatTimer = time.AfterFunc(d, x.onPing)
	} else {
		x.heartbeatTimer.Reset(d)
	}
}

// stopHeartbeat stops the heartbeat timer (Destroy call).
func (x *WsNode) stopHeartbeat() {
	x.timerMu.Lock()
	defer x.timerMu.Unlock()
	if x.heartbeatTimer != nil {
		x.heartbeatTimer.Stop()
	}
}

func (x *WsNode) Printf(format string, v ...interface{}) {
	x.ruleConfig.Logger.Printf(format, v...)
}

func (x *WsNode) isDisconnected() bool { return atomic.LoadInt32(&x.disconnected) == 1 }
func (x *WsNode) setDisconnected(d bool) {
	if d {
		atomic.StoreInt32(&x.disconnected, 1)
	} else {
		atomic.StoreInt32(&x.disconnected, 0)
	}
}

func (x *WsNode) Desc() string {
	return "WebSocket client: dial ws://server to send, or ref:// endpoint/ws to push by target (IP/deviceId/*). Routes to Success/Failure."
}
