package rest

import (
	"github.com/rulego/rulego/endpoint"
	"net/http"
	"testing"
)

func TestRestEndPoint(t *testing.T) {
	//启动http接收服务
	restEndpoint := &Rest{Config: Config{Addr: ":9090"}}

	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/hello/:name").Process(func(exchange *endpoint.Exchange) bool {
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
				exchange.Out.Headers().Set("Content-Type", "application/json")
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
			exchange.Out.Headers().Set("Content-Type", "application/json")
			exchange.Out.SetBody([]byte(exchange.In.From()))
			exchange.Out.SetBody([]byte("s1 process" + "\n"))
			return true
		}

	}).Process(func(exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).End()

	//注册路由
	restEndpoint.GET(router1)

	//路由2 处理请求，并转发到规则引擎处理
	router2 := endpoint.NewRouter().From("/api/v1/msg2/:msgType").To("chain:default").End()

	//路由3 处理请求，并转换，然后转发到规则引擎处理
	router3 := endpoint.NewRouter().From("/api/v1/msg/:msgType").Transform(func(exchange *endpoint.Exchange) bool {
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
	}).Process(func(exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).To("chain:${userId}").End()

	//注册路由
	restEndpoint.POST(router2, router3)
	//并启动服务
	_ = restEndpoint.Start()
}
