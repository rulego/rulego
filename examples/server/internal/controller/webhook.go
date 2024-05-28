package controller

import (
	"examples/server/internal/service"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
)

func TestWebhookRouter(url string) endpointApi.Router {
	return endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(GetRuleGoFunc)).From(url).Process(AuthProcess).Transform(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		//fmt.Println("接收到webhook数据")
		//r := exchange.In.(*rest.RequestMessage)
		//fmt.Println("Headers:", r.Headers())
		//fmt.Println("Metadata:", exchange.In.GetMsg().Metadata)
		//fmt.Println("Data:", exchange.In.GetMsg().Data)
		return true
	}).To("chain:${chainId}").SetOpts(
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			service.EventServiceImpl.SaveRunLog(ctx, snapshot)
		})).End()
}
