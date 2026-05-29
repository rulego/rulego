# RuleGo Server API 文档

## 概述

RuleGo Server 提供基于 REST 的 HTTP API，默认监听地址 `:9090`，API 基础路径为 `/api/v1`。

所有响应的 `Content-Type` 为 `application/json`（健康检查和静态资源除外）。

---

## 认证

大部分 API 需要认证，支持两种方式：

### 方式一：JWT Token

先调用 `POST /api/v1/login` 获取 token，然后在请求头中携带：

```
Authorization: Bearer <jwt_token>
```

也可通过 query 参数传递：`?token=<jwt_token>`

### 方式二：API Key

在 `config.conf` 的 `users` 中为用户配置 API Key，然后通过请求头或 query 参数传递：

```
Authorization: Bearer <api_key>
```

或：

```
X-API-Key: <api_key>
```

### 免认证模式

当 `config.conf` 中 `require_auth=false` 且请求未携带 `Authorization` 头时，系统使用 `default_username`（默认 `admin`）作为当前用户。

### 认证失败响应

| 状态码 | 响应体 | 说明 |
|---|---|---|
| 401 | `{"error":"unauthorized"}` | token 无效或已过期 |
| 403 | `{"error":"forbidden"}` | 权限不足 |

---

## 通用约定

### 路径参数 ID 校验

所有 `:id` 路径参数均经过校验：不能为空、长度不超过 256 字符、不能包含 `/`、`\`、`.` 字符。校验失败返回 400。

### 通用错误格式

所有错误响应均为 JSON 格式：

```json
{"error": "错误描述信息"}
```

| 状态码 | 说明 |
|---|---|
| 400 | 请求参数错误或业务处理失败 |
| 401 | 未认证或认证失败 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误（不暴露内部细节） |

> 注：500 错误统一返回 `{"error":"internal server error"}`，不泄露具体错误信息。

---

## 接口列表

### 1. 健康检查

```
GET /health
```

无需认证。返回纯文本 `OK`，`Content-Type` 非 JSON。

---

### 2. 根路径重定向

```
GET /
```

无需认证。302 重定向到 `/editor/`。

---

### 3. 登录

```
POST /api/v1/login
```

无需认证。有速率限制：同一 IP 每分钟最多 10 次。

**请求体：**

```json
{
  "username": "admin",
  "password": "admin"
}
```

**成功响应 (200)：**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expiresAt": 1700000000
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| token | string | JWT token |
| expiresAt | int64 | 过期时间（Unix 时间戳，秒） |

**错误响应：**

| 状态码 | 响应体 | 说明 |
|---|---|---|
| 400 | `{"error":"<解析错误信息>"}` | 请求体 JSON 解析失败 |
| 401 | `{"error":"invalid username or password"}` | 用户名或密码错误 |
| 429 | `{"error":"too many login attempts, please try again later"}` | 登录尝试次数过多 |
| 500 | `{"error":"internal server error"}` | Token 生成失败 |

---

### 4. 规则链管理

#### 4.1 获取规则链列表

```
GET /api/v1/rules
```

权限：`rule:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| keywords | string | "" | 按关键字筛选 |
| root | bool | - | 按根链路状态筛选（`true`/`false`） |
| disabled | bool | - | 按禁用状态筛选（`true`/`false`） |
| category | string | "" | 按分类筛选 |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：**

```json
{
  "total": 10,
  "page": 1,
  "size": 20,
  "items": [
    {
      "id": "chain_01",
      "name": "示例规则链",
      "rootRuleId": "",
      "disabled": false,
      "createTime": 1700000000000,
      "updateTime": 1700000000000
    }
  ]
}
```

**items 中对象字段（RuleChainMeta）：**

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |
| name | string | 规则链名称 |
| rootRuleId | string | 根规则 ID |
| disabled | bool | 是否禁用 |
| createTime | int64 | 创建时间（毫秒时间戳） |
| updateTime | int64 | 更新时间（毫秒时间戳） |

#### 4.2 获取规则链 DSL

```
GET /api/v1/rules/:id
```

权限：`rule:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |

**成功响应 (200)：** 返回规则链 DSL 的原始 JSON 字节。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 规则链 ID 无效 |
| 404 | 规则链不存在 |

#### 4.3 新增/修改规则链

```
POST /api/v1/rules/:id
```

权限：`rule:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |

