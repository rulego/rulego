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

	// 一键引入所有 AI 组件（节点、工具、Endpoint、Processor）
	// 按需引入可直接导入对应子包，如: _ "github.com/rulego/rulego-components-ai/agent"
	_ "github.com/rulego/rulego-components-ai/all"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/registry"
)

func init() {
	// 所有 AI 组件已通过 ai/all 的 init() 自动注册

	// 把 AI 工具注册到全局 UDF
	registerAiGlobalUdfs(registry.RegisterGlobalUdf)

	// 注册 AI 工具列表（包含表单）
	registry.RegisterBuiltin("ai/tools", getAiToolForms)

	// 设置 AI 工具提供者，供 node.go 使用
	registry.AiToolsProvider = getAiToolInfos
}

// registerAiGlobalUdfs 把 AI 工具注册到全局 UDF
func registerAiGlobalUdfs(registerFunc func(name string, value interface{})) {
	aitool.Registry.Range(func(name string, t einoTool.BaseTool) bool {
		registerFunc(name, types.Script{
			Type:    types.AiTool,
			Content: t,
		})
		return true
	})
}

// getAiToolForms 获取所有工具的表单定义
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

// getAiToolInfos 获取所有工具的信息
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
// AI 工具安全拦截切面（应用层逻辑，不属于通用库）
// ============================================================

// toolSecurityConfig AI 工具安全策略配置
type toolSecurityConfig struct {
	Enabled      bool     // 总开关
	Mode         string   // "deny" 或 "allow"
	Tools        []string // 工具名称列表（支持 * 通配符）
	DeniedTypes  []string // 拦截的工具类型：builtin, mcp, rulechain, subagent
	CmdDenyExtra []string // bash 工具额外命令黑名单
	AllowPaths   []string // 文件路径白名单，为空不限制
	DenyPaths    []string // 文件路径黑名单，优先级高于白名单
}

// toolSecurityAspect 工具安全拦截切面
// 实现 ToolCallBeforeAspect，在工具调用前进行安全检查
type toolSecurityAspect struct {
	order  int
	config toolSecurityConfig
}

func (a *toolSecurityAspect) Order() int                                            { return a.order }
func (a *toolSecurityAspect) PointCut(_ context.Context, _ *aspect.AgentPoint) bool { return a.config.Enabled }
func (a *toolSecurityAspect) New() aspect.Aspect {
	return &toolSecurityAspect{order: a.order, config: a.config}
}

func (a *toolSecurityAspect) BeforeToolCall(_ context.Context, _ *aspect.AgentPoint, call *aspect.ToolCallInfo) (*aspect.ToolCallInfo, error) {
	// 1. 检查工具类型
	if len(a.config.DeniedTypes) > 0 && isDeniedType(call.ToolType, a.config.DeniedTypes) {
		return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s type=%s", call.Name, call.ToolType)
	}

	// 2. 检查工具名称
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

	// 3. bash 类工具的命令参数二次校验（浅层检查，深度防御由 bash 工具自身负责）
	if len(a.config.CmdDenyExtra) > 0 && isBashTool(call.Name) {
		if err := checkBashCommand(call.Arguments, a.config.CmdDenyExtra); err != nil {
			return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s, %w", call.Name, err)
		}
	}

	// 4. 文件路径检查（仅对 read/write/edit 生效）
	if isFileTool(call.Name) && (len(a.config.DenyPaths) > 0 || len(a.config.AllowPaths) > 0) {
		if err := checkToolFilePath(call.Arguments, a.config.DenyPaths, a.config.AllowPaths); err != nil {
			return nil, fmt.Errorf("工具调用被安全策略拦截: tool=%s, %w", call.Name, err)
		}
	}

	return call, nil
}

// --- 工具类型检查 ---

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

// --- bash 命令检查 ---

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

// --- 文件路径检查 ---

func isFileTool(name string) bool {
	return name == "read" || name == "write" || name == "edit"
}

// cleanPaths 预处理路径列表（在注册时调用一次，避免每次工具调用时重复计算）
// 将相对路径转为绝对路径并 Clean，统一转小写用于跨平台大小写不敏感匹配
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

	// a. 先检查黑名单（优先级高于白名单）
	for _, dp := range denyPaths {
		if strings.HasPrefix(filePath, dp+string(filepath.Separator)) || filePath == dp {
			return fmt.Errorf("路径被安全策略禁止: %s", filePath)
		}
	}

	// b. 再检查白名单（白名单非空时才生效）
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
// 安全切面 hook 注册
// ============================================================

// registerAiSecurityHook 注册 AI 工具安全拦截切面的生命周期钩子
// 在 App.BeforeInit 阶段读取配置并注册切面
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

// parseCsv 解析逗号分隔的字符串，返回去除空白的切片
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
