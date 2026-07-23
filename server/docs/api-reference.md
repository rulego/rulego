# RuleGo Server API Documentation

## Overview

RuleGo Server provides REST-based HTTP API, with the default listening address `:9090` and the API base path set to `/api/v1`.

The `Content-Type` for all responses was `application/json` (excluding health checks and static resources).

---

## Authentication

Most API require authentication and support two methods:

### Method One: JWT Token

First, call `POST /api/v1/login` to get token, then carry the following in the request header:

```
Authorization: Bearer <jwt_token>
```

It can also be passed via query parameters: `?token=<jwt_token>`

### Method Two: API Key

Configure API Key for users in the `users` of `config.conf`, then pass it via request headers or query parameters:

```
Authorization: Bearer <api_key>
```

Or:

```
X-API-Key: <api_key>
```

### Authentication-disabled Mode

When `require_auth=false` is in the `config.conf` and the request does not include the `Authorization` header, the system uses `default_username` (default `admin`) as the current user.

### Authentication Failure Response

| Status code | Response body | Note |
|---|---|---|
| 401 | `{"error":"unauthorized"}` | token Invalid or expired |
| 403 | `{"error":"forbidden"}` | Insufficient permissions |

---

## General Conventions

### Path Parameters ID Validation

All `:id` path parameters are verified: must not be empty, must not exceed 256 characters, and cannot contain `/`, `\`, or `.` characters. Verification failed, returned 400.

### Standard Error Format

All error responses are in JSON format:

```json
{"error": "错误描述信息"}
```

| Status code | Note |
|---|---|
| 400 | Incorrect request parameters or business processing failure |
| 401 | Unauthenticated or authentication failed |
| 403 | Insufficient permissions |
| 404 | Resources do not exist |
| 429 | Requests are too frequent |
| 500 | Internal server errors (not exposing internal details) |

> Note: 500 errors return a uniform `{"error":"internal server error"}`, without revealing specific error information.

---

## Interface List

### 1. Health checkup

```
GET /health
```

No authentication required. Returns plain text `OK`, `Content-Type` non-JSON.

---

### 2. Root path redirect

```
GET /
```

No authentication required. 302 redirects to `/editor/`.

---

### 3. Log in

```
POST /api/v1/login
```

No authentication required. There is a rate limit: up to 10 times per minute per IP per minute for the same session.

**Request body:**

```json
{
  "username": "admin",
  "password": "admin"
}
```

**Successful Responses (200):**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expiresAt": 1700000000
}
```

| Field | Type | Note |
|---|---|---|
| token | string | JWT token |
| expiresAt | int64 | Expiration time (Unix timestamp, seconds) |

**Error response:**

| Status code | Response body | Note |
|---|---|---|
| 400 | `{"error":"<解析错误信息>"}` | Request body JSON parsing failed |
| 401 | `{"error":"invalid username or password"}` | Incorrect username or password |
| 429 | `{"error":"too many login attempts, please try again later"}` | Too many login attempts |
| 500 | `{"error":"internal server error"}` | Token Generation failed |

---

### 4. Rule chain management

#### 4.1 Obtain the list of rule chains

```
GET /api/v1/rules
```

Permission: `rule:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| keywords | string | "" | Filter by keyword |
| root | bool | - | Filter by root link state (`true` / `false`) |
| disabled | bool | - | Filter by disabled status (`true` / `false`) |
| category | string | "" | Filter by category |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):**

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

**items Object field (RuleChainMeta):**

| Field | Type | Note |
|---|---|---|
| id | string | Rule chain ID |
| name | string | Rule chain name |
| rootRuleId | string | Root rule ID |
| disabled | bool | Whether disabled |
| createTime | int64 | Creation time (millisecond timestamp) |
| updateTime | int64 | Update time (millisecond timestamp) |

#### 4.2 Obtaining the Rule Chain DSL

```
GET /api/v1/rules/:id
```

Permission: `rule:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |

