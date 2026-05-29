package user

import (
	"context"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

func TestUserModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "user" {
		t.Errorf("Name() = %q, want %q", m.Name(), "user")
	}
	if m.Priority() != 10 {
		t.Errorf("Priority() = %d, want 10", m.Priority())
	}
}

func TestUserAuthCheckPassword(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.InitUserMap()

	svc := &authService{cfg: &cfg}

	if !svc.CheckPassword("admin", "admin") {
		t.Error("default password check should succeed")
	}
	if svc.CheckPassword("admin", "wrong") {
		t.Error("wrong password should fail")
	}
	if svc.CheckPassword("", "admin") {
		t.Error("empty username should fail")
	}
}

func TestUserAuthApiKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["testuser"] = "testpass,my-api-key"
	cfg.InitUserMap()

	svc := &authService{cfg: &cfg}

	username := svc.GetUsernameByApiKey("my-api-key")
	if username != "testuser" {
		t.Errorf("GetUsernameByApiKey = %q, want %q", username, "testuser")
	}

	username = svc.GetUsernameByApiKey("wrong-key")
	if username != "" {
		t.Errorf("GetUsernameByApiKey with wrong key = %q, want empty", username)
	}

	username = svc.GetUsernameByApiKey("")
	if username != "" {
		t.Errorf("GetUsernameByApiKey with empty key = %q, want empty", username)
	}
}

func TestUserAuthGetApiKeyByUsername(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Users["testuser"] = "testpass,my-api-key"
	cfg.InitUserMap()

	svc := &authService{cfg: &cfg}

	apiKey := svc.GetApiKeyByUsername("testuser")
	if apiKey != "my-api-key" {
		t.Errorf("GetApiKeyByUsername = %q, want %q", apiKey, "my-api-key")
	}

	apiKey = svc.GetApiKeyByUsername("nonexistent")
	if apiKey != "" {
		t.Errorf("GetApiKeyByUsername for nonexistent = %q, want empty", apiKey)
	}
}

func TestUserModuleInitRegistersServices(t *testing.T) {
	m := New()
	container := app.NewContainer()
	cfg := config.DefaultConfig()
	cfg.InitUserMap()
	container.Register("core.config", &cfg)
	container.Register("core.logger", nil)

	// Register a mock user store
	mockStore := &mockUserStore{}
	container.Register("store.user", store.UserStore(mockStore))

	ctx := &app.ModuleContext{Container: container}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify both services registered
	if _, ok := container.Get(services.KeyAuthService); !ok {
		t.Error("module.user.auth not registered")
	}
	if _, ok := container.Get(services.KeyUserProfile); !ok {
		t.Error("module.user.profile not registered")
	}
}

func TestUserModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// mockUserStore is a minimal mock for store.UserStore
type mockUserStore struct{}

func (m *mockUserStore) CreateUser(_ model.User) error      { return nil }
func (m *mockUserStore) ValidatePassword(_, _ string) bool  { return false }
func (m *mockUserStore) Delete(_ string) error               { return nil }
func (m *mockUserStore) List() []model.User                 { return nil }
