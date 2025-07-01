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
	"fmt"
	"sync"
	"time"

	"github.com/rulego/rulego/utils/mqtt"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
)

// MqttClientNode 为MQTT代理提供MQTT客户端功能以发布消息的外部组件
// MqttClientNode provides MQTT client functionality for publishing messages to MQTT brokers.
//
// 核心算法：
// Core Algorithm:
// 1. 使用变量替换解析主题模板 - Parse topic template with variable substitution
// 2. 建立MQTT连接并配置QoS参数 - Establish MQTT connection and configure QoS parameters
// 3. 发布消息到指定主题 - Publish message to specified topic
// 4. 支持自动重连和连接池管理 - Support automatic reconnection and connection pool management
//
// 配置说明 - Configuration:
//
//	{
//		"server": "127.0.0.1:1883",          // MQTT broker address  MQTT 代理地址
//		"username": "user",                   // Authentication username  认证用户名
//		"password": "pass",                   // Authentication password  认证密码
//		"topic": "/device/${metadata.deviceId}",  // Publish topic with variable substitution  发布主题支持变量替换
//		"qos": 1,                            // QoS level (0, 1, 2)  QoS 级别
//		"maxReconnectInterval": 60,          // Reconnection interval in seconds  重连间隔（秒）
//		"cleanSession": true,                // Clean session flag  清理会话标志
//		"clientID": "rulegoClient",          // MQTT client identifier  MQTT 客户端标识符
//		"caFile": "/path/to/ca.crt",         // CA certificate file for SSL/TLS  SSL/TLS 的 CA 证书文件
//		"certFile": "/path/to/client.crt",   // Client certificate file  客户端证书文件
//		"certKeyFile": "/path/to/client.key" // Client private key file  客户端私钥文件
//	}
//
// 主题变量替换 - Topic variable substitution:
//   - ${metadata.key}: 访问消息元数据 - Access message metadata
//   - ${msg.key}: 访问消息负荷字段 - Access message payload fields
//
// QoS级别 - QoS levels:
//   - 0: 最多一次投递（发送即忘）- At most once delivery (fire and forget)
//   - 1: 至少一次投递（确认投递）- At least once delivery (acknowledged delivery)
//   - 2: 恰好一次投递（保证投递）- Exactly once delivery (assured delivery)
//
// 连接管理 - Connection management:
//   - 指数退避自动重连直到最大间隔 - Automatic reconnection with exponential backoff up to maximum interval
//   - SharedNode模式高效资源利用 - SharedNode pattern for efficient resource utilization
//
// 使用示例 - Usage example:
//
//	// Publish sensor data with dynamic topic
//	// 使用动态主题发布传感器数据
//	{
//		"id": "mqttPublish",
//		"type": "mqttClient",
//		"configuration": {
//			"server": "mqtt.example.com:8883",
//			"username": "sensor_device",
//			"password": "secret123",
//			"topic": "/sensors/${metadata.deviceType}/${metadata.deviceId}",
//			"qos": 1,
//			"maxReconnectInterval": 30,
//			"cleanSession": true,
//			"clientID": "ruleGo_${metadata.instanceId}",
//			"caFile": "/etc/ssl/ca.crt"
//		}
//	}

func init() {
	Registry.Add(&MqttClientNode{})
}

type MqttClientNodeConfiguration struct {
	Server   string
	Username string
	Password string
	// Topic 发布主题 可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Topic string
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
	base.SharedNode[*mqtt.Client]
	//节点配置
	Config MqttClientNodeConfiguration
	//topic 模板
	topicTemplate str.Template
	client        *mqtt.Client
	clientMutex   sync.RWMutex // Add mutex for thread safety
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
		_ = x.SharedNode.Init(ruleConfig, x.Type(), x.Config.Server, ruleConfig.NodeClientInitNow, func() (*mqtt.Client, error) {
			return x.initClient()
		})
		x.topicTemplate = str.NewTemplate(x.Config.Topic)
	}
	return err
}

// OnMsg 处理消息，使用变量替换解析主题并发布MQTT消息
// OnMsg processes messages by parsing topic with variable substitution and publishing MQTT messages.
func (x *MqttClientNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	// 开始操作，增加活跃操作计数
	x.SharedNode.BeginOp()
	defer x.SharedNode.EndOp()

	// 检查是否正在关闭
	if x.SharedNode.IsShuttingDown() {
		ctx.TellFailure(msg, fmt.Errorf("mqtt client is shutting down"))
		return
	}

	topic := x.topicTemplate.ExecuteFn(func() map[string]any {
		return base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	})

	if client, err := x.SharedNode.Get(); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		// 再次检查是否正在关闭，防止在Get()之后被关闭
		if x.SharedNode.IsShuttingDown() {
			ctx.TellFailure(msg, fmt.Errorf("mqtt client is shutting down"))
			return
		}

		if err := client.Publish(topic, x.Config.QOS, []byte(msg.GetData())); err != nil {
			ctx.TellFailure(msg, err)
		} else {
			ctx.TellSuccess(msg)
		}
	}
}

// Destroy 销毁
func (x *MqttClientNode) Destroy() {
	// 使用优雅关闭机制，等待活跃操作完成后再关闭资源
	x.SharedNode.GracefulShutdown(0, func() {
		// 只在非资源池模式下关闭本地资源
		x.clientMutex.Lock()
		defer x.clientMutex.Unlock()
		if x.client != nil {
			_ = x.client.Close()
			x.client = nil
		}
	})
}

// initClient 初始化客户端
func (x *MqttClientNode) initClient() (*mqtt.Client, error) {
	x.Locker.Lock()
	defer x.Locker.Unlock()

	x.clientMutex.RLock()
	if x.client != nil {
		existingClient := x.client
		x.clientMutex.RUnlock()
		return existingClient, nil
	}
	x.clientMutex.RUnlock()

	ctx, cancel := context.WithTimeout(context.TODO(), 4*time.Second)
	defer cancel()

	client, err := mqtt.NewClient(ctx, x.Config.ToMqttConfig())
	if err == nil {
		x.clientMutex.Lock()
		x.client = client
		x.clientMutex.Unlock()
	}
	return client, err
}
