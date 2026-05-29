package bootstrap

import (
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/store"
)

// initAppWithStores creates a default app, inits it, then registers file stores
// and re-runs module init. This matches the production flow in Run().
func initAppWithStores(t *testing.T) *app.App {
	t.Helper()

	cfg := config.DefaultConfig()

	provider := filestore.NewFileStoreProvider(cfg, nil)
	provider.SetRunLogStore(nopstore.NopRunLogStore{})

	application := DefaultApp("")

	_ = application.Container().Register("store.provider", store.StoreProvider(provider))
	userStore, err := provider.GetUserStore()
	if err != nil {
		t.Fatal(err)
	}
	_ = application.Container().Register("store.user", store.UserStore(userStore))

	if err := application.Init(); err != nil {
		t.Fatal(err)
	}

	return application
}

func TestDefaultModules(t *testing.T) {
	modules := DefaultModules()
	if len(modules) != 9 {
		t.Errorf("DefaultModules count = %d, want 9", len(modules))
	}

	names := map[string]bool{}
	for _, m := range modules {
		names[m.Name()] = true
	}
	for _, name := range []string{"user", "rule", "node", "runlog", "locale", "skill", "system", "marketplace", "mcp"} {
		if !names[name] {
			t.Errorf("missing module: %s", name)
		}
	}
}

func TestDefaultModulesPriorities(t *testing.T) {
	modules := DefaultModules()
	priorities := map[string]int{}
	for _, m := range modules {
		priorities[m.Name()] = m.Priority()
	}

	if priorities["user"] >= priorities["rule"] {
		t.Error("user should have lower priority than rule")
	}
	if priorities["rule"] >= priorities["node"] {
		t.Error("rule should have lower priority than node")
	}
}

func TestDefaultApp(t *testing.T) {
	application := DefaultApp("nonexistent.conf")
	if application == nil {
		t.Fatal("DefaultApp returned nil")
	}
}

func TestDefaultAppInit(t *testing.T) {
	application := initAppWithStores(t)

	cfg := application.Config()
	if cfg == nil {
		t.Fatal("Config should not be nil after Init")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}

	container := application.Container()
	services := []string{
		"module.user.auth",
		"module.rule.catalog",
		"module.rule.executor",
		"module.rule.manager",
		"module.runlog.service",
		"module.locale.service",
		"module.skill.service",
		"module.debug.service",
		"module.system.settings",
		"module.marketplace.service",
		"module.mcp.service",
	}
	for _, svc := range services {
		if _, err := app.GetAs[interface{}](container, svc); err != nil {
			t.Errorf("service %q not found in container: %v", svc, err)
		}
	}
}

func TestDefaultAppStartStop(t *testing.T) {
	application := initAppWithStores(t)
	if err := application.Start(); err != nil {
		t.Fatal(err)
	}
	if err := application.Stop(); err != nil {
		t.Fatal(err)
	}
}
