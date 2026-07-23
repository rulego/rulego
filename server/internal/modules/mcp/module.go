// Package MCP implements MCP (Model Context Protocol) endpoint and tool management.
//
// # Overview
//
// This module provides MCP Server capabilities for RuleGo, allowing AI agents to control the rule engine through standardized protocols.
// Supports two access methods:
//   - Streamable HTTP Remote Access: External AI clients (such as Claude Desktop, Cursor, etc.) connect via HTTP
//   - In-process access: RuleGo's internal agent nodes are directly called through the MCPToolProvider interface
//
// # Configuration
//
// MCP functionality is configured via MCPConfig (config.ini or JSON configuration):
//
//	[mcp]
//	enable = true
//	By default, endpoints are fixed to load management API tools, while component and rule chain tools load through grouped configurations.
//
// # HTTP endpoint
//
// Default group (includes all tools for the user):
//
//	GET/POST/DELETE /api/v1/mcp/{apiKey} # MCP StreamableHTTP endpoint
//
// Grouping (configured via MCPConfig.Groups, controlling subset of tools):
//
//	GET/POST/DELETE /api/v1/mcp/{apiKey}/group/{groupName}
//
// # Group configuration
//
// Groups are procedurally configured (map[string]string), with the key being the group name and the value being the tool list.
// There are no built-in default groups within the group, so you need to configure them yourself.
//
// Syntax:
//   - Name of a comma separator
//   - * indicates all tools
//   - -prefix* indicates a tool to exclude prefix matches
//   - rules = management API tool, components = component tool, chains = rule chain tool
//
// Example:
//
//	Groups: map[string]string{
//	  "readonly":  "rules,list_components,get_component_doc",
//	  "full":      "*",
//	  "no-delete": "*,-delete_rule_chain",
//	}
//
// # MCP tool list
//
// Management API tools (default endpoint fixed loading):
//
//	Tool name | Function
//	list_rule_chains | List/search rule chain (supports pagination and keyword filtering)
//	get_rule_chain | Obtain the rule chain definition JSON (for viewing or modification)
//	preview_rule_chain | Preview the rule chain (check + return JSON, do not save)
//	save_rule_chain | Create or update a rule chain (including node field validation)
//	delete_rule_chain | Delete the rule chain
//	operate_rule_chain | Operation Rule Chain (deploy/undeploy)
//	execute_rule_chain | Execute the rule chain and return the result
//	list_components | List components (including categories, fields, and join types)
//	get_component_doc | Obtain complete component documentation (supports batch queries)
//
// Component Tool (loaded via group configuration):
//
//	Each registered component automatically becomes a standalone tool, named after the component type (e.g., jsFilter, restApiCall).
//	Parameters are automatically generated from the component's ComponentForm.Fields, including field name, type, description, default value, and required tags.
//
// Rule chain tool (loaded via group configuration):
//
//	Each deployed rule chain automatically becomes an independent tool, named Rule Chain ID.
//	Parameters come from the inputSchema or DSL template variable parsing of the rule chain.
//	When the rule chain changes, it is dynamically synchronized via callbacks (OnNew/OnUpdated/OnDeleted).
//
// # External client configuration
//
// Claude Desktop（claude_desktop_config.json）：
//
//	{
//	  "mcpServers": {
//	    "rulego": {
//	      "url": "http://localhost:8080/api/v1/mcp/YOUR_API_KEY"
//	    }
//	  }
//	}
//
// In-process agent (rule chain JSON configuration):
//
//	{
//	  "type": "mcp",
//	  "config": {
//	    "server": "self",
//	    "tools": ["list_rule_chains", "get_rule_chain", "save_rule_chain"]
//	  }
//	}
//
// Tools array as filter: Only the tools listed will be loaded into the large model context. Use "*" to load all.
//
// # MCPToolProvider interface
//
// Module implements types.MCPToolProvider interface, for use by internal agent nodes:
//   - ListToolDefinitions() — Returns definitions for all registered tools
//   - CallTool(ctx, toolName, args) — Calls tools by name
//   - RegisterTool(username, name, desc, schema, handler) — Register custom tools
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/processor"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/registry"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/utils/dsl"
	"github.com/rulego/rulego/utils/str"
)

const (
	ModuleName = "mcp"
	Priority   = 25
)

// userMcpState The MCP status of each user
type userMcpState struct {
	mcpServer  *mcpserver.MCPServer
	httpServer *mcpserver.StreamableHTTPServer
}

// Module mcp business module, responsible for managing MCP SSE/HTTP endpoints and tools
type Module struct {
	cfg       *config.Config
	logger    types.Logger
	engineMgr services.EngineManager
	catalog   services.ChainCatalog
	admin     services.RuleAdminService
	executor  services.ChainExecutor
	nodeSvc   services.NodeService
	container *app.Container

	mu           sync.RWMutex
	users        map[string]*userMcpState
	groupUsers   map[string]*userMcpState           // MCP status of group users
	groups       map[string]*MCPGroup               // Group definition
	toolDefs     map[string]toolDefEntry            // Tool-defined cache (name + schema, same for all users)
	userToolDefs map[string]map[string]toolDefEntry // Tool handler for each user(username -> toolName -> entry)
}

// New to create the MCP module
func New() *Module {
	return &Module{
		users:        make(map[string]*userMcpState),
		groupUsers:   make(map[string]*userMcpState),
		groups:       make(map[string]*MCPGroup),
		toolDefs:     make(map[string]toolDefEntry),
		userToolDefs: make(map[string]map[string]toolDefEntry),
	}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger
	m.container = ctx.Container
	// Service resolution is delayed until the Start phase, because the service in the rule module is in the rule.Init().
	// Set priority 25< rule(30) to ensure mcp.Start() executes before rule.Start().
	return ctx.Container.Register(services.KeyMcpService, services.McpService(m))
}

