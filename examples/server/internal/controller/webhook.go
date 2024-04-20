package controller

import (
	"github.com/rulego/rulego/endpoint"
)

func TestWebhookRouter(url string) *endpoint.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Transform(func(router *endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("ok"))
		return true
	}).End()
}
