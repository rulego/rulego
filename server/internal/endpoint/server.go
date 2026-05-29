package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/user"
	"github.com/rulego/rulego/server/services"
)

// getAuthenticator 从 Container 获取认证器，未注册时返回默认实现
func getAuthenticator(c *app.Container, cfg *config.Config) services.Authenticator {
	if auth, err := app.GetAs[services.Authenticator](c, services.KeyAuthenticator); err == nil {
		return auth
	}
	return user.NewDefaultAuthenticator(cfg)
}

// getAuthorizer 从 Container 获取授权器，未注册时返回默认实现
func getAuthorizer(c *app.Container) services.Authorizer {
	if authz, err := app.GetAs[services.Authorizer](c, services.KeyAuthorizer); err == nil {
		return authz
	}
	return user.NewDefaultAuthorizer()
}

const apiVersion = "v1"

// Server REST 端点服务，持有容器和配置引用
type Server struct {
	container       *app.Container
	config          *config.Config
	systemRulegoCfg types.Config
	systemNodePool  *node_pool.NodePool
}

// NewServer 创建 REST 端点服务
func NewServer(container *app.Container, cfg *config.Config, logger types.Logger) *Server {
	systemRulegoCfg := rulego.NewConfig(types.WithDefaultPool(), types.WithLogger(logger))
	systemNodePool := node_pool.NewNodePool(systemRulegoCfg)
	systemRulegoCfg.NodePool = systemNodePool

	return &Server{
		container:       container,
		config:          cfg,
		systemRulegoCfg: systemRulegoCfg,
		systemNodePool:  systemNodePool,
	}
}

// GetSystemRulegoConfig 获取系统 RuleGo 配置
func (s *Server) GetSystemRulegoConfig() types.Config {
	return s.systemRulegoCfg
}

// GetSystemNodePool 获取系统节点池
func (s *Server) GetSystemNodePool() *node_pool.NodePool {
	return s.systemNodePool
}

// Container 返回服务容器
func (s *Server) Container() *app.Container {
	return s.container
}

// SetContainer 设置服务容器
func (s *Server) SetContainer(c *app.Container) {
	s.container = c
}

// basePath 返回配置的基础路径前缀
func (s *Server) basePath() string {
	if s.config != nil {
		return strings.TrimRight(s.config.BasePath, "/")
	}
	return ""
}

// apiBasePath 返回 API 基础路径
func (s *Server) apiBasePath() string {
	return s.basePath() + constants.PathApi + apiVersion
}

// NewRestEndpoint 创建 REST 端点并注册所有路由
func (s *Server) NewRestEndpoint() (endpointApi.HttpEndpoint, error) {
	ep, err := endpoint.Registry.New(
		rest.Type,
		s.systemRulegoCfg,
		rest.Config{
			Server:       s.config.Server,
			AllowCors:    s.config.AllowCors,
			ReadTimeout:  s.config.ReadTimeout,
			WriteTimeout: s.config.WriteTimeout,
		},
	)
	if err != nil {
		return nil, err
	}

	restEndpoint, ok := ep.(endpointApi.HttpEndpoint)
	if !ok {
		return nil, errors.New("is not HttpEndpoint type error")
	}

	return s.initRestEndpoint(restEndpoint)
}

// NewStandardRestEndpoint 显式创建标准 net/http REST 端点，不走注册表
func (s *Server) NewStandardRestEndpoint() (endpointApi.HttpEndpoint, error) {
	ep := &rest.Rest{}
	if err := ep.Init(s.systemRulegoCfg, types.Configuration{
		"server":       s.config.Server,
		"allowCors":    s.config.AllowCors,
		"readTimeout":  s.config.ReadTimeout,
		"writeTimeout": s.config.WriteTimeout,
	}); err != nil {
		return nil, err
	}

	return s.initRestEndpoint(ep)
}

// initRestEndpoint 初始化 REST 端点路由
func (s *Server) initRestEndpoint(ep endpointApi.HttpEndpoint) (endpointApi.HttpEndpoint, error) {
	// 全局拦截器：设置 Content-Type + panic recovery
	ep.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		// panic recovery，防止单个请求 panic 导致整个进程崩溃
		defer func() {
			if r := recover(); r != nil {
				exchange.Out.SetStatusCode(http.StatusInternalServerError)
				exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
			}
		}()

		if out, ok := exchange.Out.(endpointApi.HeaderModifier); ok {
			out.AddHeader("Content-Type", "application/json")
		} else {
			exchange.Out.Headers().Set("Content-Type", "application/json")
		}
		return true
	})

	// 健康检查
	ep.GET(endpoint.NewRouter().From(s.basePath() + constants.PathHealth).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.SetBody([]byte("OK"))
		return false
	}).End())

	// 根路径重定向
	ep.GET(endpoint.NewRouter().From(s.basePath() + "/").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			http.Redirect(w.Response(), r.Request(), s.basePath()+constants.PathEditor, http.StatusFound)
		}
		return false
	}).End())

	// 登录
	ep.POST(s.loginRoute())

	// 注册各模块路由
	s.registerRuleRoutes(ep)
	s.registerNodeRoutes(ep)
	s.registerComponentRoutes(ep)
	s.registerConfigRoutes(ep)
	s.registerAIRoutes(ep)
	s.registerSkillRoutes(ep)
	s.registerLogRoutes(ep)
	s.registerLocaleRoutes(ep)
	s.registerMarketplaceRoutes(ep)
	s.registerMCPRoutes(ep)

	// 静态资源映射
	if s.config.ResourceMapping != "" {
		ep.RegisterStaticFiles(s.config.ResourceMapping)
	}

	// 把默认HTTP服务设置成共享节点
	if s.config.ShareHttpServer {
		_, _ = node_pool.DefaultNodePool.AddNode(ep)
	}
	_, _ = s.systemNodePool.AddNode(ep)

	return ep, nil
}

// maxBodyBytes 返回配置的最大请求体字节数
func (s *Server) maxBodyBytes() int64 {
	if s.config == nil || s.config.MaxBodySize <= 0 {
		return 10 << 20 // 默认 10MB
	}
	return int64(s.config.MaxBodySize) << 20
}

// validateId 校验路径参数 ID，防止路径遍历
func validateId(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	return !strings.ContainsAny(id, "/\\.")
}

// safeInternalError 记录完整错误到日志，返回通用消息给客户端
func (s *Server) safeInternalError(exchange *endpointApi.Exchange, err error, logger types.Logger) {
	if logger != nil {
		logger.Errorf("internal error: %v", err)
	}
	exchange.Out.SetStatusCode(http.StatusInternalServerError)
	exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
}

// formatError 格式化错误响应，隐藏内部细节
func formatError(code int, publicMsg string) []byte {
	return []byte(fmt.Sprintf(`{"error":%q}`, publicMsg))
}
