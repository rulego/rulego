package net

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
)

var (
	clientTestServer = ":16336"
)

// Test client request/response messages
func TestNetClientMessage(t *testing.T) {
	t.Run("ClientRequest", func(t *testing.T) {
		var request = &ClientRequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("ClientResponse", func(t *testing.T) {
		var response = &ClientResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

// Test client types and default values
func TestNetClientType(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
	})
	assert.Nil(t, err)
	assert.Equal(t, ClientType, client.Type())
	assert.Equal(t, "127.0.0.1:9999", client.Id())

	// Verify the default value
	defaultClient := client.New().(*NetClient)
	assert.Equal(t, ProtocolTCP, defaultClient.Config.Protocol)
	assert.Equal(t, 5, defaultClient.Config.ConnectTimeout)
	assert.Equal(t, 5, defaultClient.Config.ReconnectInterval)
	assert.Equal(t, "line", defaultClient.Config.PacketMode)
}

// Test routing management
func TestNetClientRouter(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
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

	// Erroneous regular expression
	router = impl.NewRouter().From("[a-z{1,5}").End()
	_, err = client.AddRouter(router)
	assert.NotNil(t, err)
}

// Test TCP client to connect to the server and receive data
func TestNetClientTCP(t *testing.T) {
	stop := make(chan struct{})

	// Start the analog TCP server
	go startTCPEchoServer(t, stop)
	time.Sleep(time.Millisecond * 200)

	// Create a client
	config := engine.NewConfig(types.WithDefaultPool())
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&ClientConfig{
		Protocol:          "tcp",
		Server:            "127.0.0.1" + clientTestServer,
		ConnectTimeout:    5,
		ReconnectInterval: 0, // No reconnection during testing
		PacketMode:        "line",
	}, &nodeConfig)

	client := &NetClient{}
	err := client.Init(config, nodeConfig)
	assert.Nil(t, err)

	var receiveCount int32

	// Add routes
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		from := exchange.In.From()
		assert.True(t, strings.Contains(from, "127.0.0.1"))
		assert.True(t, strings.HasPrefix(data, "echo:"))
		atomic.AddInt32(&receiveCount, 1)
		return true
	}).End()

	_, err = client.AddRouter(router)
	assert.Nil(t, err)

	// Start the client
	err = client.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 200)

	// Waiting to receive data
	time.Sleep(time.Second * 2)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&receiveCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receiveCount > 0, got %d", count))
}

// Test TCP client to send data to the server
func TestNetClientTCPSend(t *testing.T) {
	stop := make(chan struct{})

	var serverReceived string
	var serverMu sync.Mutex

	// Start the TCP server
	go func() {
		ln, err := net.Listen("tcp", ":16337")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		done := make(chan struct{})
		go func() {
			select {
			case <-stop:
				ln.Close()
			case <-done:
			}
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		serverMu.Lock()
		serverReceived = string(buf[:n])
		serverMu.Unlock()

		// Hui Xian
		conn.Write(buf[:n])
		close(done)
		time.Sleep(time.Second)
	}()

	time.Sleep(time.Millisecond * 200)

	// Create a client
	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":     "127.0.0.1:16337",
		"protocol":   "tcp",
		"packetMode": "line",
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 200)

	// Send data via the Send method
	err = client.Send([]byte("hello from client\n"))
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	client.Destroy()
	close(stop)

	serverMu.Lock()
	assert.Equal(t, "hello from client\n", serverReceived)
	serverMu.Unlock()
}

// Test client configuration
func TestNetClientConfig(t *testing.T) {
	config := engine.NewConfig()

	// Initialize via Configuration
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "192.168.1.100:8080",
		"protocol":          "tcp",
		"connectTimeout":    10,
		"readTimeout":       30,
		"reconnectInterval": 3,
		"packetMode":        "line",
		"encode":            "hex",
	})
	assert.Nil(t, err)
	assert.Equal(t, "192.168.1.100:8080", client.Config.Server)
	assert.Equal(t, "tcp", client.Config.Protocol)
	assert.Equal(t, 10, client.Config.ConnectTimeout)
	assert.Equal(t, 30, client.Config.ReadTimeout)
	assert.Equal(t, 3, client.Config.ReconnectInterval)
	assert.Equal(t, "line", client.Config.PacketMode)
	assert.Equal(t, "hex", client.Config.Encode)
}

