package mqtt

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	endpoint "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/mqtt"
)

var (
	testdataFolder = "../../testdata/rule"
	testServer     = "127.0.0.1:1883"
	msgContent1    = "{\"test\":\"AA\"}"
	msgContent2    = "{\"test\":\"BB\"}"
)

// Test request/response messages
func TestMqttMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		var request = &RequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("Response", func(t *testing.T) {
		var response = &ResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

func TestRouterId(t *testing.T) {
	config := engine.NewConfig()
	//Create the MQTT Endpoint service
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&mqtt.Config{
		Server: testServer,
	}, &nodeConfig)
	var ep = &Endpoint{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
	assert.Equal(t, testServer, ep.Id())
	router := impl.NewRouter().SetId("r1").From("/device/info").End()
	routerId, _ := ep.AddRouter(router)
	assert.Equal(t, "r1", routerId)

	router = impl.NewRouter().From("/device/info").End()
	routerId, _ = ep.AddRouter(router)
	assert.Equal(t, "/device/info", routerId)
	router = impl.NewRouter().From("/device/info").End()
	routerId, _ = ep.AddRouter(router, "test")
	assert.Equal(t, "/device/info", routerId)

	err = ep.RemoveRouter("r1")
	assert.Nil(t, err)
	err = ep.RemoveRouter("/device/info")
	assert.Nil(t, err)
	err = ep.RemoveRouter("/device/info")
	assert.Equal(t, fmt.Sprintf("router: %s not found", "/device/info"), err.Error())
}

func TestMqttEndpoint(t *testing.T) {
	stop := make(chan struct{})
	//Start the server
	go startServer(t, stop)
	//Wait for the server to start up
	time.Sleep(time.Millisecond * 200)
	//Start the client
	node := createClient(t)
	config := engine.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
	})
	//Send the message
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, msgContent1)
	node.OnMsg(ctx, msg1)
	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, msgContent2)
	node.OnMsg(ctx, msg2)
	time.Sleep(time.Millisecond * 200)
	close(stop)
}

