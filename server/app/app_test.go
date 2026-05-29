package app

import (
	"context"
	"testing"

	"github.com/rulego/rulego/server/config"
)

func TestNewApp(t *testing.T) {
	app := New()
	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.Container() == nil {
		t.Error("Container() should not be nil")
	}
}

func TestNewAppWithConfig(t *testing.T) {
	app := New(WithConfigFile("nonexistent.conf"))
	if app == nil {
		t.Fatal("New() returned nil")
	}
}

func TestContainer(t *testing.T) {
	app := New()
	container := app.Container()

	container.Register("test.key", "test-value")
	val, err := GetAs[string](container, "test.key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test-value" {
		t.Errorf("GetAs = %q, want test-value", val)
	}
}

func TestContainerGetAsWrongType(t *testing.T) {
	app := New()
	container := app.Container()

	container.Register("test.key", "string-value")
	_, err := GetAs[int](container, "test.key")
	if err == nil {
		t.Error("GetAs with wrong type should return error")
	}
}

func TestModuleInterface(t *testing.T) {
	m := &testModule{name: "test", priority: 10}
	if m.Name() != "test" {
		t.Errorf("Name = %q, want test", m.Name())
	}
	if m.Priority() != 10 {
		t.Errorf("Priority = %d, want 10", m.Priority())
	}
}

type testModule struct {
	name     string
	priority int
}

func (m *testModule) Name() string                    { return m.name }
func (m *testModule) Priority() int                   { return m.priority }
func (m *testModule) Init(ctx *ModuleContext) error    { return nil }
func (m *testModule) Start(_ context.Context) error   { return nil }
func (m *testModule) Stop(_ context.Context) error    { return nil }

func TestAppInitWithDefaultConfig(t *testing.T) {
	app := New(WithModules(&testModule{name: "test", priority: 10}))
	if err := app.Init(); err != nil {
		t.Fatal(err)
	}
	cfg := app.Config()
	if cfg == nil {
		t.Fatal("Config() should not be nil after Init()")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want ./data", cfg.DataDir)
	}

	// Verify core services in container
	container := app.Container()
	if _, err := GetAs[*config.Config](container, "core.config"); err != nil {
		t.Errorf("core.config not found: %v", err)
	}
}

func TestAppStopWithoutStart(t *testing.T) {
	app := New(WithModules(&testModule{name: "test", priority: 10}))
	if err := app.Init(); err != nil {
		t.Fatal(err)
	}
	// Stop should work even without Start
	if err := app.Stop(); err != nil {
		t.Fatal(err)
	}
}
