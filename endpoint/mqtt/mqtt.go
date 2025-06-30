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
// It enables creating MQTT clients that can subscribe to topics and process incoming MQTT messages,
// routing them to appropriate rule chains or components for business logic processing.
//
// Package mqtt 为 RuleGo 框架提供 MQTT 端点实现。
// 它支持创建 MQTT 客户端，可以订阅主题并处理传入的 MQTT 消息，
// 将它们路由到适当的规则链或组件进行业务逻辑处理。
//
// Key Features / 主要特性：
//
// • MQTT Client Management: Complete MQTT client lifecycle management  MQTT 客户端管理：完整的 MQTT 客户端生命周期管理
// • Topic Subscription: Dynamic topic subscription and message routing  主题订阅：动态主题订阅和消息路由
// • QoS Support: All MQTT Quality of Service levels (0, 1, 2)  QoS 支持：所有 MQTT 服务质量级别
// • Message Publishing: Response message publishing capabilities  消息发布：响应消息发布功能
// • Connection Management: Automatic reconnection and connection pooling  连接管理：自动重连和连接池
// • Topic Filtering: Pattern-based topic matching and routing  主题过滤：基于模式的主题匹配和路由
//
// Architecture / 架构：
//
// The MQTT endpoint follows a subscription-based processing model:
// MQTT 端点遵循基于订阅的处理模型：
//
// 1. MQTT Message → RequestMessage conversion  MQTT 消息 → RequestMessage 转换
// 2. Topic routing to appropriate rule chains  主题路由到适当的规则链
// 3. RequestMessage → Rule Chain/Component processing  RequestMessage → 规则链/组件处理
// 4. Processing Result → ResponseMessage  处理结果 → ResponseMessage
// 5. ResponseMessage → MQTT Publish (optional)  ResponseMessage → MQTT 发布（可选）
//
// Initialization Methods / 初始化方法：
//
// The MQTT endpoint supports two initialization approaches:
// MQTT 端点支持两种初始化方法：
//
// 1. Registry-based Initialization / 基于注册表的初始化：
//
//	import "github.com/rulego/rulego/endpoint"
//
//	config := types.Configuration{
//	    "server": "127.0.0.1:1883",
//	    "username": "user",
//	    "password": "password",
//	    "qos": 1,
//	}
//
//	// Create endpoint through registry
//	// 通过注册表创建端点
//	endpoint, err := endpoint.Registry.New(mqtt.Type, ruleConfig, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add router and start
//	// 添加路由器并启动
//	router := endpoint.NewRouter().
//	    From("sensors/temperature/+").
//	    To("chain:temperatureProcessing")
//
//	endpoint.AddRouter(router)
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization / 动态 DSL 初始化：
//
//	dslConfig := `{
//	  "id": "mqtt-endpoint",
//	  "type": "endpoint/mqtt",
//	  "name": "MQTT Subscriber",
//	  "configuration": {
//	    "server": "127.0.0.1:1883",
//	    "username": "user",
//	    "password": "password",
//	    "qos": 1
//	  },
//	  "routers": [
//	    {
//	      "id": "r1",
//	      "from": {
//	        "path": "sensors/temperature/+"
//	      },
//	      "to": {
//	        "path": "chain:temperatureProcessing"
//	      }
//	    }
//	  ]
//	}`
//
//	// Create endpoint from DSL
//	// 从 DSL 创建端点
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// Direct Instantiation (for advanced scenarios) / 直接实例化（高级场景）：
//
//	config := mqtt.Config{
//	    Server: "127.0.0.1:1883",
//	    Username: "user",
//	    Password: "password",
//	    QOS: 1,
//	}
//
//	endpoint := &mqtt.Mqtt{}
//	err := endpoint.Init(ruleConfig, config)
//
//	router := endpoint.NewRouter().
//	    From("sensors/temperature/+").
//	    To("chain:temperatureProcessing")
//
//	endpoint.AddRouter(router)
//	endpoint.Start()
//
// Topic Pattern Matching / 主题模式匹配：
//
// The endpoint supports MQTT standard topic patterns:
// 端点支持 MQTT 标准主题模式：
//
// • Exact match: "sensors/temperature"  精确匹配
// • Single-level wildcard: "sensors/+/status"  单级通配符
// • Multi-level wildcard: "sensors/#"  多级通配符
//
// Response Publishing / 响应发布：
//
// Response messages can be published by setting metadata:
// 可以通过设置元数据来发布响应消息：
//
// • responseTopic: Target topic for response  响应的目标主题
// • responseQos: QoS level for response  响应的 QoS 级别
package mqtt

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"time"

	"github.com/rulego/rulego/utils/mqtt"

	"github.com/rulego/rulego/utils/cast"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/runtime"
)

