package router

import (
	"errors"
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/controller"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/node_pool"
	"net/http"
	"strings"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion
	moduleFlows = "rules"
	// moduleDcs dynamic component
	moduleDynamicComponents = "dynamic-components"
	// moduleSharedNodes Shares components
	moduleSharedNodes = "shared-nodes"
	moduleLocales     = "locales"
	moduleLogs        = "logs"
	moduleMarketplace = "marketplace"
	ContentTypeKey    = "Content-Type"
	JsonContextType   = "application/json"
)

// SystemRulegoConfig System rulego configuration
var SystemRulegoConfig types.Config

// SystemNodePool: The internal node pool of the system
var SystemNodePool *node_pool.NodePool

func InitRulegoConfig() {
	SystemRulegoConfig = rulego.NewConfig(types.WithDefaultPool(), types.WithLogger(logger.Logger))
	SystemNodePool = node_pool.NewNodePool(SystemRulegoConfig)
	SystemRulegoConfig.NodePool = SystemNodePool
}

// NewRestServe Rest service receives endpoints
func NewRestServe(config config.Config) (endpointApi.HttpEndpoint, error) {
	//Initialize the log
	addr := config.Server
	if strings.HasPrefix(addr, ":") {
		logger.Logger.Println("RuleGo-Server now running at http://127.0.0.1" + addr)
	} else {
		logger.Logger.Println("RuleGo-Server now running at http://" + addr)
	}

	ep, err := endpoint.Registry.New(
		rest.Type,
		SystemRulegoConfig,
		rest.Config{
			Server:    addr,
			AllowCors: true,
		},
	)
	if err != nil {
		return nil, err
	}
	var restEndpoint endpointApi.HttpEndpoint
	if ep, ok := ep.(endpointApi.HttpEndpoint); !ok {
		return nil, errors.New("is not HttpEndpoint type error")
	} else {
		restEndpoint = ep
	}
	//Added a global interceptor
	restEndpoint.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		if out, ok := exchange.Out.(endpointApi.HeaderModifier); ok {
			out.AddHeader(ContentTypeKey, JsonContextType)
		} else {
			exchange.Out.Headers().Set(ContentTypeKey, JsonContextType)
		}
		return true
	})
	//Redirect UI interface
	restEndpoint.GET(endpoint.NewRouter().From("/").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			http.Redirect(w.Response(), r.Request(), "/editor/", http.StatusFound)
		}
		return false
	}).End())
	//Create a route that retrieves all rule engine component lists
	restEndpoint.GET(controller.Node.Components(apiBasePath + "/components"))

	//Obtain all shared components
	restEndpoint.GET(controller.Node.ListNodePool(apiBasePath + "/" + moduleSharedNodes))

	//Get a list of components in the Component Market
	restEndpoint.GET(controller.Node.MarketplaceComponents(apiBasePath + "/" + moduleMarketplace + "/components"))
	//Obtain a list of component market rule chains
	restEndpoint.GET(controller.Rule.MarketplaceChains(apiBasePath + "/" + moduleMarketplace + "/chains"))

	//Get a list of all user-defined dynamic components
	restEndpoint.GET(controller.Node.CustomNodeList(apiBasePath + "/" + moduleDynamicComponents))
	//Get custom dynamic component DSL
	restEndpoint.GET(controller.Node.CustomNodeDSL(apiBasePath + "/" + moduleDynamicComponents + "/:id"))
	//Install/upgrade custom dynamic components
	restEndpoint.POST(controller.Node.CustomNodeUpgrade(apiBasePath + "/" + moduleDynamicComponents + "/:id"))
	//Install custom dynamic components
	restEndpoint.DELETE(controller.Node.CustomNodeUninstall(apiBasePath + "/" + moduleDynamicComponents + "/:id"))

	//Get a list of all rule chains
	restEndpoint.GET(controller.Rule.List(apiBasePath + "/" + moduleFlows))
	//The DSL for obtaining the latest modified rule chain is: /api/v1/rules/get/latest
	restEndpoint.GET(controller.Rule.GetLatest(apiBasePath + "/" + moduleFlows + "/:id/latest"))
	//Obtain the Rule Chain DSL
	restEndpoint.GET(controller.Rule.Get(apiBasePath + "/" + moduleFlows + "/:id"))
	//Add/modify the rule chain DSL
	restEndpoint.POST(controller.Rule.Save(apiBasePath + "/" + moduleFlows + "/:id"))
	//Delete the rule chain
	restEndpoint.DELETE(controller.Rule.Delete(apiBasePath + "/" + moduleFlows + "/:id"))
	//Save additional information from the rule chain
	restEndpoint.POST(controller.Rule.SaveBaseInfo(apiBasePath + "/" + moduleFlows + "/:id/base"))
	//Saves rule chain configuration information
	restEndpoint.POST(controller.Rule.SaveConfiguration(apiBasePath + "/" + moduleFlows + "/:id/config/:varType"))
	//Execute the rule chain and obtain the result of the rule chain processing
	restEndpoint.POST(controller.Rule.Execute(apiBasePath + "/" + moduleFlows + "/:id/execute/:msgType"))
	//Processing data reporting requests and forwarding them to the rule engine without waiting for the rules engine's results
	restEndpoint.POST(controller.Rule.PostMsg(apiBasePath + "/" + moduleFlows + "/:id/notify/:msgType"))
	//Deploy or take down the rule chain
	restEndpoint.POST(controller.Rule.Operate(apiBasePath + "/" + moduleFlows + "/:id/operate/:type"))

	//Retrieves the list of node debugging logs
	restEndpoint.GET(controller.Log.GetDebugLogs(apiBasePath + "/" + moduleLogs + "/debug"))
	//Retrieve the list of rule chain runtime logs
	restEndpoint.GET(controller.Log.List(apiBasePath + "/" + moduleLogs + "/runs"))
	//Obtain detailed rules chain runtime logs
	restEndpoint.DELETE(controller.Log.Delete(apiBasePath + "/" + moduleLogs + "/runs"))

	restEndpoint.GET(controller.Locale.Locales(apiBasePath + "/" + moduleLocales))
	restEndpoint.POST(controller.Locale.Save(apiBasePath + "/" + moduleLocales))
	//Create a user login route
	restEndpoint.POST(controller.Base.Login(apiBasePath + "/login"))

	if config.MCP.Enable {
		restEndpoint.GET(controller.MCP.Handler(apiBasePath + "/mcp/:apiKey/sse"))
		restEndpoint.POST(controller.MCP.Handler(apiBasePath + "/mcp/:apiKey/message"))
		logger.Logger.Println("RuleGo-Server mcp server running at http://127.0.0.1" + addr + apiBasePath + "/mcp/" +
			config.GetApiKeyByUsername(config.DefaultUsername) + "/sse")
	}
	// Load the static file mapping
	restEndpoint.RegisterStaticFiles(config.ResourceMapping)

	//Set the default HTTP service to a shared node
	if config.ShareHttpServer {
		_, _ = node_pool.DefaultNodePool.AddNode(restEndpoint)
	}
	//Add the default HTTP service to the system node pool
	_, _ = SystemNodePool.AddNode(restEndpoint)
	return restEndpoint, nil
}

// LoadServeFiles loads the static file mapping
func LoadServeFiles(c config.Config, restEndpoint endpointApi.HttpEndpoint) {
	if c.ResourceMapping != "" {
		restEndpoint.RegisterStaticFiles(c.ResourceMapping)
	}
}
