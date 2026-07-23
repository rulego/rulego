package rule

import (
	"context"
	"encoding/json"
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
	// If the user does not exist, return an empty configuration
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

	// Verification can be obtained
	def, err := m.Get("admin", "test-chain-1")
	if err != nil {
		t.Fatalf("Get after SaveAndLoad: %v", err)
	}
	if len(def) == 0 {
		t.Error("Get should return non-empty definition")
	}

	// You can see it by checking the verification list
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

	// Redeploy
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

// TestRuleModule_SaveAndLoad_StripsSystemAgentFromUserNamespace Verify that it is not a DefaultUsername
// Namespace callers cannot inject systemAgent tags via SaveAndLoad (to prevent disguising the undeleteable chain).
func TestRuleModule_SaveAndLoad_StripsSystemAgentFromUserNamespace(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Simulates a regular user namespace (DefaultUsername="admin", distinguished by "tenant-1")
	chainDef := `{
		"ruleChain": {
			"id": "poison-chain",
			"name": "Poison",
			"additionalInfo": {"systemAgent": true}
		},
		"metadata": {"nodes": [], "connections": []}
	}`
	if err := m.SaveAndLoad("tenant-1", "poison-chain", []byte(chainDef)); err != nil {
		t.Fatalf("SaveAndLoad: %v", err)
	}

	// After saving, read the definition: the systemAgent tag should have been stripped off
	raw, err := m.Get("tenant-1", "poison-chain")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var def types.RuleChain
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := def.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
		t.Error("systemAgent marker should be stripped for non-default namespace")
	}

	// After separation, the chain should be deleted (not a system agent)
	if err := m.Delete("tenant-1", "poison-chain"); err != nil {
		t.Errorf("Delete should succeed after stripping systemAgent, got: %v", err)
	}
}

// capturingListener records received events and asserts whether Save/Deploy/Undeploy/Delete triggers the listener
type capturingListener struct {
	savedEvents      []services.ChainLifecycleEvent
	deployedEvents   []services.ChainLifecycleEvent
	undeployedEvents []services.ChainLifecycleEvent
	deletedEvents    []services.ChainLifecycleEvent
}

func (c *capturingListener) OnSaved(e services.ChainLifecycleEvent) {
	c.savedEvents = append(c.savedEvents, e)
}
func (c *capturingListener) OnDeployed(e services.ChainLifecycleEvent) {
	c.deployedEvents = append(c.deployedEvents, e)
}
func (c *capturingListener) OnUndeployed(e services.ChainLifecycleEvent) {
	c.undeployedEvents = append(c.undeployedEvents, e)
}
func (c *capturingListener) OnDeleted(e services.ChainLifecycleEvent) {
	c.deletedEvents = append(c.deletedEvents, e)
}

// TestRuleModule_LifecycleListener_AllEvents Verify that all 4 events are triggered correctly
func TestRuleModule_LifecycleListener_AllEvents(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	listener := &capturingListener{}
	m.AddLifecycleListener(listener)

	chainDef := `{
		"ruleChain": {"id": "lifecycle-test", "name": "Lifecycle Test"},
		"metadata": {"nodes": [], "connections": []}
	}`
	// SaveAndLoad → OnSaved + OnDeployed
	if err := m.SaveAndLoad("admin", "lifecycle-test", []byte(chainDef)); err != nil {
		t.Fatal(err)
	}
	if len(listener.savedEvents) != 1 {
		t.Errorf("after SaveAndLoad, expected 1 saved event, got %d", len(listener.savedEvents))
	}
	if len(listener.deployedEvents) != 1 {
		t.Errorf("after SaveAndLoad, expected 1 deployed event, got %d", len(listener.deployedEvents))
	} else {
		e := listener.deployedEvents[0]
		if e.ChainId != "lifecycle-test" || e.Username != "admin" {
			t.Errorf("deployed event mismatch: %+v", e)
		}
	}

	// Undeploy → OnUndeployed
	if err := m.Undeploy("admin", "lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	if len(listener.undeployedEvents) != 1 {
		t.Errorf("after Undeploy, expected 1 undeployed event, got %d", len(listener.undeployedEvents))
	}

	// Deploy → OnDeployed again
	if err := m.Deploy("admin", "lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	if len(listener.deployedEvents) != 2 {
		t.Errorf("after second Deploy, expected 2 deployed events, got %d", len(listener.deployedEvents))
	}

	// Delete → OnDeleted
	if err := m.Delete("admin", "lifecycle-test"); err != nil {
		t.Fatal(err)
	}
	if len(listener.deletedEvents) != 1 {
		t.Errorf("after Delete, expected 1 deleted event, got %d", len(listener.deletedEvents))
	}
}