// Type defines the component type identifier for the MQTT endpoint.
// This identifier is used for component registration and DSL configuration.
// Type 定义 MQTT 端点的组件类型标识符。
// 此标识符用于组件注册和 DSL 配置。
const Type = types.EndpointTypePrefix + "mqtt"

// Metadata keys used for MQTT-specific information in RuleMsg metadata.
// These constants provide standardized access to MQTT message properties.
// 用于 RuleMsg 元数据中 MQTT 特定信息的元数据键。
// 这些常量提供对 MQTT 消息属性的标准化访问。
const (
	// KeyRequestTopic stores the original MQTT topic in message metadata
	// KeyRequestTopic 在消息元数据中存储原始 MQTT 主题
	KeyRequestTopic = "topic"

	// KeyResponseTopic specifies the topic for publishing response messages
	// KeyResponseTopic 指定发布响应消息的主题
	KeyResponseTopic = "responseTopic"

	// KeyResponseQos specifies the QoS level for publishing response messages
	// KeyResponseQos 指定发布响应消息的 QoS 级别
	KeyResponseQos = "responseQos"
)

// Endpoint is an alias for Mqtt to provide consistent naming with other endpoints.
// This allows users to reference the component using the standard Endpoint name.
// Endpoint 是 Mqtt 的别名，提供与其他端点一致的命名。
// 这允许用户使用标准的 Endpoint 名称引用组件。
type Endpoint = Mqtt

// RequestMessage represents an incoming MQTT message in the RuleGo processing pipeline.
// It encapsulates all the necessary information from an MQTT message and provides methods
// to access message data, topic information, and convert the message into a RuleMsg.
//
// RequestMessage 表示 RuleGo 处理管道中的传入 MQTT 消息。
// 它封装了 MQTT 消息的所有必要信息，并提供方法来访问消息数据、主题信息，
// 并将消息转换为 RuleMsg。
//
// Key Features / 主要特性：
// • MQTT Message Wrapping: Provides unified access to MQTT message properties  MQTT 消息包装：提供对 MQTT 消息属性的统一访问
// • Topic Information: Access to MQTT topic and routing information  主题信息：访问 MQTT 主题和路由信息
// • Payload Access: Efficient access to message payload data  载荷访问：高效访问消息载荷数据
// • Metadata Integration: Seamless integration with RuleGo's metadata system  元数据集成：与 RuleGo 元数据系统无缝集成
// • JSON Data Type: Automatic JSON data type assignment for rule processing  JSON 数据类型：规则处理的自动 JSON 数据类型分配
//
// Message Flow / 消息流：
// 1. MQTT message received from broker  从代理接收 MQTT 消息
// 2. RequestMessage created with message context  使用消息上下文创建 RequestMessage
// 3. Topic information extracted and stored  提取并存储主题信息
// 4. Converted to RuleMsg for rule chain processing  转换为 RuleMsg 进行规则链处理
type RequestMessage struct {
	//HTTP 风格的头部映射，存储 MQTT 特定信息  HTTP-style headers map storing MQTT-specific information  头部映射
	headers textproto.MIMEHeader
	//原始 MQTT 消息对象  Original MQTT message object  原始 MQTT 消息
	request paho.Message
	//消息载荷数据，延迟读取以优化性能  Message payload data, lazily loaded for performance  消息载荷数据
	body []byte
	//转换后的规则消息，缓存以避免重复转换  Converted rule message, cached to avoid re-conversion  转换后的规则消息
	msg *types.RuleMsg
	//处理过程中的错误信息  Error information during processing  处理错误信息
	err error
}

