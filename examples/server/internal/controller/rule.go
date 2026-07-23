package controller

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"examples/server/internal/service"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
	"net/http"
	"strconv"
	"strings"
)

var Rule = &rule{}

type rule struct {
}

// Get creates a route to obtain the specified rule chain
func (c *rule) Get(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if def, err := s.Get(chainId); err == nil {
				exchange.Out.SetBody(def)
			} else {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				return false
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// GetLatest retrieves the most recently modified rule chain
func (c *rule) GetLatest(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if def, err := s.GetLatest(); err == nil {
				exchange.Out.SetBody(def)
			} else {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				return false
			}
		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}

// Save creates a save/update specified rule for the link route
func (c *rule) Save(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if err := s.SaveAndLoad(chainId, exchange.In.Body()); err == nil {
				exchange.Out.SetStatusCode(http.StatusOK)
			} else {
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

// List creates to retrieve all rule chain routes
func (c *rule) List(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		return c.list(false, exchange)
	}).End()
}

func (c *rule) list(getFromMarketplace bool, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	username := msg.Metadata.GetValue(constants.KeyUsername)
	keywords := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords))
	rootStr := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyRoot))
	rootDisabled := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyDisabled))
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
	var root *bool
	var disabled *bool
	if i, err := strconv.ParseBool(rootStr); err == nil {
		root = &i
	}
	if i, err := strconv.ParseBool(rootDisabled); err == nil {
		disabled = &i
	}
	var list []types.RuleChain
	var total int
	var hasGetFromMarket = false
	var err error
	if getFromMarketplace {
		//Obtain the rule chain from the module market
		if config.C.MarketplaceBaseUrl != "" {
			componentList, err := GetComponentsFromMarketplace(config.C.MarketplaceBaseUrl+"/marketplace/chains", keywords, root, page, size)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
				return true
			} else {
				list = componentList.Items
				total = componentList.Total
				hasGetFromMarket = true
			}
		} else {
			username = config.C.DefaultUsername
		}
	}
	if !hasGetFromMarket {
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			list, total, err = s.List(keywords, root, disabled, size, page)
			if err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(err.Error()))
				return true
			}
		} else {
			return userNotFound(username, exchange)
		}
	}
	result := map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"items": list,
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

// Delete creates and deletes the specified rule chain route
func (c *rule) Delete(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if err := s.Delete(chainId); err == nil {
				exchange.Out.SetStatusCode(http.StatusOK)
			} else {
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

// SaveBaseInfo stores rule chain extension information
func (c *rule) SaveBaseInfo(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		var req types.RuleChainBaseInfo
		if err := json.Unmarshal([]byte(msg.GetData()), &req); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				if err := s.SaveBaseInfo(chainId, req); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}

			} else {
				return userNotFound(username, exchange)
			}
		}
		return true
	}).End()
}

// SaveConfiguration Saves the rule chain configuration
func (c *rule) SaveConfiguration(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		varType := msg.Metadata.GetValue(constants.KeyVarType)
		var req interface{}
		if err := json.Unmarshal([]byte(msg.GetData()), &req); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
				if err := s.SaveConfiguration(chainId, varType, req); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else {
				return userNotFound(username, exchange)
			}
		}
		return true
	}).End()
}
func (c *rule) transformMsg(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	msgId := exchange.In.GetParam(constants.KeyMsgId)
	if msgId != "" {
		msg.Id = msgId
	}
	//Get message types
	msg.Type = msg.Metadata.GetValue(constants.KeyMsgType)
	//Put the HTTP header into the message metadata
	if msg.Metadata.GetValue(constants.KeyHeadersToMetadata) == "true" {
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
	}
	//if msg.Metadata.GetValue(constants.KeySetWorkDir)=="true"{
	//	username := msg.Metadata.GetValue(constants.KeyUsername)
	//	Set the work directory
	//	var paths = []string{config.C.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule}
	//	msg.Metadata.PutValue(constants.KeyWorkDir, path.Join(paths...)
	//}
	return true
}

// Execute processes the request and forwards it to the rule engine, synchronously waiting for the execution results of the rule chain to be returned to the caller
// . The logic of To("chain:${id}") is equivalent to:
//
//	engine,err:=pool.Get(chainId)
//	engine.OnMsgAndWait(msg)
func (c *rule) Execute(url string) endpointApi.Router {
	var opts []types.RuleContextOption
	if config.C.SaveRunLog {
		opts = append(opts, c.addWithOnRuleChainCompleted())
	}

	return endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(GetRuleGoFunc)).
		From(url).
		Process(AuthProcess).Transform(c.transformMsg).
		Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
			exchange.Out.Headers().Set("Content-Type", "application/json")
			return true
		}).To("chain:${id}").SetOpts(opts...).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		err := exchange.Out.GetError()
		if err != nil {
			//Wrong
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else {
			//Deliver the processing result to the client; the HTTP endpoint must add Wait(), otherwise it cannot respond properly
			outMsg := exchange.Out.GetMsg()
			exchange.Out.SetBody([]byte(outMsg.GetData()))
		}
		return true
	}).Wait().End()
}

// PostMsg processes the request and forwards it to the rules engine
// . The logic of To("chain:${id}") is equivalent to:
//
//	engine,err:=pool.Get(chainId)
//	engine.OnMsg(msg)
func (c *rule) PostMsg(url string) endpointApi.Router {
	var opts []types.RuleContextOption
	if config.C.SaveRunLog {
		opts = append(opts, c.addWithOnRuleChainCompleted())
	}
	return endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(GetRuleGoFunc)).
		From(url).Process(AuthProcess).Transform(c.transformMsg).To("chain:${id}").SetOpts(opts...).End()
}

func (c *rule) addWithOnRuleChainCompleted() types.RuleContextOption {
	return types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
		var username = config.C.DefaultUsername
		if chainCtx, ok := ctx.RuleChain().(types.ChainCtx); ok {
			if def := chainCtx.Definition(); def != nil {
				if v, ok := def.RuleChain.GetAdditionalInfo(constants.KeyUsername); ok {
					username = str.ToString(v)
				}
			}
		}
		_ = service.EventServiceImpl.SaveRunLog(username, ctx, snapshot)
	})
}

// Operate: deploy/delist the rule chain
func (c *rule) Operate(url string) endpointApi.Router {
	return endpoint.NewRouter().From(url).Process(AuthProcess).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		chainId := msg.Metadata.GetValue(constants.KeyId)
		opType := msg.Metadata.GetValue(constants.KeyType)
		username := msg.Metadata.GetValue(constants.KeyUsername)
		if s, ok := service.UserRuleEngineServiceImpl.Get(username); ok {
			if opType == constants.OperateDeploy {
				if err := s.Deploy(chainId); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else if opType == constants.OperateUndeploy {
				if err := s.Undeploy(chainId); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else if opType == constants.OperateSetToMain {
				if err := s.SetMainChainId(chainId); err != nil {
					exchange.Out.SetStatusCode(http.StatusBadRequest)
					exchange.Out.SetBody([]byte(err.Error()))
				}
			} else {
				exchange.Out.SetStatusCode(http.StatusBadRequest)
				exchange.Out.SetBody([]byte("没有该操作类型:" + opType))
			}

		} else {
			return userNotFound(username, exchange)
		}
		return true
	}).End()
}
