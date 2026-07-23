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
// Package mqtt provides an MQTT endpoint implementation for the RuleGo framework.
// It supports the creation of MQTT clients, can subscribe to topics, and handle incoming MQTT messages,
// Routing them to appropriate rule chains or components for business logic processing.
//
// # Key Features
//
// • MQTT Client Management: Complete MQTT client lifecycle management
// • Topic Subscription: Dynamic topic subscription and message routing
// • QoS Support: All MQTT Quality of Service levels (0, 1, 2) QoS Support: All MQTT Quality of Service levels
// • Message Publishing: Response message publishing capabilities
// • Connection Management: Automatic reconnection and connection pooling
// • Topic Filtering: Pattern-based topic matching and routing
//
// # Architecture
//
// The MQTT endpoint follows a subscription-based processing model:
// MQTT endpoints follow a subscription-based processing model:
//
// 1. MQTT Message → RequestMessage conversion MQTT message → RequestMessage conversion
// 2. Topic routing to appropriate rule chains
// 3. RequestMessage → Rule Chain/Component Processing RequestMessage → Rule chain/component processing
// 4. Processing Result → ResponseMessage
// 5. ResponseMessage → MQTT Publish (optional) ResponseMessage → MQTT Publish (optional)
//
// # Initialization Methods
//
// The MQTT endpoint supports two initialization approaches:
// MQTT endpoints support two initialization methods:
//
// 1. Registry-based Initialization
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
//	Create endpoints through the registry
//	endpoint, err := endpoint.Registry.New(mqtt.Type, ruleConfig, config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Add router and start
//	Add the router and start it
//	router := endpoint.NewRouter().
//	    From("sensors/temperature/+").
//	    To("chain:temperatureProcessing")
//
//	endpoint.AddRouter(router)
//	endpoint.Start()
//
// 2. Dynamic DSL Initialization
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
//	Create endpoints from DSL
//	endpoint, err := endpoint.NewFromDsl([]byte(dslConfig))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	endpoint.Start()
//
// Direct Instantiation (for advanced scenarios)
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
// # Topic Pattern Matching
//
// The endpoint supports MQTT standard topic patterns:
// Endpoints support MQTT standard theme mode:
//
// • Exact match: "sensors/temperature"
// • Single-level wildcard: "sensors/+/status"
// • Multi-level wildcard: "sensors/#"
//
// # Response Publishing
//
// Response messages can be published by setting metadata:
// You can publish response messages by setting metadata:
//
// • responseTopic: Target topic for response
// • responseQos: QoS level for response
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
// Type defines the component type identifier for the MQTT endpoint.
// This identifier is used for component registration and DSL configuration.
const Type = types.EndpointTypePrefix + "mqtt"

// Metadata keys used for MQTT-specific information in RuleMsg metadata.
// These constants provide standardized access to MQTT message properties.
// Metadata keys for MQTT-specific information in RuleMsg metadata.
// These constants provide standardized access to MQTT message attributes.
const (
	// KeyRequestTopic stores the original MQTT topic in message metadata
	// KeyRequestTopic stores the original MQTT topic in the message metadata
	KeyRequestTopic = "topic"

	// KeyResponseTopic specifies the topic for publishing response messages
	// KeyResponseTopic specifies the subject for which the response message will be published
	KeyResponseTopic = "responseTopic"

	// KeyResponseQos specifies the QoS level for publishing response messages
	// KeyResponseQos specifies the QoS level for publishing response messages
	KeyResponseQos = "responseQos"
)

// Endpoint is an alias for Mqtt to provide consistent naming with other endpoints.
// This allows users to reference the component using the standard Endpoint name.
// Endpoint is an alias for MQTT, providing consistent naming with other endpoints.
// This allows users to reference components using standard Endpoint names.
type Endpoint = Mqtt

// RequestMessage represents an incoming MQTT message in the RuleGo processing pipeline.
// It encapsulates all the necessary information from an MQTT message and provides methods
// to access message data, topic information, and convert the message into a RuleMsg.
//
// RequestMessage means RuleGo handles incoming MQTT messages in the pipeline.
// It encapsulates all the necessary information for MQTT messages and provides methods to access message data and subject information,
// and convert the message into RuleMsg.
//
// Key Features
// • MQTT Message Wrapping: Provides unified access to MQTT message properties
// • Topic Information: Access to MQTT topic and routing information
// • Payload Access: Efficient access to message payload data
// • Metadata Integration: Seamless integration with RuleGo's metadata system
// • JSON Data Type: Automatic JSON data type assignment for rule processing
//
// Message Flow
// 1. MQTT message received from broker
// 2. RequestMessage created with message context
// 3. Topic information extracted and stored
// 4. Converted to RuleMsg for rule chain processing
type RequestMessage struct {
	//HTTP-style headers map storing MQTT-specific information
	headers textproto.MIMEHeader
	//Original MQTT message object
	request paho.Message
	//Message payload data, lazily loaded for performance
	body []byte
	//Converted rule message, cached to avoid re-conversion
	msg *types.RuleMsg
	//Error information during processing
	err error
}

