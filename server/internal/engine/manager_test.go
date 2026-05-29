package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/services"
)

func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{DataDir: tmpDir, DefaultUsername: "admin"}
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)
	return mgr, tmpDir
}

func TestNewManager(t *testing.T) {
	mgr, _ := setupTestManager(t)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(mgr.pool) != 0 {
		t.Errorf("new manager should have empty pool, got %d", len(mgr.pool))
	}
}

func TestManager_GetOrCreate(t *testing.T) {
	mgr, _ := setupTestManager(t)

	ue, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if ue.Username() != "testuser" {
		t.Errorf("Username() = %q, want %q", ue.Username(), "testuser")
	}
	if ue.Pool() == nil {
		t.Error("Pool() should not be nil")
	}
	if ue.RuleStore() == nil {
		t.Error("RuleStore() should not be nil")
	}

	// 再次获取应该返回同一实例
	ue2, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate second call: %v", err)
	}
	if ue2 != ue {
		t.Error("GetOrCreate should return same instance for same username")
	}
}

func TestManager_Get(t *testing.T) {
	mgr, _ := setupTestManager(t)

	// 不存在的用户
	_, ok := mgr.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent user")
	}

	// 创建后可以获取
	_, err := mgr.GetOrCreate("user1")
	if err != nil {
		t.Fatal(err)
	}
	_, ok = mgr.Get("user1")
	if !ok {
		t.Error("Get should return true for existing user")
	}
}

func TestManager_GetOrCreate_Concurrent(t *testing.T) {
	mgr, _ := setupTestManager(t)
	done := make(chan services.UserEngine, 10)

	for i := 0; i < 10; i++ {
		go func() {
			ue, err := mgr.GetOrCreate("concurrent-user")
			if err != nil {
				t.Errorf("concurrent GetOrCreate: %v", err)
				done <- nil
				return
			}
			done <- ue
		}()
	}

	var first services.UserEngine
	for i := 0; i < 10; i++ {
		ue := <-done
		if ue == nil {
			continue
		}
		if first == nil {
			first = ue
		} else if ue != first {
			t.Error("concurrent GetOrCreate returned different instances")
		}
	}
}

func TestManager_Stop(t *testing.T) {
	mgr, _ := setupTestManager(t)
	_, err := mgr.GetOrCreate("user1")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 不应该 panic
	mgr.Stop()
}

func TestManager_InitUserEngines(t *testing.T) {
	tmpDir := t.TempDir()
	// 创建用户目录
	userDir := filepath.Join(tmpDir, constants.DirWorkflows, "existing-user")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		DataDir:         tmpDir,
		DefaultUsername: "admin",
		Users: types.Properties{
			"config-user": "password",
		},
	}
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)

	if err := mgr.InitUserEngines(); err != nil {
		t.Fatalf("InitUserEngines: %v", err)
	}

	// 应该创建了目录中的用户
	if _, ok := mgr.Get("existing-user"); !ok {
		t.Error("should have engine for existing-user from directory")
	}
	// 应该创建了配置中的用户
	if _, ok := mgr.Get("config-user"); !ok {
		t.Error("should have engine for config-user from config")
	}
	// 应该创建了默认用户
	if _, ok := mgr.Get("admin"); !ok {
		t.Error("should have engine for default admin user")
	}
}

func TestUserEngine_SetMainChainId(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ue, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatal(err)
	}

	// 空的 chainId 应该返回错误
	if err := ue.SetMainChainId(""); err == nil {
		t.Error("SetMainChainId('') should return error")
	}

	// 不存在的 chainId 应该返回错误（未部署）
	if err := ue.SetMainChainId("nonexistent-chain"); err == nil {
		t.Error("SetMainChainId with undeployed chain should return error")
	}
}

func TestUserEngine_SaveAndGetSetting(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ue, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatal(err)
	}

	if err := ue.SaveSetting("test-key", "test-value"); err != nil {
		t.Fatalf("SaveSetting: %v", err)
	}
	if v := ue.GetSetting("test-key"); v != "test-value" {
		t.Errorf("GetSetting = %q, want %q", v, "test-value")
	}
}

func TestUserEngine_GetEngine_NotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ue, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := ue.GetEngine("nonexistent-chain"); ok {
		t.Error("GetEngine should return false for nonexistent chain")
	}
}

// 编译时接口检查
var _ services.UserEngine = (*UserEngine)(nil)
var _ services.EngineManager = (*Manager)(nil)