**Successful response (200):** returns the original JSON bytes of the rule chain DSL.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Rule chain ID Invalid |
| 404 | The rule chain does not exist |

#### 4.3 Add/Modify Rule Chains

```
POST /api/v1/rules/:id
```

Permission: `rule:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |

**Request body:** JSON definition of the rule chain DSL (`types. RuleChain`).

**Response:**

| Status code | Note |
|---|---|
| 200 | Save and deploy successfully |
| 400 | Rule chain ID invalid or DSL invalid |

#### 4.4 Delete the rule chain

```
DELETE /api/v1/rules/:id
```

Permission: `rule:delete`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |

**Response:**

| Status code | Note |
|---|---|
| 204 | Deleted successfully, no response body |
| 400 | Rule chain ID Invalid or deletion failed |

#### 4.5 Deploy/Offline/Set as Main Chain

```
POST /api/v1/rules/:id/operate/:type
```

Permission: `rule:operate`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |
| type | string | Operation type: `start` Deploy, `stop` Offline, `set-to-main` Set as the main rule chain |

**Response:**

| Status code | Note |
|---|---|
| 200 | Operation successful |
| 400 | Rule chain ID Invalid, unknown operation type, or operation failure |

#### 4.6 Asynchronous Execution of Rule Chains

```
POST /api/v1/rules/:id/notify/:msgType
```

Permission: `rule:execute`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |
| msgType | string | Message type (such as `JSON`, `TEXT`) |

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| _msgId | string | "" | Custom message ID |
| _headersToMetadata | bool | false | When set to `true`, injects the HTTP request header into message Metadata |
| _fromNodeId | string | "" | Starts executing from the specified node (including the path after that node) |
| _onlyNodeId | string | "" | Only executes specified nodes (stops after execution, does not propagate downstream) |

**Request body:** Any string (message data payload).

**Response:**

| Status code | Note |
|---|---|
| 200 | The message has been sent asynchronously |

#### 4.7 Get the most recently modified rule chain

```
GET /api/v1/rules/:id/latest
```

Permission: `rule:read`

Retrieves the most recent rule chain DSL saved by the current user. `:id` parameters in the path will be ignored (any value such as `_` can be used).

**Successful Response (200):** Original JSON of the rule chain DSL.

**Error response:**

| Status code | Note |
|---|---|
| 404 | The recently edited rule chain does not exist |

#### 4.8 Save basic information of the rule chain

```
POST /api/v1/rules/:id/base
```

Permission: `rule:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |

**Request body (RuleChainBaseInfo):**

```json
{
  "name": "规则链名称",
  "root": false,
  "debugMode": false,
  "additionalInfo": {},
  "configuration": {}
}
```

**Response:**

| Status code | Note |
|---|---|
| 200 | Saved successfully |
| 400 | Request body formatting error or save failure |

#### 4.9 Save Rule Chain Configuration

```
POST /api/v1/rules/:id/config/:varType
```

Permission: `rule:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |
| varType | string | Configure the key name |

**Request Body:** JSON data for configuration values.

**Response:**

| Status code | Note |
|---|---|
| 200 | Saved successfully |
| 400 | Request body formatting error or save failure |

#### 4.10 Synchronous Execution of Rule Chains

```
POST /api/v1/rules/:id/execute/:msgType
```

Permission: `rule:execute`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |
| msgType | string | Message type |

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| _msgId | string | "" | Custom message ID |
| _headersToMetadata | bool | false | When set to `true`, injects the HTTP request header into message Metadata |
| _fromNodeId | string | "" | Starts executing from the specified node (including the path after that node) |
| _onlyNodeId | string | "" | Only executes specified nodes (stops after execution, does not propagate downstream) |

**Request body:** message data payload.

**Successful Response (200):** Output message data after rule chain execution, `Content-Type: application/json`.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Execution failed, response body is error message |

#### 4.11 OpenAI Compatible interfaces

```
POST /api/v1/rules/:id/v1/chat/completions
```

Permission: `rule:execute`

Compatible with OpenAI Chat Completions API format, forwarding requests to the rule chain for processing.

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Rule chain ID |

**Request body: Chat Completion request in** OpenAI format. Supports `stream: true` to enable SSE streaming response.

**Successful Response (200): Response in** OpenAI format (streaming or non-streaming).

**Error response:**

| Status code | Note |
|---|---|
| 400 | Request body formatting error or execution failure |

---

### 5. Shared node management

#### 5.1 Obtain the list of shared nodes

```
GET /api/v1/shared-nodes
```

Permission: `component:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| keywords | string | "" | Filter by keyword |
| type | string | "" | Filter by type |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):**

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

