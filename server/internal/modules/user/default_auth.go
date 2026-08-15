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

// RoleReader 读取用户角色。由 user 模块实现，认证器用它回填 UserContext.Roles。
// 抽成接口是为了让认证器不直接依赖 store。
type RoleReader interface {
	RolesOf(username string) []string
}

// ApiKeyReader 通过 API Key 反查用户名。RoleReader 的可选扩展：
// 实现了它的 RoleReader 能让运行时签发的 API Key 生效，否则只认 config.conf 的静态 Key。
type ApiKeyReader interface {
	GetUsernameByApiKey(apikey string) string
}

// UserStateReader 查询账号停用状态。RoleReader 的可选扩展：
// 实现了它的 RoleReader 让认证器拒绝停用账号——否则停用后旧 JWT 与
// config.conf 静态 Key 仍能认证，停用形同虚设。
type UserStateReader interface {
	IsDisabled(username string) bool
}

// errUserDisabled 停用账号的认证拒绝。消息不含内部细节，可回给客户端。
var errUserDisabled = errors.New("user is disabled")

// DefaultAuthenticator 默认认证器，使用 JWT + API Key 认证。
type DefaultAuthenticator struct {
	cfg   *config.Config
	roles RoleReader
}

// NewDefaultAuthenticator 创建默认认证器。
// roles 可为 nil（此时所有认证用户视为 admin，等价于多租户落地前的行为）。
func NewDefaultAuthenticator(cfg *config.Config, roles RoleReader) *DefaultAuthenticator {
	return &DefaultAuthenticator{cfg: cfg, roles: roles}
}

// rolesOf 查询角色。无 RoleReader 时回退 admin，保证单用户部署行为不变。
func (a *DefaultAuthenticator) rolesOf(username string) []string {
	if a.roles == nil {
		return []string{model.RoleAdmin}
	}
	if r := a.roles.RolesOf(username); len(r) > 0 {
		return r
	}
	// 认证通过但查不到角色（如 config 内置账号未落 store）：给 admin，不锁死管理员
	return []string{model.RoleAdmin}
}

// Authenticate 从 authorization 头识别用户，并回填角色供授权器判权
func (a *DefaultAuthenticator) Authenticate(authorization string) (*model.UserContext, error) {
	// 尝试 API Key
	if username := a.getUsernameByApiKey(authorization); username != "" {
		if a.isDisabled(username) {
			return nil, errUserDisabled
		}
		return &model.UserContext{Username: username, Roles: a.rolesOf(username)}, nil
	}
	// 尝试 JWT
	claim, err := parseToken(a.cfg, authorization)
	if err != nil {
		return nil, err
	}
	// JWT 有效期最长 12h，期间账号可能被停用，故每次认证都要查状态
	if a.isDisabled(claim.Username) {
		return nil, errUserDisabled
	}
	// JWT 里带的 role 只作参考，权威来源是 store（避免旧 token 携带过期角色）
	roles := a.rolesOf(claim.Username)
	return &model.UserContext{Username: claim.Username, Roles: roles}, nil
}

// isDisabled 查询账号是否停用。roles 未实现 UserStateReader 时恒 false，
// 保持嵌入模式（自定义 RoleReader）的行为兼容。
func (a *DefaultAuthenticator) isDisabled(username string) bool {
	if checker, ok := a.roles.(UserStateReader); ok && checker != nil {
		return checker.IsDisabled(username)
	}
	return false
}

func (a *DefaultAuthenticator) getUsernameByApiKey(authorization string) string {
	if len(authorization) <= 7 || !strings.HasPrefix(authorization, constants.BearerPrefix) {
		return ""
	}
	apikey := authorization[7:]
	// store 优先：运行时签发/吊销的 Key 才能生效
	if reader, ok := a.roles.(ApiKeyReader); ok && reader != nil {
		if username := reader.GetUsernameByApiKey(apikey); username != "" {
			return username
		}
	}
	return a.cfg.GetUsernameByApiKey(apikey)
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
	// 只接受 HS256：系统只签发 HS256，避免同密钥其他算法的混淆攻击面
	tk, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JwtSecretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if c, ok := tk.Claims.(*ruleGoClaim); ok && tk.Valid {
		return c, nil
	}
	return nil, fmt.Errorf("token is invalid")
}

// DefaultAuthorizer 默认授权器，三档角色：
//   - admin ：全权，含用户管理
//   - editor：自己租户内全部读写，不能碰 user 资源
//   - viewer：只读
//
// 数据隔离由 per-user store 保证（每个请求只能看到自己 username 下的数据），
// 这里只管「能做什么动作」。嵌入模式可用 WithModuleOverride 替换。
type DefaultAuthorizer struct{}

// NewDefaultAuthorizer 创建默认授权器
func NewDefaultAuthorizer() *DefaultAuthorizer {
	return &DefaultAuthorizer{}
}

// 只读动作白名单：这些动作 viewer 也放行
var readOnlyActions = map[string]bool{
	"read": true,
	"list": true,
}

// Authorize 按角色判权
func (a *DefaultAuthorizer) Authorize(user *model.UserContext, resource, action string) error {
	// user 为 nil 表示匿名（require_auth=false 且无 token）：视为 admin，保持开箱体验
	if user == nil || len(user.Roles) == 0 {
		return nil
	}
	if user.HasRole(model.RoleAdmin) {
		return nil
	}
	// 用户管理只有 admin 能碰
	if resource == constants.ResourceUser {
		return &PermissionError{Resource: resource, Action: action}
	}
	if readOnlyActions[action] {
		return nil
	}
	// 写操作：editor 放行，viewer 拒绝
	if user.HasRole(model.RoleEditor) {
		return nil
	}
	return &PermissionError{Resource: resource, Action: action}
}

// PermissionError 权限不足错误
type PermissionError struct {
	Resource string
	Action   string
}

func (e *PermissionError) Error() string {
	return "permission denied: " + e.Resource + ":" + e.Action
}
