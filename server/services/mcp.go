package services

import (
	"context"
	"net/http"

	"github.com/rulego/rulego/api/types"
)

// mcpRequestingUserKey 用于 context 中传递 MCP 请求用户身份的 key。
// 多用户场景下，系统智能体部署在 DefaultUsername 的 pool 中，
// 但 MCP 工具需要操作请求用户自己的规则链，通过此 key 透传请求用户身份。
var mcpRequestingUserKey = &struct{ string }{"mcp_requesting_user"}

// ContextWithMCPRequestingUser 向 context 中注入请求用户身份。
// 用于规则链执行时通过 types.WithContext 传递，确保 MCP "self" 工具操作正确的用户空间。
func ContextWithMCPRequestingUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, mcpRequestingUserKey, username)
}

// MCPRequestingUserFromContext 从 context 中获取请求用户身份。
func MCPRequestingUserFromContext(ctx context.Context) string {
	v, _ := ctx.Value(mcpRequestingUserKey).(string)
	return v
}

// McpService MCP 服务接口
type McpService interface {
	// HandleMCP 处理 MCP StreamableHTTP 请求（GET/POST/DELETE）
	HandleMCP(username string, w http.ResponseWriter, r *http.Request) error
	// HandleGroupMCP 处理分组 MCP StreamableHTTP 请求
	HandleGroupMCP(username, groupName string, w http.ResponseWriter, r *http.Request) error
	// AddToolsFromComponent 从组件添加工具
	AddToolsFromComponent(username, componentType string, def types.ComponentForm)
	// DeleteTools 删除工具
	DeleteTools(username string, names ...string)
	// LoadTools 加载用户的工具（组件、规则链、管理API）
	LoadTools(username string)
	// AddToolsFromChain 从规则链定义添加工具
	AddToolsFromChain(username, chainId string, def types.RuleChain)
	// Callbacks 返回规则链变更回调，用于动态同步 MCP 工具
	Callbacks(username string) types.Callbacks
	// RegisterTool 注册自定义 MCP 工具到指定用户的 MCP Server。
	// 工具同时注册到本地 toolDefs 缓存，MCPToolProvider 也能发现该工具。
	RegisterTool(username, name, description string, inputSchema []byte,
		handler func(ctx context.Context, args map[string]interface{}) (string, error)) error
}
