//go:build with_ai || with_all

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/rulego/rulego-components-ai/aspect"
	aitool "github.com/rulego/rulego-components-ai/tool"

	// Import all AI components (nodes, tools, endpoints, processors) with one click.
	// Importing on demand can directly import corresponding subpackages, such as: _ "github.com/rulego/rulego-components-ai/agent"
	_ "github.com/rulego/rulego-components-ai/all"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/registry"
)

func init() {
	// All AI components have been automatically registered via ai/all's init().

	// Register AI tools to the global UDF
	registerAiGlobalUdfs(registry.RegisterGlobalUdf)

	// List of AI Tools Registration (including form)
	registry.RegisterBuiltin("ai/tools", getAiToolForms)

	// Set up AI tool providers for node.go use
	registry.AiToolsProvider = getAiToolInfos
}

// registerAiGlobalUdfs registers AI tools into a global UDF
func registerAiGlobalUdfs(registerFunc func(name string, value interface{})) {
	aitool.Registry.Range(func(name string, t einoTool.BaseTool) bool {
		registerFunc(name, types.Script{
			Type:    types.AiTool,
			Content: t,
		})
		return true
	})
}

// getAiToolForms to get form definitions for all tools
func getAiToolForms() interface{} {
	forms := aitool.Registry.GetToolForms()
	enhanced := make([]interface{}, 0, len(forms))
	for _, form := range forms {
		data := toolFormToMap(form)
		if form.Type == "skill" {
			registry.ApplySkillToolDefaults(data, form.Fields, "./skills")
		}
		enhanced = append(enhanced, data)
	}
	return map[string]interface{}{"tools": enhanced}
}

// getAiToolInfos Get information on all the tools
func getAiToolInfos(c types.Config) []interface{} {
	aiTools := c.GetUdfs(types.AiTool)
	var infos []interface{}
	ctx := context.Background()
	for _, v := range aiTools {
		if t, ok := v.(einoTool.BaseTool); ok {
			info, err := t.Info(ctx)
			if err == nil {
				infos = append(infos, info)
			}
		}
	}
	return infos
}

func toolFormToMap(form aitool.ToolForm) map[string]interface{} {
	raw, err := json.Marshal(form)
	if err != nil {
		return formBasicMap(form)
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return formBasicMap(form)
	}
	return data
}

func formBasicMap(form aitool.ToolForm) map[string]interface{} {
	return map[string]interface{}{
		"type":  form.Type,
		"label": form.Label,
		"desc":  form.Desc,
	}
}

// ============================================================
// AI tool security interception section (application layer logic, not a general library)
// ============================================================

// toolSecurityConfig AI tool security policy configuration
type toolSecurityConfig struct {
	Enabled      bool     // Main switch
	Mode         string   // "deny" or "allow"
	Tools        []string // Tool Name List (supports * wildcards)
	DeniedTypes  []string // Interception tool types: builtin, mcp, rulechain, subagent
	CmdDenyExtra []string // Bash tool additional command blacklist
	AllowPaths   []string // File path whitelist, empty without restrictions
	DenyPaths    []string // File path blacklist has higher priority than the whitelist
}

// toolSecurityAspect tool security interception face
// Implements ToolCallBeforeAspect to perform security checks before the tool is called
type toolSecurityAspect struct {
	order  int
	config toolSecurityConfig
}

func (a *toolSecurityAspect) Order() int { return a.order }
func (a *toolSecurityAspect) PointCut(_ context.Context, _ *aspect.AgentPoint) bool {
	return a.config.Enabled
}
func (a *toolSecurityAspect) New() aspect.Aspect {
	return &toolSecurityAspect{order: a.order, config: a.config}
}

func (a *toolSecurityAspect) BeforeToolCall(_ context.Context, _ *aspect.AgentPoint, call *aspect.ToolCallInfo) (*aspect.ToolCallInfo, error) {
	// 1. Check the type of tool
	if len(a.config.DeniedTypes) > 0 && isDeniedType(call.ToolType, a.config.DeniedTypes) {
		return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s type=%s", call.Name, call.ToolType)
	}

	// 2. Check the name of the tool
	if len(a.config.Tools) > 0 {
		matched := matchToolName(call.Name, a.config.Tools)
		mode := a.config.Mode
		if mode == "" {
			mode = "deny"
		}
		switch mode {
		case "deny":
			if matched {
				return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s", call.Name)
			}
		case "allow":
			if !matched {
				return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s（不在允许列表中）", call.Name)
			}
		}
	}

	// 3. Secondary validation of command parameters for bash tools (shallow inspection, deep defense handled by the bash tool itself)
	if len(a.config.CmdDenyExtra) > 0 && isBashTool(call.Name) {
		if err := checkBashCommand(call.Arguments, a.config.CmdDenyExtra); err != nil {
			return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s, %w", call.Name, err)
		}
	}

	// 4. File path check (only effective for read/write/edit)
	if isFileTool(call.Name) && (len(a.config.DenyPaths) > 0 || len(a.config.AllowPaths) > 0) {
		if err := checkToolFilePath(call.Arguments, a.config.DenyPaths, a.config.AllowPaths); err != nil {
			return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s, %w", call.Name, err)
		}
	}

	return call, nil
}

