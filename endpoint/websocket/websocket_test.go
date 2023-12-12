package websocket

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"net/http"
	"os"
	"testing"
)

var testdataFolder = "../../testdata"

func TestWebSocketEndPoint(t *testing.T) {

	buf, err := os.ReadFile(testdataFolder + "/chain_msg_type_switch.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

	//启动ws接收服务
	wsEndpoint, err := endpoint.New(Type, config, Config{Server: ":9090"})
	//添加全局拦截器
	wsEndpoint.AddInterceptors(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//权限校验逻辑
		return true
	})
	//路由1
	router1 := endpoint.NewRouter().From("/api/v1/echo/:msgType").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
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

	}).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.In.GetMsg().Type = exchange.In.GetParam("msgType")

		exchange.Out.SetBody([]byte("s2 process" + "\n"))
		return true
	}).To("chain:default").Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("规则链执行结果：" + exchange.Out.GetMsg().Data + "\n"))
		return true
	}).End()

	//注册路由
	wsEndpoint.AddRouter(router1)

	////并启动服务
	//_ = wsEndpoint.Start()
}
