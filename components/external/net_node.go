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

// Package external 把msg负荷发送到指定协议网络服务器（不支持读取数据），支持协议：tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。
// 每条消息在内容最后增加结束符：'\n'
package external

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"net"
	"sync/atomic"
	"time"
)

// EndSign 结束符
const EndSign = '\n'

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
	// 心跳间隔，用于定期发送心跳消息，单位为秒，如果<=0 则默认60
	HeartbeatInterval int
}

// NetNode 用于网络协议的数据读取和发送，支持协议：tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。
type NetNode struct {
	// 节点配置
	Config NetNodeConfiguration
	// ruleGo配置
	ruleConfig types.Config
	// 客户端连接对象
	conn net.Conn
	// 创建一个心跳定时器，用于定期发送心跳消息，可以为0表示不发心跳
	heartbeatTimer *time.Timer
	//心跳间隔
	heartbeatDuration time.Duration
	// 用于通知组件已经被销毁
	stop chan struct{}
	// 连接是否已经断开，0：没端口；1：端口
	disconnected int32
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
	x.stop = make(chan struct{}, 1)
	// 将配置转换为NetNodeConfiguration结构体
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if x.Config.Protocol == "" {
			x.Config.Protocol = "tcp"
		}
		if x.Config.ConnectTimeout == 0 {
			x.Config.ConnectTimeout = 60
		}
		if x.Config.HeartbeatInterval == 0 {
			x.Config.HeartbeatInterval = 60
		}
		x.heartbeatDuration = time.Duration(x.Config.HeartbeatInterval) * time.Second

		// 根据配置的协议和地址，创建一个客户端连接
		err = x.onConnect()
		//启动ping、重连和读取服务端数据
		if err == nil {
			go x.start()
		}
	}
	return err
}

// OnMsg 处理消息
func (x *NetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 将消息的数据转换为字节数组
	data := []byte(msg.Data)
	// 在数据的末尾加上结束符
	data = append(data, EndSign)
	x.onWrite(ctx, msg, data)
}

// Destroy 销毁
func (x *NetNode) Destroy() {
	// 发送一个空结构体到chain
	x.stop <- struct{}{}

	close(x.stop)
}

// start 启动
func (x *NetNode) start() {

	// 循环处理服务器的数据和事件
LOOP:
	for {
		// 使用select语句，监听多个通道
		select {
		// 如果接收到stop的消息
		case <-x.stop:
			x.onDisconnect()
			// 退出for循环
			break LOOP
		// 如果有服务器发送的数据
		default:
			if x.isDisconnected() {
				_ = x.onConnect()
				//5秒后重连
				time.Sleep(time.Second * 5)
			} else {
				time.Sleep(x.heartbeatDuration)
			}

		}
	}
}

func (x *NetNode) Printf(format string, v ...interface{}) {
	x.ruleConfig.Logger.Printf(format, v...)
}

func (x *NetNode) onConnect() error {
	//如果有旧连接则先关闭
	if x.conn != nil {
		_ = x.conn.Close()
	}
	// 根据配置的协议和地址，创建一个客户端连接
	conn, err := net.DialTimeout(x.Config.Protocol, x.Config.Server, time.Duration(x.Config.ConnectTimeout)*time.Second)
	if err == nil {
		x.conn = conn
		x.setDisconnected(false)

		if x.heartbeatTimer != nil {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		} else {
			// 创建一个心跳定时器，用于定期发送心跳消息
			x.heartbeatTimer = time.AfterFunc(x.heartbeatDuration, func() {
				x.onPing()
			})
		}
		x.Printf("connect success:", x.conn.RemoteAddr().String())
	} else {
		x.Printf("connect err:", err)
	}
	return err
}

func (x *NetNode) onPing() {
	// 如果连接已经断开，就跳过
	if x.isDisconnected() {
		return
	}
	// 向服务器发送心跳消息
	_, err := x.conn.Write([]byte("ping\n"))
	if err != nil {
		// 如果出现错误，就标记连接断开，并重置重连定时器
		x.setDisconnected(true)
	} else {
		x.heartbeatTimer.Reset(x.heartbeatDuration)
	}
}

func (x *NetNode) onWrite(ctx types.RuleContext, msg types.RuleMsg, data []byte) {
	// 向服务器发送数据
	_, err := x.conn.Write(data)
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		//重置心跳发送间隔
		if x.heartbeatTimer != nil {
			x.heartbeatTimer.Reset(x.heartbeatDuration)
		}
		//发送到下一个阶段
		ctx.TellSuccess(msg)
	}
}

func (x *NetNode) onDisconnect() {
	if x.conn != nil {
		_ = x.conn.Close()
	}
	// 停止心跳定时器
	x.heartbeatTimer.Stop()
	x.setDisconnected(true)
	x.Printf("onDisconnect " + x.conn.RemoteAddr().String())
}

func (x *NetNode) isDisconnected() bool {
	return atomic.LoadInt32(&x.disconnected) == 1
}

func (x *NetNode) setDisconnected(disconnected bool) {
	if disconnected {
		atomic.StoreInt32(&x.disconnected, 1)
	} else {
		atomic.StoreInt32(&x.disconnected, 0)
	}
}
