package controller

import (
	"examples/server/config"
	"examples/server/internal/constants"
	"examples/server/internal/service"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"net/http"
)

// MarketNodeList 获取组件市场动态组件
func (c *node) MarketNodeList(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		c.getCustomNodeList(true, exchange)
		return true
	}).End()
}

// MarketNodeUpgrade 把自定义组件保存到组件市场
func (c *node) MarketNodeUpgrade(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		c.customNodeInstall(config.C.DefaultUsername, true, exchange)
		return true
	}).End()
}

// MarketNodeDSL 获取动态组件DSL定义
func (c *node) MarketNodeDSL(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := config.C.DefaultUsername
		nodeType := msg.Metadata.GetValue(constants.KeyNodeType)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			dsl, err := s.ComponentService().Get(nodeType)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte(err.Error()))
			} else {
				exchange.Out.SetBody(dsl)
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}
