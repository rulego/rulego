package websocket

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
)

var testdataFolder = "../../testdata/rule"

// testServer uses an OS-assigned port: the historical fixed :9090 collides
// with unrelated services on developer machines.
var testServer = func() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return ":9090"
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return ":" + port
}()

var testConfigServer = ":9091"

// 握手来源校验：同源与无 Origin 始终放行，AllowCors 只管跨源。
// 回归 AllowCors=false（宿主自管 CORS 的嵌入式部署）时同源握手被误拒的问题。
func TestCheckOrigin(t *testing.T) {
	newReq := func(origin, host string) *http.Request {
		r := &http.Request{Host: host, Header: http.Header{}}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	t.Run("AllowCorsFalse", func(t *testing.T) {
		ws := &Websocket{}
		ws.Config.AllowCors = false
		// 无 Origin（非浏览器客户端）
		assert.True(t, ws.checkOrigin(newReq("", "example.com:8081")))
		// 同源
		assert.True(t, ws.checkOrigin(newReq("http://example.com:8081", "example.com:8081")))
		// 同源，scheme 不同但 host 相同（https 页面连 wss 同主机）
		assert.True(t, ws.checkOrigin(newReq("https://example.com:8081", "example.com:8081")))
		// 跨源：AllowCors=false 时拒绝
		assert.False(t, ws.checkOrigin(newReq("http://evil.com", "example.com:8081")))
		// 端口不同即跨源
		assert.False(t, ws.checkOrigin(newReq("http://example.com:9999", "example.com:8081")))
	})

	t.Run("AllowCorsTrue", func(t *testing.T) {
		ws := &Websocket{}
		ws.Config.AllowCors = true
		assert.True(t, ws.checkOrigin(newReq("http://evil.com", "example.com:8081")))
		assert.True(t, ws.checkOrigin(newReq("http://example.com:8081", "example.com:8081")))
	})
}

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

func TestRouterId(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{Config: rest.Config{Server: testServer}}, &nodeConfig)
	var ep = &Endpoint{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
	assert.Equal(t, testServer, ep.Id())
	router := impl.NewRouter().SetId("r1").From("/device/info").End()
	routerId, _ := ep.AddRouter(router, "GET")
	assert.Equal(t, "r1", routerId)

	router = impl.NewRouter().From("/device/info/v2").End()
	routerId, _ = ep.AddRouter(router, "POST")
	assert.Equal(t, "/device/info/v2", routerId)

	err = ep.RemoveRouter("r1")
	assert.Nil(t, err)
	err = ep.RemoveRouter("/device/info/v2")
	assert.Nil(t, err)
	err = ep.RemoveRouter("/device/info/v2")
	assert.Equal(t, fmt.Sprintf("router: %s not found", "/device/info/v2"), err.Error())
}

func TestWsEndpointConfig(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	//创建endpoint服务
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{Config: rest.Config{Server: testConfigServer}}, &nodeConfig)
	var wsStarted = &Endpoint{}
	err := wsStarted.Init(config, nodeConfig)
	assert.Nil(t, err)

	assert.Equal(t, testConfigServer, wsStarted.Id())

	err = wsStarted.Start()
	assert.Nil(t, err)

	//go func() {
	//	err := wsStarted.Start()
	//	assert.Equal(t, "http: Server closed", err.Error())
	//}()

	time.Sleep(time.Millisecond * 200)

	var epErr = &Endpoint{}
	err = epErr.Init(config, nodeConfig)

	var ep = &Endpoint{}
	err = ep.Init(config, nodeConfig)

	assert.Equal(t, testConfigServer, ep.Id())
	testUrl := "/api/test"
	router := impl.NewRouter().From(testUrl).End()
	routerId, _ := ep.AddRouter(router, "GET")
	assert.Equal(t, "/api/test", routerId)

	router = impl.NewRouter().From(testUrl).End()
	_, err = ep.AddRouter(router, "GET")
	assert.NotNil(t, err)

	//删除路由
	_ = ep.RemoveRouter(routerId)
	_ = ep.RemoveRouter(routerId, "GET")

	_, _ = ep.AddRouter(nil)
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
		t.Fatal(err)
	}
	defer func() {
		time.Sleep(time.Millisecond * 200)
		conn.Close()
	}()

	// 发送消息
	err = conn.WriteMessage(websocket.BinaryMessage, []byte("Hello, world!"))
	if err != nil {
		t.Fatal(err)
	}

	// 读取消息
	_, p, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "ok", string(p))

}

