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

func TestRest(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c)
	mqttEndpoint := &Mqtt{
		Config: mqtt.Config{
			Server: "127.0.0.1:1883",
		},
	}
	//订阅所有主题路由
	router1 := endpoint.NewRouter().From("#").Transform(func(exchange *endpoint.Exchange) {
		assert.Equal(t, 1, len(exchange.In.Headers()))
		assert.NotEqual(t, "", exchange.In.GetMsg().Data)
		fmt.Println(exchange.In.GetMsg())
	}).To("chain:default").End()
	_ = mqttEndpoint.AddRouter(router1).Start()
	<-c
}
