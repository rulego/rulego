package endpoint

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/services"
)

func (s *Server) registerMCPRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	mcpAuth := func(exchange *endpointApi.Exchange) (string, bool) {
		// 优先从 URL 路径参数获取（向后兼容），然后尝试 Authorization header
		apiKey := exchange.In.GetMsg().Metadata.GetValue(constants.MetaApiKey)
		if apiKey == "" {
			if auth := exchange.In.Headers().Get("Authorization"); auth != "" {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if apiKey == "" {
			apiKey = exchange.In.Headers().Get("X-API-Key")
		}
		if apiKey == "" {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			return "", false
		}
		username := s.config.GetUsernameByApiKey(apiKey)
		if username == "" {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			return "", false
		}
		return username, true
	}

	// GET/POST/DELETE /mcp/:apiKey - MCP StreamableHTTP 端点（默认组，全部工具）
	mcpHandler := func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username, ok := mcpAuth(exchange)
		if !ok {
			return false
		}
		mcpSvc, ok := getService[services.McpService](s, exchange, services.KeyMcpService)
		if !ok {
			return false
		}
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			// REST 框架在 GetMsg() 时已读取并关闭 body，需要恢复供 MCP handler 读取
			r.Request().Body = io.NopCloser(bytes.NewReader(r.Body()))
			if err := mcpSvc.HandleMCP(username, w.Response(), r.Request()); err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				return false
			}
		}
		return true
	}

	ep.GET(endpoint.NewRouter().From(base+"/mcp/:apiKey").Process(mcpHandler).End())
	ep.POST(endpoint.NewRouter().From(base+"/mcp/:apiKey").Process(mcpHandler).End())
	ep.DELETE(endpoint.NewRouter().From(base+"/mcp/:apiKey").Process(mcpHandler).End())

	// GET/POST/DELETE /mcp/:apiKey/group/:group - MCP 分组 StreamableHTTP 端点
	mcpGroupHandler := func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username, ok := mcpAuth(exchange)
		if !ok {
			return false
		}
		groupName := exchange.In.GetMsg().Metadata.GetValue("group")
		if groupName == "" {
			exchange.Out.SetStatusCode(http.StatusBadRequest)
			return false
		}
		mcpSvc, ok := getService[services.McpService](s, exchange, services.KeyMcpService)
		if !ok {
			return false
		}
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			// REST 框架在 GetMsg() 时已读取并关闭 body，需要恢复供 MCP handler 读取
			r.Request().Body = io.NopCloser(bytes.NewReader(r.Body()))
			if err := mcpSvc.HandleGroupMCP(username, groupName, w.Response(), r.Request()); err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				return false
			}
		}
		return true
	}

	ep.GET(endpoint.NewRouter().From(base+"/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
	ep.POST(endpoint.NewRouter().From(base+"/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
	ep.DELETE(endpoint.NewRouter().From(base+"/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
}
