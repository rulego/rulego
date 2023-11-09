package mqtt

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

var testdataFolder = "../../testdata"

func TestMqttEndpoint(t *testing.T) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	buf, err := os.ReadFile(testdataFolder + "/chain_call_rest_api.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//创建mqtt endpoint服务
	ep, err := endpoint.New(Type, config, mqtt.Config{
		Server: "127.0.0.1:1883",
	})

	//添加全局拦截器
	ep.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//订阅所有主题路由，并转发到default规则链处理
	router1 := endpoint.NewRouter().From("#").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		t.Logf("receive data:%s,topic:%s", exchange.In.GetMsg().Data, exchange.In.GetMsg().Metadata.GetValue("topic"))
		return true
	}).To("chain:default").End()
	//注册路由
	_, err = ep.AddRouter(router1)
	if err != nil {
		t.Fatal(err)
	}
	//启动服务
	err = ep.Start()
	if err != nil {
		t.Fatal(err)
	}
	<-c
}
