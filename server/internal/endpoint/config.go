package endpoint

import (
	"encoding/json"
	"sort"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerConfigRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /config/global - 获取全局配置
	ep.GET(endpoint.NewRouter().From(base+"/config/global").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		configSvc, ok := getService[services.ConfigService](s, exchange, services.KeyConfigService)
		if !ok {
			return false
		}
		cfg, err := configSvc.GetConfig()
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if cfg.Global != nil {
			writeJSON(exchange, cfg.Global)
		} else {
			exchange.Out.SetBody([]byte("{}"))
		}
		return true
	}).End())

	// POST /config/global - 更新全局配置
	ep.POST(endpoint.NewRouter().From(base+"/config/global").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		configSvc, ok := getService[services.ConfigService](s, exchange, services.KeyConfigService)
		if !ok {
			return false
		}
		var req map[string]interface{}
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := configSvc.UpdateConfig(req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		// 全局配置可能含密钥类字段：审计只记变更了哪些键名（回答"改了什么范围"），
		// 永不落键值。键名排序后截断，防止超长 payload 撑爆 Detail。
		keys := make([]string, 0, len(req))
		for k := range req {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		detail := "变更键: " + strings.Join(keys, ", ")
		if len(detail) > 200 {
			detail = detail[:200] + "…"
		}
		s.auditRecord(exchange, model.AuditEvent{
			Actor:  metadataUsername(exchange),
			Action: "config:write",
			Target: "config:global",
			Result: model.AuditResultOK,
			Detail: detail,
		})
		return true
	}).End())
}
