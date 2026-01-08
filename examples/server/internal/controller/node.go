package controller

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"net/http"
	"strconv"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
)

var Node = &node{}

type node struct {
}

// Components 创建获取规则引擎节点组件列表路由
func (c *node) Components(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			nodePool, _ := node_pool.DefaultNodePool.GetAllDef()
			//响应endpoint和节点组件配置表单列表
			list, err := json.Marshal(map[string]interface{}{
				//endpoint组件
				"endpoints": endpoint.Registry.GetComponentForms().Values(),
				//节点组件
				"nodes": s.GetRuleConfig().ComponentsRegistry.GetComponentForms().Values(),
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
		} else {
			return userNotFound(username, exchange)
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
			if s.GetRuleConfig().NodePool != nil {
				result, err = s.GetRuleConfig().NodePool.GetAllDef()
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

// CustomNodeList 获取用户所有自定义动态组件
func (c *node) CustomNodeList(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		c.getCustomNodeList(false, true, exchange)
		return true
	}).End()
}

type ComponentList struct {
	Page  int               `json:"page"`
	Size  int               `json:"size"`
	Total int               `json:"total"`
	Items []types.RuleChain `json:"items"`
}

// CustomNodeList 获取用户所有自定义动态组件，默认从本地默认用户的自定义组件获取，如果配置了MarketBaseUrl，则从组件市场获取
// - checkMy:true，检查当前用户对应的组件是否需要升级，是否已安装
func (c *node) getCustomNodeList(getFromMarketplace bool, checkMy bool, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	username := msg.Metadata.GetValue(constants.KeyUsername)
	keywords := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords))
	var page = 1
	var size = 20
	currentStr := msg.Metadata.GetValue(constants.KeyPage)
	if i, err := strconv.Atoi(currentStr); err == nil {
		page = i
	}
	pageSizeStr := msg.Metadata.GetValue(constants.KeySize)
	if i, err := strconv.Atoi(pageSizeStr); err == nil {
		size = i
	}

	var components []types.RuleChain
	var total int
	var hasGetFromMarket = false
	if getFromMarketplace {
		//从组件市场获取组件
		if config.C.MarketplaceBaseUrl != "" {
			componentList, err := GetComponentsFromMarketplace(config.C.MarketplaceBaseUrl+"/marketplace/components", keywords, nil, page, size)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
				return true
			} else {
				components = componentList.Items
				total = componentList.Total
				hasGetFromMarket = true
			}
		} else {
			username = config.C.DefaultUsername
		}
	}

	if !hasGetFromMarket {
		var err error
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			components, total, err = s.ComponentService().List(keywords, size, page)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
				return true
			}
		} else {
			return userNotFound(username, exchange)
		}
	}

	if checkMy {
		//获取当前用户已经安装的组件
		var installedList types.ComponentFormList
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			installedList = s.ComponentService().ComponentsRegistry().GetComponentForms()
		} else {
			return userNotFound(username, exchange)
		}
		//标记已安装、需要升级的组件
		for i := range components {
			item := &components[i]
			if item.RuleChain.AdditionalInfo == nil {
				item.RuleChain.AdditionalInfo = map[string]interface{}{}
			}

			if componentForm, ok := installedList.GetComponent(item.RuleChain.ID); ok {
				item.RuleChain.AdditionalInfo["installed"] = true
				if v := item.RuleChain.AdditionalInfo["version"]; str.ToString(v) != componentForm.Version {
					item.RuleChain.AdditionalInfo["upgraded"] = true
				}
			}
		}
	}

	result := map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"items": components,
	}
	if v, err := json.Marshal(result); err == nil {
		exchange.Out.SetBody(v)
	} else {
		logger.Logger.Println(err)
		exchange.Out.SetStatusCode(http.StatusBadRequest)
		exchange.Out.SetBody([]byte(err.Error()))
	}
	return true
}

// CustomNodeDSL 获取动态组件DSL定义
func (c *node) CustomNodeDSL(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		c.customNodeDSL(username, exchange)
		return true
	}).End()
}

func (c *node) customNodeDSL(username string, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	nodeType := msg.Metadata.GetValue(constants.KeyId)
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
}

// CustomNodeInstall 安装自定义动态组件
func (c *node) CustomNodeInstall(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		c.customNodeInstall(username, false, exchange)
		return true
	}).End()
}

// CustomNodeUpgrade 安装/升级自定义动态组件
func (c *node) CustomNodeUpgrade(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		c.customNodeInstall(username, true, exchange)
		return true
	}).End()
}

func (c *node) customNodeInstall(username string, upgrade bool, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	nodeType := msg.Metadata.GetValue(constants.KeyId)
	if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
		var err error
		if upgrade {
			err = s.ComponentService().Upgrade(nodeType, exchange.In.Body())
		} else {
			err = s.ComponentService().Install(nodeType, exchange.In.Body())
		}
		if err != nil {
			logger.Logger.Println(err)
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		}
	} else {
		return userNotFound(username, exchange)
	}
	return true
}

// CustomNodeUninstall 卸载自定义动态组件
func (c *node) CustomNodeUninstall(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		nodeType := msg.Metadata.GetValue(constants.KeyId)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			err := s.ComponentService().Uninstall(nodeType)
			if err != nil {
				logger.Logger.Println(err)
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte(err.Error()))
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}
