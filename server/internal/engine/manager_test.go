package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/model"
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
	// existing-user：有目录且在 UserStore 里有记录，应被初始化。
	userDir := filepath.Join(tmpDir, constants.DirWorkflows, "existing-user")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	// orphan-user：只有目录、无用户记录（模拟已删用户 purge=false 残留），应跳过。
	orphanDir := filepath.Join(tmpDir, constants.DirWorkflows, "orphan-user")
	if err := os.MkdirAll(orphanDir, 0755); err != nil {
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
	// 把 existing-user 写进 UserStore，使其成为有效用户（或孤儿目录的判据）。
	if us, err := provider.GetUserStore(); err == nil {
		if err := us.CreateUser(model.User{Username: "existing-user", Password: "x", Roles: []string{model.RoleEditor}}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	} else {
		t.Fatalf("GetUserStore: %v", err)
	}
	mgr := NewManager(cfg, logger, provider)

	if err := mgr.InitUserEngines(); err != nil {
		t.Fatalf("InitUserEngines: %v", err)
	}

	// 有目录且有用户记录：应初始化
	if _, ok := mgr.Get("existing-user"); !ok {
		t.Error("should have engine for existing-user (dir + user store record)")
	}
	// 只有目录无用户记录：应跳过，不为已删用户复活引擎
	if _, ok := mgr.Get("orphan-user"); ok {
		t.Error("orphan-user should be skipped (dir without user record)")
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

// ----------------------------------------------------------------------------
// loadRules 契约测试（验证 StoreProvider 抽象不依赖文件系统）
// ----------------------------------------------------------------------------

// mockRuleStore 模拟一个完全自定义的 RuleStore 实现（比如 DB 后端）。
// 故意不写任何文件到磁盘，用于证明 loadRules 不依赖文件系统。
type mockRuleStore struct {
	chains map[string][]byte // chainId -> DSL bytes
}

// AllChains 实现 RuleStore 接口的批量加载方法，返回全部链的 DSL。
func (m *mockRuleStore) AllChains(username string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(m.chains))
	for id, def := range m.chains {
		out[id] = def
	}
	return out, nil
}

func (m *mockRuleStore) Save(username, chainId string, def []byte) error {
	m.chains[chainId] = def
	return nil
}

func (m *mockRuleStore) Get(username, chainId string) ([]byte, error) {
	if d, ok := m.chains[chainId]; ok {
		return d, nil
	}
	return nil, nil
}

func (m *mockRuleStore) GetAsRuleChain(username, chainId string) (types.RuleChain, error) {
	return types.RuleChain{}, nil
}

func (m *mockRuleStore) List(username string, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error) {
	return nil, 0, nil
}

func (m *mockRuleStore) Delete(username, chainId string) error {
	delete(m.chains, chainId)
	return nil
}

// 构造一个最小的合法规则链 DSL（含 SystemAgent 标记的版本可选）
func testChainDSLWithFlags(chainId, name string, systemAgent bool) []byte {
	sysAgent := "false"
	if systemAgent {
		sysAgent = "true"
	}
	return []byte(`{
		"ruleChain": {
			"id": "` + chainId + `",
			"name": "` + name + `",
			"root": false,
			"debugMode": false,
			"additionalInfo": {
				"description": "test chain",
				"systemAgent": ` + sysAgent + `
			}
		},
		"metadata": {
			"firstNodeIndex": null,
			"nodes": [],
			"connections": [],
			"ruleChainConnections": []
		}
	}`)
}

// TestLoadRules_NotDependOnFilesystem 是关键契约测试：
// 用一个完全不写磁盘的 mock RuleStore，验证 loadRules 能正确加载所有链。
//
// 这条测试如果通过，就证明了 StoreProvider 抽象没有泄漏——
// 任何 RuleStore 实现（filestore、gorm、远程 API）启动时行为一致。
func TestLoadRules_NotDependOnFilesystem(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{DataDir: tmpDir, DefaultUsername: "testuser"}
	logger := types.DefaultLogger()

	// 用 filestore Provider 只是为了拿 settingStore（mock 不实现 setting）
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)

	ueIface, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	ue := ueIface.(*UserEngine)

	// 替换 ruleStore 为 mock（模拟 DB 后端，没有任何磁盘文件）
	ue.ruleStore = &mockRuleStore{chains: map[string][]byte{
		"chain-from-db-1":    testChainDSLWithFlags("chain-from-db-1", "DB Chain 1", false),
		"chain-from-db-2":    testChainDSLWithFlags("chain-from-db-2", "DB Chain 2", false),
		"system-agent-chain": testChainDSLWithFlags("system-agent-chain", "Agent", true),
	}}

	// 调用 loadRules —— 内部走 AllChains，不碰文件系统
	ue.loadRules()

	// 验证：三条链都加载到了引擎池（含 SystemAgent，List 会过滤但 AllChains 不会）
	for _, id := range []string{"chain-from-db-1", "chain-from-db-2", "system-agent-chain"} {
		if _, ok := ue.GetEngine(id); !ok {
			t.Errorf("chain %s should be in pool after loadRules", id)
		}
	}

	t.Logf("PASSED: loadRules works without filesystem via RuleStore.AllChains")
}

// TestLoadRules_EmptyStore 空 store 场景不应 panic。
func TestLoadRules_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{DataDir: tmpDir, DefaultUsername: "testuser"}
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)

	ueIface, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	ue := ueIface.(*UserEngine)
	ue.ruleStore = &mockRuleStore{chains: map[string][]byte{}}

	// 不应 panic，引擎池应为空
	ue.loadRules()

	if _, ok := ue.GetEngine("anything"); ok {
		t.Error("engine pool should be empty after loading from empty store")
	}
}

// errorStore 包装 mockRuleStore，让 AllChains 返回错误。
type errorStore struct {
	mockRuleStore
}

func (e *errorStore) AllChains(username string) (map[string][]byte, error) {
	return nil, fmt.Errorf("simulated DB connection failure")
}

// TestLoadRules_EnumerateError 验证 store 报错时 loadRules 不会 panic。
func TestLoadRules_EnumerateError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{DataDir: tmpDir, DefaultUsername: "testuser"}
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)

	ueIface, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	ue := ueIface.(*UserEngine)
	ue.ruleStore = &errorStore{}

	// 不应 panic，只是记录错误日志
	ue.loadRules()
}
