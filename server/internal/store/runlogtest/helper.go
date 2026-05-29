package runlogtest

import (
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

// TestEvent 创建测试用 Event
func TestEvent(id, chainId string, t time.Time) model.Event {
	return model.Event{
		Id:        id,
		ChainId:   chainId,
		ChainName: "test-chain",
		StartTs:   t.UnixMilli(),
		EndTs:     t.Add(100 * time.Millisecond).UnixMilli(),
		Success:   true,
	}
}

// RunStoreTests 对 store.RunLogStore 实现运行通用测试
func RunStoreTests(t *testing.T, s store.RunLogStore) {
	t.Run("SaveAndGet", func(t *testing.T) { testSaveAndGet(t, s) })
	t.Run("SaveAndList", func(t *testing.T) { testSaveAndList(t, s) })
	t.Run("ListPagination", func(t *testing.T) { testListPagination(t, s) })
	t.Run("ListFilterByChainId", func(t *testing.T) { testListFilterByChainId(t, s) })
	t.Run("ListDescending", func(t *testing.T) { testListDescending(t, s) })
	t.Run("Delete", func(t *testing.T) { testDelete(t, s) })
	t.Run("DeleteByChainId", func(t *testing.T) { testDeleteByChainId(t, s) })
	t.Run("GetNotFound", func(t *testing.T) { testGetNotFound(t, s) })
	t.Run("UserIsolation_SaveList", func(t *testing.T) { testUserIsolationSaveList(t, s) })
	t.Run("UserIsolation_Delete", func(t *testing.T) { testUserIsolationDelete(t, s) })
	t.Run("UserIsolation_DeleteByChainId", func(t *testing.T) { testUserIsolationDeleteByChainId(t, s) })
	t.Run("ConcurrentWrite", func(t *testing.T) { testConcurrentWrite(t, s) })
	t.Run("ConcurrentReadWrite", func(t *testing.T) { testConcurrentReadWrite(t, s) })
}

func testSaveAndGet(t *testing.T, s store.RunLogStore) {
	now := time.Now().Truncate(time.Millisecond)
	event := TestEvent("log-1", "chain-1", now)
	if err := s.Save("user1", event); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Get("user1", "log-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Id != "log-1" {
		t.Errorf("got.Id = %q, want %q", got.Id, "log-1")
	}
	if got.ChainId != "chain-1" {
		t.Errorf("got.ChainId = %q, want %q", got.ChainId, "chain-1")
	}
	if got.StartTs != event.StartTs {
		t.Errorf("got.StartTs = %d, want %d", got.StartTs, event.StartTs)
	}
}

func testSaveAndList(t *testing.T, s store.RunLogStore) {
	base := time.Now().Truncate(time.Millisecond)
	for i := 0; i < 5; i++ {
		event := TestEvent("list-"+string(rune('a'+i)), "chain-list", base.Add(time.Duration(i)*time.Second))
		s.Save("user1", event)
	}
	events, total, err := s.List("user1", "chain-list", time.Time{}, time.Time{}, 10, 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(events) != 5 {
		t.Errorf("len(events) = %d, want 5", len(events))
	}
}

func testListPagination(t *testing.T, s store.RunLogStore) {
	base := time.Now().Truncate(time.Millisecond)
	for i := 0; i < 25; i++ {
		event := TestEvent("page-"+string(rune('A'+i)), "chain-page", base.Add(time.Duration(i)*time.Second))
		s.Save("user1", event)
	}
	events1, total, _ := s.List("user1", "chain-page", time.Time{}, time.Time{}, 20, 1)
	if total != 25 {
		t.Errorf("total = %d, want 25", total)
	}
	if len(events1) != 20 {
		t.Errorf("page1 len = %d, want 20", len(events1))
	}
	events2, _, _ := s.List("user1", "chain-page", time.Time{}, time.Time{}, 20, 2)
	if len(events2) != 5 {
		t.Errorf("page2 len = %d, want 5", len(events2))
	}
}

func testListFilterByChainId(t *testing.T, s store.RunLogStore) {
	base := time.Now().Truncate(time.Millisecond)
	s.Save("user1", TestEvent("f-1", "chain-a", base))
	s.Save("user1", TestEvent("f-2", "chain-b", base.Add(time.Second)))
	s.Save("user1", TestEvent("f-3", "chain-a", base.Add(2*time.Second)))
	events, total, _ := s.List("user1", "chain-a", time.Time{}, time.Time{}, 10, 1)
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}
}

func testListDescending(t *testing.T, s store.RunLogStore) {
	base := time.Now().Truncate(time.Millisecond)
	s.Save("user1", TestEvent("old", "chain-desc", base))
	s.Save("user1", TestEvent("mid", "chain-desc", base.Add(1*time.Second)))
	s.Save("user1", TestEvent("new", "chain-desc", base.Add(2*time.Second)))
	events, _, _ := s.List("user1", "chain-desc", time.Time{}, time.Time{}, 10, 1)
	if len(events) < 3 {
		t.Fatalf("len(events) = %d, want >= 3", len(events))
	}
	if events[0].Id != "new" {
		t.Errorf("first = %q, want %q", events[0].Id, "new")
	}
}

func testDelete(t *testing.T, s store.RunLogStore) {
	s.Save("user1", TestEvent("del-1", "chain-del", time.Now()))
	s.Delete("user1", "del-1")
	got, _ := s.Get("user1", "del-1")
	if got.Id != "" {
		t.Errorf("after delete, got.Id = %q, want empty", got.Id)
	}
}

func testDeleteByChainId(t *testing.T, s store.RunLogStore) {
	s.Save("user1", TestEvent("dc-1", "chain-dc1", time.Now()))
	s.Save("user1", TestEvent("dc-2", "chain-dc2", time.Now()))
	s.DeleteByChainId("user1", "chain-dc1")
	events, total, _ := s.List("user1", "chain-dc1", time.Time{}, time.Time{}, 10, 1)
	if total != 0 || len(events) != 0 {
		t.Errorf("chain-dc1 should be empty, total=%d len=%d", total, len(events))
	}
	events2, total2, _ := s.List("user1", "chain-dc2", time.Time{}, time.Time{}, 10, 1)
	if total2 != 1 || len(events2) != 1 {
		t.Errorf("chain-dc2 should still exist, total=%d len=%d", total2, len(events2))
	}
}

func testGetNotFound(t *testing.T, s store.RunLogStore) {
	got, err := s.Get("user1", "nonexistent")
	if err != nil {
		t.Fatalf("Get nonexistent: %v", err)
	}
	if got.Id != "" {
		t.Errorf("got.Id = %q, want empty", got.Id)
	}
}

func testUserIsolationSaveList(t *testing.T, s store.RunLogStore) {
	s.Save("iso-user1", TestEvent("iso-1", "chain-iso", time.Now()))
	events, total, _ := s.List("iso-user2", "chain-iso", time.Time{}, time.Time{}, 10, 1)
	if total != 0 || len(events) != 0 {
		t.Errorf("user2 should not see user1 data, total=%d", total)
	}
}

func testUserIsolationDelete(t *testing.T, s store.RunLogStore) {
	s.Save("iso2-user1", TestEvent("iso2-1", "chain-iso2", time.Now()))
	s.Delete("iso2-user2", "iso2-1")
	got, _ := s.Get("iso2-user1", "iso2-1")
	if got.Id != "iso2-1" {
		t.Errorf("user1 data should not be affected by user2 delete")
	}
}

func testUserIsolationDeleteByChainId(t *testing.T, s store.RunLogStore) {
	s.Save("iso3-user1", TestEvent("iso3-1", "chain-iso3", time.Now()))
	s.Save("iso3-user2", TestEvent("iso3-2", "chain-iso3", time.Now()))
	s.DeleteByChainId("iso3-user1", "chain-iso3")
	events, total, _ := s.List("iso3-user2", "chain-iso3", time.Time{}, time.Time{}, 10, 1)
	if total != 1 || len(events) != 1 {
		t.Errorf("user2 data should not be affected, total=%d", total)
	}
}

func testConcurrentWrite(t *testing.T, s store.RunLogStore) {
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				event := TestEvent("cw-"+string(rune('A'+g))+string(rune('0'+i%10)), "chain-cw", time.Now())
				s.Save("concurrent-user", event)
			}
		}(g)
	}
	wg.Wait()
	_, total, _ := s.List("concurrent-user", "chain-cw", time.Time{}, time.Time{}, 10000, 1)
	if total != 1000 {
		t.Errorf("total = %d, want 1000", total)
	}
}

func testConcurrentReadWrite(t *testing.T, s store.RunLogStore) {
	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 写入 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			event := TestEvent("crw-"+string(rune('A'+i%26)), "chain-crw", time.Now())
			s.Save("crw-user", event)
		}
		stop <- struct{}{}
	}()

	// 读取 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				s.List("crw-user", "chain-crw", time.Time{}, time.Time{}, 10, 1)
			}
		}
	}()

	wg.Wait()
}
