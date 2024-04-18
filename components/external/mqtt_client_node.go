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
	"context"
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"sync"
	"sync/atomic"
	"time"
)

// 规则链节点配置示例：
//
//	{
//	       "id": "s3",
//	       "type": "mqttClient",
//	       "name": "mqtt推送数据",
//	       "debugMode": false,
//	       "configuration": {
//	         "Server": "127.0.0.1:1883",
//	         "Topic": "/device/msg"
//	       }
//	     }

// MqttClientNotInitErr mqttClient 未初始化错误
var MqttClientNotInitErr = errors.New("mqtt client not initialized")

func init() {
	Registry.Add(&MqttClientNode{})
}

type MqttClientNodeConfiguration struct {
	//publish topic
	Topic    string
	Server   string
	Username string
	Password string
	//MaxReconnectInterval 重连间隔 单位秒
	MaxReconnectInterval int
	QOS                  uint8
	CleanSession         bool
	ClientID             string
	CAFile               string
	CertFile             string
	CertKeyFile          string
}

func (x *MqttClientNodeConfiguration) ToMqttConfig() mqtt.Config {
	if x.MaxReconnectInterval < 0 {
		x.MaxReconnectInterval = 60
	}
	return mqtt.Config{
		Server:               x.Server,
		Username:             x.Username,
		Password:             x.Password,
		QOS:                  x.QOS,
		MaxReconnectInterval: time.Duration(x.MaxReconnectInterval) * time.Second,
		CleanSession:         x.CleanSession,
		ClientID:             x.ClientID,
		CAFile:               x.CAFile,
		CertFile:             x.CertFile,
		CertKeyFile:          x.CertKeyFile,
	}
}

type MqttClientNode struct {
	//节点配置
	Config     MqttClientNodeConfiguration
	mqttClient *mqtt.Client
	//锁
	locker sync.RWMutex
	//是否正在连接mqtt 服务器
	connecting int32
}

// Type 组件类型
func (x *MqttClientNode) Type() string {
	return "mqttClient"
}

func (x *MqttClientNode) New() types.Node {
	return &MqttClientNode{Config: MqttClientNodeConfiguration{
		Topic:                "/device/msg",
		Server:               "127.0.0.1:1883",
		QOS:                  0,
		MaxReconnectInterval: 60,
	}}
}

// Init 初始化
func (x *MqttClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		_ = x.tryInitClient()
	}
	return err
}

// OnMsg 处理消息
func (x *MqttClientNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	topic := str.SprintfDict(x.Config.Topic, msg.Metadata.Values())
	if x.mqttClient == nil {
		if err := x.tryInitClient(); err != nil {
			ctx.TellFailure(msg, err)
		} else {
			ctx.TellFailure(msg, MqttClientNotInitErr)
		}
	} else if err := x.mqttClient.Publish(topic, x.Config.QOS, []byte(msg.Data)); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
}

// Destroy 销毁
func (x *MqttClientNode) Destroy() {
	if x.mqttClient != nil {
		_ = x.mqttClient.Close()
	}
}
func (x *MqttClientNode) isConnecting() bool {
	return atomic.LoadInt32(&x.connecting) == 1
}

// TryInitClient 尝试重连mqtt客户端
func (x *MqttClientNode) tryInitClient() error {
	if x.mqttClient == nil && atomic.CompareAndSwapInt32(&x.connecting, 0, 1) {
		var err error
		ctx, cancel := context.WithTimeout(context.TODO(), 4*time.Second)
		defer func() {
			cancel()
			atomic.StoreInt32(&x.connecting, 0)
		}()
		x.mqttClient, err = mqtt.NewClient(ctx, x.Config.ToMqttConfig())
		return err
	} else {
		return nil
	}
}