// --- Tool Type Check ---

func isDeniedType(toolType aspect.ToolType, deniedTypes []string) bool {
	s := string(toolType)
	for _, dt := range deniedTypes {
		if s == dt {
			return true
		}
	}
	return false
}

func matchToolName(name string, patterns []string) bool {
	for _, p := range patterns {
		if p == name {
			return true
		}
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(name, strings.TrimSuffix(p, "*")) {
				return true
			}
		}
	}
	return false
}

// --- bash command check---

func isBashTool(name string) bool {
	return name == "bash" || strings.HasPrefix(name, "bash_")
}

func checkBashCommand(argsJSON string, denyList []string) error {
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil
	}
	cmd := strings.TrimSpace(args.Command)
	if cmd == "" {
		return nil
	}
	cmdName := extractCommandName(cmd)
	for _, denied := range denyList {
		if cmdName == denied {
			return fmt.Errorf("命令被安全策略拦截: %s", cmdName)
		}
	}
	return nil
}

func extractCommandName(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if strings.Contains(cmd, "/") || strings.Contains(cmd, "\\") {
		if idx := strings.IndexByte(cmd, ' '); idx > 0 {
			cmd = cmd[:idx]
		}
		return strings.ToLower(filepath.Base(cmd))
	}
	if idx := strings.IndexByte(cmd, ' '); idx > 0 {
		return strings.ToLower(cmd[:idx])
	}
	return strings.ToLower(cmd)
}

// --- File Path Checking ---

func isFileTool(name string) bool {
	return name == "read" || name == "write" || name == "edit"
}

// cleanPaths preprocessed path list (called once at registration to avoid repeated counting each time the tool is called)
// Convert relative paths to absolute paths and clean them, uniformly converting lowercase letters for cross-platform case-insensitive matching
func cleanPaths(paths []string) []string {
	if paths == nil {
		return nil
	}
	result := make([]string, len(paths))
	for i, p := range paths {
		p = filepath.Clean(p)
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
		}
		result[i] = strings.ToLower(p)
	}
	return result
}

func checkToolFilePath(argsJSON string, denyPaths, allowPaths []string) error {
	filePath := extractPathFromArgs(argsJSON)
	if filePath == "" {
		return nil
	}

	if !filepath.IsAbs(filePath) {
		if abs, err := filepath.Abs(filePath); err == nil {
			filePath = abs
		}
	}
	filePath = strings.ToLower(filepath.Clean(filePath))

	// a. First, check the blacklist (priority is higher than the whitelist)
	for _, dp := range denyPaths {
		if strings.HasPrefix(filePath, dp+string(filepath.Separator)) || filePath == dp {
			return fmt.Errorf("路径被安全策略禁止: %s", filePath)
		}
	}

	// b. Recheck the whitelist (only effective if the whitelist is not empty)
	if len(allowPaths) > 0 {
		allowed := false
		for _, ap := range allowPaths {
			if strings.HasPrefix(filePath, ap+string(filepath.Separator)) || filePath == ap {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("路径不在允许列表中: %s", filePath)
		}
	}

	return nil
}

func extractPathFromArgs(argsJSON string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}
	return strings.TrimSpace(args.Path)
}

// ============================================================
// Secure Facet hook registration
// ============================================================

// registerAiSecurityHook Registers the lifecycle hook of the AI tool security interception face
// Reads configuration and registers aspects during the App.BeforeInit stage.
func registerAiSecurityHook(application *app.App) {
	application.AddHook(app.NewFuncHook("ai-security", app.BeforeInit, 0,
		func(_ context.Context, appCtx *app.ModuleContext) error {
			sec := appCtx.Config.AISecurity
			if !sec.Enable {
				return nil
			}

			cfg := toolSecurityConfig{
				Enabled:      true,
				Mode:         sec.Mode,
				CmdDenyExtra: parseCsv(sec.CmdDenyExtra),
				DeniedTypes:  parseCsv(sec.DeniedTypes),
				AllowPaths:   cleanPaths(parseCsv(sec.AllowPaths)),
				DenyPaths:    cleanPaths(parseCsv(sec.DenyPaths)),
			}

			switch sec.Mode {
			case "deny":
				cfg.Tools = parseCsv(sec.DenyTools)
			case "allow":
				cfg.Tools = parseCsv(sec.AllowTools)
			}

			if cfg.Mode == "" {
				cfg.Mode = "deny"
			}

			aspect.RegisterAspect("tool_security", &toolSecurityAspect{order: 10, config: cfg})
			return nil
		}))
}

// parseCsv parses comma-separated strings and returns slices with spaces removed
func parseCsv(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
