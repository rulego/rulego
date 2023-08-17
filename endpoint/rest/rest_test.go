package rest

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint"
	"net/http"
	"os"
	"testing"
)

var testdataFolder = "../../testdata"

func TestRestEndPoint(t *testing.T) {

	buf, err := os.ReadFile(testdataFolder + "/chain_call_rest_api.json")
	if err != nil {
		t.Fatal(err)
	}
	config := rulego.NewConfig(types.WithDefaultPool())
	//注册规则链
	_, _ = rulego.New("default", buf, rulego.WithConfig(config))

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

	//路由4 处理请求，并转换，然后转发给组件
	router4 := endpoint.NewRouter().From("/api/v1/msgToComponent/:msgType").Transform(func(exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		//获取消息类型
		msg.Type = msg.Metadata.GetValue("msgType")
		return true
	}).Process(func(exchange *endpoint.Exchange) bool {
		//响应给客户端
		exchange.Out.Headers().Set("Content-Type", "application/json")
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

	//注册路由
	restEndpoint.POST(router2, router3, router4)
	//并启动服务
	_ = restEndpoint.Start()
}