func (m *Module) Start(ctx context.Context) error {
	if m.cfg == nil || !m.cfg.MCP.Enable {
		return nil
	}
	// Parsing Service (rule.Init() registered)
	engineMgr, err := app.GetAs[services.EngineManager](m.container, constants.SvcRuleEngineManager)
	if err != nil {
		return err
	}
	m.engineMgr = engineMgr

	catalog, err := app.GetAs[services.ChainCatalog](m.container, constants.SvcRuleCatalog)
	if err != nil {
		return err
	}
	m.catalog = catalog

	admin, err := app.GetAs[services.RuleAdminService](m.container, constants.SvcRuleManager)
	if err != nil {
		return err
	}
	m.admin = admin

	executor, err := app.GetAs[services.ChainExecutor](m.container, constants.SvcRuleExecutor)
	if err != nil {
		return err
	}
	m.executor = executor

	nodeSvc, err := app.GetAs[services.NodeService](m.container, constants.SvcNodeService)
	if err != nil {
		return err
	}
	m.nodeSvc = nodeSvc

	// Initialize grouping
	m.initGroups()

	// Register MCPToolProvider and load the tool
	for username := range m.cfg.Users {
		m.loadUserTools(username)
	}
	if m.cfg.DefaultUsername != "" {
		if _, ok := m.cfg.Users[m.cfg.DefaultUsername]; !ok {
			m.loadUserTools(m.cfg.DefaultUsername)
		}
	}
	// Generate global variables file for agent prompts
	m.generateGlobalVarsFile()
	return nil
}

// generateGlobalVarsFile generates a global variable name list file for the agent prompt to reference.
// Only variable names are included, not values, preventing sensitive information leaks.
func (m *Module) generateGlobalVarsFile() {
	if m.cfg.Global == nil || len(m.cfg.Global) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("可用全局变量（通过 `${global.xxx}` 引用）：\n\n")
	keys := make([]string, 0, len(m.cfg.Global))
	for k := range m.cfg.Global {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteString("- " + k + "\n")
	}

	dir := filepath.Join(m.cfg.DataDir, "system")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "global_vars.md"), []byte(sb.String()), 0644)
}

// loadUserTools loads tools for the specified user and sets callbacks
func (m *Module) loadUserTools(username string) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	// Register MCP ToolProvider to RuleConfig UDF for internal agent use.
	// Use userMCPProvider wrappers to ensure that CallTool in "self" mode injects the correct username.
	// Udf is a map type (reference type) that is written directly to the original Config.
	if m.cfg.MCP.Enable {
		cfg := ue.RuleConfig()
		if cfg.Udf == nil {
			cfg.Udf = make(map[string]interface{})
		}
		cfg.Udf[types.MCPToolProviderKey] = &userMCPProvider{module: m, username: username}
	}
	// Set rule chain change callback (always set to ensure packet endpoints can synchronize dynamically)
	ue.Pool().SetCallbacks(m.Callbacks(username))
	// Loading tools
	m.LoadTools(username)
}
func (m *Module) Stop(_ context.Context) error { return nil }

// getOrCreateState to obtain or create the user's MCP status
func (m *Module) getOrCreateState(username string) (*userMcpState, error) {
	m.mu.RLock()
	state, ok := m.users[username]
	m.mu.RUnlock()
	if ok {
		return state, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// double-check
	if state, ok = m.users[username]; ok {
		return state, nil
	}

	mcpServer := mcpserver.NewMCPServer("RuleGo MCP Server", "1.0.0")
	httpServer := mcpserver.NewStreamableHTTPServer(mcpServer,
		mcpserver.WithEndpointPath("/api/v1/mcp/"+m.cfg.GetApiKeyByUsername(username)),
	)

	state = &userMcpState{
		mcpServer:  mcpServer,
		httpServer: httpServer,
	}
	m.users[username] = state
	return state, nil
}

// HandleMCP Handles MCP StreamableHTTP Requests (GET/POST/DELETE)
func (m *Module) HandleMCP(username string, w http.ResponseWriter, r *http.Request) error {
	if !m.cfg.MCP.Enable {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("MCP is disabled"))
		return nil
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return err
	}
	ctx := context.WithValue(r.Context(), usernameKey, username)
	state.httpServer.ServeHTTP(w, r.WithContext(ctx))
	return nil
}

// AddToolsFromComponent: Adds tools from a component
func (m *Module) AddToolsFromComponent(username, componentType string, def types.ComponentForm) {
	if !m.cfg.MCP.Enable {
		return
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return
	}
	m.addToolsFromComponent(state.mcpServer, componentType, def)
}

// DeleteTools Removal tool
func (m *Module) DeleteTools(username string, names ...string) {
	if !m.cfg.MCP.Enable {
		return
	}
	m.mu.RLock()
	state, ok := m.users[username]
	m.mu.RUnlock()
	if ok {
		state.mcpServer.DeleteTools(names...)
		m.mu.Lock()
		for _, name := range names {
			delete(m.toolDefs, name)
			if userDefs, ok := m.userToolDefs[username]; ok {
				delete(userDefs, name)
			}
		}
		m.mu.Unlock()
	}
	// Synchronize the deletion of tools in the grouped MCP Server
	m.syncDeleteToGroups(username, names...)
}

// LoadTools to load users' tools
func (m *Module) LoadTools(username string) {
	if !m.cfg.MCP.Enable {
		return
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return
	}

	// By default, endpoints are fixed to load management API tools
	m.addRuleApiTools(state, username)
}

