package services

import (
	"context"
	"net/http"

	"github.com/rulego/rulego/api/types"
)

// mcpRequestingUserKey is used to pass the key for the MCP requesting user identity in context.
// In multi-user scenarios, the system agent is deployed in the DefaultUsername pool,
// However, the MCP tool requires the requesting user's own rule chain, and through this key, the requester's identity is propagated.
var mcpRequestingUserKey = &struct{ string }{"mcp_requesting_user"}

// ContextWithMCPRequestingUser injects the requesting user's identity into the context.
// Used for rule chain execution via types.WithContext to ensure the MCP "self" tool operates correctly in the user space.
func ContextWithMCPRequestingUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, mcpRequestingUserKey, username)
}

// MCPRequestingUserFromContext Retrieves the requesting user's identity from the context.
func MCPRequestingUserFromContext(ctx context.Context) string {
	v, _ := ctx.Value(mcpRequestingUserKey).(string)
	return v
}

// McpService MCP service interface
type McpService interface {
	// HandleMCP Handles MCP StreamableHTTP Requests (GET/POST/DELETE)
	HandleMCP(username string, w http.ResponseWriter, r *http.Request) error
	// HandleGroupMCP handles packet MCP StreamableHTTP requests
	HandleGroupMCP(username, groupName string, w http.ResponseWriter, r *http.Request) error
	// AddToolsFromComponent: Adds tools from a component
	AddToolsFromComponent(username, componentType string, def types.ComponentForm)
	// DeleteTools Removal tool
	DeleteTools(username string, names ...string)
	// LoadTools Tools for loading users (components, rule chains, management APIs)
	LoadTools(username string)
	// AddToolsFromChain Adds tools from the rule chain definition
	AddToolsFromChain(username, chainId string, def types.RuleChain)
	// Callbacks return rule chain change callbacks used for dynamically synchronizing MCP tools
	Callbacks(username string) types.Callbacks
	// RegisterTool registers a custom MCP tool to the specified user's MCP Server.
	// The tool is also registered in the local toolDefs cache, and MCPToolProvider can also discover the tool.
	RegisterTool(username, name, description string, inputSchema []byte,
		handler func(ctx context.Context, args map[string]interface{}) (string, error)) error
}
