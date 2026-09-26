package rest

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
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

// 测试请求/响应消息
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
	//创建rest endpoint服务
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

	//删除路由
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
	//启动服务
	go startServer(t, stop, &wg)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)

	config := engine.NewConfig(types.WithDefaultPool())
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, "ok", msg.GetData())
	})
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, "{\"name\":\"lala\"}")

	sendMsg(t, "http://127.0.0.1"+testServer+"/api/v1/msg2Chain2/TEST_MSG_TYPE1?aa=xx", "POST", msg1, ctx)
	time.Sleep(time.Millisecond * 500)
	//停止服务器
	stop <- struct{}{}
	time.Sleep(time.Millisecond * 200)
	wg.Wait()
}

// 发送消息到rest服务器
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
	//发送消息
	node.OnMsg(ctx, msg)
	return node
}

// 启动rest服务
func startServer(t *testing.T, stop chan struct{}, wg *sync.WaitGroup) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Error(err)
		wg.Done()
		return
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
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
			ReadTimeout:  10, // 默认10秒
			WriteTimeout: 10, // 默认10秒
			IdleTimeout:  60, // 默认60秒
		},
	}, restEndpoint.New()))

	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//设置跨域
	restEndpoint.GlobalOPTIONS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			// 设置 CORS 相关的响应头
			header := w.Header()
			header.Set("Access-Control-Allow-Methods", r.Header.Get("Allow"))
			header.Set("Access-Control-Allow-Headers", "*")
			header.Set("Access-Control-Allow-Origin", "*")
		}
		// 返回 204 状态码
		w.WriteHeader(http.StatusNoContent)
	}))
	//路由1
	router1 := impl.NewRouter().From("/api/v1/hello/:name").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//处理请求
		request, ok := exchange.In.(*RequestMessage)
		if ok {
			if request.request.Method != http.MethodGet {
				//响应错误
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
				//不执行后续动作
				return false
			} else {
				//响应请求
				exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
				exchange.Out.SetBody([]byte(exchange.In.From() + "\n"))
				exchange.Out.SetBody([]byte("s1 process" + "\n"))
				name := request.GetMsg().Metadata.GetValue("name")
				if name == "break" {
					//不执行后续动作
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

	//路由2 采用配置方式调用规则链
	router2 := impl.NewRouter().From("/api/v1/msg2Chain1/:msgType").To("chain:default").End()

	//路由3 采用配置方式调用规则链,to路径带变量
	router3 := impl.NewRouter().From("/api/v1/msg2Chain2/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")

		//从header获取用户ID
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		//把userId存放在msg元数据
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
		//响应给客户端
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

	//路由4 直接调用node组件方式
	router4 := impl.NewRouter().From("/api/v1/msgToComponent1/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).ToComponent(func() types.Node {
		//定义日志组件，处理数据
		var configuration = make(types.Configuration)
		configuration["jsScript"] = `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
       `
		logNode := &action.LogNode{}
		_ = logNode.Init(config, configuration)
		return logNode
	}()).End()

	//路由5 采用配置方式调用node组件
	router5 := impl.NewRouter().From("/api/v1/msgToComponent2/:msgType").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("component:log", types.Configuration{"jsScript": `
		return 'log::Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);
       `}).End()

	//注册路由,Get 方法
	_, _ = restEndpoint.AddRouter(router1, "GET")
	//注册路由，POST方式
	_, _ = restEndpoint.AddRouter(router2, "POST")
	_, _ = restEndpoint.AddRouter(router3, "POST")
	_, _ = restEndpoint.AddRouter(router4, "POST")
	_, _ = restEndpoint.AddRouter(router5, "POST")

	assert.NotNil(t, restEndpoint.Router)
	//启动服务
	err = restEndpoint.Start()
	//fmt.Println(err)
	<-stop
	restEndpoint.Destroy()
	wg.Done()
}

// freePort reserves an ephemeral port and releases it for the test server.
func freePort(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// Metadata methods and path conversion.
func TestRestMetaMethods(t *testing.T) {
	ep := &Endpoint{}
	assert.Equal(t, "endpoint", ep.Category())
	def := ep.Def()
	assert.True(t, def.Desc != "")
	assert.NotNil(t, def.RouterForm)
	assert.NotNil(t, def.RouterForm.From)

	assert.Equal(t, ":6333", ep.New().(*Rest).Config.Server)

	assert.Equal(t, "/api/device/:id", ep.convertPathParams("/api/device/{id}"))
	assert.Equal(t, "/api/files/*filepath", ep.convertPathParams("/api/files/*filepath"))
	assert.Equal(t, "/plain", ep.convertPathParams("/plain"))
}

// Request/Response message branches driven by httptest requests.
func TestRestMessageUnitBranches(t *testing.T) {
	t.Run("RequestNil", func(t *testing.T) {
		request := &RequestMessage{}
		request.SetStatusCode(500)
		assert.Nil(t, request.Headers())
		assert.Equal(t, "", request.From())
		assert.Equal(t, "", request.GetParam("k"))
		assert.Nil(t, request.Request())
		assert.Nil(t, request.Response())
	})

	t.Run("RequestGet", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api?aa=xx&bb=1&bb=2", nil)
		request := &RequestMessage{
			request:  req,
			response: httptest.NewRecorder(),
			Params:   httprouter.Params{{Key: "id", Value: "42"}},
		}
		assert.Equal(t, "/api?aa=xx&bb=1&bb=2", request.From())
		// Path parameter wins over query string.
		assert.Equal(t, "42", request.GetParam("id"))
		assert.Equal(t, "xx", request.GetParam("aa"))
		msg := request.GetMsg()
		assert.Equal(t, types.JSON, msg.GetDataType())
		// Query is stringified; assert on stable fragments only.
		assert.True(t, strings.Contains(msg.GetData(), "xx"))
		assert.True(t, strings.Contains(msg.GetData(), "bb"))
		assert.NotNil(t, request.Request())
		assert.NotNil(t, request.Response())
	})

	t.Run("RequestPostJson", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{"a":1}`))
		req.Header.Set(ContentTypeKey, JsonContextType)
		request := &RequestMessage{request: req}
		msg := request.GetMsg()
		assert.Equal(t, types.JSON, msg.GetDataType())
		assert.Equal(t, `{"a":1}`, msg.GetData())
		// Body is cached after the first read.
		assert.Equal(t, `{"a":1}`, string(request.Body()))
	})

	t.Run("RequestPostText", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("plain"))
		request := &RequestMessage{request: req}
		msg := request.GetMsg()
		assert.Equal(t, types.TEXT, msg.GetDataType())
		assert.Equal(t, "plain", msg.GetData())
	})

	t.Run("ResponseNilWriter", func(t *testing.T) {
		response := &ResponseMessage{}
		assert.Nil(t, response.Headers())
		assert.Equal(t, "", response.From())
		assert.Equal(t, "", response.GetParam("k"))
		assert.Nil(t, response.Request())
		assert.Nil(t, response.Response())
		// Mutators on a nil writer are no-ops.
		response.AddHeader("a", "b")
		response.SetHeader("a", "b")
		response.DelHeader("a")
		// Flush with no writer must not panic.
		response.Flush()
	})

	t.Run("ResponseWithRecorder", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api?aa=xx", nil)
		response := &ResponseMessage{request: req, response: recorder}
		assert.Equal(t, "/api?aa=xx", response.From())
		assert.Equal(t, "xx", response.GetParam("aa"))
		assert.NotNil(t, response.Headers())
		assert.NotNil(t, response.Request())
		assert.NotNil(t, response.Response())
		// Empty body must not call Write: the recorder body stays empty.
		response.SetBody([]byte{})
		assert.Equal(t, "", recorder.Body.String())
		// Flush delegates to the recorder's Flusher.
		response.Flush()
	})
}

// CORS: the AllowCors config installs a GlobalOPTIONS preflight handler and an
// origin header interceptor for normal requests.
func TestRestCors(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{"server": freePort(t), "allowCors": true}))
	defer ep.Destroy()

	var hit bool
	router := impl.NewRouter().From("/cors").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		hit = true
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).End()
	_, err := ep.AddRouter(router, "GET")
	assert.Nil(t, err)

	// Preflight.
	preflight := httptest.NewRequest(http.MethodOptions, "/cors", nil)
	preflight.Header.Set(HeaderKeyAccessControlRequestMethod, http.MethodGet)
	preflightRecorder := httptest.NewRecorder()
	ep.Router().ServeHTTP(preflightRecorder, preflight)
	assert.Equal(t, http.StatusNoContent, preflightRecorder.Code)
	assert.Equal(t, "*", preflightRecorder.Header().Get(HeaderKeyAccessControlAllowOrigin))

	// Normal request carries the CORS origin header via the interceptor.
	get := httptest.NewRequest(http.MethodGet, "/cors", nil)
	getRecorder := httptest.NewRecorder()
	ep.Router().ServeHTTP(getRecorder, get)
	assert.True(t, hit)
	assert.Equal(t, "*", getRecorder.Header().Get(HeaderKeyAccessControlAllowOrigin))
	assert.Equal(t, "ok", getRecorder.Body.String())
}

// Disabled routers return 404; a malformed route path surfaces the panic as
// an error from AddRouter.
func TestRestHandlerDisabledAndPanic(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{"server": freePort(t)}))
	defer ep.Destroy()

	router := impl.NewRouter().From("/gone").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("never"))
		return true
	}).End()
	routerId, err := ep.AddRouter(router, "GET")
	assert.Nil(t, err)
	assert.Nil(t, ep.RemoveRouter(routerId))

	recorder := httptest.NewRecorder()
	ep.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/gone", nil))
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	// A path without a leading slash makes httprouter panic; AddRouter recovers.
	bad := impl.NewRouter().From("no-slash").End()
	_, err = ep.AddRouter(bad, "GET")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "addRouter err"))
}

// Server lifecycle on an ephemeral port: Start/Started/GetServer/Restart/Close.
func TestRestServerLifecycle(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	addr := freePort(t)

	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{"server": addr, "readTimeout": 5, "writeTimeout": 5, "idleTimeout": 5, "disableKeepalive": true}))
	// Before Start there is no server yet.
	assert.False(t, ep.Started())
	assert.Nil(t, ep.GetServer())

	router := impl.NewRouter().From("/echo").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("v1"))
		return true
	}).End()
	_, err := ep.AddRouter(router, "GET")
	assert.Nil(t, err)
	// POST helper registers with the POST method.
	postRouter := impl.NewRouter().From("/echo").Process(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("posted"))
		return true
	}).End()
	_, _ = ep.AddRouter(postRouter, "POST")

	assert.Nil(t, ep.Start())
	assert.True(t, ep.Started())
	server := ep.GetServer()
	assert.NotNil(t, server)
	assert.Equal(t, 5*time.Second, server.ReadTimeout)
	assert.Equal(t, 5*time.Second, server.WriteTimeout)
	assert.Equal(t, 5*time.Second, server.IdleTimeout)

	getBody := func() string {
		resp, err := http.Get("http://" + addr + "/echo")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}
	assert.Equal(t, "v1", getBody())

	resp, err := http.Post("http://"+addr+"/echo", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	// Restart keeps serving (routers re-registered, body version unchanged).
	assert.Nil(t, ep.Restart())
	assert.True(t, ep.Started())
	assert.Equal(t, "v1", getBody())

	// A second instance with the same plain address keeps its own (nil) server:
	// sharing only happens through "@instanceId" style addresses.
	ep2 := &Endpoint{}
	assert.Nil(t, ep2.Init(config, types.Configuration{"server": addr}))
	assert.Nil(t, ep2.GetServer())

	assert.Nil(t, ep.Close())
	assert.False(t, ep.Started())
	ep.Destroy()
	ep2.Destroy()
}

// Static file mappings serve directory contents and survive a restart.
func TestRestRegisterStaticFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "index.html")
	if err := os.WriteFile(file, []byte("static content"), 0o600); err != nil {
		t.Fatal(err)
	}

	config := engine.NewConfig(types.WithDefaultPool())
	addr := freePort(t)
	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{"server": addr}))
	// Two mappings exercise the multi-entry split; entries without "=" are ignored.
	ep.RegisterStaticFiles("/static=" + dir + ",/files=" + dir + ",ignored-entry")
	router := impl.NewRouter().From("/api").End()
	_, _ = ep.AddRouter(router, "GET")
	assert.Nil(t, ep.Start())
	defer ep.Destroy()

	fetch := func(url string) (int, string) {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}
	code, body := fetch("http://" + addr + "/static/index.html")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "static content", body)

	code, body = fetch("http://" + addr + "/files/index.html")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "static content", body)

	// Restart re-registers the resource mapping.
	assert.Nil(t, ep.Restart())
	code, body = fetch("http://" + addr + "/static/index.html")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "static content", body)
}

// Init fails on configuration values that cannot map onto Config.
func TestRestInitError(t *testing.T) {
	ep := &Endpoint{}
	err := ep.Init(engine.NewConfig(), types.Configuration{"readTimeout": "not-a-number"})
	assert.NotNil(t, err)
}

// Listen falls back to the default HTTP/HTTPS ports when the address is empty.
// Binding the default port may fail in restricted environments; the fallback
// branch is covered either way.
func TestRestListenDefaultAddr(t *testing.T) {
	ep := &Rest{Server: &http.Server{}}
	if ln, err := ep.Listen(); err == nil {
		ln.Close()
	}
	epTLS := &Rest{Server: &http.Server{}, Config: Config{CertFile: "c.pem", CertKeyFile: "k.pem"}}
	if ln, err := epTLS.Listen(); err == nil {
		ln.Close()
	}
}

// A configuration with certificate files drives the TLS branch of startServer
// (ServeTLS fails on the bogus files but the branch and events still run).
func TestRestStartTLSBranch(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ep := &Endpoint{}
	assert.Nil(t, ep.Init(config, types.Configuration{
		"server":      freePort(t),
		"certFile":    "no-such-cert.pem",
		"certKeyFile": "no-such-key.pem",
	}))
	var events []string
	ep.SetOnEvent(func(event string, params ...interface{}) {
		events = append(events, event)
	})
	assert.Nil(t, ep.Start())
	time.Sleep(300 * time.Millisecond)
	assert.True(t, ep.Started())
	ep.Destroy()
	assert.False(t, ep.Started())
}