// AddToolsFromChain Adds tools from the rule chain definition
func (m *Module) AddToolsFromChain(username, chainId string, def types.RuleChain) {
	if !m.cfg.MCP.Enable {
		return
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return
	}
	m.addToolsFromChain(state.mcpServer, chainId, def)
	// Synchronously add to the packet MCP Server
	m.syncChainToGroups(username, chainId, def)
}

// Callbacks return a rule chain change callback
func (m *Module) Callbacks(username string) types.Callbacks {
	return types.Callbacks{
		OnUpdated: func(chainId, nodeId string, dslData []byte) {
			var def types.RuleChain
			if err := json.Unmarshal(dslData, &def); err == nil {
				m.AddToolsFromChain(username, chainId, def)
			}
		},
		OnDeleted: func(id string) {
			m.DeleteTools(username, id)
		},
		OnNew: func(chainId string, dslData []byte) {
			var def types.RuleChain
			if err := json.Unmarshal(dslData, &def); err == nil {
				m.AddToolsFromChain(username, chainId, def)
			}
		},
	}
}

// syncChainToGroups synchronizes the rule chain tool changes to all packet MCP Server for that user
func (m *Module) syncChainToGroups(username, chainId string, def types.RuleChain) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, groupState := range m.groupUsers {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || parts[0] != username {
			continue
		}
		groupName := parts[1]
		group, ok := m.groups[groupName]
		if !ok {
			continue
		}
		allowed, excluded := parseToolFilter(group.Tools)
		if isToolAllowed(chainId, allowed, excluded) {
			m.addToolsFromChain(groupState.mcpServer, chainId, def)
		}
	}
}

// syncDeleteToGroups synchronizes the tool to delete all groups on the MCP Server for that user
func (m *Module) syncDeleteToGroups(username string, names ...string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, groupState := range m.groupUsers {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || parts[0] != username {
			continue
		}
		groupState.mcpServer.DeleteTools(names...)
	}
}

// addToolsFromComponent: Adds MCP tools from component definitions
func (m *Module) addToolsFromComponent(mcpServer *mcpserver.MCPServer, name string, component types.ComponentForm) {
	var toolOptions []mcp.ToolOption
	for _, item := range component.Fields {
		var toolOption mcp.ToolOption
		desc := item.Name
		if item.Desc != "" {
			desc = item.Desc
		}
		propertyOptions := []mcp.PropertyOption{
			mcp.Description(desc),
		}
		if item.Required {
			propertyOptions = append(propertyOptions, mcp.Required())
		}
		switch item.Type {
		case "string":
			toolOption = mcp.WithString(item.Name, propertyOptions...)
		case "array", "slice":
			toolOption = mcp.WithArray(item.Name, propertyOptions...)
		case "map", "object", "struct":
			toolOption = mcp.WithObject(item.Name, propertyOptions...)
		case "bool", "boolean":
			toolOption = mcp.WithBoolean(item.Name, propertyOptions...)
		default:
			if strings.HasPrefix(item.Type, "int") || strings.HasPrefix(item.Type, "float") {
				toolOption = mcp.WithNumber(item.Name, propertyOptions...)
			} else {
				toolOption = mcp.WithString(item.Name, propertyOptions...)
			}
		}
		toolOptions = append(toolOptions, toolOption)
	}
	desc := name
	if component.Desc != "" {
		desc = component.Desc
	}
	toolOptions = append(toolOptions, mcp.WithDescription(desc))
	tool := mcp.NewTool(name, toolOptions...)
	mcpServer.AddTool(tool, m.componentToolHandler(name))
}

// componentToolHandler creates the handler function for component tools
func (m *Module) componentToolHandler(componentType string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Getting username from context (injected by SSE handler)
		username := getUsernameFromCtx(ctx)
		if username == "" {
			return nil, errors.New("username not found in context")
		}
		ue, err := m.engineMgr.GetOrCreate(username)
		if err != nil {
			return nil, err
		}
		ruleConfig := ue.RuleConfig()

		// Verify whether all parameters passed by the agent are fields present in the component definition
		if warnMsg := m.validateComponentArgs(ruleConfig, componentType, request.GetArguments()); warnMsg != "" {
			return mcp.NewToolResultText(warnMsg), nil
		}

		node, err := ruleConfig.ComponentsRegistry.NewNode(componentType)
		if err != nil {
			return nil, err
		}
		err = node.Init(ruleConfig, request.GetArguments())
		if err != nil {
			return nil, err
		}
		var msg string
		if params, ok := request.GetArguments()[constants.KeyInMessage]; ok {
			msg = str.ToString(params)
		} else if v, err := json.Marshal(request.GetArguments()); err != nil {
			return nil, err
		} else {
			msg = string(v)
		}
		wg := sync.WaitGroup{}
		wg.Add(1)
		var result string
		var resultErr error
		pool := ue.Pool()
		ruleCtx := engine.NewRuleContext(ctx, ruleConfig, nil, nil, nil, ruleConfig.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			result = msg.GetData()
			resultErr = err
			wg.Done()
		}, pool)
		node.OnMsg(ruleCtx, types.NewMsgWithJsonData(msg))
		wg.Wait()
		node.Destroy()
		return mcp.NewToolResultText(result), resultErr
	}
}

