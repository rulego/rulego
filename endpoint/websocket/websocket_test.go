package websocket

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
var testServer = ":9090"
var testConfigServer = ":9091"

// Test request/response messages
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
	//Create an endpoint service
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

	//Delete the route
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
	//Start the server
	go startServer(t, stop, &wg, false)
	//Wait for the server to start up
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "ws://127.0.0.1"+testServer+"/api/v1/echo/TEST_MSG_TYPE1?aa=xx")
	//Stop the server
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

func TestMultiplexRestEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	//Start the server
	go startServer(t, stop, &wg, true)
	//Wait for the server to start up
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "ws://127.0.0.1"+testServer+"/api/v1/echo/TEST_MSG_TYPE1?aa=xx")
	time.Sleep(time.Millisecond * 200)
	//Stop the server
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

// Send a message to the REST server
func sendMsg(t *testing.T, url string) {

	// Connect to the WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		time.Sleep(time.Millisecond * 200)
		conn.Close()
	}()

	// Send the message
	err = conn.WriteMessage(websocket.BinaryMessage, []byte("Hello, world!"))
	if err != nil {
		t.Fatal(err)
	}

	// Read the message
	_, p, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "ok", string(p))

}

// Start the server
func startServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup, isMultiplex bool) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//Register the rule chain
	_, _ = engine.New("default", buf, engine.WithConfig(config))
	var wsEndpoint endpoint.Endpoint
	restEndpoint := &rest.Endpoint{
		Config: rest.Config{Server: testServer},
	}
	//Resume using REST endpoint
	if isMultiplex {
		wsEndpoint = newWebsocketServe(t, restEndpoint)
		if err := wsEndpoint.Start(); err != nil {
			t.Fatal("error:", err)
		}
	} else {
		wsEndpoint = newWebsocketServe(t, nil)
	}

	if isMultiplex {
		//Resume using REST endpoint
		_ = restEndpoint.Start()
	} else {
		//And launch the service
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
		t.Fatal(err)
	}

	assert.Equal(t, Type, wsEndpoint.Type())
	assert.True(t, reflect.DeepEqual(&Websocket{
		Config: Config{Config: rest.Config{Server: ":6334", AllowCors: true}, SessionTTL: 1800},
	}, wsEndpoint.New()))

	if restEndpoint != nil {
		wsEndpoint = &Websocket{Rest: restEndpoint, Config: Config{Config: rest.Config{AllowCors: true}}}
	}
	//Added a global interceptor
	wsEndpoint.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Permission validation logic
		return true
	})
	//Route 1
	router1 := impl.NewRouter().From("/api/v1/echo/:msgType").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Processing requests
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
				//Response errors
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//Do not perform subsequent actions
				return false
			} else {
				//Responding to requests
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte("ok"))
				name := requestMessage.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//Do not perform subsequent actions
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

	//Register the route
	wsEndpoint.AddRouter(router1)

	assert.NotNil(t, wsEndpoint.Router())
	return wsEndpoint
}
