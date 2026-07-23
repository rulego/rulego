package types

import "context"

// MCPToolDefinition MCP tool definition. Filled by the application layer, consumed by the storage layer.
// Only standard types are used, with zero external dependencies.
type MCPToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema []byte `json:"inputSchema"` // JSON Schema bytes
}

// MCPToolProvider MCP tool provider interface.
// The application layer implements this interface and registers it in Config.Udf, while the repository layer obtains and adapts it to eino tools via UDF.
//
// Registration method:
//
//	ruleConfig.RegisterUdf("mcp_tool_provider", myProvider)
//
// How to obtain:
//
//	provider := ruleConfig.GetUdf("mcp_tool_provider", "").(types.MCPToolProvider)
type MCPToolProvider interface {
	// ListToolDefinitions returns definitions for all available tools
	ListToolDefinitions() ([]MCPToolDefinition, error)
	// CallTool calls the specified tool and returns the result text
	CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error)
}

const (
	// MCPToolProviderKey UDF registration key
	MCPToolProviderKey = "mcp_tool_provider"
)