// validateComponentArgs verifies whether all parameters passed in by the agent are fields present in the component definition
// At the same time, check whether required fields are missing
// If there are fields that do not exist or required fields are missing, a prompt message is returned; Otherwise, it returns an empty string
func (m *Module) validateComponentArgs(ruleConfig types.Config, componentType string, args map[string]interface{}) string {
	components := ruleConfig.ComponentsRegistry.GetComponentForms()
	componentForm, ok := components[componentType]
	if !ok {
		return ""
	}
	// Collect all fields in the component definition
	validFields := make(map[string]bool)
	for _, field := range componentForm.Fields {
		validFields[field.Name] = true
	}
	var warnings []string
	// Check for redundant fields
	var unknownFields []string
	for key := range args {
		if !validFields[key] {
			unknownFields = append(unknownFields, key)
		}
	}
	if len(unknownFields) > 0 {
		warnings = append(warnings, fmt.Sprintf("unknown fields: %v", unknownFields))
	}
	// Check for missing required fields
	var missingFields []string
	for _, field := range componentForm.Fields {
		if field.Required {
			if _, ok := args[field.Name]; !ok {
				missingFields = append(missingFields, field.Name)
			}
		}
	}
	if len(missingFields) > 0 {
		warnings = append(warnings, fmt.Sprintf("missing required fields: %v", missingFields))
	}
	if len(warnings) > 0 {
		// Build a list of available fields
		var fieldDocs []string
		for _, field := range componentForm.Fields {
			suffix := ""
			if field.Required {
				suffix = " (required)"
			}
			fieldDocs = append(fieldDocs, fmt.Sprintf("  - %s %s%s", field.Name, field.Type, suffix))
		}
		return fmt.Sprintf("warning: component %s validation failed:\n%s\navailable fields:\n%s",
			componentType, strings.Join(warnings, "\n"), strings.Join(fieldDocs, "\n"))
	}
	return ""
}

// validateRuleChainNodes validates the configuration fields of all nodes in the rule chain
// At the same time, check whether required fields are missing
// If there are fields that do not exist or required fields are missing, a prompt message is returned; Otherwise, it returns an empty string
func (m *Module) validateRuleChainNodes(username string, chainData []byte) string {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return ""
	}
	var ruleChain types.RuleChain
	if err := json.Unmarshal(chainData, &ruleChain); err != nil {
		return ""
	}
	components := ue.RuleConfig().ComponentsRegistry.GetComponentForms()
	var nodeWarnings []string
	for _, node := range ruleChain.Metadata.Nodes {
		componentForm, ok := components[node.Type]
		if !ok {
			continue
		}
		validFields := make(map[string]bool)
		for _, field := range componentForm.Fields {
			validFields[field.Name] = true
		}
		var issues []string
		// Check for redundant fields
		var unknownFields []string
		for key := range node.Configuration {
			if !validFields[key] {
				unknownFields = append(unknownFields, key)
			}
		}
		if len(unknownFields) > 0 {
			issues = append(issues, fmt.Sprintf("unknown fields: %v", unknownFields))
		}
		// Check for missing required fields
		var missingFields []string
		for _, field := range componentForm.Fields {
			if field.Required {
				if _, ok := node.Configuration[field.Name]; !ok {
					missingFields = append(missingFields, field.Name)
				}
			}
		}
		if len(missingFields) > 0 {
			issues = append(issues, fmt.Sprintf("missing required fields: %v", missingFields))
		}
		if len(issues) > 0 {
			// Build a list of available fields
			var fieldDocs []string
			for _, field := range componentForm.Fields {
				suffix := ""
				if field.Required {
					suffix = " (required)"
				}
				fieldDocs = append(fieldDocs, fmt.Sprintf("    - %s %s%s", field.Name, field.Type, suffix))
			}
			nodeWarnings = append(nodeWarnings, fmt.Sprintf("  node %s(%s):\n    %s\n    available fields:\n%s",
				node.Id, node.Type, strings.Join(issues, "\n    "), strings.Join(fieldDocs, "\n")))
		}
	}
	if len(nodeWarnings) > 0 {
		return "warning: node validation failed:\n" + strings.Join(nodeWarnings, "\n")
	}
	return ""
}

// loadToolsFromComponents adds tools from the component list
func (m *Module) loadToolsFromComponents(username string, state *userMcpState) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	components := ue.RuleConfig().ComponentsRegistry.GetComponentForms()
	for name, component := range components {
		m.addToolsFromComponent(state.mcpServer, name, component)
	}
}

// loadToolsFromChains Adds tools from the list of rule chains
func (m *Module) loadToolsFromChains(username string, state *userMcpState) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	ue.Pool().Pool().Range(func(key, value any) bool {
		if item, ok := value.(*engine.RuleEngine); ok {
			id := str.ToString(key)
			def := item.Definition()
			m.addToolsFromChain(state.mcpServer, id, def)
		}
		return true
	})
}

// addToolsFromChain Adds MCP tools from the rule chain definition
func (m *Module) addToolsFromChain(mcpServer *mcpserver.MCPServer, chainId string, def types.RuleChain) {
	desc := def.RuleChain.Name
	if v := str.ToString(def.RuleChain.AdditionalInfo["description"]); v != "" {
		desc = v
	}
	if desc == "" {
		return
	}

	var tool mcp.Tool
	if inputSchemaMap, ok := def.RuleChain.AdditionalInfo["inputSchema"]; ok {
		if schema, err := json.Marshal(inputSchemaMap); err == nil {
			tool = mcp.NewToolWithRawSchema(chainId, desc, schema)
		}
	} else {
		vars := dsl.ParseVars(types.MsgKey, def)
		if len(vars) > 0 {
			var toolOptions []mcp.ToolOption
			for _, item := range vars {
				toolOptions = append(toolOptions, mcp.WithString(item, mcp.Required(), mcp.Description("input param: "+item)))
			}
			toolOptions = append(toolOptions, mcp.WithDescription(desc))
			tool = mcp.NewTool(chainId, toolOptions...)
		} else {
			tool = mcp.NewTool(chainId, mcp.WithDescription(desc), mcp.WithObject(constants.KeyInMessage, mcp.Description("input message")))
		}
	}
	mcpServer.AddTool(tool, m.ruleChainToolHandler(chainId))
}

