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

// TestRuleCategoriesRoute 锁住路径名。
// /rules/categories 会让 httprouter panic（静态段与同层 :id 通配冲突），
// 故走 /category/rule 前缀（与 /rules/:id 不同层，天然不冲突）。前端若按旧路径写会 404，此用例即为防线。
func TestRuleCategoriesRoute(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, body := doGet(t, br, "/api/v1/category/rule")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/category/rule = %d, want 200；响应: %s", code, body)
	}

	// 响应形状是 {items, total}（与其他列表接口一致），不是裸数组。
	// 空库下 items 也必须存在，前端导航树直接遍历不做兜底。
	var payload struct {
		Items []string `json:"items"`
		Total *int     `json:"total"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("响应不是 {items,total} 形状: %v；原始: %s", err, body)
	}
	if payload.Items == nil {
		t.Errorf("items 字段缺失或为 null，应为数组（可空）；原始: %s", body)
	}
	if payload.Total == nil {
		t.Errorf("total 字段缺失；原始: %s", body)
	}
}

// TestRuleCategoriesOldPathIsNotRegistered 反向锁定：
// 若将来有人「修正」成 /rules/categories，这条会失败，提醒他那样注册会让服务起不来。
func TestRuleCategoriesOldPathIsNotRegistered(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, _ := doGet(t, br, "/api/v1/rules/categories")
	if code == http.StatusOK {
		t.Fatal("/api/v1/rules/categories 返回 200：该路径与 /rules/:id 通配冲突，不应存在")
	}
}

// TestOverviewRoute 总览聚合接口。响应里含 categories 字段，
// 导航树可直接复用，少一次请求——字段缺失会让前端静默拿到 undefined。
func TestOverviewRoute(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, body := doGet(t, br, "/api/v1/overview")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/overview = %d, want 200；响应: %s", code, body)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("响应不是 JSON 对象: %v；原始: %s", err, body)
	}
	if _, ok := payload["categories"]; !ok {
		t.Errorf("响应缺 categories 字段（导航树依赖它复用总览数据）；实际键: %v", keysOf(payload))
	}
}

// TestVersionRoute 版本接口，供总览与「关于」使用
func TestVersionRoute(t *testing.T) {
	br := newTestBridge(t)
	defer br.Stop()

	code, body := doGet(t, br, "/api/v1/version")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/version = %d, want 200；响应: %s", code, body)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("响应不是 JSON 对象: %v；原始: %s", err, body)
	}
	for _, k := range []string{"version", "apiVersion", "startTime"} {
		if _, ok := payload[k]; !ok {
			t.Errorf("响应缺 %q 字段；实际键: %v", k, keysOf(payload))
		}
	}
	// goVersion 是运行时指纹（可据 CVE 查攻击面），该接口只认证不鉴权，不应返回
	if _, ok := payload["goVersion"]; ok {
		t.Errorf("响应不应含 goVersion；实际键: %v", keysOf(payload))
	}
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

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
