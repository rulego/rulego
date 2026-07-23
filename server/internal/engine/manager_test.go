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

	// Retrieving again should return the same instance
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

	// Users who don't exist
	_, ok := mgr.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent user")
	}

	// You can obtain it after creation
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
	// Stop should not panic
	mgr.Stop()
}

func TestManager_InitUserEngines(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a user directory
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

	// Users should be created in the directory
	if _, ok := mgr.Get("existing-user"); !ok {
		t.Error("should have engine for existing-user from directory")
	}
	// Users should be created in the configuration
	if _, ok := mgr.Get("config-user"); !ok {
		t.Error("should have engine for config-user from config")
	}
	// Default users should have been created
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

	// An empty chainId should return an error
	if err := ue.SetMainChainId(""); err == nil {
		t.Error("SetMainChainId('') should return error")
	}

	// A non-existent chainId should return an error (undeployed)
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

// Compile-time interface check
var _ services.UserEngine = (*UserEngine)(nil)
var _ services.EngineManager = (*Manager)(nil)

// ----------------------------------------------------------------------------
// loadRules contract testing (verifying StoreProvider abstraction is filesystem-independent)
// ----------------------------------------------------------------------------

// mockRuleStore simulates a fully custom RuleStore implementation (such as a DB backend).
// Intentionally not writing any files to disk to prove that loadRules does not depend on the file system.
type mockRuleStore struct {
	chains map[string][]byte // chainId -> DSL bytes
}

// AllChains implements batch loading methods for the RuleStore interface, returning the DSL for all chains.
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

// Construct a minimal valid rule chain DSL (optional version with SystemAgent tag)
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

// TestLoadRules_NotDependOnFilesystem is the key contract test:
// Use a mock RuleStore that does not write to disks at all to verify that loadRules correctly load all chains.
//
// If this test passes, it proves that StoreProvider abstraction is not leaking—
// Any RuleStore implementation (filestore, gorm, remote API) behaves consistently when launched.
func TestLoadRules_NotDependOnFilesystem(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{DataDir: tmpDir, DefaultUsername: "testuser"}
	logger := types.DefaultLogger()

	// Using the filestore Provider is only to access settingStore (mock does not implement setting).
	provider := filestore.NewFileStoreProvider(*cfg, logger)
	mgr := NewManager(cfg, logger, provider)

	ueIface, err := mgr.GetOrCreate("testuser")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	ue := ueIface.(*UserEngine)

	// Replace ruleStore with mock (simulates a DB backend, with no disk files)
	ue.ruleStore = &mockRuleStore{chains: map[string][]byte{
		"chain-from-db-1":    testChainDSLWithFlags("chain-from-db-1", "DB Chain 1", false),
		"chain-from-db-2":    testChainDSLWithFlags("chain-from-db-2", "DB Chain 2", false),
		"system-agent-chain": testChainDSLWithFlags("system-agent-chain", "Agent", true),
	}}

	// Call loadRules — internally run AllChains, do not touch the file system
	ue.loadRules()

	// Verification: All three chains are loaded into the engine pool (including SystemAgent; List filters but AllChains does not)
	for _, id := range []string{"chain-from-db-1", "chain-from-db-2", "system-agent-chain"} {
		if _, ok := ue.GetEngine(id); !ok {
			t.Errorf("chain %s should be in pool after loadRules", id)
		}
	}

	t.Logf("PASSED: loadRules works without filesystem via RuleStore.AllChains")
}

// TestLoadRules_EmptyStore Empty store scenarios should not panic.
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

	// There should be no panic; the engine pool should be empty
	ue.loadRules()

	if _, ok := ue.GetEngine("anything"); ok {
		t.Error("engine pool should be empty after loading from empty store")
	}
}

// errorStore wraps mockRuleStore to make AllChains return an error.
type errorStore struct {
	mockRuleStore
}

func (e *errorStore) AllChains(username string) (map[string][]byte, error) {
	return nil, fmt.Errorf("simulated DB connection failure")
}

// TestLoadRules_EnumerateError When verifying store errors, loadRules do not panic.
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

	// Don't panic, just log the error
	ue.loadRules()
}
