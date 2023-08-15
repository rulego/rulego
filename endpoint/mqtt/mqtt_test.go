package mqtt

import (
	"fmt"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/test/assert"
	"os"
	"os/signal"
	"testing"
)

func TestMessage(t *testing.T) {
}

func TestMqttEndpoint(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c)

	//启动mqtt接收端服务
	mqttEndpoint := &Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
	}
	//订阅所有主题路由，并转发到default规则链处理
	router1 := endpoint.NewRouter().From("#").To("chain:default").End()
	//注册路由并启动服务
	_ = mqttEndpoint.AddRouter(router1).Start()

	<-c
}

func TestMqttEndpoint2(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c)

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
