# RuleGo Server

[Docs](https://rulego.cc/en/pages/rulego-server/) | [中文](README_ZH.md)

An application development scaffold based on RuleGo, for rapidly building AI agent, IoT, workflow and other applications.

## Architecture Overview

RuleGo Server uses a layered modular architecture where every layer can be customized:

```
┌──────────────────────────────────────────────────┐
│  Host Application (Gin / Echo / Standalone)      │
├──────────────────────────────────────────────────┤
│  Bridge Layer (adapts to standard http.Handler)   │
├──────────────────────────────────────────────────┤
│  App Lifecycle + Container (Service Container)    │
│  ┌──────────┬──────────┬──────────┬───────────┐  │
│  │ Module   │ Module   │ Module   │  Custom    │  │
│  │ (rule)   │ (user)   │ (mcp)    │  Module   │  │
│  └──────────┴──────────┴──────────┴───────────┘  │
├──────────────────────────────────────────────────┤
│  Store Layer (File / Database / Custom)           │
└──────────────────────────────────────────────────┘
```

Key extension points:

- **Modules**: Implement the `app.Module` interface, inject via `WithModules()` or replace via `WithModuleOverride()`
- **Storage**: Implement the `store.StoreProvider` interface, replace default file storage via `WithStoreProvider()`
- **Auth**: Implement `Authenticator` / `Authorizer` interfaces to replace JWT and allow-all defaults
- **Hooks**: Insert logic at 5 lifecycle phases via `WithHooks()`
- **Components**: Load AI, IoT and other component packages on demand via build tags

## Directory Structure

```
server/
├── cmd/server/                # Server entry point (with component registration via with_*.go)
├── app/                       # Public: App core (lifecycle, Container, Module interface)
├── bootstrap/                 # Public: Default module assembly
├── bridge/                    # Public: Host system bridge layer (Gin/Echo etc.)
├── config/                    # Public: Configuration management
├── model/                     # Public: Pure data models
├── services/                  # Public: Stable service interfaces exported by modules
├── store/                     # Public: Storage interfaces (for custom storage implementations)
├── internal/                  # Internal implementation (compile-time protected, not importable)
│   ├── modules/              #   Business module implementations
│   │   ├── rule/             #     Rule chain module
│   │   ├── user/             #     User module
│   │   ├── node/             #     Node pool module
│   │   ├── runlog/           #     Run log module
│   │   ├── locale/           #     i18n module
│   │   ├── skill/            #     Skill module
│   │   ├── debug/            #     Debug module
│   │   ├── system/           #     System config module
│   │   ├── marketplace/      #     Marketplace module
│   │   └── mcp/              #     MCP module
│   ├── store/                #   Storage implementations
│   │   └── filestore/        #     File-based storage
│   ├── endpoint/             #   REST endpoint server
│   ├── engine/               #   Rule engine management
│   ├── registry/             #   Built-in registry
│   ├── constants/            #   Constants and errors
│   └── utils/                #   Utility functions
├── config.conf               # Default configuration
└── data/                     # Data directory
```

## Quick Start

### Option 1: Run the server directly

```bash
cd rulego/server

# Basic version
go run ./cmd/server

# With AI components
go build -tags with_ai ./cmd/server && ./server

# With all optional components
go build -tags with_all ./cmd/server && ./server

# With specific components
go build -tags with_ai,with_iot ./cmd/server && ./server
```

### Option 2: Import as a package

```go
package main

import (
    "github.com/rulego/rulego/server/app"
    "github.com/rulego/rulego/server/bootstrap"

    // Import components as needed (see cmd/server/with_*.go)
    _ "github.com/rulego/rulego-components-ai/agent"
    _ "github.com/rulego/rulego-components-ai/tool/bash"
    // ... other components
)

func main() {
    application, _ := app.New(
        app.WithConfigFile("config.conf"),
        app.WithModules(bootstrap.DefaultModules()...),
    )
    application.Run()
}
```

### Option 3: Embedded integration (Gin example)

Use `bridge.Bridge` to bridge the full RuleGo REST API into your host framework — no manual route registration needed.

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

    // Your own routes (matched first)
    r.GET("/api/users", userListHandler)

    // Full RuleGo API — unmatched routes fall through to the Bridge handler
    r.Any("/*path", gin.WrapH(b.Handler()))

    _ = r.Run(":8080")
}
```

Shortcut with default modules:

```go
b, _ := bridge.NewBridgeWithDefaults("config.conf")
defer b.Stop()
handler := b.Handler() // standard http.Handler
```

## Visual Editor

RuleGo-Editor is the visual UI for RuleGo-Server, allowing visual management, debugging and deployment of rule chains.

- Documentation: [app.rulego.cc](https://app.rulego.cc)
- Editor Demo: [editor.rulego.cc](https://editor.rulego.cc/)
- Full Demo (with server): [http://8.134.32.225:9090/ui/](http://8.134.32.225:9090/ui/)

Usage:

- Download `editor.zip` from [Release](https://github.com/rulego/rulego/releases), extract to the same directory as the server (creates an `editor` folder)
- Start the server, then open `http://localhost:9090/` in your browser
- Customize the editor directory via `resource_mapping` in `config.conf`
- Customize the backend API address via `baseUrl` in `editor/config/config.js`

