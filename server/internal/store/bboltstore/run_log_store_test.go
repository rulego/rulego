package bboltstore

import (
	"os"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/runlogtest"
)

func TestBBoltRunLogStore(t *testing.T) {
	dir, err := os.MkdirTemp("", "bbolt-test-*")
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
