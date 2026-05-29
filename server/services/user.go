package services

import (
	"github.com/rulego/rulego/server/model"
)

// AuthService 用户认证服务接口
type AuthService interface {
	CheckPassword(username, password string) bool
	GetUsernameByApiKey(apikey string) string
	GetApiKeyByUsername(username string) string
}

// UserReader 用户信息读取接口
type UserReader interface {
	List() []model.User
}

// Authenticator 认证接口，负责从请求中识别用户身份。
// 默认实现使用 JWT + API Key 认证，嵌入模式可替换为外部认证（OAuth2、LDAP 等）。
type Authenticator interface {
	// Authenticate 从 authorization 头中识别用户，返回 UserContext。
	// authorization 为 "Bearer xxx" 格式的原始值。
	// 认证失败应返回 error。
	Authenticate(authorization string) (*model.UserContext, error)
}

// Authorizer 授权接口，负责检查用户是否有权限执行操作。
// 默认实现全部放行，嵌入模式可替换为外部 RBAC/ABAC 系统。
type Authorizer interface {
	// Authorize 检查用户是否有权限对指定资源执行操作。
	// resource: 资源类型，如 "rule", "component", "config"
	// action: 操作类型，如 "read", "write", "delete", "execute"
	// 无权限应返回 error，有权限返回 nil。
	Authorize(user *model.UserContext, resource, action string) error
}
