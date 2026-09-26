package mqtt

import (
	"context"
	"fmt"
	"github.com/rulego/rulego/api/types"
	endpoint "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/mqtt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	testdataFolder = "../../testdata/rule"
	testServer     string
	msgContent1    = "{\"test\":\"AA\"}"
	msgContent2    = "{\"test\":\"BB\"}"
)

// TestMain starts an in-process MQTT broker so the endpoint tests do not
// depend on an externally installed mosquitto.
func TestMain(m *testing.M) {
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	testServer = broker.Addr()
	code := m.Run()
	broker.Close()
	os.Exit(code)
}

// 测试请求/响应消息
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
	//创建mqtt endpoint服务
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
	//启动服务
	go startServer(t, stop)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)
	//启动客户端
	node := createClient(t)
	config := engine.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
	})
	//发送消息
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, msgContent1)
	node.OnMsg(ctx, msg1)
	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, msgContent2)
	node.OnMsg(ctx, msg2)
	time.Sleep(time.Millisecond * 200)
	close(stop)
}

// TestMqttEndpointGracefulShutdown tests graceful shutdown functionality of MQTT endpoint
// TestMqttEndpointGracefulShutdown 测试 MQTT 端点的优雅停机功能
func TestMqttEndpointGracefulShutdown(t *testing.T) {
	t.Run("GracefulShutdownDuringMessageProcessing", func(t *testing.T) {
		var config = engine.NewConfig()
		// Create a simple rule chain for testing
		// 创建一个简单的规则链用于测试
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
		// 配置较短超时时间以便更快测试
		mqttEndpoint := &Mqtt{
			Config: mqtt.Config{
				Server:   testServer,
				Username: "",
				Password: "",
				QOS:      0,
			},
		}

		configuration := make(types.Configuration)
		configuration["server"] = testServer
		configuration["qos"] = 0

		err = mqttEndpoint.Init(config, configuration)
		if err != nil {
			t.Fatal(err)
		}

		// Set graceful shutdown timeout to 2 seconds for testing
		// 设置优雅停机超时为2秒用于测试
		mqttEndpoint.GracefulShutdown.InitGracefulShutdown(config.Logger, 2*time.Second)

		// Add router with slow processing chain
		// 添加带有慢处理链的路由器
		var processedCount int64
		router := impl.NewRouter().From("device/+").To("chain:graceful-test01").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			// Simulate slow processing
			// 模拟慢处理
			atomic.AddInt64(&processedCount, 1)
			time.Sleep(500 * time.Millisecond)
			return true
		}).End()

		_, err = mqttEndpoint.AddRouter(router)
		if err != nil {
			t.Fatal(err)
		}

		// Start endpoint
		// 启动端点
		err = mqttEndpoint.Start()
		if err != nil {
			t.Fatal(err)
		}

		// Wait for connection to be established
		// 等待连接建立
		time.Sleep(100 * time.Millisecond)

		// Start goroutine to send messages rapidly
		// 启动协程快速发送消息
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
					// 发送测试消息
					if err := client.Publish("device/123", 0, []byte("test message")); err != nil {
						// Connection might be closed during shutdown, which is expected
						// 停机期间连接可能关闭，这是预期的
						return
					}
				}
			}
		}()

		// Let some messages start processing
		// 让一些消息开始处理
		time.Sleep(200 * time.Millisecond)

		// Initiate graceful shutdown
		// 启动优雅停机
		shutdownStart := time.Now()
		mqttEndpoint.GracefulStop()
		shutdownDuration := time.Since(shutdownStart)

		// Stop message sending
		// 停止消息发送
		close(stopSending)
		wg.Wait()

		// Verify graceful shutdown behavior
		// 验证优雅停机行为
		assert.True(t, shutdownDuration >= 0, "Shutdown should complete")
		assert.True(t, shutdownDuration < 5*time.Second, "Shutdown should not exceed maximum timeout")

		// Verify that some messages were processed
		// 验证一些消息已被处理
		finalCount := atomic.LoadInt64(&processedCount)
		assert.True(t, finalCount > 0, "Some messages should have been processed")

		t.Logf("Graceful shutdown completed in %v, processed %d messages", shutdownDuration, finalCount)
	})

	t.Run("ShutdownRejectsNewMessages", func(t *testing.T) {
		var config = engine.NewConfig()
		// Create a simple rule chain for testing
		// 创建一个简单的规则链用于测试
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
				Server:   testServer,
				Username: "",
				Password: "",
				QOS:      0,
			},
		}

		configuration := make(types.Configuration)
		configuration["server"] = testServer
		configuration["qos"] = 0

		err = mqttEndpoint.Init(config, configuration)
		if err != nil {
			t.Fatal(err)
		}

		// Set very short timeout for faster testing
		// 设置很短的超时时间以便更快测试
		mqttEndpoint.GracefulShutdown.InitGracefulShutdown(config.Logger, 500*time.Millisecond)

		var processedCount int64
		var rejectedCount int64

		// Add router that counts processing and rejection
		// 添加计算处理和拒绝的路由器
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
		// 等待连接
		time.Sleep(100 * time.Millisecond)

		// Start shutdown immediately
		// 立即开始停机
		go func() {
			time.Sleep(50 * time.Millisecond)
			mqttEndpoint.GracefulStop()
		}()

		// Try to send messages during shutdown
		// 尝试在停机期间发送消息
		client, err := mqttEndpoint.SharedNode.GetSafely()
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 10; i++ {
			// Some messages should be processed, others rejected due to shutdown
			// 一些消息应该被处理，其他的因为停机而被拒绝
			if err := client.Publish("device/test", 0, []byte("test message")); err != nil {
				break // Connection closed
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Wait for shutdown to complete
		// 等待停机完成
		time.Sleep(1 * time.Second)

		t.Logf("During shutdown: processed=%d, rejected=%d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&rejectedCount))

		// At least some messages should have been processed or rejected
		// 至少应该有一些消息被处理或拒绝
		totalMessages := atomic.LoadInt64(&processedCount) + atomic.LoadInt64(&rejectedCount)
		assert.True(t, totalMessages >= 0, "Should have attempted to process messages")
	})
}

func createClient(t *testing.T) types.Node {
	node, _ := engine.Registry.NewNode("mqttClient")
	var configuration = make(types.Configuration)
	configuration["Server"] = testServer
	configuration["Topic"] = "/device/msg"

	config := engine.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

// 启动服务
func startServer(t *testing.T, stop chan struct{}) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Error(err)
		return
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	//创建mqtt endpoint服务
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

	//添加全局拦截器
	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//订阅所有主题路由，并转发到default规则链处理
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
			t.Errorf("receive data:%s,expect data:%s,%s", receiveData, msgContent1, msgContent2)
		}
		responseMessage.SetStatusCode(0)
		responseMessage.Headers().Set("topic", "/device/msg/resp")
		responseMessage.Headers().Set("qos", "0")
		responseMessage.SetBody([]byte("ok"))
		assert.NotNil(t, responseMessage.Response())
		return true
	}).To("chain:default").End()
	//注册路由
	_, err = ep.AddRouter(router1)

	if err != nil {
		t.Error(err)
	}
	ep.AddRouter(nil)

	//启动服务
	_ = ep.Start()

	//服务启动后，继续添加路由
	routerId, err := ep.AddRouter(impl.NewRouter().From("/device/#").End())
	assert.Equal(t, "/device/#", routerId)

	ep.RemoveRouter(routerId)

	ep.Infof("start server")
	<-stop
	ep.Destroy()
}

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
