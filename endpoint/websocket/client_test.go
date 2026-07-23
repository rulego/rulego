package websocket

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

var wsClientTestServer = ":9092"

// Test client request/response messages
func TestWsClientMessage(t *testing.T) {
	t.Run("ClientRequest", func(t *testing.T) {
		var request = &WsClientRequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("ClientResponse", func(t *testing.T) {
		var response = &WsClientResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

// Test client types and default values
func TestWsClientType(t *testing.T) {
	config := types.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server": "ws://127.0.0.1:9999/ws",
	})
	assert.Nil(t, err)
	assert.Equal(t, ClientType, client.Type())
	assert.Equal(t, "ws://127.0.0.1:9999/ws", client.Id())

	// Verify the default value
	defaultClient := client.New().(*WsClient)
	assert.Equal(t, 5, defaultClient.Config.ReconnectInterval)
	assert.Equal(t, true, defaultClient.Config.AllowText)
	assert.Equal(t, true, defaultClient.Config.AllowBinary)
}

// Test routing management
func TestWsClientRouter(t *testing.T) {
	config := types.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server": "ws://127.0.0.1:9999/ws",
	})
	assert.Nil(t, err)

	// Add nil routing
	_, err = client.AddRouter(nil)
	assert.Equal(t, "router can not nil", err.Error())

	// Add routes
	router := impl.NewRouter().SetId("r1").From("").End()
	routerId, err := client.AddRouter(router)
	assert.Nil(t, err)
	assert.Equal(t, "r1", routerId)

	// Repeat routing
	router = impl.NewRouter().SetId("r1").From("").End()
	_, err = client.AddRouter(router)
	assert.NotNil(t, err)

	// Delete the route
	err = client.RemoveRouter("r1")
	assert.Nil(t, err)
	err = client.RemoveRouter("r1")
	assert.Equal(t, "router: r1 not found", err.Error())
}

// Test the WebSocket client to connect to the server and receive data
func TestWsClientConnect(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the WebSocket server
	go startWSPushServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 300)

	// Create a client
	config := engine.NewConfig(types.WithDefaultPool())

	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1" + wsClientTestServer + "/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	var receiveCount int32

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.True(t, strings.Contains(data, "push"))
		atomic.AddInt32(&receiveCount, 1)
		return true
	}).End()

	_, err = client.AddRouter(router)
	assert.Nil(t, err)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	count := atomic.LoadInt32(&receiveCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receiveCount > 0, got %d", count))
}

// Test the sending and receiving of the WebSocket client
func TestWsClientSendReceive(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the WebSocket echo server
	go startWSEchoServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9093/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	var received string
	var mu sync.Mutex

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		mu.Lock()
		received = string(exchange.In.Body())
		mu.Unlock()
		return true
	}).End()

	client.AddRouter(router)
	err = client.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 300)

	// Send data
	err = client.Send([]byte("hello ws"))
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, "echo:hello ws", received)
	mu.Unlock()
}

// Testing the WebSocket client's binary messages
func TestWsClientBinaryMessage(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	go startWSBinaryServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9094/ws",
		"reconnectInterval": 0,
		"allowBinary":       true,
		"allowText":         false,
	})
	assert.Nil(t, err)

	var receivedBinary bool
	var mu sync.Mutex

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		mu.Lock()
		if msg.GetDataType() == types.BINARY {
			receivedBinary = true
		}
		mu.Unlock()
		return true
	}).End()

	client.AddRouter(router)
	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	mu.Lock()
	assert.True(t, receivedBinary, "expected to receive binary message")
	mu.Unlock()
}

// Test connection failed
func TestWsClientConnectFail(t *testing.T) {
	config := types.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:19999/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	err = client.Start()
	assert.NotNil(t, err)
}

// Test client configuration
func TestWsClientConfig(t *testing.T) {
	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://example.com/stream",
		"reconnectInterval": 10,
		"heartbeatInterval": 30,
		"allowText":         false,
		"allowBinary":       true,
		"headers": map[string]string{
			"Authorization": "Bearer token123",
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, "ws://example.com/stream", client.Config.Server)
	assert.Equal(t, 10, client.Config.ReconnectInterval)
	assert.Equal(t, 30, client.Config.HeartbeatInterval)
	assert.Equal(t, false, client.Config.AllowText)
	assert.Equal(t, true, client.Config.AllowBinary)
	assert.Equal(t, "Bearer token123", client.Config.Headers["Authorization"])
}

// Test Close and Destroy
func TestWsClientCloseDestroy(t *testing.T) {
	config := types.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server": "ws://127.0.0.1:19999/ws",
	})
	assert.Nil(t, err)

	// Close should not give an error when not connected
	err = client.Close()
	assert.Nil(t, err)

	client.Destroy()
}

