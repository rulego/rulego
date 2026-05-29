# RuleGo Server

[文档](https://rulego.cc/pages/rulego-server/) | [English](README.md)

基于 RuleGo 的应用开发脚手架，支持快速构建智能体、IoT、工作流等应用。

## 架构概览

RuleGo Server 采用分层模块化架构，每一层都可以自定义实现：

```
┌──────────────────────────────────────────────────┐
│  宿主应用（Gin / Echo / 独立运行）                │
├──────────────────────────────────────────────────┤
│  Bridge 桥接层（适配标准 http.Handler）           │
├──────────────────────────────────────────────────┤
│  App 生命周期 + Container 服务容器                │
│  ┌──────────┬──────────┬──────────┬───────────┐  │
│  │ Module   │ Module   │ Module   │  自定义    │  │
│  │ (rule)   │ (user)   │ (mcp)    │  Module   │  │
│  └──────────┴──────────┴──────────┴───────────┘  │
├──────────────────────────────────────────────────┤
│  Store 存储层（文件 / 数据库 / 自定义）           │
└──────────────────────────────────────────────────┘
```

核心扩展点：

- **模块（Module）**：实现 `app.Module` 接口，通过 `WithModules()` 注入或 `WithModuleOverride()` 替换
- **存储（Store）**：实现 `store.StoreProvider` 接口，通过 `WithStoreProvider()` 替换默认文件存储
- **认证/授权**：实现 `Authenticator` / `Authorizer` 接口，替换 JWT 和全放行默认实现
- **钩子（Hook）**：通过 `WithHooks()` 在 5 个生命周期阶段插入逻辑
- **组件**：通过 build tag 按需加载 AI、IoT 等组件包

## 目录结构

```
server/
├── cmd/server/                # 服务器入口（含组件注册 with_*.go）
├── app/                       # 公开：应用核心（App 生命周期、Container、Module 接口）
├── bootstrap/                 # 公开：默认模块组装
├── bridge/                    # 公开：宿主系统桥接层（Gin/Echo 等）
├── config/                    # 公开：配置管理
├── model/                     # 公开：纯数据模型
├── services/                  # 公开：模块导出的稳定服务接口
├── store/                     # 公开：存储接口（供自定义存储实现）
├── internal/                  # 内部实现（编译器保护，外部不可 import）
│   ├── modules/              #   业务模块实现
│   │   ├── rule/             #     规则链模块
│   │   ├── user/             #     用户模块
│   │   ├── node/             #     节点池模块
│   │   ├── runlog/           #     运行日志模块
│   │   ├── locale/           #     国际化模块
│   │   ├── skill/            #     技能模块
│   │   ├── debug/            #     调试模块
│   │   ├── system/           #     系统配置模块
│   │   ├── marketplace/      #     市场模块
│   │   └── mcp/              #     MCP 模块
│   ├── store/                #   存储实现
│   │   └── filestore/        #     文件存储实现
│   ├── endpoint/             #   REST endpoint 服务器
│   ├── engine/               #   规则引擎管理
│   ├── registry/             #   内建注册表
│   ├── constants/            #   常量与错误
│   └── utils/                #   工具函数
├── config.conf               # 默认配置
└── data/                     # 数据目录
```

## 快速开始

### 方式一：直接运行 server

```bash
cd rulego/server

# 基础版本
go run ./cmd/server

# 带 AI 组件
go build -tags with_ai ./cmd/server && ./server

# 带所有可选组件
go build -tags with_all ./cmd/server && ./server

# 带指定组件
go build -tags with_ai,with_iot ./cmd/server && ./server
```

### 方式二：引用包开发应用

```go
package main

import (
    "github.com/rulego/rulego/server/app"
    "github.com/rulego/rulego/server/bootstrap"

    // 导入需要的组件（参考 cmd/server/with_*.go）
    _ "github.com/rulego/rulego-components-ai/agent"
    _ "github.com/rulego/rulego-components-ai/tool/bash"
    // ... 其他需要的组件
)

func main() {
    application, _ := app.New(
        app.WithConfigFile("config.conf"),
        app.WithModules(bootstrap.DefaultModules()...),
    )
    application.Run()
}
```

### 方式三：嵌入式接入（Gin 示例）

通过 `bridge.Bridge` 将完整的 RuleGo REST API 桥接到宿主框架，无需手动注册路由。

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/rulego/rulego/server/app"
    "github.com/rulego/rulego/server/bootstrap"
    "github.com/rulego/rulego/server/bridge"
)

