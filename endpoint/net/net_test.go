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
	testServer       = ":16335" // 使用一个不太可能冲突的端口号
	testConfigServer = "127.0.0.1:8889"
	msgContent1      = "{\"test\":\"AA\"}"
	msgContent2      = "{\"test\":\"BB\"}"
	msgContent3      = "\"test\":\"CC\\n aa\""
	msgContent4      = "{\"test\":\"DD\"}"
	msgContent5      = "{\"test\":\"FF\"}"
)

// 测试请求/响应消息
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
		//1秒超时
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
	//启动服务
	go startServer(t, stop, &wg)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)
	//启动客户端
	node := createNetClient(t)
	config := types.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
		if err2 != nil {
			t.Logf("Client callback error: %v", err2)
		}
	})
	//发送消息
	metaData := types.BuildMetadata(make(map[string]string))

	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, msgContent1)
	node.OnMsg(ctx, msg1)
	time.Sleep(time.Millisecond * 100) // 添加延迟

	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, msgContent2)
	node.OnMsg(ctx, msg2)
	time.Sleep(time.Millisecond * 100)

	msg3 := ctx.NewMsg("TEST_MSG_TYPE_CC", metaData, msgContent3)
	node.OnMsg(ctx, msg3)
	time.Sleep(time.Millisecond * 100)

	//因为服务器与\n或者\t\n分割，这里会收到2条消息
	msg4 := ctx.NewMsg("TEST_MSG_TYPE_DD", metaData, msgContent4+"\n"+msgContent5)
	node.OnMsg(ctx, msg4)
	time.Sleep(time.Millisecond * 100)

	//ping消息
	msg5 := ctx.NewMsg(PingData, metaData, PingData)
	node.OnMsg(ctx, msg5)
	time.Sleep(time.Millisecond * 100)

	//等待所有消息处理完毕
	time.Sleep(time.Second * 2)
	//销毁并 断开连接
	node.Destroy()
	//停止服务器
	stop <- struct{}{}
	wg.Wait()
}

func TestNetEndpointConfig(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	//创建tpc endpoint服务
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   testConfigServer,
		//1秒超时
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
		//1秒超时
		ReadTimeout: 1,
	}, &nodeConfig)
	var netEndpoint = &Net{}
	err = netEndpoint.Init(config, nodeConfig)

	assert.Equal(t, "tcp", netEndpoint.Config.Protocol)

	//启动失败，端口已经占用
	err = netEndpoint.Start()
	assert.NotNil(t, err)

	netEndpoint = &Net{}

	err = netEndpoint.Init(types.NewConfig(), types.Configuration{
		"server": testConfigServer,
		//1秒超时
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

	//重复
	router = impl.NewRouter().From("^{.*").End()
	_, err = ep.AddRouter(router)
	assert.Equal(t, "duplicate router ^{.*", err.Error())

	//删除路由
	_ = ep.RemoveRouter(routerId)

	router = impl.NewRouter().From("^{.*").End()
	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	//错误的表达式
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
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol: "tcp",
		Server:   testServer,
		//1秒超时
		ReadTimeout: 1,
	}, &nodeConfig)

	var ep = &Net{}
	err = ep.Init(config, nodeConfig)
	assert.Equal(t, Type, ep.Type())
	assert.True(t, reflect.DeepEqual(&Net{
		Config: Config{
			Protocol:      "tcp",
			ReadTimeout:   60,
			Server:        ":6335", // 使用实际的默认值，而不是测试端口
			PacketMode:    "line",
			PacketSize:    2,      // 实际默认值
			Encode:        "none", // 实际默认值
			MaxPacketSize: 65536,
		},
	}, ep.New()))

	//添加全局拦截器
	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	var router1Count = int32(0)
	var router2Count = int32(0)
	//匹配所有消息，转发到该路由处理
	router1 := impl.NewRouter().From("").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		from := exchange.In.From()

		requestMessage, ok := exchange.In.(*RequestMessage)
		assert.True(t, ok)
		assert.True(t, requestMessage.Conn() != nil)
		assert.Equal(t, from, requestMessage.From())

		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		receiveData := exchange.In.GetMsg().GetData()

		// 排除ping消息
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
			//发送响应
			exchange.Out.SetBody([]byte("response"))
			return true
		}).End()

	//匹配与{开头的消息，转发到该路由处理
	router2 := impl.NewRouter().From("^{.*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = "TEST_MSG_TYPE2"
		receiveData := exchange.In.GetMsg().GetData()
		if strings.HasSuffix(receiveData, "{") {
			t.Fatalf("receive data:%s,not match data:%s", receiveData, "^{.*")
		}
		atomic.AddInt32(&router2Count, 1)
		return true
	}).To("chain:default").End()

	//注册路由
	_, err = ep.AddRouter(router1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ep.AddRouter(router2)
	if err != nil {
		t.Fatal(err)
	}
	//启动服务
	err = ep.Start()

	assert.Nil(t, err)
	<-stop
	// 确保资源被正确清理
	ep.Destroy()
	assert.Equal(t, int32(5), atomic.LoadInt32(&router1Count))
	assert.Equal(t, int32(4), atomic.LoadInt32(&router2Count))
	wg.Done()
}

