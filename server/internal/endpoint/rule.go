package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/services"
)

// getRuleGoFunc 动态获取指定用户的规则引擎池
func (s *Server) getRuleGoFunc(exchange *endpointApi.Exchange) types.RuleEnginePool {
	username := metadataUsername(exchange)
	if username == "" {
		username = s.config.DefaultUsername
	}
	mgr, err := app.GetAs[services.EngineManager](s.container, services.KeyEngineManager)
	if err != nil {
		return nil
	}
	ue, err := mgr.GetOrCreate(username)
	if err != nil {
		exchange.In.SetError(fmt.Errorf("not found username=%s", username))
		return nil
	}
	return ue.Pool()
}

func (s *Server) registerRuleRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /rules - 获取规则链列表
	ep.GET(endpoint.NewRouter().From(base+"/rules").Process(s.authWithPermission("rule", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		msg := exchange.In.GetMsg()
		username := metadataUsername(exchange)
		keywords := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyKeywords))
		rootStr := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyRoot))
		disabledStr := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyDisabled))
		category := strings.TrimSpace(msg.Metadata.GetValue(constants.KeyCategory))
		page := intParam(msg, constants.KeyPage, 1)
		size := intParam(msg, constants.KeySize, 20)

		var root, disabled *bool
		if b, err := strconv.ParseBool(rootStr); err == nil {
			root = &b
		}
		if b, err := strconv.ParseBool(disabledStr); err == nil {
			disabled = &b
		}

		catalog, ok := getService[services.ChainCatalog](s, exchange, services.KeyRuleCatalog)
		if !ok {
			return false
		}
		list, total, err := catalog.List(username, keywords, root, disabled, category, size, page)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeListResult(exchange, list, total, page, size)
		return true
	}).End())

	// GET /rules/:id - 获取规则链 DSL
	ep.GET(endpoint.NewRouter().From(base + "/rules/:id").Process(s.authWithPermission("rule", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		id := metadataValue(exchange, constants.KeyId)
		if !validateId(id) {
			writeBadRequest(exchange, fmt.Errorf("invalid rule chain id"))
			return false
		}
		catalog, ok := getService[services.ChainCatalog](s, exchange, services.KeyRuleCatalog)
		if !ok {
			return false
		}
		def, err := catalog.Get(metadataUsername(exchange), id)
		if err != nil {
			exchange.Out.SetStatusCode(http.StatusNotFound)
			return false
		}
		exchange.Out.SetBody(def)
		return true
	}).End())

	// POST /rules/:id - 保存规则链
	ep.POST(endpoint.NewRouter().From(base + "/rules/:id").Process(s.authWithPermission("rule", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		id := metadataValue(exchange, constants.KeyId)
		if !validateId(id) {
			writeBadRequest(exchange, fmt.Errorf("invalid rule chain id"))
			return false
		}
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		if err := admin.SaveAndLoad(metadataUsername(exchange), id, exchange.In.Body()); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())

	// DELETE /rules/:id - 删除规则链
	ep.DELETE(endpoint.NewRouter().From(base + "/rules/:id").Process(s.authWithPermission("rule", "delete")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		id := metadataValue(exchange, constants.KeyId)
		if !validateId(id) {
			writeBadRequest(exchange, fmt.Errorf("invalid rule chain id"))
			return false
		}
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		if err := admin.Delete(metadataUsername(exchange), id); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeNoContent(exchange)
		return true
	}).End())

	// POST /rules/:id/operate/:type - 部署/下线规则链
	ep.POST(endpoint.NewRouter().From(base + "/rules/:id/operate/:type").Process(s.authWithPermission("rule", "operate")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		chainId := metadataValue(exchange, constants.KeyId)
		if !validateId(chainId) {
			writeBadRequest(exchange, fmt.Errorf("invalid rule chain id"))
			return false
		}
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		username := metadataUsername(exchange)
		opType := metadataValue(exchange, constants.KeyType)

		var opErr error
		switch opType {
		case "start":
			opErr = admin.Deploy(username, chainId)
		case "stop":
			opErr = admin.Undeploy(username, chainId)
		case "set-to-main":
			opErr = admin.SetMainChainId(username, chainId)
		default:
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte("unknown operate type: " + opType))
			return false
		}
		if opErr != nil {
			writeBadRequest(exchange, opErr)
		}
		return true
	}).End())

	// POST /rules/:id/notify/:msgType - 执行规则链（异步，不等待结果）
	ep.POST(endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(s.getRuleGoFunc)).
		From(base + "/rules/:id/notify/:msgType").
		Process(s.authWithPermission("rule", "execute")).
		Transform(s.transformRuleMsg).Process(s.resolveStartNode).
		To("chain:${id}${_targetNodePath}").SetOpts(s.runLogOpts()...).
		End())

	// GET /rules/:id/latest - 获取最近修改的规则链
	ep.GET(endpoint.NewRouter().From(base + "/rules/:id/latest").Process(s.authWithPermission("rule", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		catalog, ok := getService[services.ChainCatalog](s, exchange, services.KeyRuleCatalog)
		if !ok {
			return false
		}
		username := metadataUsername(exchange)
		if admin, _ := app.GetAs[services.RuleAdminService](s.container, services.KeyRuleManager); admin != nil {
			if latestId := admin.GetSetting(username, constants.SettingKeyLatestChainId); latestId != "" {
				if def, err := catalog.Get(username, latestId); err == nil {
					exchange.Out.SetBody(def)
					return true
				}
			}
		}
		exchange.Out.SetStatusCode(http.StatusNotFound)
		return false
	}).End())

	// POST /rules/:id/base - 保存规则链基础信息
	ep.POST(endpoint.NewRouter().From(base + "/rules/:id/base").Process(s.authWithPermission("rule", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		var baseInfo types.RuleChainBaseInfo
		if err := json.Unmarshal(exchange.In.Body(), &baseInfo); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := admin.SaveBaseInfo(metadataUsername(exchange), metadataValue(exchange, constants.KeyId), baseInfo); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())

	// POST /rules/:id/config/:varType - 保存规则链配置
	ep.POST(endpoint.NewRouter().From(base + "/rules/:id/config/:varType").Process(s.authWithPermission("rule", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		var configData interface{}
		if err := json.Unmarshal(exchange.In.Body(), &configData); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := admin.SaveConfiguration(metadataUsername(exchange), metadataValue(exchange, constants.KeyId), metadataValue(exchange, "varType"), configData); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())

	// POST /rules/:id/execute/:msgType - 同步执行规则链，等待结果返回
	ep.POST(endpoint.NewRouter(endpointApi.RouterOptions.WithRuleGoFunc(s.getRuleGoFunc)).
		From(base + "/rules/:id/execute/:msgType").
		Process(s.authWithPermission("rule", "execute")).
		Transform(s.transformRuleMsg).Process(s.resolveStartNode).
		Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
			exchange.Out.Headers().Set("Content-Type", "application/json")
			return true
		}).To("chain:${id}${_targetNodePath}").SetOpts(s.runLogOpts()...).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		if err := exchange.Out.GetError(); err != nil {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			exchange.Out.SetBody([]byte(err.Error()))
		} else {
			outMsg := exchange.Out.GetMsg()
			exchange.Out.SetBody([]byte(outMsg.GetData()))
		}
		return true
	}).Wait().End())

	// POST /rules/:id/v1/chat/completions - OpenAI 兼容路由
	ep.POST(endpoint.NewRouter().From(base + "/rules/:id/v1/chat/completions").Process(s.authWithPermission("rule", "execute")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		executor, ok := getService[services.ChainExecutor](s, exchange, services.KeyRuleExecutor)
		if !ok {
			return false
		}
		var body map[string]interface{}
		if err := json.Unmarshal(exchange.In.Body(), &body); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		stream, _ := body[constants.MetaStream].(bool)

		msg := exchange.In.GetMsg()
		msg.DataType = types.JSON
		msg.SetData(string(exchange.In.Body()))
		if stream {
			msg.Metadata.PutValue(constants.MetaStream, types.ValueTrue)
		}

		chainId := metadataValue(exchange, constants.KeyId)
		ruleMsg := types.NewMsg(0, constants.MsgTypeChatCompletions, types.JSON, msg.Metadata, string(exchange.In.Body()))

		// Use WithOnEnd to capture the rule chain output and format as OpenAI response
		// 注入请求用户身份到执行上下文，确保系统智能体的 MCP 工具操作正确的用户空间
		execCtx := services.ContextWithMCPRequestingUser(context.Background(), metadataUsername(exchange))
		if err := executor.ExecuteAndWait(metadataUsername(exchange), chainId, ruleMsg,
			types.WithContext(execCtx),
			types.WithOnEnd(func(ctx types.RuleContext, endMsg types.RuleMsg, err error, relationType string) {
				if err != nil {
					exchange.Out.SetError(err)
				}
				// Set the output message on exchange.Out so the processor can read it
				exchange.Out.SetMsg(&endMsg)
				// Use openaiStreamingResponse processor to format the response
				if p, ok := processor.OutBuiltins.Get("openaiStreamingResponse"); ok {
					p(nil, exchange)
				}
			}),
		); err != nil {
			writeBadRequest(exchange, err)
		}
		return true
	}).End())
}

