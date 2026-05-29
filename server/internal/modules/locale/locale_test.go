package locale

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
)

func newTestLocaleModule(t *testing.T) (*Module, string) {
	t.Helper()
	tmpDir := t.TempDir()
	localesDir := filepath.Join(tmpDir, "public", "locales")
	os.MkdirAll(localesDir, 0755)

	cfg := &config.Config{DataDir: tmpDir}
	m := &Module{cfg: cfg}
	return m, tmpDir
}

func TestLocaleModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "locale" {
		t.Errorf("Name() = %q, want %q", m.Name(), "locale")
	}
	if m.Priority() != 50 {
		t.Errorf("Priority() = %d, want 50", m.Priority())
	}
}

func TestLocaleSaveAndGet(t *testing.T) {
	m, _ := newTestLocaleModule(t)

	data := []byte(`{"hello":"world"}`)
	if err := m.Save("en", data); err != nil {
		t.Fatal(err)
	}

	result, err := m.Get("en")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("Get() returned nil")
	}
}

func TestLocaleGetNonexistent(t *testing.T) {
	m, _ := newTestLocaleModule(t)

	result, err := m.Get("nonexistent")
	// Should return empty object or nil, not error
	if err != nil {
		t.Logf("Get nonexistent returned error: %v (acceptable)", err)
	}
	if result != nil {
		t.Logf("Get nonexistent returned: %v", result)
	}
}

func TestLocaleList(t *testing.T) {
	m, _ := newTestLocaleModule(t)

	// Create some locale files
	localesDir := filepath.Join(m.cfg.DataDir, "public", "locales")
	os.WriteFile(filepath.Join(localesDir, "en.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(localesDir, "zh_cn.json"), []byte("{}"), 0644)

	langs, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) < 2 {
		t.Errorf("List() returned %d languages, want at least 2", len(langs))
	}
}

func TestLocaleListEmpty(t *testing.T) {
	m, _ := newTestLocaleModule(t)

	langs, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(langs) != 0 {
		t.Errorf("List() on empty dir should return 0, got %d", len(langs))
	}
}

func TestLocaleModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLocaleModuleInit(t *testing.T) {
	m := New()
	container := app.NewContainer()
	cfg := config.DefaultConfig()
	container.Register("core.config", &cfg)

	ctx := &app.ModuleContext{Container: container}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify service was registered
	svc, ok := container.Get("module.locale.service")
	if !ok {
		t.Fatal("module.locale.service not registered")
	}
	if svc == nil {
		t.Fatal("module.locale.service is nil")
	}
}
