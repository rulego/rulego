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
	"context"
	"errors"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/runtime"
	"net/textproto"
	"strconv"
	"time"
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
	RuleConfig types.Config
	Config     mqtt.Config
	client     *mqtt.Client
}

// Type 组件类型
func (m *Mqtt) Type() string {
	return Type
}

func (m *Mqtt) New() types.Node {
	return &Mqtt{Config: mqtt.Config{
		Server: "127.0.0.1:1883",
	}}
}

// Init 初始化
func (m *Mqtt) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &m.Config)
	m.RuleConfig = ruleConfig
	return err
}

// Destroy 销毁
func (m *Mqtt) Destroy() {
	_ = m.Close()
}

func (m *Mqtt) Close() error {
	if nil != m.client {
		return m.client.Close()
	}
	return nil
}

func (m *Mqtt) Id() string {
	return m.Config.Server
}

func (m *Mqtt) AddRouter(router endpoint.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", errors.New("router can not nil")
	}
	m.CheckAndSetRouterId(router)
	m.saveRouter(router)
	//服务已经启动
	if m.client != nil {
		if form := router.GetFrom(); form != nil {
			m.client.RegisterHandler(mqtt.Handler{
				Topic:  form.ToString(),
				Qos:    m.Config.QOS,
				Handle: m.handler(router),
			})
		}
	}
	return router.GetId(), nil
}

func (m *Mqtt) RemoveRouter(routerId string, params ...interface{}) error {
	router := m.deleteRouter(routerId)
	if router != nil {
		if m.client != nil {
			return m.client.UnregisterHandler(router.FromToString())
		} else {
			return nil
		}
	} else {
		return fmt.Errorf("router: %s not found", routerId)
	}
}

func (m *Mqtt) Start() error {
	if m.client == nil {
		ctx, cancel := context.WithTimeout(context.TODO(), 16*time.Second)
		defer cancel()
		if client, err := mqtt.NewClient(ctx, m.Config); err != nil {
			return err
		} else {
			m.client = client
			for _, router := range m.RouterStorage {

				if form := router.GetFrom(); form != nil {
					m.client.RegisterHandler(mqtt.Handler{
						Topic:  form.ToString(),
						Qos:    m.Config.QOS,
						Handle: m.handler(router),
					})
				}
			}
			return nil
		}
	}
	return nil
}

// 存储路由
func (m *Mqtt) saveRouter(routers ...endpoint.Router) {
	m.Lock()
	defer m.Unlock()
	if m.RouterStorage == nil {
		m.RouterStorage = make(map[string]endpoint.Router)
	}
	for _, item := range routers {
		m.RouterStorage[item.GetId()] = item
	}
}

// 从存储器中删除路由
func (m *Mqtt) deleteRouter(id string) endpoint.Router {
	m.Lock()
	defer m.Unlock()
	if m.RouterStorage != nil {
		if router, ok := m.RouterStorage[id]; ok {
			delete(m.RouterStorage, id)
			return router
		}
	}
	return nil
}

func (m *Mqtt) handler(router endpoint.Router) func(c paho.Client, data paho.Message) {
	return func(c paho.Client, data paho.Message) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				m.Printf("mqtt endpoint handler err :\n%v", runtime.Stack())
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

		m.DoProcess(context.Background(), router, exchange)
	}
}

func (m *Mqtt) Printf(format string, v ...interface{}) {
	if m.RuleConfig.Logger != nil {
		m.RuleConfig.Logger.Printf(format, v...)
	}
}