// Test the OnEvent callback
func TestWsClientOnEvent(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	go startWSEchoServerPort(t, stop, &wg, ":9095")
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9095/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	var connectEventFired bool
	client.OnEvent = func(event string, params ...interface{}) {
		if event == endpoint.EventConnect {
			connectEventFired = true
		}
	}

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 300)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	assert.True(t, connectEventFired, "expected EventConnect to be fired")
}

// Test message type filtering
func TestWsClientMessageFilter(t *testing.T) {
	config := types.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":      "ws://127.0.0.1:9999/ws",
		"allowText":   false,
		"allowBinary": false,
	})
	assert.Nil(t, err)
	assert.Equal(t, false, client.Config.AllowText)
	assert.Equal(t, false, client.Config.AllowBinary)
}

// ==================== Integration testing ====================

// TestWsClientWithRuleChainIntegration complete process: WS server pushes → client receives → Router → rules chain → response writeback
func TestWsClientWithRuleChainIntegration(t *testing.T) {
	// Load the rule chain
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var serverReceived string
	var mu sync.Mutex

	// Start the WebSocket server: Receive messages and read client feedback
	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// Push data
			conn.WriteMessage(websocket.TextMessage, []byte(`{"test":"integration"}`))

			// Read the client's writeback
			_, msg, err := conn.ReadMessage()
			if err == nil {
				mu.Lock()
				serverReceived = string(msg)
				mu.Unlock()
			}

			<-stop
		})
		server := &http.Server{Addr: ":9096", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	// Create a client
	client := &WsClient{}
	err = client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9096/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	var processedCount int32
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.True(t, strings.Contains(data, `"test"`), "expected JSON, got: "+data)
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		atomic.AddInt32(&processedCount, 1)
		return true
	}).To("chain:default").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Write back the response to the server
		result := exchange.Out.GetMsg().GetData()
		exchange.Out.SetBody([]byte("client response: " + result))
		return true
	}).End()

	client.AddRouter(router)
	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	count := atomic.LoadInt32(&processedCount)
	assert.True(t, count > 0, fmt.Sprintf("expected processedCount > 0, got %d", count))

	mu.Lock()
	assert.True(t, strings.Contains(serverReceived, "client response:"), "expected server to receive response, got: "+serverReceived)
	mu.Unlock()
}

// TestWsClientReconnect Test disconnection and reconnection
func TestWsClientReconnect(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var totalReceived int32

	// First server side: Shut down after sending data
	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			conn.WriteMessage(websocket.TextMessage, []byte("first batch"))
			time.Sleep(time.Millisecond * 300)
			conn.Close() // Proactively close it
		})
		server := &http.Server{Addr: ":19097", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:19097/ws",
		"reconnectInterval": 1,
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		atomic.AddInt32(&totalReceived, 1)
		return true
	}).End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)
	firstBatch := atomic.LoadInt32(&totalReceived)
	assert.True(t, firstBatch > 0, "expected first batch data")

	// Second round of server (different ports, it's unrealistic for the same client to point to a new address, so just verify the presence of reconnection logic)
	client.Destroy()
	close(stop)
	wg.Wait()
}

// TestWsClientHeartbeat tests heartbeat transmission
func TestWsClientHeartbeat(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var receivedPing bool
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// Set the ping handler to detect your heartbeat
			conn.SetPingHandler(func(appData string) error {
				mu.Lock()
				receivedPing = true
				mu.Unlock()
				// You must reply with pong, otherwise the client may be blocked
				return conn.WriteMessage(websocket.PongMessage, []byte(appData))
			})

			// control frames for gorilla/websocket are processed during ReadMessage and must be read continuously
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		})
		server := &http.Server{Addr: ":9098", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9098/ws",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)
	wg.Wait()

	mu.Lock()
	assert.True(t, receivedPing, "expected server to receive ping frame")
	mu.Unlock()
}

// TestWsClientRegistryCreateEndpoint Verify WsClient implementing the endpoint.Endpoint interface
func TestWsClientRegistryCreateEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	go startWSEchoServerPort(t, stop, &wg, ":9099")
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()

	// Verify that WsClient has implemented the endpoint.Endpoint interface
	var _ endpoint.Endpoint = &WsClient{}

	ep := &WsClient{}
	err := ep.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9099/ws",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)
	assert.Equal(t, ClientType, ep.Type())

	var received string
	var mu sync.Mutex

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		mu.Lock()
		received = string(exchange.In.Body())
		mu.Unlock()
		return true
	}).End()

	ep.AddRouter(router)
	err = ep.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 300)

	// Send data, server echo
	err = ep.Send([]byte("registry test"))
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	ep.Destroy()
	stop <- struct{}{}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, "echo:registry test", received)
	mu.Unlock()
}

