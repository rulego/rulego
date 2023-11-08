package net

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"os"
	"testing"
)

var testdataFolder = "../../testdata"

func TestEndpoint(t *testing.T) {
	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//启动tpc服务
	ep := &Endpoint{
		Config: Config{
			Protocol: "tcp",
			Addr:     ":8888",
		},
		RuleConfig: config,
	}
	//添加全局拦截器
	ep.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//匹配所有消息，转发到该路由处理
	router1 := endpoint.NewRouter().From("").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		t.Logf("router1 receive data:%s,from:%s", exchange.In.GetMsg().Data, exchange.In.From())
		return true
	}).To("chain:default").End()

	//匹配与{开头的消息，转发到该路由处理
	router2 := endpoint.NewRouter().From("^{.*").Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		t.Logf("router2 receive data:%s,from:%s", exchange.In.GetMsg().Data, exchange.In.From())
		return true
	}).To("chain:default").End()

	//注册路由并启动服务
	_, err = ep.AddRouterWithParams(router1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ep.AddRouterWithParams(router2)
	if err != nil {
		t.Fatal(err)
	}
	_ = ep.Start()

}
