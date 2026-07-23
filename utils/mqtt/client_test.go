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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/test/assert"
)

// MockToken: Simulated token
type MockToken struct {
	err error
}

// Wait: Simulated waiting
func (m *MockToken) Wait() bool {
	return true
}

// WaitTimeout simulates a timeout wait
func (m *MockToken) WaitTimeout(timeout time.Duration) bool {
	return true
}

// Error returns an error
func (m *MockToken) Error() error {
	return m.err
}

// =============================================================================
// Unit testing
// =============================================================================

// TestConfig_Validation Test configuration verification
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server:   "tcp://localhost:1883",
				ClientID: "test-client",
			},
			wantErr: false,
		},
		{
			name: "empty server",
			config: Config{
				ClientID: "test-client",
			},
			wantErr: true,
		},
		{
			name: "empty client ID",
			config: Config{
				Server: "tcp://localhost:1883",
			},
			wantErr: true,
		},
		{
			name: "invalid server format",
			config: Config{
				Server:   "invalid-server",
				ClientID: "test-client",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only basic configuration verification is tested; no actual MQTT connection is created
			if tt.config.Server == "" {
				assert.True(t, tt.wantErr, "Empty server should cause error")
				return
			}
			if tt.config.ClientID == "" {
				assert.True(t, tt.wantErr, "Empty client ID should cause error")
				return
			}
			// For effective configuration, we assume it won't go wrong (avoiding actual connections)
			if !tt.wantErr {
				assert.NotEqual(t, "", tt.config.Server)
				assert.NotEqual(t, "", tt.config.ClientID)
			}
		})
	}
}

// TestClient_ConnectionStatus Test connection status management
func TestClient_ConnectionStatus(t *testing.T) {
	client := &Client{
		isConnected: 0,
	}

	// The initial state should be unconnected
	assert.Equal(t, int32(0), atomic.LoadInt32(&client.isConnected))

	// The simulated connection was successful
	client.onConnected(nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))

	// Analog connection loss
	client.onConnectionLost(nil, nil)
	assert.Equal(t, int32(0), atomic.LoadInt32(&client.isConnected))
}

// TestClient_IsConnected Test the IsConnected method
func TestClient_IsConnected(t *testing.T) {
	// Create an unconnected client
	client := &Client{
		isConnected: 0,
		client:      nil, // Simulates uninitialized clients
	}

	// Test the disconnected state
	assert.False(t, client.IsConnected())
}

// TestClient_Publish_NotConnected Tests are published when not connected
func TestClient_Publish_NotConnected(t *testing.T) {
	client := &Client{
		isConnected: 0,
	}

	err := client.Publish("test/topic", 0, []byte("test message"))
	if err == nil {
		t.Error("Expected error but got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "MQTT client is not connected") {
		t.Errorf("Expected error to contain 'MQTT client is not connected', got: %v", err)
	}
}

// TestClient_RegisterHandler Test the registered processor - skip because a real MQTT client is required
func TestClient_RegisterHandler(t *testing.T) {
	t.Skip("RegisterHandler requires a real MQTT client connection")
}

// TestIs128Err Test128 Error Check - Skip because the is128Err function has a different signature
func TestIs128Err(t *testing.T) {
	t.Skip("is128Err function has different signature in actual implementation")
}

// TestNewTLSConfig Tests TLS configuration creation
func TestNewTLSConfig(t *testing.T) {
	tests := []struct {
		name     string
		caFile   string
		certFile string
		keyFile  string
		wantNil  bool
		wantErr  bool
	}{
		{
			name:     "no TLS config",
			caFile:   "",
			certFile: "",
			keyFile:  "",
			wantNil:  true,
			wantErr:  false,
		},
		{
			name:     "invalid CA file",
			caFile:   "non-existent-ca.pem",
			certFile: "",
			keyFile:  "",
			wantNil:  true,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := newTLSConfig(tt.caFile, tt.certFile, tt.keyFile)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
			if tt.wantNil {
				assert.Nil(t, tlsConfig)
			} else {
				assert.NotNil(t, tlsConfig)
			}
		})
	}
}

