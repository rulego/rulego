package services

import (
	"github.com/rulego/rulego/server/model"
)

// AuthService user authentication service interface
type AuthService interface {
	CheckPassword(username, password string) bool
	GetUsernameByApiKey(apikey string) string
	GetApiKeyByUsername(username string) string
}

// UserReader user information reading interface
type UserReader interface {
	List() []model.User
}

// The Authenticator authentication interface, responsible for identifying user identity from requests.
// By default, authentication uses JWT + API Key, and embedding mode can be replaced with external authentication (OAuth2, LDAP, etc.).
type Authenticator interface {
	// Authenticate identifies users from the authorization header and returns UserContext.
	// authorization is the original value in the format "Bearer xxx".
	// Authentication failure should return error.
	Authenticate(authorization string) (*model.UserContext, error)
}

// Authorizer is the authorization interface, responsible for checking whether users have permission to perform operations.
// By default, all releases are implemented, and the embedded mode can be replaced with an external RBAC/ABAC system.
type Authorizer interface {
	// Authorize checks whether the user has permission to perform operations on the specified resource.
	// resource: resource type, such as "rule", "component", "config"
	// action: Operation type, such as "read", "write", "delete", "execute"
	// If you have no permissions, return error; if you have permissions, return nil.
	Authorize(user *model.UserContext, resource, action string) error
}
