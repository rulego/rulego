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

	m.authSvc = &authService{cfg: m.cfg, userStore: userStore}
	if err := ctx.Container.Register(services.KeyAuthService, services.AuthService(m.authSvc)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyUserProfile, services.UserReader(m)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyUserAdmin, services.UserAdmin(m)); err != nil {
		return err
	}

	// 注册默认认证器与授权器（RegisterIfAbsent）：宿主通过 app.WithAuthenticator/
	// WithAuthorizer 注入的实现优先，此处仅在未注入时补默认值。
	// （除 WithModuleOverride 替换整个 user 模块外，宿主现在有了轻量的 SPI 注入路径。）
	// 传入 m 作为 RoleReader：认证后回填 UserContext.Roles 供授权器判权。
	ctx.Container.RegisterIfAbsent(services.KeyAuthenticator, services.Authenticator(NewDefaultAuthenticator(m.cfg, m)))
	ctx.Container.RegisterIfAbsent(services.KeyAuthorizer, services.Authorizer(NewDefaultAuthorizer()))

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

// Get 返回单个用户
func (m *Module) Get(username string) (model.User, bool) {
	return m.userStore.GetUser(username)
}

// Save 创建或更新用户
func (m *Module) Save(user model.User) error {
	return m.userStore.CreateUser(user)
}

// Delete 删除用户。调用方（endpoint 层）负责先停引擎再清数据目录。
func (m *Module) Delete(username string) error {
	return m.userStore.Delete(username)
}

// GetUsernameByApiKey 通过 API Key 反查用户名，只查 store。
// config.conf 静态 Key 的回退由认证器负责（这里实现 ApiKeyReader 供其调用）。
func (m *Module) GetUsernameByApiKey(apikey string) string {
	if apikey == "" {
		return ""
	}
	return m.userStore.GetUsernameByApiKey(apikey)
}

// RolesOf 返回用户角色。store 里没有则回退 config：config.conf 内置账号视为
// admin（保持升级前的开箱体验）。
func (m *Module) RolesOf(username string) []string {
	if u, ok := m.userStore.GetUser(username); ok && len(u.Roles) > 0 {
		return u.Roles
	}
	if m.cfg != nil && m.cfg.CheckUserExists(username) {
		return []string{model.RoleAdmin}
	}
	return nil
}

// IsDisabled 查询账号是否停用，供认证器拒绝停用账号。不在 store 视为未停用。
func (m *Module) IsDisabled(username string) bool {
	if m.userStore == nil || username == "" {
		return false
	}
	if u, ok := m.userStore.GetUser(username); ok {
		return u.Disabled
	}
	return false
}

// authService 认证服务实现
type authService struct {
	cfg       *config.Config
	userStore store.UserStore
}

// CheckPassword 校验用户名和密码。先查 UserStore（运行时创建的租户），
// 未命中再回退 config.conf 的内置账号。
func (s *authService) CheckPassword(username, password string) bool {
	if username == "" {
		return false
	}
	if s.userStore != nil {
		if u, ok := s.userStore.GetUser(username); ok {
			if u.Disabled {
				// 停用是明确意图，不给 config 回退机会
				return false
			}
			// 密码为空表示 store 未持有凭据（如内置账号只在 store 存了 apiKey），
			// 此时 store 不权威，回退 config 以免把内置账号锁死
			if u.Password != "" {
				return s.userStore.ValidatePassword(username, password)
			}
		}
	}
	return s.cfg.CheckPassword(username, password)
}

// GetUsernameByApiKey 通过 API Key 取用户名，store 优先，回退 config。
func (s *authService) GetUsernameByApiKey(apikey string) string {
	if apikey == "" {
		return ""
	}
	if s.userStore != nil {
		if username := s.userStore.GetUsernameByApiKey(apikey); username != "" {
			return username
		}
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
