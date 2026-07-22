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

// writeWait 单次写控制帧（ping）的超时
const writeWait = 10 * time.Second

// reconnectDelay 拨号失败后的重试间隔
const reconnectDelay = 5 * time.Second

func init() {
	Registry.Add(&WsNode{})
}

// WsNodeConfiguration ws 客户端节点配置
type WsNodeConfiguration struct {
	// Server 直接填 ws://host:port 拨号出站；或 ref:// 指向 endpoint/ws 寻址推送
	Server string `json:"server" label:"Server" desc:"ws://host:port 拨号发送；或 ref:// 指向 endpoint/ws 寻址推送" required:"true" ref:"primary"`
	// Target 寻址目标，仅 server 为 ref:// 时生效。IP/deviceId/*（广播）；支持 ${metadata.xxx}
	Target string `json:"target" label:"Target" desc:"仅 ref:// 时生效。IP/deviceId/* (broadcast); supports ${metadata.xxx}"`

	// 以下为出站客户端配置（仅 server 为 ws://wss:// 即拨号模式时生效，ref:// 模式忽略）
	// Headers 握手时的 HTTP 请求头，如 Authorization
	Headers map[string]string `json:"headers" label:"Headers" desc:"Handshake HTTP headers, e.g. Authorization"`
	// Subprotocol Sec-WebSocket-Protocol 子协议（拨号模式），如 mqtt / ocpp1.6。留空不协商
	Subprotocol string `json:"subprotocol" label:"Subprotocol" desc:"Sec-WebSocket-Protocol, e.g. mqtt / ocpp1.6. Leave empty to skip"`
	// ConnectTimeout 拨号握手超时（秒），0 用默认
	ConnectTimeout int `json:"connectTimeout" label:"Connect Timeout (s)" desc:"Dial handshake timeout in seconds, 0=default"`
	// InsecureSkipVerify 跳过 TLS 证书验证（wss:// 自签证书场景），与 restApiCall 一致
	InsecureSkipVerify bool `json:"insecureSkipVerify" label:"Skip TLS Verify" desc:"Set to true to skip HTTPS certificate verification"`
	// MessageType 发送的消息类型：text/binary（默认 text）
	MessageType string `json:"messageType" label:"Message Type" desc:"text or binary (default text)"`
	// HeartbeatInterval 心跳间隔（秒）。>0 定期发 PingMessage 保活，连接断开自动重连；0=禁用心跳与重连
	HeartbeatInterval int `json:"heartbeatInterval" label:"Heartbeat Interval (s)" desc:"Heartbeat interval in seconds. 0=disable heartbeat and reconnect"`
}

// WsNode 是 WebSocket 客户端节点，对称 NetNode：出站拨号（ws://）+ ref:// 寻址推送（endpoint/ws）。
type WsNode struct {
	base.SharedNode[*websocket.Conn]
	Config     WsNodeConfiguration
	ruleConfig types.Config
	// target 解析器（Init 时预编译模板），ref:// 模式使用
	targetResolver *base.TargetResolver

	// 以下仅出站模式（server 非 ref://）使用
	// mu 保护业务 WriteMessage（gorilla 的 WriteMessage 不支持并发写；
	//   WriteControl 可与 WriteMessage 并发，故 ping 不持此锁）
	mu sync.Mutex
	// timerMu 保护 heartbeatTimer 字段读写（initConnect/onPing/tryReconnect/Destroy 可能并发访问）
	timerMu        sync.Mutex
	heartbeatTimer *time.Timer
	heartbeatDur   time.Duration
	disconnected   int32 // 0=正常 1=已断开待重连
	reconnecting   int32 // CAS 防重连重入
	closed         int32 // Destroy 后置 1，阻止重连/心跳
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

// initConnect 出站拨号：连远端 ws server，应用 Headers/Subprotocols/ConnectTimeout/InsecureSkipVerify。
// 仅在 server 非 ref:// 时执行（ref:// 走 NodePool）。拨号成功后启动读循环与心跳。
func (x *WsNode) initConnect() (*websocket.Conn, error) {
	dialer := *websocket.DefaultDialer // 拷贝，避免污染全局
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
	// 心跳开启且未销毁时才启动读循环：消费对端回的 pong，并在连接断开时触发重连
	if x.heartbeatDur > 0 && atomic.LoadInt32(&x.closed) == 0 {
		go x.readLoop(c)
		x.resetHeartbeat(x.heartbeatDur)
	}
	return c, nil
}

// readLoop 持续读取以处理对端 pong 控制帧（WsNode 只发不收，业务消息丢弃）。读 error 触发重连。
func (x *WsNode) readLoop(c *websocket.Conn) {
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if atomic.LoadInt32(&x.closed) == 1 {
				return // Destroy 触发的关闭，正常退出
			}
			x.setDisconnected(true)
			x.tryReconnect()
			return
		}
	}
}

