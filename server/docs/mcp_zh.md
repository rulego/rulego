# RuleGo Server MCP 文档

[English](mcp_en.md)

RuleGo Server 内置 MCP（Model Context Protocol）服务，允许 AI 智能体通过自然语言生成、修改和管理规则链。

## 功能特性

- **自然语言生成规则链**：描述需求，AI 自动生成规则链 JSON
- **实时预览**：使用 `preview_rule_chain` 工具预览，画布实时更新
- **规则链管理**：创建、更新、删除、部署、执行规则链
- **组件查询**：浏览可用组件、查询组件文档
- **共享节点池**：复用已配置的网络连接资源

## MCP 工具列表

| 工具名 | 功能 |
|--------|------|
| `list_rule_chains` | 列出/搜索规则链 |
| `get_rule_chain` | 获取规则链定义 JSON |
| `preview_rule_chain` | 预览规则链（校验+返回 JSON，不保存） |
| `save_rule_chain` | 创建或更新规则链（保存+部署） |
| `delete_rule_chain` | 删除规则链 |
| `operate_rule_chain` | 操作规则链（deploy/undeploy） |
| `execute_rule_chain` | 执行规则链并返回结果 |
| `list_components` | 列出可用组件 |
| `get_component_doc` | 获取组件完整文档 |
| `list_node_pool` | 列出共享节点池资源 |

## MCP 工具类型

MCP 服务支持三种工具类型：

### 管理 API 工具

由 `load_apis_as_tool = true` 控制（默认开启），即上表列出的规则链 CRUD 管理工具。

### 组件工具

组件工具通过分组配置中的 `components` 关键字加载，无需额外开关。每个已注册组件自动成为独立 MCP 工具：

- **工具名**：组件类型名（如 `jsFilter`、`restApiCall`）
- **描述**：来自 `ComponentForm.Desc`
- **参数**：从 `ComponentForm.Fields` 自动生成，包含字段名、类型、描述、默认值、必填标记
- **排除**：通过分组配置语法排除，例如 `components,-jsFilter,-*Filter`

### 规则链工具

规则链工具通过分组配置中的 `chains` 关键字加载，无需额外开关。每个已部署规则链自动成为独立 MCP 工具：

- **工具名**：规则链 ID
- **描述**：来自规则链 `additionalInfo.description`，未设置时回退到 `Name`（两者都为空则跳过）
- **参数**：来自规则链的 `inputSchema` 或 DSL 模板变量自动解析
- **动态同步**：规则链新增、更新、删除时通过回调自动同步工具列表
- **排除**：通过分组配置语法排除，例如 `chains,-_assistant,-system_*`

规则链设置描述示例（在规则链 JSON 的 `additionalInfo` 中）：

```json
{
  "ruleChain": {
    "id": "temperature_alert",
    "additionalInfo": {
      "description": "接收温度数据，超过阈值时发送告警通知"
    }
  }
}
```

## 系统智能体

RuleGo Server 内置 `_assistant` 智能体，启动时自动部署到 `data/system/agents/_assistant/`（需通过 build tag 加载 AI 组件后才生效）。

智能体功能：

- 根据自然语言描述生成规则链
- 预览和修改规则链
- 测试执行规则链
- 查询组件文档

配置 LLM 后即可使用：

```ini
[global]
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```

## API Key 获取

API Key 在配置文件的 `[users]` 部分配置：

```ini
[users]
admin = admin,2af255ea5618467d914c67a8beeca31d
```

其中 `2af255ea5618467d914c67a8beeca31d` 就是该用户的 API Key。

## MCP 分组配置

分组可以控制 AI 客户端可用的工具范围，提高安全性和专注度。

```ini
[mcp.groups]
# 只读分组
readonly = list_rule_chains,get_rule_chain,list_components,get_component_doc

# 完整分组
full = *

# 无删除分组
no-delete = *,-delete_rule_chain

# 管理分组（适用于外部 AI 客户端）
manager = preview_rule_chain,save_rule_chain,list_rule_chains,get_rule_chain,delete_rule_chain,operate_rule_chain,execute_rule_chain,list_components,get_component_doc
```

分组语法：

- `*`：全部工具
- `-prefix*`：排除前缀匹配的工具
- `rules`：管理 API 工具（list/get/preview/save/delete/operate/execute_rule_chain, list_components, get_component_doc）
- `components`：全部组件工具
- `chains`：全部规则链工具
- 具体工具名：精确匹配单个工具（如 `jsFilter`、`temperature_alert`）

分组示例（包含组件和规则链工具）：

```ini
[mcp.groups]
# 只暴露特定组件和规则链
iot_tools = components,chains,-_*Filter,-_assistant

# 只读 + 特定规则链
data_reader = list_rule_chains,get_rule_chain,temperature_alert,humidity_monitor
```

## AI IDE 配置

在支持 MCP 协议的 AI IDE 中配置 RuleGo MCP 服务。

### 通用配置

所有支持 SSE MCP 的客户端通用配置格式：

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

使用分组限制工具范围：

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY/group/manager"
    }
  }
}
```

### Claude Code 配置

**全局配置**：编辑 `~/.claude/claude_desktop_config.json`

**项目配置**：在项目根目录创建 `.mcp.json`

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

### Trae 配置

Trae 是字节跳动推出的 AI IDE，支持 MCP 协议。

1. 打开 Trae，进入设置（Settings）
2. 找到 MCP 配置项
3. 添加 RuleGo MCP 服务：

```json
{
  "mcpServers": {
    "rulego": {
      "url": "http://localhost:9090/api/v1/mcp/YOUR_API_KEY"
    }
  }
}
```

### Cursor / 其他 MCP 客户端

任何支持 SSE MCP 的客户端都可以使用上述通用配置格式，将 `url` 指向 RuleGo Server 的 MCP 端点即可。

## 示例对话

```
用户: 帮我创建一个规则链，接收 HTTP 请求，过滤出温度大于 30 的数据，然后发送到 MQTT

AI: 我来帮你创建这个规则链...
[调用 preview_rule_chain 工具预览]

用户: 看起来不错，保存吧

AI: [调用 save_rule_chain 工具保存并部署]
```

更多示例：

```
用户: 列出所有规则链

AI: [调用 list_rule_chains 工具]

用户: 获取 temperature_monitor 规则链的定义

AI: [调用 get_rule_chain 工具]

用户: 在 jsFilter 节点后面添加一个日志节点

AI: [调用 preview_rule_chain 工具预览修改后的规则链]
```