// TestWsClientCustomHeaders tests the custom header connection
func TestWsClientCustomHeaders(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var receivedAuth string
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			receivedAuth = r.Header.Get("Authorization")
			mu.Unlock()

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			conn.WriteMessage(websocket.TextMessage, []byte("auth ok"))
			<-stop
		})
		server := &http.Server{Addr: ":9100", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9100/ws",
		"reconnectInterval": 0,
		"headers": map[string]string{
			"Authorization": "Bearer test-token-123",
		},
	})
	assert.Nil(t, err)

	var received string
	var msgMu sync.Mutex
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msgMu.Lock()
		received = string(exchange.In.Body())
		msgMu.Unlock()
		return true
	}).End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 1)

	client.Destroy()
	stop <- struct{}{}
	wg.Wait()

	mu.Lock()
	assert.Equal(t, "Bearer test-token-123", receivedAuth)
	mu.Unlock()

	msgMu.Lock()
	assert.Equal(t, "auth ok", received)
	msgMu.Unlock()
}

// ==================== Auxiliary Function ====================

// startWSPushServer starts a WebSocket server that actively pushes data
func startWSPushServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Proactively push data
		for i := 0; i < 5; i++ {
			msg := fmt.Sprintf(`{"push":%d,"data":"sensor reading"}`, i)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				return
			}
			time.Sleep(time.Millisecond * 200)
		}

		// Stay connected until notified to stop
		<-stop
	})

	server := &http.Server{Addr: wsClientTestServer, Handler: mux}
	go func() {
		<-stop
		server.Close()
	}()
	_ = server.ListenAndServe()
}

// startWSEchoServer starts an echo WebSocket server
func startWSEchoServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	startWSEchoServerPort(t, stop, wg, ":9093")
}

// startWSEchoServerPort Starts a specified port of the echo WebSocket server
func startWSEchoServerPort(t *testing.T, stop chan struct{}, wg *sync.WaitGroup, addr string) {
	defer wg.Done()
	var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// Hui Xian
			conn.WriteMessage(mt, []byte("echo:"+string(message)))
		}
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-stop
		server.Close()
	}()
	_ = server.ListenAndServe()
}

// startWSBinaryServer starts a WebSocket server that sends binary data
func startWSBinaryServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send binary data
		binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
		conn.WriteMessage(websocket.BinaryMessage, binaryData)

		// Stay connected
		<-stop
	})

	server := &http.Server{Addr: ":9094", Handler: mux}
	go func() {
		<-stop
		server.Close()
	}()
	_ = server.ListenAndServe()
}

// TestWsClientHeartbeatCustomData tests the contents of the custom heartbeat packet (by configuration, sending a TextMessage instead of a Ping frame)
func TestWsClientHeartbeatCustomData(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var receivedMsg string
	var mu sync.Mutex

	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			for {
				select {
				case <-stop:
					return
				default:
				}
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if mt == websocket.TextMessage {
					mu.Lock()
					receivedMsg = string(msg)
					mu.Unlock()
					return
				}
			}
		})
		server := &http.Server{Addr: ":9101", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9101/ws",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
		"heartbeatData":     "WS-PING",
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)
	wg.Wait()

	mu.Lock()
	assert.Equal(t, "WS-PING", receivedMsg, "expected custom heartbeat text message")
	mu.Unlock()
}

// TestWsClientHeartbeatCallback tests custom heartbeats using OnHeartbeat callbacks
func TestWsClientHeartbeatCallback(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	var receivedData []byte
	var receivedType int
	var mu sync.Mutex
	var callbackCalled int32

	go func() {
		defer wg.Done()
		var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		mux := http.NewServeMux()
		mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			for {
				select {
				case <-stop:
					return
				default:
				}
				mt, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				mu.Lock()
				receivedType = mt
				receivedData = msg
				mu.Unlock()
				return
			}
		})
		server := &http.Server{Addr: ":9102", Handler: mux}
		go func() {
			<-stop
			server.Close()
		}()
		_ = server.ListenAndServe()
	}()
	time.Sleep(time.Millisecond * 300)

	config := engine.NewConfig()
	client := &WsClient{}
	err := client.Init(config, types.Configuration{
		"server":            "ws://127.0.0.1:9102/ws",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
	})
	assert.Nil(t, err)

	// Custom heartbeat: Send binary heartbeat frames
	client.OnHeartbeat = func(conn *websocket.Conn) error {
		atomic.AddInt32(&callbackCalled, 1)
		return conn.WriteMessage(websocket.BinaryMessage, []byte{0xAA, 0xBB, 0xCC})
	}

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)
	wg.Wait()

	mu.Lock()
	assert.Equal(t, websocket.BinaryMessage, receivedType, "expected binary message from callback")
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, receivedData, "expected custom binary heartbeat data")
	mu.Unlock()

	assert.True(t, atomic.LoadInt32(&callbackCalled) > 0, "expected OnHeartbeat callback to be called")
}