// 启动服务
func startServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup, isMultiplex bool) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Error(err)
		wg.Done()
		return
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))
	var wsEndpoint endpoint.Endpoint
	restEndpoint := &rest.Endpoint{
		Config: rest.Config{Server: testServer},
	}
	//复用rest endpoint
	if isMultiplex {
		wsEndpoint = newWebsocketServe(t, restEndpoint)
		if wsEndpoint == nil {
			wg.Done()
			return
		}
		if err := wsEndpoint.Start(); err != nil {
			t.Error("error:", err)
			wg.Done()
			return
		}
	} else {
		wsEndpoint = newWebsocketServe(t, nil)
	}

	if isMultiplex {
		//复用rest endpoint
		_ = restEndpoint.Start()
	} else {
		if wsEndpoint == nil {
			wg.Done()
			return
		}
		//并启动服务
		_ = wsEndpoint.Start()
	}
	<-stop
	wsEndpoint.Destroy()
	restEndpoint.Destroy()
	wg.Done()
}

func newWebsocketServe(t *testing.T, restEndpoint *rest.Rest) endpoint.Endpoint {
	config := engine.NewConfig(types.WithDefaultPool())
	//wsEndpoint, err := endpoint.New(Type, config, Config{Server: testServer})

	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{Config: rest.Config{Server: testServer, AllowCors: true}}, &nodeConfig)
	var wsEndpoint = &Endpoint{}
	err := wsEndpoint.Init(config, nodeConfig)
	if err != nil {
		t.Error(err)
		return nil
	}

	assert.Equal(t, Type, wsEndpoint.Type())
	assert.True(t, reflect.DeepEqual(&Websocket{
		Config: Config{Config: rest.Config{Server: ":6334", AllowCors: true}, SessionTTL: 1800},
	}, wsEndpoint.New()))

	if restEndpoint != nil {
		wsEndpoint = &Websocket{Rest: restEndpoint, Config: Config{Config: rest.Config{AllowCors: true}}}
	}
	//添加全局拦截器
	wsEndpoint.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := impl.NewRouter().From("/api/v1/echo/:msgType").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//处理请求
		requestMessage, ok := exchange.In.(*RequestMessage)
		if ok {
			assert.True(t, ok)
			assert.NotNil(t, requestMessage.Request())
			assert.Equal(t, "websocket", requestMessage.Headers().Get("Upgrade"))

			assert.Equal(t, "Hello, world!", string(exchange.In.Body()))
			assert.Equal(t, "Hello, world!", string(exchange.In.GetMsg().GetData()))

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

	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = exchange.In.GetParam("msgType")
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).To("chain:default").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("规则链执行结果：" + exchange.Out.GetMsg().GetData() + "\n"))
		return true
	}).End()

	//注册路由
	wsEndpoint.AddRouter(router1)

	assert.NotNil(t, wsEndpoint.Router())
	return wsEndpoint
}

