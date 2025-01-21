package router

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/controller"
	"net/http"
	"strings"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
)

const (
	// base HTTP paths.
	apiVersion       = "v1"
	apiBasePath      = "/api/" + apiVersion
	moduleComponents = "components"
	moduleFlows      = "rules"
	moduleNodes      = "nodes"
	moduleLocales    = "locales"
	moduleLogs       = "logs"
	ContentTypeKey   = "Content-Type"
	JsonContextType  = "application/json"
)

// NewRestServe rest服务 接收端点
func NewRestServe(config config.Config) *rest.Endpoint {
	//初始化日志
	addr := config.Server
	if strings.HasPrefix(addr, ":") {
		logger.Logger.Println("RuleGo-Server now running at http://127.0.0.1" + addr)
	} else {
		logger.Logger.Println("RuleGo-Server now running at http://" + addr)
	}

	restEndpoint := &rest.Endpoint{
		Config: rest.Config{
			Server:    addr,
			AllowCors: true,
		},
		RuleConfig: rulego.NewConfig(types.WithDefaultPool(), types.WithLogger(logger.Logger)),
	}
	//添加全局拦截器
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		return true
	})
	//重定向UI界面
	restEndpoint.GET(endpoint.NewRouter().From("/").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			http.Redirect(w.Response(), r.Request(), "/editor/", http.StatusFound)
		}
		return false
	}).End())
	//创建获取所有规则引擎组件列表路由
	restEndpoint.GET(controller.Node.Components(apiBasePath + "/components"))
	//获取所有共享组件
	restEndpoint.GET(controller.Node.ListNodePool(apiBasePath + "/" + moduleNodes + "/shared"))

	//获取所有规则链列表
	restEndpoint.GET(controller.Rule.List(apiBasePath + "/" + moduleFlows))
	//获取最新修改的规则链DSL 实际是：/api/v1/chains/get/latest
	restEndpoint.GET(controller.Rule.GetLatest(apiBasePath + "/" + moduleFlows + "/:id/latest"))
	//获取规则链DSL
	restEndpoint.GET(controller.Rule.Get(apiBasePath + "/" + moduleFlows + "/:id"))
	//新增/修改规则链DSL
	restEndpoint.POST(controller.Rule.Save(apiBasePath + "/" + moduleFlows + "/:id"))
	//删除规则链
	restEndpoint.DELETE(controller.Rule.Delete(apiBasePath + "/" + moduleFlows + "/:id"))
	//保存规则链附加信息
	restEndpoint.POST(controller.Rule.SaveBaseInfo(apiBasePath + "/" + moduleFlows + "/:id/base"))
	//保存规则链配置信息
	restEndpoint.POST(controller.Rule.SaveConfiguration(apiBasePath + "/" + moduleFlows + "/:id/config/:varType"))
	//执行规则链,并得到规则链处理结果
	restEndpoint.POST(controller.Rule.Execute(apiBasePath + "/" + moduleFlows + "/:id/execute/:msgType"))
	//处理数据上报请求，并转发到规则引擎，不等待规则引擎处理结果
	restEndpoint.POST(controller.Rule.PostMsg(apiBasePath + "/" + moduleFlows + "/:id/notify/:msgType"))
	//部署或者下线规则链
	restEndpoint.POST(controller.Rule.Operate(apiBasePath + "/" + moduleFlows + "/:id/operate/:type"))

	//获取节点调试日志列表
	restEndpoint.GET(controller.Log.GetDebugLogs(apiBasePath + "/" + moduleLogs + "/debug"))
	//获取规则链运行日志列表
	restEndpoint.GET(controller.Log.List(apiBasePath + "/" + moduleLogs + "/runs"))
	restEndpoint.DELETE(controller.Log.Delete(apiBasePath + "/" + moduleLogs + "/runs"))

	restEndpoint.GET(controller.Locale.Locales(apiBasePath + "/" + moduleLocales))
	restEndpoint.POST(controller.Locale.Save(apiBasePath + "/" + moduleLocales))
	//创建用户登录路由
	restEndpoint.POST(controller.Base.Login(apiBasePath + "/login"))

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
