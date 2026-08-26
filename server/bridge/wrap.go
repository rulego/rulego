package bridge

import (
	"bufio"
	"bytes"
	"fmt"
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
//   - 缓冲模式下上游一旦调用 Flush（流式响应的标志），立即把已缓冲内容直写并切换
//     透传，保证增量推送语义不变。
//
// wrapHandler wraps the upstream handler with a ResponseWrapper.
// Buffering policy (SSE/stream/binary safe): only JSON responses are buffered and
// rewritten; anything else passes through untouched. A Flush during buffering dumps
// the buffer and switches to pass-through, preserving incremental streaming.
func wrapHandler(h http.Handler, wrap ResponseWrapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wrap == nil || r.Method == http.MethodHead || r.Method == http.MethodOptions {
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
	wrapped, code := wrap(b.status, b.buf.Bytes())
	if code <= 0 {
		code = b.status
	}
	b.ResponseWriter.WriteHeader(code)
	if len(wrapped) > 0 {
		_, _ = b.ResponseWriter.Write(wrapped)
	}
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
