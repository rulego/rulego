package endpoint

import (
	"encoding/json"

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
		// 全局配置可能含密钥类字段：审计只记事件，不落任何键名与值
		s.auditRecord(exchange, model.AuditEvent{
			Actor:  metadataUsername(exchange),
			Action: "config:write",
			Target: "config:global",
			Result: model.AuditResultOK,
		})
		return true
	}).End())
}
