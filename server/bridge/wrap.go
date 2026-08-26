package bridge

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// wrapHandler 用 ResponseWrapper 包装上游 handler。
// 缓冲策略（避免破坏 SSE/流式/大文件下载）：
//   - 仅当 WriteHeader 时 Content-Type 为 application/json（或未设置，默认 JSON）
//     且状态码允许有 body 时进入缓冲模式，响应结束后整体交给 wrapper；
//   - 非 JSON（SSE text/event-stream、octet-stream 下载等）、204/304、HEAD/OPTIONS
//     直接透传，不做任何缓冲；
//   - MCP 端点（/api/v1/mcp/*）的 JSON-RPC 报文原样透传（见 isRawAPIPath）；
//   - 缓冲模式下上游一旦调用 Flush（流式响应的标志），立即把已缓冲内容直写并切换
//     透传，保证增量推送语义不变。
//
// wrapHandler wraps the upstream handler with a ResponseWrapper.
// Buffering policy (SSE/stream/binary safe): only JSON responses are buffered and
// rewritten; anything else (including raw MCP JSON-RPC) passes through untouched.
// A Flush during buffering dumps the buffer and switches to pass-through,
// preserving incremental streaming.
func wrapHandler(h http.Handler, wrap ResponseWrapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wrap == nil || r.Method == http.MethodHead || r.Method == http.MethodOptions || isRawAPIPath(r.URL.Path) {
			h.ServeHTTP(w, r)
			return
		}
		bw := &bufferedJSONWriter{ResponseWriter: w}
		h.ServeHTTP(bw, r)
		bw.finish(wrap)
	})
}

// bufferedJSONWriter 缓冲 JSON 响应；判定为不可缓冲时整体退化为直写。
type bufferedJSONWriter struct {
	http.ResponseWriter
	status        int
	buf           bytes.Buffer
	headerWritten bool
	passthrough   bool
}

func (b *bufferedJSONWriter) WriteHeader(code int) {
	if b.headerWritten {
		return
	}
	b.headerWritten = true
	b.status = code
	if code == http.StatusNoContent || code == http.StatusNotModified || !isJSONContentType(b.Header().Get("Content-Type")) {
		b.passthrough = true
		b.ResponseWriter.WriteHeader(code)
	}
}

func (b *bufferedJSONWriter) Write(p []byte) (int, error) {
	if !b.headerWritten {
		b.WriteHeader(http.StatusOK)
	}
	if b.passthrough {
		return b.ResponseWriter.Write(p)
	}
	return b.buf.Write(p)
}

// Flush 上游流式写法的信号：把已缓冲内容直写底层并切换透传。
func (b *bufferedJSONWriter) Flush() {
	if b.passthrough {
		if f, ok := b.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	b.passthrough = true
	b.headerWritten = true
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	b.ResponseWriter.WriteHeader(status)
	if b.buf.Len() > 0 {
		_, _ = b.ResponseWriter.Write(b.buf.Bytes())
		b.buf.Reset()
	}
	if f, ok := b.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack 透传 WebSocket 升级：劫持后响应已不受本层控制，直接切换透传。
func (b *bufferedJSONWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := b.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("bridge: underlying ResponseWriter does not support Hijack")
	}
	b.passthrough = true
	b.headerWritten = true
	return hj.Hijack()
}

// finish 响应结束时应用包装器（仅缓冲模式）。
func (b *bufferedJSONWriter) finish(wrap ResponseWrapper) {
	if b.passthrough {
		return
	}
	if !b.headerWritten {
		// 上游未写任何内容：按 200 空响应交给包装器决定
		b.status = http.StatusOK
	}
	body := b.buf.Bytes()
	// 上游已 gzip 压缩的 JSON（≥1KB 响应会触发内置压缩）：先解压成明文再交给
	// 包装器，否则包装器解析压缩体失败会静默跳过包装，大响应丢失信封。
	// 解压失败（响应体与 Content-Encoding 不符）按原样交给包装器，由其透传规则兜底。
	if strings.TrimSpace(b.Header().Get("Content-Encoding")) == "gzip" {
		if plain, err := gunzipBody(body); err == nil {
			body = plain
			b.Header().Del("Content-Encoding")
			b.Header().Del("Content-Length")
		}
	}
	wrapped, code := wrap(b.status, body)
	if code <= 0 {
		code = b.status
	}
	b.ResponseWriter.WriteHeader(code)
	if len(wrapped) > 0 {
		_, _ = b.ResponseWriter.Write(wrapped)
	}
}

// gunzipBody 解压 gzip 响应体。
func gunzipBody(body []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// isRawAPIPath 判定必须原样透传、不做信封包装的 API 路径。
// MCP StreamableHTTP 端点（/api/v1/mcp/*）的 JSON 形态响应是 JSON-RPC 协议报文，
// 包装会破坏标准 MCP 客户端解析（SSE 形态已按 Content-Type 透传）。
func isRawAPIPath(path string) bool {
	return strings.Contains(path, "/api/v1/mcp/")
}

// isJSONContentType 判断是否为 JSON 响应（含 charset 变体；空视为 JSON——
// net/http 默认会做 http.DetectContentType 探测，缓冲后统一交给 wrapper，
// wrapper 对解析失败的 body 应原样返回）。
func isJSONContentType(ct string) bool {
	if ct == "" {
		return true
	}
	return strings.Contains(ct, "application/json")
}