#### 5.2 Add/Update Shared Nodes

```
POST /api/v1/shared-nodes/:id/:type
```

Permission: `component:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Node ID |
| type | string | Component type |

**Request Body:** node defines JSON. Behavior varies depending on `type` parameters:
- `type=endpoint`: The request body is parsed as `types.EndpointDsl`
- Other type: The request body is parsed as `types.RuleNode`

**Response:**

| Status code | Note |
|---|---|
| 200 | Saved successfully |
| 400 | Request format error or save failure |

#### 5.3 Obtaining Shared Nodes

```
GET /api/v1/shared-nodes/:id/:type
```

Permission: `component:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Node ID |
| type | string | Component type |

**Successful Response (200):** node defined JSON.

**Error response:**

| Status code | Note |
|---|---|
| 404 | The node does not have |

#### 5.4 Deleting Shared Nodes

```
DELETE /api/v1/shared-nodes/:id/:type
```

Permission: `component:delete`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Node ID |
| type | string | Component type |

**Response:**

| Status code | Note |
|---|---|
| 204 | Deleted successfully, no response body |
| 400 | Delete failed |

---

### 6. Dynamic component management

#### 6.1 Obtaining a Dynamic Component List

```
GET /api/v1/dynamic-components
```

Permission: `component:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| keywords | string | "" | Filter by keyword |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):**

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

#### 6.2 Obtaining Dynamic Component DSL

```
GET /api/v1/dynamic-components/:id
```

Permission: `component:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Component type |

**Successful Response (200):** original JSON of component DSL.

**Error response:**

| Status code | Note |
|---|---|
| 404 | The component does not exist |

#### 6.3 Installing/Upgrading Dynamic Components

```
POST /api/v1/dynamic-components/:id
```

Permission: `component:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Component type |

**Request Body: The JSON definition of** component DSL.

**Response:**

| Status code | Note |
|---|---|
| 200 | Installation/Upgrade Successful |
| 400 | DSL Invalid or installation failed |

#### 6.4 Uninstalling Dynamic Components

```
DELETE /api/v1/dynamic-components/:id
```

Permission: `component:delete`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Component type |

**Response:**

| Status code | Note |
|---|---|
| 204 | Uninstallation successful, no response body |
| 400 | Uninstall failed |

---

### 7. Component registry

#### 7.1 Get the list of registered components

```
GET /api/v1/components
```

Permission: `component:read`

**Successful Responses (200):**

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

| Field | Type | Note |
|---|---|---|
| endpoints | []ComponentForm | List of registered endpoint component forms |
| nodes | []ComponentForm | List of registered node component forms (including dynamic components) |
| tools | null | Reserved field |
| builtins | object | Built-in resources: processor list, node pool definitions, global variable names, AI tools |
| skillPath | string | Skill file storage path |

---

### 8. System configuration

#### 8.1 Obtaining Global Configuration

```
GET /api/v1/config/global
```

Permission: `config:read`

**Successful Response (200):** Global configuration of JSON objects. If not configured, return `{}`.

#### 8.2 Global configuration update

```
POST /api/v1/config/global
```

Permission: `config:write`

**Request Body:** Global configuration of JSON object (`map[string]interface{}`).

**Response:**

| Status code | Note |
|---|---|
| 200 | Update successful |
| 400 | Request body formatting error or update failure |

---

### 9. AI Assistant management

#### 9.1 Obtaining Assistant System Prompts

```
GET /api/v1/system/agents/:id/prompt
```

Permission: `config:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Assistant ID (e.g., `chainAssistant`, default `_assistant`) |

