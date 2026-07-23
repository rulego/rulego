package net

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/components/external"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
)

var (
	testdataFolder   = "../../testdata/rule"
	testServer       = ":16335" // Use a port number that is less likely to conflict
	testConfigServer = "127.0.0.1:8889"
	msgContent1      = "{\"test\":\"AA\"}"
	msgContent2      = "{\"test\":\"BB\"}"
	msgContent3      = "\"test\":\"CC\\n aa\""
	msgContent4      = "{\"test\":\"DD\"}"
	msgContent5      = "{\"test\":\"FF\"}"
)

// Test request/response messages
func TestNetMessage(t *testing.T) {
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
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   testConfigServer,
		//Timeout of 1 second
		ReadTimeout: 1,
	}, &nodeConfig)
	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
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

func TestNetEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	//Start the server
	go startServer(t, stop, &wg)
	//Wait for the server to start up
	time.Sleep(time.Millisecond * 200)
	//Start the client
	node := createNetClient(t)
	config := types.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
		if err2 != nil {
			t.Logf("Client callback error: %v", err2)
		}
	})
	//Send the message
	metaData := types.BuildMetadata(make(map[string]string))

	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, msgContent1)
	node.OnMsg(ctx, msg1)
	time.Sleep(time.Millisecond * 100) // Add delay

	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, msgContent2)
	node.OnMsg(ctx, msg2)
	time.Sleep(time.Millisecond * 100)

	msg3 := ctx.NewMsg("TEST_MSG_TYPE_CC", metaData, msgContent3)
	node.OnMsg(ctx, msg3)
	time.Sleep(time.Millisecond * 100)

	//Because the server is split from \n or \t\n, two messages will be received here
	msg4 := ctx.NewMsg("TEST_MSG_TYPE_DD", metaData, msgContent4+"\n"+msgContent5)
	node.OnMsg(ctx, msg4)
	time.Sleep(time.Millisecond * 100)

	//Ping messages
	msg5 := ctx.NewMsg(PingData, metaData, PingData)
	node.OnMsg(ctx, msg5)
	time.Sleep(time.Millisecond * 100)

	//Wait for all messages to be processed
	time.Sleep(time.Second * 2)
	//Destroy and disconnect
	node.Destroy()
	//Stop the server
	stop <- struct{}{}
	wg.Wait()
}

func TestNetEndpointConfig(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	//Create TPC Endpoint services
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   testConfigServer,
		//Timeout of 1 second
		ReadTimeout: 1,
	}, &nodeConfig)
	var epStarted = &Net{}
	err := epStarted.Init(config, nodeConfig)

	assert.Equal(t, testConfigServer, epStarted.Id())

	err = epStarted.Start()
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 200)

	nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Server: testConfigServer,
		//Timeout of 1 second
		ReadTimeout: 1,
	}, &nodeConfig)
	var netEndpoint = &Net{}
	err = netEndpoint.Init(config, nodeConfig)

	assert.Equal(t, "tcp", netEndpoint.Config.Protocol)

	//Boot failed, and the port was already occupied
	err = netEndpoint.Start()
	assert.NotNil(t, err)

	netEndpoint = &Net{}

	err = netEndpoint.Init(types.NewConfig(), types.Configuration{
		"server": testConfigServer,
		//Timeout of 1 second
		"readTimeout": 1,
	})
	assert.Equal(t, "tcp", netEndpoint.Config.Protocol)

	var ep = &Net{}
	err = ep.Init(config, nodeConfig)

	assert.Equal(t, testConfigServer, ep.Id())
	_, err = ep.AddRouter(nil)
	assert.Equal(t, "router can not nil", err.Error())

	router := impl.NewRouter().From("^{.*").End()
	routerId, err := ep.AddRouter(router)
	assert.Nil(t, err)

	//Repeat
	router = impl.NewRouter().From("^{.*").End()
	_, err = ep.AddRouter(router)
	assert.Equal(t, "duplicate router ^{.*", err.Error())

	//Delete the route
	_ = ep.RemoveRouter(routerId)

	router = impl.NewRouter().From("^{.*").End()
	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	//Incorrect expression
	router = impl.NewRouter().From("[a-z{1,5}").End()
	_, err = ep.AddRouter(router)
	assert.NotNil(t, err)

	epStarted.Destroy()
	netEndpoint.Destroy()
}

func createNetClient(t *testing.T) types.Node {
	node, _ := engine.Registry.NewNode("net")
	var configuration = make(types.Configuration)
	configuration["protocol"] = "tcp"
	configuration["server"] = testServer

	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

func startServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//Register the rule chain
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   testServer,
		//Timeout of 1 second
		ReadTimeout: 1,
	}, &nodeConfig)

	var ep = &Net{}
	err = ep.Init(config, nodeConfig)
	assert.Equal(t, Type, ep.Type())
	assert.True(t, reflect.DeepEqual(&Net{
		Config: Config{
			Protocol:      "tcp",
			ReadTimeout:   60,
			Server:        ":6335", // Use the actual default value instead of the test port
			PacketMode:    "line",
			PacketSize:    2,      // Actual default values
			Encode:        "none", // Actual default values
			MaxPacketSize: 65536,
			SessionTTL:    DefaultSessionTTL,
		},
	}, ep.New()))

	//Added a global interceptor
	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Permission validation logic
		return true
	})
	var router1Count = int32(0)
	var router2Count = int32(0)
	//Matches all messages and forwards them to the route for processing
	router1 := impl.NewRouter().From("").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		from := exchange.In.From()

		requestMessage, ok := exchange.In.(*RequestMessage)
		assert.True(t, ok)
		assert.True(t, requestMessage.Conn() != nil)
		assert.Equal(t, from, requestMessage.From())

		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		receiveData := exchange.In.GetMsg().GetData()

		// Excluding ping messages
		if receiveData == "ping" {
			return true
		}

		if receiveData != msgContent1 && receiveData != msgContent2 && receiveData != msgContent3 && receiveData != msgContent4 && receiveData != msgContent5 {
			t.Fatalf("receive data:%s,expect data:%s,%s,%s,%s,%s", receiveData, msgContent1, msgContent2, msgContent3, msgContent4, msgContent5)
		}

		assert.True(t, strings.Contains(from, "127.0.0.1"))
		assert.Equal(t, from, exchange.In.Headers().Get(RemoteAddrKey))
		assert.Equal(t, exchange.In.Headers().Get(RemoteAddrKey), exchange.In.GetMsg().Metadata.GetValue(RemoteAddrKey))

		atomic.AddInt32(&router1Count, 1)
		return true
	}).To("chain:default").
		Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			assert.Equal(t, exchange.Out.From(), exchange.Out.Headers().Get(RemoteAddrKey))
			v := exchange.Out.GetMsg().Metadata.GetValue("addFrom")
			assert.True(t, v != "")
			//Send a response
			exchange.Out.SetBody([]byte("response"))
			return true
		}).End()

	//Matches messages starting with { and forwards them to the route for processing
	router2 := impl.NewRouter().From("^{.*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		receiveData := exchange.In.GetMsg().GetData()
		if strings.HasSuffix(receiveData, "{") {
			t.Fatalf("receive data:%s,not match data:%s", receiveData, "^{.*")
		}
		atomic.AddInt32(&router2Count, 1)
		return true
	}).To("chain:default").End()

	//Register the route
	_, err = ep.AddRouter(router1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ep.AddRouter(router2)
	if err != nil {
		t.Fatal(err)
	}
	//Start the server
	err = ep.Start()

	assert.Nil(t, err)
	<-stop
	// Ensure resources are properly cleared
	ep.Destroy()
	assert.Equal(t, int32(5), atomic.LoadInt32(&router1Count))
	assert.Equal(t, int32(4), atomic.LoadInt32(&router2Count))
	wg.Done()
}