// 测试数据包分割器创建
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

// 测试固定长度数据包分割
func TestFixedLengthEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// 启动固定长度服务器
	go startFixedLengthServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 创建TCP客户端连接到服务器
	conn, err := net.Dial("tcp", ":8090")
	assert.Nil(t, err)
	defer conn.Close()

	// 创建16字节的测试数据包
	// 前4字节：设备ID (1001)
	// 接下来4字节：命令 (1)
	// 后8字节：数据负载
	testData := make([]byte, 16)
	// 设备ID: 1001 = 0x03E9 (big-endian)
	testData[0], testData[1], testData[2], testData[3] = 0x00, 0x00, 0x03, 0xE9
	// 命令: 1 (big-endian)
	testData[4], testData[5], testData[6], testData[7] = 0x00, 0x00, 0x00, 0x01
	// 数据负载
	copy(testData[8:], []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})

	// 发送数据到服务器
	_, err = conn.Write(testData)
	assert.Nil(t, err)

	// 读取服务器响应（期望16字节响应）
	response := make([]byte, 16)
	_, err = conn.Read(response)
	assert.Nil(t, err)

	// 验证响应
	// 状态码应该是成功 (0x00000000)
	assert.Equal(t, byte(0x00), response[0])
	assert.Equal(t, byte(0x00), response[1])
	assert.Equal(t, byte(0x00), response[2])
	assert.Equal(t, byte(0x00), response[3])

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// 测试长度前缀数据包分割
func TestLengthPrefixEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// 启动长度前缀服务器
	go startLengthPrefixServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 创建TCP客户端连接到服务器
	conn, err := net.Dial("tcp", ":8091")
	assert.Nil(t, err)
	defer conn.Close()

	// 测试心跳消息: 长度(2字节) + 消息类型(1字节)
	heartbeatData := []byte{0x00, 0x01, 0x10} // 长度1 + 心跳类型0x10
	_, err = conn.Write(heartbeatData)
	assert.Nil(t, err)

	// 读取心跳响应: 长度(2字节) + 状态(1字节) + 时间戳(4字节)
	heartbeatResponse := make([]byte, 7)
	_, err = conn.Read(heartbeatResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), heartbeatResponse[0]) // 长度高字节
	assert.Equal(t, byte(0x05), heartbeatResponse[1]) // 长度低字节 (5字节数据)
	assert.Equal(t, byte(0x00), heartbeatResponse[2]) // 成功状态

	// 测试数据上传消息: 长度(2字节) + 消息类型(1字节) + 传感器ID(2字节) + 温度值(4字节)
	dataUploadMsg := []byte{0x00, 0x07, 0x20, 0x12, 0x34, 0x00, 0x00, 0x00, 0x1A} // 长度7 + 类型0x20 + 传感器ID 0x1234 + 温度26
	_, err = conn.Write(dataUploadMsg)
	assert.Nil(t, err)

	// 读取数据上传响应: 长度(2字节) + 状态(1字节) + 传感器ID回显(2字节)
	uploadResponse := make([]byte, 5)
	_, err = conn.Read(uploadResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), uploadResponse[0]) // 长度高字节
	assert.Equal(t, byte(0x03), uploadResponse[1]) // 长度低字节 (3字节数据)
	assert.Equal(t, byte(0x00), uploadResponse[2]) // 成功状态
	assert.Equal(t, byte(0x12), uploadResponse[3]) // 传感器ID回显高字节
	assert.Equal(t, byte(0x34), uploadResponse[4]) // 传感器ID回显低字节

	// 测试未知消息类型
	unknownMsg := []byte{0x00, 0x02, 0xFF, 0x99} // 长度2 + 未知类型0xFF
	_, err = conn.Write(unknownMsg)
	assert.Nil(t, err)

	// 读取错误响应
	errorResponse := make([]byte, 4)
	_, err = conn.Read(errorResponse)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), errorResponse[0]) // 长度高字节
	assert.Equal(t, byte(0x02), errorResponse[1]) // 长度低字节 (2字节数据)
	assert.Equal(t, byte(0xFF), errorResponse[2]) // 错误状态
	assert.Equal(t, byte(0x04), errorResponse[3]) // 错误码

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// 测试自定义分隔符数据包分割
func TestDelimiterEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// 启动分隔符服务器
	go startDelimiterServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 创建TCP客户端连接到服务器
	conn, err := net.Dial("tcp", ":8092")
	assert.Nil(t, err)
	defer conn.Close()

	// 测试AT命令
	atCommand := "AT+INFO\r\n"
	_, err = conn.Write([]byte(atCommand))
	assert.Nil(t, err)

	// 读取AT命令响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	assert.Nil(t, err)
	response := string(buffer[:n])
	assert.True(t, strings.Contains(response, "OK"))
	assert.True(t, strings.Contains(response, "Device: RuleGo-Test"))

	// 测试传感器数据命令
	sensorCommand := "SENSOR,TEMP,01,25.6\r\n"
	_, err = conn.Write([]byte(sensorCommand))
	assert.Nil(t, err)

	// 读取传感器响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "ACK,TEMP,01,OK"))

	// 测试Modbus ASCII命令
	modbusCommand := ":010300010001FA\r\n"
	_, err = conn.Write([]byte(modbusCommand))
	assert.Nil(t, err)

	// 读取Modbus响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, ":01030401020304FF"))

	// 测试无效格式命令
	invalidCommand := "INVALID_FORMAT\r\n"
	_, err = conn.Write([]byte(invalidCommand))
	assert.Nil(t, err)

	// 读取错误响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "ERROR"))
	assert.True(t, strings.Contains(response, "Unknown command"))

	// 测试无效传感器数据格式
	invalidSensor := "SENSOR,TEMP\r\n"
	_, err = conn.Write([]byte(invalidSensor))
	assert.Nil(t, err)

	// 读取无效传感器响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "NAK,INVALID_FORMAT"))

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// 测试配置默认值
func TestConfigDefaults(t *testing.T) {
	ep := &Net{}
	config := types.NewConfig()

	// 测试默认配置
	err := ep.Init(config, types.Configuration{})
	assert.Nil(t, err)
	assert.Equal(t, "tcp", ep.Config.Protocol)
	assert.Equal(t, "line", ep.Config.PacketMode)
	assert.Equal(t, 65536, ep.Config.MaxPacketSize)

	// 测试部分配置
	err = ep.Init(config, types.Configuration{
		"packetMode": "fixed",
		"packetSize": 20,
	})
	assert.Nil(t, err)
	assert.Equal(t, "fixed", ep.Config.PacketMode)
	assert.Equal(t, 20, ep.Config.PacketSize)
	assert.Equal(t, 65536, ep.Config.MaxPacketSize) // 默认值
}