// Body returns the MQTT message payload as a byte slice.
// The payload is extracted from the MQTT message on first access and cached for performance.
// This method provides efficient access to the message content.
//
// Body returns the MQTT message payload as a byte slice.
// Extract payloads from MQTT messages on the first visit and cache them to improve performance.
// This method provides efficient access to the content of the message.
//
// Returns
// • []byte: MQTT message payload content
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
// Headers return an HTTP-style header containing MQTT-specific information.
// The header includes the original MQTT theme and other related metadata.
// This provides a standardized way to access MQTT message attributes.
//
// Returns
// • textproto.MIMEHeader: Headers map with MQTT information
//
// Header Contents
// • topic: Original MQTT topic name
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
// From Return the MQTT subject name for this message.
// This is used in the RuleGo framework for routing and logging purposes.
//
// Returns
// • string: MQTT topic name, empty string if no request MQTT topic name; if no request is made, it is an empty string
func (r *RequestMessage) From() string {
	if r.request == nil {
		return ""
	}
	return r.request.Topic()
}

// GetParam returns an empty string as MQTT messages do not support URL-style parameters.
// This method exists to satisfy the Message interface but is not applicable for MQTT.
//
// GetParam returns an empty string because MQTT messages do not support URL-style parameters.
// This method exists to satisfy the Message interface, but it is not suitable for MQTT.
//
// Parameters
// • key: Parameter name (ignored in MQTT context)
//
// Returns
// • string: Always returns empty string
func (r *RequestMessage) GetParam(key string) string {
	return ""
}

// SetMsg sets the RuleMsg for this MQTT request message.
// This is typically used during message processing to cache the converted message.
//
// SetMsg sets RuleMsg for this MQTT request message.
// This is usually used during message processing to cache the transformed messages.
//
// Parameters
// • msg: The rule message to associate with this request
func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

// GetMsg converts the MQTT message to a RuleMsg for rule chain processing.
// The conversion includes automatic JSON data type assignment and metadata population.
// The MQTT topic is automatically added to the message metadata for routing purposes.
//
// GetMsg converts MQTT messages into RuleMsg for rule chain processing.
// Conversion includes automatic JSON data type assignment and metadata filling.
// MQTT topics are automatically added to message metadata for routing purposes.
//
// Returns
// • *types.RuleMsg: Converted rule message ready for processing
//
// Conversion Details
// • Data Type: Always set to JSON for flexible processing
// • Source: Set to MQTT topic name
// • Payload: MQTT message payload as string
// • Metadata: Includes original topic information
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
// SetStatusCode is non-operative for MQTT request messages because status codes do not apply.
// This method exists to satisfy the Message interface.
//
// Parameters
// • statusCode: Status code (ignored in MQTT context)
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

// SetBody sets the message payload content.
// This is typically used for testing or message transformation scenarios.
//
// SetBody sets the message payload content.
// This is usually used for testing or message conversion scenarios.
//
// Parameters
// • body: Message payload content to set
func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

// SetError sets an error associated with this MQTT request message.
// This is used to track errors during message processing.
//
// SetError sets the error associated with this MQTT request message.
// Used to track errors during message processing.
//
// Parameters
// • err: Error to associate with this message
func (r *RequestMessage) SetError(err error) {
	r.err = err
}

// GetError returns any error associated with this MQTT request message.
// This is useful for error handling and debugging.
//
// GetError returns any error associated with this MQTT request message.
// This is useful for error handling and debugging.
//
// Returns
// • error: Associated error, nil if no error
func (r *RequestMessage) GetError() error {
	return r.err
}

// Request returns the underlying MQTT message object.
// This provides direct access to the original paho.Message for advanced scenarios.
//
// Request returns the underlying MQTT message object.
// This provides direct access to the original paho.Message for advanced scenarios.
//
// Returns
// • paho.Message: Original MQTT message object
func (r *RequestMessage) Request() paho.Message {
	return r.request
}