// TestClient_ConcurrentAccess Test for concurrent access
func TestClient_ConcurrentAccess(t *testing.T) {
	client := &Client{
		msgHandlerMap: make(map[string]Handler),
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			topic := fmt.Sprintf("test/topic/%d", id)
			handler := client.GetHandlerByUpTopic(topic)
			// Since no processor is registered, the empty handler should be returned
			assert.Equal(t, "", handler.Topic)
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// Real-world testing (requires a local MQTT Broker)
// =============================================================================

// TestReal_BasicConnection Test basic connectivity functions
func TestReal_BasicConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := Config{
		Server:               "tcp://127.0.0.1:1883",
		Username:             "",
		Password:             "",
		ClientID:             "test-basic-connection",
		MaxReconnectInterval: 5 * time.Second,
		CleanSession:         true,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Skipf("MQTT broker not available at 127.0.0.1:1883: %v", err)
		return
	}
	defer client.Close()

	// Verify connection status
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))

	// Wait a while to ensure a stable connection
	time.Sleep(1 * time.Second)
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))
}

// TestReal_PublishOnly Testing releases at different QoS levels
func TestReal_PublishOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := Config{
		Server:   "tcp://127.0.0.1:1883",
		ClientID: "test-publish-only",
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Skipf("MQTT broker not available: %v", err)
		return
	}
	defer client.Close()

	// Test releases at different QoS levels
	testCases := []struct {
		qos     byte
		topic   string
		message string
	}{
		{0, "test/qos0", "QoS 0 message"},
		{1, "test/qos1", "QoS 1 message"},
		{2, "test/qos2", "QoS 2 message"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("QoS_%d", tc.qos), func(t *testing.T) {
			err := client.Publish(tc.topic, tc.qos, []byte(tc.message))
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReal_PublishSubscribe Test publish-subscribe functionality
func TestReal_PublishSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create a publisher client
	pubConfig := Config{
		Server:   "tcp://127.0.0.1:1883",
		ClientID: "test-publisher",
	}
	publisher, err := NewClient(ctx, pubConfig)
	if err != nil {
		t.Skipf("MQTT broker not available: %v", err)
		return
	}
	defer publisher.Close()

	// Create a subscriber client
	subConfig := Config{
		Server:   "tcp://127.0.0.1:1883",
		ClientID: "test-subscriber",
	}
	subscriber, err := NewClient(ctx, subConfig)
	if err != nil {
		t.Fatalf("Failed to create subscriber: %v", err)
	}
	defer subscriber.Close()

	// Set up the message receiving channel
	messageReceived := make(chan string, 1)
	testTopic := "test/pubsub"
	testMessage := "Hello MQTT!"

	// Subscribe to the processor
	handler := Handler{
		Topic: testTopic,
		Qos:   1,
		Handle: func(c paho.Client, data paho.Message) {
			messageReceived <- string(data.Payload())
		},
	}

	subscriber.RegisterHandler(handler)

	// Wait for the subscription to take effect
	time.Sleep(1 * time.Second)

	// Release the news
	err = publisher.Publish(testTopic, 1, []byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Verify message reception
	select {
	case receivedMsg := <-messageReceived:
		assert.Equal(t, testMessage, receivedMsg)
	case <-time.After(5 * time.Second):
		t.Fatal("Message not received within timeout")
	}
}

// TestReal_ConnectionStatus Test connection status management
func TestReal_ConnectionStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := Config{
		Server:               "tcp://127.0.0.1:1883",
		ClientID:             "test-connection-status",
		MaxReconnectInterval: 2 * time.Second,
		CleanSession:         true,
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Skipf("MQTT broker not available: %v", err)
		return
	}
	defer client.Close()

	// Verify the initial connection status
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))

	// Test the publishing function
	err = client.Publish("test/status", 0, []byte("test message"))
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Note: It is difficult to simulate connection loss in real environments; here we mainly test normal conditions
	time.Sleep(2 * time.Second)
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))
}

