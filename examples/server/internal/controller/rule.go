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

// Get 创建获取指定规则链路由
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

// GetLatest 获取最近修改的规则链
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

// Save 创建保存/更新指定规则链路由
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

// List 创建获取所有规则链路由
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
		//从组件市场获取规则链
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

// Delete 创建删除指定规则链路由
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

// SaveBaseInfo 保存规则链扩展信息
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

// SaveConfiguration 保存规则链配置
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
	//获取消息类型
	msg.Type = msg.Metadata.GetValue(constants.KeyMsgType)
	//把http header放入消息元数据
	if msg.Metadata.GetValue(constants.KeyHeadersToMetadata) == "true" {
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
	}
	//if msg.Metadata.GetValue(constants.KeySetWorkDir)=="true"{
	//	username := msg.Metadata.GetValue(constants.KeyUsername)
	//	//设置工作目录
	//	var paths = []string{config.C.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule}
	//	msg.Metadata.PutValue(constants.KeyWorkDir, path.Join(paths...)
	//}
	return true
}

// Execute 处理请求，并转发到规则引擎，同步等待规则链执行结果返回给调用方
// .To("chain:${id}") 这段逻辑相当于：
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
			//错误
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(exchange.Out.GetError().Error()))
		} else {
			//把处理结果响应给客户端，http endpoint 必须增加 Wait()，否则无法正常响应
			outMsg := exchange.Out.GetMsg()
			exchange.Out.SetBody([]byte(outMsg.GetData()))
		}
		return true
	}).Wait().End()
}

// PostMsg 处理请求，并转发到规则引擎
// .To("chain:${id}") 这段逻辑相当于：
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

// Operate 部署/下架规则链
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
