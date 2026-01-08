package router

import (
	"examples/server/config"
	"examples/server/internal/constants"
	"examples/server/internal/controller"
	"examples/server/internal/service"
	"time"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	websocketEndpoint "github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/utils/json"
)

// NewWebsocketServe Websocket服务 接收端点
func NewWebsocketServe(c config.Config, httpEndpoint endpointApi.HttpEndpoint) (endpoint.Endpoint, error) {

	// 使用Registry创建websocket端点，使用HTTP端点作为底层端点
	ep, err := endpoint.Registry.New(
		websocketEndpoint.Type,
		SystemRulegoConfig,
		websocketEndpoint.Config{
			Server:    "ref://" + httpEndpoint.Id(),
			AllowCors: true,
		},
	)
	if err != nil {
		return nil, err
	}

	ep.SetOnEvent(func(eventName string, params ...interface{}) {
		switch eventName {
		case endpointApi.EventConnect:
			exchange := params[0].(*endpointApi.Exchange)
			username := exchange.In.Headers().Get(constants.KeyUsername)
			if username == "" {
				username = config.C.DefaultUsername
			}
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				chainId := exchange.In.GetParam(constants.KeyChainId)
				clientId := exchange.In.GetParam(constants.KeyClientId)
				s.AddOnDebugObserver(chainId, clientId, func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
					errStr := ""
					if err != nil {
						errStr = err.Error()
					}
					var log = map[string]interface{}{
						"chainId":      chainId,
						"flowType":     flowType,
						"nodeId":       nodeId,
						"relationType": relationType,
						"err":          errStr,
						"msg":          msg,
						"ts":           time.Now().UnixMilli(),
					}
					jsonStr, _ := json.Marshal(log)
					exchange.Out.SetBody(jsonStr)
					//写入报错
					if exchange.Out.GetError() != nil {
						s.RemoveOnDebugObserver(clientId)
					}
				})
			}
		case endpointApi.EventDisconnect:
			exchange := params[0].(*endpointApi.Exchange)
			username := exchange.In.Headers().Get(constants.KeyUsername)
			if username == "" {
				username = config.C.DefaultUsername
			}
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				s.RemoveOnDebugObserver(exchange.In.GetParam(constants.KeyClientId))
			}
		}
	})
	httpEndpoint.SetOnEvent(func(eventName string, params ...interface{}) {
		switch eventName {
		case endpointApi.EventRestart:
			_, _ = ep.AddRouter(controller.Log.WsNodeLogRouter(apiBasePath + "/" + moduleLogs + "/ws/:chainId/:clientId"))
		}
	})
	_, _ = ep.AddRouter(controller.Log.WsNodeLogRouter(apiBasePath + "/" + moduleLogs + "/ws/:chainId/:clientId"))

	return ep, nil
}