**请求体：** 规则链 DSL 的 JSON 定义（`types.RuleChain`）。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 保存并部署成功 |
| 400 | 规则链 ID 无效或 DSL 无效 |

#### 4.4 删除规则链

```
DELETE /api/v1/rules/:id
```

权限：`rule:delete`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |

**响应：**

| 状态码 | 说明 |
|---|---|
| 204 | 删除成功，无响应体 |
| 400 | 规则链 ID 无效或删除失败 |

#### 4.5 部署/下线/设为主链

```
POST /api/v1/rules/:id/operate/:type
```

权限：`rule:operate`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |
| type | string | 操作类型：`start` 部署，`stop` 下线，`set-to-main` 设为主规则链 |

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 操作成功 |
| 400 | 规则链 ID 无效、未知操作类型或操作失败 |

#### 4.6 异步执行规则链

```
POST /api/v1/rules/:id/notify/:msgType
```

权限：`rule:execute`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |
| msgType | string | 消息类型（如 `JSON`、`TEXT`） |

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| _msgId | string | "" | 自定义消息 ID |
| _headersToMetadata | bool | false | 设为 `true` 时将 HTTP 请求头注入消息 Metadata |
| _fromNodeId | string | "" | 从指定节点开始执行（含该节点之后的路径） |
| _onlyNodeId | string | "" | 仅执行指定节点（执行后停止，不向下游传播） |

**请求体：** 任意字符串（消息数据载荷）。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 消息已异步发送 |

#### 4.7 获取最近修改的规则链

```
GET /api/v1/rules/:id/latest
```

权限：`rule:read`

获取当前用户最近一次保存的规则链 DSL。路径中的 `:id` 参数会被忽略（可使用任意值如 `_`）。

**成功响应 (200)：** 规则链 DSL 的原始 JSON。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 404 | 最近编辑的规则链不存在 |

#### 4.8 保存规则链基础信息

```
POST /api/v1/rules/:id/base
```

权限：`rule:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |

**请求体（RuleChainBaseInfo）：**

```json
{
  "name": "规则链名称",
  "root": false,
  "debugMode": false,
  "additionalInfo": {},
  "configuration": {}
}
```

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 保存成功 |
| 400 | 请求体格式错误或保存失败 |

#### 4.9 保存规则链配置

```
POST /api/v1/rules/:id/config/:varType
```

权限：`rule:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |
| varType | string | 配置键名 |

**请求体：** 配置值的 JSON 数据。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 保存成功 |
| 400 | 请求体格式错误或保存失败 |

#### 4.10 同步执行规则链

```
POST /api/v1/rules/:id/execute/:msgType
```

权限：`rule:execute`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |
| msgType | string | 消息类型 |

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| _msgId | string | "" | 自定义消息 ID |
| _headersToMetadata | bool | false | 设为 `true` 时将 HTTP 请求头注入消息 Metadata |
| _fromNodeId | string | "" | 从指定节点开始执行（含该节点之后的路径） |
| _onlyNodeId | string | "" | 仅执行指定节点（执行后停止，不向下游传播） |

**请求体：** 消息数据载荷。

**成功响应 (200)：** 规则链执行后的输出消息数据，`Content-Type: application/json`。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 执行失败，响应体为错误信息 |

#### 4.11 OpenAI 兼容接口

```
POST /api/v1/rules/:id/v1/chat/completions
```

权限：`rule:execute`

兼容 OpenAI Chat Completions API 格式，将请求转发到规则链处理。

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 规则链 ID |

**请求体：** OpenAI 格式的 Chat Completion 请求。支持 `stream: true` 开启 SSE 流式响应。

**成功响应 (200)：** OpenAI 格式的响应（流式或非流式）。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 请求体格式错误或执行失败 |

---

### 5. 共享节点管理

#### 5.1 获取共享节点列表

```
GET /api/v1/shared-nodes
```

权限：`component:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| keywords | string | "" | 按关键字筛选 |
| type | string | "" | 按类型筛选 |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：**

```json
{
  "total": 3,
  "page": 1,
  "size": 20,
  "items": [
    {
      "id": "mqtt_client",
      "name": "MQTT客户端",
      "type": "mqttClient"
    }
  ]
}
```

#### 5.2 添加/更新共享节点

```
POST /api/v1/shared-nodes/:id/:type
```

权限：`component:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 节点 ID |
| type | string | 组件类型 |