**Successful Responses (200):**

```json
{
  "agentId": "chainAssistant",
  "content": "你是一个智能助手..."
}
```

| Field | Type | Note |
|---|---|---|
| agentId | string | Assistant ID |
| content | string | System prompt content |

**Error response:**

| Status code | Note |
|---|---|
| 400 | Assistant ID Invalid |
| 404 | `{"error":"assistant prompt not found"}` |

#### 9.2 Updated Assistant System Prompts

```
POST /api/v1/system/agents/:id/prompt
```

Permission: `config:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Assistant ID |

**Request body (JSON):**

```json
{
  "content": "你是一个新的系统提示词..."
}
```

**Successful Responses (200):**

```json
{
  "agentId": "chainAssistant",
  "content": "你是一个新的系统提示词..."
}
```

**Error response:**

| Status code | Note |
|---|---|
| 400 | Assistant ID Invalid or request body formatting error |
| 500 | Write or reload failure |

#### 9.3 Obtaining Assistant Model Configuration

```
GET /api/v1/system/agents/:id/model
```

Permission: `config:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Assistant ID |

**Successful Responses (200):**

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

| Field | Type | Note |
|---|---|---|
| agentId | string | Assistant ID |
| model.provider | string | LLM Provider |
| model.url | string | API Address |
| model.key | string | API Key |
| model.model | string | Model Name |
| model.maxStep | int | Maximum number of steps |
| model.maxToolOutputLength | int | Tool output maximum length (0 means no limit) |
| model.params | object | Model parameters |
| model.params.temperature | float64 | Temperature |
| model.params.topP | float64 | Top-P |
| model.params.frequencyPenalty | float64 | Frequency penalty |
| model.params.presencePenalty | float64 | There is a penalty |
| model.params.maxTokens | int | Maximum output token number |

**Error response:**

| Status code | Note |
|---|---|
| 400 | Assistant ID Invalid |
| 404 | `{"error":"assistant model config not found"}` |

#### 9.4 Updated the assistant model configuration

```
POST /api/v1/system/agents/:id/model
```

Permission: `config:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Assistant ID |

**Request Body:** Model Configuration JSON (structure consistent with field `model` in 9.3).

**Successful Response (200):** Consistent format with 9.3 successful response, returning the updated full configuration.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Assistant ID Invalid or request body formatting error |
| 500 | Write or reload failure |

---

### 10. Skills management

#### 10.1 Obtain the skill list

```
GET /api/v1/skills
```

Permission: `skill:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| scope | string | "" | Scope filtering (currently only supports `global`) |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):**

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

| Field | Type | Note |
|---|---|---|
| path | string | Skill storage root path |
| total | int | Total |
| page | int | Page number |
| size | int | Number per page |
| items | []Skill | Skill List |

#### 10.2 Obtain skill details

```
GET /api/v1/skills/:id
```

Permission: `skill:read`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Skill Name |

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| scope | string | "" | Scope |

**Successful response (200):** returns a single `Skill` object (same items element structure as in 10.1).

#### 10.3 Creating Skills

```
POST /api/v1/skills
```

Permission: `skill:write`

**Request body:**

```json
{
  "name": "mySkill",
  "description": "技能描述",
  "scope": "global",
  "content": "# 技能内容\n..."
}
```

**Successful Response (201):** Returns the created `Skill` object.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Request body formatting error, scope invalid, or failed creation |

#### 10.4 Skill Update

```
PUT /api/v1/skills/:id
```

Permission: `skill:write`

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| id | string | Skill Name |

**Request body:** and create skills (`name` fields will be overwritten by path parameters).

**Successful response (200):** Returns the updated `Skill` object.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Incorrect request body formatting, invalid scope, or update failure |

