package jsonlstore

import (
	"os"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/runlogtest"
	"github.com/rulego/rulego/server/model"
)

func TestJsonlRunLogStore(t *testing.T) {
	dir, err := os.MkdirTemp("", "jsonl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := config.Config{DataDir: dir}
	s, err := NewRunLogStore(cfg, types.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	t.Run("Common", func(t *testing.T) { runlogtest.RunStoreTests(t, s) })
}

// TestJsonlRunLogStore_RejectsUnsafeChainId 验证 chainId 不被拼入路径造成穿越。
func TestJsonlRunLogStore_RejectsUnsafeChainId(t *testing.T) {
	dir, err := os.MkdirTemp("", "jsonl-unsafe-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	s, err := NewRunLogStore(config.Config{DataDir: dir}, types.DefaultLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, id := range []string{"../../evil", `a\b`, ".."} {
		event := model.Event{Id: "e1", ChainId: id, ChainName: "c", StartTs: 1, EndTs: 2}
		if err := s.Save("alice", event); err == nil {
			t.Errorf("Save(chainId=%q) should fail", id)
		}
		if _, _, err := s.List("alice", id, time.Time{}, time.Time{}, 10, 1); err == nil {
			t.Errorf("List(chainId=%q) should fail", id)
		}
		if err := s.DeleteByChainId("alice", id); err == nil {
			t.Errorf("DeleteByChainId(chainId=%q) should fail", id)
		}
	}
}
