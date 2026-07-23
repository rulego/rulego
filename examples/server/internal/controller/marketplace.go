package controller

import (
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"strconv"
)

// MarketplaceComponents to obtain dynamic component market components
func (c *node) MarketplaceComponents(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		checkMyStr := exchange.In.GetParam("checkMy") //Check your own components
		var checkMy bool
		if i, err := strconv.ParseBool(checkMyStr); err == nil {
			checkMy = i
		}
		c.getCustomNodeList(true, checkMy, exchange)
		return true
	}).End()
}

func (c *rule) MarketplaceChains(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		c.list(true, exchange)
		return true
	}).End()
}
