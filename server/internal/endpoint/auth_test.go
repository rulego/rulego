package endpoint

import (
	"testing"
	"time"

	endpointApi "github.com/rulego/rulego/api/types/endpoint"
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
