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

package mqtt

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	endpoint "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/mqtt"
)

// waitFor polls cond until it returns true or the timeout expires; calls t.Fatal on timeout.
func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s not reached within %s", what, timeout)
}

// TestMqttEndpointStatusLifecycle covers the mqtt endpoint connection-status lifecycle:
// connect (Connected) -> message flow -> broker failure (Reconnecting) -> paho auto-reconnect (Connected).
func TestMqttEndpointStatusLifecycle(t *testing.T) {
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer broker.Close()
	addr := broker.Addr()

	config := engine.NewConfig()
	ep := &Mqtt{
		Config: mqtt.Config{
			Server:               addr,
			QOS:                  0,
			MaxReconnectInterval: 2 * time.Second, // speed up paho reconnect backoff
		},
	}
	// configuration only sets server/qos; MaxReconnectInterval keeps the Config value
	assert.Nil(t, ep.Init(config, types.Configuration{"server": addr, "qos": 0}))

	var received int64
	router := impl.NewRouter().From("/status/test").Transform(func(r endpoint.Router, ex *endpoint.Exchange) bool {
		atomic.AddInt64(&received, 1)
		return true
	}).End()
	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	assert.Nil(t, ep.Start())
	defer ep.Destroy()

	// 1. connected on start
	waitFor(t, "initial connect -> Connected", 5*time.Second, func() bool {
		return ep.ConnectionStatus().Status == types.StatusConnected
	})

	// 2. message flow: publisher publishes, endpoint subscribes, Transform counts
	pubCtx, pubCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pubCancel()
	publisher, err := mqtt.NewClient(pubCtx, mqtt.Config{Server: addr, MaxReconnectInterval: 2 * time.Second})
	assert.Nil(t, err)
	defer publisher.Close()
	assert.Nil(t, publisher.Publish("/status/test", 0, []byte("hi")))
	waitFor(t, "receive published message", 3*time.Second, func() bool {
		return atomic.LoadInt64(&received) >= 1
	})

	// 3. broker outage (drop connections AND reject new ones for a while) -> status stably Reconnecting
	broker.SimulateOutage(3 * time.Second)
	waitFor(t, "disconnect -> Reconnecting", 5*time.Second, func() bool {
		return ep.ConnectionStatus().Status == types.StatusReconnecting
	})

	// 4. outage window ends, paho auto-reconnect recovers -> Connected
	waitFor(t, "auto-reconnect -> Connected", 12*time.Second, func() bool {
		return ep.ConnectionStatus().Status == types.StatusConnected
	})
}
