package filestore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/server/internal/constants"
)

// newTestRuleStore 创建 RuleStore 并预先建好用户目录（NewRuleStore 写 index 需要）。
func newTestRuleStore(t *testing.T, username string) *RuleStore {
	t.Helper()
	cfg := newTestConfig(t)
	userRulesDir := filepath.Join(cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule)
	if err := os.MkdirAll(userRulesDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", userRulesDir, err)
	}
	store, err := NewRuleStore(cfg, username)
	if err != nil {
		t.Fatalf("NewRuleStore: %v", err)
	}
	return store
}

// TestRuleStore_AllChains 验证 AllChains 返回所有链（含 SystemAgent）的 ID+DSL。
func TestRuleStore_AllChains(t *testing.T) {
	store := newTestRuleStore(t, "alice")

	chains := []struct {
		id  string
		dsl []byte
	}{
		{"normal-1", []byte(`{"ruleChain": {"id": "normal-1", "name": "N1", "additionalInfo": {}}, "metadata": {"nodes": []}}`)},
		{"normal-2", []byte(`{"ruleChain": {"id": "normal-2", "name": "N2", "additionalInfo": {}}, "metadata": {"nodes": []}}`)},
		{"agent-1", []byte(`{"ruleChain": {"id": "agent-1", "name": "A1", "additionalInfo": {"systemAgent": true}}, "metadata": {"nodes": []}}`)},
	}
	for _, c := range chains {
		if err := store.Save("alice", c.id, c.dsl); err != nil {
			t.Fatalf("Save(%s): %v", c.id, err)
		}
	}

	got, err := store.AllChains("alice")
	if err != nil {
		t.Fatalf("AllChains: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("AllChains returned %d chains, want 3", len(got))
	}
	for _, c := range chains {
		dsl, ok := got[c.id]
		if !ok {
			t.Errorf("AllChains missing %s", c.id)
			continue
		}
		// DSL 内容应与存入一致（JSON 经 Save 重新格式化，比较解析后结构）
		if extractID(t, dsl) != c.id {
			t.Errorf("AllChains(%s) DSL mismatch", c.id)
		}
	}

	// 对比：List 过滤 SystemAgent，应只剩 2 条
	_, total, err := store.List("alice", "", nil, nil, "", 0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("List total = %d, want 2 (SystemAgent filtered)", total)
	}
}

// TestRuleStore_AllChains_Empty 空用户场景
func TestRuleStore_AllChains_Empty(t *testing.T) {
	store := newTestRuleStore(t, "empty-user")
	got, err := store.AllChains("empty-user")
	if err != nil {
		t.Fatalf("AllChains: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("AllChains on empty user returned %d, want 0", len(got))
	}
}

func extractID(t *testing.T, dsl []byte) string {
	t.Helper()
	var rc struct {
		RuleChain struct {
			ID string `json:"id"`
		} `json:"ruleChain"`
	}
	if err := json.Unmarshal(dsl, &rc); err != nil {
		t.Fatalf("unmarshal dsl: %v", err)
	}
	return rc.RuleChain.ID
}