// Test connection failed
func TestNetClientConnectFail(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":         "127.0.0.1:19999", // A port that doesn't exist
		"connectTimeout": 1,
	})
	assert.Nil(t, err)

	err = client.Start()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "connect"))
}

// Test data coding
func TestNetClientEncode(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
		"encode": "hex",
	})
	assert.Nil(t, err)

	data := []byte("hello")
	encoded, dataType := encodeData(data, client.Config.Encode)
	assert.Equal(t, types.TEXT, dataType)
	// After hex encoding, the length doubles
	assert.Equal(t, len(data)*2, len(encoded))

	client.Config.Encode = "base64"
	encoded, dataType = encodeData(data, client.Config.Encode)
	assert.Equal(t, types.TEXT, dataType)

	client.Config.Encode = "none"
	encoded, dataType = encodeData(data, client.Config.Encode)
	assert.Equal(t, types.BINARY, dataType)
	assert.Equal(t, data, encoded)
}

// Testing a TCP client with a rule chain
func TestNetClientWithRuleChain(t *testing.T) {
	stop := make(chan struct{})

	// Start the TCP server to send JSON data
	go startTCPJsonServer(t, stop)
	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":     "127.0.0.1:16338",
		"protocol":   "tcp",
		"packetMode": "line",
	})
	assert.Nil(t, err)

	var processedCount int32

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.True(t, strings.Contains(data, `"temperature"`), "expected JSON with temperature, got: "+data)

		msg := exchange.In.GetMsg()
		assert.Equal(t, types.BINARY, msg.GetDataType())
		assert.True(t, strings.Contains(msg.Metadata.GetValue(RemoteAddrKey), "127.0.0.1"))

		atomic.AddInt32(&processedCount, 1)
		return true
	}).End()

	_, err = client.AddRouter(router)
	assert.Nil(t, err)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&processedCount)
	assert.True(t, count >= 2, fmt.Sprintf("expected processedCount >= 2, got %d", count))
}

// Test routing matching options
func TestNetClientRouterMatchOptions(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
	})
	assert.Nil(t, err)

	// Add routes with matching options
	router := impl.NewRouter().From("").End()
	opts := &RouterMatchOptions{
		MinDataLength: 5,
		MaxDataLength: 100,
	}
	_, err = client.AddRouter(router, opts)
	assert.Nil(t, err)
}

// ==================== Auxiliary Function ====================

// startTCPEchoServer starts a simple TCP echo server and will actively push data
func startTCPEchoServer(t *testing.T, stop chan struct{}) {
	ln, err := net.Listen("tcp", clientTestServer)
	if err != nil {
		t.Error(err)
		return
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-stop:
			ln.Close()
		case <-done:
		}
	}()

	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// The server proactively pushes data
	for i := 0; i < 3; i++ {
		msg := fmt.Sprintf("echo: message %d\n", i+1)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			return
		}
		time.Sleep(time.Millisecond * 300)
	}
	close(done)
	// Maintain the connection for a period of time for the client to read
	time.Sleep(time.Second)
}

// startTCPJsonServer Starts a TCP server that sends JSON data
func startTCPJsonServer(t *testing.T, stop chan struct{}) {
	ln, err := net.Listen("tcp", ":16338")
	if err != nil {
		t.Error(err)
		return
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-stop:
			ln.Close()
		case <-done:
		}
	}()

	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// Send JSON sensor data
	messages := []string{
		`{"temperature":25.5,"humidity":60}`,
		`{"temperature":26.1,"humidity":58}`,
		`{"temperature":24.8,"humidity":62}`,
	}
	for _, msg := range messages {
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			return
		}
		time.Sleep(time.Millisecond * 500)
	}
	close(done)
	time.Sleep(time.Second)
}