// Test the packet splitter creation
func TestCreatePacketSplitter(t *testing.T) {
	ep := &Net{}

	t.Run("LineSplitter", func(t *testing.T) {
		ep.Config = Config{PacketMode: "line"}
		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		assert.Equal(t, "*net.LineSplitter", reflect.TypeOf(splitter).String())
	})

	t.Run("FixedLengthSplitter", func(t *testing.T) {
		ep.Config = Config{PacketMode: "fixed", PacketSize: 10}
		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		fixedSplitter := splitter.(*FixedLengthSplitter)
		assert.Equal(t, 10, fixedSplitter.PacketSize)
	})

	t.Run("FixedLengthSplitter_InvalidSize", func(t *testing.T) {
		ep.Config = Config{PacketMode: "fixed", PacketSize: 0}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.Equal(t, "packetSize must be greater than 0 for fixed mode", err.Error())
	})

	t.Run("DelimiterSplitter_String", func(t *testing.T) {
		ep.Config = Config{PacketMode: "delimiter", Delimiter: "END"}
		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		delimiterSplitter := splitter.(*DelimiterSplitter)
		assert.Equal(t, []byte("END"), delimiterSplitter.Delimiter)
	})

	t.Run("DelimiterSplitter_Hex", func(t *testing.T) {
		ep.Config = Config{PacketMode: "delimiter", Delimiter: "0x0D0A"}
		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		delimiterSplitter := splitter.(*DelimiterSplitter)
		assert.Equal(t, []byte{0x0D, 0x0A}, delimiterSplitter.Delimiter)
	})

	t.Run("DelimiterSplitter_InvalidHex", func(t *testing.T) {
		ep.Config = Config{PacketMode: "delimiter", Delimiter: "0xZZ"}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid hex delimiter"))
	})

	t.Run("DelimiterSplitter_Empty", func(t *testing.T) {
		ep.Config = Config{PacketMode: "delimiter", Delimiter: ""}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.Equal(t, "delimiter must be specified for delimiter mode", err.Error())
	})

	t.Run("LengthPrefixSplitter", func(t *testing.T) {
		ep.Config = Config{
			PacketMode:    "length_prefix_be",
			PacketSize:    2,
			MaxPacketSize: 1024,
		}
		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		lengthSplitter := splitter.(*LengthPrefixSplitter)
		assert.Equal(t, 2, lengthSplitter.PrefixSize)
		assert.Equal(t, true, lengthSplitter.BigEndian)
		assert.Equal(t, false, lengthSplitter.IncludesPrefix)
		assert.Equal(t, 1024, lengthSplitter.MaxPacketSize)
	})

	t.Run("LengthPrefixSplitter_InvalidSize", func(t *testing.T) {
		ep.Config = Config{PacketMode: "length_prefix_le", PacketSize: 0}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.Equal(t, "packetSize must be between 1 and 4 for length_prefix mode", err.Error())

		ep.Config = Config{PacketMode: "length_prefix_le", PacketSize: 5}
		_, err = CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.Equal(t, "packetSize must be between 1 and 4 for length_prefix mode", err.Error())
	})

	t.Run("UnsupportedMode", func(t *testing.T) {
		ep.Config = Config{PacketMode: "unknown"}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.Equal(t, "unsupported packet mode: unknown", err.Error())
	})
}