// Body returns the MQTT message payload as a byte slice.
// The payload is extracted from the MQTT message on first access and cached for performance.
// This method provides efficient access to the message content.
//
// Body 返回 MQTT 消息载荷作为字节切片。
// 首次访问时从 MQTT 消息中提取载荷并缓存以提高性能。
// 此方法提供对消息内容的高效访问。
//
// Returns / 返回：
// • []byte: MQTT message payload content  MQTT 消息载荷内容
func (r *RequestMessage) Body() []byte {
	if r.body == nil && r.request != nil {
		r.body = r.request.Payload()
	}
	return r.body
}

// Headers returns HTTP-style headers containing MQTT-specific information.
// The headers include the original MQTT topic and other relevant metadata.
// This provides a standardized way to access MQTT message properties.
//
// Headers 返回包含 MQTT 特定信息的 HTTP 风格头部。
// 头部包括原始 MQTT 主题和其他相关元数据。
// 这提供了访问 MQTT 消息属性的标准化方式。
//
// Returns / 返回：
// • textproto.MIMEHeader: Headers map with MQTT information  包含 MQTT 信息的头部映射
//
// Header Contents / 头部内容：
// • topic: Original MQTT topic name  原始 MQTT 主题名称
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	if r.request != nil {
		r.headers.Set(KeyRequestTopic, r.request.Topic())
	}
	return r.headers
}

// From returns the MQTT topic name for this message.
// This is used for routing and logging purposes in the RuleGo framework.
//
// From 返回此消息的 MQTT 主题名称。
// 这在 RuleGo 框架中用于路由和日志记录目的。
//
// Returns / 返回：
// • string: MQTT topic name, empty string if no request  MQTT 主题名称，如果没有请求则为空字符串
func (r *RequestMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.Topic()
}

// GetParam returns an empty string as MQTT messages do not support URL-style parameters.
// This method exists to satisfy the Message interface but is not applicable for MQTT.
//
// GetParam 返回空字符串，因为 MQTT 消息不支持 URL 风格的参数。
// 此方法存在是为了满足 Message 接口，但不适用于 MQTT。
//
// Parameters / 参数：
// • key: Parameter name (ignored in MQTT context)  参数名称（在 MQTT 上下文中被忽略）
//
// Returns / 返回：
// • string: Always returns empty string  总是返回空字符串
func (r *RequestMessage) GetParam(key string) string {
	return ""
}

// SetMsg sets the RuleMsg for this MQTT request message.
// This is typically used during message processing to cache the converted message.
//
// SetMsg 为此 MQTT 请求消息设置 RuleMsg。
// 这通常在消息处理期间用于缓存转换后的消息。
//
// Parameters / 参数：
// • msg: The rule message to associate with this request  要与此请求关联的规则消息
func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

// GetMsg converts the MQTT message to a RuleMsg for rule chain processing.
// The conversion includes automatic JSON data type assignment and metadata population.
// The MQTT topic is automatically added to the message metadata for routing purposes.
//
// GetMsg 将 MQTT 消息转换为 RuleMsg 以进行规则链处理。
// 转换包括自动 JSON 数据类型分配和元数据填充。
// MQTT 主题自动添加到消息元数据中用于路由目的。
//
// Returns / 返回：
// • *types.RuleMsg: Converted rule message ready for processing  转换后的规则消息，可供处理
//
// Conversion Details / 转换详情：
// • Data Type: Always set to JSON for flexible processing  数据类型：总是设置为 JSON 以便灵活处理
// • Source: Set to MQTT topic name  来源：设置为 MQTT 主题名称
// • Payload: MQTT message payload as string  载荷：MQTT 消息载荷作为字符串
// • Metadata: Includes original topic information  元数据：包含原始主题信息
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))
		ruleMsg.Metadata.PutValue(KeyRequestTopic, r.From())
		r.msg = &ruleMsg
	}
	return r.msg
}

// SetStatusCode is a no-op for MQTT request messages as status codes are not applicable.
// This method exists to satisfy the Message interface.
//
// SetStatusCode 对于 MQTT 请求消息是无操作，因为状态码不适用。
// 此方法存在是为了满足 Message 接口。
//
// Parameters / 参数：
// • statusCode: Status code (ignored in MQTT context)  状态码（在 MQTT 上下文中被忽略）
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