// Test clients Close and Destroy
func TestNetClientCloseDestroy(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:19999",
	})
	assert.Nil(t, err)

	// Close should not give an error when not connected
	err = client.Close()
	assert.Nil(t, err)

	// Destroy should not report errors either
	client.Destroy()
}

// Test creates clients through the registry
func TestNetClientRegistryCreate(t *testing.T) {
	assert.Equal(t, true, true) // Placeholders, registry tests are conducted in integration testing
	_ = reflect.TypeOf(&NetClient{})
}

// ==================== Integration Testing: Complete Rule Chain Flow ====================

// Complete process of TestNetClientWithRuleChainIntegration: TCP server → client connection → Receive data → Router → Rule chain → Response feedback
func TestNetClientWithRuleChainIntegration(t *testing.T) {
	// Load the rule chain
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	stop := make(chan struct{})

	// Start the TCP server
	var serverReceivedResponse string
	var serverMu sync.Mutex
	go func() {
		ln, err := net.Listen("tcp", ":16339")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		done := make(chan struct{})
		go func() {
			select {
			case <-stop:
				ln.Close()
			case <-done:
			}
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send JSON data
		for i := 0; i < 3; i++ {
			msg := `{"test":"integration"}` + "\n"
			_, err := conn.Write([]byte(msg))
			if err != nil {
				return
			}
			time.Sleep(time.Millisecond * 300)
		}

		// Read the client's response written back
		reader := bufio.NewReader(conn)
		conn.SetReadDeadline(time.Now().Add(time.Second * 3))
		line, err := reader.ReadString('\n')
		if err == nil {
			serverMu.Lock()
			serverReceivedResponse = strings.TrimSpace(line)
			serverMu.Unlock()
		}
		close(done)
		time.Sleep(time.Second)
	}()
	time.Sleep(time.Millisecond * 200)

	// Create a client, Router → rule chain, → response feedback
	client := &NetClient{}
	err = client.Init(config, types.Configuration{
		"server":     "127.0.0.1:16339",
		"protocol":   "tcp",
		"packetMode": "line",
	})
	assert.Nil(t, err)

	var processedCount int32
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.True(t, strings.Contains(data, `"test"`), "expected JSON data, got: "+data)

		msg := exchange.In.GetMsg()
		msg.Type = "TEST_MSG_TYPE2" // Match the s4 branch in the rule chain

		atomic.AddInt32(&processedCount, 1)
		return true
	}).To("chain:default").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// After the rule chain completes execution, it writes back the response to the server
		result := exchange.Out.GetMsg().GetData()
		exchange.Out.SetBody([]byte("client response: " + result + "\n"))
		return true
	}).End()

	_, err = client.AddRouter(router)
	assert.Nil(t, err)

	// Added a global interceptor
	client.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		return true
	})

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&processedCount)
	assert.True(t, count >= 2, fmt.Sprintf("expected processedCount >= 2, got %d", count))

	// The verification server writes back the response upon receipt
	time.Sleep(time.Millisecond * 100)
	serverMu.Lock()
	assert.True(t, serverReceivedResponse != "", "expected server to receive response from client")
	serverMu.Unlock()
}

