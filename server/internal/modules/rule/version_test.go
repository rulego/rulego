package rule

import (
	"context"
	"strings"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/bboltstore"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

func setupRuleModuleWithVersions(t *testing.T) *Module {
	m, _ := setupRuleModuleVersionsOpts(t, false)
	return m
}

// setupRuleModuleVersionsCfg 在默认配置基础上改版本相关配置后再建模块
func setupRuleModuleVersionsCfg(t *testing.T, mutate func(*config.Config)) (*Module, *app.Container) {
	t.Helper()
	cfg := config.Config{DataDir: t.TempDir(), DefaultUsername: "admin",
		RuleVersionRetentionCount: 50, RuleVersionDraftRetention: 5}
	mutate(&cfg)
	cfg.InitUserMap()

	container := app.NewContainer()
	logger := types.DefaultLogger()
	provider := filestore.NewFileStoreProvider(cfg, logger)
	vs, err := bboltstore.NewRuleVersionStore(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vs.Close() })
	provider.SetRuleVersionStore(vs)
	container.Register("store.provider", store.StoreProvider(provider))

	m := New()
	if err := m.Init(&app.ModuleContext{Container: container, Config: &cfg, Logger: logger}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return m, container
}

func setupRuleModuleVersionsOpts(t *testing.T, disable bool) (*Module, *app.Container) {
	return setupRuleModuleVersionsCfg(t, func(c *config.Config) { c.RuleVersionDisable = disable })
}

func chainDefV(name string) string {
	return `{
		"ruleChain": {"id": "ver-chain", "name": "` + name + `"},
		"metadata": {
			"nodes": [{"id": "n1", "type": "jsFilter", "name": "f", "configuration": {}}],
			"connections": []
		}
	}`
}

func TestRuleVersion_SaveSnapshot(t *testing.T) {
	m := setupRuleModuleWithVersions(t)

	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	items, total, err := m.ListVersions("admin", "ver-chain", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(items))
	}
	v := items[0]
	if v.Source != "save" {
		t.Errorf("Source = %q, want save", v.Source)
	}
	if v.ChainName != "A" {
		t.Errorf("ChainName = %q, want A", v.ChainName)
	}
	if v.NodeCount != 1 {
		t.Errorf("NodeCount = %d, want 1", v.NodeCount)
	}
	if v.DslSize == 0 {
		t.Error("DslSize should be > 0")
	}
	if v.Dsl != nil {
		t.Error("ListVersions should strip Dsl")
	}
}

func TestRuleVersion_Rollback(t *testing.T) {
	m := setupRuleModuleWithVersions(t)

	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	items, _, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	v1 := items[0].Id

	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("B"))); err != nil {
		t.Fatal(err)
	}
	if def, _ := m.Get("admin", "ver-chain"); !strings.Contains(string(def), `"B"`) {
		t.Fatal("second save should take effect")
	}

	if err := m.RollbackVersion("admin", "ver-chain", v1); err != nil {
		t.Fatalf("RollbackVersion: %v", err)
	}
	def, err := m.Get("admin", "ver-chain")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(def), `"A"`) {
		t.Error("rollback should restore version A DSL")
	}

	// 回滚本身产生一个新版本，source=rollback
	items, total, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 3 {
		t.Fatalf("total = %d, want 3 (A + B + rollback)", total)
	}
	if items[0].Source != "rollback" {
		t.Errorf("newest Source = %q, want rollback", items[0].Source)
	}

	// 回滚后正常保存的 source 应回到 save（标记不残留）
	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("C"))); err != nil {
		t.Fatal(err)
	}
	items, _, _ = m.ListVersions("admin", "ver-chain", 20, 1)
	if items[0].Source != "save" {
		t.Errorf("after rollback, save Source = %q, want save", items[0].Source)
	}
}

func TestRuleVersion_RollbackNotFound(t *testing.T) {
	m := setupRuleModuleWithVersions(t)
	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A")))
	if err := m.RollbackVersion("admin", "ver-chain", "no-such-version"); err == nil {
		t.Error("rollback to missing version should fail")
	}
}

