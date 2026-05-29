package user

import (
	"context"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

const (
	ModuleName = "user"
	Priority   = 10
)

// Module user 业务模块，负责用户认证和用户管理。
type Module struct {
	cfg       *config.Config
	userStore store.UserStore
	authSvc   *authService
}

// New 创建 user 模块
func New() *Module {
	return &Module{}
}

// Name 返回模块名称
func (m *Module) Name() string { return ModuleName }

// Priority 返回模块优先级
func (m *Module) Priority() int { return Priority }

// Init 初始化模块，从 Container 获取配置和存储服务
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

	// 注册默认认证器和授权器，嵌入模式可通过 WithModuleOverride 替换整个 user 模块
	if err := ctx.Container.Register(services.KeyAuthenticator, services.Authenticator(NewDefaultAuthenticator(m.cfg))); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyAuthorizer, services.Authorizer(NewDefaultAuthorizer())); err != nil {
		return err
	}

	return nil
}

// Start 启动模块（无后台任务）
func (m *Module) Start(_ context.Context) error { return nil }

// Stop 停止模块
func (m *Module) Stop(_ context.Context) error { return nil }

// List 返回用户列表
func (m *Module) List() []model.User {
	return m.userStore.List()
}

// authService 认证服务实现
type authService struct {
	cfg *config.Config
}

// CheckPassword 校验用户名和密码
func (s *authService) CheckPassword(username, password string) bool {
	if username == "" {
		return false
	}
	return s.cfg.CheckPassword(username, password)
}

// GetUsernameByApiKey 通过 API Key 获取用户名
func (s *authService) GetUsernameByApiKey(apikey string) string {
	if apikey == "" {
		return ""
	}
	return s.cfg.GetUsernameByApiKey(apikey)
}

// GetApiKeyByUsername 通过用户名获取 API Key
func (s *authService) GetApiKeyByUsername(username string) string {
	if username == "" {
		return ""
	}
	return s.cfg.GetApiKeyByUsername(username)
}
