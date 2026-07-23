package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/constants"
)

// getService fetchs services from the container and writes a 500 error response on failure (without exposing internal details)
func getService[T any](s *Server, exchange *endpointApi.Exchange, name string) (T, bool) {
	svc, err := app.GetAs[T](s.container, name)
	if err != nil {
		exchange.Out.SetStatusCode(http.StatusInternalServerError)
		exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
		return svc, false
	}
	return svc, true
}

// writeError Writes the error response
func writeError(exchange *endpointApi.Exchange, code int, err error) {
	exchange.Out.SetStatusCode(code)
	exchange.Out.SetBody([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
}

// writeBadRequest writes 400 error responses (client errors, can expose information)
func writeBadRequest(exchange *endpointApi.Exchange, err error) {
	writeError(exchange, http.StatusBadRequest, err)
}

// writeInternalError writes 500 error responses (hides internal error details)
func writeInternalError(exchange *endpointApi.Exchange, err error) {
	exchange.Out.SetStatusCode(http.StatusInternalServerError)
	exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
}

// writeJSON writes JSON responses, returns 500 when serialization fails
func writeJSON(exchange *endpointApi.Exchange, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		writeInternalError(exchange, err)
		return
	}
	exchange.Out.SetBody(b)
}

// intParam retrieves integer parameters from message metadata
func intParam(msg *types.RuleMsg, key string, defaultVal int) int {
	if i, err := strconv.Atoi(msg.Metadata.GetValue(key)); err == nil {
		return i
	}
	return defaultVal
}

// writeNoContent writes 204 but no content response
func writeNoContent(exchange *endpointApi.Exchange) {
	exchange.Out.SetStatusCode(204)
}

// writeListResult writes the paged list response
func writeListResult(exchange *endpointApi.Exchange, items interface{}, total, page, size int) {
	writeJSON(exchange, map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"items": items,
	})
}

// metadataUsername Retrieves the username from the exchange
func metadataUsername(exchange *endpointApi.Exchange) string {
	return exchange.In.GetMsg().Metadata.GetValue(constants.KeyUsername)
}

// metadataValue retrieves the metadata value of the specified key from the exchange
func metadataValue(exchange *endpointApi.Exchange, key string) string {
	return exchange.In.GetMsg().Metadata.GetValue(key)
}
