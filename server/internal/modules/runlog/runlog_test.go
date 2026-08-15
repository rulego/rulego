package runlog

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/internal/store/nopstore"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
)

func TestRunlogModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "runlog" {
		t.Errorf("Name() = %q, want %q", m.Name(), "runlog")
	}
	if m.Priority() != 45 {
		t.Errorf("Priority() = %d, want 45", m.Priority())
	}
}

func TestRunlogModuleInitRegistersService(t *testing.T) {
	tmpDir := t.TempDir()
	m := New()
	container := app.NewContainer()
	cfg := config.Config{DataDir: tmpDir}
	container.Register("core.config", &cfg)
	logger := types.DefaultLogger()
	container.Register("core.logger", logger)
	provider := filestore.NewFileStoreProvider(cfg, nil)
	provider.SetRunLogStore(nopstore.NopRunLogStore{})
	container.Register("store.provider", store.StoreProvider(provider))

	ctx := &app.ModuleContext{Container: container, Config: &cfg, Logger: logger}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if _, ok := container.Get(services.KeyRunLogService); !ok {
		t.Error("module.runlog.service not registered")
	}
}

func TestRunlogModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRunlogServiceImplListEmpty(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	events, total, err := svc.List("admin", "", time.Time{}, time.Time{}, 20, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0 for empty store", total)
	}
	if events != nil {
		t.Errorf("events should be nil for nop store")
	}
}

func TestRunlogServiceImplGetNonexistent(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	_, err := svc.Get("admin", "nonexistent")
	// NopRunLogStore returns zero value, no error
	if err != nil {
		t.Logf("Get() on nonexistent returned error (acceptable): %v", err)
	}
}

func TestRunlogServiceImplDeleteNonexistent(t *testing.T) {
	cfg := &config.Config{}
	svc := &runLogServiceImpl{cfg: cfg, store: nopstore.NopRunLogStore{}}

	err := svc.Delete("admin", "nonexistent")
	if err != nil {
		t.Fatalf("NopRunLogStore.Delete should return nil, got: %v", err)
	}
}

// Compile-time interface check
var _ services.RunLogService = (*runLogServiceImpl)(nil)

// TestDebugDataStore_UserIsolation 不同用户同名链的调试数据互不可见。
func TestDebugDataStore_UserIsolation(t *testing.T) {
	s := NewDebugDataStore(10)
	s.Add("alice", "chain-1", "n1", map[string]interface{}{"ts": int64(1), "user": "alice"})
	s.Add("bob", "chain-1", "n1", map[string]interface{}{"ts": int64(2), "user": "bob"})

	if got := s.GetPage("alice", "chain-1", "n1", 1, 20); got["total"].(int) != 1 {
		t.Errorf("alice total = %v, want 1", got["total"])
	}
	if got := s.GetPage("bob", "chain-1", "n1", 1, 20); got["total"].(int) != 1 {
		t.Errorf("bob total = %v, want 1", got["total"])
	}
	s.Clear("alice", "chain-1")
	if got := s.GetPage("alice", "chain-1", "n1", 1, 20); got["total"].(int) != 0 {
		t.Errorf("alice total after clear = %v, want 0", got["total"])
	}
	if got := s.GetPage("bob", "chain-1", "n1", 1, 20); got["total"].(int) != 1 {
		t.Errorf("bob total after alice clear = %v, want 1", got["total"])
	}
}

// TestSendDebugDataToClients_ConcurrentUnregister 并发发送与注销+close 不应 panic。
func TestSendDebugDataToClients_ConcurrentUnregister(t *testing.T) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	// 3 个发送者持续广播
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					SendDebugDataToClients("alice", "chain-1", map[string]interface{}{"ts": int64(1)})
				}
			}
		}()
	}
	// 注册/注销/关闭循环
	for i := 0; i < 50; i++ {
		client := &DebugDataClient{Username: "alice", ChainId: "chain-1", DataCh: make(chan map[string]interface{}, 1)}
		RegisterDebugClient(client)
		UnregisterDebugClient(client)
		close(client.DataCh)
	}
	close(done)
	wg.Wait()
}

// TestSendDebugDataToClients_UserFilter 广播只投递给同用户的客户端。
func TestSendDebugDataToClients_UserFilter(t *testing.T) {
	aliceCh := make(chan map[string]interface{}, 1)
	bobCh := make(chan map[string]interface{}, 1)
	RegisterDebugClient(&DebugDataClient{Username: "alice", ChainId: "c", DataCh: aliceCh})
	RegisterDebugClient(&DebugDataClient{Username: "bob", ChainId: "c", DataCh: bobCh})
	defer func() {
		UnregisterDebugClient(&DebugDataClient{Username: "alice", ChainId: "c"})
		UnregisterDebugClient(&DebugDataClient{Username: "bob", ChainId: "c"})
	}()

	SendDebugDataToClients("alice", "c", map[string]interface{}{"ts": int64(1)})
	select {
	case <-aliceCh:
	default:
		t.Error("alice client should receive")
	}
	select {
	case <-bobCh:
		t.Error("bob client should not receive alice data")
	default:
	}
}