## Public API Packages

| Package | Import Path | Purpose |
|---------|------------|---------|
| app | `github.com/rulego/rulego/server/app` | App lifecycle, Container, Module interface |
| bootstrap | `github.com/rulego/rulego/server/bootstrap` | Default module assembly (DefaultModules) |
| bridge | `github.com/rulego/rulego/server/bridge` | Host system bridge layer (Gin/Echo etc.) |
| config | `github.com/rulego/rulego/server/config` | Config struct, Load() |
| model | `github.com/rulego/rulego/server/model` | Pure data models |
| services | `github.com/rulego/rulego/server/services` | Stable service interfaces exported by modules |
| store | `github.com/rulego/rulego/server/store` | Storage interfaces |
| components | `github.com/rulego/rulego/server/cmd/server` | Component registration (side-effect import, enabled via build tag) |

## Component Aggregation Packages

Enabled via build tags. See `cmd/server/with_*.go` for specific imports:

| Build Tag | Included Components |
|-----------|---------------------|
| `with_all` | All optional components (equivalent to enabling all tags below) |
| `with_ai` | Agent, LLM, four-primitive tools, etc. |
| `with_iot` | OPC UA, Modbus, Serial, etc. |
| `with_etl` | Data transformation components |
| `with_ci` | CI/CD components |
| `with_extend` | Kafka, NATS, Redis, Lua, etc. |

## Application Options

`app.New()` supports the following functional options:

| Option | Description |
|--------|-------------|
| `WithConfigFile(path)` | Configuration file path |
| `WithModules(m...)` | Add modules |
| `WithModuleOverride(m)` | Replace a registered module by name |
| `WithStoreProvider(p)` | Inject a custom store provider (replaces default file storage) |
| `WithHooks(h...)` | Add lifecycle hooks |
| `WithGlobal(props)` | Inject global config, merged with `[global]` section from config file (injected values override file values) |
| `WithTypesLogger(l)` | Inject a custom logger (Zap, Logrus, etc.) |
| `WithTransportDisabled()` | Disable default transport layer (embedded mode) |
| `WithoutAutoMkdir()` | Disable auto-creation of data directories during Init |

## Custom Development

### Custom Module

Implement the `app.Module` interface and inject via `WithModules()`:

```go
type MyModule struct{}

func (m *MyModule) Name() string     { return "my_module" }
func (m *MyModule) Priority() int    { return 50 }
func (m *MyModule) Init(ctx *app.ModuleContext) error {
    // Register services into the container
    ctx.Container.Register("module.my_module.service", &MyService{})
    return nil
}
func (m *MyModule) Start(ctx context.Context) error { return nil }
func (m *MyModule) Stop(ctx context.Context) error  { return nil }

// Usage
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(append(bootstrap.DefaultModules(), &MyModule{})...),
)
```

Replace a built-in module using `WithModuleOverride()` (e.g., replace the default rule module):

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(bootstrap.DefaultModules()...),
    app.WithModuleOverride(&MyRuleModule{}),  // replaces module with Name() == "rule"
)
```

### Custom Storage

Implement the `store.StoreProvider` interface and inject via `WithStoreProvider()`:

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithStoreProvider(&MyDbStoreProvider{db: myDb}),
    app.WithModules(bootstrap.DefaultModules()...),
)
```

Interfaces to implement:

| Interface | Purpose |
|-----------|---------|
| `RuleStore` | Rule chain CRUD |
| `UserStore` | User management |
| `SettingStore` | User settings |
| `RunLogStore` | Run logs |
| `ComponentStore` | Component definitions |
| `NodePoolStore` | Node pool |
| `StoreProvider` | Factory interface, creates per-user Store instances |

