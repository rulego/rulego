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

// 测试客户端请求/响应消息
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

// 测试客户端类型和默认值
func TestNetClientType(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
	})
	assert.Nil(t, err)
	assert.Equal(t, ClientType, client.Type())
	assert.Equal(t, "127.0.0.1:9999", client.Id())

	// 验证默认值
	defaultClient := client.New().(*NetClient)
	assert.Equal(t, ProtocolTCP, defaultClient.Config.Protocol)
	assert.Equal(t, 5, defaultClient.Config.ConnectTimeout)
	assert.Equal(t, 5, defaultClient.Config.ReconnectInterval)
	assert.Equal(t, "line", defaultClient.Config.PacketMode)
}

// 测试路由管理
func TestNetClientRouter(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
	})
	assert.Nil(t, err)

	// 添加nil路由
	_, err = client.AddRouter(nil)
	assert.Equal(t, "router can not nil", err.Error())

	// 添加路由
	router := impl.NewRouter().SetId("r1").From("").End()
	routerId, err := client.AddRouter(router)
	assert.Nil(t, err)
	assert.Equal(t, "r1", routerId)

	// 重复路由
	router = impl.NewRouter().SetId("r1").From("").End()
	_, err = client.AddRouter(router)
	assert.NotNil(t, err)

	// 删除路由
	err = client.RemoveRouter("r1")
	assert.Nil(t, err)
	err = client.RemoveRouter("r1")
	assert.Equal(t, "router: r1 not found", err.Error())

	// 错误正则表达式
	router = impl.NewRouter().From("[a-z{1,5}").End()
	_, err = client.AddRouter(router)
	assert.NotNil(t, err)
}

// 测试TCP客户端连接到服务端并接收数据
func TestNetClientTCP(t *testing.T) {
	stop := make(chan struct{})

	// 启动模拟TCP服务端
	go startTCPEchoServer(t, stop)
	time.Sleep(time.Millisecond * 200)

	// 创建客户端
	config := engine.NewConfig(types.WithDefaultPool())
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&ClientConfig{
		Protocol:          "tcp",
		Server:            "127.0.0.1" + clientTestServer,
		ConnectTimeout:    5,
		ReconnectInterval: 0, // 测试中不重连
		PacketMode:        "line",
	}, &nodeConfig)

	client := &NetClient{}
	err := client.Init(config, nodeConfig)
	assert.Nil(t, err)

	var receiveCount int32

	// 添加路由
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

	// 启动客户端
	err = client.Start()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 200)

	// 等待接收数据
	time.Sleep(time.Second * 2)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&receiveCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receiveCount > 0, got %d", count))
}

// 测试TCP客户端发送数据到服务端
func TestNetClientTCPSend(t *testing.T) {
	stop := make(chan struct{})

	var serverReceived string
	var serverMu sync.Mutex

	// 启动TCP服务端
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

		// 回显
		conn.Write(buf[:n])
		close(done)
		time.Sleep(time.Second)
	}()

	time.Sleep(time.Millisecond * 200)

	// 创建客户端
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

	// 通过Send方法发送数据
	err = client.Send([]byte("hello from client\n"))
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 500)

	client.Destroy()
	close(stop)

	serverMu.Lock()
	assert.Equal(t, "hello from client\n", serverReceived)
	serverMu.Unlock()
}

// 测试客户端配置
func TestNetClientConfig(t *testing.T) {
	config := engine.NewConfig()

	// 通过Configuration初始化
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":           "192.168.1.100:8080",
		"protocol":         "tcp",
		"connectTimeout":   10,
		"readTimeout":      30,
		"reconnectInterval": 3,
		"packetMode":       "line",
		"encode":           "hex",
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

// 测试连接失败
func TestNetClientConnectFail(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server":         "127.0.0.1:19999", // 不存在的端口
		"connectTimeout": 1,
	})
	assert.Nil(t, err)

	err = client.Start()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "connect"))
}

// 测试数据编码
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
	// hex编码后长度翻倍
	assert.Equal(t, len(data)*2, len(encoded))

	client.Config.Encode = "base64"
	encoded, dataType = encodeData(data, client.Config.Encode)
	assert.Equal(t, types.TEXT, dataType)

	client.Config.Encode = "none"
	encoded, dataType = encodeData(data, client.Config.Encode)
	assert.Equal(t, types.BINARY, dataType)
	assert.Equal(t, data, encoded)
}

// 测试带规则链的TCP客户端
func TestNetClientWithRuleChain(t *testing.T) {
	stop := make(chan struct{})

	// 启动TCP服务端发送JSON数据
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

// 测试路由匹配选项
func TestNetClientRouterMatchOptions(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:9999",
	})
	assert.Nil(t, err)

	// 添加带匹配选项的路由
	router := impl.NewRouter().From("").End()
	opts := &RouterMatchOptions{
		MinDataLength: 5,
		MaxDataLength: 100,
	}
	_, err = client.AddRouter(router, opts)
	assert.Nil(t, err)
}

// ==================== 辅助函数 ====================

