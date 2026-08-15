package endpoint

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/rest"
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

// writeError 写入错误响应。用 json.Marshal 而非 %q 拼接：%q 对控制字符产出
// \xNN 转义，不是合法 JSON。
func writeError(exchange *endpointApi.Exchange, code int, err error) {
	exchange.Out.SetStatusCode(code)
	body, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		body = []byte(`{"error":"internal server error"}`)
	}
	exchange.Out.SetBody(body)
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

// writeJSON 写入 JSON 响应，序列化失败时返回 500。
// 响应体在客户端支持 gzip 且有收益时透明压缩。
func writeJSON(exchange *endpointApi.Exchange, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		writeInternalError(exchange, err)
		return
	}
	exchange.Out.SetBody(maybeGzipJSON(exchange, b))
}

// writeJSONStatus 带自定义状态码的 JSON 响应。
// SetStatusCode 会立即刷出响应头，必须先压缩设头再写状态码，
// 否则 Content-Encoding 丢失而体已压缩。
func writeJSONStatus(exchange *endpointApi.Exchange, code int, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		writeInternalError(exchange, err)
		return
	}
	b = maybeGzipJSON(exchange, b)
	exchange.Out.SetStatusCode(code)
	exchange.Out.SetBody(b)
}

// maybeGzipJSON 小于 1KB 或压完更大则原样返回。
// 必须在任何 SetStatusCode/SetBody 之前调用。
func maybeGzipJSON(exchange *endpointApi.Exchange, body []byte) []byte {
	if len(body) < 1024 || !gzipEnabled {
		return body
	}
	req, ok := exchange.In.(*rest.RequestMessage)
	if !ok || !requestAcceptsGzip(req.Request()) {
		return body
	}
	gz := gzipBytes(body)
	if len(gz) >= len(body) {
		return body
	}
	h := exchange.Out.Headers()
	if h == nil {
		return body
	}
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	return gz
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
