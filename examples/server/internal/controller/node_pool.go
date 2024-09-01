package controller

import (
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/utils/json"
	"net/http"
)

// ListNodePool 获取所有共享组件
func ListNodePool(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			var result = map[string][]*types.RuleNode{}
			var err error
			if s.GetRuleConfig().NetPool != nil {
				result, err = s.GetRuleConfig().NetPool.GetAllDef()
			}
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				return false
			}
			if v, err := json.Marshal(result); err == nil {
				exchange.Out.SetBody(v)
			} else {
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				return false
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}
