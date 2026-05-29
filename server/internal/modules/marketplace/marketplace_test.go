package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
