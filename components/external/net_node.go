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
	"net"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
)

// EndSign 结束符
const EndSign = '\n'

// PingData ping内容
var PingData = []byte("ping\n")

// 注册节点
func init() {
	Registry.Add(&NetNode{})
}

// NetNodeConfiguration 组件的配置
type NetNodeConfiguration struct {
	// 通信协议，可以是tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。默认tcp协议
	Protocol string
	// 服务器的地址，格式为host:port
	Server string
	// 连接超时，单位为秒，如果<=0 则默认60
	ConnectTimeout int
	// 心跳间隔，用于定期发送心跳消息，单位为秒，如果=0，则不发心跳包。默认60
	HeartbeatInterval int
}

// NetNode provides network protocol communication capabilities for sending messages over various protocols.
// It supports TCP, UDP, IP, Unix sockets, and other protocols supported by Go's net package,
// with automatic heartbeat, reconnection, and connection lifecycle management.
//
// NetNode 为通过各种协议发送消息提供网络协议通信能力。
// 支持 TCP、UDP、IP、Unix 套接字和 Go net 包支持的其他协议，
// 具有自动心跳、重连和连接生命周期管理功能。
//
// Configuration:
// 配置说明：
//
//	{
//		"protocol": "tcp",              // Network protocol  网络协议
//		"server": "192.168.1.100:8080", // Server address  服务器地址
//		"connectTimeout": 30,           // Connection timeout in seconds  连接超时（秒）
//		"heartbeatInterval": 60         // Heartbeat interval in seconds (0=disabled)  心跳间隔（秒，0=禁用）
//	}
//
// Supported Protocols:
// 支持的协议：
//
//   - "tcp": TCP protocol for reliable, connection-oriented communication
//     TCP 协议，用于可靠的面向连接通信
//   - "udp": UDP protocol for fast, connectionless communication
//     UDP 协议，用于快速的无连接通信
//   - "ip4:1", "ip6:ipv6-icmp", "ip6:58": Raw IP protocols
//     原始 IP 协议
//   - "unix", "unixgram": Unix domain sockets for local communication
//     Unix 域套接字，用于本地通信
//   - Any protocol supported by Go's net.Dial function
//     Go net.Dial 函数支持的任何协议
//
// Smart Data Type Handling:
// 智能数据类型处理：
//
// The component intelligently handles different data types:
// 组件智能处理不同的数据类型：
//   - BINARY: Uses GetBytes(), sends raw bytes without terminator
//     二进制：使用 GetBytes()，发送原始字节不添加终止符
//   - JSON/TEXT: Uses GetData(), appends newline terminator ('\n')
//     JSON/文本：使用 GetData()，追加换行符终止符（'\n'）
//
// Message Format:
// 消息格式：
//
// For non-binary data, messages are sent with an automatic newline terminator ('\n') appended.
// Binary data is sent as-is without any modifications.
// This ensures proper message framing while preserving binary data integrity.
//
// 对于非二进制数据，消息发送时自动追加换行符终止符（'\n'）。
// 二进制数据按原样发送，不做任何修改。
// 这确保了正确的消息帧同时保持二进制数据的完整性。
//
// Connection Management:
// 连接管理：
//
// The component implements:
// 组件实现：
//   - Automatic connection establishment and reconnection  自动连接建立和重连
//   - Configurable heartbeat with ping mechanism  可配置的心跳和 ping 机制
//   - Connection pooling through SharedNode pattern  通过 SharedNode 模式的连接池
//   - Graceful connection cleanup on destroy  销毁时的优雅连接清理
//
// Heartbeat Mechanism:
// 心跳机制：
//
// When heartbeatInterval > 0, the component sends periodic "ping\n" messages
// to maintain connection liveness. Failed heartbeats trigger automatic reconnection.
//
// 当 heartbeatInterval > 0 时，组件发送周期性的 "ping\n" 消息来维持连接活性。
// 心跳失败会触发自动重连。
//
// Error Handling and Reconnection:
// 错误处理和重连：
//
// The component includes robust error handling with automatic reconnection on:
// 组件包含强大的错误处理，在以下情况自动重连：
//   - Connection timeouts  连接超时
//   - Network errors during message sending  消息发送期间的网络错误
//   - Heartbeat failures  心跳失败
//   - Server disconnections  服务器断开连接
//
// Thread Safety:
// 线程安全：
//
// The component uses atomic operations and mutex locks to ensure safe concurrent
// access across multiple rule chain executions.
//
// 组件使用原子操作和互斥锁确保多个规则链执行间的安全并发访问。
//
// Output Relations:
// 输出关系：
//
//   - Success: Message sent successfully  消息发送成功
//   - Failure: Network error or connection failure  网络错误或连接失败
//
// Usage Examples:
// 使用示例：
//
//	// TCP client for sending JSON telemetry data
//	// 用于发送 JSON 遥测数据的 TCP 客户端
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
//	// 用于发送二进制数据的 UDP 客户端（不添加终止符）
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
	// 节点配置
	Config NetNodeConfiguration
	// ruleGo配置
	ruleConfig types.Config
	// 创建一个心跳定时器，用于定期发送心跳消息，可以为0表示不发心跳
	heartbeatTimer *time.Timer
	//心跳间隔
	heartbeatDuration time.Duration
	// 连接是否已经断开，0：没端口；1：端口
	disconnected int32
	//断开连接次数
	disconnectedCount int32
}

