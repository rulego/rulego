package endpoint

import (
	"testing"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
)

func TestExtractAuthorization(t *testing.T) {
	setHeader := func(exchange *endpointApi.Exchange, key, value string) {
		exchange.In.Headers().Set(key, value)
	}

	t.Run("Authorization header wins", func(t *testing.T) {
		exchange := newTestExchange(t)
		setHeader(exchange, "Authorization", "Bearer jwt-token")
		setHeader(exchange, "X-API-Key", "api-key")
		if got := extractAuthorization(exchange); got != "Bearer jwt-token" {
			t.Fatalf("expected Authorization header to win, got %q", got)
		}
	})

	t.Run("X-API-Key header", func(t *testing.T) {
		exchange := newTestExchange(t)
		setHeader(exchange, "X-API-Key", "api-key")
		if got := extractAuthorization(exchange); got != constants.BearerPrefix+"api-key" {
			t.Fatalf("expected Bearer-prefixed api key, got %q", got)
		}
	})

	t.Run("empty without headers", func(t *testing.T) {
		exchange := newTestExchange(t)
		if got := extractAuthorization(exchange); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})
}

func TestConfigureLoginLimiter(t *testing.T) {
	t.Run("0 取默认值", func(t *testing.T) {
		configureLoginLimiter(0, 0)
		if limiter.maxAttempts != defaultMaxLoginAttempts || limiter.window != defaultLoginWindow {
			t.Fatalf("expected defaults, got max=%d window=%v", limiter.maxAttempts, limiter.window)
		}
	})

	t.Run("自定义阈值与窗口", func(t *testing.T) {
		configureLoginLimiter(2, 1)
		defer configureLoginLimiter(defaultMaxLoginAttempts, int(defaultLoginWindow/time.Second))
		if !limiter.check("ip-a") || !limiter.check("ip-a") {
			t.Fatal("expected first two attempts allowed")
		}
		if limiter.check("ip-a") {
			t.Fatal("expected third attempt blocked")
		}
	})

	t.Run("负数关闭限流", func(t *testing.T) {
		configureLoginLimiter(-1, 60)
		defer configureLoginLimiter(defaultMaxLoginAttempts, int(defaultLoginWindow/time.Second))
		for i := 0; i < 100; i++ {
			if !limiter.check("ip-b") {
				t.Fatalf("expected unlimited attempts when disabled, blocked at %d", i+1)
			}
		}
	})
}

func TestAuthWithPermission(t *testing.T) {
	newAuthServer := func(requireAuth bool) *Server {
		return &Server{
			container: app.NewContainer(),
			config: &config.Config{
				RequireAuth:     requireAuth,
				DefaultUsername: "admin",
				JwtSecretKey:    "unit-test-secret",
			},
		}
	}

	t.Run("免鉴权下无效token按匿名放行", func(t *testing.T) {
		srv := newAuthServer(false)
		exchange := newTestExchange(t)
		exchange.In.Headers().Set("Authorization", "Bearer expired-or-garbage")
		if !srv.authWithPermission("rule", "read")(nil, exchange) {
			t.Fatal("expected request to pass")
		}
		if outStatus(exchange) == 401 {
			t.Fatal("expected no 401 when RequireAuth=false")
		}
		if got := metadataUsername(exchange); got != "admin" {
			t.Fatalf("expected default username admin, got %q", got)
		}
	})

	t.Run("免鉴权匿名默认admin可写全局配置", func(t *testing.T) {
		srv := newAuthServer(false)
		srv.config.Users = map[string]string{"admin": "pass,admin-key"}
		srv.config.InitUserMap()
		exchange := newTestExchange(t)
		if !srv.authWithPermission("config", "write")(nil, exchange) {
			t.Fatalf("expected anonymous admin to pass config:write, got status %d", outStatus(exchange))
		}
	})

	t.Run("鉴权开启下无效token仍401", func(t *testing.T) {
		srv := newAuthServer(true)
		exchange := newTestExchange(t)
		exchange.In.Headers().Set("Authorization", "Bearer expired-or-garbage")
		if srv.authWithPermission("rule", "read")(nil, exchange) {
			t.Fatal("expected request to be rejected")
		}
		if outStatus(exchange) != 401 {
			t.Fatalf("expected 401, got %d", outStatus(exchange))
		}
	})
}

// 聊天直通限流：窗口期内超过阈值返回拒绝，负数配置关闭
func TestChatLimiter(t *testing.T) {
	configureChatLimiter(2, 60)
	defer configureChatLimiter(defaultMaxChatRequests, int(defaultChatWindow/time.Second))
	if !chatLimiter.check("user-a") || !chatLimiter.check("user-a") {
		t.Fatal("expected first two requests allowed")
	}
	if chatLimiter.check("user-a") {
		t.Fatal("expected third request blocked")
	}
	if !chatLimiter.check("user-b") {
		t.Fatal("other user should not be affected")
	}

	configureChatLimiter(-1, 60)
	defer configureChatLimiter(defaultMaxChatRequests, int(defaultChatWindow/time.Second))
	for i := 0; i < 100; i++ {
		if !chatLimiter.check("user-c") {
			t.Fatalf("expected unlimited requests when disabled, blocked at %d", i+1)
		}
	}
}
