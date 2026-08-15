package filestore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/server/internal/constants"
)

// saveChain 便捷保存一条链 DSL
func saveChain(t *testing.T, store *RuleStore, username, id, name, updateTime string) {
	t.Helper()
	dsl := `{"ruleChain": {"id": "` + id + `", "name": "` + name + `", "root": true,
		"additionalInfo": {"updateTime": "` + updateTime + `", "description": "desc of ` + id + `"}},
		"metadata": {"nodes": [{"id": "n1", "type": "jsFilter"}],
		"endpoints": [{"id": "e1", "type": "endpoint/net", "name": "TCP"}]}}`
	if err := store.Save(username, id, []byte(dsl)); err != nil {
		t.Fatalf("Save(%s): %v", id, err)
	}
}

// listIDs 取 List 结果里的 id 集合（root 全量）
func listIDs(t *testing.T, store *RuleStore, username string) map[string]bool {
	t.Helper()
	items, _, err := store.List(username, "", nil, nil, "", 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	out := make(map[string]bool, len(items))
	for _, c := range items {
		out[c.RuleChain.ID] = true
	}
	return out
}

// TestRuleStore_ListSummary 验证 List 只返回摘要：additionalInfo 摘要字段 + 首个 endpoint
// 类型在，完整 nodes 元数据不在；排序按 updateTime 倒序。
func TestRuleStore_ListSummary(t *testing.T) {
	store := newTestRuleStore(t, "bob")
	saveChain(t, store, "bob", "older", "旧链", "2026/08/01 10:00:00")
	saveChain(t, store, "bob", "newer", "新链", "2026/08/14 10:00:00")

	items, total, err := store.List("bob", "", nil, nil, "", 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("List total=%d items=%d, want 2/2", total, len(items))
	}
	if items[0].RuleChain.ID != "newer" || items[1].RuleChain.ID != "older" {
		t.Errorf("List order = [%s, %s], want [newer, older] (updateTime desc)",
			items[0].RuleChain.ID, items[1].RuleChain.ID)
	}
	for _, c := range items {
		addi := c.RuleChain.AdditionalInfo
		if addi == nil || addi["updateTime"] == nil || addi["description"] == nil {
			t.Errorf("List(%s) missing additionalInfo summary: %+v", c.RuleChain.ID, addi)
		}
		if len(c.Metadata.Endpoints) != 1 || c.Metadata.Endpoints[0].Type != "endpoint/net" {
			t.Errorf("List(%s) endpoints summary = %+v, want first type endpoint/net",
				c.RuleChain.ID, c.Metadata.Endpoints)
		}
		if len(c.Metadata.Nodes) != 0 {
			t.Errorf("List(%s) should not carry full nodes metadata (got %d nodes)",
				c.RuleChain.ID, len(c.Metadata.Nodes))
		}
	}
}

// TestRuleStore_Reconcile_ManualUpload 手动上传 DSL（绕过 API 直接落盘）：
// 运行中的 store 下次 List 应自动补索引（磁盘对账），无需重启。
func TestRuleStore_Reconcile_ManualUpload(t *testing.T) {
	store := newTestRuleStore(t, "carol")
	saveChain(t, store, "carol", "api-chain", "API保存的", "2026/08/14 09:00:00")

	// 模拟手动上传：直接写文件到用户规则链目录
	rulesDir := filepath.Join(store.config.DataDir, constants.DirWorkflows, "carol", constants.DirWorkflowsRule)
	manual := `{"ruleChain": {"id": "manual-chain", "name": "手动上传", "root": true,
		"additionalInfo": {"updateTime": "2026/08/14 10:00:00"}}, "metadata": {"nodes": []}}`
	if err := os.WriteFile(filepath.Join(rulesDir, "manual-chain.json"), []byte(manual), 0o644); err != nil {
		t.Fatalf("write manual dsl: %v", err)
	}

	ids := listIDs(t, store, "carol")
	if !ids["api-chain"] || !ids["manual-chain"] {
		t.Errorf("List after manual upload = %v, want both api-chain and manual-chain", ids)
	}
	// 摘要字段应来自文件内容
	items, _, _ := store.List("carol", "", nil, nil, "", 0, 0)
	for _, c := range items {
		if c.RuleChain.ID == "manual-chain" && c.RuleChain.Name != "手动上传" {
			t.Errorf("manual-chain name = %q, want 手动上传", c.RuleChain.Name)
		}
	}
}

// TestRuleStore_Reconcile_ManualDelete 手动删除 DSL 文件：残留索引应被清掉。
func TestRuleStore_Reconcile_ManualDelete(t *testing.T) {
	store := newTestRuleStore(t, "dave")
	saveChain(t, store, "dave", "keep", "保留", "2026/08/14 09:00:00")
	saveChain(t, store, "dave", "gone", "将删", "2026/08/14 10:00:00")

	rulesDir := filepath.Join(store.config.DataDir, constants.DirWorkflows, "dave", constants.DirWorkflowsRule)
	if err := os.Remove(filepath.Join(rulesDir, "gone.json")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	ids := listIDs(t, store, "dave")
	if ids["gone"] {
		t.Errorf("List after manual delete still contains gone: %v", ids)
	}
	if !ids["keep"] {
		t.Errorf("List lost keep: %v", ids)
	}
}

// TestRuleStore_Reconcile_Overwrite 手动覆写已有 DSL 文件（同 id 不同内容）：
// mtime 变化应触发重读，摘要更新为新内容。
func TestRuleStore_Reconcile_Overwrite(t *testing.T) {
	store := newTestRuleStore(t, "erin")
	saveChain(t, store, "erin", "chain-x", "旧名字", "2026/08/14 09:00:00")

	rulesDir := filepath.Join(store.config.DataDir, constants.DirWorkflows, "erin", constants.DirWorkflowsRule)
	overwritten := `{"ruleChain": {"id": "chain-x", "name": "新名字", "root": true,
		"additionalInfo": {"updateTime": "2026/08/14 11:00:00", "description": "覆写后的描述"}},
		"metadata": {"nodes": []}}`
	target := filepath.Join(rulesDir, "chain-x.json")
	if err := os.WriteFile(target, []byte(overwritten), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	// 保证 mtime 与 Save 时不同（部分文件系统粒度粗）
	newTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, newTime, newTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	items, _, _ := store.List("erin", "", nil, nil, "", 0, 0)
	if len(items) != 1 || items[0].RuleChain.Name != "新名字" {
		t.Fatalf("List after overwrite = %+v, want name 新名字", items)
	}
	if items[0].RuleChain.AdditionalInfo["description"] != "覆写后的描述" {
		t.Errorf("description = %v, want 覆写后的描述", items[0].RuleChain.AdditionalInfo["description"])
	}
}

// TestRuleStore_IndexVersionBackfill 旧版索引（无 v 字段、无摘要字段）加载后应触发
// 全量重扫回填：摘要字段齐备、SchemaVersion 升级。
func TestRuleStore_IndexVersionBackfill(t *testing.T) {
	cfg := newTestConfig(t)
	username := "frank"
	rulesDir := filepath.Join(cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := NewRuleStore(cfg, username)
	if err != nil {
		t.Fatal(err)
	}
	saveChain(t, store, username, "legacy", "旧索引链", "2026/08/14 09:00:00")

	// 手工把索引降级成 v1 形态（去掉 v 与新字段），模拟升级前数据
	idx, err := os.ReadFile(store.getIndexPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(idx, &raw); err != nil {
		t.Fatal(err)
	}
	rules := raw["rules"].(map[string]interface{})
	for _, v := range rules {
		m := v.(map[string]interface{})
		delete(m, "v")
		delete(m, "description")
		delete(m, "message")
		delete(m, "firstEndpointType")
		delete(m, "mtime")
	}
	delete(raw, "v")
	downgraded, _ := json.Marshal(raw)
	if err := os.WriteFile(store.getIndexPath(), downgraded, 0o644); err != nil {
		t.Fatal(err)
	}

	// 重新加载：v0 → rebuild → 新字段回填
	store2, err := NewRuleStore(cfg, username)
	if err != nil {
		t.Fatalf("NewRuleStore with legacy index: %v", err)
	}
	items, _, _ := store2.List(username, "", nil, nil, "", 0, 0)
	if len(items) != 1 {
		t.Fatalf("List = %d items, want 1", len(items))
	}
	if items[0].RuleChain.AdditionalInfo["description"] != "desc of legacy" {
		t.Errorf("backfilled description = %v, want 'desc of legacy'",
			items[0].RuleChain.AdditionalInfo["description"])
	}
	if len(items[0].Metadata.Endpoints) != 1 || items[0].Metadata.Endpoints[0].Type != "endpoint/net" {
		t.Errorf("backfilled endpoints = %+v, want endpoint/net", items[0].Metadata.Endpoints)
	}
	store2.RLock()
	v := store2.index.SchemaVersion
	store2.RUnlock()
	if v != ruleIndexSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", v, ruleIndexSchemaVersion)
	}
}

// TestRuleStore_Reconcile_RestartNewStore 手动上传后走 NewRuleStore（重启路径）也能看到。
func TestRuleStore_Reconcile_RestartNewStore(t *testing.T) {
	cfg := newTestConfig(t)
	username := "grace"
	rulesDir := filepath.Join(cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manual := `{"ruleChain": {"id": "uploaded", "name": "重启后发现", "root": true,
		"additionalInfo": {"updateTime": "2026/08/14 10:00:00"}}, "metadata": {"nodes": []}}`
	if err := os.WriteFile(filepath.Join(rulesDir, "uploaded.json"), []byte(manual), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := NewRuleStore(cfg, username)
	if err != nil {
		t.Fatal(err)
	}
	ids := listIDs(t, store, username)
	if !ids["uploaded"] {
		t.Errorf("fresh store List = %v, want uploaded visible", ids)
	}
}

// TestRuleStore_ListKeywords 关键字命中摘要（name/id），不依赖文件读取。
func TestRuleStore_ListKeywords(t *testing.T) {
	store := newTestRuleStore(t, "heidi")
	saveChain(t, store, "heidi", "chain-a1", "温度采集", "2026/08/14 09:00:00")
	saveChain(t, store, "heidi", "chain-b2", "告警推送", "2026/08/14 10:00:00")

	items, total, err := store.List("heidi", "温度", nil, nil, "", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].RuleChain.ID != "chain-a1" {
		t.Errorf("List(keywords=温度) total=%d items=%+v", total, items)
	}
	items, total, _ = store.List("heidi", "chain-b2", nil, nil, "", 20, 1)
	if total != 1 || len(items) != 1 {
		t.Errorf("List(keywords=chain-b2) total=%d", total)
	}
	if !strings.Contains(store.getIndexPath(), "heidi") {
		t.Errorf("index path %s should be user-scoped", store.getIndexPath())
	}
}

// TestRuleStore_GetWithoutList_CategoryDir 手动上传到分类子目录的 DSL，
// 不经过任何 List 请求直接按 id Get：应通过对账兜底找到文件。
func TestRuleStore_GetWithoutList_CategoryDir(t *testing.T) {
	store := newTestRuleStore(t, "ivan")
	saveChain(t, store, "ivan", "seed", "种子链", "2026/08/14 09:00:00")

	// 手动上传到分类子目录
	catDir := filepath.Join(store.ruleBasePath(), "demo", "sub")
	if err := os.MkdirAll(catDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dsl := `{"ruleChain": {"id": "cat-manual", "name": "分类目录手动链", "root": true,
		"additionalInfo": {"category": "demo/sub", "updateTime": "2026/08/14 12:00:00"}},
		"metadata": {"nodes": []}}`
	if err := os.WriteFile(filepath.Join(catDir, "cat-manual.json"), []byte(dsl), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := store.Get("ivan", "cat-manual")
	if err != nil {
		t.Fatalf("Get without prior List: %v", err)
	}
	if extractID(t, data) != "cat-manual" {
		t.Errorf("Get returned wrong chain")
	}
}

// TestRuleStore_Get_CategoryDirFallback 文件在根目录但 additionalInfo.category
// 已设置（启用分类前的旧文件/手动挪动）：Get 按分类路径找不到时回退根目录。
func TestRuleStore_Get_CategoryDirFallback(t *testing.T) {
	store := newTestRuleStore(t, "kate")
	// 直接把带分类标记的 DSL 写进根目录，模拟历史遗留状态
	dsl := `{"ruleChain": {"id": "legacy-cat", "name": "旧分类链", "root": true,
		"additionalInfo": {"category": "iot", "updateTime": "2026/08/15 10:00:00"}},
		"metadata": {"nodes": []}}`
	if err := os.WriteFile(filepath.Join(store.ruleBasePath(), "legacy-cat.json"), []byte(dsl), 0o644); err != nil {
		t.Fatal(err)
	}
	// 先 List 让对账把它收进索引（category 记为 iot）
	if _, _, err := store.List("kate", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := store.Get("kate", "legacy-cat")
	if err != nil {
		t.Fatalf("Get with category/root mismatch: %v", err)
	}
	if extractID(t, data) != "legacy-cat" {
		t.Errorf("Get returned wrong chain")
	}
}
