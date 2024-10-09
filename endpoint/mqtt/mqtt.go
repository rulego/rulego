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

// Package mqtt provides an MQTT endpoint implementation for the RuleGo framework.
// It allows creating MQTT clients that can subscribe to topics and process incoming MQTT messages,
// routing them to appropriate rule chains or components for further processing.
//
// Key components in this package include:
// - Endpoint (alias Mqtt): Implements the MQTT client and message handling
// - RequestMessage: Represents an incoming MQTT message
// - ResponseMessage: Represents the MQTT message to be published as a response
//
// The MQTT endpoint supports dynamic routing configuration, allowing users to
// define topic subscriptions and their corresponding rule chain or component destinations.
// It also provides flexibility in handling different MQTT QoS levels and message formats.
//
// This package integrates with the broader RuleGo ecosystem, enabling seamless
// data flow from MQTT messages to rule processing and back to MQTT responses.
package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/runtime"
)

// Type 组件类型
const Type = types.EndpointTypePrefix + "mqtt"
const (
	// KeyResponseTopic 响应主题metadataKey
	KeyResponseTopic = "responseTopic"
	// KeyResponseQos 响应Qos metadataKey
	KeyResponseQos = "responseQos"
)

// Endpoint 别名
type Endpoint = Mqtt

// RequestMessage http请求消息
type RequestMessage struct {
	headers textproto.MIMEHeader
	request paho.Message
	body    []byte
	msg     *types.RuleMsg
	err     error
}

// Body 获取请求体
func (r *RequestMessage) Body() []byte {
	if r.body == nil && r.request != nil {
		r.body = r.request.Payload()
	}
	return r.body
}

func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	if r.request != nil {
		r.headers.Set("topic", r.request.Topic())
	}
	return r.headers
}

// From 获取主题
func (r *RequestMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.Topic()
}

// GetParam 不提供获取参数
func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		//默认指定是JSON格式，如果不是该类型，请在process函数中修改
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))

		ruleMsg.Metadata.PutValue("topic", r.From())

		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode 不提供设置状态码
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

// SetBody 设置消息体
func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *RequestMessage) SetError(err error) {
	r.err = err
}

func (r *RequestMessage) GetError() error {
	return r.err
}

func (r *RequestMessage) Request() paho.Message {
	return r.request
}

// ResponseMessage http响应消息
type ResponseMessage struct {
	headers  textproto.MIMEHeader
	request  paho.Message
	response paho.Client
	body     []byte
	msg      *types.RuleMsg
	err      error
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *ResponseMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.Topic()
}

// GetParam 不提供获取参数
func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

// 从msg.Metadata或者响应头获取
func (r *ResponseMessage) getMetadataValue(metadataName, headerName string) string {
	var v string
	if r.GetMsg() != nil {
		metadata := r.GetMsg().Metadata
		v = metadata.GetValue(metadataName)
	}
	if v == "" {
		return r.Headers().Get(headerName)
	} else {
		return v
	}
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	topic := r.getMetadataValue(KeyResponseTopic, KeyResponseTopic)
	if topic != "" {
		qosStr := r.getMetadataValue(KeyResponseQos, KeyResponseQos)
		qos := byte(0)
		if qosStr != "" {
			qosInt, _ := strconv.Atoi(qosStr)
			qos = byte(qosInt)
		}
		r.response.Publish(topic, qos, false, r.body)
	}
}

func (r *ResponseMessage) SetError(err error) {
	r.err = err
}

func (r *ResponseMessage) GetError() error {
	return r.err
}

func (r *ResponseMessage) Response() paho.Client {
	return r.response
}

// Mqtt MQTT 接收端端点
type Mqtt struct {
	impl.BaseEndpoint
	base.SharedNode[*mqtt.Client]
	RuleConfig types.Config
	Config     mqtt.Config
	client     *mqtt.Client
	started    bool
}

