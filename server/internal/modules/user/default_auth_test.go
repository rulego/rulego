package user

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

func TestDefaultAuthenticator_APIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["testuser"] = "testpass,my-api-key"
	cfg.InitUserMap()

	auth := NewDefaultAuthenticator(&cfg)

	// 有效 API Key
	ctx, err := auth.Authenticate("Bearer my-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Username != "testuser" {
		t.Errorf("username = %q, want %q", ctx.Username, "testuser")
	}

	// 无效 API Key
	_, err = auth.Authenticate("Bearer wrong-key")
	if err == nil {
		t.Error("expected error for invalid API key")
	}

	// 空 authorization
	_, err = auth.Authenticate("")
	if err == nil {
		t.Error("expected error for empty authorization")
	}
}

func TestDefaultAuthenticator_JWT(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitUserMap()

	auth := NewDefaultAuthenticator(&cfg)

	// 创建有效 JWT
	expiresAt := time.Now().Add(1 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ruleGoClaim{
		Username: "jwtuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    cfg.JwtIssuer,
		},
	})
	tokenStr, err := token.SignedString([]byte(cfg.JwtSecretKey))
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// 有效 JWT
	ctx, err := auth.Authenticate("Bearer " + tokenStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Username != "jwtuser" {
		t.Errorf("username = %q, want %q", ctx.Username, "jwtuser")
	}

	// 过期 JWT
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, ruleGoClaim{
		Username: "expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			Issuer:    cfg.JwtIssuer,
		},
	})
	expiredStr, _ := expiredToken.SignedString([]byte(cfg.JwtSecretKey))
	_, err = auth.Authenticate("Bearer " + expiredStr)
	if err == nil {
		t.Error("expected error for expired token")
	}

	// 无效签名
	_, err = auth.Authenticate("Bearer invalid-token-string")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestDefaultAuthenticator_Priority(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["apikeyuser"] = "pass,my-key"
	cfg.InitUserMap()

	auth := NewDefaultAuthenticator(&cfg)

	// API Key 优先于 JWT（当 API Key 匹配时直接返回）
	ctx, err := auth.Authenticate("Bearer my-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Username != "apikeyuser" {
		t.Errorf("username = %q, want %q", ctx.Username, "apikeyuser")
	}
}

func TestDefaultAuthorizer_AllowAll(t *testing.T) {
	authz := NewDefaultAuthorizer()
	user := &model.UserContext{Username: "testuser", Roles: []string{"viewer"}}

	// 所有操作都应放行
	resources := []string{"rule", "component", "config", "log", "locale", "marketplace"}
	actions := []string{"read", "write", "delete", "execute", "operate", "access"}

	for _, res := range resources {
		for _, act := range actions {
			if err := authz.Authorize(user, res, act); err != nil {
				t.Errorf("Authorize(%q, %q, %q) = %v, want nil", user.Username, res, act, err)
			}
		}
	}
}

func TestDefaultAuthorizer_NilUser(t *testing.T) {
	authz := NewDefaultAuthorizer()

	// nil user 也不应 panic
	if err := authz.Authorize(nil, "rule", "read"); err != nil {
		t.Errorf("Authorize(nil, ...) = %v, want nil", err)
	}
}

// customAuthorizer 用于测试自定义授权器
type customAuthorizer struct {
	denied map[string]bool
}

func (a *customAuthorizer) Authorize(user *model.UserContext, resource, action string) error {
	key := resource + ":" + action
	if a.denied[key] {
		return &PermissionDeniedError{Resource: resource, Action: action}
	}
	return nil
}

type PermissionDeniedError struct {
	Resource string
	Action   string
}

func (e *PermissionDeniedError) Error() string {
	return "permission denied: " + e.Resource + ":" + e.Action
}

func TestCustomAuthorizer(t *testing.T) {
	authz := &customAuthorizer{
		denied: map[string]bool{
			"rule:delete":    true,
			"config:write":   true,
			"component:write": true,
		},
	}

	user := &model.UserContext{Username: "viewer", Roles: []string{"viewer"}}

	tests := []struct {
		resource string
		action   string
		wantErr  bool
	}{
		{"rule", "read", false},
		{"rule", "write", false},
		{"rule", "delete", true},
		{"config", "read", false},
		{"config", "write", true},
		{"component", "read", false},
		{"component", "write", true},
		{"log", "read", false},
	}

	for _, tt := range tests {
		err := authz.Authorize(user, tt.resource, tt.action)
		if (err != nil) != tt.wantErr {
			t.Errorf("Authorize(%q, %q) error = %v, wantErr %v", tt.resource, tt.action, err, tt.wantErr)
		}
	}
}

func TestUserContext_Fields(t *testing.T) {
	ctx := &model.UserContext{
		Username: "admin",
		Roles:    []string{"admin", "editor"},
		Attrs:    map[string]string{"dept": "engineering"},
	}

	if ctx.Username != "admin" {
		t.Errorf("Username = %q, want %q", ctx.Username, "admin")
	}
	if len(ctx.Roles) != 2 {
		t.Errorf("Roles len = %d, want 2", len(ctx.Roles))
	}
	if ctx.Attrs["dept"] != "engineering" {
		t.Errorf("Attrs[dept] = %q, want %q", ctx.Attrs["dept"], "engineering")
	}
}

func TestAuthenticatorInterface(t *testing.T) {
	// 验证 DefaultAuthenticator 实现了 Authenticator 接口
	var _ interface {
		Authenticate(authorization string) (*model.UserContext, error)
	} = &DefaultAuthenticator{}
}

func TestAuthorizerInterface(t *testing.T) {
	// 验证 DefaultAuthorizer 实现了 Authorizer 接口
	var _ interface {
		Authorize(user *model.UserContext, resource, action string) error
	} = &DefaultAuthorizer{}
}