// ruleChainToolHandler creates the handler function for the rulechain tool
func (m *Module) ruleChainToolHandler(chainId string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		username := getUsernameFromCtx(ctx)
		if username == "" {
			return nil, errors.New("username not found in context")
		}
		ue, err := m.engineMgr.GetOrCreate(username)
		if err != nil {
			return nil, err
		}
		ruleEngine, ok := ue.GetEngine(chainId)
		if !ok {
			return nil, fmt.Errorf("rule chain not found: %s", chainId)
		}

		var msg string
		if params, ok := request.GetArguments()[constants.KeyInMessage]; ok {
			msg = str.ToString(params)
		} else if v, err := json.Marshal(request.GetArguments()); err != nil {
			return nil, err
		} else {
			msg = string(v)
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		var result string
		var resultErr error
		ruleEngine.OnMsgAndWait(types.NewMsgWithJsonData(msg), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			result = msg.GetData()
			resultErr = err
			wg.Done()
		}))
		wg.Wait()
		return mcp.NewToolResultText(result), resultErr
	}
}

// --- Management API Tools ---

// addRuleApiTools Adds a rule chain management API tool
func (m *Module) addRuleApiTools(state *userMcpState, username string) {
	m.addListRuleChainsTool(state.mcpServer, username)
	m.addGetRuleChainTool(state.mcpServer, username)
	m.addPreviewRuleChainTool(state.mcpServer, username)
	m.addSaveRuleChainTool(state.mcpServer, username)
	m.addDeleteRuleChainTool(state.mcpServer, username)
	m.addOperateRuleChainTool(state.mcpServer, username)
	m.addExecuteRuleChainTool(state.mcpServer, username)
	m.addListComponentsTool(state.mcpServer, username)
	m.addGetComponentDocTool(state.mcpServer, username)
	m.addListNodePoolTool(state.mcpServer, username)
}

func (m *Module) addListRuleChainsTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("list_rule_chains",
		mcp.WithDescription("List/search rule chains"),
		mcp.WithString("keywords", mcp.Description("Keywords for filtering rule chains")),
		mcp.WithBoolean("root", mcp.Description("Filter by root rule chain")),
		mcp.WithBoolean("disabled", mcp.Description("Filter by disabled rule chain")),
		mcp.WithNumber("page", mcp.Description("Page number")),
		mcp.WithNumber("size", mcp.Description("Page size")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		keywords := strings.TrimSpace(str.ToString(request.GetArguments()["keywords"]))
		rootStr := strings.TrimSpace(str.ToString(request.GetArguments()["root"]))
		disabledStr := strings.TrimSpace(str.ToString(request.GetArguments()["disabled"]))
		var page = 1
		var size = 20
		if i, err := strconv.Atoi(str.ToString(request.GetArguments()["page"])); err == nil {
			page = i
		}
		if i, err := strconv.Atoi(str.ToString(request.GetArguments()["size"])); err == nil {
			size = i
		}
		var root *bool
		var disabled *bool
		if i, err := strconv.ParseBool(rootStr); err == nil {
			root = &i
		}
		if i, err := strconv.ParseBool(disabledStr); err == nil {
			disabled = &i
		}
		list, count, err := m.catalog.List(username, keywords, root, disabled, "", size, page)
		if err != nil {
			return nil, err
		}
		result := map[string]interface{}{
			"total": count,
			"page":  page,
			"size":  size,
			"items": list,
		}
		if v, err := json.Marshal(result); err == nil {
			return mcp.NewToolResultText(string(v)), nil
		} else {
			return nil, err
		}
	})
}

