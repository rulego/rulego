// Package mcp 实现 MCP（Model Context Protocol）端点和工具管理。
//
// # 概述
//
// 本模块为 RuleGo 提供 MCP Server 能力，允许 AI 智能体通过标准化协议操控规则引擎。
// 支持两种接入方式：
//   - Streamable HTTP 远程接入：外部 AI 客户端（Claude Desktop、Cursor 等）通过 HTTP 连接
//   - 进程内接入：RuleGo 内部 agent 节点通过 MCPToolProvider 接口直接调用
//
// # 配置
//
// MCP 功能通过 MCPConfig 配置（config.ini 或 JSON 配置）：
//
//	[mcp]
//	enable = true
//	默认端点固定加载管理 API 工具，组件和规则链工具通过分组配置加载。
//
// # HTTP 端点
//
// 默认组（包含该用户的全部工具）：
//
//	GET/POST/DELETE /api/v1/mcp/{apiKey}           # MCP StreamableHTTP 端点
//
// 分组（通过 MCPConfig.Groups 配置，控制工具子集）：
//
//	GET/POST/DELETE /api/v1/mcp/{apiKey}/group/{groupName}
//
// # 分组配置
//
// Groups 通过程序化配置（map[string]string），key 为组名，value 为工具列表。
// 分组内没有内置默认分组，需要自行配置。
//
// 语法：
//   - 逗号分隔工具名
//   - * 表示全部工具
//   - -prefix* 表示排除前缀匹配的工具
//   - rules = 管理 API 工具，components = 组件工具，chains = 规则链工具
//
// 示例：
//
//	Groups: map[string]string{
//	  "readonly":  "rules,list_components,get_component_doc",
//	  "full":      "*",
//	  "no-delete": "*,-delete_rule_chain",
//	}
//
// # MCP 工具清单
//
// 管理 API 工具（默认端点固定加载）：
//
//	工具名              | 作用
//	list_rule_chains    | 列出/搜索规则链（支持分页、关键词过滤）
//	get_rule_chain      | 获取规则链定义 JSON（供查看或修改）
//	preview_rule_chain  | 预览规则链（校验+返回JSON，不保存）
//	save_rule_chain     | 创建或更新规则链（含节点字段校验）
//	delete_rule_chain   | 删除规则链
//	operate_rule_chain  | 操作规则链（deploy/undeploy）
//	execute_rule_chain  | 执行规则链并返回结果
//	list_components     | 列出组件（含分类、字段、连接类型）
//	get_component_doc   | 获取组件完整文档（支持批量查询）
//
// 组件工具（通过分组配置加载）：
//
//	每个注册组件自动成为独立工具，工具名为组件类型名（如 jsFilter、restApiCall）。
//	参数从组件的 ComponentForm.Fields 自动生成，包含字段名、类型、描述、默认值、必填标记。
//
// 规则链工具（通过分组配置加载）：
//
//	每个已部署规则链自动成为独立工具，工具名为规则链 ID。
//	参数来自规则链的 inputSchema 或 DSL 模板变量解析。
//	规则链变更时通过 Callbacks 动态同步（OnNew/OnUpdated/OnDeleted）。
//
// # 外部客户端配置
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
// 进程内 agent（规则链 JSON 配置）：
//
//	{
//	  "type": "mcp",
//	  "config": {
//	    "server": "self",
//	    "tools": ["list_rule_chains", "get_rule_chain", "save_rule_chain"]
//	  }
//	}
//
// tools 数组为过滤器：只列出的工具才会加载到大模型上下文。使用 "*" 加载全部。
//
// # MCPToolProvider 接口
//
// Module 实现了 types.MCPToolProvider 接口，供内部 agent 节点使用：
//   - ListToolDefinitions() — 返回所有已注册工具的定义
//   - CallTool(ctx, toolName, args) — 按名称调用工具
//   - RegisterTool(username, name, desc, schema, handler) — 注册自定义工具
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

// userMcpState 每个用户的 MCP 状态
type userMcpState struct {
	mcpServer  *mcpserver.MCPServer
	httpServer *mcpserver.StreamableHTTPServer
}

// Module mcp 业务模块，负责 MCP SSE/HTTP 端点和工具管理
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
	groupUsers   map[string]*userMcpState              // 分组用户的 MCP 状态
	groups       map[string]*MCPGroup                  // 分组定义
	toolDefs     map[string]toolDefEntry               // 工具定义缓存（名称+schema，所有用户相同）
	userToolDefs map[string]map[string]toolDefEntry    // 每个用户的工具 handler（username -> toolName -> entry）
}