// 测试并发安全性 - 简化版本
func TestPacketSplitterConcurrency(t *testing.T) {
	// 简化的并发测试，只测试基本功能
	// 测试多个分割器同时创建
	var wg sync.WaitGroup
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每个 goroutine 使用独立的配置，避免数据竞争
			config := Config{PacketMode: "line"}
			_, err := CreatePacketSplitter(config)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// 验证所有分割器创建成功
	for err := range results {
		assert.Nil(t, err)
	}
}

// 辅助函数：启动固定长度服务器
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

	// 添加路由处理固定长度数据
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		dataStr := exchange.In.GetMsg().GetData()
		data := []byte(dataStr)
		// 验证接收到16字节数据
		assert.Equal(t, 16, len(data))

		// 解析数据（修正字节序解析）
		deviceId := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
		command := uint32(data[4])<<24 | uint32(data[5])<<16 | uint32(data[6])<<8 | uint32(data[7])

		assert.Equal(t, uint32(1001), deviceId)
		assert.Equal(t, uint32(1), command)

		// 发送16字节响应
		response := make([]byte, 16)
		// 状态码：成功
		response[0], response[1], response[2], response[3] = 0x00, 0x00, 0x00, 0x00
		// 设备ID回显
		copy(response[4:8], data[0:4])
		// 其他数据
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

