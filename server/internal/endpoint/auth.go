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
)

// ruleGoClaim JWT claim
type ruleGoClaim struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// loginLimiter login rate limiter (IP-based)
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptInfo
}

type attemptInfo struct {
	count    int
	lastTime time.Time
}

// Global login speed limiter
var limiter = &loginLimiter{
	attempts: make(map[string]*attemptInfo),
}

const (
	maxLoginAttempts = 10              // Maximum number of attempts per IP window period
	loginWindow      = 1 * time.Minute // Window period
)

// If check allows, it returns true
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

// Regularly clean up expired speed-limited items
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

// extractAuthorization Extracts authorization values from requests
func extractAuthorization(exchange *endpointApi.Exchange) string {
	authorization := exchange.In.Headers().Get("Authorization")
	if authorization == "" {
		if token := exchange.In.GetParam("token"); token != "" {
			authorization = constants.BearerPrefix + token
		}
	}
	return authorization
}

// authProcess only authenticates middleware (does not check permissions) and is backward compatible.
// Equivalent to authWithPermission("", "").
func (s *Server) authProcess() func(endpointApi.Router, *endpointApi.Exchange) bool {
	return s.authWithPermission("", "")
}

// authWithPermission returns authentication + authorization middleware.
// When resource/action is empty, only authentication is performed without checking permissions.
// Authenticator/Authorizer is obtained from the container and is implemented by default when not registered.
func (s *Server) authWithPermission(resource, action string) func(endpointApi.Router, *endpointApi.Exchange) bool {
	return func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		cfg := s.config
		authorization := extractAuthorization(exchange)

		// Anonymous access: Use the default user when RequireAuth=false and no token
		if !cfg.RequireAuth && authorization == "" {
			exchange.In.GetMsg().Metadata.PutValue(constants.KeyUsername, cfg.DefaultUsername)
			return true
		}

		// Obtain the authenticator from the container
		authenticator := getAuthenticator(s.container, cfg)
		userCtx, err := authenticator.Authenticate(authorization)
		if err != nil {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			exchange.Out.SetBody([]byte(`{"error":"unauthorized"}`))
			return false
		}

		// Set the username to metadata
		exchange.In.GetMsg().Metadata.PutValue(constants.KeyUsername, userCtx.Username)

		// Obtain an authorizer from the container (skip permission check if the resource is empty)
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

// clientIP extracts the client's IP from the exchange
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

// loginRoute logs in to the route
func (s *Server) loginRoute() endpointApi.Router {
	cfg := s.config
	return endpoint.NewRouter().From(s.apiBasePath() + constants.PathLogin).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		// Login speed limits
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

		if user.Username == "" || user.Password == "" || !cfg.CheckPassword(user.Username, user.Password) {
			exchange.Out.SetStatusCode(http.StatusUnauthorized)
			exchange.Out.SetBody([]byte(`{"error":"invalid username or password"}`))
			return false
		}

		expiresAt := time.Now().Add(time.Duration(cfg.JwtExpireTime) * time.Millisecond)
		token, err := createToken(cfg, ruleGoClaim{
			Username: user.Username,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				Issuer:    cfg.JwtIssuer,
			},
		})
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{"token": *token, "expiresAt": expiresAt.Unix()})
		return true
	}).End()
}
