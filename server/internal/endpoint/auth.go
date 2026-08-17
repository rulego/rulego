package endpoint

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

// ruleGoClaim JWT claim
type ruleGoClaim struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// loginLimiter 登录速率限制器（基于 IP）
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptInfo
}

type attemptInfo struct {
	count    int
	lastTime time.Time
}

// 全局登录限速器
var limiter = &loginLimiter{
	attempts: make(map[string]*attemptInfo),
}

const (
	maxLoginAttempts = 10              // 每个 IP 窗口期内最大尝试次数
	loginWindow      = 1 * time.Minute // 窗口期
)

// check 允许该 IP 继续登录则返回 true
func (l *loginLimiter) check(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	info, ok := l.attempts[ip]
	if !ok || time.Since(info.lastTime) > loginWindow {
		l.attempts[ip] = &attemptInfo{count: 1, lastTime: time.Now()}
		return true
	}
	info.lastTime = time.Now()
	info.count++
	return info.count <= maxLoginAttempts
}

// 后台定期清理过期的限速条目
func init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			limiter.mu.Lock()
			for ip, info := range limiter.attempts {
				if time.Since(info.lastTime) > loginWindow {
					delete(limiter.attempts, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()
}

func createToken(cfg *config.Config, claim ruleGoClaim) (*string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenStr, err := token.SignedString([]byte(cfg.JwtSecretKey))
	if err != nil {
		return nil, err
	}
	return &tokenStr, nil
}

// extractAuthorization 从请求中提取 authorization 值。
// 从 URL query 取 token 而非 GetParam，避免 FormValue 触发 ParseMultipartForm 消费 body。
func extractAuthorization(exchange *endpointApi.Exchange) string {
	authorization := exchange.In.Headers().Get("Authorization")
	if authorization == "" {
		if reqMsg, ok := exchange.In.(interface{ Request() *http.Request }); ok {
			if req := reqMsg.Request(); req != nil {
				if token := req.URL.Query().Get("token"); token != "" {
					authorization = constants.BearerPrefix + token
				}
			}
		}
	}
	return authorization
}

// authProcess 仅认证中间件（不检查权限），等价于 authWithPermission("", "")，保留是为了向后兼容。
func (s *Server) authProcess() func(endpointApi.Router, *endpointApi.Exchange) bool {
	return s.authWithPermission("", "")
}

// authWithPermission 返回认证+授权中间件。resource/action 为空时仅认证不鉴权；
// Authenticator/Authorizer 从 Container 取，未注册则用默认实现。
func (s *Server) authWithPermission(resource, action string) func(endpointApi.Router, *endpointApi.Exchange) bool {
	return func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		cfg := s.config
		authorization := extractAuthorization(exchange)

		// 匿名分支（RequireAuth=false 且无 token）：用默认用户身份放行，但仍要走
		// Authorizer。否则 require_auth=false 下 /users 这类敏感接口会被匿名直接
		// 调用（建/删用户、purge 数据目录）。默认用户当前是 admin 能过，但若将来
		// 降权或收紧资源，这层校验就是防线。
		if !cfg.RequireAuth && authorization == "" {
			username := cfg.DefaultUsername
			exchange.In.GetMsg().Metadata.PutValue(constants.KeyUsername, username)
			if resource != "" && action != "" {
				userCtx := &model.UserContext{Username: username, Roles: s.rolesOfUser(username)}
				if err := getAuthorizer(s.container).Authorize(userCtx, resource, action); err != nil {
					exchange.Out.SetStatusCode(http.StatusForbidden)
					exchange.Out.SetBody([]byte(`{"error":"forbidden"}`))
					return false
				}
			}
			return true
		}

		// 从 Container 取认证器
		authenticator := getAuthenticator(s.container, cfg)
		userCtx, err := authenticator.Authenticate(authorization)
		if err != nil {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			exchange.Out.SetBody([]byte(`{"error":"unauthorized"}`))
			return false
		}

		// 设置用户名到 metadata
		exchange.In.GetMsg().Metadata.PutValue(constants.KeyUsername, userCtx.Username)

		// 鉴权（resource 为空则跳过）
		if resource != "" && action != "" {
			authorizer := getAuthorizer(s.container)
			if err := authorizer.Authorize(userCtx, resource, action); err != nil {
				exchange.Out.SetStatusCode(http.StatusForbidden)
				exchange.Out.SetBody([]byte(`{"error":"forbidden"}`))
				return false
			}
		}
		return true
	}
}

// clientIP 从 exchange 提取客户端 IP
func clientIP(exchange *endpointApi.Exchange) string {
	xff := exchange.In.Headers().Get("X-Forwarded-For")
	if xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	xri := exchange.In.Headers().Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	return exchange.In.From()
}

// checkPassword 校验密码。优先用容器里的 AuthService（先查 UserStore 再回退 config.conf），
// 未注册时退回只认 config 的实现。
func (s *Server) checkPassword(username, password string) bool {
	if svc, err := getServiceRaw[services.AuthService](s, services.KeyAuthService); err == nil {
		return svc.CheckPassword(username, password)
	}
	return s.config.CheckPassword(username, password)
}

// loginRoute 登录路由
func (s *Server) loginRoute() endpointApi.Router {
	cfg := s.config
	return endpoint.NewRouter().From(s.apiBasePath() + constants.PathLogin).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		// 登录速率限制
		ip := clientIP(exchange)
		if !limiter.check(ip) {
			exchange.Out.SetStatusCode(http.StatusTooManyRequests)
			exchange.Out.SetBody([]byte(`{"error":"too many login attempts, please try again later"}`))
			return false
		}

		var user struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(exchange.In.Body(), &user); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		user.Username = strings.TrimSpace(user.Username)
		user.Password = strings.TrimSpace(user.Password)

		if user.Username == "" || user.Password == "" || !s.checkPassword(user.Username, user.Password) {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			exchange.Out.SetBody([]byte(`{"error":"invalid username or password"}`))
			return false
		}

		// 角色带回登录响应：前端据此显隐「用户管理」入口、决定编辑器是否只读
		roles := s.rolesOfUser(user.Username)

		expiresAt := time.Now().Add(time.Duration(cfg.JwtExpireTime) * time.Millisecond)
		token, err := createToken(cfg, ruleGoClaim{
			Username: user.Username,
			Role:     strings.Join(roles, ","),
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				Issuer:    cfg.JwtIssuer,
			},
		})
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{
			"token":     *token,
			"expiresAt": expiresAt.Unix(),
			"username":  user.Username,
			"roles":     roles,
		})
		return true
	}).End()
}
