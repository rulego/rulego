package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/modules/system"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerConfigRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /config/global - 获取全局配置（敏感键值掩码显示，明文走 reveal 端点）
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
		result := make(map[string]string, len(cfg.Global))
		for k, v := range cfg.Global {
			if system.IsSensitiveKey(k) {
				result[k] = system.MaskedValue
			} else {
				result[k] = v
			}
		}
		writeJSON(exchange, result)
		return true
	}).End())

	// GET /config/global/reveal/:key - 查看敏感键明文（需写权限，落审计）
	ep.GET(endpoint.NewRouter().From(base+"/config/global/reveal/:key").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		key := metadataValue(exchange, "key")
		configSvc, ok := getService[services.ConfigService](s, exchange, services.KeyConfigService)
		if !ok {
			return false
		}
		cfg, err := configSvc.GetConfig()
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		v, exists := cfg.Global[key]
		if !exists {
			exchange.Out.SetStatusCode(http.StatusNotFound)
			writeJSON(exchange, map[string]string{"error": "key not found"})
			return true
		}
		s.auditRecord(exchange, model.AuditEvent{
			Actor:  metadataUsername(exchange),
			Action: "config:reveal",
			Target: "config:global:" + key,
			Result: model.AuditResultOK,
		})
		writeJSON(exchange, map[string]string{"key": key, "value": v})
		return true
	}).End())

	// GET /config/global/overrides - 运行时覆盖清单（data/config.json 留存的键，
	// 原始写法、敏感值掩码）。前端据此标记可「恢复文件值」的键
	ep.GET(endpoint.NewRouter().From(base+"/config/global/overrides").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		configSvc, ok := getService[services.ConfigService](s, exchange, services.KeyConfigService)
		if !ok {
			return false
		}
		overrides, err := configSvc.GlobalOverrides()
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, overrides)
		return true
	}).End())

	// POST /config/global/revert/:key - 删除某键的运行时覆盖，恢复为 config.conf 文件值
	ep.POST(endpoint.NewRouter().From(base+"/config/global/revert/:key").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		key := metadataValue(exchange, "key")
		if key == "" {
			writeBadRequest(exchange, fmt.Errorf("key is required"))
			return false
		}
		configSvc, ok := getService[services.ConfigService](s, exchange, services.KeyConfigService)
		if !ok {
			return false
		}
		reload, err := configSvc.RestoreGlobalKey(key)
		if err != nil {
			writeBadRequest(exchange, err)
			return true
		}
		s.auditRecord(exchange, model.AuditEvent{
			Actor:  metadataUsername(exchange),
			Action: "config:revert",
			Target: "config:global:" + key,
			Result: model.AuditResultOK,
			Detail: "恢复为 config.conf 文件值",
		})
		writeJSON(exchange, reload)
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
		reload, err := configSvc.UpdateConfig(req)
		if err != nil {
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
		if reload == nil {
			reload = &services.GlobalReloadResult{}
		}
		writeJSON(exchange, reload)
		return true
	}).End())
}
