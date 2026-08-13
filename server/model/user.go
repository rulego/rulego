package model

// 角色常量。多租户下「租户 = 用户」，角色决定该租户账号能做什么。
const (
	// RoleAdmin 全权，含用户管理
	RoleAdmin = "admin"
	// RoleEditor 自己租户内全部读写，不能管用户
	RoleEditor = "editor"
	// RoleViewer 只读
	RoleViewer = "viewer"
)

// User 用户领域模型。
type User struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	ApiKey   string `json:"apiKey,omitempty"`
	// Roles 角色列表，为空视为 RoleEditor
	Roles []string `json:"roles,omitempty"`
	// Disabled 停用后不能登录，数据保留
	Disabled bool `json:"disabled,omitempty"`
}

func (u User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// UserContext 认证后的用户上下文：身份 + 角色 + 附加属性。
type UserContext struct {
	Username string            `json:"username"`
	Roles    []string          `json:"roles,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

// HasRole nil 接收者直接返回 false，调用方无需先判空。
func (c *UserContext) HasRole(role string) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}
