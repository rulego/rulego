package mqtt

import (
	"fmt"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/test/assert"
	"os"
	"os/signal"
	"testing"
)

var testdataFolder = "../../testdata"

func TestMqttEndpoint(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c)

	buf, err := os.ReadFile(testdataFolder + "/chain_call_rest_api.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//启动mqtt接收端服务
	mqttEndpoint := &Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
	}
	//订阅所有主题路由，并转发到default规则链处理
	router1 := endpoint.NewRouter().From("#").Transform(func(exchange *endpoint.Exchange) bool {
		t.Logf("receive data:%s,topic:%s", exchange.In.GetMsg().Data, exchange.In.GetMsg().Metadata.GetValue("topic"))
		return true
	}).To("chain:default").End()
	//注册路由并启动服务
	_ = mqttEndpoint.AddRouter(router1).Start()

	<-c
}

func TestMqttEndpoint2(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c)

	buf, err := os.ReadFile(testdataFolder + "/chain_call_rest_api.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//mqtt 接收数据
	mqttEndpoint := &Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
	}
	//订阅所有主题路由，并转发到default规则链处理
	router1 := endpoint.NewRouter().From("#").Transform(func(exchange *endpoint.Exchange) bool {
		assert.Equal(t, 1, len(exchange.In.Headers()))
		assert.NotEqual(t, "", exchange.In.GetMsg().Data)
		fmt.Println(exchange.In.GetMsg())
		return true
	}).To("chain:default").End()

	//注册路由并启动服务
	_ = mqttEndpoint.AddRouter(router1).Start()

	<-c
}