#### 10.5 Removing skills

```
DELETE /api/v1/skills/:id
```

Permission: `skill:delete`: ** No response entity.

**Error response:**

| Status code | Note |
|---|---|
| 400 | Skill ID invalid, scope invalid, or failed to remove |

#### 10.6 Upload Skills

```
POST /api/v1/skills/upload
```

Permission: `skill:write`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| scope | string | "" | Scope |

**Request body:** `multipart/form-data` format, field name `file`, containing skill archive files (ZIP, maximum 64MB).

**Successful Response (200):** Returns the list of imported skills `[]Skill`.

**Error response:**

| Status code | Note |
|---|---|
| 400 | No file uploads, formatting errors, or import failures |

---

### 11. Runlog

#### 11.1 Get the Runtime Log

```
GET /api/v1/logs/runs
```

Permission: `log:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| id | string | "" | Query by log ID (ignore other parameters when passing, return a single record) |
| chainId | string | "" | Filter by rule chain ID |
| startTime | int64 | - | Start time (millisecond timestamp) |
| endTime | int64 | - | End time (millisecond timestamp) |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Behavior:**
- Passing `id`: Returns a single log JSON object
- Do not `id`: Return to paginated list

**Successful responses — Single (200):**

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

**Successful responses — List (200):**

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

**Event Object field:**

| Field | Type | Note |
|---|---|---|
| id | string | Log ID |
| chainId | string | Rule chain ID |
| chainName | string | Rule chain name |
| startTs | int64 | Start time (millisecond timestamp) |
| endTs | int64 | End time (millisecond timestamp) |
| success | bool | Whether execution succeeded |
| errorMsg | string | Error message (value on failure) |
| logs | object | Node execution log details (optional) |

**Error response:**

| Status code | Note |
|---|---|
| 404 | When querying ID, the log does not exist |

#### 11.2 Delete the Runlog

```
DELETE /api/v1/logs/runs
```

Permission: `log:delete`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| id | string | "" | Delete |ID by log
| chainId | string | "" | Delete by rule chain ID (delete all logs on that chain) |

At least one of the `id` or `chainId` is introduced.

**Response:**

| Status code | Note |
|---|---|
| 204 | Deleted successfully, no response body |
| 400 | Neither `chainId` nor `id` is sent in |
| 500 | Delete failed |

#### 11.3 Obtain Node Debug Logs

```
GET /api/v1/logs/debug
```

Permission: `log:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| chainId | string | "" | Rule chain ID |
| nodeId | string | "" | Node ID |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):**

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

#### 11.4 WebSocket Real-time Debugging

```
WS /api/v1/logs/ws/:chainId/:clientId?token={jwtToken}
```

Authentication is required (passing JWT via query parameters `token`).

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| chainId | string | Rule chain ID |
| clientId | string | Client-side unique identifier |

After connection, the server pushes node debug data in real time. Each message format is:

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

| Field | Type | Note |
|---|---|---|
| chainId | string | Rule chain ID |
| flowType | string | Flow direction: `In` or `Out` |
| nodeId | string | Node ID |
| relationType | string | Relationship Type |
| err | string | Error message |
| msg | object | News content |
| ts | int64 | Timestamp (milliseconds) |

---

### 12. Internationalization

#### 12.1 Getting Language Lists / Language Packs

```
GET /api/v1/locales
```

Permission: `locale:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| lang | string | "" | Language codes (such as `en`, `zh_cn`) |

**Behavior:**

- Pass in `lang`: Returns the JSON object for the corresponding language pack.
- No `lang`: Returns a list of available language codes.

**Successful response (without lang):**

```json
["en", "zh_cn"]
```

#### 12.2 Saving Language Packs

```
POST /api/v1/locales?lang={lang}
```

Permission: `locale:write`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| lang | string | "en" | Language code |

**Request Body:** language package JSON object.

**Response:**

| Status code | Note |
|---|---|
| 200 | Saved successfully |
| 400 | Save failed |

---

### 13. Module market

#### 13.1 Obtaining the Market Component List

```
GET /api/v1/marketplace/components
```

Permission: `marketplace:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| keywords | string | "" | Filter by keyword |
| checkMy | bool | false | When set to `true`, the installed and upgradeable status is marked as |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):** Standard Pagination Format (`total` / `page` / `size` / `items`). When `checkMy=true`, the `ruleChain.additionalInfo` of each component in the `items` appends `installed` (bool) and `upgraded` (bool) fields.