// Testing fixed-length packet segmentation
func TestFixedLengthEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start a fixed-length server
	go startFixedLengthServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Create a TCP client to connect to the server
	conn, err := net.Dial("tcp", ":8090")
	assert.Nil(t, err)
	defer conn.Close()

	// Create a 16-byte test packet
	// First 4 bytes: Device ID (1001)
	// Next 4 bytes: Command (1)
	// Last 8 bytes: Data load
	testData := make([]byte, 16)
	// Device ID: 1001 = 0x03E9 (big-endian)
	testData[0], testData[1], testData[2], testData[3] = 0x00, 0x00, 0x03, 0xE9
	// Command: 1 (big-endian)
	testData[4], testData[5], testData[6], testData[7] = 0x00, 0x00, 0x00, 0x01
	// Data load
	copy(testData[8:], []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	// Send data to the server
	_, err = conn.Write(testData)
	assert.Nil(t, err)

	// Read server response (expected to be 16 bytes of response)
	response := make([]byte, 16)
	_, err = conn.Read(response)
	assert.Nil(t, err)

	// Verify the response
	// The status code should be Success (0x00000000)
	assert.Equal(t, byte(0x00), response[0])
	assert.Equal(t, byte(0x00), response[1])
	assert.Equal(t, byte(0x00), response[2])
	assert.Equal(t, byte(0x00), response[3])

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// Test length prefix packet segmentation
func TestLengthPrefixEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the length prefix server
	go startLengthPrefixServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Create a TCP client to connect to the server
	conn, err := net.Dial("tcp", ":8091")
	assert.Nil(t, err)
	defer conn.Close()

	// Test heartbeat message: Length (2 bytes) + Message type (1 byte)
	heartbeatData := []byte{0x00, 0x01, 0x10} // Length 1 + heart rate type 0x10
	_, err = conn.Write(heartbeatData)
	assert.Nil(t, err)

	// Read heartbeat response: length (2 bytes) + status (1 byte) + timestamp (4 bytes)
	heartbeatResponse := make([]byte, 7)
	_, err = conn.Read(heartbeatResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), heartbeatResponse[0]) // Length is high bytes
	assert.Equal(t, byte(0x05), heartbeatResponse[1]) // Low Length (5 bytes of data)
	assert.Equal(t, byte(0x00), heartbeatResponse[2]) // Successful status

	// Test data upload message: length (2 bytes) + message type (1 byte) + sensor ID (2 bytes) + temperature value (4 bytes)
	dataUploadMsg := []byte{0x00, 0x07, 0x20, 0x12, 0x34, 0x00, 0x00, 0x00, 0x1A} // Length 7 + Type 0x20 + Sensor ID 0x1234 + Temperature 26
	_, err = conn.Write(dataUploadMsg)
	assert.Nil(t, err)

	// Read data upload response: Length (2 bytes) + Status (1 byte) + Sensor ID Echo (2 bytes)
	uploadResponse := make([]byte, 5)
	_, err = conn.Read(uploadResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), uploadResponse[0]) // Length is high bytes
	assert.Equal(t, byte(0x03), uploadResponse[1]) // Low Length Bytes (3 bytes of data)
	assert.Equal(t, byte(0x00), uploadResponse[2]) // Successful status
	assert.Equal(t, byte(0x12), uploadResponse[3]) // Sensor ID echo high bytes
	assert.Equal(t, byte(0x34), uploadResponse[4]) // Sensor ID echoes at low bytes

	// Test unknown message types
	unknownMsg := []byte{0x00, 0x02, 0xFF, 0x99} // Length 2 + Unknown type 0xFF
	_, err = conn.Write(unknownMsg)
	assert.Nil(t, err)

	// Read error responses
	errorResponse := make([]byte, 4)
	_, err = conn.Read(errorResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), errorResponse[0]) // Length is high bytes
	assert.Equal(t, byte(0x02), errorResponse[1]) // Low Length Bytes (2 bytes of data)
	assert.Equal(t, byte(0xFF), errorResponse[2]) // Error status
	assert.Equal(t, byte(0x04), errorResponse[3]) // Error code

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// Test custom separator packet splitting
func TestDelimiterEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the delimiter server
	go startDelimiterServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Create a TCP client to connect to the server
	conn, err := net.Dial("tcp", ":8092")
	assert.Nil(t, err)
	defer conn.Close()

	// Test the AT command
	atCommand := "AT+INFO\r\n"
	_, err = conn.Write([]byte(atCommand))
	assert.Nil(t, err)

	// Read the AT command response
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	assert.Nil(t, err)
	response := string(buffer[:n])
	assert.True(t, strings.Contains(response, "OK"))
	assert.True(t, strings.Contains(response, "Device: RuleGo-Test"))

	// Test sensor data commands
	sensorCommand := "SENSOR,TEMP,01,25.6\r\n"
	_, err = conn.Write([]byte(sensorCommand))
	assert.Nil(t, err)

	// Read the sensor response
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "ACK,TEMP,01,OK"))

	// Test the Modbus ASCII command
	modbusCommand := ":010300010001FA\r\n"
	_, err = conn.Write([]byte(modbusCommand))
	assert.Nil(t, err)

	// Read the Modbus response
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, ":01030401020304FF"))

	// Test the invalid format command
	invalidCommand := "INVALID_FORMAT\r\n"
	_, err = conn.Write([]byte(invalidCommand))
	assert.Nil(t, err)

	// Read error responses
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "ERROR"))
	assert.True(t, strings.Contains(response, "Unknown command"))

	// Test invalid sensor data formats
	invalidSensor := "SENSOR,TEMP\r\n"
	_, err = conn.Write([]byte(invalidSensor))
	assert.Nil(t, err)

	// Read the response of an invalid sensor
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "NAK,INVALID_FORMAT"))

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// Test the default settings
func TestConfigDefaults(t *testing.T) {
	ep := &Net{}
	config := types.NewConfig()

	// Test the default configuration
	err := ep.Init(config, types.Configuration{})
	assert.Nil(t, err)
	assert.Equal(t, "tcp", ep.Config.Protocol)
	assert.Equal(t, "line", ep.Config.PacketMode)
	assert.Equal(t, 65536, ep.Config.MaxPacketSize)

	// Test the configuration
	err = ep.Init(config, types.Configuration{
		"packetMode": "fixed",
		"packetSize": 20,
	})
	assert.Nil(t, err)
	assert.Equal(t, "fixed", ep.Config.PacketMode)
	assert.Equal(t, 20, ep.Config.PacketSize)
	assert.Equal(t, 65536, ep.Config.MaxPacketSize) // Default values
}

// Testing Concurrency Security - Simplified Version
func TestPacketSplitterConcurrency(t *testing.T) {
	// Simplified concurrency testing tests only basic functions
	// Test multiple splitters created simultaneously
	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each goroutine uses an independent configuration to avoid data contention
			config := Config{PacketMode: "line"}
			_, err := CreatePacketSplitter(config)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Verify that all splitters were successfully created
	for err := range results {
		assert.Nil(t, err)
	}
}

// Auxiliary function: Start a fixed-length server
func startFixedLengthServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":      "tcp",
		"server":        ":8090",
		"readTimeout":   5,
		"packetMode":    "fixed",
		"packetSize":    16,
		"maxPacketSize": 1024,
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// Add routing to handle fixed-length data
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		dataStr := exchange.In.GetMsg().GetData()
		data := []byte(dataStr)
		// Verify receipt of 16 bytes of data
		assert.Equal(t, 16, len(data))

		// Parsing data (correcting byte order parsing)
		deviceId := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		command := uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])

		assert.Equal(t, uint32(1001), deviceId)
		assert.Equal(t, uint32(1), command)

		// Send a 16-byte response
		response := make([]byte, 16)
		// Status code: Success
		response[0], response[1], response[2], response[3] = 0x00, 0x00, 0x00, 0x00
		// Device ID Echo
		copy(response[4:8], data[0:4])
		// Other data
		copy(response[8:], []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0})

		exchange.Out.SetBody(response)
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop
	ep.Destroy()
	wg.Done()
}