// TestRuleModule_LifecycleListener_BaseDefault Verify that the default implementation of BaseChainLifecycleListener is available
// (Listeners only care about OnDeleted; others are implemented by default null in Base)
type deleteOnlyListener struct {
	services.BaseChainLifecycleListener
	deletedCount int
}

func (d *deleteOnlyListener) OnDeleted(services.ChainLifecycleEvent) { d.deletedCount++ }

func TestRuleModule_LifecycleListener_BaseDefault(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	listener := &deleteOnlyListener{}
	m.AddLifecycleListener(listener)

	chainDef := `{
		"ruleChain": {"id": "base-test", "name": "Base Test"},
		"metadata": {"nodes": [], "connections": []}
	}`
	if err := m.SaveAndLoad("admin", "base-test", []byte(chainDef)); err != nil {
		t.Fatal(err)
	}
	// SaveAndLoad should not trigger OnDeleted
	if listener.deletedCount != 0 {
		t.Errorf("deleteOnlyListener should not fire on SaveAndLoad, got %d", listener.deletedCount)
	}

	if err := m.Delete("admin", "base-test"); err != nil {
		t.Fatal(err)
	}
	if listener.deletedCount != 1 {
		t.Errorf("deleteOnlyListener should fire on Delete, got %d", listener.deletedCount)
	}
}

// TestRuleModule_LifecycleListener_PanicSafe Verify that the listener panic does not interrupt Deploy
type panickingListener struct {
	services.BaseChainLifecycleListener
}

func (p *panickingListener) OnDeployed(services.ChainLifecycleEvent) { panic("intentional") }
func (p *panickingListener) OnSaved(services.ChainLifecycleEvent)    { panic("intentional") }

func TestRuleModule_LifecycleListener_PanicSafe(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	m.AddLifecycleListener(&panickingListener{})

	chainDef := `{
		"ruleChain": {"id": "panic-test", "name": "Panic Test"},
		"metadata": {"nodes": [], "connections": []}
	}`
	// SaveAndLoad should succeed, even though both OnSaved/OnDeployed panic
	if err := m.SaveAndLoad("admin", "panic-test", []byte(chainDef)); err != nil {
		t.Fatalf("SaveAndLoad should succeed despite listener panic, got: %v", err)
	}
}

// TestRuleModule_CategoryFilter Verify that category filtering is working properly
// (category is persisted via AdditionalInfo["category"], and the filestore supports filtering)
func TestRuleModule_CategoryFilter(t *testing.T) {
	m, _ := setupRuleModule(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	chains := []struct {
		id       string
		category string
	}{
		{"chain-timer-1", "timer"},
		{"chain-timer-2", "timer"},
		{"chain-api-1", "api"},
		{"chain-mqtt-1", "mqtt"},
	}
	for _, c := range chains {
		def := `{
			"ruleChain": {
				"id": "` + c.id + `",
				"name": "` + c.id + `",
				"additionalInfo": {"category": "` + c.category + `"}
			},
			"metadata": {"nodes": [], "connections": []}
		}`
		if err := m.SaveAndLoad("admin", c.id, []byte(def)); err != nil {
			t.Fatalf("SaveAndLoad %s: %v", c.id, err)
		}
	}

	// Filter timer categories
	timerChains, total, err := m.List("admin", "", nil, nil, "timer", 0, 0)
	if err != nil {
		t.Fatalf("List timer: %v", err)
	}
	if total != 2 {
		t.Errorf("timer category total = %d, want 2", total)
	}
	for _, c := range timerChains {
		cat, _ := c.RuleChain.GetAdditionalInfo(constants.KeyCategory)
		if cat != "timer" {
			t.Errorf("returned chain %s has category %v, want timer", c.RuleChain.ID, cat)
		}
	}

	// Filter API categories
	_, apiTotal, err := m.List("admin", "", nil, nil, "api", 0, 0)
	if err != nil {
		t.Fatalf("List api: %v", err)
	}
	if apiTotal != 1 {
		t.Errorf("api category total = %d, want 1", apiTotal)
	}

	// No filtering
	_, allTotal, err := m.List("admin", "", nil, nil, "", 0, 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if allTotal != 4 {
		t.Errorf("all category total = %d, want 4", allTotal)
	}
}

// Compile-time interface check
var _ services.ChainCatalog = (*Module)(nil)
var _ services.ChainExecutor = (*Module)(nil)
var _ services.RuleAdminService = (*Module)(nil)
