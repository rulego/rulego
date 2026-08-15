package filestore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 分类存储测试矩阵：分类=子目录是存储层的路径约定，这里的用例覆盖
// 写入、读取、删除、改分类、索引错位、多副本收敛的各个组合。

// dslOf 构造最小 DSL，category 为空时不带 category 字段
func dslOf(id, name, category string) []byte {
	cat := ""
	if category != "" {
		cat = `, "category": "` + category + `"`
	}
	return []byte(`{"ruleChain": {"id": "` + id + `", "name": "` + name + `", "root": true,
		"additionalInfo": {"updateTime": "2026/08/15 12:00:00"` + cat + `}},
		"metadata": {"nodes": []}}`)
}

// mustWrite 手工落盘一个 DSL 文件（模拟手动上传/历史遗留），stamp 控制新旧
func mustWrite(t *testing.T, store *RuleStore, username, rel string, dsl []byte, older bool) string {
	t.Helper()
	p := filepath.Join(store.ruleBasePath(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, dsl, 0o644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now()
	if older {
		stamp = stamp.Add(-2 * time.Hour)
	}
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return p
}

func listCategory(t *testing.T, store *RuleStore, username, id string) interface{} {
	t.Helper()
	items, _, err := store.List(username, "", nil, nil, "", 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, c := range items {
		if c.RuleChain.ID == id {
			return c.RuleChain.AdditionalInfo["category"]
		}
	}
	return nil
}

// 常规链路：带分类保存 → 落分类子目录、索引/列表分类正确、Get/删除正常
func TestCategory_SaveGetListDelete_RoundTrip(t *testing.T) {
	store := newTestRuleStore(t, "rt")
	if err := store.Save("rt", "c1", dslOf("c1", "链1", "iot")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(store.ruleBasePath(), "iot", "c1.json")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file should be in iot dir: %v", err)
	}
	if got := listCategory(t, store, "rt", "c1"); got != "iot" {
		t.Errorf("list category = %v, want iot", got)
	}
	if _, err := store.Get("rt", "c1"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := store.Delete("rt", "c1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("file should be removed, stat err=%v", err)
	}
	if got := listCategory(t, store, "rt", "c1"); got != nil {
		t.Errorf("deleted chain still listed, category=%v", got)
	}
}

// 多级分类：a/b 落两级目录，索引与读取按全路径解析
func TestCategory_MultiLevel(t *testing.T) {
	store := newTestRuleStore(t, "ml")
	if err := store.Save("ml", "c2", dslOf("c2", "链2", "collect/modbus")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(store.ruleBasePath(), "collect", "modbus", "c2.json")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file should be in collect/modbus: %v", err)
	}
	if got := listCategory(t, store, "ml", "c2"); got != "collect/modbus" {
		t.Errorf("list category = %v, want collect/modbus", got)
	}
	if _, err := store.Get("ml", "c2"); err != nil {
		t.Fatalf("Get multi-level: %v", err)
	}
}

// 同分类重复保存：单文件，无副本
func TestCategory_SaveSameCategory_NoDuplicate(t *testing.T) {
	store := newTestRuleStore(t, "same")
	for i := 0; i < 3; i++ {
		if err := store.Save("same", "c3", dslOf("c3", "链3", "iot")); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := filepath.Glob(filepath.Join(store.ruleBasePath(), "*", "c3.json"))
	if len(entries) != 1 {
		t.Errorf("expected single file in category dir, got %v", entries)
	}
	root := filepath.Join(store.ruleBasePath(), "c3.json")
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("root copy should not exist, stat err=%v", err)
	}
}

// 索引错位：索引分类=iot、文件在根目录（历史遗留）。
// Get 回退根目录可读；Save 后归位到 iot/；对账清掉根目录残留
func TestCategory_IndexDirMismatch_HealsOnSave(t *testing.T) {
	store := newTestRuleStore(t, "heal")
	// 正常路径先落 iot/ 并进索引，再把文件挪到根目录，制造索引/位置错位
	mustWrite(t, store, "heal", "iot/c4.json", dslOf("c4", "链4", "iot"), false)
	if _, _, err := store.List("heal", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	iotFile := filepath.Join(store.ruleBasePath(), "iot", "c4.json")
	rootFile := filepath.Join(store.ruleBasePath(), "c4.json")
	if err := os.Rename(iotFile, rootFile); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get("heal", "c4"); err != nil {
		t.Fatalf("Get with index/file mismatch: %v", err)
	}
	// 保存同分类：写入 iot/，根目录残留由对账清除
	if err := store.Save("heal", "c4", dslOf("c4", "链4", "iot")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(iotFile); err != nil {
		t.Fatalf("file should be back in iot dir: %v", err)
	}
	if _, _, err := store.List("heal", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(rootFile); !os.IsNotExist(err) {
		t.Errorf("stale root copy should be converged away, stat err=%v", err)
	}
}

// 删除错位链：索引分类=iot、文件在根目录，Delete 必须清掉根目录文件，
// 否则对账会把链复活
func TestCategory_DeleteWithMismatch_DoesNotResurrect(t *testing.T) {
	store := newTestRuleStore(t, "del")
	mustWrite(t, store, "del", "iot/c5.json", dslOf("c5", "链5", "iot"), false)
	if _, _, err := store.List("del", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	rootFile := filepath.Join(store.ruleBasePath(), "c5.json")
	if err := os.Rename(filepath.Join(store.ruleBasePath(), "iot", "c5.json"), rootFile); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("del", "c5"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(rootFile); !os.IsNotExist(err) {
		t.Fatalf("root file should be removed by Delete, stat err=%v", err)
	}
	if _, _, err := store.List("del", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := listCategory(t, store, "del", "c5"); got != nil {
		t.Errorf("chain resurrected after delete, category=%v", got)
	}
}

// 多副本收敛的两个方向：新副本在分类目录（列表应归位分类）；
// 新副本在根目录（分类以文件内容为准，旧分类目录副本被清除）
func TestCategory_DuplicateConverge_NewestInCategoryDir(t *testing.T) {
	store := newTestRuleStore(t, "dup1")
	mustWrite(t, store, "dup1", "c6.json", dslOf("c6", "链6", ""), true)         // 根目录旧（无分类）
	mustWrite(t, store, "dup1", "iot/c6.json", dslOf("c6", "链6", "iot"), false) // iot 新
	if _, _, err := store.List("dup1", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.ruleBasePath(), "c6.json")); !os.IsNotExist(err) {
		t.Errorf("stale root copy should be removed, stat err=%v", err)
	}
	if got := listCategory(t, store, "dup1", "c6"); got != "iot" {
		t.Errorf("category = %v, want iot", got)
	}
	if _, err := store.Get("dup1", "c6"); err != nil {
		t.Errorf("Get after convergence: %v", err)
	}
}

func TestCategory_DuplicateConverge_NewestInRoot(t *testing.T) {
	store := newTestRuleStore(t, "dup2")
	mustWrite(t, store, "dup2", "iot/c7.json", dslOf("c7", "链7", "iot"), true) // iot 旧
	mustWrite(t, store, "dup2", "c7.json", dslOf("c7", "链7", ""), false)       // 根目录新（无分类）
	if _, _, err := store.List("dup2", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.ruleBasePath(), "iot", "c7.json")); !os.IsNotExist(err) {
		t.Errorf("stale iot copy should be removed, stat err=%v", err)
	}
	if got := listCategory(t, store, "dup2", "c7"); got != nil {
		t.Errorf("category = %v, want empty", got)
	}
}

// 手动把文件从分类 a 挪到分类 b（不走 API）：对账以新的为准
func TestCategory_ManualMoveBetweenCategories(t *testing.T) {
	store := newTestRuleStore(t, "mv")
	mustWrite(t, store, "mv", "a/c8.json", dslOf("c8", "链8", "a"), true)
	mustWrite(t, store, "mv", "b/c8.json", dslOf("c8", "链8", "b"), false)
	if _, _, err := store.List("mv", "", nil, nil, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.ruleBasePath(), "a", "c8.json")); !os.IsNotExist(err) {
		t.Errorf("old category copy should be removed, stat err=%v", err)
	}
	if got := listCategory(t, store, "mv", "c8"); got != "b" {
		t.Errorf("category = %v, want b", got)
	}
}

// 不存在的链：Get 返回错误（对账兜底后仍找不到）
func TestCategory_GetMissing(t *testing.T) {
	store := newTestRuleStore(t, "miss")
	if _, err := store.Get("miss", "no-such-id"); err == nil {
		t.Error("Get on missing chain should fail")
	}
}

// 分类路径穿越防护：索引里的非法分类不能拼出逃逸路径
func TestCategory_UnsafeCategoryRejected(t *testing.T) {
	store := newTestRuleStore(t, "unsafe")
	// 直接写入索引一个带穿越的分类，模拟被篡改的索引。
	// 先 Delete 后 Get：Delete 不做对账、直接拒绝；Get 拒绝后对账会把幽灵条目清掉
	store.Lock()
	store.index.Rules["evil"] = RuleMeta{ID: "evil", Category: "../escape"}
	store.Unlock()
	if err := store.Delete("unsafe", "evil"); err == nil {
		t.Error("Delete with unsafe category should fail")
	}
	if _, err := store.Get("unsafe", "evil"); err == nil {
		t.Error("Get with unsafe category should fail")
	}
}