// SetBody sets the message payload content.
// This is typically used for testing or message transformation scenarios.
//
// SetBody 设置消息载荷内容。
// 这通常用于测试或消息转换场景。
//
// Parameters / 参数：
// • body: Message payload content to set  要设置的消息载荷内容
func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

// SetError sets an error associated with this MQTT request message.
// This is used to track errors during message processing.
//
// SetError 设置与此 MQTT 请求消息关联的错误。
// 用于跟踪消息处理期间的错误。
//
// Parameters / 参数：
// • err: Error to associate with this message  要与此消息关联的错误
func (r *RequestMessage) SetError(err error) {
	r.err = err
}

// GetError returns any error associated with this MQTT request message.
// This is useful for error handling and debugging.
//
// GetError 返回与此 MQTT 请求消息关联的任何错误。
// 这对于错误处理和调试很有用。
//
// Returns / 返回：
// • error: Associated error, nil if no error  关联的错误，如果没有错误则为 nil
func (r *RequestMessage) GetError() error {
	return r.err
}

// Request returns the underlying MQTT message object.
// This provides direct access to the original paho.Message for advanced scenarios.
//
// Request 返回底层的 MQTT 消息对象。
// 这为高级场景提供对原始 paho.Message 的直接访问。
//
// Returns / 返回：
// • paho.Message: Original MQTT message object  原始 MQTT 消息对象
func (r *RequestMessage) Request() paho.Message {
	return r.request
}

