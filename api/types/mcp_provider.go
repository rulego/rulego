package types

import "context"

// MCPToolDefinition MCP 工具定义。由应用层填充，库层消费。
// 只使用标准类型，零外部依赖。
type MCPToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema []byte `json:"inputSchema"` // JSON Schema bytes
}

// MCPToolProvider MCP 工具提供者接口。
// 应用层实现此接口并注册到 Config.Udf，库层通过 UDF 获取并适配为 eino 工具。
//
// 注册方式：
//
//	ruleConfig.RegisterUdf("mcp_tool_provider", myProvider)
//
// 获取方式：
//
//	provider := ruleConfig.GetUdf("mcp_tool_provider", "").(types.MCPToolProvider)
type MCPToolProvider interface {
	// ListToolDefinitions 返回所有可用工具的定义
	ListToolDefinitions() ([]MCPToolDefinition, error)
	// CallTool 调用指定工具，返回结果文本
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
}

const (
	// MCPToolProviderKey UDF 注册 key
	MCPToolProviderKey = "mcp_tool_provider"
)
