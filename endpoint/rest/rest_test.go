package rest

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
)

var testdataFolder = "../../testdata/rule"
var testServer = ":9090"
var testConfigServer = ":9091"

type countingResponseWriter struct {
	header           http.Header
	writeHeaderCount int
	statusCode       int
	body             []byte
}

// Header returns the mutable header map used by the test response writer.
func (w *countingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

// Write appends response bytes so tests can assert the final body content.
func (w *countingResponseWriter) Write(body []byte) (int, error) {
	w.body = append(w.body, body...)
	return len(body), nil
}

// WriteHeader records status code writes for repeated-header assertions.
func (w *countingResponseWriter) WriteHeader(statusCode int) {
	w.writeHeaderCount++
	w.statusCode = statusCode
}

type panicResponseWriter struct {
	header http.Header
}

// Header returns the mutable header map used by the panic response writer.
func (w *panicResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

// Write simulates a closed client connection by panicking on writes.
func (w *panicResponseWriter) Write(body []byte) (int, error) {
	panic("writer closed")
}

// WriteHeader simulates a closed client connection by panicking on header writes.
func (w *panicResponseWriter) WriteHeader(statusCode int) {
	panic("writer closed")
}

// Test request/response messages
func TestRestMessage(t *testing.T) {
	t.Run("Request", func(t *testing.T) {
		var request = &RequestMessage{}
		test.EndpointMessage(t, request)
	})
	t.Run("Response", func(t *testing.T) {
		var response = &ResponseMessage{}
		test.EndpointMessage(t, response)
	})
}

func TestResponseMessageSetStatusCodeWritesHeaderOnce(t *testing.T) {
	writer := &countingResponseWriter{}
	response := &ResponseMessage{
		response: writer,
	}

	response.SetStatusCode(http.StatusBadRequest)
	response.SetStatusCode(http.StatusBadRequest)
	response.SetBody([]byte(`{"error":"bad request"}`))

	assert.Equal(t, 1, writer.writeHeaderCount)
	assert.Equal(t, http.StatusBadRequest, writer.statusCode)
	assert.Equal(t, `{"error":"bad request"}`, string(writer.body))
}

// TestResponseMessageHeaderModifierMethods verifies REST responses implement header mutation APIs used by streaming processors.
func TestResponseMessageHeaderModifierMethods(t *testing.T) {
	writer := &countingResponseWriter{}
	response := &ResponseMessage{
		response: writer,
	}

	headerModifier, ok := interface{}(response).(endpoint.HeaderModifier)
	assert.True(t, ok)

	headerModifier.SetHeader("Content-Type", "text/event-stream")
	headerModifier.AddHeader("X-Test", "value1")
	headerModifier.AddHeader("X-Test", "value2")
	headerModifier.DelHeader("X-Remove")

	metadata := headerModifier.GetMetadata()
	metadata.PutValue("stream", "true")

	assert.Equal(t, "text/event-stream", writer.Header().Get("Content-Type"))
	assert.Equal(t, 2, len(writer.Header().Values("X-Test")))
	assert.Equal(t, "true", metadata.GetValue("stream"))
}

// TestResponseMessageSetBodyDoesNotPanicOnClosedWriter verifies closed client connections are converted into response errors instead of panics.
func TestResponseMessageSetBodyDoesNotPanicOnClosedWriter(t *testing.T) {
	response := &ResponseMessage{
		response: &panicResponseWriter{},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SetBody should not panic, got: %v", r)
		}
	}()

	response.SetBody([]byte("stream chunk"))

	assert.Equal(t, "stream chunk", string(response.Body()))
	assert.NotNil(t, response.GetError())
}

func TestRouterId(t *testing.T) {
	config := types.NewConfig()
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Server: testServer,
	}, &nodeConfig)
	var ep = &Endpoint{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
	assert.Equal(t, testServer, ep.Id())
	router := impl.NewRouter().SetId("r1").From("/device/info").End()
	routerId, _ := ep.AddRouter(router, "GET")
	assert.Equal(t, "r1", routerId)

	router = impl.NewRouter().From("/device/info").End()
	routerId, _ = ep.AddRouter(router, "POST")
	assert.Equal(t, "POST:/device/info", routerId)

	err = ep.RemoveRouter("r1")
	assert.Nil(t, err)
	err = ep.RemoveRouter("POST:/device/info")
	assert.Nil(t, err)
	err = ep.RemoveRouter("GET:/device/info")
	assert.Equal(t, fmt.Sprintf("router: %s not found", "GET:/device/info"), err.Error())
}

