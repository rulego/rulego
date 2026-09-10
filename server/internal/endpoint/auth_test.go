package endpoint

import (
	"testing"

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
