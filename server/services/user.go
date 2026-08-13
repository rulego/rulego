package services

import (
	"github.com/rulego/rulego/server/model"
)

// AuthService 认证服务：密码与 API Key 双向查证。
type AuthService interface {
	CheckPassword(username, password string) bool
	GetUsernameByApiKey(apikey string) string
	GetApiKeyByUsername(username string) string
}

// UserReader 只读用户列表接口。
type UserReader interface {
	List() []model.User
}

// UserAdmin 用户管理接口（多租户下「租户 = 用户」）。
// 由 user 模块实现，endpoint 层 /users CRUD 消费。
type UserAdmin interface {
	List() []model.User
	Get(username string) (model.User, bool)
	Save(user model.User) error
	Delete(username string) error
	RolesOf(username string) []string
}

// Authenticator 从请求凭证识别用户身份。
// 默认实现走 JWT + API Key；嵌入模式可换成 OAuth2、LDAP 等外部认证。
type Authenticator interface {
	// Authenticate 解析 authorization 头（"Bearer xxx" 原始值），返回 UserContext。
	// 失败返回 error。
	Authenticate(authorization string) (*model.UserContext, error)
}

// Authorizer 检查用户能否对资源执行操作。
// 默认实现按 admin/editor/viewer 三档判权（匿名与无角色视为 admin，保持开箱体验）；
// 嵌入模式可换成外部 RBAC/ABAC。
type Authorizer interface {
	// Authorize 检查 user 对 resource 能否做 action。
	// resource 如 "rule"/"component"/"config"，action 如 "read"/"write"/"delete"/"execute"。
	// 有权限返回 nil，否则 error。
	Authorize(user *model.UserContext, resource, action string) error
}