func TestRestEndpointConfig(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	//Create a REST Endpoint service
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&Config{
		Server: testConfigServer,
	}, &nodeConfig)
	var epStarted = &Endpoint{}
	err := epStarted.Init(config, nodeConfig)

	assert.Equal(t, testConfigServer, epStarted.Id())
	err = epStarted.Start()
	assert.Nil(t, err)

	time.Sleep(time.Millisecond * 200)

	var epErr = &Endpoint{}
	err = epErr.Init(config, nodeConfig)

	_, err = epErr.AddRouter(nil, "POST")
	assert.Equal(t, "router can not nil", err.Error())

	restEndpoint := &Endpoint{}
	err = restEndpoint.Init(config, nodeConfig)

	assert.Equal(t, testConfigServer, restEndpoint.Id())
	//_, err := ep.AddRouter(nil)
	//assert.Equal(t, "router can not nil", err.Error())
	testUrl := "/api/test"
	router := impl.NewRouter().From(testUrl).End()
	_, err = restEndpoint.AddRouter(router)
	assert.Equal(t, "need to specify HTTP method", err.Error())

	router = impl.NewRouter().From(testUrl).End()
	routerId, err := restEndpoint.AddRouter(router, "POST")
	assert.Equal(t, "POST:/api/test", routerId)

	//restEndpoint, ok := ep.(*Rest)
	//assert.True(t, ok)

	router = impl.NewRouter().From(testUrl).End()
	//restEndpoint.POST(router)
	restEndpoint.GET(router)
	restEndpoint.DELETE(router)
	restEndpoint.PATCH(router)
	restEndpoint.OPTIONS(router)
	restEndpoint.HEAD(router)
	restEndpoint.PUT(router)

	//Delete the route
	restEndpoint.RemoveRouter(routerId)
	restEndpoint.RemoveRouter(routerId, "POST")

	epStarted.Destroy()
	epErr.Destroy()
	time.Sleep(time.Millisecond * 200)
}

func TestRestEndpoint(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	//Start the server
	go startServer(t, stop, &wg)
	//Wait for the server to start up
	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, "ok", msg.GetData())
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "{\"name\":\"lala\"}")

	sendMsg(t, "http://127.0.0.1"+testServer+"/api/v1/msg2Chain2/TEST_MSG_TYPE1?aa=xx", "POST", msg1, ctx)
	time.Sleep(time.Millisecond * 500)
	//Stop the server
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

// Send a message to the REST server
func sendMsg(t *testing.T, url, method string, msg types.RuleMsg, ctx types.RuleContext) types.Node {
	node, _ := engine.Registry.NewNode("restApiCall")
	var configuration = make(types.Configuration)
	configuration["restEndpointUrlPattern"] = url
	configuration["requestMethod"] = method
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	//Send the message
	node.OnMsg(ctx, msg)
	return node
}