// 辅助函数：启动长度前缀服务器
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

	// 添加路由处理长度前缀数据
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := exchange.In.GetMsg().GetBytes()

		// 验证长度前缀
		assert.True(t, len(data) >= 3)
		dataLength := uint16(data[0])<<8 | uint16(data[1])
		messageType := data[2]

		expectedLength := int(dataLength) + 2 // 数据长度 + 长度前缀
		assert.Equal(t, expectedLength, len(data))

		var response []byte
		switch messageType {
		case 0x10: // HEARTBEAT
			// 响应：长度(2字节) + 状态(1字节) + 时间戳(4字节)
			timestamp := uint32(time.Now().Unix())
			response = []byte{
				0x00, 0x05, // 长度5字节
				0x00, // 成功状态
				byte(timestamp >> 24), byte(timestamp >> 16), byte(timestamp >> 8), byte(timestamp),
			}
		case 0x20: // DATA_UPLOAD
			// 响应：长度(2字节) + 状态(1字节) + 传感器ID回显(2字节)
			if len(data) >= 7 {
				response = []byte{
					0x00, 0x03, // 长度3字节
					0x00,             // 成功状态
					data[3], data[4], // 传感器ID回显
				}
			}
		default:
			// 未知消息类型
			response = []byte{0x00, 0x02, 0xFF, 0x04} // 错误响应
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

// 辅助函数：启动自定义分隔符服务器
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

	// 添加路由处理分隔符数据
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		command := exchange.In.GetMsg().GetData()
		var response string

		if strings.HasPrefix(command, "AT+") {
			// AT命令处理
			if strings.Contains(command, "INFO") {
				response = "OK\r\nDevice: RuleGo-Test\r\nVersion: 1.0.0\r\n"
			} else {
				response = "ERROR\r\nUnknown AT command\r\n"
			}
		} else if strings.HasPrefix(command, "SENSOR,") {
			// CSV命令处理
			parts := strings.Split(command, ",")
			if len(parts) >= 4 {
				response = fmt.Sprintf("ACK,%s,%s,OK\r\n", parts[1], parts[2])
			} else {
				response = "NAK,INVALID_FORMAT\r\n"
			}
		} else if strings.HasPrefix(command, ":") {
			// Modbus ASCII处理
			response = ":01030401020304FF\r\n" // 模拟响应
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

// 辅助函数：启动并发测试服务器
func startConcurrentServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	config := engine.NewConfig(types.WithDefaultPool())

	ep := &Net{}
	nodeConfig := types.Configuration{
		"protocol":    "tcp",
		"server":      ":8093",
		"readTimeout": 10,
		"packetMode":  "line", // 使用默认行分割模式
	}

	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	var messageCount int32

	// 添加路由处理并发消息
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		atomic.AddInt32(&messageCount, 1)

		// 简单的JSON解析验证
		data := exchange.In.GetMsg().GetData()
		assert.True(t, strings.Contains(data, "client"))
		assert.True(t, strings.Contains(data, "message"))

		// 发送确认响应
		response := fmt.Sprintf("{\"ack\":true,\"received\":\"%s\"}\n", data)
		exchange.Out.SetBody([]byte(response))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	err = ep.Start()
	assert.Nil(t, err)

	<-stop

	// 验证接收到期望数量的消息
	expectedMessages := int32(50) // 10个客户端 * 5条消息
	actualMessages := atomic.LoadInt32(&messageCount)
	assert.Equal(t, expectedMessages, actualMessages)

	ep.Destroy()
	wg.Done()
}

// 测试路由匹配选项功能
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

		// 验证路由被正确添加
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.matchOptions) // 默认没有匹配选项
	})

	t.Run("原始数据匹配", func(t *testing.T) {
		options := &RouterMatchOptions{
			MatchRawData: true,
		}
		router := impl.NewRouter().From("test").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// 验证路由选项被正确设置
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

		// 验证路由选项被正确设置
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
		router := impl.NewRouter().From("length.*").End() // 使用不同的正则避免重复
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// 验证路由选项被正确设置
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

		// 验证所有选项被正确设置
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

// 测试TCP处理器的路由匹配逻辑
func TestTcpHandlerRouteMatching(t *testing.T) {
	ep := &Net{}
	handler := &TcpHandler{endpoint: ep}

	// 创建测试路由
	router1 := &RegexpRouter{
		regexp:       nil, // 匹配所有
		matchOptions: nil, // 默认行为
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

		// 测试默认匹配（无选项）
		result := handler.matchesRouter(router1, rawData, encodedData, exchange)
		assert.True(t, result) // 无正则表达式，应该匹配所有
	})

	t.Run("原始数据匹配", func(t *testing.T) {
		rawData := []byte("test data")
		encodedData := []byte("encoded test data") // 编码后的数据
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				body: encodedData,
			},
		}

		// 测试原始数据匹配
		result := handler.matchesRouter(router2, rawData, encodedData, exchange)
		assert.True(t, result) // 原始数据包含"test"
	})

	t.Run("数据长度过滤", func(t *testing.T) {
		// 数据太短
		shortData := []byte("short")
		exchange1 := &endpoint.Exchange{
			In: &RequestMessage{body: shortData},
		}
		result1 := handler.matchesRouter(router3, shortData, shortData, exchange1)
		assert.False(t, result1) // 长度5 < 最小长度10

		// 数据长度合适
		validData := []byte("this is valid data for testing")
		exchange2 := &endpoint.Exchange{
			In: &RequestMessage{body: validData},
		}
		result2 := handler.matchesRouter(router3, validData, validData, exchange2)
		assert.True(t, result2) // 长度在范围内

		// 数据太长
		longData := make([]byte, 100)
		for i := range longData {
			longData[i] = 'a'
		}
		exchange3 := &endpoint.Exchange{
			In: &RequestMessage{body: longData},
		}
		result3 := handler.matchesRouter(router3, longData, longData, exchange3)
		assert.False(t, result3) // 长度100 > 最大长度50
	})
}

