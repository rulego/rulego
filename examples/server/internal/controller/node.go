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
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
)

var Node = &node{}

type node struct {
}

// Components: Create a list of routing for the fetched rule engine node components
func (c *node) Components(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			nodePool, _ := node_pool.DefaultNodePool.GetAllDef()
			//Component configuration with built-in options
			builtins := make(map[string]interface{})
			for k, v := range service.Builtins() {
				builtins[k] = v
			}
			// Endpoints has built-in routing options
			builtins["endpoints"] = map[string]interface{}{
				//in the processor list
				"inProcessors": processor.InBuiltins.Names(),
				//in the processor list
				"outProcessors": processor.OutBuiltins.Names(),
			}
			//Shared node pool
			builtins["nodePool"] = nodePool

			//Configure the list of forms in response to endpoint and node components
			list, err := json.Marshal(map[string]interface{}{
				//Endpoint components
				"endpoints": endpoint.Registry.GetComponentForms().Values(),
				//Node components
				"nodes": s.GetRuleConfig().ComponentsRegistry.GetComponentForms().Values(),
				//Component configuration with built-in options
				"builtins": builtins,
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

// ListNodePool retrieves all shared components
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

// CustomNodeList retrieves all custom dynamic components from the user
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

// CustomNodeList retrieves all custom dynamic components from the user, which is taken by default from the local default user's custom components. If MarketBaseUrl is configured, it is obtained from the component marketplace
// - checkMy:true, checks whether the current user's component needs to be upgraded and if it is installed
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
		//Sourcing modules from the module market
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
		//Get the components that the current user has installed
		var installedList types.ComponentFormList
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			installedList = s.ComponentService().ComponentsRegistry().GetComponentForms()
		} else {
			return userNotFound(username, exchange)
		}
		//Mark installed components that need upgrades
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

// CustomNodeDSL obtains dynamic component DSL definitions
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

// CustomNodeInstall installs custom dynamic components
func (c *node) CustomNodeInstall(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		c.customNodeInstall(username, false, exchange)
		return true
	}).End()
}

// CustomNodeUpgrade installs/upgrades custom dynamic components
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

// CustomNodeUninstall uninstalls custom dynamic components
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
