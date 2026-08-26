# rulego-server 嵌入模式接入指南

rulego-server 可以作为库嵌入宿主应用（Gin/Echo/标准 net/http），与宿主共用一个 HTTP 端口，不监听任何独立端口。宿主通过官方选项注入存储、认证/授权和响应格式，无需在 Init 后偷改容器。

## 最小接入（Gin 宿主，约 10 行）

```go
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/bridge"
	"github.com/rulego/rulego/server/bootstrap"
	"github.com/rulego/rulego/server/config"
)

func main() {
	cfg := config.DefaultConfig()
	cfg.DataDir = "./data/rulego"
	cfg.BasePath = "/rulego" // 所有路由注册时带该前缀

	b, err := bridge.New(
		bridge.WithAppOptions(
			app.WithConfig(cfg),
			app.WithModules(bootstrap.DefaultModules()...),
		),
	)
	if err != nil {
		panic(err)
	}
	defer b.Stop()

	r := gin.Default()
	// 显式挂载：路由已含 BasePath 前缀，不要 StripPrefix
	r.Any("/rulego/*path", gin.WrapH(b.Handler()))
	r.Run(":8080")
}
```

嵌入模式三条原则：

1. **不监听端口**——`bridge.New` 只注册路由并提取 `http.Handler`，所有流量经宿主端口转发；
2. **认证可托管**——`app.WithAuthenticator/WithAuthorizer` 在模块 Init 前注入宿主身份体系；
3. **响应可包装**——`bridge.WithResponseWrapper` 把裸 JSON 重组为宿主信封，SSE/流式/二进制自动透传。

## 选项速查

### app 选项（构造 App）

| 选项 | 说明 |
|---|---|
| `app.WithConfig(cfg)` | 编程式配置（优先于 ConfigFile） |
| `app.WithConfigFile(path)` | 配置文件路径 |
| `app.WithModules(m...)` | 加载模块；`bootstrap.DefaultModules()` 为全量 |
| `app.WithStoreProvider(p)` | 自定义存储（如 gorm 数据库实现） |
| `app.WithAuthenticator(a)` | 宿主认证器：把宿主 JWT/会话翻译为 `*model.UserContext`。**UserContext.Username 是 store 层的数据分区键**（多租户场景映射为租户 ID），真实身份放 `Attrs` |
| `app.WithAuthorizer(a)` | 宿主授权器：把 `resource\|action` 映射为宿主权限点做 RBAC |
| `app.WithTypesLogger(l)` | 注入宿主日志框架 |
| `app.WithoutAutoMkdir()` | 宿主自管目录时禁用自动建目录 |

认证器/授权器经选项注入后，`user` 模块检测到容器已有实现会跳过自己的默认值（`RegisterIfAbsent` 语义）——不需要 `WithModuleOverride` 替换整个模块。

### bridge 选项（一站式构造）

| 选项 | 说明 |
|---|---|
| `bridge.WithApp(a)` | 使用已构造（未 Init）的 `*app.App` |
| `bridge.WithAppOptions(opts...)` | 未提供 App 时，内部按这些选项构造 |
| `bridge.WithoutLocalAuth()` | 关闭 rulego 自带 `/api/v1/login`、`/users*` 路由。认证完全由宿主 SPI 承担时使用，避免本地账号体系（含默认口令）暴露 |
| `bridge.WithResponseWrapper(w)` | 响应包装器 `func(status int, body []byte) ([]byte, int)`，见下文 |

## 响应包装（统一信封）

rulego-server 默认返回裸 JSON。宿主前端若有统一 `{code, message, data}` 信封约定，用 `WithResponseWrapper` 在服务端完成重组，前端无需适配层：

```go
func envelope(status int, body []byte) ([]byte, int) {
	var payload any
	if json.Unmarshal(bytes.TrimSpace(body), &payload) != nil {
		return body, status // 非 JSON（如 /health 的 "OK"）原样透传
	}
	if status >= 400 {
		var e struct{ Error string `json:"error"` }
		_ = json.Unmarshal(body, &e)
		out, _ := json.Marshal(map[string]any{"code": status, "message": e.Error})
		return out, status
	}
	out, _ := json.Marshal(map[string]any{"code": 200, "message": "success", "data": payload})
	return out, status
}
```

包装层对以下响应**自动跳过**（不缓冲、不改动）：

- SSE / 流式响应（`text/event-stream`，或上游调用 Flush）
- 非 JSON Content-Type（二进制下载等）
- 204/304 状态码、HEAD/OPTIONS 请求

## 多租户

store 层所有接口按第一参数 `username` 分区。嵌入模式的惯用法：

- `Authenticator` 把宿主 JWT 中的租户 ID 映射为 `UserContext.Username`（数据分区键），同租户用户共享链/日志/设置；
- 真实用户 ID 放 `UserContext.Attrs`，供 `Authorizer` 做按用户 的 RBAC；
- `username` 会被拼进文件存储路径，务必校验字符集（如 `^[a-zA-Z0-9_-]+$`）防路径穿越。

参考实现：gflow 的 `internal/bridge/auth.go`（GflowAuthenticator / GflowAuthorizer）。

## 嵌入 vs 独立部署

| 维度 | 嵌入（bridge.New） | 独立（app.Run / cmd/server） |
|---|---|---|
| 端口 | 与宿主共用 | 独立监听 |
| 认证 | 宿主 SPI 注入 | 自管账号（config Users / UserStore）+ 自签 JWT |
| 存储 | 通常注入宿主数据库（WithStoreProvider） | 默认文件存储，可配数据库 |
| 进程内调用 | 宿主可直接从 App 容器取 `ChainExecutor` 等 | 需走 HTTP（`/rules/:id/execute`、`/notify`） |
| 适用 | 单二进制交付、深度集成 | 多宿主共享、独立扩缩容 |

独立部署时若前端仍想挂子路径，设 `BasePath` 后用任意反向代理（nginx `location /rulego/`）转发即可，路由前缀语义与嵌入模式一致。