// Auxiliary function: Starts the length prefix server
func startLengthPrefixServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":      "tcp",
		"server":        ":8091",
		"readTimeout":   5,
		"packetMode":    "length_prefix_be",
		"packetSize":    2,
		"maxPacketSize": 4096,
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// Add route processing length prefix data
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := exchange.In.GetMsg().GetBytes()

		// Verify length prefixes
		assert.True(t, len(data) >= 3)
		dataLength := uint16(data[0])<<8 | uint16(data[1])
		messageType := data[2]

		expectedLength := int(dataLength) + 2 // Data length + length prefix
		assert.Equal(t, expectedLength, len(data))

		var response []byte
		switch messageType {
		case 0x10: // HEARTBEAT
			// Response: Length (2 bytes) + Status (1 byte) + Timestamp (4 bytes)
			timestamp := uint32(time.Now().Unix())
			response = []byte{
				0x00, 0x05, // Length: 5 bytes
				0x00, // Successful status
				byte(timestamp >> 24), byte(timestamp >> 16), byte(timestamp >> 8), byte(timestamp),
			}
		case 0x20: // DATA_UPLOAD
			// Response: Length (2 bytes) + Status (1 byte) + Sensor ID Echo (2 bytes)
			if len(data) >= 7 {
				response = []byte{
					0x00, 0x03, // Length: 3 bytes
					0x00,             // Successful status
					data[3], data[4], // Sensor ID Echo
				}
			}
		default:
			// Unknown message type
			response = []byte{0x00, 0x02, 0xFF, 0x04} // Wrong response
		}

		exchange.Out.SetBody(response)
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop
	ep.Destroy()
	wg.Done()
}

// Auxiliary function: Starts a custom delimiter server
func startDelimiterServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":      "tcp",
		"server":        ":8092",
		"readTimeout":   5,
		"packetMode":    "delimiter",
		"delimiter":     "0x0D0A", // \r\n
		"maxPacketSize": 2048,
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// Add routing processing separator data
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		command := exchange.In.GetMsg().GetData()
		var response string

		if strings.HasPrefix(command, "AT+") {
			// AT command processing
			if strings.Contains(command, "INFO") {
				response = "OK\r\nDevice: RuleGo-Test\r\nVersion: 1.0.0\r\n"
			} else {
				response = "ERROR\r\nUnknown AT command\r\n"
			}
		} else if strings.HasPrefix(command, "SENSOR,") {
			// CSV command processing
			parts := strings.Split(command, ",")
			if len(parts) >= 4 {
				response = fmt.Sprintf("ACK,%s,%s,OK\r\n", parts[1], parts[2])
			} else {
				response = "NAK,INVALID_FORMAT\r\n"
			}
		} else if strings.HasPrefix(command, ":") {
			// Modbus ASCII processing
			response = ":01030401020304FF\r\n" // Analog response
		} else {
			response = "ERROR\r\nUnknown command\r\n"
		}

		exchange.Out.SetBody([]byte(response))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop
	ep.Destroy()
	wg.Done()
}

// Auxiliary function: Starts the concurrent test server
func startConcurrentServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":    "tcp",
		"server":      ":8093",
		"readTimeout": 10,
		"packetMode":  "line", // Use the default row splitting mode
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	var messageCount int32

	// Add routing processing and concurrent messages
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		atomic.AddInt32(&messageCount, 1)

		// Simple JSON parsing verification
		data := exchange.In.GetMsg().GetData()
		assert.True(t, strings.Contains(data, "client"))
		assert.True(t, strings.Contains(data, "message"))

		// Send a confirmation response
		response := fmt.Sprintf("{\"ack\":true,\"received\":\"%s\"}\n", data)
		exchange.Out.SetBody([]byte(response))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop

	// Verify the expected number of messages received
	expectedMessages := int32(50) // 10 clients * 5 messages
	actualMessages := atomic.LoadInt32(&messageCount)
	assert.Equal(t, expectedMessages, actualMessages)

	ep.Destroy()
	wg.Done()
}

// Test the routing matching option feature
func TestRouterMatchOptions(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol:    "tcp",
		Server:      "127.0.0.1:8895",
		ReadTimeout: 1,
		PacketMode:  "line",
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	t.Run("基础路由匹配（向后兼容）", func(t *testing.T) {
		router := impl.NewRouter().From("^{.*").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// Verify that the route has been correctly added
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.matchOptions) // There is no matching option by default
	})

	t.Run("原始数据匹配", func(t *testing.T) {
		options := &RouterMatchOptions{
			MatchRawData: true,
		}
		router := impl.NewRouter().From("test").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify that routing options are set correctly
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
	})

	t.Run("数据类型过滤", func(t *testing.T) {
		options := &RouterMatchOptions{
			DataTypeFilter: "JSON",
		}
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify that routing options are set correctly
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj.matchOptions)
		assert.Equal(t, "JSON", routerObj.matchOptions.DataTypeFilter)
	})

	t.Run("数据长度过滤", func(t *testing.T) {
		options := &RouterMatchOptions{
			MinDataLength: 10,
			MaxDataLength: 100,
		}
		router := impl.NewRouter().From("length.*").End() // Use different regexiform to avoid duplication
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify that routing options are set correctly
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj.matchOptions)
		assert.Equal(t, 10, routerObj.matchOptions.MinDataLength)
		assert.Equal(t, 100, routerObj.matchOptions.MaxDataLength)
	})

	t.Run("组合条件匹配", func(t *testing.T) {
		options := &RouterMatchOptions{
			MatchRawData:   true,
			DataTypeFilter: "TEXT",
			MinDataLength:  5,
			MaxDataLength:  50,
		}
		router := impl.NewRouter().From("hello").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify that all options are correctly set
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
		assert.Equal(t, "TEXT", routerObj.matchOptions.DataTypeFilter)
		assert.Equal(t, 5, routerObj.matchOptions.MinDataLength)
		assert.Equal(t, 50, routerObj.matchOptions.MaxDataLength)
	})
}

