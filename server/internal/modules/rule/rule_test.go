package rule

import (
	"context"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

func setupRuleModule(t *testing.T) (*Module, *app.Container) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.Config{DataDir: tmpDir, DefaultUsername: "admin"}
	cfg.InitUserMap()

	container := app.NewContainer()
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(cfg, logger)
	container.Register("store.provider", store.StoreProvider(provider))

	ctx := &app.ModuleContext{Container: container, Config: &cfg, Logger: logger}

	m := New()
	if err := m.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return m, container
}

func TestRuleModule_Interface(t *testing.T) {
	m := New()
	if m.Name() != "rule" {
		t.Errorf("Name() = %q, want %q", m.Name(), "rule")
	}
	if m.Priority() != 30 {
		t.Errorf("Priority() = %d, want 30", m.Priority())
	}
}

func TestRuleModule_Init(t *testing.T) {
	m, container := setupRuleModule(t)
	_, _ = m, container

	if _, ok := container.Get(services.KeyRuleCatalog); !ok {
		t.Error("KeyRuleCatalog not registered")
	}
	if _, ok := container.Get(services.KeyRuleExecutor); !ok {
		t.Error("KeyRuleExecutor not registered")
	}
	if _, ok := container.Get(services.KeyRuleManager); !ok {
		t.Error("KeyRuleManager not registered")
	}
	if _, ok := container.Get(services.KeyEngineManager); !ok {
		t.Error("KeyEngineManager not registered")
	}
}

func TestRuleModule_StartStop(t *testing.T) {
	m, _ := setupRuleModule(t)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestRuleModule_List_Empty(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chains, total, err := m.List("admin", "", nil, nil, "", 20, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if chains != nil && len(chains) != 0 {
		t.Errorf("chains should be empty, got %d", len(chains))
	}
}

func TestRuleModule_Get_NotFound(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := m.Get("admin", "nonexistent")
	if err == nil {
		t.Error("Get with nonexistent chain should return error")
	}
}

func TestRuleModule_Execute_ChainNotFound(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test":true}`)
	err := m.Execute("admin", "nonexistent-chain", msg)
	if err == nil {
		t.Error("Execute with nonexistent chain should return error")
	}
}

func TestRuleModule_Delete_Nonexistent(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := m.Delete("admin", "nonexistent")
	if err == nil {
		t.Error("Delete nonexistent should return error")
	}
}

func TestRuleModule_SaveBaseInfo_EmptyChainId(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := m.SaveBaseInfo("admin", "", types.RuleChainBaseInfo{})
	if err == nil {
		t.Error("SaveBaseInfo with empty chainId should return error")
	}
}

func TestRuleModule_SaveConfiguration_EmptyChainId(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := m.SaveConfiguration("admin", "", "key", "value")
	if err == nil {
		t.Error("SaveConfiguration with empty chainId should return error")
	}
}

func TestRuleModule_SaveConfiguration_ChainNotFound(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := m.SaveConfiguration("admin", "nonexistent", "key", "value")
	if err == nil {
		t.Error("SaveConfiguration with nonexistent chain should return error")
	}
}

func TestRuleModule_GetRuleConfig_NoUser(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	rc := m.GetRuleConfig("nonexistent")
	// 用户不存在时应返回空配置
	_ = rc
}

func TestRuleModule_GetSetting_NoUser(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	v := m.GetSetting("nonexistent", "some-key")
	if v != "" {
		t.Errorf("GetSetting for nonexistent user should return empty, got %q", v)
	}
}

func TestRuleModule_isSystemAgent(t *testing.T) {
	m := &Module{}
	tests := []struct {
		name string
		rc   types.RuleChain
		want bool
	}{
		{"system agent", types.RuleChain{RuleChain: types.RuleChainBaseInfo{AdditionalInfo: map[string]interface{}{constants.KeySystemAgent: true}}}, true},
		{"not system", types.RuleChain{RuleChain: types.RuleChainBaseInfo{AdditionalInfo: map[string]interface{}{constants.KeySystemAgent: false}}}, false},
		{"nil info", types.RuleChain{RuleChain: types.RuleChainBaseInfo{AdditionalInfo: nil}}, false},
		{"no key", types.RuleChain{RuleChain: types.RuleChainBaseInfo{AdditionalInfo: map[string]interface{}{"other": true}}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.isSystemAgent(tt.rc); got != tt.want {
				t.Errorf("isSystemAgent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleModule_SaveAndLoad(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chainDef := `{
		"ruleChain": {
			"id": "test-chain-1",
			"name": "Test Chain"
		},
		"metadata": {
			"nodes": [],
			"connections": []
		}
	}`

	err := m.SaveAndLoad("admin", "test-chain-1", []byte(chainDef))
	if err != nil {
		t.Fatalf("SaveAndLoad: %v", err)
	}

	// 验证能获取到
	def, err := m.Get("admin", "test-chain-1")
	if err != nil {
		t.Fatalf("Get after SaveAndLoad: %v", err)
	}
	if len(def) == 0 {
		t.Error("Get should return non-empty definition")
	}

	// 验证 List 能看到
	chains, total, err := m.List("admin", "", nil, nil, "", 20, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(chains) != 1 {
		t.Fatalf("chains count = %d, want 1", len(chains))
	}
	if chains[0].RuleChain.ID != "test-chain-1" {
		t.Errorf("chain ID = %q, want %q", chains[0].RuleChain.ID, "test-chain-1")
	}
}

func TestRuleModule_DeployUndeploy(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chainDef := `{
		"ruleChain": {"id": "deploy-test", "name": "Deploy Test"},
		"metadata": {"nodes": [], "connections": []}
	}`
	if err := m.SaveAndLoad("admin", "deploy-test", []byte(chainDef)); err != nil {
		t.Fatal(err)
	}

	// Undeploy
	if err := m.Undeploy("admin", "deploy-test"); err != nil {
		t.Fatalf("Undeploy: %v", err)
	}

	// 重新 Deploy
	if err := m.Deploy("admin", "deploy-test"); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
}

func TestRuleModule_ExecuteOnDeployedChain(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chainDef := `{
		"ruleChain": {"id": "exec-chain", "name": "Exec Chain"},
		"metadata": {"nodes": [], "connections": []}
	}`
	if err := m.SaveAndLoad("admin", "exec-chain", []byte(chainDef)); err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"hello":"world"}`)
	if err := m.Execute("admin", "exec-chain", msg); err != nil {
		t.Fatalf("Execute on deployed chain: %v", err)
	}
}

func TestRuleModule_Delete_SystemAgent(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chainDef := `{
		"ruleChain": {
			"id": "system-agent-1",
			"name": "System Agent",
			"additionalInfo": {"systemAgent": true}
		},
		"metadata": {"nodes": [], "connections": []}
	}`
	if err := m.SaveAndLoad("admin", "system-agent-1", []byte(chainDef)); err != nil {
		t.Fatal(err)
	}

	err := m.Delete("admin", "system-agent-1")
	if err == nil {
		t.Error("Delete system agent should return error")
	}
}

// 编译时接口检查
var _ services.ChainCatalog = (*Module)(nil)
var _ services.ChainExecutor = (*Module)(nil)
var _ services.RuleAdminService = (*Module)(nil)
