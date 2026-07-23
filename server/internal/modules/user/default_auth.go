package user

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
)

// DefaultAuthenticator is the default authenticator that uses JWT + API Key for authentication.
// Completely consistent with the original authProcess() logic, ensuring backward compatibility.
type DefaultAuthenticator struct {
	cfg *config.Config
}

// NewDefaultAuthenticator creates the default authenticator
func NewDefaultAuthenticator(cfg *config.Config) *DefaultAuthenticator {
	return &DefaultAuthenticator{cfg: cfg}
}

// Authenticate identifies users from the authorization header
func (a *DefaultAuthenticator) Authenticate(authorization string) (*model.UserContext, error) {
	// Try the API Key
	if username := a.getUsernameByApiKey(authorization); username != "" {
		return &model.UserContext{Username: username}, nil
	}
	// Try JWT
	claim, err := parseToken(a.cfg, authorization)
	if err != nil {
		return nil, err
	}
	return &model.UserContext{Username: claim.Username}, nil
}

func (a *DefaultAuthenticator) getUsernameByApiKey(authorization string) string {
	if len(authorization) <= 7 || !strings.HasPrefix(authorization, constants.BearerPrefix) {
		return ""
	}
	return a.cfg.GetUsernameByApiKey(authorization[7:])
}

// ruleGoClaim JWT claim
type ruleGoClaim struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func parseToken(cfg *config.Config, authorization string) (*ruleGoClaim, error) {
	if len(authorization) <= 7 {
		return nil, errors.New("illegal token")
	}
	token := authorization[7:]
	claims := &ruleGoClaim{}
	tk, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JwtSecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if c, ok := tk.Claims.(*ruleGoClaim); ok && tk.Valid {
		return c, nil
	}
	return nil, fmt.Errorf("token is invalid")
}

// DefaultAuthorizer: The default authorizer, all permissions are granted (consistent with current behavior).
// Embedding mode can be replaced with custom implementations.
type DefaultAuthorizer struct{}

// NewDefaultAuthorizer creates the default authorizer
func NewDefaultAuthorizer() *DefaultAuthorizer {
	return &DefaultAuthorizer{}
}

// Authorize to let all the authors go
func (a *DefaultAuthorizer) Authorize(user *model.UserContext, resource, action string) error {
	return nil
}
