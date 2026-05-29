# RuleGo Server MCP Documentation

[中文](mcp_zh.md)

RuleGo Server includes a built-in MCP (Model Context Protocol) service that allows AI agents to generate, modify, and manage rule chains through natural language.

## Features

- **Natural language rule chain generation**: Describe your requirements and AI generates rule chain JSON
- **Real-time preview**: Use the `preview_rule_chain` tool to preview; the canvas updates in real time
- **Rule chain management**: Create, update, delete, deploy, and execute rule chains
- **Component query**: Browse available components and query component documentation
- **Shared node pool**: Reuse configured network connection resources

## MCP Tool List

| Tool Name | Description |
|-----------|-------------|
| `list_rule_chains` | List/search rule chains |
| `get_rule_chain` | Get rule chain definition JSON |
| `preview_rule_chain` | Preview rule chain (validate + return JSON, no save) |
| `save_rule_chain` | Create or update rule chain (save + deploy) |
| `delete_rule_chain` | Delete a rule chain |
| `operate_rule_chain` | Operate rule chain (deploy/undeploy) |
| `execute_rule_chain` | Execute rule chain and return result |
| `list_components` | List available components |
| `get_component_doc` | Get full component documentation |
| `list_node_pool` | List shared node pool resources |

## MCP Tool Types

The MCP service supports three tool types:

### Management API Tools

Controlled by `load_apis_as_tool = true` (enabled by default). These are the rule chain CRUD management tools listed in the table above.

### Component Tools

Component tools are loaded via the `components` keyword in group configuration, no additional switch needed. Each registered component becomes an independent MCP tool:

- **Tool name**: Component type name (e.g., `jsFilter`, `restApiCall`)
- **Description**: From `ComponentForm.Desc`
- **Parameters**: Auto-generated from `ComponentForm.Fields`, including field name, type, description, default value, and required flag
- **Exclusion**: Use group configuration syntax to exclude, e.g., `components,-jsFilter,-*Filter`

### Rule Chain Tools

Rule chain tools are loaded via the `chains` keyword in group configuration, no additional switch needed. Each deployed rule chain becomes an independent MCP tool:

- **Tool name**: Rule chain ID
- **Description**: From `additionalInfo.description`, falls back to `Name` if not set (skipped if both are empty)
- **Parameters**: Auto-resolved from `inputSchema` or DSL template variables
- **Dynamic sync**: Tools are automatically synced via callbacks when chains are added, updated, or deleted
- **Exclusion**: Use group configuration syntax to exclude, e.g., `chains,-_assistant,-system_*`

Example of setting a rule chain description (in the chain JSON's `additionalInfo`):

```json
{
  "ruleChain": {
    "id": "temperature_alert",
    "additionalInfo": {
      "description": "Receives temperature data and sends alert notifications when threshold is exceeded"
    }
  }
}
```

## Built-in Agent

RuleGo Server includes a built-in `_assistant` agent, automatically deployed to `data/system/agents/_assistant/` at startup (requires AI components loaded via build tag).

Agent capabilities:

- Generate rule chains from natural language descriptions
- Preview and modify rule chains
- Test-execute rule chains
- Query component documentation

Enable it by configuring an LLM:

```ini
[global]
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```

## API Key

API Keys are configured in the `[users]` section of the configuration file:

```ini
[users]
admin = admin,2af255ea5618467d914c67a8beeca31d
```

The value `2af255ea5618467d914c67a8beeca31d` is the API Key for that user.

## MCP Group Configuration

Groups control which tools are available to AI clients, improving security and focus.

```ini
[mcp.groups]
# Read-only group
readonly = list_rule_chains,get_rule_chain,list_components,get_component_doc

# Full access group
full = *

# No-delete group
no-delete = *,-delete_rule_chain

# Manager group (for external AI clients)
manager = preview_rule_chain,save_rule_chain,list_rule_chains,get_rule_chain,delete_rule_chain,operate_rule_chain,execute_rule_chain,list_components,get_component_doc
```

Group syntax:

- `*`: All tools
- `-prefix*`: Exclude tools matching the prefix
- `rules`: Management API tools (list/get/preview/save/delete/operate/execute_rule_chain, list_components, get_component_doc)
- `components`: All component tools
- `chains`: All rule chain tools
- Specific tool name: Exact match for a single tool (e.g., `jsFilter`, `temperature_alert`)

Group examples (including component and chain tools):

```ini
[mcp.groups]
# Expose specific components and rule chains
iot_tools = components,chains,-_*Filter,-_assistant

# Read-only + specific rule chains
data_reader = list_rule_chains,get_rule_chain,temperature_alert,humidity_monitor
```

## AI IDE Configuration

Configure the RuleGo MCP service in AI IDEs that support the MCP protocol.

### Generic Configuration

All SSE MCP-compatible clients use this format:

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

Use groups to limit available tools:

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY/group/manager"
    }
  }
}
```

### Claude Code Configuration

**Global config**: Edit `~/.claude/claude_desktop_config.json`

**Project config**: Create `.mcp.json` in your project root

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

### Trae Configuration

Trae is an AI IDE by ByteDance that supports the MCP protocol.

1. Open Trae and go to Settings
2. Find the MCP configuration section
3. Add the RuleGo MCP service:

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

### Cursor / Other MCP Clients

Any client supporting SSE MCP can use the generic configuration format above. Point the `url` to the RuleGo Server MCP endpoint.

## Example Conversations

```
User: Create a rule chain that receives HTTP requests, filters data with temperature > 30, then sends to MQTT

AI: Let me create this rule chain for you...
[Calls preview_rule_chain tool to preview]

User: Looks good, save it

AI: [Calls save_rule_chain tool to save and deploy]
```

More examples:

```
User: List all rule chains

AI: [Calls list_rule_chains tool]

User: Get the definition of the temperature_monitor rule chain

AI: [Calls get_rule_chain tool]

User: Add a log node after the jsFilter node

AI: [Calls preview_rule_chain tool to preview the modified rule chain]
```