// wsFreePort reserves an ephemeral port and releases it for the test server.
func wsFreePort(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// Metadata methods: category, definition, instance and id.
func TestWsMetaMethods(t *testing.T) {
	ws := &Websocket{}
	assert.Equal(t, "endpoint", ws.Category())
	def := ws.Def()
	assert.True(t, def.Desc != "")
	assert.NotNil(t, def.RouterForm)
	assert.NotNil(t, def.RouterForm.From)

	instance, err := ws.GetInstance()
	assert.Nil(t, err)
	assert.True(t, instance == ws)

	ws.Config.Server = ":7777"
	assert.Equal(t, ":7777", ws.Id())
}

// Request/Response message unit branches.
func TestWsMessageUnitBranches(t *testing.T) {
	request := &RequestMessage{}
	request.SetStatusCode(500)
	assert.Nil(t, request.Headers())
	assert.Equal(t, "", request.From())
	assert.Equal(t, "", request.GetParam("k"))
	assert.Nil(t, request.Request())

	req := httptest.NewRequest(http.MethodGet, "/api?aa=xx", nil)
	request = &RequestMessage{request: req, Params: httprouter.Params{{Key: "id", Value: "42"}}}
	assert.Equal(t, "/api?aa=xx", request.From())
	assert.Equal(t, "42", request.GetParam("id"))
	assert.Equal(t, "xx", request.GetParam("aa"))
	assert.NotNil(t, request.Headers())
	assert.NotNil(t, request.Request())
	// Binary frames map to BINARY data type.
	binReq := &RequestMessage{request: req, body: []byte{0x01}, messageType: websocket.BinaryMessage}
	assert.Equal(t, types.BINARY, binReq.GetMsg().GetDataType())

	response := &ResponseMessage{}
	response.SetStatusCode(500)
	// No sender: body is stored without writing and without error.
	response.SetBody([]byte("stored"))
	assert.Equal(t, "stored", string(response.Body()))
	assert.Nil(t, response.GetError())
	assert.Equal(t, "", response.From())
	assert.Equal(t, "", response.GetParam("k"))
	assert.NotNil(t, response.Headers())
}

// wsSender guards against nil connections and defaults to text frames.
func TestWsSenderUnits(t *testing.T) {
	sender := &wsSender{}
	assert.NotNil(t, sender.Send([]byte("x")))
	assert.NotNil(t, sender.SendWithType([]byte("x"), websocket.TextMessage))
	assert.Nil(t, sender.Close())
}

// wsSender over a real connection: Send writes frames and Close shuts down.
func TestWsSenderLiveConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}

	sender := &wsSender{conn: conn}
	assert.Nil(t, sender.Send([]byte("text-frame")))
	assert.Nil(t, sender.SendWithType([]byte{0x01}, websocket.BinaryMessage))
	assert.Nil(t, sender.Close())
	// After close, writes fail.
	assert.NotNil(t, sender.Send([]byte("x")))
}

// Session addressing: SendToTarget pushes to the session extracted from the
// first frame; unknown targets error.
func TestWsSendToTarget(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	addr := wsFreePort(t)

	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{
		"server":     addr,
		"sessionKey": "${msg.deviceId}",
	}))
	router := impl.NewRouter().From("/ws").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		return true
	}).End()
	_, err := ep.AddRouter(router)
	assert.Nil(t, err)

	var eventMu sync.Mutex
	var events []string
	ep.SetOnEvent(func(event string, params ...interface{}) {
		eventMu.Lock()
		defer eventMu.Unlock()
		events = append(events, event)
	})
	eventsInclude := func(name string) bool {
		eventMu.Lock()
		defer eventMu.Unlock()
		for _, e := range events {
			if e == name {
				return true
			}
		}
		return false
	}

	assert.Nil(t, ep.Start())
	defer ep.Destroy()
	assert.True(t, eventsInclude(endpoint.EventInitServer))

	url := "ws://" + addr + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"deviceId":"DEV_9"}`)); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(ep.Lookup("DEV_9")) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if len(ep.Lookup("DEV_9")) != 1 {
		t.Fatal("session DEV_9 not registered")
	}
	assert.True(t, eventsInclude(endpoint.EventConnect))

	sent, failed, err := ep.SendToTarget("DEV_9", []byte("PUSH"))
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)
	assert.Nil(t, err)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, websocket.TextMessage, mt)
	assert.Equal(t, "PUSH", string(data))

	_, _, err = ep.SendToTarget("NO_SUCH_TARGET", []byte("x"))
	assert.NotNil(t, err)

	// Disabling the router closes active connections on the next frame.
	_ = ep.RemoveRouter(router.GetId())
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("again")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break // server closed the connection after the router was disabled
		}
	}
}
