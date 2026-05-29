package model

// User 用户领域模型
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ApiKey   string `json:"apiKey"`
}

// UserContext 认证后的用户上下文，包含身份和授权信息
type UserContext struct {
	Username string            `json:"username"`
	Roles    []string          `json:"roles,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}