func TestRuleVersion_SaveVersionMark(t *testing.T) {
	m := setupRuleModuleWithVersions(t)

	if err := m.SaveVersion("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	items, total, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if items[0].Source != "mark" {
		t.Errorf("Source = %q, want mark", items[0].Source)
	}

	// 手动版本不去重：同内容再存一条 mark 仍留档
	if err := m.SaveVersion("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	// 同内容草稿保存被去重跳过（与最新 mark 快照相同）
	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	items, total, _ = m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 2 {
		t.Fatalf("total = %d, want 2 (mark x2, draft deduped)", total)
	}
	if items[0].Source != "mark" {
		t.Errorf("newest Source = %q, want mark", items[0].Source)
	}

	// 手动版本后普通保存的 source 不残留 mark
	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("B"))); err != nil {
		t.Fatal(err)
	}
	items, _, _ = m.ListVersions("admin", "ver-chain", 20, 1)
	if items[0].Source != "save" {
		t.Errorf("after mark, draft save Source = %q, want save", items[0].Source)
	}
}

func TestRuleVersion_DraftDedup(t *testing.T) {
	m := setupRuleModuleWithVersions(t)

	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A")))
	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A")))
	_, total, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 1 {
		t.Fatalf("total = %d after identical draft saves, want 1", total)
	}
	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("B")))
	_, total, _ = m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 2 {
		t.Fatalf("total = %d after changed draft save, want 2", total)
	}
}

func TestRuleVersion_DraftRetentionTrim(t *testing.T) {
	m, _ := setupRuleModuleVersionsCfg(t, func(c *config.Config) { c.RuleVersionDraftRetention = 2 })

	for _, name := range []string{"A", "B", "C"} {
		if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV(name))); err != nil {
			t.Fatal(err)
		}
	}
	items, total, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 2 {
		t.Fatalf("total = %d, want 2 (draft cap)", total)
	}
	// 新在前：留下 B、C，最旧的 A 被裁
	if items[0].ChainName != "C" || items[1].ChainName != "B" {
		t.Errorf("kept = %q,%q, want C,B", items[1].ChainName, items[0].ChainName)
	}
}

func TestRuleVersion_ManualRetentionTrim(t *testing.T) {
	// 手动版本按 RuleVersionRetentionCount 裁，草稿档不受影响
	m, _ := setupRuleModuleVersionsCfg(t, func(c *config.Config) {
		c.RuleVersionRetentionCount = 3
		c.RuleVersionDraftRetention = 5
	})
	for _, name := range []string{"A", "B", "C", "D"} {
		if err := m.SaveVersion("admin", "ver-chain", []byte(chainDefV(name))); err != nil {
			t.Fatal(err)
		}
	}
	_, total, _ := m.ListVersions("admin", "ver-chain", 20, 1)
	if total != 3 {
		t.Fatalf("total = %d, want 3 (manual cap)", total)
	}
}

func TestRuleVersion_DraftDisabled(t *testing.T) {
	// 草稿档 0：普通保存不留快照，手动版本与回滚不受影响
	m, _ := setupRuleModuleVersionsCfg(t, func(c *config.Config) { c.RuleVersionDraftRetention = 0 })

	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A")))
	if _, total, _ := m.ListVersions("admin", "ver-chain", 20, 1); total != 0 {
		t.Fatalf("draft saves should not snapshot when draft retention is 0, got %d", total)
	}
	if err := m.SaveVersion("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := m.ListVersions("admin", "ver-chain", 20, 1); total != 1 {
		t.Fatalf("manual save version should snapshot, got %d", total)
	}
}

func TestRuleVersion_DeleteChainCleansVersions(t *testing.T) {
	m := setupRuleModuleWithVersions(t)
	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A")))
	_ = m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("B")))

	if err := m.Delete("admin", "ver-chain"); err != nil {
		t.Fatal(err)
	}
	_, total, err := m.ListVersions("admin", "ver-chain", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("total = %d after chain delete, want 0", total)
	}
}

func TestRuleVersion_ServiceRegisteredOnlyWithStore(t *testing.T) {
	// setupRuleModule 不注入版本存储：服务不应注册，已有行为不受影响
	m, container := setupRuleModule(t)
	_, _ = m, container
	if _, ok := container.Get(services.KeyRuleVersionService); ok {
		t.Error("KeyRuleVersionService should not be registered without version store")
	}
}

func TestRuleVersion_DisabledByConfig(t *testing.T) {
	// 存储已注入但配置关闭：模块不采用存储、不注册服务、保存不产生快照
	m, container := setupRuleModuleVersionsOpts(t, true)
	if m.versions != nil {
		t.Fatal("versions should stay nil when disabled by config")
	}
	if _, ok := container.Get(services.KeyRuleVersionService); ok {
		t.Error("KeyRuleVersionService should not be registered when disabled")
	}
	if err := m.SaveAndLoad("admin", "ver-chain", []byte(chainDefV("A"))); err != nil {
		t.Fatal(err)
	}
}
