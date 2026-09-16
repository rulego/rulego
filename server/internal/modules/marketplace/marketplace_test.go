package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
)

func TestMarketplaceModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "marketplace" {
		t.Errorf("Name() = %q, want %q", m.Name(), "marketplace")
	}
	if m.Priority() != 70 {
		t.Errorf("Priority() = %d, want 70", m.Priority())
	}
}

func TestMarketplaceModuleInit(t *testing.T) {
	m := New()
	container := app.NewContainer()
	cfg := config.DefaultConfig()
	container.Register("core.config", &cfg)

	ctx := &app.ModuleContext{Container: container}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if _, ok := container.Get(services.KeyMarketplaceService); !ok {
		t.Error("module.marketplace.service not registered")
	}
}

func TestMarketplaceModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMarketplaceEmptyBaseUrl(t *testing.T) {
	m := &Module{cfg: &config.Config{}}

	components, err := m.GetComponents("", 1, 20)
	if err != nil {
		t.Errorf("GetComponents with empty URL should not error, got %v", err)
	}
	if components == nil || len(components.Items) != 0 {
		t.Errorf("GetComponents with empty URL should return empty items, got %v", components)
	}

	chains, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Errorf("GetChains with empty URL should not error, got %v", err)
	}
	if chains == nil || len(chains.Items) != 0 {
		t.Errorf("GetChains with empty URL should return empty items, got %v", chains)
	}
}

func TestMarketplaceGetComponents_ArrayResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []interface{}{
			map[string]interface{}{"name": "comp1", "type": "action"},
			map[string]interface{}{"name": "comp2", "type": "filter"},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{MarketplaceBaseUrl: server.URL}}

	result, err := m.GetComponents("", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Errorf("GetComponents returned %d items, want 2", len(result.Items))
	}
	if result.Total != 2 {
		t.Errorf("GetComponents total = %d, want 2", result.Total)
	}
}

func TestMarketplaceGetComponents_PaginatedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证查询参数被透传
		if r.URL.Query().Get("keywords") != "test" {
			t.Errorf("keywords param not passed, got %q", r.URL.Query().Get("keywords"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page param not passed, got %q", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("size") != "10" {
			t.Errorf("size param not passed, got %q", r.URL.Query().Get("size"))
		}

		data := map[string]interface{}{
			"total": 50,
			"page":  2,
			"size":  10,
			"items": []interface{}{
				map[string]interface{}{"name": "comp1"},
				map[string]interface{}{"name": "comp2"},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{MarketplaceBaseUrl: server.URL}}

	result, err := m.GetComponents("test", 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Errorf("GetComponents returned %d items, want 2", len(result.Items))
	}
	if result.Total != 50 {
		t.Errorf("GetComponents total = %d, want 50", result.Total)
	}
	if result.Page != 2 {
		t.Errorf("GetComponents page = %d, want 2", result.Page)
	}
	if result.Size != 10 {
		t.Errorf("GetComponents size = %d, want 10", result.Size)
	}
}

func TestMarketplaceGetComponents_DataKeyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{"name": "comp1"},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{MarketplaceBaseUrl: server.URL}}

	result, err := m.GetComponents("", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Errorf("GetComponents returned %d items, want 1", len(result.Items))
	}
}

func TestMarketplaceGetChains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []interface{}{
			map[string]interface{}{"id": "chain1"},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{MarketplaceBaseUrl: server.URL}}

	result, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Errorf("GetChains returned %d items, want 1", len(result.Items))
	}
}

func TestMarketplaceGetChains_WithRoot(t *testing.T) {
	root := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("root") != "true" {
			t.Errorf("root param not passed, got %q", r.URL.Query().Get("root"))
		}
		data := []interface{}{
			map[string]interface{}{"id": "chain1"},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{MarketplaceBaseUrl: server.URL}}

	result, err := m.GetChains(&root, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Errorf("GetChains returned %d items, want 1", len(result.Items))
	}
}

// chainDSL 生成一条最小可用的规则链 DSL JSON
func chainDSL(id, name string, root bool, description, category string, tags ...string) string {
	additionalInfo := map[string]interface{}{}
	if description != "" {
		additionalInfo["description"] = description
	}
	if category != "" {
		additionalInfo["category"] = category
	}
	if len(tags) > 0 {
		additionalInfo["tags"] = tags
	}
	data, _ := json.Marshal(map[string]interface{}{
		"ruleChain": map[string]interface{}{
			"id":             id,
			"name":           name,
			"root":           root,
			"disabled":       false,
			"additionalInfo": additionalInfo,
		},
		"metadata": map[string]interface{}{},
	})
	return string(data)
}