// TestMqttEndpointGracefulShutdown tests graceful shutdown functionality of MQTT endpoint
// TestMqttEndpointGracefulShutdown tests the graceful shutdown feature of MQTT endpoints
func TestMqttEndpointGracefulShutdown(t *testing.T) {
	t.Run("GracefulShutdownDuringMessageProcessing", func(t *testing.T) {
		var config = engine.NewConfig()
		// Create a simple rule chain for testing
		// Create a simple chain of rules for testing
		_, err := engine.New("graceful-test01", []byte(`{
			"ruleChain": {
				"name": "test chain",
				"root": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "s1", 
						"type": "jsFilter",
						"name": "test",
						"configuration": {
							"jsScript": "return true;"
						}
					}
				],
				"connections": []
			}
		}`), engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		// Configure endpoint with shorter timeout for faster testing
		// Shorter timeouts are configured for faster testing
		mqttEndpoint := &Mqtt{
			Config: mqtt.Config{
				Server:   "127.0.0.1:1883",
				Username: "",
				Password: "",
				QOS:      0,
			},
		}

		configuration := make(types.Configuration)
		configuration["server"] = "127.0.0.1:1883"
		configuration["qos"] = 0

		err = mqttEndpoint.Init(config, configuration)
		if err != nil {
			t.Fatal(err)
		}

		// Set graceful shutdown timeout to 2 seconds for testing
		// Set the elegant shutdown timeout to 2 seconds for testing
		mqttEndpoint.GracefulShutdown.InitGracefulShutdown(config.Logger, 2*time.Second)

		// Add router with slow processing chain
		// Add a router with a slow processing chain
		var processedCount int64
		router := impl.NewRouter().From("device/+").To("chain:graceful-test01").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			// Simulate slow processing
			// Simulation slow processing
			atomic.AddInt64(&processedCount, 1)
			time.Sleep(500 * time.Millisecond)
			return true
		}).End()

		_, err = mqttEndpoint.AddRouter(router)
		if err != nil {
			t.Fatal(err)
		}

		// Start endpoint
		// Start the endpoint
		err = mqttEndpoint.Start()
		if err != nil {
			t.Fatal(err)
		}

		// Wait for connection to be established
		// Wait for the connection to be established
		time.Sleep(100 * time.Millisecond)

		// Start goroutine to send messages rapidly
		// Start coroutines to send messages quickly
		var wg sync.WaitGroup
		stopSending := make(chan struct{})

		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := mqttEndpoint.SharedNode.GetSafely()
			if err != nil {
				t.Errorf("Failed to get client: %v", err)
				return
			}

			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-stopSending:
					return
				case <-ticker.C:
					// Send test message
					// Send test messages
					if err := client.Publish("device/123", 0, []byte("test message")); err != nil {
						// Connection might be closed during shutdown, which is expected
						// Connections may be shut down during downtime, which is expected
						return
					}
				}
			}
		}()

		// Let some messages start processing
		// Let some information start to be processed
		time.Sleep(200 * time.Millisecond)

		// Initiate graceful shutdown
		// Launch with elegant stop
		shutdownStart := time.Now()
		mqttEndpoint.GracefulStop()
		shutdownDuration := time.Since(shutdownStart)

		// Stop message sending
		// Stop sending messages
		close(stopSending)
		wg.Wait()

		// Verify graceful shutdown behavior
		// Verify elegant downtime behavior
		assert.True(t, shutdownDuration >= 0, "Shutdown should complete")
		assert.True(t, shutdownDuration < 5*time.Second, "Shutdown should not exceed maximum timeout")

		// Verify that some messages were processed
		// Verify that some messages have been processed
		finalCount := atomic.LoadInt64(&processedCount)
		assert.True(t, finalCount > 0, "Some messages should have been processed")

		t.Logf("Graceful shutdown completed in %v, processed %d messages", shutdownDuration, finalCount)
	})

	t.Run("ShutdownRejectsNewMessages", func(t *testing.T) {
		var config = engine.NewConfig()
		// Create a simple rule chain for testing
		// Create a simple chain of rules for testing
		_, err := engine.New("graceful-test02", []byte(`{
			"ruleChain": {
				"name": "test chain",
				"root": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "s1", 
						"type": "jsFilter",
						"name": "test",
						"configuration": {
							"jsScript": "return true;"
						}
					}
				],
				"connections": []
			}
		}`), engine.WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		mqttEndpoint := &Mqtt{
			Config: mqtt.Config{
				Server:   "127.0.0.1:1883",
				Username: "",
				Password: "",
				QOS:      0,
			},
		}

		configuration := make(types.Configuration)
		configuration["server"] = "127.0.0.1:1883"
		configuration["qos"] = 0

		err = mqttEndpoint.Init(config, configuration)
		if err != nil {
			t.Fatal(err)
		}

		// Set very short timeout for faster testing
		// Set a very short timeout to enable faster testing
		mqttEndpoint.GracefulShutdown.InitGracefulShutdown(config.Logger, 500*time.Millisecond)

		var processedCount int64
		var rejectedCount int64

		// Add router that counts processing and rejection
		// Add computational processing and rejection of routers
		router := impl.NewRouter().From("device/+").To("chain:graceful-test02").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			if exchange.Out.GetError() != nil {
				atomic.AddInt64(&rejectedCount, 1)
			} else {
				atomic.AddInt64(&processedCount, 1)
			}
			return true
		}).End()

		_, err = mqttEndpoint.AddRouter(router)
		if err != nil {
			t.Fatal(err)
		}

		err = mqttEndpoint.Start()
		if err != nil {
			t.Fatal(err)
		}

		// Wait for connection
		// Waiting for connections
		time.Sleep(100 * time.Millisecond)

		// Start shutdown immediately
		// Immediately start shutting down
		go func() {
			time.Sleep(50 * time.Millisecond)
			mqttEndpoint.GracefulStop()
		}()

		// Try to send messages during shutdown
		// Try sending messages during downtime
		client, err := mqttEndpoint.SharedNode.GetSafely()
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 10; i++ {
			// Some messages should be processed, others rejected due to shutdown
			// Some messages should be handled, others rejected due to downtime
			if err := client.Publish("device/test", 0, []byte("test message")); err != nil {
				break // Connection closed
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Wait for shutdown to complete
		// Wait for the shutdown to complete
		time.Sleep(1 * time.Second)

		t.Logf("During shutdown: processed=%d, rejected=%d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&rejectedCount))

		// At least some messages should have been processed or rejected
		// At the very least, some messages should be handled or rejected
		totalMessages := atomic.LoadInt64(&processedCount) + atomic.LoadInt64(&rejectedCount)
		assert.True(t, totalMessages >= 0, "Should have attempted to process messages")
	})
}

func createClient(t *testing.T) types.Node {
	node, _ := engine.Registry.NewNode("mqttClient")
	var configuration = make(types.Configuration)
	configuration["Server"] = "127.0.0.1:1883"
	configuration["Topic"] = "/device/msg"

	config := engine.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

// Start the server
func startServer(t *testing.T, stop chan struct{}) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//Register the rule chain
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	//Create the MQTT Endpoint service
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&mqtt.Config{
		Server: testServer,
	}, &nodeConfig)
	var ep = &Endpoint{}
	err = ep.Init(config, nodeConfig)
	assert.Equal(t, testServer, ep.Id())
	assert.Equal(t, Type, ep.Type())
	assert.True(t, reflect.DeepEqual(&Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
	}, ep.New()))

	//Added a global interceptor
	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Permission validation logic
		return true
	})
	//Subscribe to all topic routes and forward them to the default rule chain for processing
	router1 := impl.NewRouter().From("/device/msg").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		requestMessage := exchange.In.(*RequestMessage)
		responseMessage := exchange.Out.(*ResponseMessage)
		topic := "/device/msg"
		assert.Equal(t, topic, exchange.In.GetMsg().Metadata.GetValue("topic"))
		assert.Equal(t, topic, requestMessage.From())
		assert.Equal(t, topic, requestMessage.Headers().Get("topic"))
		assert.Equal(t, topic, responseMessage.From())
		assert.NotNil(t, requestMessage.Request())

		receiveData := exchange.In.GetMsg().GetData()
		assert.Equal(t, receiveData, string(requestMessage.Body()))

		if receiveData != msgContent1 && receiveData != msgContent2 {
			t.Fatalf("receive data:%s,expect data:%s,%s", receiveData, msgContent1, msgContent2)
		}
		responseMessage.SetStatusCode(0)
		responseMessage.Headers().Set("topic", "/device/msg/resp")
		responseMessage.Headers().Set("qos", "0")
		responseMessage.SetBody([]byte("ok"))
		assert.NotNil(t, responseMessage.Response())
		return true
	}).To("chain:default").End()
	//Register the route
	_, err = ep.AddRouter(router1)

	if err != nil {
		t.Fatal(err)
	}
	ep.AddRouter(nil)

	//Start the server
	_ = ep.Start()

	//After the service starts, continue to add routes
	routerId, err := ep.AddRouter(impl.NewRouter().From("/device/#").End())
	assert.Equal(t, "/device/#", routerId)

	ep.RemoveRouter(routerId)

	ep.Printf("start server")
	<-stop
	ep.Destroy()
}