// New 创建 mcp 模块
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
	// 服务解析延迟到 Start 阶段，因为 rule 模块的服务在 rule.Init() 中注册。
	// 设置优先级 25 < rule(30)，保证 mcp.Start() 在 rule.Start() 之前执行。
	return ctx.Container.Register(services.KeyMcpService, services.McpService(m))
}

func (m *Module) Start(ctx context.Context) error {
	if m.cfg == nil || !m.cfg.MCP.Enable {
		return nil
	}
	// 解析服务（rule.Init() 已注册）
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

	// 初始化分组
	m.initGroups()

	// 注册 MCPToolProvider 并加载工具
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

// generateGlobalVarsFile 生成全局变量名列表文件，供智能体提示词 include 引用。
// 只包含变量名，不包含值，防止敏感信息泄露。
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

// loadUserTools 为指定用户加载工具并设置回调
func (m *Module) loadUserTools(username string) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	// 注册 MCP ToolProvider 到 RuleConfig UDF，供内部 agent 使用。
	// 使用 userMCPProvider 包装，确保 "self" 模式下 CallTool 注入正确的 username。
	// Udf 是 map 类型（引用类型），直接写入对原始 Config 生效。
	if m.cfg.MCP.Enable {
		cfg := ue.RuleConfig()
		if cfg.Udf == nil {
			cfg.Udf = make(map[string]interface{})
		}
		cfg.Udf[types.MCPToolProviderKey] = &userMCPProvider{module: m, username: username}
	}
	// 设置规则链变更回调（始终设置，确保分组端点能动态同步）
	ue.Pool().SetCallbacks(m.Callbacks(username))
	// 加载工具
	m.LoadTools(username)
}
func (m *Module) Stop(_ context.Context) error { return nil }

// getOrCreateState 获取或创建用户的 MCP 状态
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

// HandleMCP 处理 MCP StreamableHTTP 请求（GET/POST/DELETE）
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

// AddToolsFromComponent 从组件添加工具
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

// DeleteTools 删除工具
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
	// 同步删除分组 MCP Server 中的工具
	m.syncDeleteToGroups(username, names...)
}

// LoadTools 加载用户的工具
func (m *Module) LoadTools(username string) {
	if !m.cfg.MCP.Enable {
		return
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return
	}

	// 默认端点固定加载管理 API 工具
	m.addRuleApiTools(state, username)
}

// AddToolsFromChain 从规则链定义添加工具
func (m *Module) AddToolsFromChain(username, chainId string, def types.RuleChain) {
	if !m.cfg.MCP.Enable {
		return
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return
	}
	m.addToolsFromChain(state.mcpServer, chainId, def)
	// 同步添加到分组 MCP Server
	m.syncChainToGroups(username, chainId, def)
}

// Callbacks 返回规则链变更回调
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

// syncChainToGroups 将规则链工具变更同步到该用户的所有分组 MCP Server
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

// syncDeleteToGroups 将工具删除同步到该用户的所有分组 MCP Server
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

// addToolsFromComponent 从组件定义添加 MCP 工具
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