// Type 组件类型
func (x *Mqtt) Type() string {
	return Type
}

func (x *Mqtt) New() types.Node {
	return &Mqtt{Config: mqtt.Config{
		Server: "127.0.0.1:1883",
	}}
}

// Init 初始化
func (x *Mqtt) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	x.RuleConfig = ruleConfig
	_ = x.SharedNode.Init(x.RuleConfig, x.Type(), x.Config.Server, true, func() (*mqtt.Client, error) {
		return x.initClient()
	})
	return err
}

// Destroy 销毁
func (x *Mqtt) Destroy() {
	_ = x.Close()
}

func (x *Mqtt) Close() error {
	if x.client != nil {
		return x.client.Close()
	}
	return nil
}

func (x *Mqtt) Id() string {
	return x.Config.Server
}

func (x *Mqtt) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	x.CheckAndSetRouterId(router)
	x.saveRouter(router)
	//服务已经启动
	if x.started {
		if form := router.GetFrom(); form != nil {
			client, err := x.SharedNode.Get()
			if err != nil {
				return "", err
			}
			client.RegisterHandler(mqtt.Handler{
				Topic:  form.ToString(),
				Qos:    x.Config.QOS,
				Handle: x.handler(router),
			})
		}
	}
	return router.GetId(), nil
}

func (x *Mqtt) RemoveRouter(routerId string, params ...interface{}) error {
	router := x.deleteRouter(routerId)
	if router != nil {
		client, _ := x.SharedNode.Get()
		if client != nil {
			return client.UnregisterHandler(router.FromToString())
		} else {
			return nil
		}
	} else {
		return fmt.Errorf("router: %s not found", routerId)
	}
}

func (x *Mqtt) Start() error {
	if x.started {
		return nil
	}
	client, err := x.SharedNode.Get()
	if err != nil {
		return err
	}
	for _, router := range x.RouterStorage {
		if form := router.GetFrom(); form != nil {
			client.RegisterHandler(mqtt.Handler{
				Topic:  form.ToString(),
				Qos:    x.Config.QOS,
				Handle: x.handler(router),
			})
		}
	}
	x.started = true
	return nil
}

// 存储路由
func (x *Mqtt) saveRouter(routers ...endpoint.Router) {
	x.Lock()
	defer x.Unlock()
	if x.RouterStorage == nil {
		x.RouterStorage = make(map[string]endpoint.Router)
	}
	for _, item := range routers {
		x.RouterStorage[item.GetId()] = item
	}
}

// 从存储器中删除路由
func (x *Mqtt) deleteRouter(id string) endpoint.Router {
	x.Lock()
	defer x.Unlock()
	if x.RouterStorage != nil {
		if router, ok := x.RouterStorage[id]; ok {
			delete(x.RouterStorage, id)
			return router
		}
	}
	return nil
}

func (x *Mqtt) handler(router endpoint.Router) func(c paho.Client, data paho.Message) {
	return func(c paho.Client, data paho.Message) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				x.Printf("mqtt endpoint handler err :\n%v", runtime.Stack())
			}
		}()
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: data,
			},
			Out: &ResponseMessage{
				request:  data,
				response: c,
			}}

		x.DoProcess(context.Background(), router, exchange)
	}
}

func (x *Mqtt) Printf(format string, v ...interface{}) {
	if x.RuleConfig.Logger != nil {
		x.RuleConfig.Logger.Printf(format, v...)
	}
}

// initClient 初始化客户端
func (x *Mqtt) initClient() (*mqtt.Client, error) {
	if x.client != nil {
		return x.client, nil
	} else {
		ctx, cancel := context.WithTimeout(context.TODO(), 4*time.Second)
		x.Lock()
		defer func() {
			cancel()
			x.Unlock()
		}()
		if x.client != nil {
			return x.client, nil
		}
		var err error
		x.client, err = mqtt.NewClient(ctx, x.Config)
		return x.client, err
	}
}