// 测试UDP处理器的路由匹配逻辑
func TestUdpHandlerRouteMatching(t *testing.T) {
	ep := &Net{}
	handler := &UDPHandler{endpoint: ep}

	// 创建测试路由，测试数据类型过滤
	router := &RegexpRouter{
		regexp: regexp.MustCompile(".*"),
		matchOptions: &RouterMatchOptions{
			DataTypeFilter: "JSON",
		},
	}

	t.Run("数据类型过滤", func(t *testing.T) {
		jsonData := []byte(`{"key": "value"}`)

		// 创建JSON类型的消息
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				body: jsonData,
			},
		}
		// 设置消息类型为JSON
		msg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), string(jsonData))
		exchange.In.SetMsg(&msg)

		result := handler.matchesRouter(router, jsonData, jsonData, exchange)
		assert.True(t, result) // JSON类型匹配

		// 测试不匹配的类型
		textMsg := types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "plain text")
		exchange.In.SetMsg(&textMsg)

		result2 := handler.matchesRouter(router, jsonData, jsonData, exchange)
		assert.False(t, result2) // TEXT类型不匹配JSON过滤器
	})
}

// 测试UDP端点
func TestUDPEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// 启动UDP服务器
	go startUDPServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 创建UDP客户端连接到服务器
	conn, err := net.Dial("udp", ":8094")
	assert.Nil(t, err)
	defer conn.Close()

	// 发送UDP消息
	message1 := "Hello UDP Server"
	_, err = conn.Write([]byte(message1))
	assert.Nil(t, err)

	// 读取UDP响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	assert.Nil(t, err)
	response := string(buffer[:n])
	assert.True(t, strings.Contains(response, "UDP received: Hello UDP Server"))

	// 发送JSON格式UDP消息
	jsonMsg := `{"type":"sensor","data":{"temperature":25.5,"humidity":60}}`
	_, err = conn.Write([]byte(jsonMsg))
	assert.Nil(t, err)

	// 读取JSON响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, `"status":"received"`))
	assert.True(t, strings.Contains(response, `"type":"json"`))

	// 发送心跳消息 (应该被过滤不响应)
	_, err = conn.Write([]byte(PingData))
	assert.Nil(t, err)

	// 心跳消息不应该有响应，所以我们发送另一条消息来确认服务器还在工作
	testMsg := "test after ping"
	_, err = conn.Write([]byte(testMsg))
	assert.Nil(t, err)

	// 读取测试消息响应
	n, err = conn.Read(buffer)
	assert.Nil(t, err)
	response = string(buffer[:n])
	assert.True(t, strings.Contains(response, "UDP received: test after ping"))

	time.Sleep(time.Millisecond * 100)
	stop <- struct{}{}
	wg.Wait()
}

// 测试编码功能
func TestEncodeFeatures(t *testing.T) {
	ep := &Net{}

	t.Run("十六进制编码", func(t *testing.T) {
		ep.Config = Config{Encode: "hex"}
		input := []byte("Hello")
		expected := []byte("48656c6c6f") // 修正为小写，与Go标准库hex.Encode输出一致
		result, dataType := ep.encode(input)
		assert.Equal(t, string(expected), string(result))
		assert.Equal(t, types.TEXT, dataType)
	})

	t.Run("Base64编码", func(t *testing.T) {
		ep.Config = Config{Encode: "base64"}
		input := []byte("Hello World")
		result, dataType := ep.encode(input)
		assert.Equal(t, types.TEXT, dataType)
		// 验证结果是有效的Base64
		assert.True(t, len(result) > 0)
		// 简单验证Base64字符集
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
		result, dataType := ep.encode(input)
		assert.Equal(t, string(input), string(result))
		assert.Equal(t, types.BINARY, dataType) // 默认为二进制
	})

	t.Run("未知编码类型", func(t *testing.T) {
		ep.Config = Config{Encode: "unknown"}
		input := []byte("Hello")
		result, dataType := ep.encode(input)
		assert.Equal(t, string(input), string(result))
		assert.Equal(t, types.BINARY, dataType) // 默认为二进制
	})
}

// 测试数据包分割器错误处理
func TestPacketSplitterErrorHandling(t *testing.T) {
	ep := &Net{}

	t.Run("分隔符解析错误", func(t *testing.T) {
		ep.Config = Config{
			PacketMode: "delimiter",
			Delimiter:  "0xZZ", // 无效的十六进制
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

		// 模拟一个包长度超过限制的数据
		reader := strings.NewReader("\x00\x20test") // 长度32 > 最大10
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

		// 模拟长度小于前缀大小的数据
		reader := strings.NewReader("\x00\x01") // 长度1 < 前缀大小2
		bufReader := bufio.NewReader(reader)
		_, err := splitter.ReadPacket(bufReader)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "invalid packet length"))
	})
}

