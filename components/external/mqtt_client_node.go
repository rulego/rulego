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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"time"
)

//规则链节点配置示例：
// {
//        "id": "s3",
//        "type": "mqttClient",
//        "name": "mqtt推送数据",
//        "debugMode": false,
//        "configuration": {
//          "Server": "127.0.0.1:1883",
//          "Topic": "/device/msg"
//        }
//      }
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
}

//Type 组件类型
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

//Init 初始化
func (x *MqttClientNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.TODO(), 16*time.Second)
		defer cancel()
		x.mqttClient, err = mqtt.NewClient(ctx, x.Config.ToMqttConfig())
	}
	return err
}

//OnMsg 处理消息
func (x *MqttClientNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	topic := str.SprintfDict(x.Config.Topic, msg.Metadata.Values())
	err := x.mqttClient.Publish(topic, x.Config.QOS, []byte(msg.Data))
	if err != nil {
		ctx.TellFailure(msg, err)
	} else {
		ctx.TellSuccess(msg)
	}
	return err
}

//Destroy 销毁
func (x *MqttClientNode) Destroy() {
	if x.mqttClient != nil {
		_ = x.mqttClient.Close()
	}
}