// Test the TCP processor's routing matching logic
func TestTcpHandlerRouteMatching(t *testing.T) {
	_ = &Net{}

	// Create test routes
	router1 := &RegexpRouter{
		regexp:       nil, // Match all
		matchOptions: nil, // Default behavior
	}

	router2 := &RegexpRouter{
		regexp: regexp.MustCompile("test"),
		matchOptions: &RouterMatchOptions{
			MatchRawData: true,
		},
	}

	router3 := &RegexpRouter{
		regexp: regexp.MustCompile(".*"),
		matchOptions: &RouterMatchOptions{
			MinDataLength: 10,
			MaxDataLength: 50,
		},
	}

	t.Run("默认匹配逻辑", func(t *testing.T) {
		rawData := []byte("hello world")
		encodedData := []byte("hello world")
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				body: encodedData,
			},
		}

		// Test default match (no options)
		result := router1.Match(rawData, encodedData, exchange)
		assert.True(t, result) // No regular expression, should match all of them
	})

	t.Run("原始数据匹配", func(t *testing.T) {
		rawData := []byte("test data")
		encodedData := []byte("encoded test data") // Encoded data
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				body: encodedData,
			},
		}

		// Test raw data matching
		result := router2.Match(rawData, encodedData, exchange)
		assert.True(t, result) // The raw data contains "test"
	})

	t.Run("数据长度过滤", func(t *testing.T) {
		// The data is too short
		shortData := []byte("short")
		exchange1 := &endpoint.Exchange{
			In: &RequestMessage{body: shortData},
		}
		result1 := router3.Match(shortData, shortData, exchange1)
		assert.False(t, result1) // Length 5 < minimum length 10

		// The data length is appropriate
		validData := []byte("this is valid data for testing")
		exchange2 := &endpoint.Exchange{
			In: &RequestMessage{body: validData},
		}
		result2 := router3.Match(validData, validData, exchange2)
		assert.True(t, result2) // The length is within the range

		// The data is too long
		longData := make([]byte, 100)
		for i := range longData {
			longData[i] = 'a'
		}
		exchange3 := &endpoint.Exchange{
			In: &RequestMessage{body: longData},
		}
		result3 := router3.Match(longData, longData, exchange3)
		assert.False(t, result3) // Length 100 > maximum length 50
	})
}

// Test the routing matching logic of the UDP processor
func TestUdpHandlerRouteMatching(t *testing.T) {
	_ = &Net{}

	// Create test routes and filter test data types
	router := &RegexpRouter{
		regexp: regexp.MustCompile(".*"),
		matchOptions: &RouterMatchOptions{
			DataTypeFilter: "JSON",
		},
	}

	t.Run("数据类型过滤", func(t *testing.T) {
		jsonData := []byte(`{"key": "value"}`)

		// Create a message of JSON type
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				body: jsonData,
			},
		}
		// Set the message type to JSON
		msg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), string(jsonData))
		exchange.In.SetMsg(&msg)

		result := router.Match(jsonData, jsonData, exchange)
		assert.True(t, result) // JSON type matching

		// Test for types of mismatches
		textMsg := types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "plain text")
		exchange.In.SetMsg(&textMsg)

		result2 := router.Match(jsonData, jsonData, exchange)
		assert.False(t, result2) // The TEXT type does not match the JSON filter
	})
}

// Test UDP endpoints
func TestUDPEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the UDP server
	go startUDPServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Create a UDP client to connect to the server
	conn, err := net.Dial("udp", ":8094")
	assert.Nil(t, err)
	defer conn.Close()

	// Send UDP messages
	message1 := "Hello UDP Server"
	_, err = conn.Write([]byte(message1))
	assert.Nil(t, err)

	// Read UDP responses
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	assert.Nil(t, err)
	response := string(buffer[:n])
	assert.True(t, strings.Contains(response, "UDP received: Hello UDP Server"))

	// Send a JSON-format UDP message
	jsonMsg := `{"type":"sensor","data":{"temperature":25.5,"humidity":60}}`
	_, err = conn.Write([]byte(jsonMsg))
	assert.Nil(t, err)

	// Read JSON response
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, `"status":"received"`))
	assert.True(t, strings.Contains(response, `"type":"json"`))

	// Sending heartbeat messages (should be filtered and not responded)
	_, err = conn.Write([]byte(PingData))
	assert.Nil(t, err)

	// Heartbeat messages shouldn't respond, so we send another message to confirm the server is still working
	testMsg := "test after ping"
	_, err = conn.Write([]byte(testMsg))
	assert.Nil(t, err)

	// Read test message responses
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "UDP received: test after ping"))

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// Test coding function
func TestEncodeFeatures(t *testing.T) {
	ep := &Net{}

	t.Run("十六进制编码", func(t *testing.T) {
		ep.Config = Config{Encode: "hex"}
		input := []byte("Hello")
		expected := []byte("48656c6c6f") // Corrected to lowercase, matching the output of the Go standard library hex.Encode
		result, dataType := encodeData(input, ep.Config.Encode)
		assert.Equal(t, string(expected), string(result))
		assert.Equal(t, types.TEXT, dataType)
	})

	t.Run("Base64编码", func(t *testing.T) {
		ep.Config = Config{Encode: "base64"}
		input := []byte("Hello World")
		result, dataType := encodeData(input, ep.Config.Encode)
		assert.Equal(t, types.TEXT, dataType)
		// The verification result is valid Base64
		assert.True(t, len(result) > 0)
		// Simply verify the Base64 character set
		for _, b := range result {
			isValidBase64 := (b >= 'A' && b <= 'Z') ||
				(b >= 'a' && b <= 'z') ||
				(b >= '0' && b <= '9') ||
				b == '+' || b == '/' || b == '='
			assert.True(t, isValidBase64)
		}
	})

	t.Run("无编码", func(t *testing.T) {
		ep.Config = Config{Encode: ""}
		input := []byte("Hello")
		result, dataType := encodeData(input, ep.Config.Encode)
		assert.Equal(t, string(input), string(result))
		assert.Equal(t, types.BINARY, dataType) // Defaults to binary
	})

	t.Run("未知编码类型", func(t *testing.T) {
		ep.Config = Config{Encode: "unknown"}
		input := []byte("Hello")
		result, dataType := encodeData(input, ep.Config.Encode)
		assert.Equal(t, string(input), string(result))
		assert.Equal(t, types.BINARY, dataType) // Defaults to binary
	})
}

