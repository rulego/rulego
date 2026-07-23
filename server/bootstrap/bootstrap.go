// Package bootstrap provides a default assembly plan, combining App, Modules, Store, and Transport together.
// Public packages. Externally, you can directly use DefaultModules() to obtain the default module list, and through the app.WithModuleOverride replaces a specific module.
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
	"github.com/rulego/rulego/server/internal/modules/locale"
	"github.com/rulego/rulego/server/internal/modules/marketplace"
	"github.com/rulego/rulego/server/internal/modules/mcp"
	"github.com/rulego/rulego/server/internal/modules/node"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/internal/modules/skill"
	"github.com/rulego/rulego/server/internal/modules/system"
	"github.com/rulego/rulego/server/internal/modules/user"
	"github.com/rulego/rulego/server/services"
)

// DefaultModules returns a list of default business modules.
// Can be connected with the app.WithModuleOverride is used together to replace specific modules.
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

// DefaultApp creates an application instance using the default configuration.
// Equivalent to an app.New(app.WithConfigFile(configFile), app.WithModules(bootstrap.DefaultModules()...)).
func DefaultApp(configFile string) *app.App {
	return app.New(
		app.WithConfigFile(configFile),
		app.WithModules(DefaultModules()...),
	)
}

// Run initializes and starts the application, including the endpoint transport layer, then blocks the waiting signal.
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

	// Create WebSocket endpoints to share HTTP services
	wsEp, err := srv.NewWebsocketEndpoint(restEp)
	if err != nil {
		return err
	}

	// When share_http_server is enabled, the main HTTP endpoint is injected into the engine manager,
	// Allows each user pool to reuse the main HTTP server via ref://<config.Server>
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
	// Start the WebSocket endpoint
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