// 测试响应消息的JSON处理
func TestResponseMessageJSONHandling(t *testing.T) {
	response := &ResponseMessage{}

	t.Run("JSON消息自动添加换行符", func(t *testing.T) {
		// 模拟JSON消息
		jsonMsg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"test":"data"}`)
		response.SetMsg(&jsonMsg)

		// 设置不带换行符的JSON数据
		jsonData := []byte(`{"response":"ok"}`)
		response.SetBody(jsonData)

		// 验证是否自动添加了换行符
		body := response.Body()
		assert.True(t, strings.HasSuffix(string(body), LineBreak))
	})

	t.Run("非JSON消息不添加换行符", func(t *testing.T) {
		// 模拟TEXT消息
		textMsg := types.NewMsg(0, "", types.TEXT, types.NewMetadata(), "test data")
		response.SetMsg(&textMsg)

		// 设置普通文本数据
		textData := []byte("simple response")
		response.SetBody(textData)

		// 验证没有自动添加换行符
		body := response.Body()
		assert.Equal(t, "simple response", string(body))
	})

	t.Run("已有换行符的JSON不重复添加", func(t *testing.T) {
		// 模拟JSON消息
		jsonMsg := types.NewMsg(0, "", types.JSON, types.NewMetadata(), `{"test":"data"}`)
		response.SetMsg(&jsonMsg)

		// 设置已带换行符的JSON数据
		jsonData := []byte(`{"response":"ok"}` + LineBreak)
		response.SetBody(jsonData)

		// 验证换行符数量正确
		body := response.Body()
		lineBreakCount := strings.Count(string(body), LineBreak)
		assert.Equal(t, 1, lineBreakCount)
	})
}

// 测试并发读写安全性
func TestConcurrentSafety(t *testing.T) {
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 启动并发安全测试服务器
	wg.Add(1)
	go startConcurrentServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 启动多个客户端进行并发测试
	clientCount := 10
	messagesPerClient := 5

	var clientWg sync.WaitGroup
	for i := 0; i < clientCount; i++ {
		clientWg.Add(1)
		go func(clientId int) {
			defer clientWg.Done()

			// 创建TCP客户端连接
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

				// 读取响应
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

// 测试边界条件
func TestBoundaryConditions(t *testing.T) {
	config := types.NewConfig()

	t.Run("空配置初始化", func(t *testing.T) {
		ep := &Net{}
		err := ep.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, "tcp", ep.Config.Protocol)
		// 空配置时Server字段为空字符串，不是默认值
		assert.Equal(t, "", ep.Config.Server)
		assert.Equal(t, 0, ep.Config.ReadTimeout)
	})

	t.Run("最小配置", func(t *testing.T) {
		ep := &Net{}
		err := ep.Init(config, types.Configuration{
			"server": ":0", // 使用随机端口
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

// 测试连接管理
func TestConnectionManagement(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Protocol:    "tcp",
		Server:      "127.0.0.1:8898",
		ReadTimeout: 1, // 短超时用于快速测试
	}, &nodeConfig)

	var ep = &Net{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)

	// 添加一个简单路由
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("connected"))
		return true
	}).End()

	_, err = ep.AddRouter(router)
	assert.Nil(t, err)

	t.Run("启动和停止服务", func(t *testing.T) {
		err = ep.Start()
		assert.Nil(t, err)

		// 验证服务器ID
		assert.Equal(t, "127.0.0.1:8898", ep.Id())

		// 关闭服务
		err = ep.Close()
		assert.Nil(t, err)

		// 重复关闭应该不出错
		err = ep.Close()
		assert.Nil(t, err)

		// 销毁
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

// 测试不同数据包分割模式的路由
func TestPacketModeRouting(t *testing.T) {
	// 测试固定长度模式的路由
	t.Run("固定长度模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "fixed",
			PacketSize: 8,
		}

		// 创建针对固定长度数据的路由
		options := &RouterMatchOptions{
			MinDataLength: 8,
			MaxDataLength: 8,
		}
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// 验证路由设置
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.Equal(t, 8, routerObj.matchOptions.MinDataLength)
		assert.Equal(t, 8, routerObj.matchOptions.MaxDataLength)
	})

	// 测试长度前缀模式的路由
	t.Run("长度前缀模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "length_prefix_le",
			PacketSize: 2,
		}

		// 创建考虑长度前缀的路由
		options := &RouterMatchOptions{
			MatchRawData:  true, // 匹配包含长度前缀的完整数据
			MinDataLength: 3,    // 最小2字节前缀+1字节数据
		}
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// 验证路由设置
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
		assert.Equal(t, 3, routerObj.matchOptions.MinDataLength)
	})

	// 测试分隔符模式的路由
	t.Run("分隔符模式路由", func(t *testing.T) {
		ep := &Net{}
		ep.Config = Config{
			PacketMode: "delimiter",
			Delimiter:  "END",
		}

		// 创建匹配不包含分隔符的数据的路由
		options := &RouterMatchOptions{
			MatchRawData: true, // 匹配去除分隔符后的原始数据
		}
		router := impl.NewRouter().From("^[^E]*$").End() // 不包含E字符的数据（简化正则）
		routerId, err := ep.AddRouter(router, options)
		assert.Nil(t, err)

		// 验证路由设置
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.NotNil(t, routerObj.matchOptions)
		assert.True(t, routerObj.matchOptions.MatchRawData)
	})
}

// 测试编码数据的路由匹配
func TestEncodedDataRouting(t *testing.T) {
	ep := &Net{}
	handler := &TcpHandler{endpoint: ep}

	t.Run("十六进制编码路由", func(t *testing.T) {
		// 测试十六进制编码数据的路由
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^[0-9A-Fa-f]+$"), // 匹配十六进制字符
			matchOptions: &RouterMatchOptions{
				MatchRawData: false, // 匹配编码后的数据
			},
		}

		rawData := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F} // "Hello"
		// 模拟十六进制编码
		encodedData := []byte("48656C6C6F")

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := handler.matchesRouter(router, rawData, encodedData, exchange)
		assert.True(t, result) // 编码后的数据是十六进制
	})

	t.Run("Base64编码路由", func(t *testing.T) {
		// 测试Base64编码数据的路由
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^[A-Za-z0-9+/]+=*$"), // 匹配Base64字符
			matchOptions: &RouterMatchOptions{
				MatchRawData: false, // 匹配编码后的数据
			},
		}

		rawData := []byte("Hello World")
		// 模拟Base64编码
		encodedData := []byte("SGVsbG8gV29ybGQ=")

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := handler.matchesRouter(router, rawData, encodedData, exchange)
		assert.True(t, result) // 编码后的数据是Base64
	})

	t.Run("原始二进制数据路由", func(t *testing.T) {
		// 测试原始二进制数据的路由
		router := &RegexpRouter{
			regexp: regexp.MustCompile("^\x48\x65\x6C"), // 匹配二进制模式"Hel"
			matchOptions: &RouterMatchOptions{
				MatchRawData: true, // 匹配原始数据
			},
		}

		rawData := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F} // "Hello"
		encodedData := []byte("48656C6C6F")             // 十六进制编码

		exchange := &endpoint.Exchange{
			In: &RequestMessage{body: encodedData},
		}

		result := handler.matchesRouter(router, rawData, encodedData, exchange)
		assert.True(t, result) // 原始数据以"Hel"开头
	})
}

// 测试路由优先级和多路由匹配
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
		// 添加通用路由
		generalRouter := impl.NewRouter().From(".*").End()
		_, err := ep.AddRouter(generalRouter)
		assert.Nil(t, err)

		// 添加特定路由
		specificOptions := &RouterMatchOptions{
			DataTypeFilter: "JSON",
			MinDataLength:  10,
		}
		specificRouter := impl.NewRouter().From("^{.*").End()
		_, err = ep.AddRouter(specificRouter, specificOptions)
		assert.Nil(t, err)

		// 添加长度限制路由
		lengthOptions := &RouterMatchOptions{
			MaxDataLength: 100,
		}
		lengthRouter := impl.NewRouter().From("test").End()
		_, err = ep.AddRouter(lengthRouter, lengthOptions)
		assert.Nil(t, err)

		// 验证所有路由都被添加
		ep.Lock()
		routerCount := len(ep.routers)
		ep.Unlock()
		assert.Equal(t, 3, routerCount)
	})

	t.Run("路由条件互斥", func(t *testing.T) {
		// 清空现有路由
		ep.Lock()
		ep.routers = make(map[string]*RegexpRouter)
		ep.Unlock()

		// 添加只匹配短数据的路由
		shortOptions := &RouterMatchOptions{
			MaxDataLength: 10,
		}
		shortRouter := impl.NewRouter().From("short.*").End()
		_, err := ep.AddRouter(shortRouter, shortOptions)
		assert.Nil(t, err)

		// 添加只匹配长数据的路由
		longOptions := &RouterMatchOptions{
			MinDataLength: 20,
		}
		longRouter := impl.NewRouter().From("long.*").End()
		_, err = ep.AddRouter(longRouter, longOptions)
		assert.Nil(t, err)

		// 验证互斥路由被添加
		ep.Lock()
		routerCount := len(ep.routers)
		ep.Unlock()
		assert.Equal(t, 2, routerCount)
	})
}

// 测试路由错误处理
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
		router := impl.NewRouter().From("[a-z{1,5}").End() // 无效的正则
		_, err := ep.AddRouter(router)
		assert.NotNil(t, err) // 应该返回错误
	})

	t.Run("重复路由ID", func(t *testing.T) {
		router1 := impl.NewRouter().SetId("duplicate").From("test1").End()
		_, err := ep.AddRouter(router1)
		assert.Nil(t, err)

		router2 := impl.NewRouter().SetId("duplicate").From("test2").End()
		_, err = ep.AddRouter(router2)
		assert.NotNil(t, err) // 应该返回重复错误
		assert.True(t, strings.Contains(err.Error(), "duplicate router"))
	})

	t.Run("删除不存在的路由", func(t *testing.T) {
		err := ep.RemoveRouter("nonexistent")
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "not found"))
	})
}

// 测试特殊匹配功能
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

		// 验证路由被正确添加
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.regexp) // 空字符串不会编译正则表达式
	})

	t.Run("星号匹配所有", func(t *testing.T) {
		router := impl.NewRouter().From("*").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// 验证路由被正确添加
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		// "*" 会被编译为正则表达式，但在匹配时会被特殊处理
	})

	t.Run("点星匹配所有", func(t *testing.T) {
		router := impl.NewRouter().From(".*").End()
		routerId, err := ep.AddRouter(router)
		assert.Nil(t, err)

		// 验证路由被正确添加
		ep.Lock()
		routerObj := ep.routers[routerId]
		ep.Unlock()
		assert.NotNil(t, routerObj)
		assert.Nil(t, routerObj.regexp) // ".*" 是特殊匹配所有的情况，regexp应该为nil
	})
}

// 辅助函数：启动UDP服务器
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

	// 添加路由处理UDP数据
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		data := exchange.In.GetMsg().GetData()
		atomic.AddInt32(&messageCount, 1)

		// 验证消息不是心跳
		assert.NotEqual(t, PingData, data)

		var response string
		if strings.HasPrefix(data, "{") {
			// JSON消息
			response = `{"status":"received","type":"json"}`
		} else {
			// 普通文本消息
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

	// 验证接收到预期数量的消息（排除ping消息，实际会收到3条消息）
	actualMessages := atomic.LoadInt32(&messageCount)
	assert.True(t, actualMessages >= 2) // 至少应该收到2条消息

	ep.Destroy()
	wg.Done()
}

// 测试处理器的数据类型转换功能
func TestProcessorDataTypeConversion(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})

	// 启动测试服务器
	go startProcessorTestServer(t, stop, &wg)
	time.Sleep(time.Millisecond * 200)

	// 创建客户端
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

	// 测试JSON数据转换
	jsonData := `{"test":"data"}` + "\n"
	metaData := types.NewMetadata()
	metaData.PutValue("processor", "setJsonDataType")
	msg1 := types.NewMsg(0, "TEST_JSON", types.BINARY, metaData, jsonData)
	client.OnMsg(ctx, msg1)

	// 测试文本数据转换
	textData := "Hello World\n"
	metaData2 := types.NewMetadata()
	metaData2.PutValue("processor", "setTextDataType")
	msg2 := types.NewMsg(0, "TEST_TEXT", types.BINARY, metaData2, textData)
	client.OnMsg(ctx, msg2)

	// 测试二进制数据转换
	binaryData := string([]byte{0x01, 0x02, 0x03, 0x04}) + "\n"
	metaData3 := types.NewMetadata()
	metaData3.PutValue("processor", "setBinaryDataType")
	msg3 := types.NewMsg(0, "TEST_BINARY", types.TEXT, metaData3, binaryData)
	client.OnMsg(ctx, msg3)

	// 等待响应
	time.Sleep(time.Millisecond * 500)

	// 验证结果
	responseMutex.Lock()
	assert.True(t, len(responseReceived) >= 1, "应该收到至少一个响应")
	responseMutex.Unlock()

	stop <- struct{}{}
	wg.Wait()
}

// 创建net客户端节点
func createNetClientNode(t *testing.T, config types.Config, protocol, server string) types.Node {
	// 注册NET客户端组件
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
		"heartbeatInterval": 0, // 禁用心跳
	}

	err = node.Init(config, configuration)
	assert.Nil(t, err)

	return node
}

// 启动处理器测试服务器
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

	// 添加路由使用处理器转换数据类型
	router := impl.NewRouter().From(".*").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// 根据metadata中的processor字段选择处理器
		processorName := exchange.In.GetMsg().Metadata.GetValue("processor")

		// 应用对应的处理器
		switch processorName {
		case "setJsonDataType":
			// 使用内置的JSON数据类型处理器
			if proc, exists := processor.InBuiltins.Get("setJsonDataType"); exists {
				proc(router, exchange)
			}
		case "setTextDataType":
			// 使用内置的文本数据类型处理器
			if proc, exists := processor.InBuiltins.Get("setTextDataType"); exists {
				proc(router, exchange)
			}
		case "setBinaryDataType":
			// 使用内置的二进制数据类型处理器
			if proc, exists := processor.InBuiltins.Get("setBinaryDataType"); exists {
				proc(router, exchange)
			}
		}

		// 验证数据类型转换是否成功
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