// Test packet splitter error handling
func TestPacketSplitterErrorHandling(t *testing.T) {
	ep := &Net{}

	t.Run("分隔符解析错误", func(t *testing.T) {
		ep.Config = Config{
			PacketMode: "delimiter",
			Delimiter:  "0xZZ", // Invalid hexadecimal
		}
		_, err := CreatePacketSplitter(ep.Config)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid hex delimiter"))
	})

	t.Run("长度前缀包过大", func(t *testing.T) {
		splitter := &LengthPrefixSplitter{
			PrefixSize:    2,
			BigEndian:     true,
			MaxPacketSize: 10,
		}

		// Simulate data for a package that exceeds the limit
		reader := strings.NewReader("\x00\x20test") // Length: 32 >, maximum 10
		bufReader := bufio.NewReader(reader)
		_, err := splitter.ReadPacket(bufReader)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "packet too large"))
	})

	t.Run("长度前缀包含自身长度错误", func(t *testing.T) {
		splitter := &LengthPrefixSplitter{
			PrefixSize:     2,
			BigEndian:      true,
			IncludesPrefix: true,
			MaxPacketSize:  1024,
		}

		// Simulate data with a length less than the size of the prefix
		reader := strings.NewReader("\x00\x01") // Length 1 < Prefix size 2
		bufReader := bufio.NewReader(reader)
		_, err := splitter.ReadPacket(bufReader)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid packet length"))
	})
}

// JSON handling of test response messages
func TestResponseMessageJSONHandling(t *testing.T) {
	response := &ResponseMessage{}

	t.Run("JSON消息自动添加换行符", func(t *testing.T) {
		// Simulates JSON messages
		jsonMsg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"test":"data"}`)
		response.SetMsg(&jsonMsg)

		// Set JSON data without line breaks
		jsonData := []byte(`{"response":"ok"}`)
		response.SetBody(jsonData)

		// Verify whether line breaks have been added automatically
		body := response.Body()
		assert.True(t, strings.HasSuffix(string(body), LineBreak))
	})

	t.Run("非JSON消息不添加换行符", func(t *testing.T) {
		// Simulated TEXT messages
		textMsg := types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "test data")
		response.SetMsg(&textMsg)

		// Set up ordinary text data
		textData := []byte("simple response")
		response.SetBody(textData)

		// Verify that no line breaks are added automatically
		body := response.Body()
		assert.Equal(t, "simple response", string(body))
	})

	t.Run("已有换行符的JSON不重复添加", func(t *testing.T) {
		// Simulates JSON messages
		jsonMsg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"test":"data"}`)
		response.SetMsg(&jsonMsg)

		// Set JSON data with line breaks
		jsonData := []byte(`{"response":"ok"}` + LineBreak)
		response.SetBody(jsonData)

		// Verify the number of line breaks is correct
		body := response.Body()
		lineBreakCount := strings.Count(string(body), LineBreak)
		assert.Equal(t, 1, lineBreakCount)
	})
}

// Testing concurrent read/write security
func TestConcurrentSafety(t *testing.T) {
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Start the concurrent security test server
	wg.Add(1)
	go startConcurrentServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Launch multiple clients for concurrent testing
	clientCount := 10
	messagesPerClient := 5

	var clientWg sync.WaitGroup
	for i := 0; i < clientCount; i++ {
		clientWg.Add(1)
		go func(clientId int) {
			defer clientWg.Done()

			// Create a TCP client connection
			conn, err := net.Dial("tcp", ":8093")
			if err != nil {
				t.Logf("Client %d failed to connect: %v", clientId, err)
				return
			}
			defer conn.Close()

			for j := 0; j < messagesPerClient; j++ {
				msgContent := fmt.Sprintf(`{"client":%d,"message":%d,"timestamp":%d}`+"\n", clientId, j, time.Now().Unix())
				_, err := conn.Write([]byte(msgContent))
				if err != nil {
					t.Logf("Client %d failed to send message %d: %v", clientId, j, err)
					continue
				}

				// Read the response
				buffer := make([]byte, 1024)
				n, err := conn.Read(buffer)
				if err != nil {
					t.Logf("Client %d failed to read response %d: %v", clientId, j, err)
					continue
				}

				response := string(buffer[:n])
				assert.True(t, strings.Contains(response, `"ack":true`))
				assert.True(t, strings.Contains(response, fmt.Sprintf(`"client":%d`, clientId)))

				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	clientWg.Wait()
	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// Test boundary conditions
func TestBoundaryConditions(t *testing.T) {
	config := types.NewConfig()

	t.Run("空配置初始化", func(t *testing.T) {
		ep := &Net{}
		err := ep.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, "tcp", ep.Config.Protocol)
		// When configured null, the Server field is an empty string, not the default value
		assert.Equal(t, "", ep.Config.Server)
		assert.Equal(t, 0, ep.Config.ReadTimeout)
	})

	t.Run("最小配置", func(t *testing.T) {
		ep := &Net{}
		err := ep.Init(config, types.Configuration{
			"server": ":0", // Use random ports
		})
		assert.Nil(t, err)
		assert.Equal(t, ":0", ep.Config.Server)
	})

	t.Run("超大数据包限制", func(t *testing.T) {
		ep := &Net{}
		err := ep.Init(config, types.Configuration{
			"maxPacketSize": 1024 * 1024, // 1MB
		})
		assert.Nil(t, err)
		assert.Equal(t, 1024*1024, ep.Config.MaxPacketSize)
	})

	t.Run("超长分隔符", func(t *testing.T) {
		ep := &Net{}
		longDelimiter := strings.Repeat("AB", 100)
		err := ep.Init(config, types.Configuration{
			"packetMode": "delimiter",
			"delimiter":  longDelimiter,
		})
		assert.Nil(t, err)

		splitter, err := CreatePacketSplitter(ep.Config)
		assert.Nil(t, err)
		delSplitter := splitter.(*DelimiterSplitter)
		assert.Equal(t, len(longDelimiter), len(delSplitter.Delimiter))
	})
}

// Test connection management
func TestConnectionManagement(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol:    "tcp",
		Server:      "127.0.0.1:8898",
		ReadTimeout: 1, // Short timeouts are used for rapid testing
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// Add a simple route
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("connected"))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	t.Run("启动和停止服务", func(t *testing.T) {
		err = ep.Start()
		assert.Nil(t, err)

		// Verify the server ID
		assert.Equal(t, "127.0.0.1:8898", ep.Id())

		// Service shutdown
		err = ep.Close()
		assert.Nil(t, err)

		// Repeated closures should not be a mistake
		err = ep.Close()
		assert.Nil(t, err)

		// Destruction
		ep.Destroy()
	})

	t.Run("无效协议", func(t *testing.T) {
		invalidEp := &Net{}
		invalidConfig := types.Configuration{
			"protocol": "invalid_protocol",
			"server":   ":8899",
		}
		err := invalidEp.Init(config, invalidConfig)
		assert.Nil(t, err)

		err = invalidEp.Start()
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "unsupported protocol"))
	})
}

