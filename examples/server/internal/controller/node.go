package controller

import (
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/utils/json"
	"net/http"
)

var Node = &node{}

type node struct {
}

// Components 创建获取规则引擎节点组件列表路由
func (c *node) Components(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		nodePool, _ := node_pool.DefaultNodePool.GetAllDef()
		//响应endpoint和节点组件配置表单列表
		list, err := json.Marshal(map[string]interface{}{
			//endpoint组件
			"endpoints": endpoint.Registry.GetComponentForms().Values(),
			//节点组件
			"nodes": rulego.Registry.GetComponentForms().Values(),
			//组件配置内置选项
			"builtins": map[string]interface{}{
				// functions节点组件
				"functions": map[string]interface{}{
					//函数名选项
					"functionName": action.Functions.Names(),
				},
				//endpoints内置路由选项
				"endpoints": map[string]interface{}{
					//in 处理器列表
					"inProcessors": processor.InBuiltins.Names(),
					//in 处理器列表
					"outProcessors": processor.OutBuiltins.Names(),
				},
				//共享节点池
				"nodePool": nodePool,
			},
		})
		if err != nil {
			exchange.Out.SetStatusCode(http.StatusInternalServerError)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			exchange.Out.SetBody(list)
		}
		return true
	}).End()
}

// ListNodePool 获取所有共享组件
func (c *node) ListNodePool(url string) endpointApi.Router {
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
