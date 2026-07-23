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
		// Prioritize obtaining URL path parameters (backward compatible), then try the Authorization header
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

	// GET/POST/DELETE/mcp/:apiKey - MCP StreamableHTTP endpoint (default group, all tools)
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
			// The REST framework reads and closes the body at GetMsg(), and needs to be restored for the MCP handler to read
			r.Request().Body = io.NopCloser(bytes.NewReader(r.Body()))
			if err := mcpSvc.HandleMCP(username, w.Response(), r.Request()); err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				return false
			}
		}
		return true
	}

	ep.GET(endpoint.NewRouter().From(base + "/mcp/:apiKey").Process(mcpHandler).End())
	ep.POST(endpoint.NewRouter().From(base + "/mcp/:apiKey").Process(mcpHandler).End())
	ep.DELETE(endpoint.NewRouter().From(base + "/mcp/:apiKey").Process(mcpHandler).End())

	// GET/POST/DELETE /mcp/:apiKey/group/:group - MCP grouping StreamableHTTP endpoints
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
			// The REST framework reads and closes the body at GetMsg(), and needs to be restored for the MCP handler to read
			r.Request().Body = io.NopCloser(bytes.NewReader(r.Body()))
			if err := mcpSvc.HandleGroupMCP(username, groupName, w.Response(), r.Request()); err != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				return false
			}
		}
		return true
	}

	ep.GET(endpoint.NewRouter().From(base + "/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
	ep.POST(endpoint.NewRouter().From(base + "/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
	ep.DELETE(endpoint.NewRouter().From(base + "/mcp/:apiKey/group/:group").Process(mcpGroupHandler).End())
}