**请求体：** 节点定义 JSON。行为根据 `type` 参数不同而异：
- `type=endpoint`：请求体解析为 `types.EndpointDsl`
- 其他 type：请求体解析为 `types.RuleNode`

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 保存成功 |
| 400 | 请求格式错误或保存失败 |

#### 5.3 获取共享节点

```
GET /api/v1/shared-nodes/:id/:type
```

权限：`component:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 节点 ID |
| type | string | 组件类型 |

**成功响应 (200)：** 节点定义 JSON。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 404 | 节点不存在 |

#### 5.4 删除共享节点

```
DELETE /api/v1/shared-nodes/:id/:type
```

权限：`component:delete`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 节点 ID |
| type | string | 组件类型 |

**响应：**

| 状态码 | 说明 |
|---|---|
| 204 | 删除成功，无响应体 |
| 400 | 删除失败 |

---

### 6. 动态组件管理

#### 6.1 获取动态组件列表

```
GET /api/v1/dynamic-components
```

权限：`component:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| keywords | string | "" | 按关键字筛选 |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：**

```json
{
  "total": 2,
  "page": 1,
  "size": 20,
  "items": [
    {
      "ruleChain": {
        "id": "myComponent",
        "name": "自定义组件"
      }
    }
  ]
}
```

#### 6.2 获取动态组件 DSL

```
GET /api/v1/dynamic-components/:id
```

权限：`component:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 组件类型 |

**成功响应 (200)：** 组件 DSL 的原始 JSON。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 404 | 组件不存在 |

#### 6.3 安装/升级动态组件

```
POST /api/v1/dynamic-components/:id
```

权限：`component:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 组件类型 |

**请求体：** 组件 DSL 的 JSON 定义。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 安装/升级成功 |
| 400 | DSL 无效或安装失败 |

#### 6.4 卸载动态组件

```
DELETE /api/v1/dynamic-components/:id
```

权限：`component:delete`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 组件类型 |

**响应：**

| 状态码 | 说明 |
|---|---|
| 204 | 卸载成功，无响应体 |
| 400 | 卸载失败 |

---

### 7. 组件注册表

#### 7.1 获取已注册组件列表

```
GET /api/v1/components
```

权限：`component:read`

**成功响应 (200)：**

```json
{
  "endpoints": [
    {"type": "endpoint/rest", "name": "REST", "label": "...", ...},
    {"type": "endpoint/mqtt", "name": "MQTT", "label": "...", ...}
  ],
  "nodes": [
    {"type": "jsFilter", "name": "JS过滤器", "label": "...", ...}
  ],
  "tools": null,
  "builtins": {
    "endpoints": {
      "inProcessors": ["setMetadata", "..."],
      "outProcessors": ["openaiStreamingResponse", "..."]
    },
    "nodePool": {"group1": [...]},
    "globals": ["data_dir", "llm_url", "..."],
    "ai/tools": {
      "tools": [...]
    }
  },
  "skillPath": "./skills"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| endpoints | []ComponentForm | 已注册的 endpoint 组件表单列表 |
| nodes | []ComponentForm | 已注册的节点组件表单列表（含动态组件） |
| tools | null | 预留字段 |
| builtins | object | 内置资源：处理器列表、节点池定义、全局变量名、AI 工具 |
| skillPath | string | 技能文件存储路径 |

---

### 8. 系统配置

#### 8.1 获取全局配置

```
GET /api/v1/config/global
```

权限：`config:read`

**成功响应 (200)：** 全局配置 JSON 对象。若未配置则返回 `{}`。

#### 8.2 更新全局配置

```
POST /api/v1/config/global
```

权限：`config:write`

**请求体：** 全局配置 JSON 对象（`map[string]interface{}`）。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 更新成功 |
| 400 | 请求体格式错误或更新失败 |

---

### 9. AI 助手管理

#### 9.1 获取助手系统提示词

```
GET /api/v1/system/agents/:id/prompt
```

权限：`config:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 助手 ID（如 `chainAssistant`，默认 `_assistant`） |

**成功响应 (200)：**