#### 13.2 Obtain the list of market rule chains

```
GET /api/v1/marketplace/chains
```

Permission: `marketplace:read`

**Query parameters:**

| Parameter | Type | Default value | Note |
|---|---|---|---|
| keywords | string | "" | Filter by keyword |
| root | bool | - | Filter by root rule chain |
| page | int | 1 | Page number |
| size | int | 20 | Number per page |

**Successful Responses (200):** Standard Pagination Format (`total` / `page` / `size` / `items`).

---

### 14. MCP（Model Context Protocol）

MCP endpoints use independent API Key authentication (passed via path parameters, `Authorization` headers, or `X-API-Key` headers), without going through standard auth middleware.

Authentication priority: URL path parameters → `Authorization: Bearer` heads → `X-API-Key`.

#### 14.1 MCP StreamableHTTP endpoint

```
GET/POST/DELETE /api/v1/mcp/:apiKey
```

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| apiKey | string | User API Key |

Use MCP StreamableHTTP transport protocol. `POST` Send JSON-RPC requests, `GET` establish SSE event streams, `DELETE` close sessions.

**Error response:**

| Status code | Note |
|---|---|
| 401 | API Key Invalid |
| 500 | MCP Failed handling |

#### 14.2 MCP Grouping StreamableHTTP Endpoints

```
GET/POST/DELETE /api/v1/mcp/:apiKey/group/:group
```

**Path parameters:**

| Parameter | Type | Note |
|---|---|---|
| apiKey | string | User API Key |
| group | string | Tool group name |

Exposing a subset of MCP tools by group, with transport protocols consistent with 14.1.

**Error response:**

| Status code | Note |
|---|---|
| 400 | The group name is empty |
| 401 | API Key Invalid |
| 500 | MCP Failed handling |

---

### 15. Static resources

| Path | Note |
|---|---|
| `/editor/*` | Editor front-end static resource |
| `/images/*` | Image resources |

Path mapping is configured through `config.conf`'s `resource_mapping`.

---

## Configuration reference

Key configuration items affecting API behavior (`config.conf`):

| Configuration Item | Default value | Note |
|---|---|---|
| server | `:9090` | HTTP Listening address |
| base_path | "" | API Route prefix (used in embed mode) |
| require_auth | false | Whether authentication is required |
| default_username | `admin` | Default user when authentication is disabled |
| jwt_secret_key | (Built-in) | JWT Signature key (recommended to set via environment variable `JWT_SECRET_KEY`) |
| jwt_expire_time | `43200000` | JWT Expiration time (milliseconds, default 12 hours) |
| jwt_issuer | `rulego.cc` | JWT Issuer |
| resource_mapping | (Built-in) | Static file path mapping |
| allow_cors | true | Whether cross-origin |
| read_timeout | 30 | HTTP Read timeout (seconds) |
| write_timeout | 300 | HTTP Write timeout (seconds, AI chats require longer timeouts) |
| max_body_size | 10 | Maximum size of the request body (MB) |
| save_run_log | false | Do you want to save the runtime log? |
| run_log_store_type | `bbolt` | Runtime log storage type: `bbolt` or `file` (JSON Lines) |
| pprof.enable | `false` | Whether to enable pprof |
| pprof.addr | `0.0.0.0:6060` | pprof Service Address |
| mcp.enable | `true` | Whether to enable MCP service |
| marketplace_base_url | - | Component market remote address |
| skill_path | `./skills` | Skill file storage path |
