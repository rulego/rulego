package controller

import (
	"examples/server/config"
	"examples/server/internal/service"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
)

var MCP = &mcp{}

type mcp struct {
}

func (c *mcp) Handler(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		r := exchange.In.(*rest.RequestMessage)
		apiKey := r.Params.ByName("apiKey")
		var username string
		if apiKey != "" {
			username = service.UserServiceImpl.GetUsernameByApiKey(apiKey)
		}

		if config.Get().RequireAuth && username == "" {
			//不允许匿名访问
			return unauthorized(username, exchange)
		} else if username == "" {
			username = config.C.DefaultUsername
		}
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			inMsg := exchange.In.(*rest.RequestMessage)
			mcpService := s.MCPService()
			if mcpService != nil {
				mcpService.SSEServer().ServeHTTP(inMsg.Response(), inMsg.Request())
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}
