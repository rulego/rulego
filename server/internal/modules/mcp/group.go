// Package mcp MCP 端点分组逻辑
package mcp

import (
	"context"
	"net/http"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/str"
)

// MCPGroup MCP 工具分组
type MCPGroup struct {
	Name        string
	Description string
	Tools       []string // 包含的工具列表，支持 * 和 -prefix* 语法
}

// userMcpState 在 module.go 中定义，group 复用同一类型

// initGroups 初始化分组配置
func (m *Module) initGroups() {
	if m.cfg.MCP.Groups == nil {
		return
	}
	for name, toolsStr := range m.cfg.MCP.Groups {
		tools := strings.Split(toolsStr, ",")
		m.groups[name] = &MCPGroup{
			Name:  name,
			Tools: tools,
		}
	}
}

// getOrCreateGroupState 获取或创建用户的分组 MCP 状态
func (m *Module) getOrCreateGroupState(username, groupName string) (*userMcpState, error) {
	key := username + ":" + groupName

	m.mu.RLock()
	if state, ok := m.groupUsers[key]; ok {
		m.mu.RUnlock()
		return state, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// double-check
	if state, ok := m.groupUsers[key]; ok {
		return state, nil
	}

	apiKey := m.cfg.GetApiKeyByUsername(username)
	mcpServer := mcpserver.NewMCPServer("RuleGo MCP Server - "+groupName, "1.0.0")
	httpServer := mcpserver.NewStreamableHTTPServer(mcpServer,
		mcpserver.WithEndpointPath("/api/v1/mcp/"+apiKey+"/group/"+groupName),
	)

	state := &userMcpState{
		mcpServer:  mcpServer,
		httpServer: httpServer,
	}
	m.groupUsers[key] = state

	// 加载该组的工具
	m.loadGroupTools(state, username, groupName)

	return state, nil
}

// HandleGroupMCP 处理分组 MCP StreamableHTTP 请求（GET/POST/DELETE）
func (m *Module) HandleGroupMCP(username, groupName string, w http.ResponseWriter, r *http.Request) error {
	if !m.cfg.MCP.Enable {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("MCP is disabled"))
		return nil
	}

	if !m.isGroupExists(groupName) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Group not found: " + groupName))
		return nil
	}

	state, err := m.getOrCreateGroupState(username, groupName)
	if err != nil {
		return err
	}

	ctx := mcpContextWithValues(r.Context(), username, groupName)
	state.httpServer.ServeHTTP(w, r.WithContext(ctx))
	return nil
}

// isGroupExists 检查分组是否存在
func (m *Module) isGroupExists(groupName string) bool {
	if groupName == "default" {
		return true
	}
	_, ok := m.groups[groupName]
	return ok
}

// loadGroupTools 加载分组工具
func (m *Module) loadGroupTools(state *userMcpState, username, groupName string) {
	group, ok := m.groups[groupName]
	if !ok {
		return
	}

	// 解析工具过滤规则
	allowedTools, excludedPrefixes := parseToolFilter(group.Tools)

	// 加载规则链管理 API
	if containsToolType(allowedTools, "rules") || hasRuleApiTools(allowedTools) || len(allowedTools) == 0 {
		m.addGroupRuleApiTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}

	// 加载组件工具
	if containsToolType(allowedTools, "components") || len(allowedTools) == 0 {
		m.addGroupComponentTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}

	// 加载规则链工具
	if containsToolType(allowedTools, "chains") || len(allowedTools) == 0 {
		m.addGroupChainTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}
}

