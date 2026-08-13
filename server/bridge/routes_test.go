package bridge

// 路由级验收测试。
//
// 立项原因：多租户三处接线全错（路由参数撞 metadata 键、loginRoute 绕过 authService、
// API Key 走 config 静态 map）时，go test 仍然全绿——因为当时的测试全在服务层直接调方法，
// 没有一个走过 HTTP 路由。「服务层写对 ≠ 功能可用」这个教训必须落到测试里，不能只写进文档。
//
// 本文件覆盖 Console 依赖的新增路由。bridge 与主 server 共用同一套路由注册
// （NewStandardRestEndpoint），故这里过了就等于主 server 的路由也过了。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// doGet 用 bridge handler 发一个 GET，返回状态码与响应体
func doGet(t *testing.T, br *Bridge, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	br.Handler().ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// TestUsersMeRoute /users/me：当前登录者信息。
// 曾经的坑是登录不走 UserStore，这类只有过 HTTP 才暴露。
func TestUsersMeRoute(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, body := doGet(t, br, "/api/v1/users/me")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/users/me = %d, want 200；响应: %s", code, body)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("响应不是 JSON 对象: %v；原始: %s", err, body)
	}
	if payload["username"] != "admin" {
		t.Errorf("username = %v, want admin（配置里 default_username=admin）", payload["username"])
	}
	// 密码必须被 sanitizeUser 剥掉。
	// 注意 apiKey 是**有意返回**的（handler 显式回填，PATCH /users/me 可重置它），
	// 不属于泄露，故不在此断言之列。
	if v, ok := payload["password"]; ok && v != "" {
		t.Errorf("响应泄露密码字段: %v", v)
	}
}

// TestUsersListRoute /users 列表。需 user:read 权限，
// 测试配置 require_auth=false 下应放行。
func TestUsersListRoute(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, body := doGet(t, br, "/api/v1/users")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/users = %d, want 200；响应: %s", code, body)
	}
	// 列表形状不锁死（可能是数组或带分页的对象），只要是合法 JSON 且非空响应
	var any interface{}
	if err := json.Unmarshal(body, &any); err != nil {
		t.Fatalf("响应不是合法 JSON: %v；原始: %s", err, body)
	}
}
