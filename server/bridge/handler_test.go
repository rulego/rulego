package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/user"
)

func TestBridgeHealthEndpoint(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	handler := br.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Fatalf("expected OK, got %s", w.Body.String())
	}
}

func TestBridgeLoginEndpoint(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	body := `{"username":"admin","password":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := result["token"]; !ok {
		t.Fatal("response missing token field")
	}
}

func TestBridgeRulesEndpoint(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	token := loginAndGetToken(t, br)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := result["items"]; !ok {
		t.Fatal("response missing items field")
	}
}

func TestBridgeWithStripPrefix(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	// 模拟挂载到 /rulego 前缀下
	mux := http.NewServeMux()
	mux.Handle("/rulego/", http.StripPrefix("/rulego", br.Handler()))

	// 测试: /rulego/health -> 内部路由 /health
	req := httptest.NewRequest(http.MethodGet, "/rulego/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Fatalf("expected OK, got %s", w.Body.String())
	}

	// 测试: /rulego/api/v1/login -> 内部路由 /api/v1/login
	loginBody := `{"username":"admin","password":"admin"}`
	req = httptest.NewRequest(http.MethodPost, "/rulego/api/v1/login", stringReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBridgeStop(t *testing.T) {
	br := newTestBridge(t)
	if err := br.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
}

func newTestBridge(t *testing.T) *Bridge {
	t.Helper()

	// 写临时配置文件，使用随机端口避免冲突
	cfgContent := "server = :0\ndata_dir = " + filepath.Join(t.TempDir(), "data") + "\n" +
		"default_username = admin\n" +
		"require_auth = false\n" +
		"[users]\nadmin = admin,2af255ea5618467d914c67a8beeca31d\n"
	cfgFile := filepath.Join(t.TempDir(), "config.conf")
	os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	application := app.New(
		app.WithConfigFile(cfgFile),
		app.WithModules(user.New(), rule.New()),
	)
	br, err := NewBridge(application)
	if err != nil {
		t.Fatalf("NewBridge error: %v", err)
	}
	return br
}

func loginAndGetToken(t *testing.T, br *Bridge) string {
	t.Helper()
	body := `{"username":"admin","password":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("login response parse error: %v", err)
	}
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatal("login returned empty token")
	}
	return token
}

func stringReader(s string) *strings.Reader {
	return strings.NewReader(s)
}