### Custom Auth

Replace `Authenticator` or `Authorizer` via the container:

```go
// Replace in Module.Init
func (m *MyModule) Init(ctx *app.ModuleContext) error {
    ctx.Container.Replace(services.KeyAuthenticator, &OAuth2Authenticator{})
    ctx.Container.Replace(services.KeyAuthorizer, &RBACAuthorizer{})
    return nil
}
```

### Lifecycle Hooks

Insert logic at 5 lifecycle phases via `WithHooks()`:

```go
application, _ := app.New(
    app.WithConfigFile("config.conf"),
    app.WithModules(bootstrap.DefaultModules()...),
    app.WithHooks(
        app.NewFuncHook("my_hook", app.AfterStart, 0,
            func(ctx context.Context, appCtx *app.ModuleContext) error {
                // Post-start initialization logic
                return nil
            },
        ),
    ),
)
```

Phases: `BeforeInit` → `AfterInit` → `BeforeStart` → `AfterStart` → `OnStop`

## Module Service Interfaces

Access stable interfaces exported by modules through the Container:

```go
// Rule chain catalog service
catalog := app.MustGetAs[services.ChainCatalog](container, services.KeyRuleCatalog)

// Rule chain executor
executor := app.MustGetAs[services.ChainExecutor](container, services.KeyRuleExecutor)

// Rule chain admin service
admin := app.MustGetAs[services.RuleAdminService](container, services.KeyRuleManager)
```

Full service list:

| Container Key | Interface | Purpose |
|---------------|-----------|---------|
| `module.rule.catalog` | `ChainCatalog` | Rule chain catalog (read-only) |
| `module.rule.executor` | `ChainExecutor` | Execute rule chains |
| `module.rule.manager` | `RuleAdminService` | Rule chain admin (CRUD + deploy) |
| `module.rule.engine_manager` | `EngineManager` | Multi-tenant engine pool |
| `module.node.service` | `NodeService` | Component + node pool operations |
| `module.runlog.service` | `RunLogService` | Run logs |
| `module.locale.service` | `LocaleService` | Internationalization |
| `module.marketplace.service` | `MarketplaceService` | Marketplace |
| `module.mcp.service` | `McpService` | MCP protocol service |
| `module.system.settings` | `ConfigService` | System configuration |
| `module.user.auth` | `AuthService` | Password/API Key authentication |
| `module.user.profile` | `UserReader` | User info reading |
| `module.user.authenticator` | `Authenticator` | Identity authentication (replaceable, default JWT) |
| `module.user.authorizer` | `Authorizer` | Authorization (replaceable, default allow-all) |
| `module.skill.service` | `SkillService` | AI skill management |
| `module.debug.service` | `DebugService` | Debug service |

## Configuration

```ini
# Basic configuration
data_dir = ./data
server = :9090
default_username = admin

# MCP service configuration
[mcp]
enable = true
# Default endpoint always exposes management API tools; components and chains via groups only

# MCP tool group configuration
[mcp.groups]
manager = preview_rule_chain,save_rule_chain,list_rule_chains,get_rule_chain,delete_rule_chain,operate_rule_chain,execute_rule_chain,list_components,get_component_doc

[global]
# Global variables, accessible in rule chains via ${global.xxx}
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```

## Managing Rule Chains via Natural Language (MCP)

RuleGo Server includes a built-in MCP (Model Context Protocol) service that allows AI agents to generate, modify, and manage rule chains through natural language. For detailed configuration and AI IDE integration, see the [MCP documentation](docs/mcp_en.md).

### MCP Tool List

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

### MCP Tool Types

The default MCP endpoint always exposes management API tools (rule chain CRUD, component query, etc.). Components and rule chains are exposed only through group configuration:

| Group keyword | Description |
|---------------|-------------|
| `rules` | Management API tools (always loaded in default endpoint) |
| `components` | Expose each registered component as an MCP tool; tool name is component type, description from `ComponentForm.Desc` |
| `chains` | Expose each deployed rule chain as an MCP tool; tool name is chain ID, description from `additionalInfo.description`, falls back to `Name` |

### Built-in Agent

RuleGo Server includes a built-in `_assistant` agent, automatically deployed to `data/system/agents/_assistant/` at startup (requires AI components loaded via build tag). Enable it by configuring an LLM:

```ini
[global]
llm_url = https://api.openai.com/v1
llm_api_key = ${OPENAI_API_KEY}
llm_model = gpt-4
```
