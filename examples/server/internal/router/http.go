package router

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/controller"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"net/http"
	"strings"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion
)

// NewRestServe rest服务 接收端点
func NewRestServe(config config.Config) *rest.Endpoint {
	//初始化日志
	addr := config.Server
	logger.Logger.Println("rest serve initialised.addr=" + addr)
	restEndpoint := &rest.Endpoint{
		Config:     rest.Config{Server: addr},
		RuleConfig: rulego.NewConfig(types.WithDefaultPool(), types.WithLogger(logger.Logger)),
	}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.Headers().Set("Content-Type", "application/json")
		exchange.Out.Headers().Set("Access-Control-Allow-Origin", "*")
		return true
	})
	//设置跨域
	restEndpoint.GlobalOPTIONS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			// 设置 CORS 相关的响应头
			header := w.Header()
			header.Set("Access-Control-Allow-Methods", "*")
			header.Set("Access-Control-Allow-Headers", "*")
			header.Set("Access-Control-Allow-Origin", "*")
		}
		// 返回 204 状态码
		w.WriteHeader(http.StatusNoContent)
	}))
	//创建获取所有规则引擎组件列表路由
	restEndpoint.GET(controller.ComponentsRouter(apiBasePath + "/components"))
	//获取所有规则链列表
	restEndpoint.GET(controller.ListDslRouter(apiBasePath + "/rules"))
	//获取规则链DSL
	restEndpoint.GET(controller.GetDslRouter(apiBasePath + "/rule/:chainId"))
	//新增/修改规则链DSL
	restEndpoint.POST(controller.SaveDslRouter(apiBasePath + "/rule/:chainId"))
	//删除规则链
	restEndpoint.DELETE(controller.DeleteDslRouter(apiBasePath + "/rule/:chainId"))
	//保存规则链附加信息
	restEndpoint.POST(controller.SaveBaseInfo(apiBasePath + "/rule/:chainId/saveInfo"))
	//保存规则链配置信息
	restEndpoint.POST(controller.SaveConfiguration(apiBasePath + "/rule/:chainId/saveConfig/:varType"))
	//执行规则链,并得到规则链处理结果
	restEndpoint.POST(controller.ExecuteRuleRouter(apiBasePath + "/rule/:chainId/execute/:msgType"))
	//处理数据上报请求，并转发到规则引擎，不等待规则引擎处理结果
	restEndpoint.POST(controller.PostMsgRouter(apiBasePath + "/rule/:chainId/notify/:msgType"))

	//处理数据上报请求，并转发到规则引擎
	restEndpoint.POST(controller.PostMsgRouter(apiBasePath + "/msg/:chainId/:msgType"))

	//获取节点调试数据
	restEndpoint.GET(controller.GetDebugDataRouter(apiBasePath + "/event/debug"))

	restEndpoint.GET(controller.GetRunsRouter(apiBasePath + "/event/runs"))
	restEndpoint.DELETE(controller.DeleteRunsRouter(apiBasePath + "/event/runs"))

	restEndpoint.POST(controller.TestWebhookRouter(apiBasePath + "/webhook/:integrationType/:username/:chainId"))

	//获取所有共享组件
	restEndpoint.GET(controller.ListNodePool(apiBasePath + "/node_pool/list"))

	//静态文件映射
	loadServeFiles(config, restEndpoint)
	return restEndpoint
}

// 加载静态文件映射
func loadServeFiles(c config.Config, restEndpoint *rest.Endpoint) {
	if c.ResourceMapping != "" {
		mapping := strings.Split(c.ResourceMapping, ",")
		for _, item := range mapping {
			files := strings.Split(item, "=")
			if len(files) == 2 {
				restEndpoint.Router().ServeFiles(strings.TrimSpace(files[0]), http.Dir(strings.TrimSpace(files[1])))
			}
		}
	}
}
