// Package bridge provides a standard library net/http bridge layer, allowing host systems (such as Gin/Echo) to embed RuleGo services.
//
// Embedded Access Example (Gin):
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
//		Method 1: Custom Modules (Recommended)
//		application, _ := app.New(
//			app.WithConfigFile("config.conf"),
//			app.WithModules(user.New(), rule.New()),
//		)
//		b, _ := bridge.NewBridge(application)
//
//		r := gin.Default()
//
//		User-made routes
//		r.GET("/api/users", userListHandler)
//
//		RuleGo routes are mounted under the /rulego prefix to avoid conflicts with host routes
//		r.Group("/rulego").Any("/*path", gin.WrapH(b.Handler()))
//
//		r.Run(":8080")
//	}
//
//	Method 2: Default module
//	b, _ := bridge.NewBridgeWithDefaults("config.conf")
//	handler := b.Handler()
package bridge

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/bootstrap"
	"github.com/rulego/rulego/server/config"
	srvEndpoint "github.com/rulego/rulego/server/internal/endpoint"
)

// Bridge bridges RuleGo REST endpoints to standard net/http handlers.
// Allows host systems (such as Gin/Echo) to pass through standard http.Handler provides access to the complete RuleGo API.
type Bridge struct {
	app     *app.App
	handler http.Handler
}

// NewBridge Create a bridge that accepts app.App that have been constructed but not yet Inited.
// It automatically registers storage hooks, initializes applications, and creates REST endpoints.
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

	// Use standard net/http endpoints (avoid the registry, ensuring it is not replaced by fasthttp)
	restEp, err := srv.NewStandardRestEndpoint()
	if err != nil {
		return nil, err
	}

	if err := application.Start(); err != nil {
		return nil, err
	}

	// Do NOT call restEp.Start(): rest.Rest.Start() binds cfg.Server and
	// serves, but an embedded host must own the only listener. Routes are
	// already registered by initRestEndpoint, and Router() carries the CORS
	// setup, so the router itself is the complete handler.
	type routerProvider interface {
		Router() *httprouter.Router
	}
	if rp, ok := restEp.(routerProvider); ok {
		if h := rp.Router(); h != nil {
			return &Bridge{app: application, handler: h}, nil
		}
	}

	return &Bridge{app: application}, nil
}

// NewBridgeWithDefaults creates a bridge using the default module.
// Suitable for fast access scenarios, including all default modules such as user, rule, node, etc.
func NewBridgeWithDefaults(configFile string) (*Bridge, error) {
	application := app.New(
		app.WithConfigFile(configFile),
		app.WithModules(bootstrap.DefaultModules()...),
	)
	return NewBridge(application)
}

// Handler returns the standard http.Handler, which can directly embed frameworks like Gin/Echo.
// The host can access http.StripPrefix removes the mount prefix:
//
//	r.Group("/rulego").Any("/*path", gin.WrapH(http.StripPrefix("/rulego", bridge.Handler())))
func (b *Bridge) Handler() http.Handler {
	return b.handler
}

// The app returns the underlying app.App for lifecycle management.
func (b *Bridge) App() *app.App {
	return b.app
}

// Stop the application and endpoint.
func (b *Bridge) Stop() error {
	if b.app != nil {
		return b.app.Stop()
	}
	return nil
}
