package bboltstore

import (
	"strconv"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

func newTestVersionStore(t *testing.T, retention int) *RuleVersionStore {
	t.Helper()
	cfg := config.Config{DataDir: t.TempDir(), RuleVersionRetentionCount: retention}
	s, err := NewRuleVersionStore(cfg, types.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func ver(id, chainId string, ts int64, dsl string) model.RuleVersion {
	return model.RuleVersion{Id: id, ChainId: chainId, Ts: ts, Source: "save", Dsl: []byte(dsl)}
}

func TestRuleVersionStore_Roundtrip(t *testing.T) {
	s := newTestVersionStore(t, 0)

	if err := s.Save("admin", ver("v1", "chainA", 1000, `{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("admin", ver("v2", "chainA", 2000, `{"a":2}`)); err != nil {
		t.Fatal(err)
	}
	// 其他链与其他用户互不串
	if err := s.Save("admin", ver("v1", "chainB", 3000, `{"b":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("user2", ver("v1", "chainA", 4000, `{"c":1}`)); err != nil {
		t.Fatal(err)
	}

	items, total, err := s.List("admin", "chainA", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("admin/chainA: total=%d len=%d, want 2/2", total, len(items))
	}
	// 倒序：最新在前；列表不回传 Dsl
	if items[0].Id != "v2" || items[1].Id != "v1" {
		t.Errorf("order = [%s, %s], want [v2, v1]", items[0].Id, items[1].Id)
	}
	if items[0].Dsl != nil {
		t.Error("List should strip Dsl")
	}

	got, err := s.Get("admin", "chainA", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Dsl) != `{"a":1}` {
		t.Errorf("Get Dsl = %s, want {\"a\":1}", got.Dsl)
	}

	if _, err := s.Get("admin", "chainA", "nope"); err != store.ErrRuleVersionNotFound {
		t.Errorf("Get missing = %v, want ErrRuleVersionNotFound", err)
	}

	// 分页
	page1, total, _ := s.List("admin", "chainA", 1, 1)
	if total != 2 || len(page1) != 1 || page1[0].Id != "v2" {
		t.Errorf("page1 = %v total=%d, want newest only", page1, total)
	}
}

func TestRuleVersionStore_Retention(t *testing.T) {
	s := newTestVersionStore(t, 3)
	for i := 0; i < 6; i++ {
		v := ver("v"+strconv.Itoa(i), "chainA", int64(1000+i), `{}`)
		if err := s.Save("admin", v); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := s.List("admin", "chainA", 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("total=%d len=%d, want 3/3 (retention)", total, len(items))
	}
	// 保留最新的 3、4、5；裁掉 0、1、2
	for _, it := range items {
		if it.Id == "v0" || it.Id == "v1" || it.Id == "v2" {
			t.Errorf("old version %s kept", it.Id)
		}
	}
}

func TestRuleVersionStore_DeleteByChainId(t *testing.T) {
	s := newTestVersionStore(t, 0)
	_ = s.Save("admin", ver("v1", "chainA", 1000, `{}`))
	_ = s.Save("admin", ver("v2", "chainA", 2000, `{}`))
	_ = s.Save("admin", ver("v1", "chainB", 3000, `{}`))

	if err := s.DeleteByChainId("admin", "chainA"); err != nil {
		t.Fatal(err)
	}
	_, total, _ := s.List("admin", "chainA", 20, 1)
	if total != 0 {
		t.Errorf("chainA total = %d, want 0", total)
	}
	_, total, _ = s.List("admin", "chainB", 20, 1)
	if total != 1 {
		t.Errorf("chainB total = %d, want 1（不受连带影响）", total)
	}
}

func TestRuleVersionStore_Seq(t *testing.T) {
	s := newTestVersionStore(t, 0)
	for i := 0; i < 3; i++ {
		if err := s.Save("admin", ver("v"+strconv.Itoa(i+1), "chainA", int64(1000+i), `{}`)); err != nil {
			t.Fatal(err)
		}
	}
	// 另一条链独立编号
	if err := s.Save("admin", ver("x1", "chainB", 2000, `{}`)); err != nil {
		t.Fatal(err)
	}
	items, _, _ := s.List("admin", "chainA", 20, 1)
	// 倒序 v3,v2,v1
	if items[0].Seq != 3 || items[1].Seq != 2 || items[2].Seq != 1 {
		t.Errorf("seqs = %d,%d,%d, want 3,2,1", items[0].Seq, items[1].Seq, items[2].Seq)
	}
	bItems, _, _ := s.List("admin", "chainB", 20, 1)
	if bItems[0].Seq != 1 {
		t.Errorf("chainB seq = %d, want 1", bItems[0].Seq)
	}

	// 保留裁剪后新版本号继续递增，不回收重排
	s2 := newTestVersionStore(t, 2)
	for i := 0; i < 4; i++ {
		_ = s2.Save("admin", ver("v"+strconv.Itoa(i+1), "chainC", int64(1000+i), `{}`))
	}
	cItems, total, _ := s2.List("admin", "chainC", 20, 1)
	if total != 2 || cItems[0].Seq != 4 || cItems[1].Seq != 3 {
		t.Errorf("after trim total=%d seqs=%d,%d, want 2 / 4,3", total, cItems[0].Seq, cItems[1].Seq)
	}
}