func main() {
    application, _ := app.New(
        app.WithConfigFile("config.conf"),
        app.WithModules(bootstrap.DefaultModules()...),
    )
    b, _ := bridge.NewBridge(application)
    defer b.Stop()

    r := gin.Default()

    // 用户自己的路由（优先匹配）
    r.GET("/api/users", userListHandler)

    // RuleGo 完整 API，未匹配的路由全部交给 Bridge 处理
    r.Any("/*path", gin.WrapH(b.Handler()))

    _ = r.Run(":8080")
}
```

使用默认模块的快捷方式：

```go
b, _ := bridge.NewBridgeWithDefaults("config.conf")
defer b.Stop()
handler := b.Handler() // 标准 http.Handler
```

## 可视化编辑器

RuleGo-Editor 是 RuleGo-Server 的可视化 UI 界面，可以对规则链进行可视化管理、调试和部署。

- 文档：[app.rulego.cc](https://app.rulego.cc)
- 编辑器演示：[editor.rulego.cc](https://editor.rulego.cc/)
- 完整演示（含服务端）：[http://8.134.32.225:9090/ui/](http://8.134.32.225:9090/ui/)

使用步骤：

- 从 [Release](https://github.com/rulego/rulego/releases) 下载 `editor.zip`，解压到 server 相同目录（会生成 `editor` 文件夹）
- 启动 server 后，打开浏览器访问 `http://localhost:9090/` 即可使用
- 通过 `config.conf` 的 `resource_mapping` 配置修改 editor 目录
- 通过 `editor/config/config.js` 的 `baseUrl` 配置修改后端 API 地址

## 公开 API 包

| 包 | 导入路径 | 用途 |
|----|---------|------|
| app | `github.com/rulego/rulego/server/app` | App 生命周期、Container、Module 接口 |
| bootstrap | `github.com/rulego/rulego/server/bootstrap` | 默认模块组装（DefaultModules） |
| bridge | `github.com/rulego/rulego/server/bridge` | 宿主系统桥接层（Gin/Echo 等） |
| config | `github.com/rulego/rulego/server/config` | Config 结构体、Load() |
| model | `github.com/rulego/rulego/server/model` | 纯数据模型 |
| services | `github.com/rulego/rulego/server/services` | 模块导出的稳定服务接口 |
| store | `github.com/rulego/rulego/server/store` | 存储接口（供自定义存储实现） |
| components | `github.com/rulego/rulego/server/cmd/server` | 组件注册（副作用 import，通过 build tag 启用） |

## 组件聚合包

通过 build tag 启用，具体导入参见 `cmd/server/with_*.go`：

| Build Tag | 包含内容 |
|-----------|---------|
| `with_all` | 所有可选组件（等同于同时启用下面全部标签） |
| `with_ai` | Agent、LLM、四原语工具等 |
| `with_iot` | OPC UA、Modbus、Serial 等 |
| `with_etl` | 数据转换组件 |
| `with_ci` | CI/CD 组件 |
| `with_extend` | Kafka、NATS、Redis、Lua 等 |

## 应用选项（Option）

`app.New()` 支持以下函数选项：

| Option | 说明 |
|--------|------|
| `WithConfigFile(path)` | 配置文件路径 |
| `WithModules(m...)` | 添加模块 |
| `WithModuleOverride(m)` | 按名称替换已注册的模块 |
| `WithStoreProvider(p)` | 注入自定义存储提供者（替换默认文件存储） |
| `WithHooks(h...)` | 添加生命周期钩子 |
| `WithGlobal(props)` | 注入全局配置，与配置文件 `[global]` 合并（注入值覆盖文件值） |
| `WithTypesLogger(l)` | 注入自定义日志器（Zap、Logrus 等） |
| `WithTransportDisabled()` | 禁用默认传输层（嵌入式模式） |
| `WithoutAutoMkdir()` | 禁用 Init 时自动创建数据目录 |

## 自定义开发

### 自定义模块

实现 `app.Module` 接口，通过 `WithModules()` 注入：

```go
type MyModule struct{}

func (m *MyModule) Name() string     { return "my_module" }
func (m *MyModule) Priority() int    { return 50 }
func (m *MyModule) Init(ctx *app.ModuleContext) error {
    // 注册服务到容器
    ctx.Container.Register("module.my_module.service", &MyService{})
    return nil
}
func (m *MyModule) Start(ctx context.Context) error { return nil }
func (m *MyModule) Stop(ctx context.Context) error  { return nil }

// 使用
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(append(bootstrap.DefaultModules(), &MyModule{})...),
)
```

用 `WithModuleOverride()` 替换内置模块（如替换默认的 rule 模块）：

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(bootstrap.DefaultModules()...),
    app.WithModuleOverride(&MyRuleModule{}),  // 替换 Name() == "rule" 的模块
)
```

### 自定义存储

实现 `store.StoreProvider` 接口，通过 `WithStoreProvider()` 注入：

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithStoreProvider(&MyDbStoreProvider{db: myDb}),
    app.WithModules(bootstrap.DefaultModules()...),
)
```

需要实现的接口：