// OnMsg server 为 ref:// 时寻址推送，否则出站 WriteMessage。
func (x *WsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	data := msg.GetBytes()
	if x.IsFromPool() {
		x.onSendToEndpoint(ctx, msg, data)
		return
	}
	x.onWrite(ctx, msg, data)
}

// onWrite 出站：经 SharedNode 拿 ws 连接，加锁 WriteMessage 发送。
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
		// 写失败：连接已坏
		if x.heartbeatDur > 0 {
			// 开启心跳：标记断开并触发自动重连
			if !x.isDisconnected() {
				x.setDisconnected(true)
				x.tryReconnect()
			}
		} else {
			// 禁用心跳：重置 SharedNode 缓存，下次 GetSafely 重新拨号（自愈，不启动重连定时器）
			_ = x.SharedNode.Close()
		}
		ctx.TellFailure(msg, err)
		return
	}
	ctx.TellSuccess(msg)
}

// onSendToEndpoint 寻址推送：resolveRefEndpoint(同链优先→NodePool) → TargetSender.SendToTarget。
// 与 NetNode.onSendToEndpoint 同构，复用 SendToTarget 已封装的 Lookup+遍历+统计逻辑，避免行为分裂。
func (x *WsNode) onSendToEndpoint(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	target := x.targetResolver.Resolve(ctx, msg)
	// 表达式解析为空（非显式 *）不静默广播，避免配错变成全量推送
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

// onPing 心跳：发 PingMessage 保活。已断开则改触发重连。
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
	// WriteControl 可与 WriteMessage 并发，无需持 mu
	if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
		x.Printf("ws ping failed: %v", err)
		x.setDisconnected(true)
		x.tryReconnect()
		return
	}
	x.resetHeartbeat(x.heartbeatDur)
}

// tryReconnect 关闭旧连接（重置 SharedNode 缓存）后重新拨号。
// CAS 保证同一时刻只有一个重连在跑；拨号失败则延时由 onPing 重试。
func (x *WsNode) tryReconnect() {
	if x.IsFromPool() || atomic.LoadInt32(&x.closed) == 1 {
		return
	}
	if !atomic.CompareAndSwapInt32(&x.reconnecting, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&x.reconnecting, 0)
	// 二次检查 closed：CAS 通过后才 Destroy 的情况，避免销毁后还重新拨号
	if atomic.LoadInt32(&x.closed) == 1 {
		return
	}

	// Close 触发 CloseFunc（conn.Close），旧 readLoop 随之 ReadMessage 失败退出；
	// 同时重置 clientInitialized，使下一次 GetSafely 重新拨号
	_ = x.SharedNode.Close()
	if _, err := x.SharedNode.GetSafely(); err != nil {
		// initConnect 拨号失败：延时后由 onPing 再次尝试
		x.resetHeartbeat(reconnectDelay)
	} else {
		// 拨号成功：initConnect 内已 setDisconnected(false) 并启动新 readLoop
		x.Printf("ws reconnected to %s", x.Config.Server)
		x.resetHeartbeat(x.heartbeatDur)
	}
}

func (x *WsNode) Destroy() {
	atomic.StoreInt32(&x.closed, 1)
	x.stopHeartbeat()
	_ = x.SharedNode.Close()
}

// resetHeartbeat 重置心跳定时器为指定间隔。
func (x *WsNode) resetHeartbeat(d time.Duration) {
	x.timerMu.Lock()
	defer x.timerMu.Unlock()
	if x.heartbeatTimer == nil {
		x.heartbeatTimer = time.AfterFunc(d, x.onPing)
	} else {
		x.heartbeatTimer.Reset(d)
	}
}

// stopHeartbeat 停止心跳定时器（Destroy 调用）。
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