func (m *Module) addGetRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("get_rule_chain",
		mcp.WithDescription("Get rule chain definition by id. Returns the current rule chain JSON for review or modification. Use save_rule_chain to save changes."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := str.ToString(request.GetArguments()["id"])
		if id == "" {
			return nil, errors.New("id is required")
		}
		data, err := m.catalog.GetAsRuleChain(username, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get rule chain: %w", err)
		}
		result, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (m *Module) addSaveRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("save_rule_chain",
		mcp.WithDescription("Create or update a rule chain (save and deploy)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
		mcp.WithObject("body", mcp.Required(), mcp.Description("Rule chain definition")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, ok := request.GetArguments()["id"]
		if !ok {
			return nil, errors.New("id is required")
		}
		body, ok := request.GetArguments()["body"]
		if !ok {
			return nil, errors.New("body is required")
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		// Check whether all fields in the node configuration are those present in the component definition
		if warnMsg := m.validateRuleChainNodes(username, b); warnMsg != "" {
			return mcp.NewToolResultText(warnMsg), nil
		}

		err = m.admin.SaveAndLoad(username, str.ToString(id), b)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText("save ok"), nil
	})
}

func (m *Module) addPreviewRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("preview_rule_chain",
		mcp.WithDescription("Preview a rule chain: validate and return the chain JSON without saving. Used in web editor for real-time canvas preview. Call save_rule_chain when user confirms to save."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
		mcp.WithObject("body", mcp.Required(), mcp.Description("Rule chain definition")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, ok := request.GetArguments()["id"]
		if !ok {
			return nil, errors.New("id is required")
		}
		body, ok := request.GetArguments()["body"]
		if !ok {
			return nil, errors.New("body is required")
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		// Check whether all fields in the node configuration are those present in the component definition
		if warnMsg := m.validateRuleChainNodes(username, b); warnMsg != "" {
			return mcp.NewToolResultText(warnMsg), nil
		}

		// It does not save; it directly returns the JSON rule chain that passed the checksum
		var result map[string]interface{}
		if err := json.Unmarshal(b, &result); err != nil {
			return nil, err
		}
		result["_preview"] = true
		result["_id"] = id
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(resultBytes)), nil
	})
}

func (m *Module) addDeleteRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("delete_rule_chain",
		mcp.WithDescription("Delete a rule chain"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, ok := request.GetArguments()["id"]
		if !ok {
			return nil, errors.New("id is required")
		}
		err := m.admin.Delete(username, str.ToString(id))
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText("delete ok"), nil
	})
}

func (m *Module) addOperateRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("operate_rule_chain",
		mcp.WithDescription("Operate a rule chain: deploy(start running), undeploy(stop running)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
		mcp.WithString("action", mcp.Required(), mcp.Description("Operation: deploy or undeploy")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := str.ToString(request.GetArguments()["id"])
		if id == "" {
			return nil, errors.New("id is required")
		}
		action := strings.ToLower(str.ToString(request.GetArguments()["action"]))
		var err error
		var msg string
		switch action {
		case "deploy":
			err = m.admin.Deploy(username, id)
			msg = "deploy ok"
		case "undeploy":
			err = m.admin.Undeploy(username, id)
			msg = "undeploy ok"
		default:
			return nil, fmt.Errorf("unsupported action: %s, supported: deploy, undeploy", action)
		}
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(msg), nil
	})
}

func (m *Module) addExecuteRuleChainTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("execute_rule_chain",
		mcp.WithDescription("Execute a rule chain with input message"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Rule chain id")),
		mcp.WithObject("message", mcp.Required(), mcp.Description("Input message")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := str.ToString(request.GetArguments()["id"])
		if id == "" {
			return nil, errors.New("id is required")
		}
		ruleMsg := types.NewMsgWithJsonData(str.ToString(request.GetArguments()["message"]))
		wg := sync.WaitGroup{}
		wg.Add(1)
		var result string
		var resultErr error
		err := m.executor.ExecuteAndWait(username, id, ruleMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			result = msg.GetData()
			resultErr = err
			wg.Done()
		}))
		if err != nil {
			return nil, err
		}
		wg.Wait()
		return mcp.NewToolResultText(result), resultErr
	})
}

