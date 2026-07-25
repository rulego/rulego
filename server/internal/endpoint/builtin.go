package endpoint

import (
	"encoding/json"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/registry"
)

// registerBuiltinRoutes 注册动态组件配置查询路由，处理器由构建标签按需注册（如 with_iot 注册 OPC UA 在线浏览）
func (s *Server) registerBuiltinRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// POST /builtins/:name - 动态组件配置查询（body 为查询参数）
	ep.POST(endpoint.NewRouter().From(base+"/builtins/:name").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		name := strings.TrimSpace(metadataValue(exchange, "name"))
		var params map[string]interface{}
		if len(exchange.In.Body()) > 0 {
			if err := json.Unmarshal(exchange.In.Body(), &params); err != nil {
				writeBadRequest(exchange, err)
				return false
			}
		}
		result, err := registry.QueryDynamicBuiltin(name, params)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		writeJSON(exchange, result)
		return true
	}).End())
}
