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

// DefaultAuthenticator 默认认证器，使用 JWT + API Key 认证。
// 与原有 authProcess() 逻辑完全一致，保证向后兼容。
type DefaultAuthenticator struct {
	cfg *config.Config
}

// NewDefaultAuthenticator 创建默认认证器
func NewDefaultAuthenticator(cfg *config.Config) *DefaultAuthenticator {
	return &DefaultAuthenticator{cfg: cfg}
}

// Authenticate 从 authorization 头识别用户
func (a *DefaultAuthenticator) Authenticate(authorization string) (*model.UserContext, error) {
	// 尝试 API Key
	if username := a.getUsernameByApiKey(authorization); username != "" {
		return &model.UserContext{Username: username}, nil
	}
	// 尝试 JWT
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

// DefaultAuthorizer 默认授权器，全部放行（与当前行为一致）。
// 嵌入模式可替换为自定义实现。
type DefaultAuthorizer struct{}

// NewDefaultAuthorizer 创建默认授权器
func NewDefaultAuthorizer() *DefaultAuthorizer {
	return &DefaultAuthorizer{}
}

// Authorize 全部放行
func (a *DefaultAuthorizer) Authorize(user *model.UserContext, resource, action string) error {
	return nil
}
