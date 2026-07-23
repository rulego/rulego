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

// getAuthenticator retrieves the authenticator from the container and returns the default implementation when not registered
func getAuthenticator(c *app.Container, cfg *config.Config) services.Authenticator {
	if auth, err := app.GetAs[services.Authenticator](c, services.KeyAuthenticator); err == nil {
		return auth
	}
	return user.NewDefaultAuthenticator(cfg)
}

// getAuthorizer obtains the authorizer from the Container; returns the default implementation when not registered
func getAuthorizer(c *app.Container) services.Authorizer {
	if authz, err := app.GetAs[services.Authorizer](c, services.KeyAuthorizer); err == nil {
		return authz
	}
	return user.NewDefaultAuthorizer()
}

const apiVersion = "v1"

// Server REST endpoint service, holding container and configuration references
type Server struct {
	container       *app.Container
	config          *config.Config
	systemRulegoCfg types.Config
	systemNodePool  *node_pool.NodePool
}

// NewServer creates REST endpoint services
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

// GetSystemRulegoConfig Obtains the system RuleGo configuration
func (s *Server) GetSystemRulegoConfig() types.Config {
	return s.systemRulegoCfg
}

// GetSystemNodePool Retrieves the system node pool
func (s *Server) GetSystemNodePool() *node_pool.NodePool {
	return s.systemNodePool
}

// Container returns a service container
func (s *Server) Container() *app.Container {
	return s.container
}

// SetContainer sets up the service container
func (s *Server) SetContainer(c *app.Container) {
	s.container = c
}

// basePath returns the base path prefix of the configuration
func (s *Server) basePath() string {
	if s.config != nil {
		return strings.TrimRight(s.config.BasePath, "/")
	}
	return ""
}

// apiBasePath returns the API base path
func (s *Server) apiBasePath() string {
	return s.basePath() + constants.PathApi + apiVersion
}

// NewRestEndpoint creates a REST endpoint and registers all routes
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

// NewStandardRestEndpoint explicitly creates a standard net/http REST endpoint without going through the registry
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

// initRestEndpoint initializes REST endpoint routing
func (s *Server) initRestEndpoint(ep endpointApi.HttpEndpoint) (endpointApi.HttpEndpoint, error) {
	// Global Interceptor: Set Content-Type + panic recovery
	ep.AddInterceptors(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		// Panic recovery, preventing a single request from panicking and causing the entire process to crash
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

	// Health checkup
	ep.GET(endpoint.NewRouter().From(s.basePath() + constants.PathHealth).Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		exchange.Out.SetBody([]byte("OK"))
		return false
	}).End())

	// Root path redirect
	ep.GET(endpoint.NewRouter().From(s.basePath() + "/").Process(func(router endpointApi.Router, exchange *endpointApi.Exchange) bool {
		r, ok1 := exchange.In.(*rest.RequestMessage)
		w, ok2 := exchange.Out.(*rest.ResponseMessage)
		if ok1 && ok2 {
			http.Redirect(w.Response(), r.Request(), s.basePath()+constants.PathEditor, http.StatusFound)
		}
		return false
	}).End())

	// Log in
	ep.POST(s.loginRoute())

	// Register routes for each module
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

	// Static resource mapping
	if s.config.ResourceMapping != "" {
		ep.RegisterStaticFiles(s.config.ResourceMapping)
	}

	// Set the default HTTP service to a shared node
	if s.config.ShareHttpServer {
		_, _ = node_pool.DefaultNodePool.AddNode(ep)
	}
	_, _ = s.systemNodePool.AddNode(ep)

	return ep, nil
}

// maxBodyBytes returns the maximum requested body byte number configured for the configuration
func (s *Server) maxBodyBytes() int64 {
	if s.config == nil || s.config.MaxBodySize <= 0 {
		return 10 << 20 // Default is 10MB
	}
	return int64(s.config.MaxBodySize) << 20
}

// validateId verifies the path parameter ID to prevent path traversal
func validateId(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	return !strings.ContainsAny(id, "/\\.")
}

// safeInternalError records the complete error in the log and returns a general message to the client
func (s *Server) safeInternalError(exchange *endpointApi.Exchange, err error, logger types.Logger) {
	if logger != nil {
		logger.Errorf("internal error: %v", err)
	}
	exchange.Out.SetStatusCode(http.StatusInternalServerError)
	exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
}

// formatError formatting error responses to hide internal details
func formatError(code int, publicMsg string) []byte {
	return []byte(fmt.Sprintf(`{"error":%q}`, publicMsg))
}