// componentToolHandler 创建组件工具的处理函数
func (m *Module) componentToolHandler(componentType string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 从 context 获取 username（由 SSE handler 注入）
		username := getUsernameFromCtx(ctx)
		if username == "" {
			return nil, errors.New("username not found in context")
		}
		ue, err := m.engineMgr.GetOrCreate(username)
		if err != nil {
			return nil, err
		}
		ruleConfig := ue.RuleConfig()

		// 校验智能体传入的参数是否都是组件定义中存在的字段
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

// validateComponentArgs 校验智能体传入的参数是否都是组件定义中存在的字段
// 同时校验必填字段是否缺失
// 如果存在不存在的字段或缺少必填字段，返回提示信息；否则返回空字符串
func (m *Module) validateComponentArgs(ruleConfig types.Config, componentType string, args map[string]interface{}) string {
	components := ruleConfig.ComponentsRegistry.GetComponentForms()
	componentForm, ok := components[componentType]
	if !ok {
		return ""
	}
	// 收集组件定义中的所有字段
	validFields := make(map[string]bool)
	for _, field := range componentForm.Fields {
		validFields[field.Name] = true
	}
	var warnings []string
	// 检查多余字段
	var unknownFields []string
	for key := range args {
		if !validFields[key] {
			unknownFields = append(unknownFields, key)
		}
	}
	if len(unknownFields) > 0 {
		warnings = append(warnings, fmt.Sprintf("unknown fields: %v", unknownFields))
	}
	// 检查缺少的必填字段
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
		// 构建可用字段列表
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

// validateRuleChainNodes 校验规则链中所有节点的配置字段
// 同时校验必填字段是否缺失
// 如果存在不存在的字段或缺少必填字段，返回提示信息；否则返回空字符串
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
		// 检查多余字段
		var unknownFields []string
		for key := range node.Configuration {
			if !validFields[key] {
				unknownFields = append(unknownFields, key)
			}
		}
		if len(unknownFields) > 0 {
			issues = append(issues, fmt.Sprintf("unknown fields: %v", unknownFields))
		}
		// 检查缺少的必填字段
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
			// 构建可用字段列表
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

// loadToolsFromComponents 从组件列表添加工具
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

// loadToolsFromChains 从规则链列表添加工具
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

// addToolsFromChain 从规则链定义添加 MCP 工具
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

// ruleChainToolHandler 创建规则链工具的处理函数
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

// addRuleApiTools 添加规则链管理 API 工具
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

		// 校验节点配置中的字段是否都是组件定义中存在的字段
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

		// 校验节点配置中的字段是否都是组件定义中存在的字段
		if warnMsg := m.validateRuleChainNodes(username, b); warnMsg != "" {
			return mcp.NewToolResultText(warnMsg), nil
		}

		// 不保存，直接返回校验通过的规则链 JSON
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

// detectRefField 启发式检测配置中哪个字段是连接地址
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
// MCPToolProvider 接口实现
// ============================================

// toolDefEntry 本地缓存的工具定义
type toolDefEntry struct {
	def     types.MCPToolDefinition
	handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// userMCPProvider 为特定用户包装 MCPToolProvider，在 CallTool 时注入 username 到 context。
// 解决多个用户共享同一个 Module 时 toolDefs 全局覆盖的问题。
type userMCPProvider struct {
	module   *Module
	username string
}

func (p *userMCPProvider) ListToolDefinitions() ([]types.MCPToolDefinition, error) {
	return p.module.ListToolDefinitions()
}

func (p *userMCPProvider) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	// 优先使用 context 中的请求用户（多用户 chat 场景），否则使用绑定用户
	username := p.username
	if requestingUser := services.MCPRequestingUserFromContext(ctx); requestingUser != "" {
		username = requestingUser
	}
	ctx = context.WithValue(ctx, usernameKey, username)
	return p.module.CallTool(ctx, toolName, args)
}

// ListToolDefinitions 实现 types.MCPToolProvider 接口。
// 从本地缓存收集所有已注册工具的定义。
func (m *Module) ListToolDefinitions() ([]types.MCPToolDefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []types.MCPToolDefinition
	for _, t := range m.toolDefs {
		result = append(result, t.def)
	}
	return result, nil
}

// CallTool 实现 types.MCPToolProvider 接口。
// 优先使用 context 中的 username 查找该用户的 handler，避免多用户闭包覆盖问题。
func (m *Module) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	// 优先从 context 获取 username，查找 per-user handler
	username := getUsernameFromCtx(ctx)

	m.mu.RLock()
	var entry toolDefEntry
	var ok bool
	if username != "" {
		if userDefs, exists := m.userToolDefs[username]; exists {
			entry, ok = userDefs[toolName]
		}
	}
	// 回退到全局 toolDefs（兼容无 username 的调用场景）
	if !ok {
		entry, ok = m.toolDefs[toolName]
	}
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP tool not found: %s", toolName)
	}

	// 构造 mcp.CallToolRequest
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

// RegisterTool 注册自定义 MCP 工具到指定用户的 MCP Server。
// 工具同时注册到本地 toolDefs 缓存，MCPToolProvider 也能发现该工具。
func (m *Module) RegisterTool(username, name, description string, inputSchema []byte,
	handler func(ctx context.Context, args map[string]interface{}) (string, error)) error {
	if !m.cfg.MCP.Enable {
		return errors.New("MCP is disabled")
	}
	state, err := m.getOrCreateState(username)
	if err != nil {
		return err
	}

	// 构建 mcp.Tool
	var tool mcp.Tool
	if len(inputSchema) > 0 {
		tool = mcp.NewToolWithRawSchema(name, description, inputSchema)
	} else {
		tool = mcp.NewTool(name, mcp.WithDescription(description))
	}

	// 包装 handler 以适配 mcp.CallToolRequest 签名
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

// registerMCPTool 注册工具到 MCP Server 和本地缓存。
// username 用于 per-user handler 存储，防止多用户同名工具互相覆盖。
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
	// 工具定义（名称+schema）全局存储，所有用户相同
	m.toolDefs[tool.Name] = entry
	// handler 按 username 存储，避免多用户闭包覆盖
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

// getUsernameFromCtx 从 context 获取 username
func getUsernameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(usernameKey).(string); ok {
		return v
	}
	return ""
}

// getGroupFromCtx 从 context 获取 group
func getGroupFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(groupKey).(string); ok {
		return v
	}
	return ""
}