// ResponseMessage represents an outgoing MQTT message in the RuleGo processing pipeline.
// It handles the conversion of rule processing results back into MQTT publish operations,
// including topic selection, QoS configuration, and message content publishing.
//
// ResponseMessage 表示 RuleGo 处理管道中的传出 MQTT 消息。
// 它处理规则处理结果转换回 MQTT 发布操作，包括主题选择、QoS 配置和消息内容发布。
//
// Key Features / 主要特性：
// • Automatic Publishing: Response content is automatically published to MQTT broker  自动发布：响应内容自动发布到 MQTT 代理
// • Topic Configuration: Response topic can be configured via metadata  主题配置：可通过元数据配置响应主题
// • QoS Control: Quality of Service level can be specified for published messages  QoS 控制：可为发布的消息指定服务质量级别
// • Metadata Integration: Uses RuleGo metadata for publishing configuration  元数据集成：使用 RuleGo 元数据进行发布配置
// • Error Handling: Built-in error tracking for publishing operations  错误处理：发布操作的内置错误跟踪
//
// Publishing Behavior / 发布行为：
// When SetBody() is called, the message is automatically published to the MQTT broker
// using the topic and QoS specified in metadata or headers.
// 调用 SetBody() 时，消息会自动发布到 MQTT 代理，
// 使用元数据或头部中指定的主题和 QoS。
//
// Configuration via Metadata / 通过元数据配置：
// • responseTopic: Target topic for publishing  发布的目标主题
// • responseQos: QoS level for publishing (0, 1, or 2)  发布的 QoS 级别
type ResponseMessage struct {
	//HTTP 风格的头部映射，存储 MQTT 响应配置  HTTP-style headers map storing MQTT response configuration  头部映射
	headers textproto.MIMEHeader
	//原始请求消息，用于上下文信息  Original request message for context information  原始请求消息
	request paho.Message
	//MQTT 客户端，用于发布响应消息  MQTT client for publishing response messages  MQTT 客户端
	response paho.Client
	//响应消息体数据  Response message body data  响应消息体
	body []byte
	//处理结果的规则消息  Rule message with processing results  处理结果的规则消息
	msg *types.RuleMsg
	//响应处理过程中的错误  Error during response processing  响应处理错误
	err error
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

// Mqtt represents an MQTT endpoint implementation for the RuleGo framework.
// It provides a complete MQTT client solution with topic subscription, message processing,
// and integration with RuleGo's rule chains and components.
//
// Mqtt 表示 RuleGo 框架的 MQTT 端点实现。
// 它提供完整的 MQTT 客户端解决方案，具有主题订阅、消息处理以及与 RuleGo 规则链和组件的集成。
//
// Architecture / 架构：
//
// The MQTT endpoint follows a publish-subscribe messaging pattern:
// MQTT 端点遵循发布-订阅消息传递模式：
//
// 1. MQTT Client Layer: Handles low-level MQTT protocol operations  MQTT 客户端层：处理低级 MQTT 协议操作
// 2. Topic Subscription Layer: Manages topic subscriptions and message routing  主题订阅层：管理主题订阅和消息路由
// 3. Message Processing Layer: Converts MQTT messages to RuleMsg format  消息处理层：将 MQTT 消息转换为 RuleMsg 格式
// 4. Rule Engine Integration: Executes business logic on received messages  规则引擎集成：对接收的消息执行业务逻辑
//
// Key Features / 主要特性：
//
// • MQTT Client Management: Complete client lifecycle with connection management  MQTT 客户端管理：完整的客户端生命周期和连接管理
// • Topic Subscription: Dynamic topic subscription with wildcard support  主题订阅：支持通配符的动态主题订阅
// • Connection Sharing: Multiple endpoint instances can share the same client  连接共享：多个端点实例可以共享同一客户端
// • Automatic Reconnection: Built-in reconnection logic for reliability  自动重连：内置重连逻辑以确保可靠性
// • QoS Support: All MQTT Quality of Service levels (0, 1, 2)  QoS 支持：所有 MQTT 服务质量级别
// • Message Publishing: Response message publishing capabilities  消息发布：响应消息发布功能
// • Topic Pattern Matching: Support for MQTT topic wildcards (+ and #)  主题模式匹配：支持 MQTT 主题通配符
//
// Connection Management / 连接管理：
//
// The endpoint uses shared connections to optimize resource usage:
// 端点使用共享连接来优化资源使用：
//
// • Single connection per server address  每个服务器地址单一连接
// • Automatic connection establishment and maintenance  自动连接建立和维护
// • Graceful connection shutdown on endpoint destruction  端点销毁时优雅的连接关闭
//
// Topic Subscription / 主题订阅：
//
// Supports MQTT standard topic patterns:
// 支持 MQTT 标准主题模式：
//
// • Exact topics: "sensors/temperature"  精确主题
// • Single-level wildcards: "sensors/+/status"  单级通配符
// • Multi-level wildcards: "sensors/#"  多级通配符
//
// Thread Safety / 线程安全：
//
// The MQTT endpoint is designed for concurrent operations:
// MQTT 端点设计用于并发操作：
//
// • Route management operations are thread-safe  路由管理操作是线程安全的
// • Message handling supports concurrent processing  消息处理支持并发处理
// • Connection operations are protected for concurrent access  连接操作受保护以支持并发访问
//
// Performance Considerations / 性能考虑：
//
// • Shared client connections reduce resource overhead  共享客户端连接减少资源开销
// • Efficient topic matching algorithms  高效的主题匹配算法
// • Configurable connection parameters for optimization  可配置的连接参数用于优化
// • Non-blocking message processing  非阻塞消息处理
type Mqtt struct {
	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint 提供通用端点功能
	impl.BaseEndpoint

	// SharedNode enables client sharing between multiple endpoint instances
	// SharedNode 启用多个端点实例之间的客户端共享
	base.SharedNode[*mqtt.Client]

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig 提供对规则引擎配置的访问
	RuleConfig types.Config

	// Config contains the MQTT client configuration settings
	// Config 包含 MQTT 客户端配置设置
	Config mqtt.Config

	// client is the underlying MQTT client instance
	// client 是底层的 MQTT 客户端实例
	client *mqtt.Client

	// started indicates whether the MQTT client has been started and is subscribing
	// started 指示 MQTT 客户端是否已启动并正在订阅
	started bool
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
	var v, ok = configuration["maxReconnectInterval"]
	if !ok {
		v, ok = configuration["MaxReconnectInterval"]
	}
	if v != nil {
		// 兼容默认秒方式
		if num := cast.ToInt64(v); num != 0 {
			configuration["maxReconnectInterval"] = fmt.Sprintf("%ds", num)
		}
	}
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
