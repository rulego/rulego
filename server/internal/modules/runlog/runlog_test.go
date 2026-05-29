package runlog

import (
	"context"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

func TestRunlogModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "runlog" {
		t.Errorf("Name() = %q, want %q", m.Name(), "runlog")
	}
	if m.Priority() != 45 {
		t.Errorf("Priority() = %d, want 45", m.Priority())
	}
}

func TestRunlogModuleInitRegistersService(t *testing.T) {
	tmpDir := t.TempDir()
	m := New()
	container := app.NewContainer()
	cfg := config.Config{DataDir: tmpDir}
	container.Register("core.config", &cfg)
	logger := types.DefaultLogger()
	container.Register("core.logger", logger)
	provider := filestore.NewFileStoreProvider(cfg, nil)
	provider.SetRunLogStore(nopstore.NopRunLogStore{})
	container.Register("store.provider", store.StoreProvider(provider))

	ctx := &app.ModuleContext{Container: container, Config: &cfg, Logger: logger}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if _, ok := container.Get(services.KeyRunLogService); !ok {
		t.Error("module.runlog.service not registered")
	}
}

func TestRunlogModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunlogServiceImplListEmpty(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	events, total, err := svc.List("admin", "", time.Time{}, time.Time{}, 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0 for empty store", total)
	}
	if events != nil {
		t.Errorf("events should be nil for nop store")
	}
}

func TestRunlogServiceImplGetNonexistent(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	_, err := svc.Get("admin", "nonexistent")
	// NopRunLogStore returns zero value, no error
	if err != nil {
		t.Logf("Get() on nonexistent returned error (acceptable): %v", err)
	}
}

func TestRunlogServiceImplDeleteNonexistent(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	err := svc.Delete("admin", "nonexistent")
	if err != nil {
		t.Fatalf("NopRunLogStore.Delete should return nil, got: %v", err)
	}
}

// Compile-time interface check
var _ services.RunLogService = (*runLogServiceImpl)(nil)
