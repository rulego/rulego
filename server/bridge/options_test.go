package bridge

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/user"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

// fakeAuthenticator 宿主认证器桩：任何请求都认证为 username=fake-tenant。
type fakeAuthenticator struct{}

func (fakeAuthenticator) Authenticate(_ string) (*model.UserContext, error) {
	return &model.UserContext{Username: "fake-tenant"}, nil
}

// envelopeWrapper 宿主信封桩：成功包 {code:200,data}，错误包 {code:status,message}，
// 非 JSON 原样返回。
func envelopeWrapper(status int, body []byte) ([]byte, int) {
	trimmed := strings.TrimSpace(string(body))
	var payload interface{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return body, status // 非 JSON（如 health 的 "OK"）原样透传
	}
	if status >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		msg := fmt.Sprintf("request failed (%d)", status)
		if json.Unmarshal([]byte(trimmed), &e) == nil && e.Error != "" {
			msg = e.Error
		}
		out, _ := json.Marshal(map[string]interface{}{"code": status, "message": msg})
		return out, status
	}
	out, _ := json.Marshal(map[string]interface{}{"code": 200, "message": "success", "data": payload})
	return out, status
}

// newOptsBridge 用编程式配置构造 bridge（不开本地登录端点时 RequireAuth=false 直接匿名访问）。
func newOptsBridge(t *testing.T, opts ...Option) *Bridge {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Server = ":0"
	cfg.RequireAuth = false
	cfg.DisableLocalAuth = true
	all := append([]Option{
		WithAppOptions(
			app.WithConfig(&cfg),
			app.WithModules(user.New(), rule.New()),
		),
	}, opts...)
	br, err := New(all...)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	return br
}

// TestNew_AuthenticatorOptionWins 验证 WithAuthenticator 注入的认证器优先于
// user 模块默认实现（注册发生在模块 Init 之前，user 模块 RegisterIfAbsent 跳过）。
func TestNew_AuthenticatorOptionWins(t *testing.T) {
	br := newOptsBridge(t, WithAppOptions(app.WithAuthenticator(fakeAuthenticator{})))
	defer br.Stop()

	got, err := app.GetAs[services.Authenticator](br.App().Container(), services.KeyAuthenticator)
	if err != nil {
		t.Fatalf("get authenticator: %v", err)
	}
	if _, ok := got.(fakeAuthenticator); !ok {
		t.Fatalf("expected fakeAuthenticator in container, got %T", got)
	}
}

// TestNew_NoListener 验证嵌入模式不监听端口：构造时指定一个空闲端口，
// New 之后该端口仍可被他人绑定（若 Start() 走了 ListenAndServe 则绑定失败）。
func TestNew_NoListener(t *testing.T) {
	// 找一个当前空闲端口
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Server = fmt.Sprintf("127.0.0.1:%d", port)
	cfg.RequireAuth = false
	cfg.DisableLocalAuth = true

	br, err := New(WithAppOptions(
		app.WithConfig(&cfg),
		app.WithModules(user.New(), rule.New()),
	))
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	defer br.Stop()

	// 端口应仍可绑定（bridge 未占用）
	ln2, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("port %d still occupied after New: %v (bridge should not listen)", port, err)
	}
	ln2.Close()
}

// TestNew_ResponseWrapperJSON 验证 JSON 响应被信封包装。
func TestNew_ResponseWrapperJSON(t *testing.T) {
	br := newOptsBridge(t, WithResponseWrapper(envelopeWrapper))
	defer br.Stop()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Code int         `json:"code"`
		Data interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response not enveloped: %v body=%s", err, w.Body.String())
	}
	if env.Code != 200 {
		t.Fatalf("expected envelope code 200, got %d", env.Code)
	}
	data, _ := env.Data.(map[string]interface{})
	if _, ok := data["items"]; !ok {
		t.Fatalf("envelope data missing items: %s", w.Body.String())
	}
}

// TestNew_ResponseWrapperNonJSONPassthrough 验证非 JSON 响应（health 的 "OK"）不被包装。
func TestNew_ResponseWrapperNonJSONPassthrough(t *testing.T) {
	br := newOptsBridge(t, WithResponseWrapper(envelopeWrapper))
	defer br.Stop()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Body.String() != "OK" {
		t.Fatalf("expected raw OK, got %q", w.Body.String())
	}
}

// TestNew_LocalAuthDisabled 验证 WithoutLocalAuth 时 /api/v1/login 未注册（404）。
func TestNew_LocalAuthDisabled(t *testing.T) {
	br := newOptsBridge(t) // newOptsBridge 已含 DisableLocalAuth
	defer br.Stop()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(`{"username":"admin","password":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for disabled login route, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWrapHandler_SSEPassthrough 验证 SSE 流式响应不被缓冲：分块 + Flush 逐段直达。
func TestWrapHandler_SSEPassthrough(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		w.Write([]byte("data: chunk1\n\n"))
		f.Flush()
		w.Write([]byte("data: chunk2\n\n"))
		f.Flush()
	})

	h := wrapHandler(upstream, func(status int, body []byte) ([]byte, int) {
		t.Fatalf("wrapper must not be called for SSE, got status=%d body=%s", status, body)
		return nil, 0
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/stream", nil))

	if got := rec.Body.String(); got != "data: chunk1\n\ndata: chunk2\n\n" {
		t.Fatalf("SSE body corrupted: %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type changed: %s", ct)
	}
}

// TestWrapHandler_NoContentAndHEADPassthrough 验证 204 与 HEAD 不进包装器。
func TestWrapHandler_NoContentAndHEADPassthrough(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	})

	called := false
	h := wrapHandler(upstream, func(status int, body []byte) ([]byte, int) {
		called = true
		return body, status
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/x", nil))
	if rec.Code != http.StatusNoContent || called {
		t.Fatalf("204 should pass through unwrapped, code=%d wrapperCalled=%v", rec.Code, called)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodHead, "/x", nil))
	if called {
		t.Fatal("HEAD should bypass the wrapper entirely")
	}
}

// TestWrapHandler_ErrorEnveloped 验证错误状态码的 JSON 响应同样被包装（保持原 status）。
func TestWrapHandler_ErrorEnveloped(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid username or password"}`))
	})

	h := wrapHandler(upstream, envelopeWrapper)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/login", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status must be preserved, got %d", rec.Code)
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("error response not enveloped: %s", rec.Body.String())
	}
	if env.Code != 400 || env.Message != "invalid username or password" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}