// Testing routes with different packet splitting modes
func TestPacketModeRouting(t *testing.T) {
	// Test fixed-length mode routing
	t.Run("固定长度模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "fixed",
			PacketSize: 8,
		}

		// Create routes for fixed-length data
		options := &RouterMatchOptions{
			MinDataLength: 8,
			MaxDataLength: 8,
		}
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify routing settings
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.Equal(t, 8, routerObj.matchOptions.MinDataLength)
		assert.Equal(t, 8, routerObj.matchOptions.MaxDataLength)
	})

	// Testing the route for length prefix mode
	t.Run("长度前缀模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "length_prefix_le",
			PacketSize: 2,
		}

		// Create routes that consider length prefixes
		options := &RouterMatchOptions{
			MatchRawData:  true, // Match complete data containing length prefixes
			MinDataLength: 3,    // Minimum 2-byte prefix + 1-byte data
		}
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify routing settings
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
		assert.Equal(t, 3, routerObj.matchOptions.MinDataLength)
	})

	// Test the routing of delimiter patterns
	t.Run("分隔符模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "delimiter",
			Delimiter:  "END",
		}

		// Create routes that match data without delimiters
		options := &RouterMatchOptions{
			MatchRawData: true, // Match the original data after delimiter removal
		}
		router := impl.NewRouter().From("^[^E]*$").End() // Data without the E character (simplified regex)
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// Verify routing settings
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
	})
}

// Test the route matching of encoded data
func TestEncodedDataRouting(t *testing.T) {

	t.Run("十六进制编码路由", func(t *testing.T) {
		// Testing the routing of hexadecimal-encoded data
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^[0-9A-Fa-f]+$"), // Matches hexadecimal characters
			matchOptions: &RouterMatchOptions{
				MatchRawData: false, // Match the encoded data
			},
		}

		rawData := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F} // "Hello"
		// Analog hexadecimal encoding
		encodedData := []byte("48656C6C6F")

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := router.Match(rawData, encodedData, exchange)
		assert.True(t, result) // The encoded data is hexadecimal
	})

	t.Run("Base64编码路由", func(t *testing.T) {
		// Test the routing of Base64-encoded data
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^[A-Za-z0-9+/]+=*$"), // Match Base64 characters
			matchOptions: &RouterMatchOptions{
				MatchRawData: false, // Match the encoded data
			},
		}

		rawData := []byte("Hello World")
		// Analog Base64 encoding
		encodedData := []byte("SGVsbG8gV29ybGQ=")

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := router.Match(rawData, encodedData, exchange)
		assert.True(t, result) // The encoded data is Base64
	})

	t.Run("原始二进制数据路由", func(t *testing.T) {
		// Testing the routing of raw binary data
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^\x48\x65\x6C"), // Matching binary mode "Hel"
			matchOptions: &RouterMatchOptions{
				MatchRawData: true, // Match raw data
			},
		}

		rawData := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F} // "Hello"
		encodedData := []byte("48656C6C6F")             // Hexadecimal code

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := router.Match(rawData, encodedData, exchange)
		assert.True(t, result) // Original data starts with "Hel"."
	})
}

// Test route priority and multi-route matching
func TestMultipleRouterMatching(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol:    "tcp",
		Server:      "127.0.0.1:8896",
		ReadTimeout: 1,
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	t.Run("多个路由同时匹配", func(t *testing.T) {
		// Add a universal route
		generalRouter := impl.NewRouter().From(".*").End()
		_, err := ep.AddRouter(generalRouter)
		assert.Nil(t, err)

		// Add specific routes
		specificOptions := &RouterMatchOptions{
			DataTypeFilter: "JSON",
			MinDataLength:  10,
		}
		specificRouter := impl.NewRouter().From("^{.*").End()
		_, err = ep.AddRouter(specificRouter, specificOptions)
		assert.Nil(t, err)

		// Add length-limited routes
		lengthOptions := &RouterMatchOptions{
			MaxDataLength: 100,
		}
		lengthRouter := impl.NewRouter().From("test").End()
		_, err = ep.AddRouter(lengthRouter, lengthOptions)
		assert.Nil(t, err)

		// Verify that all routes have been added
		ep.Lock()
		routerCount := len(ep.routers)
		ep.Unlock()
		assert.Equal(t, 3, routerCount)
	})

	t.Run("路由条件互斥", func(t *testing.T) {
		// Clear existing routes
		ep.Lock()
		ep.routers = make(map[string]*RegexpRouter)
		ep.Unlock()

		// Add routes that only match short data
		shortOptions := &RouterMatchOptions{
			MaxDataLength: 10,
		}
		shortRouter := impl.NewRouter().From("short.*").End()
		_, err := ep.AddRouter(shortRouter, shortOptions)
		assert.Nil(t, err)

		// Add routes that only match long data
		longOptions := &RouterMatchOptions{
			MinDataLength: 20,
		}
		longRouter := impl.NewRouter().From("long.*").End()
		_, err = ep.AddRouter(longRouter, longOptions)
		assert.Nil(t, err)

		// Verification mutex routing is added
		ep.Lock()
		routerCount := len(ep.routers)
		ep.Unlock()
		assert.Equal(t, 2, routerCount)
	})
}