func (m *Module) addListComponentsTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("list_components",
		mcp.WithDescription("List available RuleGo components with name, description and relation types. Includes both node components and endpoint components. Use get_component_doc to get field details."),
		mcp.WithString("category", mcp.Description("Filter by category (filter, transform, external, action, common, flow, endpoint)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ue, err := m.engineMgr.GetOrCreate(username)
		if err != nil {
			return nil, err
		}

		category := str.ToString(request.GetArguments()["category"])
		var sb strings.Builder

		// Get node components
		components := ue.RuleConfig().ComponentsRegistry.GetComponentForms()
		hasNodeComponents := false
		nodeTable := strings.Builder{}
		nodeTable.WriteString("| type | category | desc | relationTypes |\n|------|----------|------|---------------|\n")
		for name, comp := range components {
			if category != "" && !strings.Contains(strings.ToLower(comp.Category), strings.ToLower(category)) && !strings.Contains(strings.ToLower(name), strings.ToLower(category)) {
				continue
			}
			relTypes := "Success"
			if comp.RelationTypes != nil && len(*comp.RelationTypes) > 0 {
				relTypes = strings.Join(*comp.RelationTypes, "/")
			}
			nodeTable.WriteString("| " + name + " | " + comp.Category + " | " + comp.Desc + " | " + relTypes + " |\n")
			hasNodeComponents = true
		}

		// Get endpoint components
		endpointForms := endpoint.Registry.GetComponentForms()
		hasEndpointComponents := false
		endpointTable := strings.Builder{}
		endpointTable.WriteString("| type | desc |\n|------|------|\n")
		for name, comp := range endpointForms {
			if category != "" && !strings.Contains(strings.ToLower(comp.Category), strings.ToLower(category)) && !strings.Contains(strings.ToLower(name), strings.ToLower(category)) {
				continue
			}
			endpointTable.WriteString("| " + name + " | " + comp.Desc + " |\n")
			hasEndpointComponents = true
		}

		// Build output with sections
		if hasNodeComponents {
			sb.WriteString("## Node Components\n\n")
			sb.WriteString(nodeTable.String())
		}

		if hasEndpointComponents {
			if hasNodeComponents {
				sb.WriteString("\n")
			}
			sb.WriteString("## Endpoint Components\n\n")
			sb.WriteString(endpointTable.String())
		}

		if !hasNodeComponents && !hasEndpointComponents {
			sb.WriteString("No components found for the specified category.")
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

func (m *Module) addGetComponentDocTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("get_component_doc",
		mcp.WithDescription("Get component documentation: fields, types, defaults, descriptions, and relation types. Supports both node components and endpoint components. Returns built-in processors info when querying endpoint components."),
		mcp.WithString("type", mcp.Required(), mcp.Description("Component type from list_components, e.g. jsFilter, restApiCall, endpoint/rest, endpoint/mqtt")),
		mcp.WithArray("types", mcp.Description("Multiple component types for batch query")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ue, err := m.engineMgr.GetOrCreate(username)
		if err != nil {
			return nil, err
		}

		// Merge node components and endpoint components
		allComponents := make(types.ComponentFormList)
		for k, v := range ue.RuleConfig().ComponentsRegistry.GetComponentForms() {
			allComponents[k] = v
		}
		for k, v := range endpoint.Registry.GetComponentForms() {
			allComponents[k] = v
		}

		// Get builtins for field options
		builtins := registry.Builtins()

		var names []string
		if single := str.ToString(request.GetArguments()["type"]); single != "" {
			names = append(names, single)
		}
		if arr, ok := request.GetArguments()["types"]; ok {
			if arrSlice, ok := arr.([]interface{}); ok {
				for _, item := range arrSlice {
					if s := str.ToString(item); s != "" {
						names = append(names, s)
					}
				}
			}
		}
		if len(names) == 0 {
			return nil, errors.New("type or types is required")
		}

		hasEndpoint := false
		var sb strings.Builder
		for _, name := range names {
			comp, ok := allComponents[name]
			if !ok {
				sb.WriteString("## " + name + "\n> Not found\n\n")
				continue
			}
			if strings.HasPrefix(name, "endpoint/") {
				hasEndpoint = true
			}
			sb.WriteString("## " + name + "\n")
			sb.WriteString(comp.Desc + "\n\n")
			sb.WriteString("**Category**: " + comp.Category + "\n\n")
			if comp.RelationTypes != nil && len(*comp.RelationTypes) > 0 {
				sb.WriteString("**Relations**: " + strings.Join(*comp.RelationTypes, "/") + "\n\n")
			}
			if len(comp.Fields) > 0 {
				sb.WriteString("| field | type | required | default | desc |\n")
				sb.WriteString("|-------|------|----------|---------|------|\n")
				for _, f := range comp.Fields {
					req := ""
					if f.Required {
						req = "yes"
					}
					defVal := ""
					if f.DefaultValue != nil {
						if b, err := json.Marshal(f.DefaultValue); err == nil {
							defVal = string(b)
						}
					}
					sb.WriteString("| " + f.Name + " | " + f.Type + " | " + req + " | " + defVal + " | " + f.Desc + " |\n")
				}
				sb.WriteString("\n")
			}
			// Include field options from builtins
			if b, ok := builtins[name]; ok {
				if options, err := json.MarshalIndent(b, "", "  "); err == nil {
					sb.WriteString("**Options**:\n```json\n" + string(options) + "\n```\n\n")
				}
			}
			if comp.RouterForm != nil {
				sb.WriteString("**Router**:\n")
				if comp.RouterForm.Hide {
					sb.WriteString("- Uses default router (auto-generate `from.path: \"*\"`)\n")
				} else if comp.RouterForm.From != nil {
					p := comp.RouterForm.From.Path
					req := ""
					if p.Required {
						req = " (required)"
					}
					sb.WriteString("- from.path: " + p.Desc + req + "\n")
				}
				if comp.RouterForm.Params != nil {
					p := comp.RouterForm.Params
					req := ""
					if p.Required {
						req = " (required)"
					}
					sb.WriteString("- params: " + p.Desc + req + "\n")
				}
				sb.WriteString("\n")
			}
		}

		// Append built-in processors info when querying endpoint components
		if hasEndpoint {
			sb.WriteString("## Built-in Processors\n\n")
			sb.WriteString("Endpoint 路由支持 `from.processors`（输入处理器）和 `to.processors`（输出处理器）。\n\n")
			inNames := processor.InBuiltins.Names()
			sort.Strings(inNames)
			if len(inNames) > 0 {
				sb.WriteString("| from.processors | 说明 |\n|-----------------|------|\n")
				for _, n := range inNames {
					sb.WriteString("| " + n + " | " + processorDesc(n) + " |\n")
				}
				sb.WriteString("\n")
			}
			outNames := processor.OutBuiltins.Names()
			sort.Strings(outNames)
			if len(outNames) > 0 {
				sb.WriteString("| to.processors | 说明 |\n|---------------|------|\n")
				for _, n := range outNames {
					sb.WriteString("| " + n + " | " + processorDesc(n) + " |\n")
				}
				sb.WriteString("\n")
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}

// processorDesc returns description for built-in processors
func processorDesc(name string) string {
	switch name {
	case "headersToMetadata":
		return "Extract HTTP request headers to message metadata"
	case "setJsonDataType":
		return "Set message data type to JSON"
	case "setTextDataType":
		return "Set message data type to TEXT"
	case "setBinaryDataType":
		return "Set message data type to binary"
	case "toHex":
		return "Convert binary data to hexadecimal string"
	case "responseToBody":
		return "Format message data as HTTP response body"
	case "metadataToHeaders":
		return "Map message metadata to HTTP response headers"
	default:
		return name
	}
}

func (m *Module) addListNodePoolTool(mcpServer *mcpserver.MCPServer, username string) {
	m.registerMCPTool(username, mcpServer, mcp.NewTool("list_node_pool",
		mcp.WithDescription("List available shared node pool resources. Returns ref://id, component type, and which config field to put ref:// into"),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		uname := getUsernameFromCtx(ctx)
		if uname == "" {
			uname = username
		}
		list, _, err := m.nodeSvc.ListNodePool(uname, 1, 100, "", "")
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return mcp.NewToolResultText("共享节点池为空"), nil
		}
		var sb strings.Builder
		sb.WriteString("可用共享节点（连接地址字段使用 ref://id 引用）：\n\n")
		for _, item := range list {
			b, _ := json.Marshal(item)
			var node struct {
				Id            string                 `json:"id"`
				Type          string                 `json:"type"`
				Configuration map[string]interface{} `json:"configuration"`
			}
			_ = json.Unmarshal(b, &node)
			refField := detectRefField(node.Configuration)
			if refField != "" {
				sb.WriteString(fmt.Sprintf("- ref://%s | %s | 放到 %s 字段\n", node.Id, node.Type, refField))
			} else {
				sb.WriteString(fmt.Sprintf("- ref://%s | %s\n", node.Id, node.Type))
			}
		}
		return mcp.NewToolResultText(sb.String()), nil
	})
}

// detectRefField In the heuristic detection configuration, which field is the connection address
func detectRefField(config map[string]interface{}) string {
	for key, val := range config {
		s := fmt.Sprintf("%v", val)
		if strings.Contains(s, "://") || strings.Contains(s, "@tcp") {
			return key
		}
		if parts := strings.Split(s, ":"); len(parts) == 2 {
			if _, err := strconv.Atoi(parts[1]); err == nil {
				return key
			}
		}
	}
	return ""
}

// ============================================
// MCPToolProvider interface implementation
// ============================================

// toolDefEntry Definition for local caching
type toolDefEntry struct {
	def     types.MCPToolDefinition
	handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// userMCPProvider wraps MCPToolProvider for specific users and injects username into context when CallTool.
// Solves the issue of toolDefs global coverage when multiple users share the same module.
type userMCPProvider struct {
	module   *Module
	username string
}

func (p *userMCPProvider) ListToolDefinitions() ([]types.MCPToolDefinition, error) {
	return p.module.ListToolDefinitions()
}

func (p *userMCPProvider) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	// Prioritize using the requested user in context (multi-user chat scenario); otherwise, use bound users
	username := p.username
	if requestingUser := services.MCPRequestingUserFromContext(ctx); requestingUser != "" {
		username = requestingUser
	}
	ctx = context.WithValue(ctx, usernameKey, username)
	return p.module.CallTool(ctx, toolName, args)
}

// ListToolDefinitions implementation types.MCPToolProvider interface.
// Collect definitions for all registered tools from the local cache.
func (m *Module) ListToolDefinitions() ([]types.MCPToolDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []types.MCPToolDefinition
	for _, t := range m.toolDefs {
		result = append(result, t.def)
	}
	return result, nil
}

// CallTool implements types.MCPToolProvider interface.
// Prioritize using the username in context to find the user's handler to avoid multi-user closure coverage.
func (m *Module) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	// First, get the username from the context and look for the per-user handler
	username := getUsernameFromCtx(ctx)

	m.mu.RLock()
	var entry toolDefEntry
	var ok bool
	if username != "" {
		if userDefs, exists := m.userToolDefs[username]; exists {
			entry, ok = userDefs[toolName]
		}
	}
	// Revert to global toolDefs (compatible with usernameless call scenarios)
	if !ok {
		entry, ok = m.toolDefs[toolName]
	}
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP tool not found: %s", toolName)
	}

	// Structure mcp.CallToolRequest
	request := mcp.CallToolRequest{}
	request.Params.Name = toolName
	request.Params.Arguments = args

	result, err := entry.handler(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to call MCP tool %s: %w", toolName, err)
	}

	if result.IsError {
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				return "", fmt.Errorf("MCP tool error: %s", textContent.Text)
			}
		}
		return "", fmt.Errorf("MCP tool error: unknown error")
	}

	var contents []string
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			contents = append(contents, textContent.Text)
		}
	}
	return strings.Join(contents, "\n"), nil
}

