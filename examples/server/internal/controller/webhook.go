package controller

import (
	"examples/server/internal/service"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
)

func TestWebhookRouter(url string) *endpoint.Router {
	return endpoint.NewRouter(endpoint.WithRuleGoFunc(GetRuleGoFunc)).From(url).Process(AuthProcess).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		//fmt.Println("接收到webhook数据")
		//r := exchange.In.(*rest.RequestMessage)
		//fmt.Println("Headers:", r.Headers())
		//fmt.Println("Metadata:", exchange.In.GetMsg().Metadata)
		//fmt.Println("Data:", exchange.In.GetMsg().Data)
		return true
	}).To("chain:${chainId}").RuleContextOption(
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			service.EventServiceImpl.SaveRunLog(ctx, snapshot)
		})).End()
}
