// Package bootstrap 提供默认组装方案，将 App、Modules、Store、Transport 组合在一起。
// 公开包，外部可直接使用 DefaultModules() 获取默认模块列表，并通过 app.WithModuleOverride 替换特定模块。
package bootstrap

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/modules/locale"
	"github.com/rulego/rulego/server/internal/modules/marketplace"
	"github.com/rulego/rulego/server/internal/modules/mcp"
	"github.com/rulego/rulego/server/internal/modules/node"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/internal/modules/skill"
	"github.com/rulego/rulego/server/internal/modules/system"
	"github.com/rulego/rulego/server/internal/modules/user"
	srvEndpoint "github.com/rulego/rulego/server/internal/endpoint"
)

// DefaultModules 返回默认业务模块列表。
// 可与 app.WithModuleOverride 配合使用，替换特定模块。
func DefaultModules() []app.Module {
	return []app.Module{
		user.New(),
		rule.New(),
		node.New(),
		runlog.New(),
		locale.New(),
		skill.New(),
		system.New(),
		marketplace.New(),
		mcp.New(),
	}
}

// DefaultApp 创建一个使用默认配置的应用实例。
// 等价于 app.New(app.WithConfigFile(configFile), app.WithModules(bootstrap.DefaultModules()...))。
func DefaultApp(configFile string) *app.App {
	return app.New(
		app.WithConfigFile(configFile),
		app.WithModules(DefaultModules()...),
	)
}

// Run 初始化并启动应用，包括 endpoint 传输层，然后阻塞等待信号。
func Run(application *app.App) error {
	app.RegisterDefaultStoresHook(application)

	if err := application.Init(); err != nil {
		return err
	}

	cfg := application.Config()
	typesLogger := application.Logger()

	pprofSrv := app.StartPprof(cfg, typesLogger)

	srv := srvEndpoint.NewServer(application.Container(), cfg, typesLogger)

	restEp, err := srv.NewRestEndpoint()
	if err != nil {
		return err
	}

	// 创建 WebSocket 端点，共享 HTTP 服务
	wsEp, err := srv.NewWebsocketEndpoint(restEp)
	if err != nil {
		return err
	}

	if err := application.Start(); err != nil {
		return err
	}

	if err := restEp.Start(); err != nil {
		return err
	}
	typesLogger.Infof("RuleGo-Server started on %s", cfg.Server)
	// 启动 WebSocket 端点
	if err := wsEp.Start(); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	typesLogger.Infof("received signal: %v, shutting down...", sig)

	if pprofSrv != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = pprofSrv.Shutdown(shutdownCtx)
	}
	_ = application.Stop()
	return nil
}