func writeMarketChain(t *testing.T, dir, subDir, fileName, content string) {
	t.Helper()
	target := filepath.Join(dir, subDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, fileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupLocalMarket 创建含 2 条根链 + 2 条子链的本地市场目录，
// 其中 misfiled.json 故意放错目录（chains/ 下放 root=false），用于验证按 DSL 字段而非目录名过滤
func setupLocalMarket(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeMarketChain(t, dir, "chains", "a.json", chainDSL("c1", "Temperature Monitor", true, "monitor temperature data", "iot", "modbus", "sensor"))
	writeMarketChain(t, dir, "chains", "b.json", chainDSL("c2", "Device Control", true, "switch devices on", "control", "actuator"))
	writeMarketChain(t, dir, "chains", "misfiled.json", chainDSL("c3", "Misfiled Sub", false, "belongs to sub", "control"))
	writeMarketChain(t, dir, "sub-chains", "c.json", chainDSL("c4", "Sub Flow", false, "child chain", "control", "child"))
	return dir
}

func TestMarketplaceGetChains_LocalDir(t *testing.T) {
	m := &Module{cfg: &config.Config{MarketplaceLocalDir: setupLocalMarket(t)}}

	result, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 4 {
		t.Errorf("all chains: total = %d, items = %d, want 4/4", result.Total, len(result.Items))
	}
	if result.Page != 1 || result.Size != 20 {
		t.Errorf("page/size = %d/%d, want 1/20", result.Page, result.Size)
	}

	rootTrue, rootFalse := true, false
	result, err = m.GetChains(&rootTrue, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Errorf("root=true: total = %d, items = %d, want 2/2", result.Total, len(result.Items))
	}

	result, err = m.GetChains(&rootFalse, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Errorf("root=false: total = %d, items = %d, want 2/2", result.Total, len(result.Items))
	}
}

func TestMarketplaceGetChains_LocalDirKeywords(t *testing.T) {
	m := &Module{cfg: &config.Config{MarketplaceLocalDir: setupLocalMarket(t)}}

	cases := []struct {
		name     string
		keywords string
		want     int
	}{
		{"name hit case-insensitive", "TEMPERATURE", 1},
		{"description hit", "switch", 1},
		{"tag hit", "modbus", 1},
		{"category hit", "iot", 1},
		{"no hit", "missing", 0},
	}
	for _, tc := range cases {
		result, err := m.GetChains(nil, tc.keywords, 1, 20)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if result.Total != tc.want {
			t.Errorf("%s: total = %d, want %d", tc.name, result.Total, tc.want)
		}
	}
}

func TestMarketplaceGetChains_LocalDirPagination(t *testing.T) {
	m := &Module{cfg: &config.Config{MarketplaceLocalDir: setupLocalMarket(t)}}

	result, err := m.GetChains(nil, "", 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 3 {
		t.Errorf("page 1: total = %d, items = %d, want 4/3", result.Total, len(result.Items))
	}

	result, err = m.GetChains(nil, "", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 1 || result.Page != 2 || result.Size != 3 {
		t.Errorf("page 2: total = %d, items = %d, page = %d, size = %d, want 4/1/2/3",
			result.Total, len(result.Items), result.Page, result.Size)
	}

	result, err = m.GetChains(nil, "", 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 0 {
		t.Errorf("page 3: total = %d, items = %d, want 4/0", result.Total, len(result.Items))
	}
}

func TestMarketplaceGetChains_LocalDirMissing(t *testing.T) {
	m := &Module{cfg: &config.Config{MarketplaceLocalDir: filepath.Join(t.TempDir(), "not-exist")}}

	result, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if result == nil || len(result.Items) != 0 || result.Total != 0 {
		t.Errorf("missing dir should return empty result, got %+v", result)
	}
}

func TestMarketplaceGetChains_LocalDirBadJsonSkipped(t *testing.T) {
	dir := t.TempDir()
	writeMarketChain(t, dir, "chains", "bad.json", "{not valid json")
	writeMarketChain(t, dir, "chains", "good.json", chainDSL("c1", "Good Chain", true, "", ""))
	writeMarketChain(t, dir, "chains", "readme.txt", "hello")

	m := &Module{cfg: &config.Config{MarketplaceLocalDir: dir}}

	result, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Errorf("bad json should be skipped: total = %d, items = %d, want 1/1", result.Total, len(result.Items))
	}
}

func TestMarketplaceGetChains_LocalDirRemotePriority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := []interface{}{map[string]interface{}{"id": "remote-1"}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	m := &Module{cfg: &config.Config{
		MarketplaceBaseUrl:   server.URL,
		MarketplaceLocalDir:  setupLocalMarket(t),
	}}

	result, err := m.GetChains(nil, "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("remote should take priority: total = %d, want 1", result.Total)
	}
	if item, ok := result.Items[0].(map[string]interface{}); !ok || item["id"] != "remote-1" {
		t.Errorf("expected remote item, got %v", result.Items[0])
	}
}
