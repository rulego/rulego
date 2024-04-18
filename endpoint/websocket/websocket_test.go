package websocket

import (
	"github.com/gorilla/websocket"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"log"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

var testdataFolder = "../../testdata"
var testServer = ":9090"
var testConfigServer = ":9091"

// 测试请求/响应消息
func TestWebSocketMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		var request = &RequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("Response", func(t *testing.T) {
		var response = &ResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

func TestWsEndpointConfig(t *testing.T) {
	config := rulego.NewConfig(types.WithDefaultPool())
	//创建endpoint服务
	wsStarted, _ := endpoint.New(Type, config, Config{
		Server: testConfigServer,
	})
	assert.Equal(t, testConfigServer, wsStarted.Id())

	go func() {
		err := wsStarted.Start()
		assert.Equal(t, "http: Server closed", err.Error())
	}()

	time.Sleep(time.Millisecond * 200)

	epErr, _ := endpoint.New(Type, config, types.Configuration{
		"server": testConfigServer,
	})

	ep, _ := endpoint.New(Type, config, types.Configuration{
		"server": testConfigServer,
	})

	assert.Equal(t, testConfigServer, ep.Id())
	testUrl := "/api/test"
	router := endpoint.NewRouter().From(testUrl).End()
	routerId, _ := ep.AddRouter(router, "GET")
	assert.Equal(t, "/api/test", routerId)

	router = endpoint.NewRouter().From(testUrl).End()
	routerId, _ = ep.AddRouter(router, "GET")
	assert.Equal(t, "/api/test", routerId)

	//删除路由
	ep.RemoveRouter(routerId)
	ep.RemoveRouter(routerId, "GET")

	ep.AddRouter(nil)
	wsStarted.Destroy()
	epErr.Destroy()
	time.Sleep(time.Millisecond * 200)
}

func TestWsEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	//启动服务
	go startServer(t, stop, &wg, false)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "ws://127.0.0.1"+testServer+"/api/v1/echo/TEST_MSG_TYPE1?aa=xx")
	//停止服务器
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

func TestMultiplexRestEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	//启动服务
	go startServer(t, stop, &wg, true)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "ws://127.0.0.1"+testServer+"/api/v1/echo/TEST_MSG_TYPE1?aa=xx")
	time.Sleep(time.Millisecond * 200)
	//停止服务器
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

// 发送消息到rest服务器
func sendMsg(t *testing.T, url string) {

	// 连接WebSocket服务器
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		time.Sleep(time.Millisecond * 200)
		conn.Close()
	}()

	// 发送消息
	err = conn.WriteMessage(websocket.BinaryMessage, []byte("Hello, world!"))
	if err != nil {
		log.Fatal(err)
	}

	// 读取消息
	_, p, err := conn.ReadMessage()
	if err != nil {
		log.Fatal(err)
	}
	assert.Equal(t, "ok", string(p))

}

// 启动服务
func startServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup, isMultiplex bool) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))
	var wsEndpoint endpoint.Endpoint
	//var restStartGroup sync.WaitGroup
	//restStartGroup.Add(1)
	restEndpoint := &rest.Endpoint{
		Config: rest.Config{Server: testServer},
	}
	//复用rest endpoint
	if isMultiplex {
		restEndpoint.OnEventFunc = func(eventName string, params ...interface{}) {
			if eventName == endpoint.EventInitServer {
				wsEndpoint = newWebsocketServe(t, restEndpoint)
				if err := wsEndpoint.Start(); err != nil {
					t.Fatal("error:", err)
				}
			}
		}
	} else {
		wsEndpoint = newWebsocketServe(t, nil)
	}

	go func() {
		for {
			select {
			case <-stop:
				// 接收到中断信号，退出循环
				wsEndpoint.Destroy()
				restEndpoint.Destroy()
				return
			default:
			}
		}
	}()

	if isMultiplex {
		//复用rest endpoint
		_ = restEndpoint.Start()
	} else {
		//并启动服务
		_ = wsEndpoint.Start()
	}

	wg.Done()
}

func newWebsocketServe(t *testing.T, restEndpoint *rest.Rest) endpoint.Endpoint {
	config := rulego.NewConfig(types.WithDefaultPool())
	wsEndpoint, err := endpoint.New(Type, config, Config{Server: testServer})
	if err != nil {
		t.Fatal(err)
	}

	if restEndpoint != nil {
		wsEndpoint = &Websocket{RestEndpoint: restEndpoint}
	}
	//添加全局拦截器
	wsEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/echo/:msgType").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//处理请求
		requestMessage, ok := exchange.In.(*RequestMessage)
		if ok {
			assert.True(t, ok)
			assert.NotNil(t, requestMessage.Request())
			assert.Equal(t, "websocket", requestMessage.Headers().Get("Upgrade"))

			assert.Equal(t, "Hello, world!", string(exchange.In.Body()))
			assert.Equal(t, "Hello, world!", string(exchange.In.GetMsg().Data))

			from := requestMessage.From()
			msgType := requestMessage.GetMsg().Metadata.GetValue("msgType")
			assert.Equal(t, "/api/v1/echo/"+msgType+"?aa=xx", from)
			assert.Equal(t, "xx", requestMessage.GetParam("aa"))

			responseMessage, _ := exchange.Out.(*ResponseMessage)

			assert.Equal(t, "/api/v1/echo/"+msgType+"?aa=xx", responseMessage.From())
			assert.Equal(t, "xx", responseMessage.GetParam("aa"))

			if requestMessage.request.Method != http.MethodGet {
				//响应错误
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//不执行后续动作
				return false
			} else {
				//响应请求
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte("ok"))
				name := requestMessage.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//不执行后续动作
					return false
				} else {
					return true
				}

			}
		} else {
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(exchange.In.From()))
			exchange.Out.SetBody([]byte("s1 process" + "\n"))
			return true
		}

	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = exchange.In.GetParam("msgType")
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).To("chain:default").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("规则链执行结果：" + exchange.Out.GetMsg().Data + "\n"))
		return true
	}).End()

	//注册路由
	wsEndpoint.AddRouter(router1)

	assert.NotNil(t, wsEndpoint.(*Endpoint).Router())
	return wsEndpoint
}