// RegisterTool registers a custom MCP tool to the specified user's MCP Server.
// The tool is also registered in the local toolDefs cache, and MCPToolProvider can also discover the tool.
func (m *Module) RegisterTool(username, name, description string, inputSchema []byte,
	handler func(ctx context.Context, args map[string]interface{}) (string, error)) error {
	if !m.cfg.MCP.Enable {
		return errors.New("MCP is disabled")
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return err
	}

	// Build mcp.Tool
	var tool mcp.Tool
	if len(inputSchema) > 0 {
		tool = mcp.NewToolWithRawSchema(name, description, inputSchema)
	} else {
		tool = mcp.NewTool(name, mcp.WithDescription(description))
	}

	// Packaged handler to fit mcp.CallToolRequest signs
	mcpHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := handler(ctx, request.GetArguments())
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(result), nil
	}

	m.registerMCPTool(username, state.mcpServer, tool, mcpHandler)
	return nil
}

// registerMCPTool Register the tool to MCP Server and local cache.
// username is used for per-user handler storage, preventing multiple users with the same name from overlapping each other.
func (m *Module) registerMCPTool(username string, mcpServer *mcpserver.MCPServer, tool mcp.Tool,
	handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	mcpServer.AddTool(tool, handler)

	schemaBytes, _ := json.Marshal(tool.InputSchema)
	entry := toolDefEntry{
		def: types.MCPToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaBytes,
		},
		handler: handler,
	}

	m.mu.Lock()
	// Tool definitions (name + schema) are stored globally, with all users having the same data
	m.toolDefs[tool.Name] = entry
	// Handler is stored by username to avoid multi-user closure overriding
	if m.userToolDefs[username] == nil {
		m.userToolDefs[username] = make(map[string]toolDefEntry)
	}
	m.userToolDefs[username][tool.Name] = entry
	m.mu.Unlock()
}

// context key for username
type contextKey string

const usernameKey contextKey = "mcp_username"
const groupKey contextKey = "mcp_group"

// getUsernameFromCtx retrieves the username from the context
func getUsernameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// getGroupFromCtx retrieves the group from the context
func getGroupFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(groupKey).(string); ok {
		return v
	}
	return ""
}
