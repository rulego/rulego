/*
 * Copyright 2025 The RuleGo Authors.
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
	"errors"
	"fmt"
	"os"

	paho "github.com/eclipse/paho.mqtt.golang"
	string2 "github.com/rulego/rulego/utils/str"

	"sync"
	"sync/atomic"
	"time"
)

// Handler subscribes to data processors
type Handler struct {
	//Subscribe to the topic
	Topic string
	//Subscribe to QoS
	Qos byte
	//Receive subscription data for processing
	Handle func(c paho.Client, data paho.Message)
}

// Config client configuration
type Config struct {
	Server               string        `json:"server" label:"Server" desc:"MQTT broker address, format host:port, e.g. 127.0.0.1:1883" required:"true" ref:"primary"`
	Username             string        `json:"username" label:"Username" desc:"MQTT authentication username" ref:"shared"`
	Password             string        `json:"password" label:"Password" desc:"MQTT authentication password" ref:"shared"`
	MaxReconnectInterval time.Duration `json:"maxReconnectInterval" label:"Max Reconnect Interval" desc:"Max reconnection interval, e.g. 10s, 1m"`
	QOS                  uint8         `json:"qos" label:"QoS" desc:"QoS level: 0(at most once), 1(at least once), 2(exactly once)"`
	CleanSession         bool          `json:"cleanSession" label:"Clean Session" desc:"Whether to clear previous session state"`
	ClientID             string        `json:"clientId" label:"Client ID" desc:"MQTT client unique identifier, default is random"`
	CAFile               string        `json:"caFile" label:"CA File" desc:"CA certificate file path for TLS" ref:"shared"`
	CertFile             string        `json:"certFile" label:"Cert File" desc:"TLS client certificate file path" ref:"shared"`
	CertKeyFile          string        `json:"certKeyFile" label:"Cert Key File" desc:"TLS client private key file path" ref:"shared"`
}

// Client: mqtt client
type Client struct {
	sync.RWMutex
	wg     sync.WaitGroup
	client paho.Client
	//Subscribe to themes and processor mappings
	msgHandlerMap map[string]Handler
	// Connection status indicator (0=Not connected, 1=Connected)
	isConnected int32
}

// NewClient creates an MQTT client instance
// Supports automatic reconnection and exponential retry strategies
func NewClient(ctx context.Context, conf Config) (*Client, error) {
	var err error

	b := Client{
		msgHandlerMap: make(map[string]Handler),
		isConnected:   0, // Initialize to unconnected state
	}

	opts := paho.NewClientOptions()
	opts.AddBroker(conf.Server)
	opts.SetUsername(conf.Username)
	opts.SetPassword(conf.Password)
	opts.SetCleanSession(conf.CleanSession)
	if conf.ClientID == "" {
		//Random clientId
		opts.SetClientID("rulego/" + string2.RandomStr(8))
	} else {
		opts.SetClientID(conf.ClientID)
	}

	// Set callback functions
	opts.SetOnConnectHandler(b.onConnected)
	opts.SetConnectionLostHandler(b.onConnectionLost)
	opts.SetReconnectingHandler(b.onReconnecting)

	// Configure automatic reconnection
	opts.SetAutoReconnect(true)
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

	// Initial connection retries logic, using exponential retreat strategies
	maxRetries := 5
	retryInterval := time.Second * 2

	for i := 0; i < maxRetries; i++ {
		if token := b.client.Connect(); token.Wait() && token.Error() != nil {
			select {
			case <-ctx.Done():
				// context is canceled or timed out, returning an error
				return nil, ctx.Err()
			case <-time.After(retryInterval):
				// Exponential Retreat: Increase the interval between retries by 50%
				retryInterval = time.Duration(float64(retryInterval) * 1.5)
				if retryInterval > conf.MaxReconnectInterval {
					retryInterval = conf.MaxReconnectInterval
				}
			}
		} else {
			// Connection successful, set the connection status
			atomic.StoreInt32(&b.isConnected, 1)
			return &b, nil
		}
	}

	// Reaches the maximum number of retries and returns the last connection error
	if token := b.client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect after %d retries: %v", maxRetries, token.Error())
	}

	atomic.StoreInt32(&b.isConnected, 1)
	return &b, nil
}

// RegisterHandler Registers and subscribes to the data processor
func (b *Client) RegisterHandler(handler Handler) {
	b.Lock()
	defer b.Unlock()
	b.msgHandlerMap[handler.Topic] = handler
	b.subscribeHandler(handler)
}

// UnregisterHandler deletes the subscribed data processor
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

// GetHandlerByUpTopic Gets the data processor through topics
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

// IsConnected checks whether the MQTT client is connected
func (b *Client) IsConnected() bool {
	return atomic.LoadInt32(&b.isConnected) == 1
}

// Publish the data
func (b *Client) Publish(topic string, qos byte, data []byte) error {
	// Check the connection status
	if !b.IsConnected() {
		return errors.New("MQTT client is not connected")
	}

	token := b.client.Publish(topic, qos, false, data)
	// Use a 5-second timeout to wait for the release to complete
	if !token.WaitTimeout(5 * time.Second) {
		return errors.New("publish timeout after 5 seconds")
	}

	if token.Error() != nil {
		return token.Error()
	}

	return nil
}

// onConnected MQTT connection successful callback
func (b *Client) onConnected(c paho.Client) {
	atomic.StoreInt32(&b.isConnected, 1)
	b.subscribe()
}

func (b *Client) subscribe() {
	b.RLock()
	// Creates a copy of the processor to avoid holding locks during iterations
	handlers := make([]Handler, 0, len(b.msgHandlerMap))
	for _, handler := range b.msgHandlerMap {
		handlers = append(handlers, handler)
	}
	b.RUnlock()

	// Subscribe without holding a lock
	for _, handler := range handlers {
		b.subscribeHandler(handler)
	}
}

func (b *Client) subscribeHandler(handler Handler) {
	topic := handler.Topic
	for {
		if token := b.client.Subscribe(topic, handler.Qos, handler.Handle).(*paho.SubscribeToken); token.Wait() && (token.Error() != nil || is128Err(token, topic)) { //128 ACK error
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
}

// Determine whether it is an ACL 128 error
func is128Err(token *paho.SubscribeToken, topic string) bool {
	result, ok := token.Result()[topic]
	return ok && result == 128
}

// onReconnecting MQTT reconnects during callback
// It is called when the client attempts to reconnect
func (b *Client) onReconnecting(c paho.Client, opts *paho.ClientOptions) {
}

// onConnectionLost MQTT connection lost callback
// It is called when the connection to the MQTT proxy is accidentally lost
func (b *Client) onConnectionLost(c paho.Client, reason error) {
	atomic.StoreInt32(&b.isConnected, 0)
}

func newTLSConfig(CAFile, certFile, certKeyFile string) (*tls.Config, error) {
	if CAFile == "" && certFile == "" && certKeyFile == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	// Import trusted certificates from CAFile.pem.
	if CAFile != "" {
		caCert, err := os.ReadFile(CAFile)
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

// NormalizeConfigKeys is compatible with case differences in older configuration keys
// Map old versions of keys (such as qOS, clientID, cAFile) to new versions of keys (qos, clientId, caFile)
func NormalizeConfigKeys(configuration map[string]interface{}) {
	keyMappings := [][2]string{
		{"qOS", "qos"},
		{"clientID", "clientId"},
		{"cAFile", "caFile"},
	}
	for _, mapping := range keyMappings {
		oldKey, newKey := mapping[0], mapping[1]
		if _, ok := configuration[newKey]; !ok {
			if v, ok := configuration[oldKey]; ok {
				configuration[newKey] = v
			}
		}
	}
}
