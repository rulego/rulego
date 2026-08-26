// Package bridge 提供标准库 net/http 桥接层，允许宿主系统（如 Gin/Echo）嵌入 RuleGo 服务。
//
// 嵌入模式三条原则：
//  1. 不监听端口：所有流量经宿主 HTTP 服务器转发（Handler() 返回的路由器），
//     宿主与 rulego-server 共用一个端口；
//  2. 认证可托管：app.WithAuthenticator/WithAuthorizer 在模块 Init 前注入宿主身份体系，
//     配合 WithoutLocalAuth() 可完全关闭 rulego 自带的登录/用户管理端点；
//  3. 响应可包装：WithResponseWrapper 把 rulego 的裸 JSON 响应重组为宿主统一信封
//     （SSE/流式/二进制响应自动透传，不缓冲）。
//
// 嵌入式接入示例（Gin）：
//
//	package main
//
//	import (
//		"github.com/gin-gonic/gin"
//		"github.com/julienschmidt/httprouter"
//		"github.com/rulego/rulego/server/app"
//		"github.com/rulego/rulego/server/bridge"
//		"github.com/rulego/rulego/server/bootstrap"
//	)
//
//	func main() {
//		b, err := bridge.New(
//			bridge.WithAppOptions(
//				app.WithConfig(&cfg),                      // BasePath 设为 "/rulego"
//				app.WithModules(bootstrap.DefaultModules()...),
//				app.WithAuthenticator(&MyAuthenticator{}), // 宿主身份体系
//				app.WithAuthorizer(&MyAuthorizer{}),
//			),
//			bridge.WithoutLocalAuth(),                    // 关闭 rulego 自带 /login、/users*
//			bridge.WithResponseWrapper(myEnvelope),       // {code,message,data} 信封
//		)
//		if err != nil {
//			panic(err)
//		}
//		defer b.Stop()
//
//		r := gin.Default()
//		// 显式挂载到子路径（rulego 路由注册时已含 BasePath 前缀，勿 StripPrefix）
//		r.Any("/rulego/*path", gin.WrapH(b.Handler()))
//		r.Run(":8080")
//	}
package bridge

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/bootstrap"
	"github.com/rulego/rulego/server/config"
	srvEndpoint "github.com/rulego/rulego/server/internal/endpoint"
)

// ResponseWrapper 把 rulego-server 的 JSON 响应重组为宿主格式。
// 入参为上游 status 和响应体，返回（新响应体, 新 status；0 表示沿用原 status）。
// 仅对缓冲的 JSON 响应调用；SSE/流式/二进制/204/HEAD 请求不经此函数（直接透传）。
// ResponseWrapper rewrites rulego-server JSON responses into the host format.
type ResponseWrapper func(status int, body []byte) ([]byte, int)

// Bridge 桥接 RuleGo REST endpoint 到标准 net/http Handler。
// 允许宿主系统（如 Gin/Echo）通过标准 http.Handler 访问完整的 RuleGo API。
type Bridge struct {
	app     *app.App
	handler http.Handler
}

// Option bridge 构造选项
type Option func(*options)

type options struct {
	application *app.App
	appOpts     []app.Option
	respWrapper ResponseWrapper
	noLocalAuth bool
}

// WithApp 使用已构造（未 Init）的 app.App。与 WithAppOptions 二选一，同时提供时本选项优先。
// WithApp uses an already-constructed (not yet Init-ed) app.App.
func WithApp(application *app.App) Option {
	return func(o *options) { o.application = application }
}

// WithAppOptions 指定构造 app.App 的选项（未提供 WithApp 时生效）。
// WithAppOptions specifies options for constructing the app.App when WithApp is absent.
func WithAppOptions(opts ...app.Option) Option {
	return func(o *options) { o.appOpts = append(o.appOpts, opts...) }
}

