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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

// Embedded in-process broker shared by all tests in this package.
var brokerServer string

func TestMain(m *testing.M) {
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	brokerServer = "tcp://" + broker.Addr()
	code := m.Run()
	broker.Close()
	os.Exit(code)
}

// TestNewClient covers connect success (explicit and random client id),
// context cancellation against an unreachable broker, and TLS material errors.
func TestNewClient(t *testing.T) {
	client, err := NewClient(context.Background(), Config{
		Server:   brokerServer,
		ClientID: "test-new-client",
	})
	assert.Nil(t, err)
	assert.True(t, client.IsConnected())
	assert.Nil(t, client.Close())

	// empty ClientID falls back to a random "rulego/xxx" id
	client, err = NewClient(context.Background(), Config{Server: brokerServer})
	assert.Nil(t, err)
	assert.True(t, client.IsConnected())
	assert.Nil(t, client.Close())

	// unreachable broker with a short context must abort the retry loop with ctx.Err()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = NewClient(ctx, Config{Server: "tcp://127.0.0.1:1", ClientID: "test-ctx-abort"})
	assert.True(t, err == context.DeadlineExceeded)

	// broken TLS material fails fast without dialing
	_, err = NewClient(context.Background(), Config{
		Server:   brokerServer,
		ClientID: "test-bad-tls",
		CAFile:   "non-existent-ca.pem",
	})
	assert.True(t, err != nil && strings.Contains(err.Error(), "error loading mqtt certificate files"))
}

// TestNewTLSConfig covers the nil shortcut, CA load failure and a real key pair.
func TestNewTLSConfig(t *testing.T) {
	tlsConfig, err := newTLSConfig("", "", "")
	assert.Nil(t, err)
	assert.Nil(t, tlsConfig)

	tlsConfig, err = newTLSConfig("non-existent-ca.pem", "", "")
	assert.True(t, err != nil)
	assert.Nil(t, tlsConfig)

	caFile, certFile, keyFile := writeSelfSignedCert(t)
	tlsConfig, err = newTLSConfig(caFile, certFile, keyFile)
	assert.Nil(t, err)
	assert.NotNil(t, tlsConfig)
	assert.NotNil(t, tlsConfig.RootCAs)
	assert.Equal(t, 1, len(tlsConfig.Certificates))
}

// TestClientPublishSubscribe drives one full lifecycle against the embedded
// broker: subscribe, QoS0/1/2 publish, large payload, concurrent clients,
// unsubscribe and Close.
func TestClientPublishSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	publisher, err := NewClient(ctx, Config{Server: brokerServer, ClientID: "test-publisher"})
	assert.Nil(t, err)
	defer publisher.Close()

	subscriber, err := NewClient(ctx, Config{Server: brokerServer, ClientID: "test-subscriber"})
	assert.Nil(t, err)
	defer subscriber.Close()

	received := make(chan string, 16)
	topic := "test/pubsub"
	subscriber.RegisterHandler(Handler{
		Topic: topic,
		Qos:   1,
		Handle: func(c paho.Client, data paho.Message) {
			received <- string(data.Payload())
		},
	})
	// subscription must be visible before publishing
	time.Sleep(500 * time.Millisecond)

	for _, qos := range []byte{0, 1, 2} {
		assert.Nil(t, publisher.Publish(topic, qos, []byte(fmt.Sprintf("qos-%d", qos))))
	}
	for i := 0; i < 3; i++ {
		select {
		case msg := <-received:
			assert.True(t, strings.HasPrefix(msg, "qos-"))
		case <-time.After(5 * time.Second):
			t.Fatalf("message %d not received within timeout", i)
		}
	}

	// 10KB payload survives the round trip
	large := make([]byte, 10*1024)
	for i := range large {
		large[i] = byte('A' + i%26)
	}
	assert.Nil(t, publisher.Publish(topic, 1, large))
	select {
	case msg := <-received:
		assert.Equal(t, len(large), len(msg))
	case <-time.After(5 * time.Second):
		t.Fatal("large message not received within timeout")
	}

	// several clients publishing concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c, err := NewClient(ctx, Config{
				Server:   brokerServer,
				ClientID: fmt.Sprintf("test-concurrent-%d", id),
			})
			if err != nil {
				t.Errorf("concurrent client %d: %v", id, err)
				return
			}
			defer c.Close()
			if err := c.Publish(fmt.Sprintf("test/client/%d", id), 1, []byte("hello")); err != nil {
				t.Errorf("concurrent publish %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	// unregister stops delivery, unknown topic is a no-op
	assert.Nil(t, subscriber.UnregisterHandler(topic))
	assert.Nil(t, subscriber.UnregisterHandler("never/registered"))
	assert.Nil(t, subscriber.Close())
	assert.Nil(t, publisher.Close())
}

// TestClientReconnect simulates a broker outage and verifies the client
// reports disconnected, stays reconnecting for the outage window, then
// recovers and publishes again.
func TestClientReconnect(t *testing.T) {
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer broker.Close()

	client, err := NewClient(context.Background(), Config{
		Server:               "tcp://" + broker.Addr(),
		ClientID:             "test-reconnect",
		MaxReconnectInterval: time.Second,
	})
	assert.Nil(t, err)
	defer client.Close()
	assert.True(t, client.IsConnected())

	// outage keeps reconnects rejected long enough to observe the 0 state
	broker.SimulateOutage(3 * time.Second)
	deadline := time.Now().Add(10 * time.Second)
	for client.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, !client.IsConnected(), "client should observe the connection drop")

	deadline = time.Now().Add(15 * time.Second)
	for !client.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, client.IsConnected(), "client should reconnect after the outage ends")
	assert.Nil(t, client.Publish("test/reconnect", 1, []byte("back")))
}