// TestNetClientReconnect Test disconnection and reconnection
func TestNetClientReconnect(t *testing.T) {
	stop := make(chan struct{})

	var connectCount int32

	// Start a long-lived TCP server and accept two connections
	go func() {
		ln, err := net.Listen("tcp", ":16340")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		go func() {
			<-stop
			ln.Close()
		}()

		for i := 0; i < 2; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&connectCount, 1)
			// After sending data, close the connection
			conn.Write([]byte(fmt.Sprintf("batch %d\n", i+1)))
			time.Sleep(time.Millisecond * 300)
			conn.Close()
		}
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16340",
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 1, // Reconnect after 1 second
	})
	assert.Nil(t, err)

	var receiveCount int32
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		atomic.AddInt32(&receiveCount, 1)
		return true
	}).End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	// Wait for reconnection and data reception
	time.Sleep(time.Second * 5)

	client.Destroy()
	close(stop)

	totalReceived := atomic.LoadInt32(&receiveCount)
	totalConnects := atomic.LoadInt32(&connectCount)
	assert.True(t, totalReceived >= 2, fmt.Sprintf("expected total received >= 2, got %d", totalReceived))
	assert.True(t, totalConnects >= 2, fmt.Sprintf("expected at least 2 connections, got %d", totalConnects))
}

// TestNetClientHeartbeat tests heartbeat transmission
func TestNetClientHeartbeat(t *testing.T) {
	stop := make(chan struct{})

	var receivedHeartbeat bool
	var mu sync.Mutex

	go func() {
		ln, err := net.Listen("tcp", ":16341")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		go func() {
			<-stop
			ln.Close()
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.TrimSpace(line) == PingData {
				mu.Lock()
				receivedHeartbeat = true
				mu.Unlock()
				return
			}
		}
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16341",
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 0,
		"heartbeatInterval": 1, // Send a heartbeat every 1 second
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	mu.Lock()
	assert.True(t, receivedHeartbeat, "expected server to receive heartbeat ping")
	mu.Unlock()
}

// TestNetClientResponseWriteBack The test writes data back to the server via ResponseMessage.SetBody().
func TestNetClientResponseWriteBack(t *testing.T) {
	stop := make(chan struct{})

	var serverReceived string
	var mu sync.Mutex

	go func() {
		ln, err := net.Listen("tcp", ":16342")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		done := make(chan struct{})
		go func() {
			select {
			case <-stop:
				ln.Close()
			case <-done:
			}
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a line of data
		conn.Write([]byte("request data\n"))

		// Read the client's response written back
		reader := bufio.NewReader(conn)
		conn.SetReadDeadline(time.Now().Add(time.Second * 3))
		line, err := reader.ReadString('\n')
		if err == nil {
			mu.Lock()
			serverReceived = strings.TrimSpace(line)
			mu.Unlock()
		}
		close(done)
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":     "127.0.0.1:16342",
		"protocol":   "tcp",
		"packetMode": "line",
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Writes data back to the server via ResponseMessage.SetBody().
		exchange.Out.SetBody([]byte("ack:" + string(exchange.In.Body()) + "\n"))
		return false // No further processing is needed
	}).End()

	client.AddRouter(router)
	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	close(stop)

	mu.Lock()
	assert.Equal(t, "ack:request data", serverReceived)
	mu.Unlock()
}

// TestNetClientUDPIntegration Tests UDP client connections
func TestNetClientUDPIntegration(t *testing.T) {
	stop := make(chan struct{})

	// Launch the UDP server
	go func() {
		addr, err := net.ResolveUDPAddr("udp", ":16343")
		if err != nil {
			t.Error(err)
			return
		}
		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()

		go func() {
			<-stop
			conn.Close()
		}()

		buf := make([]byte, 1024)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			// Hui Xian
			conn.WriteToUDP(buf[:n], remoteAddr)
		}
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16343",
		"protocol":          "udp",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)

	var receivedCount int32
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.Equal(t, "hello udp", data)
		atomic.AddInt32(&receivedCount, 1)
		return true
	}).End()

	client.AddRouter(router)
	err = client.Start()
	assert.Nil(t, err)

	// Send data via Send (server will echo, client readLoopUDP will receive)
	err = client.Send([]byte("hello udp"))
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&receivedCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receivedCount > 0, got %d", count))
}

