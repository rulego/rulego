/*
 * Copyright 2024 The RuleGo Authors.
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

package test

import (
	"bufio"
	"net"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/test/assert"
)

func newBrokerClient(t *testing.T, addr string, handler mqtt.MessageHandler) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://" + addr).
		SetClientID("test-" + t.Name() + "-" + time.Now().Format("150405.000000000")).
		SetAutoReconnect(false).
		SetConnectTimeout(time.Second * 3).
		SetWriteTimeout(time.Second * 3).
		SetOrderMatters(false)
	if handler != nil {
		opts.SetDefaultPublishHandler(handler)
	}
	return mqtt.NewClient(opts)
}

func TestMqttBrokerAddrAndClose(t *testing.T) {
	b, err := NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer b.Close()

	assert.True(t, b.Addr() != "")
	// Close is idempotent
	b.Close()
	b.Close()
}

func TestMqttBrokerConnectPublishSubscribe(t *testing.T) {
	b, err := NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer b.Close()

	received := make(chan string, 4)
	client := newBrokerClient(t, b.Addr(), func(_ mqtt.Client, m mqtt.Message) {
		received <- string(m.Payload())
	})
	token := client.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	token = client.Subscribe("sensor/+/temp", 0, nil)
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	// wait until the broker registered the subscription
	deadline := time.Now().Add(time.Second * 3)
	for time.Now().Before(deadline) {
		if len(b.Subscriptions()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 1, len(b.Subscriptions()))
	assert.Equal(t, "sensor/+/temp", b.Subscriptions()[0])

	token = client.Publish("sensor/room1/temp", 0, false, []byte("21.5"))
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	select {
	case got := <-received:
		assert.Equal(t, "21.5", got)
	case <-time.After(time.Second * 3):
		t.Fatal("wildcard subscription did not receive the published message")
	}

	client.Disconnect(100)
}

func TestMqttBrokerUnsubscribe(t *testing.T) {
	b, err := NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer b.Close()

	client := newBrokerClient(t, b.Addr(), nil)
	token := client.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	token = client.Subscribe("a/b", 0, nil)
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	deadline := time.Now().Add(time.Second * 3)
	for time.Now().Before(deadline) && len(b.Subscriptions()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 1, len(b.Subscriptions()))

	token = client.Unsubscribe("a/b")
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	deadline = time.Now().Add(time.Second * 3)
	for time.Now().Before(deadline) && len(b.Subscriptions()) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 0, len(b.Subscriptions()))

	client.Disconnect(100)
}

func TestMqttBrokerDisconnectAll(t *testing.T) {
	b, err := NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer b.Close()

	client := newBrokerClient(t, b.Addr(), nil)
	token := client.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	b.DisconnectAll()
	// the listener stays up, so a fresh connect succeeds right away
	client2 := newBrokerClient(t, b.Addr(), nil)
	token = client2.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)
	client2.Disconnect(100)
	client.Disconnect(100)
}

func TestMqttBrokerSimulateOutage(t *testing.T) {
	b, err := NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer b.Close()

	client := newBrokerClient(t, b.Addr(), nil)
	token := client.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)

	b.SimulateOutage(400 * time.Millisecond)

	// new connections are rejected while the outage window is open
	rejected := newBrokerClient(t, b.Addr(), nil)
	token = rejected.Connect()
	token.WaitTimeout(time.Second * 2)
	assert.True(t, token.Error() != nil)

	time.Sleep(500 * time.Millisecond)

	recovered := newBrokerClient(t, b.Addr(), nil)
	token = recovered.Connect()
	token.WaitTimeout(time.Second * 5)
	assert.False(t, token.Error() != nil)
	recovered.Disconnect(100)
	client.Disconnect(100)
}

// Direct unit tests for the wire-format helpers.
func TestTopicMatch(t *testing.T) {
	tests := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"a/b", "a/b", true},
		{"a/b", "a/c", false},
		{"+", "a", true},
		{"+", "a/b", false},
		{"+/b", "a/b", true},
		{"a/#", "a", true},
		{"a/#", "a/b/c", true},
		{"a/+/#", "a/b/c/d", true},
		{"a/+", "a/b/c", false},
		{"#", "a/b/c", true},
		{"a/b", "ab", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, topicMatch(tt.filter, tt.topic), tt.filter+" vs "+tt.topic)
	}
}

func TestEncodeRemainingLength(t *testing.T) {
	tests := []struct {
		n    int
		want []byte
	}{
		{0, []byte{0x00}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x80, 0x80, 0x01}},
		{2097152, []byte{0x80, 0x80, 0x80, 0x01}},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, encodeRemainingLength(tt.n), "n=%d", tt.n)
	}
}

func TestParsePublish(t *testing.T) {
	topic, payload, pid, ok := parsePublish([]byte{0x00, 0x03, 'a', '/', 'b', 1, 2, 3}, 0)
	assert.True(t, ok)
	assert.Equal(t, "a/b", topic)
	assert.Equal(t, []byte{1, 2, 3}, payload)
	assert.Nil(t, pid)

	// QoS1: the packet id between topic and payload is stripped
	topic, payload, pid, ok = parsePublish([]byte{0x00, 0x03, 'a', '/', 'b', 0x00, 0x07, 1, 2, 3}, 1)
	assert.True(t, ok)
	assert.Equal(t, "a/b", topic)
	assert.Equal(t, []byte{1, 2, 3}, payload)
	assert.Equal(t, []byte{0x00, 0x07}, pid)

	_, _, _, ok = parsePublish([]byte{0x00}, 0)
	assert.False(t, ok)

	// declared topic longer than the body
	_, _, _, ok = parsePublish([]byte{0x00, 0x05, 'a'}, 0)
	assert.False(t, ok)

	// QoS1 body too short to carry the packet id
	_, _, _, ok = parsePublish([]byte{0x00, 0x01, 'a'}, 1)
	assert.False(t, ok)
}

// readPacket must reject a remaining-length field that never terminates
// within the four bytes allowed by the MQTT spec.
func TestReadPacketMalformedRemainingLength(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		// publish-ish first byte, then five continuation bytes
		_, _ = client.Write([]byte{0x30, 0x80, 0x80, 0x80, 0x80, 0x80})
	}()

	_, _, err := readPacket(bufio.NewReader(server))
	assert.NotNil(t, err)
}

func TestReadPacketTruncatedBody(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		// header promises 4 body bytes but only 1 arrives before close
		_, _ = client.Write([]byte{0x30, 0x04, 0x00})
		time.Sleep(50 * time.Millisecond)
		client.Close()
	}()

	_, _, err := readPacket(bufio.NewReader(server))
	assert.NotNil(t, err)
}
