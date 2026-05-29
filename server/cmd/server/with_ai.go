//go:build with_ai || with_all

package main

import (
	"context"
	"encoding/json"

	einoTool "github.com/cloudwego/eino/components/tool"
	aitool "github.com/rulego/rulego-components-ai/tool"

	// 一键引入所有 AI 组件（节点、工具、Endpoint、Processor）
	// 按需引入可直接导入对应子包，如: _ "github.com/rulego/rulego-components-ai/agent"
	_ "github.com/rulego/rulego-components-ai/all"

	"github.com/rulego/rulego/api/types"
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
