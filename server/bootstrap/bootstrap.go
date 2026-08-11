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
	srvEndpoint "github.com/rulego/rulego/server/internal/endpoint"
	"github.com/rulego/rulego/server/internal/engine"
	"github.com/rulego/rulego/server/services"
)

// DefaultModules 返回默认业务模块列表。
// 可与 app.WithModuleOverride 配合使用，替换特定模块。
func DefaultModules() []app.Module {
	return Modules(User, Rule, Node, RunLog, Locale, Skill, System, Marketplace, MCP, IoTPoint)
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

	// 开启 share_http_server 时，把主 HTTP 端点注入引擎管理器，
	// 使每个用户池可通过 ref://<config.Server> 复用主 HTTP server。
	if cfg.ShareHttpServer {
		if mgr, err := app.GetAs[services.EngineManager](application.Container(), services.KeyEngineManager); err == nil {
			if concrete, ok := mgr.(*engine.Manager); ok {
				concrete.SetSystemEndpoint(restEp)
			}
		}
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