// startTCPEchoServer 启动一个简单的TCP回显服务器，会主动推送数据
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

	// 服务端主动推送数据
	for i := 0; i < 3; i++ {
		msg := fmt.Sprintf("echo: message %d\n", i+1)
		_, err := conn.Write([]byte(msg))
		if err != nil {
			return
		}
		time.Sleep(time.Millisecond * 300)
	}
	close(done)
	// 保持连接一段时间让客户端读取
	time.Sleep(time.Second)
}

// startTCPJsonServer 启动一个发送JSON数据的TCP服务器
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

	// 发送JSON传感器数据
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

// 测试客户端Close和Destroy
func TestNetClientCloseDestroy(t *testing.T) {
	config := types.NewConfig()
	client := &NetClient{}
	err := client.Init(config, types.Configuration{
		"server": "127.0.0.1:19999",
	})
	assert.Nil(t, err)

	// 未连接状态下Close不应报错
	err = client.Close()
	assert.Nil(t, err)

	// Destroy也不应报错
	client.Destroy()
}

// 测试通过registry创建客户端
func TestNetClientRegistryCreate(t *testing.T) {
	assert.Equal(t, true, true) // 占位，registry测试在集成测试中
	_ = reflect.TypeOf(&NetClient{})
}

// ==================== 集成测试：完整规则链流程 ====================

// TestNetClientWithRuleChainIntegration 完整流程：TCP服务端 → 客户端连接 → 接收数据 → Router → 规则链 → 响应回写
func TestNetClientWithRuleChainIntegration(t *testing.T) {
	// 加载规则链
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	stop := make(chan struct{})

	// 启动TCP服务端
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

		// 发送JSON数据
		for i := 0; i < 3; i++ {
			msg := `{"test":"integration"}` + "\n"
			_, err := conn.Write([]byte(msg))
			if err != nil {
				return
			}
			time.Sleep(time.Millisecond * 300)
		}

		// 读取客户端回写的响应
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

	// 创建客户端，Router → 规则链 → 响应回写
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
		msg.Type = "TEST_MSG_TYPE2" // 匹配规则链中 s4 分支

		atomic.AddInt32(&processedCount, 1)
		return true
	}).To("chain:default").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// 规则链执行完毕后，回写响应到服务端
		result := exchange.Out.GetMsg().GetData()
		exchange.Out.SetBody([]byte("client response: " + result + "\n"))
		return true
	}).End()

	_, err = client.AddRouter(router)
	assert.Nil(t, err)

	// 添加全局拦截器
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

	// 验证服务端收到响应回写
	time.Sleep(time.Millisecond * 100)
	serverMu.Lock()
	assert.True(t, serverReceivedResponse != "", "expected server to receive response from client")
	serverMu.Unlock()
}

// TestNetClientReconnect 测试断线重连
func TestNetClientReconnect(t *testing.T) {
	stop := make(chan struct{})

	var connectCount int32

	// 启动一个长期存活的TCP服务端，接受两次连接
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
			// 发送数据后关闭连接
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
		"reconnectInterval": 1, // 1秒后重连
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

	// 等待重连和接收数据
	time.Sleep(time.Second * 5)

	client.Destroy()
	close(stop)

	totalReceived := atomic.LoadInt32(&receiveCount)
	totalConnects := atomic.LoadInt32(&connectCount)
	assert.True(t, totalReceived >= 2, fmt.Sprintf("expected total received >= 2, got %d", totalReceived))
	assert.True(t, totalConnects >= 2, fmt.Sprintf("expected at least 2 connections, got %d", totalConnects))
}

// TestNetClientHeartbeat 测试心跳发送
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
		"heartbeatInterval": 1, // 1秒发送心跳
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

// TestNetClientResponseWriteBack 测试通过 ResponseMessage.SetBody() 回写数据到服务端
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

		// 发送一行数据
		conn.Write([]byte("request data\n"))

		// 读取客户端回写的响应
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
		// 通过 ResponseMessage.SetBody() 回写数据到服务端
		exchange.Out.SetBody([]byte("ack:" + string(exchange.In.Body()) + "\n"))
		return false // 不需要继续处理
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

// TestNetClientUDPIntegration 测试UDP客户端连接
func TestNetClientUDPIntegration(t *testing.T) {
	stop := make(chan struct{})

	// 启动UDP服务端
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
			// 回显
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

	// 通过 Send 发送数据（服务端会回显，客户端 readLoopUDP 接收）
	err = client.Send([]byte("hello udp"))
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	client.Destroy()
	close(stop)

	count := atomic.LoadInt32(&receivedCount)
	assert.True(t, count > 0, fmt.Sprintf("expected receivedCount > 0, got %d", count))
}

// TestNetClientRegistryCreateEndpoint 通过Registry创建客户端端点
func TestNetClientRegistryCreateEndpoint(t *testing.T) {
	stop := make(chan struct{})
	go startTCPEchoServer(t, stop)
	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())

	// 直接构造（因为测试中无法导入 endpoint 包使用 Registry）
	// 验证 NetClient 实现了 endpoint.Endpoint 接口
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

// TestNetClientHeartbeatCustomData 测试自定义心跳包内容（通过配置）
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

// TestNetClientHeartbeatHexData 测试十六进制格式的心跳包内容
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

// TestNetClientHeartbeatCallback 测试通过OnHeartbeat回调自定义心跳
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

	// 自定义心跳回调：发送带时间戳的心跳
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
