package user

import (
	"context"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

const (
	ModuleName = "user"
	Priority   = 10
)

// Module user: Business module, responsible for user authentication and user management.
type Module struct {
	cfg       *config.Config
	userStore store.UserStore
	authSvc   *authService
}

// New creates the user module
func New() *Module {
	return &Module{}
}

// Name returns the module name
func (m *Module) Name() string { return ModuleName }

// Priority: Returns the module's priority
func (m *Module) Priority() int { return Priority }

// Init initializes the module to obtain configuration and storage services from the container
func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config

	userStore, err := app.GetAs[store.UserStore](ctx.Container, "store.user")
	if err != nil {
		return err
	}
	m.userStore = userStore

	m.authSvc = &authService{cfg: m.cfg}
	if err := ctx.Container.Register(services.KeyAuthService, services.AuthService(m.authSvc)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyUserProfile, services.UserReader(m)); err != nil {
		return err
	}

	// Register default authenticators and authorizers; embedded mode can replace the entire user module with WithModuleOverride
	if err := ctx.Container.Register(services.KeyAuthenticator, services.Authenticator(NewDefaultAuthenticator(m.cfg))); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyAuthorizer, services.Authorizer(NewDefaultAuthorizer())); err != nil {
		return err
	}

	return nil
}

// Start Module (No background tasks)
func (m *Module) Start(_ context.Context) error { return nil }

// Stop the module
func (m *Module) Stop(_ context.Context) error { return nil }

// List returns a list of users
func (m *Module) List() []model.User {
	return m.userStore.List()
}

// authService authentication service implementation
type authService struct {
	cfg *config.Config
}

// CheckPassword verifies username and password
func (s *authService) CheckPassword(username, password string) bool {
	if username == "" {
		return false
	}
	return s.cfg.CheckPassword(username, password)
}

// GetUsernameByApiKey obtains the username through the API Key
func (s *authService) GetUsernameByApiKey(apikey string) string {
	if apikey == "" {
		return ""
	}
	return s.cfg.GetUsernameByApiKey(apikey)
}

// GetApiKeyByUsername Obtains the API Key through the username
func (s *authService) GetApiKeyByUsername(username string) string {
	if username == "" {
		return ""
	}
	return s.cfg.GetApiKeyByUsername(username)
}