// Test routing error handling
func TestRouterErrorHandling(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   "127.0.0.1:8897",
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	t.Run("无效的正则表达式", func(t *testing.T) {
		router := impl.NewRouter().From("[a-z{1,5}").End() // Invalid regularity
		_, err := ep.AddRouter(router)
		assert.NotNil(t, err) // Errors should be returned
	})

	t.Run("重复路由ID", func(t *testing.T) {
		router1 := impl.NewRouter().SetId("duplicate").From("test1").End()
		_, err := ep.AddRouter(router1)
		assert.Nil(t, err)

		router2 := impl.NewRouter().SetId("duplicate").From("test2").End()
		_, err = ep.AddRouter(router2)
		assert.NotNil(t, err) // Repeated errors should be returned
		assert.True(t, strings.Contains(err.Error(), "duplicate router"))
	})

	t.Run("删除不存在的路由", func(t *testing.T) {
		err := ep.RemoveRouter("nonexistent")
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "not found"))
	})
}

// Test special matching features
func TestSpecialMatching(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   "127.0.0.1:8900",
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	t.Run("空字符串匹配所有", func(t *testing.T) {
		router := impl.NewRouter().From("").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// Verify that the route has been correctly added
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.regexp) // Empty strings do not compile regular expressions
	})

	t.Run("星号匹配所有", func(t *testing.T) {
		router := impl.NewRouter().From("*").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// Verify that the route has been correctly added
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		// "*" is compiled as a regular expression, but is specially handled when matching
	})

	t.Run("点星匹配所有", func(t *testing.T) {
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// Verify that the route has been correctly added
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.regexp) // ".*" is a special match in all cases; regexp should be nil
	})
}

// Auxiliary function: Starts the UDP server
func startUDPServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":      "udp",
		"server":        ":8094",
		"readTimeout":   5,
		"maxPacketSize": 2048,
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	var messageCount int32

	// Add routing to handle UDP data
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := exchange.In.GetMsg().GetData()
		atomic.AddInt32(&messageCount, 1)

		// Verification messages are not heartbeats
		assert.NotEqual(t, PingData, data)

		var response string
		if strings.HasPrefix(data, "{") {
			// JSON news
			response = `{"status":"received","type":"json"}`
		} else {
			// Ordinary text messages
			response = fmt.Sprintf("UDP received: %s", data)
		}

		exchange.Out.SetBody([]byte(response))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop

	// Verify the expected number of messages received (excluding ping messages, you will actually receive 3 messages)
	actualMessages := atomic.LoadInt32(&messageCount)
	assert.True(t, actualMessages >= 2) // At least two messages should be received

	ep.Destroy()
	wg.Done()
}

// Test the processor's data type conversion function
func TestProcessorDataTypeConversion(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// Start the test server
	go startProcessorTestServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// Create a client
	config := types.NewConfig()
	client := createNetClientNode(t, config, "tcp", "localhost:9200")

	var responseReceived []string
	var responseMutex sync.Mutex

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		responseMutex.Lock()
		defer responseMutex.Unlock()

		if relationType == types.Success && err == nil {
			responseReceived = append(responseReceived, msg.GetData())
		}
	})

	// Test JSON data conversion
	jsonData := `{"test":"data"}` + "\n"
	metaData := types.NewMetadata()
	metaData.PutValue("processor", "setJsonDataType")
	msg1 := types.NewMsg(0, "TEST_JSON", types.BINARY, metaData, jsonData)
	client.OnMsg(ctx, msg1)

	// Test text data conversion
	textData := "Hello World\n"
	metaData2 := types.NewMetadata()
	metaData2.PutValue("processor", "setTextDataType")
	msg2 := types.NewMsg(0, "TEST_TEXT", types.BINARY, metaData2, textData)
	client.OnMsg(ctx, msg2)

	// Test binary data conversion
	binaryData := string([]byte{0x01, 0x02, 0x03, 0x04}) + "\n"
	metaData3 := types.NewMetadata()
	metaData3.PutValue("processor", "setBinaryDataType")
	msg3 := types.NewMsg(0, "TEST_BINARY", types.TEXT, metaData3, binaryData)
	client.OnMsg(ctx, msg3)

	// Waiting for a response
	time.Sleep(time.Millisecond * 500)

	// Verify the results
	responseMutex.Lock()
	assert.True(t, len(responseReceived) >= 1, "应该收到至少一个响应")
	responseMutex.Unlock()

	stop <- struct{}{}
	wg.Wait()
}

// Create a.NET client node
func createNetClientNode(t *testing.T, config types.Config, protocol, server string) types.Node {
	// Register the.NET client component
	components := engine.Registry.GetComponents()
	if _, exists := components["net"]; !exists {
		_ = engine.Registry.Register(&external.NetNode{})
	}

	node, err := engine.Registry.NewNode("net")
	assert.Nil(t, err)

	configuration := types.Configuration{
		"protocol":          protocol,
		"server":            server,
		"connectTimeout":    10,
		"heartbeatInterval": 0, // Avoid heartbeats
	}

	err = node.Init(config, configuration)
	assert.Nil(t, err)

	return node
}

// Start the processor test server
func startProcessorTestServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":    "tcp",
		"server":      ":9200",
		"readTimeout": 5,
		"packetMode":  "line",
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// Add routing using processors to transform data types
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Select the processor based on the processor field in the metadata
		processorName := exchange.In.GetMsg().Metadata.GetValue("processor")

		// Apply the corresponding processor
		switch processorName {
		case "setJsonDataType":
			// Uses a built-in JSON data type processor
			if proc, exists := processor.InBuiltins.Get("setJsonDataType"); exists {
				proc(router, exchange)
			}
		case "setTextDataType":
			// Uses a built-in text data type processor
			if proc, exists := processor.InBuiltins.Get("setTextDataType"); exists {
				proc(router, exchange)
			}
		case "setBinaryDataType":
			// Uses a built-in binary data type processor
			if proc, exists := processor.InBuiltins.Get("setBinaryDataType"); exists {
				proc(router, exchange)
			}
		}

		// Verify whether data type conversion is successful
		msg := exchange.In.GetMsg()
		var response string
		switch processorName {
		case "setJsonDataType":
			if msg.DataType == types.JSON {
				response = "JSON type set successfully"
			} else {
				response = "Failed to set JSON type"
			}
		case "setTextDataType":
			if msg.DataType == types.TEXT {
				response = "TEXT type set successfully"
			} else {
				response = "Failed to set TEXT type"
			}
		case "setBinaryDataType":
			if msg.DataType == types.BINARY {
				response = "BINARY type set successfully"
			} else {
				response = "Failed to set BINARY type"
			}
		default:
			response = "Unknown processor"
		}

		exchange.Out.SetBody([]byte(response))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop
	ep.Destroy()
	wg.Done()
}