// ResponseMessage represents an outgoing MQTT message in the RuleGo processing pipeline.
// It handles the conversion of rule processing results back into MQTT publish operations,
// including topic selection, QoS configuration, and message content publishing.
//
// ResponseMessage means RuleGo handles outgoing MQTT messages in the pipeline.
// It handles the conversion of rule processing results back into MQTT publishing operations, including topic selection, QoS configuration, and message content publishing.
//
// Key Features
// • Automatic Publishing: Response content is automatically published to MQTT broker
// • Topic Configuration: Response topic can be configured via metadata
// • QoS Control: Quality of Service level can be specified for published messages QoS Control: Specifies a Service Level for published messages
// • Metadata Integration: Uses RuleGo metadata for publishing configuration
// • Error Handling: Built-in error tracking for publishing operations
//
// Publishing Behavior
// When SetBody() is called, the message is automatically published to the MQTT broker
// using the topic and QoS specified in metadata or headers.
// When SetBody() is called, the message is automatically published to the MQTT proxy,
// Use metadata or topics specified in the header and QoS.
//
// Configuration via Metadata
// • responseTopic: Target topic for publishing
// • responseQos: QoS level for publishing (0, 1, or 2)
type ResponseMessage struct {
	//HTTP-style headers mapping storing MQTT response configuration
	headers textproto.MIMEHeader
	//Original request message for context information
	request paho.Message
	//MQTT client for publishing response messages MQTT client
	response paho.Client
	//Response message body data
	body []byte
	//Rule message with processing results
	msg *types.RuleMsg
	//Error during response processing
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

// GetParam does not provide acquisition parameters
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

// From msg.Metadata or response header access
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
// Mqtt represents the MQTT endpoint implementation of the RuleGo framework.
// It offers a complete MQTT client solution with topic subscriptions, message processing, and integration with RuleGo rule chains and components.
//
// # Architecture
//
// The MQTT endpoint follows a publish-subscribe messaging pattern:
// MQTT endpoints follow a publish-subscribe messaging model:
//
// 1. MQTT Client Layer: Handles low-level MQTT protocol operations
// 2. Topic Subscription Layer: Manages topic subscriptions and message routing
// 3. Message Processing Layer: Converts MQTT messages to RuleMsg format
// 4. Rule Engine Integration: Executes business logic on received messages
//
// # Key Features
//
// • MQTT Client Management: Complete client lifecycle and connection management
// • Topic Subscription: Dynamic topic subscription with wildcard support
// • Connection Sharing: Multiple endpoint instances can share the same client
// • Automatic Reconnection: Built-in reconnection logic for reliability
// • QoS Support: All MQTT Quality of Service levels (0, 1, 2) QoS Support: All MQTT Quality of Service levels
// • Message Publishing: Response message publishing capabilities
// • Topic Pattern Matching: Support for MQTT topic wildcards (+ and #)
//
// # Connection Management
//
// The endpoint uses shared connections to optimize resource usage:
// Endpoints use shared connections to optimize resource usage:
//
// • Single connection per server address
// • Automatic connection establishment and maintenance
// • Graceful connection shutdown on endpoint destruction
//
// # Topic Subscription
//
// Supports MQTT standard topic patterns:
// Supports MQTT standard theme mode:
//
// • Exact topics: "sensors/temperature"
// • Single-level wildcards: "sensors/+/status"
// • Multi-level wildcards: "sensors/#"
//
// # Thread Safety
//
// The MQTT endpoint is designed for concurrent operations:
// MQTT endpoints are designed for concurrent operations:
//
// • Route management operations are thread-safe
// • Message handling supports concurrent processing
// • Connection operations are protected for concurrent access
//
// # Performance Considerations
//
// • Shared client connections reduce resource overhead
// • Efficient topic matching algorithms
// • Configurable connection parameters for optimization
// • Non-blocking message processing
type Mqtt struct {
	// BaseEndpoint provides common endpoint functionality
	// BaseEndpoint provides universal endpoint functionality
	impl.BaseEndpoint

	// SharedNode enables client sharing between multiple endpoint instances
	// SharedNode enables client sharing among multiple endpoint instances
	base.SharedNode[*mqtt.Client]

	// GracefulShutdown provides graceful shutdown capabilities
	// GracefulShutdown offers an elegant shutdown function
	base.GracefulShutdown

	// RuleConfig provides access to the rule engine configuration
	// RuleConfig provides access to the rule engine configuration
	RuleConfig types.Config

	// Config contains the MQTT client configuration settings
	// Config contains MQTT client configuration settings
	Config mqtt.Config

	// started indicates whether the MQTT client has been started and is subscribing
	// 'started' indicates whether the MQTT client is starting and subscribed
	started bool
}

// Type returns the component type
func (x *Mqtt) Type() string {
	return Type
}

// Category returns the component category
func (x *Mqtt) Category() string {
	return "endpoint"
}

// Def returns the component definition including description and router form metadata.
func (x *Mqtt) Def() types.ComponentForm {
	return types.ComponentForm{
		Desc: "MQTT client endpoint: connects to an MQTT broker, subscribes to topics (supports + / # wildcards), and processes each incoming message",
		RouterForm: &types.RouterForm{
			From: &types.RouterFormField{
				Path: types.ComponentFormField{
					Name:     "path",
					Type:     "string",
					Label:    "Topic",
					Desc:     "MQTT topic filter to subscribe; supports wildcards + (single level, e.g. sensors/+/temp) and # (multi level, e.g. sensors/#), e.g. devices/msg",
					Required: true,
				},
			},
		},
	}
}

func (x *Mqtt) New() types.Node {
	return &Mqtt{Config: mqtt.Config{
		Server: "127.0.0.1:1883",
	}}
}

// Init initializes the component
func (x *Mqtt) Init(ruleConfig types.Config, configuration types.Configuration) error {
	// Compatible with old keys
	mqtt.NormalizeConfigKeys(configuration)
	var v, ok = configuration["maxReconnectInterval"]
	if !ok {
		v, ok = configuration["MaxReconnectInterval"]
	}
	if v != nil {
		// Compatible with default second mode
		if num := cast.ToInt64(v); num != 0 {
			configuration["maxReconnectInterval"] = fmt.Sprintf("%ds", num)
		}
	}
	err := maps.Map2Struct(configuration, &x.Config)
	x.RuleConfig = ruleConfig

	// Initialize the elegant downtime function - use reasonable default timeout (10 seconds)
	x.GracefulShutdown.InitGracefulShutdown(x.RuleConfig.Logger, 10*time.Second)

	_ = x.SharedNode.InitWithClose(x.RuleConfig, x.Type(), x.Config.Server, true, func() (*mqtt.Client, error) {
		return x.initClient()
	}, func(client *mqtt.Client) error {
		if client != nil {
			return client.Close()
		}
		return nil
	})
	return err
}

// Destroy releases resources
func (x *Mqtt) Destroy() {
	x.GracefulShutdown.GracefulStop(func() {
		_ = x.Close()
	})
}

// GracefulStop provides graceful shutdown for the MQTT endpoint
// GracefulStop provides elegant downtime for MQTT endpoints
func (x *Mqtt) GracefulStop() {
	x.GracefulShutdown.GracefulStop(func() {
		_ = x.Close()
	})
}

func (x *Mqtt) Close() error {
	// SharedNode manages client shutdowns through the cleanup function in InitWithClose
	// SharedNode manages client closure through the cleanup function in InitWithClose
	return x.SharedNode.Close()
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
	//The service has already started
	if x.started {
		if form := router.GetFrom(); form != nil {
			client, err := x.SharedNode.GetSafely()
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
		client, _ := x.SharedNode.GetSafely()
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
	client, err := x.SharedNode.GetSafely()
	if err != nil {
		return err
	}
	x.RLock()
	routers := make(map[string]endpoint.Router, len(x.RouterStorage))
	for k, v := range x.RouterStorage {
		routers[k] = v
	}
	x.RUnlock()
	for _, router := range routers {
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

// Store the route
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

// Delete the route from memory
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
			//Capture anomalies
			if e := recover(); e != nil {
				x.Printf("mqtt endpoint handler err :\n%v", runtime.Stack())
			}
		}()

		// Check if the machine is being shut down
		if err := x.GracefulShutdown.CheckShutdownSignal(); err != nil {
			x.Printf("MQTT message ignored due to shutdown: %v", err)
			return
		}

		// Increase the count of active operations
		x.GracefulShutdown.IncrementActiveOperations()
		defer x.GracefulShutdown.DecrementActiveOperations()

		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: data,
			},
			Out: &ResponseMessage{
				request:  data,
				response: c,
			}}

		// Handle messages using a downtime context
		x.DoProcess(x.GracefulShutdown.GetShutdownContext(), router, exchange)
	}
}

func (x *Mqtt) Printf(format string, v ...interface{}) {
	if x.RuleConfig.Logger != nil {
		x.RuleConfig.Logger.Printf(format, v...)
	}
}

// initClient initializes the client
func (x *Mqtt) initClient() (*mqtt.Client, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 4*time.Second)
	defer cancel()
	return mqtt.NewClient(ctx, x.Config)
}
