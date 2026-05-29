package runlogtest

import (
	"fmt"
	"os"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/bboltstore"
	"github.com/rulego/rulego/server/internal/store/jsonlstore"
)

func TestStoreFileSize(t *testing.T) {
	count := 1000

	// BBolt
	bboltDir, _ := os.MkdirTemp("", "size-bbolt-*")
	defer os.RemoveAll(bboltDir)
	cfg := config.Config{DataDir: bboltDir}
	bs, _ := bboltstore.NewRunLogStore(cfg, types.DefaultLogger())
	for i := 0; i < count; i++ {
		_ = bs.Save("user1", makeEvent(i))
	}
	bs.Close()
	bboltSize := fileSize(bboltDir + "/runlog.db")
	bboltPerRow := bboltSize / int64(count)
	t.Logf("BBolt:   %d 条日志, 总大小=%s, 每条≈%s", count, formatSize(bboltSize), formatSize(bboltPerRow))

	// JSON Lines
	jsonlDir, _ := os.MkdirTemp("", "size-jsonl-*")
	defer os.RemoveAll(jsonlDir)
	cfg2 := config.Config{DataDir: jsonlDir}
	js, _ := jsonlstore.NewRunLogStore(cfg2, types.DefaultLogger())
	for i := 0; i < count; i++ {
		_ = js.Save("user1", makeEvent(i))
	}
	js.Close()
	jsonlSize := fileSize(jsonlDir + "/workflows/user1/runs/bench-chain.jsonl")
	jsonlPerRow := jsonlSize / int64(count)
	t.Logf("Jsonl:   %d 条日志, 总大小=%s, 每条≈%s", count, formatSize(jsonlSize), formatSize(jsonlPerRow))
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func formatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}
