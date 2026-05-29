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

// getService 从容器获取服务，失败时写入 500 错误响应（不暴露内部细节）
func getService[T any](s *Server, exchange *endpointApi.Exchange, name string) (T, bool) {
	svc, err := app.GetAs[T](s.container, name)
	if err != nil {
		exchange.Out.SetStatusCode(http.StatusInternalServerError)
		exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
		return svc, false
	}
	return svc, true
}

// writeError 写入错误响应
func writeError(exchange *endpointApi.Exchange, code int, err error) {
	exchange.Out.SetStatusCode(code)
	exchange.Out.SetBody([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
}

// writeBadRequest 写入 400 错误响应（客户端错误，可以暴露信息）
func writeBadRequest(exchange *endpointApi.Exchange, err error) {
	writeError(exchange, http.StatusBadRequest, err)
}

// writeInternalError 写入 500 错误响应（隐藏内部错误细节）
func writeInternalError(exchange *endpointApi.Exchange, err error) {
	exchange.Out.SetStatusCode(http.StatusInternalServerError)
	exchange.Out.SetBody([]byte(`{"error":"internal server error"}`))
}

// writeJSON 写入 JSON 响应，序列化失败时返回 500
func writeJSON(exchange *endpointApi.Exchange, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		writeInternalError(exchange, err)
		return
	}
	exchange.Out.SetBody(b)
}

// intParam 从消息元数据获取整数参数
func intParam(msg *types.RuleMsg, key string, defaultVal int) int {
	if i, err := strconv.Atoi(msg.Metadata.GetValue(key)); err == nil {
		return i
	}
	return defaultVal
}

// writeNoContent 写入 204 无内容响应
func writeNoContent(exchange *endpointApi.Exchange) {
	exchange.Out.SetStatusCode(204)
}

// writeListResult 写入分页列表响应
func writeListResult(exchange *endpointApi.Exchange, items interface{}, total, page, size int) {
	writeJSON(exchange, map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"items": items,
	})
}

// metadataUsername 从 exchange 获取用户名
func metadataUsername(exchange *endpointApi.Exchange) string {
	return exchange.In.GetMsg().Metadata.GetValue(constants.KeyUsername)
}

// metadataValue 从 exchange 获取指定 key 的元数据值
func metadataValue(exchange *endpointApi.Exchange, key string) string {
	return exchange.In.GetMsg().Metadata.GetValue(key)
}
