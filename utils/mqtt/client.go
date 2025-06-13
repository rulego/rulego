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

// Package mqtt provides MQTT client functionality for the RuleGo rule engine.
//
// This package implements an MQTT client using the Paho MQTT library, allowing
// for communication with MQTT brokers. It includes functionality for connecting
// to MQTT brokers, publishing messages, and subscribing to topics.
//
// Key components:
// - Config: Struct for configuring the MQTT client connection.
// - Client: The main struct representing the MQTT client.
// - Handler: Struct for defining subscription handlers.
//
// The package supports features such as:
// - TLS/SSL connections
// - Authentication with username and password
// - Automatic reconnection
// - QoS levels for publishing and subscribing
// - Custom message handlers for subscriptions
//
// This package is crucial for components that require MQTT communication,
// such as the MqttNode in the external package.
package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	paho "github.com/eclipse/paho.mqtt.golang"
	string2 "github.com/rulego/rulego/utils/str"

	"io/ioutil"
	"sync"
	"time"
)

// Handler 订阅数据处理器
type Handler struct {
	//订阅主题
	Topic string
	//订阅Qos
	Qos byte
	//接收订阅数据 处理
	Handle func(c paho.Client, data paho.Message)
}

// Config 客户端配置
type Config struct {
	//mqtt broker 地址
	Server string
	//用户名
	Username string
	//密码
	Password string
	//重连重试间隔
	MaxReconnectInterval time.Duration
	QOS                  uint8
	CleanSession         bool
	//client Id
	ClientID    string
	CAFile      string
	CertFile    string
	CertKeyFile string
}

// Client mqtt客户端
type Client struct {
	sync.RWMutex
	wg     sync.WaitGroup
	client paho.Client
	//订阅主题和处理器映射
	msgHandlerMap map[string]Handler
}

// NewClient 创建一个MQTT客户端实例
func NewClient(ctx context.Context, conf Config) (*Client, error) {
	var err error

	b := Client{
		msgHandlerMap: make(map[string]Handler),
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(conf.Server)
	opts.SetUsername(conf.Username)
	opts.SetPassword(conf.Password)
	opts.SetCleanSession(conf.CleanSession)
	if conf.ClientID == "" {
		//随机clientId
		opts.SetClientID("rulego/" + string2.RandomStr(8))
	} else {
		opts.SetClientID(conf.ClientID)
	}
	opts.SetOnConnectHandler(b.onConnected)
	opts.SetConnectionLostHandler(b.onConnectionLost)
	if conf.MaxReconnectInterval <= 0 {
		conf.MaxReconnectInterval = time.Second * 60
	}
	opts.SetMaxReconnectInterval(conf.MaxReconnectInterval)

	tlsconfig, err := newTLSConfig(conf.CAFile, conf.CertFile, conf.CertKeyFile)
	if err != nil {
		return nil, fmt.Errorf("error loading mqtt certificate files,ca_cert=%s,tls_cert=%s,tls_key=%s", conf.CAFile, conf.CertFile, conf.CertKeyFile)
	}
	//tls
	if tlsconfig != nil {
		opts.SetTLSConfig(tlsconfig)
	}
	b.client = paho.NewClient(opts)

	for {
		if token := b.client.Connect(); token.Wait() && token.Error() != nil {
			select {
			case <-ctx.Done():
				//context被取消或超时，返回错误
				return nil, token.Error()
			case <-time.After(2 * time.Second):
				//定时器到期，继续重试
			}
		} else {
			break
		}
	}

	return &b, nil
}

// RegisterHandler 注册订阅数据处理器
func (b *Client) RegisterHandler(handler Handler) {
	b.Lock()
	defer b.Unlock()
	b.msgHandlerMap[handler.Topic] = handler
	b.subscribeHandler(handler)
}

// UnregisterHandler 删除订阅数据处理器
func (b *Client) UnregisterHandler(topic string) error {
	b.Lock()
	defer b.Unlock()

	// Check if handler exists before unsubscribing
	if _, exists := b.msgHandlerMap[topic]; !exists {
		return nil // Already unregistered, no error
	}

	if token := b.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		delete(b.msgHandlerMap, topic)
		return nil
	}
}

// GetHandlerByUpTopic 通过主题获取数据处理器
func (b *Client) GetHandlerByUpTopic(topic string) Handler {
	b.RLock()
	defer b.RUnlock()
	return b.msgHandlerMap[topic]
}

func (b *Client) Close() error {
	b.RLock()
	// Create a copy to avoid holding lock during unsubscribe operations
	handlers := make([]Handler, 0, len(b.msgHandlerMap))
	for _, v := range b.msgHandlerMap {
		handlers = append(handlers, v)
	}
	b.RUnlock()

	// Unsubscribe from all topics without holding locks
	for _, v := range handlers {
		b.client.Unsubscribe(v.Topic)
	}
	b.client.Disconnect(500)
	return nil
}

// Publish 发布数据
func (b *Client) Publish(topic string, qos byte, data []byte) error {
	if token := b.client.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		return nil
	}
}

func (b *Client) onConnected(c paho.Client) {
	b.subscribe()
}

func (b *Client) subscribe() {
	b.RLock()
	// 创建处理器副本以避免在迭代过程中持有锁
	handlers := make([]Handler, 0, len(b.msgHandlerMap))
	for _, handler := range b.msgHandlerMap {
		handlers = append(handlers, handler)
	}
	b.RUnlock()

	// 在不持有锁的情况下订阅
	for _, handler := range handlers {
		b.subscribeHandler(handler)
	}
}

func (b *Client) subscribeHandler(handler Handler) {
	topic := handler.Topic
	for {
		if token := b.client.Subscribe(topic, handler.Qos, handler.Handle).(*paho.SubscribeToken); token.Wait() && (token.Error() != nil || is128Err(token, topic)) { //128 ACK错误
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

// 判断是否是acl 128错误
func is128Err(token *paho.SubscribeToken, topic string) bool {
	result, ok := token.Result()[topic]
	return ok && result == 128
}

func (b *Client) onConnectionLost(c paho.Client, reason error) {
}

func newTLSConfig(CAFile, certFile, certKeyFile string) (*tls.Config, error) {
	if CAFile == "" && certFile == "" && certKeyFile == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	// Import trusted certificates from CAFile.pem.
	if CAFile != "" {
		caCert, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCert)

		tlsConfig.RootCAs = certPool // RootCAs = certs used to verify server cert.
	}

	// Import certificate and the key
	if certFile != "" && certKeyFile != "" {
		kp, err := tls.LoadX509KeyPair(certFile, certKeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{kp}
	}
	return tlsConfig, nil
}
