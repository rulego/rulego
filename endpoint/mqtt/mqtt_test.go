package mqtt

import (
	"fmt"
	"github.com/rulego/rulego/api/types"
	endpoint "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/maps"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	testdataFolder = "../../testdata/rule"
	testServer     = "127.0.0.1:1883"
	msgContent1    = "{\"test\":\"AA\"}"
	msgContent2    = "{\"test\":\"BB\"}"
)

// 测试请求/响应消息
func TestMqttMessage(t *testing.T) {
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
	//创建mqtt endpoint服务
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&mqtt.Config{
		Server: testServer,
	}, nodeConfig)
	var ep = &Endpoint{}
	err := ep.Init(config, nodeConfig)
	assert.Nil(t, err)
	assert.Equal(t, testServer, ep.Id())
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

func TestMqttEndpoint(t *testing.T) {
	stop := make(chan struct{})
	//启动服务
	go startServer(t, stop)
	//等待服务器启动完毕
	time.Sleep(time.Millisecond * 200)
	//启动客户端
	node := createClient(t)
	config := types.NewConfig()
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, types.Success, relationType)
	})
	//发送消息
	metaData := types.BuildMetadata(make(map[string]string))
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", metaData, msgContent1)
	node.OnMsg(ctx, msg1)
	msg2 := ctx.NewMsg("TEST_MSG_TYPE_BB", metaData, msgContent2)
	node.OnMsg(ctx, msg2)
	time.Sleep(time.Millisecond * 200)
	close(stop)
}

func createClient(t *testing.T) types.Node {
	node, _ := engine.Registry.NewNode("mqttClient")
	var configuration = make(types.Configuration)
	configuration["Server"] = "127.0.0.1:1883"
	configuration["Topic"] = "/device/msg"

	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	return node
}

// 启动服务
func startServer(t *testing.T, stop chan struct{}) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := engine.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = engine.New("default", buf, engine.WithConfig(config))

	//创建mqtt endpoint服务
	var nodeConfig = make(types.Configuration)
	_ = maps.Map2Struct(&mqtt.Config{
		Server: testServer,
	}, nodeConfig)
	var ep = &Endpoint{}
	err = ep.Init(config, nodeConfig)
	assert.Equal(t, testServer, ep.Id())
	assert.Equal(t, "mqtt", ep.Type())
	assert.True(t, reflect.DeepEqual(&Mqtt{}, ep.New()))

	//添加全局拦截器
	ep.AddInterceptors(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//订阅所有主题路由，并转发到default规则链处理
	router1 := impl.NewRouter().From("/device/msg").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		requestMessage := exchange.In.(*RequestMessage)
		responseMessage := exchange.Out.(*ResponseMessage)
		topic := "/device/msg"
		assert.Equal(t, topic, exchange.In.GetMsg().Metadata.GetValue("topic"))
		assert.Equal(t, topic, requestMessage.From())
		assert.Equal(t, topic, requestMessage.Headers().Get("topic"))
		assert.Equal(t, topic, responseMessage.From())
		assert.NotNil(t, requestMessage.Request())

		receiveData := exchange.In.GetMsg().Data
		assert.Equal(t, receiveData, string(requestMessage.Body()))

		if receiveData != msgContent1 && receiveData != msgContent2 {
			t.Fatalf("receive data:%s,expect data:%s,%s", receiveData, msgContent1, msgContent2)
		}
		responseMessage.SetStatusCode(0)
		responseMessage.Headers().Set("topic", "/device/msg/resp")
		responseMessage.Headers().Set("qos", "0")
		responseMessage.SetBody([]byte("ok"))
		assert.NotNil(t, responseMessage.Response())
		return true
	}).To("chain:default").End()
	//注册路由
	_, err = ep.AddRouter(router1)

	if err != nil {
		t.Fatal(err)
	}
	ep.AddRouter(nil)

	//启动服务
	_ = ep.Start()

	//服务启动后，继续添加路由
	routerId, err := ep.AddRouter(impl.NewRouter().From("/device/#").End())
	assert.Equal(t, "/device/#", routerId)

	ep.RemoveRouter(routerId)

	ep.Printf("start server")
	<-stop
	ep.Destroy()
}
