package model

// User user domain model
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ApiKey   string `json:"apiKey"`
}

// UserContext The authenticated user context containing identity and authorization information
type UserContext struct {
	Username string            `json:"username"`
	Roles    []string          `json:"roles,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}
