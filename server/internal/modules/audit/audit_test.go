package audit

import (
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/store/auditstore"
	"github.com/rulego/rulego/server/model"
)

func newTestService(t *testing.T) (*auditService, *auditstore.AuditLogStore) {
	t.Helper()
	cfg := config.Config{DataDir: t.TempDir()}
	s, err := auditstore.NewAuditLogStore(cfg, nil)
	if err != nil {
		t.Fatalf("new audit store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	svc := &auditService{writer: newAsyncAuditWriter(s, nil)}
	return svc, s
}

func TestAsyncWriterFlush(t *testing.T) {
	svc, store := newTestService(t)
	svc.writer.Start()

	for i := 0; i < 5; i++ {
		svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Target: "rule:a", Result: model.AuditResultOK})
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, total, _ := store.List(model.AuditFilter{})
		if total == 5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("flush timeout: total = %d, want 5", total)
		}
		time.Sleep(10 * time.Millisecond)
	}
	svc.writer.Stop()
}

// 队列满必须丢弃并计数，不阻塞调用方
func TestAsyncWriterDropOnFull(t *testing.T) {
	svc, _ := newTestService(t)
	// 不 Start，队列只进不出
	for i := 0; i < queueSize; i++ {
		svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Result: model.AuditResultOK})
	}
	svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Result: model.AuditResultOK})
	if got := svc.writer.Dropped(); got != 1 {
		t.Fatalf("dropped = %d, want 1", got)
	}
}

// Stop 同步刷完剩余队列——停机不丢审计事件
func TestStopFlushesQueue(t *testing.T) {
	svc, store := newTestService(t)
	// 不 Start：模拟 worker 已退出后仍有积压的场景
	for i := 0; i < 3; i++ {
		svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Result: model.AuditResultOK})
	}
	svc.writer.Stop()

	_, total, err := store.List(model.AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Fatalf("total after stop flush = %d, want 3", total)
	}
}

// 攻击者可控字段（登录尝试用户名、XFF 头）超长必须截断，防单行撑爆审计文件
func TestRecordTruncatesFields(t *testing.T) {
	svc, store := newTestService(t)
	svc.writer.Start()
	svc.Record(model.AuditEvent{
		Actor:  strings.Repeat("u", 10000),
		IP:     strings.Repeat("1", 10000),
		UA:     strings.Repeat("U", 10000),
		Action: "rule:write",
		Target: strings.Repeat("t", 10000),
		Detail: strings.Repeat("详", 10000),
		Result: model.AuditResultOK,
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, total, _ := store.List(model.AuditFilter{})
		if total == 1 {
			if len([]rune(got[0].Actor)) != maxActorLen {
				t.Fatalf("actor len = %d, want %d", len([]rune(got[0].Actor)), maxActorLen)
			}
			if len([]rune(got[0].IP)) != maxIPLen {
				t.Fatalf("ip len = %d, want %d", len([]rune(got[0].IP)), maxIPLen)
			}
			if len([]rune(got[0].UA)) != maxUALen {
				t.Fatalf("ua len = %d, want %d", len([]rune(got[0].UA)), maxUALen)
			}
			if len([]rune(got[0].Target)) != maxTargetLen {
				t.Fatalf("target len = %d, want %d", len([]rune(got[0].Target)), maxTargetLen)
			}
			if len([]rune(got[0].Detail)) != maxDetailLen {
				t.Fatalf("detail len = %d, want %d", len([]rune(got[0].Detail)), maxDetailLen)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for event")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 审计关闭（writer 为 nil）时 Record 静默跳过、List 返回空
func TestServiceDisabled(t *testing.T) {
	svc := &auditService{}
	svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Result: model.AuditResultOK})
	got, total, err := svc.List(model.AuditFilter{})
	if err != nil || total != 0 || len(got) != 0 {
		t.Fatalf("disabled service should no-op: total=%d err=%v", total, err)
	}
}

// Record 补零值 Ts
func TestRecordFillsTs(t *testing.T) {
	svc, store := newTestService(t)
	svc.writer.Start()
	svc.Record(model.AuditEvent{Actor: "alice", Action: "rule:write", Result: model.AuditResultOK})
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, total, _ := store.List(model.AuditFilter{})
		if total == 1 {
			if got[0].Ts <= 0 {
				t.Fatalf("ts should be filled")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for event")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