// Start the REST service
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
		Server: testServer,
	}, &nodeConfig)
	var restEndpoint = &Endpoint{}
	err = restEndpoint.Init(config, nodeConfig)
	assert.Equal(t, Type, restEndpoint.Type())
	assert.True(t, reflect.DeepEqual(&Rest{
		Config: Config{
			Server:       ":6333",
			ReadTimeout:  10, // Default is 10 seconds
			WriteTimeout: 10, // Default is 10 seconds
			IdleTimeout:  60, // Default is 60 seconds
		},
	}, restEndpoint.New()))

	//Added a global interceptor
	restEndpoint.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Permission validation logic
		return true
	})
	//Set up cross-domain
	restEndpoint.GlobalOPTIONS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			// Set CORS-related response headers
			header := w.Header()
			header.Set("Access-Control-Allow-Methods", r.Header.Get("Allow"))
			header.Set("Access-Control-Allow-Headers", "*")
			header.Set("Access-Control-Allow-Origin", "*")
		}
		// Return the 204 status code
		w.WriteHeader(http.StatusNoContent)
	}))
	//Route 1
	router1 := impl.NewRouter().From("/api/v1/hello/:name").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Processing requests
		request, ok := exchange.In.(*RequestMessage)
		if ok {
			if request.request.Method != http.MethodGet {
				//Response errors
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//Do not perform subsequent actions
				return false
			} else {
				//Responding to requests
				exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
				exchange.Out.SetBody([]byte(exchange.In.From() + "\n"))
				exchange.Out.SetBody([]byte("s1 process" + "\n"))
				name := request.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//Do not perform subsequent actions
					return false
				} else {
					return true
				}

			}
		} else {
			exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
			exchange.Out.SetBody([]byte(exchange.In.From()))
			exchange.Out.SetBody([]byte("s1 process" + "\n"))
			return true
		}

	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).End()

	//Route 2 calls the rule chain using configuration methods
	router2 := impl.NewRouter().From("/api/v1/msg2Chain1/:msgType").To("chain:default").End()

	//Route 3 calls the rule chain using configuration mode, with the to path with variables
	router3 := impl.NewRouter().From("/api/v1/msg2Chain2/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")

		//Obtain the user ID from the header
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		//Store userId in the msg metadata
		msg.Metadata.PutValue("userId", userId)
		return true
	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		requestMessage, ok := exchange.In.(*RequestMessage)
		assert.True(t, ok)
		assert.NotNil(t, requestMessage.Request())
		assert.Equal(t, JsonContextType, requestMessage.Headers().Get(ContentTypeKey))

		from := requestMessage.From()
		msgType := requestMessage.GetMsg().Metadata.GetValue("msgType")
		assert.Equal(t, "/api/v1/msg2Chain2/"+msgType+"?aa=xx", from)
		assert.Equal(t, "xx", requestMessage.GetParam("aa"))

		responseMessage, ok := exchange.Out.(*ResponseMessage)
		assert.NotNil(t, responseMessage.Response())

		assert.Equal(t, "/api/v1/msg2Chain2/"+msgType+"?aa=xx", responseMessage.From())
		assert.Equal(t, "xx", responseMessage.GetParam("aa"))
		//Respond to the client
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		exchange.Out.SetStatusCode(200)
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("chain:${userId}").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		outMsg := exchange.Out.GetMsg()
		if outMsg != nil {
			assert.Equal(t, true, len(outMsg.Metadata.Values()) > 1)
		}
		return true
	}).End()

	//Routing 4: Direct call to node components
	router4 := impl.NewRouter().From("/api/v1/msgToComponent1/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Respond to the client
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).ToComponent(func() types.Node {
		//Define log components and process data
		var configuration = make(types.Configuration)
		configuration["jsScript"] = `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
       `
		logNode := &action.LogNode{}
		_ = logNode.Init(config, configuration)
		return logNode
	}()).End()

	//Route 5 calls node components using configuration methods
	router5 := impl.NewRouter().From("/api/v1/msgToComponent2/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//Get message types
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//Respond to the client
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
       `}).End()

	//Register a route and get a method
	_, _ = restEndpoint.AddRouter(router1, "GET")
	//Register routing and POST methods
	_, _ = restEndpoint.AddRouter(router2, "POST")
	_, _ = restEndpoint.AddRouter(router3, "POST")
	_, _ = restEndpoint.AddRouter(router4, "POST")
	_, _ = restEndpoint.AddRouter(router5, "POST")

	assert.NotNil(t, restEndpoint.Router)
	//Start the server
	err = restEndpoint.Start()
	//fmt.Println(err)
	<-stop
	restEndpoint.Destroy()
	wg.Done()
}
