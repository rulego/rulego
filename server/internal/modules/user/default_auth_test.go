package user

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
)

func TestDefaultAuthenticator_APIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["testuser"] = "testpass,my-api-key"
	cfg.InitUserMap()

	// RoleReader 传 nil：所有认证用户视为 admin，等价于多租户落地前的行为
	auth := NewDefaultAuthenticator(&cfg, nil)

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

	auth := NewDefaultAuthenticator(&cfg, nil)

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

	auth := NewDefaultAuthenticator(&cfg, nil)

	// API Key 优先于 JWT（当 API Key 匹配时直接返回）
	ctx, err := auth.Authenticate("Bearer my-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Username != "apikeyuser" {
		t.Errorf("username = %q, want %q", ctx.Username, "apikeyuser")
	}
}

// stubUserState 同时实现 RoleReader / ApiKeyReader / UserStateReader 的测试桩，
// 模拟「账号已在 store 停用」的状态。
type stubUserState struct {
	disabled map[string]bool
	apiKeys  map[string]string // apiKey -> username（模拟 store 反查）
	roles    map[string][]string
}

func (s *stubUserState) RolesOf(username string) []string        { return s.roles[username] }
func (s *stubUserState) GetUsernameByApiKey(apikey string) string { return s.apiKeys[apikey] }
func (s *stubUserState) IsDisabled(username string) bool          { return s.disabled[username] }

// 停用账号后，config.conf 里的静态 API Key 仍能被 cfg 回退命中。
// 认证器必须查停用状态，否则停用对内置账号的 Key 形同虚设。
func TestDefaultAuthenticator_DisabledUser_ConfigApiKeyFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["builtuser"] = "pass,built-key"
	cfg.InitUserMap()

	stub := &stubUserState{
		disabled: map[string]bool{"builtuser": true},
		// store 反查不到该 Key（停用账号的 Key 不在 store），会走 cfg 回退
	}
	auth := NewDefaultAuthenticator(&cfg, stub)

	if _, err := auth.Authenticate("Bearer built-key"); err == nil {
		t.Error("停用账号的 config API Key 不应能认证")
	}
}

// 停用账号后旧 JWT 在过期前不应继续有效
func TestDefaultAuthenticator_DisabledUser_JWT(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitUserMap()

	stub := &stubUserState{disabled: map[string]bool{"jwtuser": true}}
	auth := NewDefaultAuthenticator(&cfg, stub)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ruleGoClaim{
		Username: "jwtuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    cfg.JwtIssuer,
		},
	})
	tokenStr, err := token.SignedString([]byte(cfg.JwtSecretKey))
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	if _, err := auth.Authenticate("Bearer " + tokenStr); err == nil {
		t.Error("停用账号的 JWT 不应能认证")
	}
}

// 未停用账号两条路径都要正常放行，停用检查不能误伤
func TestDefaultAuthenticator_EnabledUser_StillPasses(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["builtuser"] = "pass,built-key"
	cfg.InitUserMap()

	stub := &stubUserState{
		disabled: map[string]bool{}, // 无人停用
		apiKeys:  map[string]string{"store-key": "storeuser"},
		roles:    map[string][]string{"builtuser": {model.RoleAdmin}, "storeuser": {model.RoleEditor}},
	}
	auth := NewDefaultAuthenticator(&cfg, stub)

	// store Key
	ctx, err := auth.Authenticate("Bearer store-key")
	if err != nil || ctx.Username != "storeuser" {
		t.Errorf("store Key 认证 = (%v, %v), want storeuser", ctx, err)
	}
	// config Key 回退
	ctx, err = auth.Authenticate("Bearer built-key")
	if err != nil || ctx.Username != "builtuser" {
		t.Errorf("config Key 认证 = (%v, %v), want builtuser", ctx, err)
	}
}

// 多租户三档授权：admin 全权含用户管理 / editor 自己租户内读写但不能碰 user / viewer 只读。
// 本用例取代旧的 TestDefaultAuthorizer_AllowAll——授权器已从「无条件放行」改为按角色判权。
func TestDefaultAuthorizer_Roles(t *testing.T) {
	authz := NewDefaultAuthorizer()

	tests := []struct {
		name     string
		roles    []string
		resource string
		action   string
		wantErr  bool
	}{
		// admin：全通，含 user 资源
		{"admin 读 rule", []string{model.RoleAdmin}, "rule", "read", false},
		{"admin 写 rule", []string{model.RoleAdmin}, "rule", "write", false},
		{"admin 删 rule", []string{model.RoleAdmin}, "rule", "delete", false},
		{"admin 管 user", []string{model.RoleAdmin}, constants.ResourceUser, "write", false},

		// editor：普通资源读写通，user 资源拒
		{"editor 读 rule", []string{model.RoleEditor}, "rule", "read", false},
		{"editor 写 rule", []string{model.RoleEditor}, "rule", "write", false},
		{"editor 删 rule", []string{model.RoleEditor}, "rule", "delete", false},
		{"editor 执行 rule", []string{model.RoleEditor}, "rule", "execute", false},
		{"editor 管 user 应拒", []string{model.RoleEditor}, constants.ResourceUser, "write", true},
		{"editor 读 user 也拒", []string{model.RoleEditor}, constants.ResourceUser, "read", true},

		// viewer：只读通，写类全拒
		{"viewer 读 rule", []string{model.RoleViewer}, "rule", "read", false},
		{"viewer list rule", []string{model.RoleViewer}, "rule", "list", false},
		{"viewer 写 rule 应拒", []string{model.RoleViewer}, "rule", "write", true},
		{"viewer 删 rule 应拒", []string{model.RoleViewer}, "rule", "delete", true},
		{"viewer 执行 rule 应拒", []string{model.RoleViewer}, "rule", "execute", true},
		{"viewer 部署 rule 应拒", []string{model.RoleViewer}, "rule", "operate", true},
		{"viewer 管 user 应拒", []string{model.RoleViewer}, constants.ResourceUser, "read", true},

		// 无角色：视为 admin，保持升级前的开箱体验（见 default_auth.go 的 rolesOf）
		{"无角色写 rule", nil, "rule", "write", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &model.UserContext{Username: "u", Roles: tt.roles}
			err := authz.Authorize(user, tt.resource, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("Authorize(roles=%v, %q, %q) = %v, wantErr %v",
					tt.roles, tt.resource, tt.action, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultAuthorizer_NilUser(t *testing.T) {
	authz := NewDefaultAuthorizer()

	// nil user = 匿名（require_auth=false 且无 token），视为 admin 放行，不应 panic
	if err := authz.Authorize(nil, "rule", "read"); err != nil {
		t.Errorf("Authorize(nil, read) = %v, want nil", err)
	}
	if err := authz.Authorize(nil, "rule", "write"); err != nil {
		t.Errorf("Authorize(nil, write) = %v, want nil", err)
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
			"rule:delete":     true,
			"config:write":    true,
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

// 同密钥签发的非 HS256 token 应被拒绝。
func TestParseToken_RejectsNonHS256(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitUserMap()

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, ruleGoClaim{
		Username: "hacker",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	})
	tokenStr, err := token.SignedString([]byte(cfg.JwtSecretKey))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := parseToken(&cfg, "Bearer "+tokenStr); err == nil {
		t.Error("HS512 token should be rejected")
	}
}
