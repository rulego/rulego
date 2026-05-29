// Package bridge 提供标准库 net/http 桥接层，允许宿主系统（如 Gin/Echo）嵌入 RuleGo 服务。
//
// 嵌入式接入示例（Gin）：
//
//	package main
//
//	import (
//		"github.com/gin-gonic/gin"
//		"github.com/rulego/rulego/server/app"
//		"github.com/rulego/rulego/server/internal/modules/rule"
//		"github.com/rulego/rulego/server/internal/modules/user"
//		"github.com/rulego/rulego/server/bridge"
//	)
//
//	func main() {
//		// 方式一：自定义模块（推荐）
//		application, _ := app.New(
//			app.WithConfigFile("config.conf"),
//			app.WithModules(user.New(), rule.New()),
//		)
//		b, _ := bridge.NewBridge(application)
//
//		r := gin.Default()
//
//		// 用户自己的路由
//		r.GET("/api/users", userListHandler)
//
//		// RuleGo 路由挂载到 /rulego 前缀下，避免与宿主路由冲突
//		r.Group("/rulego").Any("/*path", gin.WrapH(b.Handler()))
//
//		r.Run(":8080")
//	}
//
//	// 方式二：默认模块
//	b, _ := bridge.NewBridgeWithDefaults("config.conf")
//	handler := b.Handler()
package bridge

import (
	"net/http"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/bootstrap"
	"github.com/rulego/rulego/server/config"
	srvEndpoint "github.com/rulego/rulego/server/internal/endpoint"
)

// Bridge 桥接 RuleGo REST endpoint 到标准 net/http Handler。
// 允许宿主系统（如 Gin/Echo）通过标准 http.Handler 访问完整的 RuleGo API。
type Bridge struct {
	app     *app.App
	handler http.Handler
}

// NewBridge 创建桥接器，接受已构造但未 Init 的 app.App。
// 会自动注册存储钩子、初始化应用、创建 REST endpoint。
func NewBridge(application *app.App) (*Bridge, error) {
	app.RegisterDefaultStoresHook(application)

	if err := application.Init(); err != nil {
		return nil, err
	}

	cfg := application.Config()
	if cfg == nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	typesLogger := application.Logger()

	pprofSrv := app.StartPprof(cfg, typesLogger)
	_ = pprofSrv

	srv := srvEndpoint.NewServer(application.Container(), cfg, typesLogger)
	_ = srv

	// 使用标准 net/http 端点（不走注册表，确保不被 fasthttp 替换）
	restEp, err := srv.NewStandardRestEndpoint()
	if err != nil {
		return nil, err
	}

	if err := application.Start(); err != nil {
		return nil, err
	}

	// 启动 endpoint（注册路由但不监听端口）
	if err := restEp.Start(); err != nil {
		return nil, err
	}

	// 获取底层 http.Server 的 Handler
	type serverProvider interface {
		GetServer() *http.Server
	}
	if sp, ok := restEp.(serverProvider); ok {
		if h := sp.GetServer().Handler; h != nil {
			return &Bridge{app: application, handler: h}, nil
		}
	}

	return &Bridge{app: application}, nil
}

// NewBridgeWithDefaults 使用默认模块创建桥接器。
// 适用于快速接入场景，包含 user、rule、node 等全部默认模块。
func NewBridgeWithDefaults(configFile string) (*Bridge, error) {
	application := app.New(
		app.WithConfigFile(configFile),
		app.WithModules(bootstrap.DefaultModules()...),
	)
	return NewBridge(application)
}

// Handler 返回标准 http.Handler，可直接嵌入 Gin/Echo 等框架。
// 宿主可通过 http.StripPrefix 去掉挂载前缀：
//
//	r.Group("/rulego").Any("/*path", gin.WrapH(http.StripPrefix("/rulego", bridge.Handler())))
func (b *Bridge) Handler() http.Handler {
	return b.handler
}

// App 返回底层的 app.App，用于生命周期管理。
func (b *Bridge) App() *app.App {
	return b.app
}

// Stop 停止应用和 endpoint。
func (b *Bridge) Stop() error {
	if b.app != nil {
		return b.app.Stop()
	}
	return nil
}
