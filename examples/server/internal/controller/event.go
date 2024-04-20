package controller

import (
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/utils/json"
	"net/http"
)

// GetDebugDataRouter 创建获取节点调试数据路由
func GetDebugDataRouter(url string) *endpoint.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.ChainId)
		nodeId := msg.Metadata.GetValue(constants.NodeId)
		username := msg.Metadata.GetValue(constants.Username)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			page := s.DebugData().GetToPage(chainId, nodeId)
			if v, err := json.Marshal(page); err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
			} else {
				exchange.Out.SetBody(v)
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

func WsNodeLogRouter(url string) *endpoint.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.Username)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			s.AddOnDebugObserver(exchange.In.GetParam(constants.ClientId), func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
				exchange.Out.SetBody([]byte(msg.Data))
			})
		}
		return true
	}).End()
}