// WithResponseWrapper 注入响应包装器，把 JSON 响应重组为宿主统一信封。
// 流式（SSE）与非 JSON 响应自动透传，不受影响。
// WithResponseWrapper injects a response wrapper that reshapes JSON responses
// into the host envelope. Streaming (SSE) and non-JSON responses pass through untouched.
func WithResponseWrapper(w ResponseWrapper) Option {
	return func(o *options) { o.respWrapper = w }
}

// WithoutLocalAuth 关闭 rulego-server 自身的登录/用户管理路由（/api/v1/login、/users*）。
// 嵌入模式下认证由宿主 Authenticator SPI 承担时使用，避免本地账号体系（含默认口令）暴露。
// WithoutLocalAuth disables rulego-server's own login/user-management routes.
func WithoutLocalAuth() Option {
	return func(o *options) { o.noLocalAuth = true }
}

// New 一站式构造：创建 App（或使用 WithApp 注入的实例）→ Init → 注册路由 → 启动模块，
// 返回可挂载到任意 HTTP 框架的 Handler。不监听任何端口。
// New is the one-stop constructor: creates the App (or uses the one from WithApp),
// runs Init, registers routes, starts modules, and returns a mountable Handler.
// It never listens on any port.
func New(opts ...Option) (*Bridge, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	application := o.application
	if application == nil {
		application = app.New(o.appOpts...)
	}

	app.RegisterDefaultStoresHook(application)

	if err := application.Init(); err != nil {
		return nil, err
	}

	cfg := application.Config()
	if cfg == nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	if o.noLocalAuth {
		cfg.DisableLocalAuth = true
	}

	typesLogger := application.Logger()
	_ = app.StartPprof(cfg, typesLogger) // 受 cfg.Pprof.Enable 控制；返回值仅在宿主需要主动关闭时使用

	srv := srvEndpoint.NewServer(application.Container(), cfg, typesLogger)
	// 使用标准 net/http 端点（不走注册表，确保不被 fasthttp 替换）。
	// 只注册路由，不调用 restEp.Start()：Start 会 ListenAndServe 绑定独立端口，
	// 嵌入模式的所有流量必须经宿主端口转发（原则 1）。
	restEp, err := srv.NewStandardRestEndpoint()
	if err != nil {
		return nil, err
	}

	if err := application.Start(); err != nil {
		return nil, err
	}

	router, ok := restEp.(interface{ Router() *httprouter.Router })
	if !ok || router.Router() == nil {
		return nil, ErrNoHandler
	}

	handler := http.Handler(router.Router())
	if o.respWrapper != nil {
		handler = wrapHandler(handler, o.respWrapper)
	}

	return &Bridge{app: application, handler: handler}, nil
}

// ErrNoHandler 表示 REST endpoint 未暴露可用的路由器，无法桥接。
var ErrNoHandler = errNoHandler{}

type errNoHandler struct{}

func (errNoHandler) Error() string { return "bridge: rest endpoint exposes no http router" }

// NewBridge 创建桥接器，接受已构造但未 Init 的 app.App。
// 等价于 New(WithApp(application))。新代码建议直接使用 New + 选项。
func NewBridge(application *app.App) (*Bridge, error) {
	return New(WithApp(application))
}

// NewBridgeWithDefaults 使用默认模块创建桥接器。
// 适用于快速接入场景，包含 user、rule、node 等全部默认模块。
func NewBridgeWithDefaults(configFile string) (*Bridge, error) {
	return New(WithAppOptions(
		app.WithConfigFile(configFile),
		app.WithModules(bootstrap.DefaultModules()...),
	))
}

// Handler 返回标准 http.Handler，可直接嵌入 Gin/Echo 等框架。
// 路由注册时已包含 config.BasePath 前缀：宿主挂载到同一路径时勿再 StripPrefix。
//
//	r.Any("/rulego/*path", gin.WrapH(bridge.Handler()))
func (b *Bridge) Handler() http.Handler {
	return b.handler
}

// App 返回底层的 app.App，用于生命周期管理与服务获取。
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
