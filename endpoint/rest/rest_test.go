package rest

import (
	"github.com/rulego/rulego/endpoint"
	"net/http"
	"testing"
)

func TestRest(t *testing.T) {
	restEndpoint := &Rest{Config: Config{Addr: ":9090"}}

	router1 := endpoint.NewRouter().From("/api/v1/hello/").Process(func(exchange *endpoint.Exchange) {
		//处理请求
		request, ok := exchange.In.(*RequestMessage)
		if ok {
			if request.request.Method != http.MethodGet {
				exchange.Out.SetStatusCode(http.StatusMethodNotAllowed)
			} else {
				//响应请求
				exchange.Out.Headers().Set("Content-Type", "application/json")
				exchange.Out.SetBody([]byte(exchange.In.From()))
			}
		} else {
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(exchange.In.From()))
		}

	}).End()

	//处理请求，并转发到规则引擎
	router2 := endpoint.NewRouter().From("/api/v1/msg/").Transform(func(exchange *endpoint.Exchange) {
		from := exchange.In.From()
		msg := exchange.In.GetMsg()
		//获取消息类型
		msgType := from[len("/api/v1/msg/"):]
		msg.Type = msgType

		//用户ID
		userId := exchange.In.Headers().Get("userId")
		if userId == "" {
			userId = "default"
		}
		msg.Metadata.PutValue("userId", userId)
	}).Process(func(exchange *endpoint.Exchange) {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
	}).To("chain:${userId}").End()

	//注册路由
	_ = restEndpoint.AddRouter(router1, router2).Start()
}