// TestClient_Publish_NotConnected covers the guard on disconnected clients.
func TestClient_Publish_NotConnected(t *testing.T) {
	client := &Client{isConnected: 0}
	err := client.Publish("test/topic", 0, []byte("test message"))
	assert.True(t, err != nil && strings.Contains(err.Error(), "MQTT client is not connected"))
	assert.False(t, client.IsConnected())
}

// TestClient_ConnectionStatus drives the connect/lost callbacks directly.
func TestClient_ConnectionStatus(t *testing.T) {
	client := &Client{isConnected: 0}
	assert.Equal(t, int32(0), atomic.LoadInt32(&client.isConnected))

	client.onConnected(nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&client.isConnected))

	client.onConnectionLost(nil, nil)
	assert.Equal(t, int32(0), atomic.LoadInt32(&client.isConnected))
}

// TestClient_ConcurrentAccess verifies handler lookups under concurrency.
func TestClient_ConcurrentAccess(t *testing.T) {
	client := &Client{msgHandlerMap: make(map[string]Handler)}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handler := client.GetHandlerByUpTopic(fmt.Sprintf("test/topic/%d", id))
			assert.Equal(t, "", handler.Topic)
		}(i)
	}
	wg.Wait()
}

// newUnreachableClient builds a client pointed at a dead broker without going
// through NewClient's retry loop; paho returns an error token immediately.
func newUnreachableClient() *Client {
	b := &Client{
		msgHandlerMap: make(map[string]Handler),
		isConnected:   0,
	}
	opts := paho.NewClientOptions()
	opts.AddBroker("tcp://127.0.0.1:1")
	opts.SetClientID("rulego-test-unreachable")
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(2 * time.Second)
	opts.SetOnConnectHandler(b.onConnected)
	opts.SetConnectionLostHandler(b.onConnectionLost)
	opts.SetReconnectingHandler(b.onReconnecting)
	b.client = paho.NewClient(opts)
	return b
}

// A broker that is permanently unreachable must not let the subscribe retry
// hold the write lock, or GetHandlerByUpTopic/UnregisterHandler/Close starve.
func TestClient_RegisterHandlerNoLockStarvation(t *testing.T) {
	b := newUnreachableClient()

	go b.RegisterHandler(Handler{
		Topic: "test/starvation",
		Qos:   1,
		Handle: func(c paho.Client, data paho.Message) {
		},
	})

	// the handler must be readable while the subscribe retries; poll in a
	// goroutine so a held lock fails the test via timeout instead of hanging.
	found := make(chan struct{})
	go func() {
		for {
			if h := b.GetHandlerByUpTopic("test/starvation"); h.Topic == "test/starvation" {
				close(found)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-found:
	case <-time.After(3 * time.Second):
		t.Fatal("GetHandlerByUpTopic starved: RegisterHandler holds lock during subscribe retry")
	}
}

// TestNormalizeConfigKeys covers legacy key case mapping.
func TestNormalizeConfigKeys(t *testing.T) {
	configuration := map[string]interface{}{
		"qOS":      uint8(1),
		"clientID": "legacy",
		"cAFile":   "/tmp/ca.pem",
		"other":    "keep",
	}
	NormalizeConfigKeys(configuration)
	assert.Equal(t, uint8(1), configuration["qos"])
	assert.Equal(t, "legacy", configuration["clientId"])
	assert.Equal(t, "/tmp/ca.pem", configuration["caFile"])
	assert.Equal(t, "keep", configuration["other"])

	// new-style keys win over legacy ones
	configuration = map[string]interface{}{"qOS": uint8(0), "qos": uint8(2)}
	NormalizeConfigKeys(configuration)
	assert.Equal(t, uint8(2), configuration["qos"])
}

// writeSelfSignedCert generates a self-signed certificate and returns the
// PEM-encoded CA, cert and key file paths.
func writeSelfSignedCert(t *testing.T) (caFile, certFile, keyFile string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "rulego-mqtt-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	assert.Nil(t, err)

	dir := t.TempDir()
	caFile = filepath.Join(dir, "ca.pem")
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	assert.Nil(t, os.WriteFile(caFile, certPem, 0o600))
	assert.Nil(t, os.WriteFile(certFile, certPem, 0o600))
	keyDER, err := x509.MarshalECPrivateKey(key)
	assert.Nil(t, err)
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	assert.Nil(t, os.WriteFile(keyFile, keyPem, 0o600))
	return
}