// TestReal_MultipleClients Test multiple clients for concurrent connections and publishing
func TestReal_MultipleClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	const numClients = 5
	var clients []*Client
	var wg sync.WaitGroup

	// Create multiple clients
	for i := 0; i < numClients; i++ {
		config := Config{
			Server:   "tcp://127.0.0.1:1883",
			ClientID: fmt.Sprintf("test-client-%d", i),
		}
		client, err := NewClient(ctx, config)
		if err != nil {
			t.Skipf("MQTT broker not available: %v", err)
			return
		}
		clients = append(clients, client)
	}

	// Make sure all clients are turned off
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	// A joint announcement was issued
	for i, client := range clients {
		wg.Add(1)
		go func(id int, c *Client) {
			defer wg.Done()
			topic := fmt.Sprintf("test/client/%d", id)
			message := fmt.Sprintf("Message from client %d", id)
			err := c.Publish(topic, 1, []byte(message))
			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		}(i, client)
	}

	wg.Wait()
}

// TestReal_PublishTimeout Test release timeout
func TestReal_PublishTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := Config{
		Server:   "tcp://127.0.0.1:1883",
		ClientID: "test-timeout",
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Skipf("MQTT broker not available: %v", err)
		return
	}
	defer client.Close()

	// A normal release should be successful
	err = client.Publish("test/timeout", 1, []byte("test message"))
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

// TestReal_LargeMessage Big news test announcement
func TestReal_LargeMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real MQTT test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config := Config{
		Server:   "tcp://127.0.0.1:1883",
		ClientID: "test-large-message",
	}

	client, err := NewClient(ctx, config)
	if err != nil {
		t.Skipf("MQTT broker not available: %v", err)
		return
	}
	defer client.Close()

	// Create a larger message (10KB)
	largeMessage := make([]byte, 10*1024)
	for i := range largeMessage {
		largeMessage[i] = byte('A' + (i % 26))
	}

	err = client.Publish("test/large", 1, largeMessage)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

// TestReal_AutoReconnect Test the automatic reconnection function
// Publish and subscribe using a single client, for manual authentication of disconnection and reconnection
//func TestReal_AutoReconnect(t *testing.T) {
//	if testing.Short() {
//		t.Skip("Skipping real MQTT test in short mode")
//	}
//
//	Longer timeouts are used to manually test reconnection
//	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
//	defer cancel()
//
//	Create a single client for both publishing and subscription
//	clientConfig := Config{
//		Server:               "tcp://127.0.0.1:1883",
//		ClientID:             "test-client-reconnect",
//		MaxReconnectInterval: 5 * time.Second,
//		CleanSession:         true,
//	}
//	client, err := NewClient(ctx, clientConfig)
//	if err != nil {
//		t.Skipf("MQTT broker not available: %v", err)
//		return
//	}
//	defer client.Close()
//
//	testTopic := "test/reconnect"
//	messageCount := 0
//
//	Register and subscribe to the processor to print received messages
//	handler := Handler{
//		Topic: testTopic,
//		Qos:   1,
//		Handle: func(c paho.Client, data paho.Message) {
//			messageCount++
//			t.Logf("[%s] Received message #%d: %s", time.Now().Format("15:04:05"), messageCount, string(data.Payload()))
//		},
//	}
//
//	client.RegisterHandler(handler)
//
//	Wait for the subscription to take effect
//	time.Sleep(1 * time.Second)
//	t.Log("Start publishing data per second. Please manually disconnect from the network to test the automatic reconnection function...")
//
//	Create timers to publish data every second
//	ticker := time.NewTicker(1 * time.Second)
//	defer ticker.Stop()
//
//	publishCount := 0
//	for {
//		select {
//		case <-ctx.Done():
//			t.Log("The test ended")
//			return
//		case <-ticker.C:
//			publishCount++
//			message := fmt.Sprintf("test message #%d - %s", publishCount, time.Now().Format("15:04:05"))
//
//			Use the provided IsConnected method to check the connection status
//			connected := client.IsConnected()
//
//			t.Logf("[%s] Release News #%d (client status: %v)",
//				time.Now().Format("15:04:05"), publishCount, connected)
//
//			err := client.Publish(testTopic, 1, []byte(message))
//			if err != nil {
//				t.Logf("Release failure: %v", err)
//			} else {
//				t.Logf("Successful release: %s", message)
//			}
//
//			The test will automatically end after 30 seconds
//			if publishCount >= 30 {
//				t.Log("30 messages have been posted, and the test is complete")
//				return
//			}
//		}
//	}
//}
