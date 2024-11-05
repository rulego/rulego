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
	// 心跳间隔，用于定期发送心跳消息，单位为秒，如果<=0 则默认60
	HeartbeatInterval int
}

// NetNode 把消息负荷通过网络协议发送，支持协议：tcp、udp、ip4:1、ip6:ipv6-icmp、ip6:58、unix、unixgram，以及net包支持的协议类型。
// 发送前会在消息负荷最后增加结束符：'\n'
type NetNode struct {
	base.SharedNode[net.Conn]
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
	if err := maps.Map2Struct(configuration, &x.Config); err != nil {
		return err
	}
	// 设置默认值
	x.setDefaultConfig()
	x.heartbeatDuration = time.Duration(x.Config.HeartbeatInterval) * time.Second
	return x.SharedNode.Init(ruleConfig, x.Type(), x.Config.Server, false, x.initConnect)
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
	x.onDisconnect()
}

func (x *NetNode) Printf(format string, v ...interface{}) {
	x.ruleConfig.Logger.Printf(format, v...)
}

// initConnect 方法简化
func (x *NetNode) initConnect() (net.Conn, error) {
	if x.conn != nil && !x.isDisconnected() {
		return x.conn, nil
	}

	x.Locker.Lock()
	defer x.Locker.Unlock()

	if x.conn != nil && !x.isDisconnected() {
		return x.conn, nil
	}

	conn, err := net.DialTimeout(x.Config.Protocol, x.Config.Server, time.Duration(x.Config.ConnectTimeout)*time.Second)
	if err != nil {
		return nil, err
	}

	x.conn = conn
	x.setDisconnected(false)
	// 初始化心跳定时器
	if x.heartbeatTimer == nil {
		x.heartbeatTimer = time.AfterFunc(x.heartbeatDuration, func() {
			x.onPing()
		})
	} else {
		x.heartbeatTimer.Reset(x.heartbeatDuration)
	}
	return conn, nil
}

// 重连
func (x *NetNode) tryReconnect() {
	conn, err := net.DialTimeout(x.Config.Protocol, x.Config.Server, time.Duration(x.Config.ConnectTimeout)*time.Second)
	if err != nil {
		// 5秒后重试
		x.heartbeatTimer.Reset(5 * time.Second)
	} else {
		x.Locker.Lock()
		x.conn = conn
		x.setDisconnected(false)
		x.Locker.Unlock()

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
	if conn, err := x.SharedNode.Get(); err == nil {
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
	if conn, err := x.SharedNode.Get(); err != nil {
		ctx.TellFailure(msg, err)
	} else if _, err := conn.Write(data); err != nil {
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
		// 停止心跳定时器
		if x.heartbeatTimer != nil {
			x.heartbeatTimer.Stop()
		}
		_ = x.conn.Close()
		x.setDisconnected(true)
	}
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

// 抽取配置默认值设置
func (x *NetNode) setDefaultConfig() {
	if x.Config.Protocol == "" {
		x.Config.Protocol = "tcp"
	}
	if x.Config.ConnectTimeout <= 0 {
		x.Config.ConnectTimeout = 60
	}
	if x.Config.HeartbeatInterval <= 0 {
		x.Config.HeartbeatInterval = 60
	}
}