```json
{
  "agentId": "chainAssistant",
  "content": "你是一个智能助手..."
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| agentId | string | 助手 ID |
| content | string | 系统提示词内容 |

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 助手 ID 无效 |
| 404 | `{"error":"assistant prompt not found"}` |

#### 9.2 更新助手系统提示词

```
POST /api/v1/system/agents/:id/prompt
```

权限：`config:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 助手 ID |

**请求体（JSON）：**

```json
{
  "content": "你是一个新的系统提示词..."
}
```

**成功响应 (200)：**

```json
{
  "agentId": "chainAssistant",
  "content": "你是一个新的系统提示词..."
}
```

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 助手 ID 无效或请求体格式错误 |
| 500 | 写入或重载失败 |

#### 9.3 获取助手模型配置

```
GET /api/v1/system/agents/:id/model
```

权限：`config:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 助手 ID |

**成功响应 (200)：**

```json
{
  "agentId": "chainAssistant",
  "model": {
    "provider": "openai",
    "url": "https://api.openai.com/v1",
    "key": "sk-***",
    "model": "gpt-4o",
    "maxStep": 25,
    "maxToolOutputLength": 0,
    "params": {
      "temperature": 0.7,
      "topP": 1,
      "frequencyPenalty": 0,
      "presencePenalty": 0,
      "maxTokens": 0
    }
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| agentId | string | 助手 ID |
| model.provider | string | LLM 提供商 |
| model.url | string | API 地址 |
| model.key | string | API 密钥 |
| model.model | string | 模型名称 |
| model.maxStep | int | 最大执行步数 |
| model.maxToolOutputLength | int | 工具输出最大长度（0 表示不限制） |
| model.params | object | 模型参数 |
| model.params.temperature | float64 | 温度 |
| model.params.topP | float64 | Top-P |
| model.params.frequencyPenalty | float64 | 频率惩罚 |
| model.params.presencePenalty | float64 | 存在惩罚 |
| model.params.maxTokens | int | 最大输出 token 数 |

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 助手 ID 无效 |
| 404 | `{"error":"assistant model config not found"}` |

#### 9.4 更新助手模型配置

```
POST /api/v1/system/agents/:id/model
```

权限：`config:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 助手 ID |

**请求体：** 模型配置 JSON（结构与 9.3 中 `model` 字段一致）。

**成功响应 (200)：** 与 9.3 成功响应格式一致，返回更新后的完整配置。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 助手 ID 无效或请求体格式错误 |
| 500 | 写入或重载失败 |

---

### 10. 技能管理

#### 10.1 获取技能列表

```
GET /api/v1/skills
```

权限：`skill:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| scope | string | "" | 作用域过滤（当前仅支持 `global`） |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：**

```json
{
  "path": "./skills",
  "total": 5,
  "page": 1,
  "size": 20,
  "items": [
    {
      "name": "mySkill",
      "description": "技能描述",
      "content": "# 技能内容\n...",
      "path": "./skills/mySkill.md",
      "scope": "global",
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2024-01-01T00:00:00Z"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| path | string | 技能存储根路径 |
| total | int | 总数 |
| page | int | 页码 |
| size | int | 每页数量 |
| items | []Skill | 技能列表 |

#### 10.2 获取技能详情

```
GET /api/v1/skills/:id
```

权限：`skill:read`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 技能名称 |

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| scope | string | "" | 作用域 |

**成功响应 (200)：** 返回单个 `Skill` 对象（同 10.1 中 items 元素结构）。

#### 10.3 创建技能

```
POST /api/v1/skills
```

权限：`skill:write`

**请求体：**

```json
{
  "name": "mySkill",
  "description": "技能描述",
  "scope": "global",
  "content": "# 技能内容\n..."
}
```

**成功响应 (201)：** 返回创建的 `Skill` 对象。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 请求体格式错误、scope 无效，或创建失败 |

#### 10.4 更新技能

```
PUT /api/v1/skills/:id
```

权限：`skill:write`

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| id | string | 技能名称 |

**请求体：** 同创建技能（`name` 字段会被路径参数覆盖）。

**成功响应 (200)：** 返回更新后的 `Skill` 对象。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 请求体格式错误、scope 无效，或更新失败 |

#### 10.5 删除技能

```
DELETE /api/v1/skills/:id
```

权限：`skill:delete`：** 无响应体。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 技能 ID 无效、scope 无效，或删除失败 |

#### 10.6 上传技能

```
POST /api/v1/skills/upload
```

权限：`skill:write`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| scope | string | "" | 作用域 |

**请求体：** `multipart/form-data` 格式，字段名为 `file`，包含技能归档文件（ZIP，最大 64MB）。

**成功响应 (200)：** 返回导入的技能列表 `[]Skill`。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 无上传文件、格式错误，或导入失败 |

---

### 11. 运行日志

#### 11.1 获取运行日志

```
GET /api/v1/logs/runs
```

权限：`log:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| id | string | "" | 按日志 ID 查询（传入时忽略其他参数，返回单条记录） |
| chainId | string | "" | 按规则链 ID 筛选 |
| startTime | int64 | - | 开始时间（毫秒时间戳） |
| endTime | int64 | - | 结束时间（毫秒时间戳） |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**行为：**
- 传入 `id`：返回单条日志 JSON 对象
- 不传 `id`：返回分页列表

**成功响应 — 单条 (200)：**

```json
{
  "id": "evt_001",
  "chainId": "chain_01",
  "chainName": "示例规则链",
  "startTs": 1700000000000,
  "endTs": 1700000001000,
  "success": true,
  "errorMsg": "",
  "logs": null
}
```

**成功响应 — 列表 (200)：**

```json
{
  "total": 5,
  "page": 1,
  "size": 20,
  "items": [
    {
      "id": "evt_001",
      "chainId": "chain_01",
      "chainName": "示例规则链",
      "startTs": 1700000000000,
      "endTs": 1700000001000,
      "success": true,
      "errorMsg": "",
      "logs": null
    }
  ]
}
```

**Event 对象字段：**

| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 日志 ID |
| chainId | string | 规则链 ID |
| chainName | string | 规则链名称 |
| startTs | int64 | 开始时间（毫秒时间戳） |
| endTs | int64 | 结束时间（毫秒时间戳） |
| success | bool | 是否执行成功 |
| errorMsg | string | 错误信息（失败时有值） |
| logs | object | 节点执行日志详情（可选） |

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 404 | 按ID查询时日志不存在 |

#### 11.2 删除运行日志

```
DELETE /api/v1/logs/runs
```

权限：`log:delete`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| id | string | "" | 按日志 ID 删除 |
| chainId | string | "" | 按规则链 ID 删除（删除该链所有日志） |

至少传入 `id` 或 `chainId` 之一。

**响应：**

| 状态码 | 说明 |
|---|---|
| 204 | 删除成功，无响应体 |
| 400 | `chainId` 和 `id` 均未传入 |
| 500 | 删除失败 |

#### 11.3 获取节点调试日志

```
GET /api/v1/logs/debug
```

权限：`log:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| chainId | string | "" | 规则链 ID |
| nodeId | string | "" | 节点 ID |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：**

```json
{
  "total": 10,
  "page": 1,
  "size": 20,
  "items": [
    {
      "chainId": "chain_01",
      "nodeId": "node_1",
      "flowType": "In",
      "relationType": "Success",
      "msg": {},
      "err": "",
      "ts": 1700000000000
    }
  ]
}
```

#### 11.4 WebSocket 实时调试

```
WS /api/v1/logs/ws/:chainId/:clientId?token={jwtToken}
```

需要认证（通过 query 参数 `token` 传递 JWT）。

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| chainId | string | 规则链 ID |
| clientId | string | 客户端唯一标识 |

连接后，服务端实时推送节点调试数据，每条消息格式：

```json
{
  "chainId": "chain_01",
  "flowType": "In",
  "nodeId": "node_1",
  "relationType": "Success",
  "err": "",
  "msg": {},
  "ts": 1700000000000
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| chainId | string | 规则链 ID |
| flowType | string | 流向：`In` 或 `Out` |
| nodeId | string | 节点 ID |
| relationType | string | 关系类型 |
| err | string | 错误信息 |
| msg | object | 消息内容 |
| ts | int64 | 时间戳（毫秒） |

---

### 12. 国际化

#### 12.1 获取语言列表 / 语言包

```
GET /api/v1/locales
```

权限：`locale:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| lang | string | "" | 语言代码（如 `en`、`zh_cn`） |

**行为：**

- 传入 `lang`：返回对应语言包的 JSON 对象。
- 不传 `lang`：返回可用语言代码列表。

**成功响应（不传 lang）：**

```json
["en", "zh_cn"]
```

#### 12.2 保存语言包

```
POST /api/v1/locales?lang={lang}
```

权限：`locale:write`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| lang | string | "en" | 语言代码 |

**请求体：** 语言包 JSON 对象。

**响应：**

| 状态码 | 说明 |
|---|---|
| 200 | 保存成功 |
| 400 | 保存失败 |

---

### 13. 组件市场

#### 13.1 获取市场组件列表

```
GET /api/v1/marketplace/components
```

权限：`marketplace:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| keywords | string | "" | 按关键字筛选 |
| checkMy | bool | false | 设为 `true` 时标记已安装和可升级状态 |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：** 标准分页格式（`total`/`page`/`size`/`items`）。当 `checkMy=true` 时，`items` 中每个组件的 `ruleChain.additionalInfo` 会附加 `installed`（bool）和 `upgraded`（bool）字段。

#### 13.2 获取市场规则链列表

```
GET /api/v1/marketplace/chains
```

权限：`marketplace:read`

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| keywords | string | "" | 按关键字筛选 |
| root | bool | - | 按根规则链筛选 |
| page | int | 1 | 页码 |
| size | int | 20 | 每页数量 |

**成功响应 (200)：** 标准分页格式（`total`/`page`/`size`/`items`）。

---

### 14. MCP（Model Context Protocol）

MCP 端点使用独立的 API Key 认证（通过路径参数、`Authorization` 头或 `X-API-Key` 头传递），不走标准 auth 中间件。

认证优先级：URL 路径参数 → `Authorization: Bearer` 头 → `X-API-Key` 头。

#### 14.1 MCP StreamableHTTP 端点

```
GET/POST/DELETE /api/v1/mcp/:apiKey
```

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| apiKey | string | 用户的 API Key |

使用 MCP StreamableHTTP 传输协议。`POST` 发送 JSON-RPC 请求，`GET` 建立 SSE 事件流，`DELETE` 关闭会话。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 401 | API Key 无效 |
| 500 | MCP 处理失败 |

#### 14.2 MCP 分组 StreamableHTTP 端点

```
GET/POST/DELETE /api/v1/mcp/:apiKey/group/:group
```

**路径参数：**

| 参数 | 类型 | 说明 |
|---|---|---|
| apiKey | string | 用户的 API Key |
| group | string | 工具分组名称 |

按分组暴露 MCP 工具子集，传输协议与 14.1 一致。

**错误响应：**

| 状态码 | 说明 |
|---|---|
| 400 | 分组名称为空 |
| 401 | API Key 无效 |
| 500 | MCP 处理失败 |

---

### 15. 静态资源

| 路径 | 说明 |
|---|---|
| `/editor/*` | 编辑器前端静态资源 |
| `/images/*` | 图片资源 |

路径映射通过 `config.conf` 的 `resource_mapping` 配置。

---

## 配置参考

影响 API 行为的关键配置项（`config.conf`）：

| 配置项 | 默认值 | 说明 |
|---|---|---|
| server | `:9090` | HTTP 监听地址 |
| base_path | "" | API 路由前缀（嵌入模式使用） |
| require_auth | false | 是否强制认证 |
| default_username | `admin` | 免认证时的默认用户 |
| jwt_secret_key | (内置) | JWT 签名密钥（建议通过环境变量 `JWT_SECRET_KEY` 设置） |
| jwt_expire_time | `43200000` | JWT 过期时间（毫秒，默认 12 小时） |
| jwt_issuer | `rulego.cc` | JWT 签发者 |
| resource_mapping | (内置) | 静态文件路径映射 |
| allow_cors | true | 是否允许跨域 |
| read_timeout | 30 | HTTP 读超时（秒） |
| write_timeout | 300 | HTTP 写超时（秒，AI 聊天需要较长超时） |
| max_body_size | 10 | 请求体最大大小（MB） |
| save_run_log | false | 是否保存运行日志 |
| run_log_store_type | `bbolt` | 运行日志存储类型：`bbolt` 或 `file`（JSON Lines） |
| pprof.enable | `false` | 是否启用 pprof |
| pprof.addr | `0.0.0.0:6060` | pprof 服务地址 |
| mcp.enable | `true` | 是否启用 MCP 服务 |
| marketplace_base_url | - | 组件市场远程地址 |
| skill_path | `./skills` | 技能文件存储路径 |
