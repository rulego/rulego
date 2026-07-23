/*
 * Copyright 2025 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"encoding/json"
	"errors"
	"examples/server/config"
	"examples/server/internal/constants"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/dsl"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/str"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// McpService custom component services
type McpService struct {
	username          string
	config            config.Config
	ruleConfig        types.Config
	Pool              types.RuleEnginePool
	componentService  *ComponentService
	ruleEngineService *RuleEngineService
	mcpServer         *server.MCPServer
	sseServer         *server.SSEServer
}

func NewMcpService(ruleConfig types.Config, c config.Config, pool types.RuleEnginePool, componentService *ComponentService, username string) (*McpService, error) {
	folderPath := path.Join(c.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)

	service := &McpService{
		username:         username,
		config:           c,
		ruleConfig:       ruleConfig,
		componentService: componentService,
		Pool:             pool,
	}
	service.mcpServer = service.NewMCPServer()
	service.sseServer = service.NewSSEServer(server.WithBasePath("/api/v1/mcp/" + UserServiceImpl.GetApiKeyByUsername(username)))
	return service, nil
}
func (s *McpService) GetRuleConfig() types.Config {
	return s.ruleConfig
}

func (s *McpService) NewMCPServer() *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"RuleGo MCP Server", // Server name
		"1.0.0",             // Server version
	)
	return mcpServer
}

func (s *McpService) Callbacks() types.Callbacks {
	return types.Callbacks{
		OnUpdated: func(chainId, nodeId string, dsl []byte) {
			var def types.RuleChain
			err := json.Unmarshal(dsl, &def)
			if err == nil {
				s.AddToolsFromChain(chainId, def)
			}
		},
		OnDeleted: func(id string) {
			s.DeleteTools(id)
		},
		OnNew: func(chainId string, dsl []byte) {
			var def types.RuleChain
			err := json.Unmarshal(dsl, &def)
			if err == nil {
				s.AddToolsFromChain(chainId, def)
			}
		},
	}
}
func (s *McpService) LoadTools() {
	if s.mcpServer != nil {
		// Add tools from the component list
		if s.config.MCP.LoadComponentsAsTool {
			s.LoadToolsFromComponents()
		}
		//if s.config.MCP.LoadChainsAsTool {// Already loaded from Callbacks.OnNew
		//	s.LoadToolsFromChains()
		//}
		if s.config.MCP.LoadApisAsTool {
			s.AddRuleApiTools()
		}
	}
}

func (s *McpService) NewSSEServer(opts ...server.SSEOption) *server.SSEServer {
	return server.NewSSEServer(s.mcpServer, opts...)
}

func (s *McpService) MCPServer() *server.MCPServer {
	return s.mcpServer
}

func (s *McpService) SSEServer() *server.SSEServer {
	return s.sseServer
}

func (s *McpService) DeleteTools(names ...string) {
	s.mcpServer.DeleteTools(names...)
}

// LoadToolsFromComponents adds tools from the component list
func (s *McpService) LoadToolsFromComponents() {
	components := s.componentService.ComponentsRegistry().GetComponentForms()
	for name, component := range components {
		if !s.CheckExclude(name, true) {
			s.AddToolsFromComponent(name, component)
		}
	}
}

// CheckExclude Checks whether components need to be excluded
func (s *McpService) CheckExclude(name string, isComponent bool) bool {
	if isComponent {
		for _, item := range strings.Split(s.config.MCP.ExcludeComponents, ",") {
			if match, _ := filepath.Match(strings.TrimSpace(item), name); match {
				return true
			}
		}
	} else {
		for _, item := range strings.Split(s.config.MCP.ExcludeChains, ",") {
			if match, _ := filepath.Match(strings.TrimSpace(item), name); match {
				return true
			}
		}
	}
	return false
}

// AddToolsFromComponent: Adds tools from component definitions
func (s *McpService) AddToolsFromComponent(name string, component types.ComponentForm) {
	var toolOptions []mcp.ToolOption
	for _, item := range component.Fields {
		var toolOption mcp.ToolOption
		desc := item.Name
		if item.Desc != "" {
			desc = item.Desc
		}
		var propertyOptions = []mcp.PropertyOption{
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
	toolOptions = append(toolOptions, mcp.WithDescription(desc)) // Tool description
	// Add tools
	tool := mcp.NewTool(name, // Tool name
		toolOptions...,
	)
	// Add processors to the tool
	s.mcpServer.AddTool(tool, s.componentToolHandler(name))
}
func (s *McpService) componentToolHandler(componentType string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		node, err := s.componentService.ComponentsRegistry().NewNode(componentType)
		if err != nil {
			return nil, err
		}
		err = node.Init(s.ruleConfig, request.Params.Arguments)
		if err != nil {
			return nil, err
		}
		message := str.ToString(request.Params.Arguments[constants.KeyInMessage])
		wg := sync.WaitGroup{}
		wg.Add(1)
		var result string
		var resultErr error
		ruleCtx := engine.NewRuleContext(ctx, s.ruleConfig, nil, nil, nil, s.ruleConfig.Pool, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			result = msg.GetData()
			resultErr = err
			wg.Done()
		}, s.Pool)
		node.OnMsg(ruleCtx, types.NewMsgWithJsonData(message))
		wg.Wait()
		node.Destroy()
		return mcp.NewToolResultText(result), resultErr
	}
}

// LoadToolsFromChains Adds tools from the list of rule chains
func (s *McpService) LoadToolsFromChains() {
	s.Pool.Range(func(key, value any) bool {
		if item, ok := value.(*engine.RuleEngine); ok {
			def := item.Definition()
			var id = str.ToString(key)
			if !s.CheckExclude(id, false) {
				s.AddToolsFromChain(id, def)
			}
		}
		return true
	})
}
func (s *McpService) AddToolsFromChain(id string, def types.RuleChain) {
	var desc = def.RuleChain.Name

	if v := str.ToString(def.RuleChain.AdditionalInfo["description"]); v != "" {
		desc = v
	}
	if desc != "" {
		var tool mcp.Tool
		if inputSchemaMap, ok := def.RuleChain.AdditionalInfo["inputSchema"]; ok {
			if schema, err := json.Marshal(inputSchemaMap); err == nil {
				tool = mcp.NewToolWithRawSchema(id, desc, schema)
			}
		} else {
			//Automatically retrieves all variables from all nodes
			vars := dsl.ParseVars(types.MsgKey, def)
			if len(vars) > 0 {
				var toolOptions []mcp.ToolOption
				for _, item := range vars {
					toolOptions = append(toolOptions, mcp.WithString(item, mcp.Required(), mcp.Description("input param: "+item)))
				}
				toolOptions = append(toolOptions, mcp.WithDescription(desc))
				tool = mcp.NewTool(id, toolOptions...)
			} else {
				tool = mcp.NewTool(id, mcp.WithDescription(desc), mcp.WithObject(constants.KeyInMessage, mcp.Description("input message")))
			}
		}
		// Add processors to the tool
		s.mcpServer.AddTool(tool, s.ruleChainToolHandler(id))
	}
}
func (s *McpService) ruleChainToolHandler(chainId string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ruleEngine, ok := s.Pool.Get(chainId)
		if !ok {
			return nil, errors.New("rule chain not found")
		} else {
			var msg string
			if params, ok := request.Params.Arguments[constants.KeyInMessage]; ok {
				msg = str.ToString(params)
			} else if v, err := json.Marshal(request.Params.Arguments); err != nil {
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
}

const (
	// ToolNameSaveRuleChain stores the rule chain
	ToolNameSaveRuleChain = "saveRuleChain"
	// ToolNameListRuleChain lists the rule chain
	ToolNameListRuleChain = "listRuleChain"
	// ToolNameDeleteRuleChain Deletes the rule chain
	ToolNameDeleteRuleChain = "deleteRuleChain"
	// ToolNameExecuteRuleChain executes the rule chain
	ToolNameExecuteRuleChain = "executeRuleChain"
)

// AddRuleApiTools Rule chain tool
func (s *McpService) AddRuleApiTools() {
	s.AddListRuleTool()
	s.AddSaveRuleTool()
	s.AddDeleteRuleTool()
	s.AddExecuteRuleTool()
}

// AddListRuleTool Adds a list rule chain tool
func (s *McpService) AddListRuleTool() {
	tool := mcp.NewTool(ToolNameListRuleChain,
		mcp.WithDescription("List RuleGo rule chain"),
		mcp.WithString(constants.KeyKeywords, mcp.Description("Keywords for filtering rule chains")),
		mcp.WithBoolean(constants.KeyRoot, mcp.Description("Indicates whether the rule chain is a root rule chain")),
		mcp.WithBoolean(constants.KeyDisabled, mcp.Description("Indicates whether the rule chain is disabled")),
		mcp.WithNumber(constants.KeyPage, mcp.Description("Page number for pagination")),
		mcp.WithNumber(constants.KeySize, mcp.Description("Number of items per page for pagination")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		keywords := strings.TrimSpace(str.ToString(request.Params.Arguments[constants.KeyKeywords]))
		rootStr := strings.TrimSpace(str.ToString(constants.KeyRoot))
		rootDisabled := strings.TrimSpace(str.ToString(constants.KeyDisabled))
		var page = 1
		var size = 20
		currentStr := str.ToString(request.Params.Arguments[constants.KeyPage])
		if i, err := strconv.Atoi(currentStr); err == nil {
			page = i
		}
		pageSizeStr := str.ToString(request.Params.Arguments[constants.KeySize])
		if i, err := strconv.Atoi(pageSizeStr); err == nil {
			size = i
		}
		var root *bool
		var disabled *bool
		if i, err := strconv.ParseBool(rootStr); err == nil {
			root = &i
		}
		if i, err := strconv.ParseBool(rootDisabled); err == nil {
			disabled = &i
		}
		list, count, err := s.ruleEngineService.List(keywords, root, disabled, size, page)
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

// AddSaveRuleTool Adds a chain of saving rule chain tools
func (s *McpService) AddSaveRuleTool() {
	tool := mcp.NewTool(ToolNameSaveRuleChain,
		mcp.WithDescription("Save or update RuleGo rule chain"),
		mcp.WithString(constants.KeyId, mcp.Required(), mcp.Description("rule chain id")),
		mcp.WithObject(constants.KeyBody, mcp.Required(), mcp.Description("rule chain")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, ok := request.Params.Arguments[constants.KeyId]
		if !ok {
			return nil, errors.New("id is required")
		}
		body, ok := request.Params.Arguments[constants.KeyBody]
		if !ok {
			return nil, errors.New("body is required")
		}
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		err = s.ruleEngineService.SaveAndLoad(str.ToString(id), b)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText("save ok"), nil
	})
}

// AddDeleteRuleTool Chain of Add/Remove Rule Chain
func (s *McpService) AddDeleteRuleTool() {
	tool := mcp.NewTool(ToolNameDeleteRuleChain,
		mcp.WithDescription("Delete RuleGo rule chain"),
		mcp.WithString(constants.KeyId, mcp.Required(), mcp.Description("rule chain id")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, ok := request.Params.Arguments[constants.KeyId]
		if !ok {
			return nil, errors.New("id is required")
		}
		err := s.ruleEngineService.Delete(str.ToString(id))
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText("delete ok"), nil
	})
}

func (s *McpService) AddExecuteRuleTool() {
	tool := mcp.NewTool(ToolNameExecuteRuleChain,
		mcp.WithDescription("Execute RuleGo rule chain by chain id"),
		mcp.WithString(constants.KeyId, mcp.Required(), mcp.Description("Rule chain id")),
		mcp.WithObject(constants.KeyInMessage, mcp.Required(), mcp.Description("Execute rule chain input message")),
	)
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := str.ToString(request.Params.Arguments[constants.KeyId])
		if id == "" {
			return nil, errors.New("id is required")
		}
		ruleMsg := types.NewMsgWithJsonData(str.ToString(request.Params.Arguments[constants.KeyInMessage]))
		wg := sync.WaitGroup{}
		wg.Add(1)
		var result string
		var resultErr error
		err := s.ruleEngineService.ExecuteAndWait(id, ruleMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
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