// addGroupRuleApiTools 添加分组的规则链管理 API 工具
func (m *Module) addGroupRuleApiTools(mcpServer *mcpserver.MCPServer, username string, allowedTools map[string]bool, excludedPrefixes []string) {
	ruleApiTools := []struct {
		name    string
		addFunc func(*mcpserver.MCPServer, string)
	}{
		{"list_rule_chains", func(s *mcpserver.MCPServer, u string) { m.addListRuleChainsTool(s, u) }},
		{"get_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addGetRuleChainTool(s, u) }},
		{"preview_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addPreviewRuleChainTool(s, u) }},
		{"save_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addSaveRuleChainTool(s, u) }},
		{"delete_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addDeleteRuleChainTool(s, u) }},
		{"operate_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addOperateRuleChainTool(s, u) }},
		{"execute_rule_chain", func(s *mcpserver.MCPServer, u string) { m.addExecuteRuleChainTool(s, u) }},
		{"list_components", func(s *mcpserver.MCPServer, u string) { m.addListComponentsTool(s, u) }},
		{"get_component_doc", func(s *mcpserver.MCPServer, u string) { m.addGetComponentDocTool(s, u) }},
	}

	for _, tool := range ruleApiTools {
		if isToolAllowed(tool.name, allowedTools, excludedPrefixes) {
			tool.addFunc(mcpServer, username)
		}
	}
}

// addGroupComponentTools 添加分组的组件工具
func (m *Module) addGroupComponentTools(mcpServer *mcpserver.MCPServer, username string, allowedTools map[string]bool, excludedPrefixes []string) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	components := ue.RuleConfig().ComponentsRegistry.GetComponentForms()
	for name, component := range components {
		if isToolAllowed(name, allowedTools, excludedPrefixes) {
			m.addToolsFromComponent(mcpServer, name, component)
		}
	}
}

// addGroupChainTools 添加分组的规则链工具
func (m *Module) addGroupChainTools(mcpServer *mcpserver.MCPServer, username string, allowedTools map[string]bool, excludedPrefixes []string) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	ue.Pool().Pool().Range(func(key, value any) bool {
		if item, ok := value.(*engine.RuleEngine); ok {
			id := str.ToString(key)
			if isToolAllowed(id, allowedTools, excludedPrefixes) {
				def := item.Definition()
				m.addToolsFromChain(mcpServer, id, def)
			}
		}
		return true
	})
}

// parseToolFilter 解析工具过滤规则
// 返回：allowedTools（精确匹配的工具名），excludedPrefixes（排除的前缀）
// 如果 tools 为空或只包含 "*"，返回 nil, nil 表示全部允许
func parseToolFilter(tools []string) (map[string]bool, []string) {
	if len(tools) == 0 {
		return nil, nil
	}

	allowedTools := make(map[string]bool)
	var excludedPrefixes []string
	hasWildcard := false

	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}

		if tool == "*" {
			hasWildcard = true
			continue
		}

		if strings.HasPrefix(tool, "-") {
			// 排除规则
			pattern := tool[1:]
			if strings.HasSuffix(pattern, "*") {
				// 排除前缀
				excludedPrefixes = append(excludedPrefixes, pattern[:len(pattern)-1])
			} else {
				// 精确排除（转为前缀处理）
				excludedPrefixes = append(excludedPrefixes, pattern)
			}
		} else {
			// 包含规则
			allowedTools[tool] = true
		}
	}

	// 如果有通配符且没有指定具体工具，返回 nil 表示全部允许（但保留排除规则）
	if hasWildcard && len(allowedTools) == 0 {
		return nil, excludedPrefixes
	}

	// 如果没有指定任何规则，返回 nil 表示全部允许
	if len(allowedTools) == 0 && len(excludedPrefixes) == 0 {
		return nil, nil
	}

	return allowedTools, excludedPrefixes
}

// isToolAllowed 检查工具是否允许
func isToolAllowed(toolName string, allowedTools map[string]bool, excludedPrefixes []string) bool {
	// 检查是否被排除
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(toolName, prefix) {
			return false
		}
	}

	// 如果没有指定允许的工具，则全部允许
	if len(allowedTools) == 0 {
		return true
	}

	// 检查是否在允许列表中
	return allowedTools[toolName]
}

// containsToolType 检查工具类型是否在允许列表中
func containsToolType(allowedTools map[string]bool, toolType string) bool {
	if len(allowedTools) == 0 {
		return true
	}
	return allowedTools[toolType]
}

// hasRuleApiTools 检查是否有任何规则链管理 API 工具在允许列表中
func hasRuleApiTools(allowedTools map[string]bool) bool {
	ruleApiNames := []string{
		"list_rule_chains", "get_rule_chain", "preview_rule_chain",
		"save_rule_chain", "delete_rule_chain",
		"operate_rule_chain",
		"execute_rule_chain",
		"list_components", "get_component_doc",
	}
	for _, name := range ruleApiNames {
		if allowedTools[name] {
			return true
		}
	}
	return false
}

// mcpContextWithValues 创建包含 MCP 相关值的 context
func mcpContextWithValues(ctx context.Context, username, groupName string) context.Context {
	ctx = context.WithValue(ctx, usernameKey, username)
	ctx = context.WithValue(ctx, groupKey, groupName)
	return ctx
}