| 接口 | 用途 |
|------|------|
| `RuleStore` | 规则链 CRUD |
| `UserStore` | 用户管理 |
| `SettingStore` | 用户设置 |
| `RunLogStore` | 运行日志 |
| `ComponentStore` | 组件定义 |
| `NodePoolStore` | 节点池 |
| `StoreProvider` | 工厂接口，按用户创建上述 Store |

### 自定义认证/授权

通过容器替换 `Authenticator` 或 `Authorizer`：

```go
// 在 Module.Init 中替换
func (m *MyModule) Init(ctx *app.ModuleContext) error {
    ctx.Container.Replace(services.KeyAuthenticator, &OAuth2Authenticator{})
    ctx.Container.Replace(services.KeyAuthorizer, &RBACAuthorizer{})
    return nil
}
```

### 生命周期钩子

通过 `WithHooks()` 在 App 生命周期的 5 个阶段插入逻辑：

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(bootstrap.DefaultModules()...),
    app.WithHooks(
        app.NewFuncHook("my_hook", app.AfterStart, 0,
            func(ctx context.Context, appCtx *app.ModuleContext) error {
                // 应用启动后的初始化逻辑
                return nil
            },
        ),
    ),
)
```

阶段：`BeforeInit` → `AfterInit` → `BeforeStart` → `AfterStart` → `OnStop`

## 模块服务接口

通过 Container 获取模块导出的稳定接口：

```go
// 规则链目录服务
catalog := app.MustGetAs[services.ChainCatalog](container, services.KeyRuleCatalog)

// 规则链执行器
executor := app.MustGetAs[services.ChainExecutor](container, services.KeyRuleExecutor)

// 规则链管理服务
admin := app.MustGetAs[services.RuleAdminService](container, services.KeyRuleManager)
```

完整服务列表：

| 容器键 | 接口 | 用途 |
|--------|------|------|
| `module.rule.catalog` | `ChainCatalog` | 规则链目录（只读） |
| `module.rule.executor` | `ChainExecutor` | 执行规则链 |
| `module.rule.manager` | `RuleAdminService` | 规则链管理（增删改部署） |
| `module.rule.engine_manager` | `EngineManager` | 多租户引擎池 |
| `module.node.service` | `NodeService` | 组件 + 节点池操作 |
| `module.runlog.service` | `RunLogService` | 运行日志 |
| `module.locale.service` | `LocaleService` | 国际化 |
| `module.marketplace.service` | `MarketplaceService` | 市场 |
| `module.mcp.service` | `McpService` | MCP 协议服务 |
| `module.system.settings` | `ConfigService` | 系统配置 |
| `module.user.auth` | `AuthService` | 密码/API Key 认证 |
| `module.user.profile` | `UserReader` | 用户信息读取 |
| `module.user.authenticator` | `Authenticator` | 身份认证（可替换，默认 JWT） |
| `module.user.authorizer` | `Authorizer` | 权限校验（可替换，默认全放行） |
| `module.skill.service` | `SkillService` | AI 技能管理 |
| `module.debug.service` | `DebugService` | 调试服务 |

## 配置说明

```ini
# 基础配置
data_dir = ./data
server = :9090
default_username = admin

# MCP 服务配置
[mcp]
enable = true
# 默认端点固定暴露管理 API 工具，组件和规则链通过分组配置暴露

# MCP 工具分组配置
[mcp.groups]
manager = preview_rule_chain,save_rule_chain,list_rule_chains,get_rule_chain,delete_rule_chain,operate_rule_chain,execute_rule_chain,list_components,get_component_doc

[global]
# 全局变量，规则链可通过 ${global.xxx} 引用
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```

## 通过自然语言管理规则链（MCP）

RuleGo Server 内置 MCP 服务，允许 AI 智能体通过自然语言生成、修改和管理规则链。详细配置和 AI IDE 集成请参阅 [MCP 文档](docs/mcp_zh.md)。

### MCP 工具列表

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

### MCP 工具类型

默认 MCP 端点固定暴露管理 API 工具（规则链 CRUD、组件查询等）。组件和规则链工具只通过分组配置暴露：

| 分组关键字 | 说明 |
|-----------|------|
| `rules` | 管理 API 工具（默认端点固定加载） |
| `components` | 将每个注册组件暴露为 MCP 工具，工具名为组件类型名，描述来自 `ComponentForm.Desc` |
| `chains` | 将每个已部署规则链暴露为 MCP 工具，工具名为规则链 ID，描述来自 `additionalInfo.description`，未设置时回退到 `Name` |

### 系统智能体

RuleGo Server 内置 `_assistant` 智能体，启动时自动部署到 `data/system/agents/_assistant/`（需通过 build tag 加载 AI 组件后才生效）。配置 LLM 后即可使用：

```ini
[global]
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```
