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

package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	paho "github.com/eclipse/paho.mqtt.golang"
	string2 "github.com/rulego/rulego/utils/str"
	"log"

	"io/ioutil"
	"sync"
	"time"
)

//Handler 订阅数据处理器
type Handler struct {
	//订阅主题
	Topic string
	//订阅Qos
	Qos byte
	//接收订阅数据 处理
	Handle func(c paho.Client, data paho.Message)
}

//Config 客户端配置
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

//Client mqtt客户端
type Client struct {
	sync.RWMutex
	wg     sync.WaitGroup
	client paho.Client
	//订阅主题和处理器映射
	msgHandlerMap map[string]Handler
}

// NewClient 创建一个MQTT客户端实例
func NewClient(conf Config) (*Client, error) {
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
		log.Printf("error loading mqtt certificate files,ca_cert=%s,tls_cert=%s,tls_key=%s", conf.CAFile, conf.CertFile, conf.CertKeyFile)
	}
	//tls
	if tlsconfig != nil {
		opts.SetTLSConfig(tlsconfig)
	}
	log.Printf("connecting to mqtt broker,server=%s", conf.Server)
	b.client = paho.NewClient(opts)
	for {
		if token := b.client.Connect(); token.Wait() && token.Error() != nil {
			log.Printf("connecting to mqtt broker failed, will retry in 2s: %s", token.Error())
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	return &b, nil
}

//RegisterHandler 注册订阅数据处理器
func (b *Client) RegisterHandler(handler Handler) {
	b.Lock()
	defer b.Unlock()
	b.msgHandlerMap[handler.Topic] = handler
	b.subscribeHandler(handler)
}

//UnregisterHandler 删除订阅数据处理器
func (b *Client) UnregisterHandler(topic string) error {
	if token := b.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		b.Lock()
		defer b.Unlock()
		delete(b.msgHandlerMap, topic)
		return nil
	}
}

//GetHandlerByUpTopic 通过主题获取数据处理器
func (b *Client) GetHandlerByUpTopic(topic string) Handler {
	b.RLock()
	defer b.RUnlock()
	return b.msgHandlerMap[topic]
}

func (b *Client) Close() error {
	for _, v := range b.msgHandlerMap {
		b.client.Unsubscribe(v.Topic)
	}
	return nil
}

//Publish 发布数据
func (b *Client) Publish(topic string, qos byte, data []byte) error {
	if token := b.client.Publish(topic, qos, false, data); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		return nil
	}

}

func (b *Client) onConnected(c paho.Client) {
	log.Printf("connected to mqtt server")
	b.subscribe()

}

func (b *Client) subscribe() {
	for _, handler := range b.msgHandlerMap {
		b.subscribeHandler(handler)
	}
}

func (b *Client) subscribeHandler(handler Handler) {
	topic := handler.Topic
	for {
		log.Printf("subscribing to topic,topic=%s,qos=%d", topic, int(handler.Qos))
		if token := b.client.Subscribe(topic, handler.Qos, handler.Handle).(*paho.SubscribeToken); token.Wait() && (token.Error() != nil || is128Err(token, topic)) { //128 ACK错误
			log.Printf("subscribe error,topic=%s,qos=%d", topic, int(handler.Qos))
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

//判断是否是acl 128错误
func is128Err(token *paho.SubscribeToken, topic string) bool {
	result, ok := token.Result()[topic]
	return ok && result == 128
}

func (b *Client) onConnectionLost(c paho.Client, reason error) {
	log.Printf("mqtt connection error: %s", reason)
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
			log.Printf("could not load ca certificate")
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
			log.Printf("could not load mqtt tls key-pair")
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{kp}
	}
	return tlsConfig, nil
}