// transformRuleMsg 预处理消息：设置 ID、类型、headers 到 metadata
func (s *Server) transformRuleMsg(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	if msgId := exchange.In.GetParam(constants.KeyMsgId); msgId != "" {
		msg.Id = msgId
	}
	msg.Type = exchange.In.GetMsg().Metadata.GetValue(constants.KeyMsgType)
	if exchange.In.GetMsg().Metadata.GetValue(constants.ParamHeadersToMetadata) == types.ValueTrue {
		headers := exchange.In.Headers()
		for k := range headers {
			msg.Metadata.PutValue(k, headers.Get(k))
		}
	}
	return true
}

// resolveStartNode 从查询参数中读取 _fromNodeId 和 _onlyNodeId，设置 _targetNodePath 元数据
func (s *Server) resolveStartNode(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
	msg := exchange.In.GetMsg()
	onlyNodeId := msg.Metadata.GetValue(constants.ParamOnlyNodeId)
	fromNodeId := msg.Metadata.GetValue(constants.ParamFromNodeId)
	if onlyNodeId != "" {
		msg.Metadata.PutValue(constants.ParamTargetNodePath, ":"+onlyNodeId)
		msg.Metadata.PutValue(types.KeySkipTellNext, types.ValueTrue)
	} else if fromNodeId != "" {
		msg.Metadata.PutValue(constants.ParamTargetNodePath, ":"+fromNodeId)
	} else {
		msg.Metadata.PutValue(constants.ParamTargetNodePath, "")
	}
	return true
}

// runLogOpts 根据 SaveRunLog 配置返回 WithOnRuleChainCompleted 选项
func (s *Server) runLogOpts() []types.RuleContextOption {
	if !s.config.SaveRunLog {
		return nil
	}
	runLogSvc, err := app.GetAs[services.RunLogService](s.container, services.KeyRunLogService)
	if err != nil {
		return nil
	}
	return []types.RuleContextOption{
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			_ = runLogSvc.SaveRunLog(metadataUsernameFromCtx(ctx), ctx, snapshot)
		}),
	}
}

// metadataUsernameFromCtx 从规则链上下文获取用户名
func metadataUsernameFromCtx(ctx types.RuleContext) string {
	if chainCtx, ok := ctx.RuleChain().(types.ChainCtx); ok {
		if def := chainCtx.Definition(); def != nil {
			if v, ok := def.RuleChain.GetAdditionalInfo(constants.KeyUsername); ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}