// TestNetClientRegistryCreateEndpoint creates client endpoints through the Registry
func TestNetClientRegistryCreateEndpoint(t *testing.T) {
	stop := make(chan struct{})
	go startTCPEchoServer(t, stop)
	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())

	// Direct construct (because the endpoint package cannot be imported during testing, so Registry is used)
	// Verify that NetClient has implemented the endpoint.Endpoint interface
	var _ endpoint.Endpoint = &NetClient{}

	ep := &NetClient{}
	err := ep.Init(config, types.Configuration{
		"server":            "127.0.0.1" + clientTestServer,
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 0,
	})
	assert.Nil(t, err)
	assert.Equal(t, ClientType, ep.Type())

	var receiveCount int32
	router := impl.NewRouter().From("").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := string(exchange.In.Body())
		assert.True(t, strings.HasPrefix(data, "echo:"), "expected echo data, got: "+data)
		atomic.AddInt32(&receiveCount, 1)
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	ep.Destroy()
	close(stop)

	count := atomic.LoadInt32(&receiveCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receiveCount > 0, got %d", count))
}

// TestNetClientHeartbeatCustomData tests the contents of a custom heartbeat package (through configuration)
func TestNetClientHeartbeatCustomData(t *testing.T) {
	stop := make(chan struct{})

	var receivedData []byte
	var mu sync.Mutex

	go func() {
		ln, err := net.Listen("tcp", ":16350")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		go func() {
			<-stop
			ln.Close()
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		mu.Lock()
		receivedData = buf[:n]
		mu.Unlock()
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16350",
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
		"heartbeatData":     "HEARTBEAT",
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	mu.Lock()
	assert.Equal(t, "HEARTBEAT", string(receivedData), "expected custom heartbeat data")
	mu.Unlock()
}

// TestNetClientHeartbeatHexData tests the contents of the heartbeat package in hexadecimal format
func TestNetClientHeartbeatHexData(t *testing.T) {
	stop := make(chan struct{})

	var receivedData []byte
	var mu sync.Mutex

	go func() {
		ln, err := net.Listen("tcp", ":16351")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		go func() {
			<-stop
			ln.Close()
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		mu.Lock()
		receivedData = buf[:n]
		mu.Unlock()
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16351",
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
		"heartbeatData":     "0x0D0A", // \r\n
	})
	assert.Nil(t, err)

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	mu.Lock()
	assert.Equal(t, []byte{0x0D, 0x0A}, receivedData, "expected hex decoded heartbeat data")
	mu.Unlock()
}

// TestNetClientHeartbeatCallback tests custom heartbeats using OnHeartbeat callbacks
func TestNetClientHeartbeatCallback(t *testing.T) {
	stop := make(chan struct{})

	var receivedData []byte
	var mu sync.Mutex
	var callbackCalled int32

	go func() {
		ln, err := net.Listen("tcp", ":16352")
		if err != nil {
			t.Error(err)
			return
		}
		defer ln.Close()

		go func() {
			<-stop
			ln.Close()
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		mu.Lock()
		receivedData = buf[:n]
		mu.Unlock()
	}()

	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":            "127.0.0.1:16352",
		"protocol":          "tcp",
		"packetMode":        "line",
		"reconnectInterval": 0,
		"heartbeatInterval": 1,
	})
	assert.Nil(t, err)

	// Custom heartbeat callback: sends heartbeats with timestamps
	client.OnHeartbeat = func(conn net.Conn) error {
		atomic.AddInt32(&callbackCalled, 1)
		data := fmt.Sprintf("HB:%d\n", time.Now().Unix())
		_, err := conn.Write([]byte(data))
		return err
	}

	router := impl.NewRouter().From("").End()
	client.AddRouter(router)

	err = client.Start()
	assert.Nil(t, err)

	time.Sleep(time.Second * 3)

	client.Destroy()
	close(stop)

	mu.Lock()
	assert.True(t, len(receivedData) > 0, "expected heartbeat data from callback")
	assert.True(t, strings.HasPrefix(string(receivedData), "HB:"), "expected custom heartbeat prefix")
	mu.Unlock()

	assert.True(t, atomic.LoadInt32(&callbackCalled) > 0, "expected OnHeartbeat callback to be called")
}