// Type 组件类型
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

// Init 初始化
func (x *NetNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	x.ruleConfig = ruleConfig
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	// 设置默认值
	x.setDefaultConfig()
	x.heartbeatDuration = time.Duration(x.Config.HeartbeatInterval) * time.Second
	return x.SharedNode.InitWithClose(ruleConfig, x.Type(), x.Config.Server, ruleConfig.NodeClientInitNow, x.initConnect, func(conn net.Conn) error {
		// 清理回调函数：关闭连接并清理相关状态
		x.onDisconnect()
		return conn.Close()
	})
}

// OnMsg 处理消息
func (x *NetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var data []byte

	// 根据数据类型智能处理
	if msg.GetDataType() == types.BINARY {
		// 二进制数据：直接获取字节数组，不添加结束符
		data = msg.GetBytes()
	} else {
		// 文本或JSON数据：获取字符串并添加换行符结束符
		strData := msg.GetData()
		data = []byte(strData)
		data = append(data, EndSign)
	}

	x.onWrite(ctx, msg, data)
}

// Destroy 销毁
func (x *NetNode) Destroy() {
	_ = x.SharedNode.Close()
}

func (x *NetNode) Printf(format string, v ...interface{}) {
	x.ruleConfig.Logger.Printf(format, v...)
}

// initConnect 方法简化
func (x *NetNode) initConnect() (net.Conn, error) {
	conn, err := net.DialTimeout(x.Config.Protocol, x.Config.Server, time.Duration(x.Config.ConnectTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	x.setDisconnected(false)
	if x.heartbeatDuration != 0 {
		// 初始化心跳定时器
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

// 重连
func (x *NetNode) tryReconnect() {
	// 尝试通过SharedNode获取新连接（会触发重新初始化）
	if conn, err := x.SharedNode.GetSafely(); err != nil {
		// 5秒后重试
		x.heartbeatTimer.Reset(5 * time.Second)
	} else {
		x.setDisconnected(false)
		x.Printf("Reconnected to: %s", conn.RemoteAddr().String())
		// 重连成功后，重置为正常的心跳间隔
		x.heartbeatTimer.Reset(x.heartbeatDuration)
	}
}

func (x *NetNode) onPing() {
	// 如果连接已经断开，尝试重连
	if x.isDisconnected() {
		x.tryReconnect()
		return
	}
	// 发送心跳
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
	// 向服务器发送数据
	if conn, err := x.SharedNode.GetSafely(); err != nil {
		ctx.TellFailure(msg, err)
	} else if _, err := conn.Write(data); err != nil {
		if atomic.LoadInt32(&x.disconnectedCount) == 0 {
			x.setDisconnected(true)
			//重试一次
			x.onWrite(ctx, msg, data)
		} else {
			x.setDisconnected(true)
			ctx.TellFailure(msg, err)
		}
	} else {
		//重置心跳发送间隔
		if x.heartbeatTimer != nil {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		}
		//发送到下一个节点
		ctx.TellSuccess(msg)
	}
}

func (x *NetNode) onDisconnect() {
	// 停止心跳定时器
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

// 默认值设置
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
