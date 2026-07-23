// Package MCP MCP endpoint packet logic
package mcp

import (
	"context"
	"net/http"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/str"
)

// MCPGroup MCP tool grouping
type MCPGroup struct {
	Name        string
	Description string
	Tools       []string // Includes a list of tools that supports * and -prefix* syntax
}

// userMcpState is defined in module.go, and group reuses the same type

// initGroups initializes the group configuration
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

// getOrCreateGroupState Retrieves or creates the user's grouped MCP status
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

	// Tools for loading the group
	m.loadGroupTools(state, username, groupName)

	return state, nil
}

// HandleGroupMCP Handles Packet MCP StreamableHTTP Request (GET/POST/DELETE)
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

// isGroupExists checks whether a group exists
func (m *Module) isGroupExists(groupName string) bool {
	if groupName == "default" {
		return true
	}
	_, ok := m.groups[groupName]
	return ok
}

// loadGroupTools Load grouping tools
func (m *Module) loadGroupTools(state *userMcpState, username, groupName string) {
	group, ok := m.groups[groupName]
	if !ok {
		return
	}

	// Parsing tool filtering rules
	allowedTools, excludedPrefixes := parseToolFilter(group.Tools)

	// Load the Rule Chain Management API
	if containsToolType(allowedTools, "rules") || hasRuleApiTools(allowedTools) || len(allowedTools) == 0 {
		m.addGroupRuleApiTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}

	// Load component tool
	if containsToolType(allowedTools, "components") || len(allowedTools) == 0 {
		m.addGroupComponentTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}

	// Load the rule chain tool
	if containsToolType(allowedTools, "chains") || len(allowedTools) == 0 {
		m.addGroupChainTools(state.mcpServer, username, allowedTools, excludedPrefixes)
	}
}

// addGroupRuleApiTools An API tool for managing the rule chain of groups for adding groups
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

// addGroupComponentTools Tool for adding grouped components
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

// addGroupChainTools is a tool for adding a rulechain of packets
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

// parseToolFilter parsing tool filtering rules
// Returns: allowedTools (exact matching tool name), excludedPrefixes (excluded prefixes)
// If tools is empty or only contains "*", return nil, nil means all allowed
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
			// Exclusion rule
			pattern := tool[1:]
			if strings.HasSuffix(pattern, "*") {
				// Remove prefixes
				excludedPrefixes = append(excludedPrefixes, pattern[:len(pattern)-1])
			} else {
				// Precise Exclusion (Conversion to Prefix Processing)
				excludedPrefixes = append(excludedPrefixes, pattern)
			}
		} else {
			// Include rules
			allowedTools[tool] = true
		}
	}

	// If wildcards exist and no specific tool is specified, returning nil means all are allowed (but the exclusion rule is retained).
	if hasWildcard && len(allowedTools) == 0 {
		return nil, excludedPrefixes
	}

	// If no rules are specified, returning nil means all are allowed
	if len(allowedTools) == 0 && len(excludedPrefixes) == 0 {
		return nil, nil
	}

	return allowedTools, excludedPrefixes
}

// isToolAllowed checks whether the tool is allowed
func isToolAllowed(toolName string, allowedTools map[string]bool, excludedPrefixes []string) bool {
	// Check if it has been excluded
	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(toolName, prefix) {
			return false
		}
	}

	// If no tools are specified, all are allowed
	if len(allowedTools) == 0 {
		return true
	}

	// Check if you are on the allowlist
	return allowedTools[toolName]
}

// containsToolType checks whether the tool type is on the allowlist
func containsToolType(allowedTools map[string]bool, toolType string) bool {
	if len(allowedTools) == 0 {
		return true
	}
	return allowedTools[toolType]
}

// hasRuleApiTools checks whether any rule chain management API tools are on the allowlist
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

// mcpContextWithValues creates a context containing MCP-related values
func mcpContextWithValues(ctx context.Context, username, groupName string) context.Context {
	ctx = context.WithValue(ctx, usernameKey, username)
	ctx = context.WithValue(ctx, groupKey, groupName)
	return ctx
}
