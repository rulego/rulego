package auditstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

func newTestStore(t *testing.T, cfg config.Config) *AuditLogStore {
	t.Helper()
	if cfg.DataDir == "" {
		cfg.DataDir = t.TempDir()
	}
	s, err := NewAuditLogStore(cfg, nil)
	if err != nil {
		t.Fatalf("NewAuditLogStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSaveAndList(t *testing.T) {
	s := newTestStore(t, config.Config{})
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	events := []model.AuditEvent{
		{Actor: "alice", Action: "rule:write", Target: "rule:a", Result: model.AuditResultOK, Ts: now.Add(-2 * time.Hour).UnixMilli()},
		{Actor: "bob", Action: "auth:login", Result: model.AuditResultDenied, Ts: now.Add(-1 * time.Hour).UnixMilli()},
		{Actor: "alice", Action: "rule:operate", Op: "deploy", Target: "rule:b", Result: model.AuditResultOK, Ts: now.UnixMilli()},
		{Actor: "alice", Action: "rule:write", Target: "rule:c", Result: model.AuditResultOK, Ts: yesterday.UnixMilli()},
	}
	for _, e := range events {
		if err := s.Save(e); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	// 全量倒序
	got, total, err := s.List(model.AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 {
		t.Fatalf("total = %d, want 4", total)
	}
	if got[0].Target != "rule:b" || got[3].Target != "rule:c" {
		t.Fatalf("not in desc order: first=%s last=%s", got[0].Target, got[3].Target)
	}

	// 过滤：actor / action / result / target 前缀 / 时间范围
	if _, total, _ := s.List(model.AuditFilter{Actor: "bob"}); total != 1 {
		t.Fatalf("filter actor total = %d, want 1", total)
	}
	if _, total, _ := s.List(model.AuditFilter{Action: "auth:login"}); total != 1 {
		t.Fatalf("filter action total = %d, want 1", total)
	}
	if _, total, _ := s.List(model.AuditFilter{Result: model.AuditResultDenied}); total != 1 {
		t.Fatalf("filter result total = %d, want 1", total)
	}
	if _, total, _ := s.List(model.AuditFilter{Target: "rule:"}); total != 3 {
		t.Fatalf("filter target prefix total = %d, want 3", total)
	}
	rangeFilter := model.AuditFilter{StartTime: now.Add(-90 * time.Minute), EndTime: now.Add(time.Minute)}
	if _, total, _ := s.List(rangeFilter); total != 2 {
		t.Fatalf("filter time range total = %d, want 2", total)
	}

	// 分页
	page2, total, err := s.List(model.AuditFilter{Size: 2, Page: 2})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if total != 4 || len(page2) != 2 {
		t.Fatalf("page2 = %d items, total %d; want 2 items, total 4", len(page2), total)
	}
	if page2[0].Target != "rule:a" {
		t.Fatalf("page2 first = %s, want rule:a", page2[0].Target)
	}
	// 超出页
	empty, total, _ := s.List(model.AuditFilter{Size: 2, Page: 3})
	if total != 4 || len(empty) != 0 {
		t.Fatalf("page3 = %d items, want 0", len(empty))
	}
}

// 掉电撕裂的末行（无换行的半截 JSON）应被跳过，不影响其余记录可查
func TestTornLastLine(t *testing.T) {
	s := newTestStore(t, config.Config{})
	now := time.Now()
	if err := s.Save(model.AuditEvent{Actor: "alice", Action: "auth:login", Result: model.AuditResultOK, Ts: now.UnixMilli()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Save(model.AuditEvent{Actor: "bob", Action: "auth:login", Result: model.AuditResultOK, Ts: now.Add(time.Second).UnixMilli()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	// 模拟半截写入
	f, err := os.OpenFile(s.filePath(now), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString(`{"ts":123,"actor":"ca`); err != nil {
		t.Fatalf("write torn line: %v", err)
	}
	_ = f.Close()

	_, total, err := s.List(model.AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 (torn line skipped)", total)
	}
}

func TestRetentionByDays(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t, config.Config{DataDir: dir, AuditRetentionDays: 1})

	old := time.Now().AddDate(0, 0, -3)
	if err := s.Save(model.AuditEvent{Actor: "alice", Action: "auth:login", Result: model.AuditResultOK, Ts: old.UnixMilli()}); err != nil {
		t.Fatalf("save old: %v", err)
	}
	if err := s.Save(model.AuditEvent{Actor: "bob", Action: "auth:login", Result: model.AuditResultOK, Ts: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("save today: %v", err)
	}

	s.cleanExpired()

	if _, err := os.Stat(s.filePath(old)); !os.IsNotExist(err) {
		t.Fatalf("expired file should be removed, stat err = %v", err)
	}
	_, total, _ := s.List(model.AuditFilter{})
	if total != 1 {
		t.Fatalf("total after clean = %d, want 1", total)
	}
}

func TestRetentionBySize(t *testing.T) {
	s := newTestStore(t, config.Config{})
	s.maxSizeBytes = 1500

	yesterday := time.Now().AddDate(0, 0, -1)
	big := strings.Repeat("x", 800)
	if err := s.Save(model.AuditEvent{Actor: "alice", Action: "rule:write", Detail: big, Result: model.AuditResultOK, Ts: yesterday.UnixMilli()}); err != nil {
		t.Fatalf("save old: %v", err)
	}
	if err := s.Save(model.AuditEvent{Actor: "bob", Action: "rule:write", Detail: big, Result: model.AuditResultOK, Ts: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("save today: %v", err)
	}

	s.cleanExpired()

	files, _ := s.listFiles()
	if len(files) != 1 || files[0].day.Format(dayLayout) != time.Now().Format(dayLayout) {
		t.Fatalf("size cap should keep only today's file, got %d files", len(files))
	}
}

// 当天的活跃文件即使超总量也不删
func TestRetentionNeverDeleteToday(t *testing.T) {
	s := newTestStore(t, config.Config{})
	s.maxSizeBytes = 1

	if err := s.Save(model.AuditEvent{Actor: "alice", Action: "auth:login", Result: model.AuditResultOK, Ts: time.Now().UnixMilli()}); err != nil {
		t.Fatalf("save: %v", err)
	}
	s.cleanExpired()

	if _, err := os.Stat(filepath.Join(s.dir(), filePrefix+time.Now().Format(dayLayout)+fileSuffix)); err != nil {
		t.Fatalf("today's file must survive size cap: %v", err)
	}
}

// 跨天文件的窗口切片与深页语义：
// 250 条跨两天事件，验证倒序全局序、窗口交集、深页空页返回正确总数、超深页不溢出
func TestListWindowAndDeepPage(t *testing.T) {
	s := newTestStore(t, config.Config{})
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	// 今天 200 条（新→旧 i=0..199），昨天 50 条
	for i := 0; i < 200; i++ {
		if err := s.Save(model.AuditEvent{Actor: "u", Action: "rule:write", Target: "today", Result: model.AuditResultOK, Ts: now.Add(-time.Duration(i) * time.Minute).UnixMilli()}); err != nil {
			t.Fatalf("save today: %v", err)
		}
	}
	for i := 0; i < 50; i++ {
		if err := s.Save(model.AuditEvent{Actor: "u", Action: "rule:write", Target: "yesterday", Result: model.AuditResultOK, Ts: yesterday.Add(-time.Duration(i) * time.Minute).UnixMilli()}); err != nil {
			t.Fatalf("save yesterday: %v", err)
		}
	}

	// 第一页：全局最新 20 条全在今天的文件里
	got, total, err := s.List(model.AuditFilter{Size: 20, Page: 1})
	if err != nil || total != 250 || len(got) != 20 {
		t.Fatalf("page1: total=%d len=%d err=%v", total, len(got), err)
	}
	if got[0].Target != "today" {
		t.Fatalf("page1[0] = %s, want today", got[0].Target)
	}

	// 跨天窗口：size=30 的第 7 页窗口 [180,210)，today 20 条 + yesterday 10 条
	got, total, err = s.List(model.AuditFilter{Size: 30, Page: 7})
	if err != nil || total != 250 || len(got) != 30 {
		t.Fatalf("page6x30: total=%d len=%d err=%v", total, len(got), err)
	}
	if got[0].Target != "today" || got[19].Target != "today" || got[20].Target != "yesterday" || got[29].Target != "yesterday" {
		t.Fatalf("page6x30 boundary wrong: %s/%s/%s/%s", got[0].Target, got[19].Target, got[20].Target, got[29].Target)
	}

	// 末页半页：250 条 20/页，第 13 页只有 10 条
	got, total, err = s.List(model.AuditFilter{Size: 20, Page: 13})
	if err != nil || total != 250 || len(got) != 10 {
		t.Fatalf("page13: total=%d len=%d err=%v", total, len(got), err)
	}

	// 超深页（page*size 溢出量级）：不 panic，空页 + 真实总数
	got, total, err = s.List(model.AuditFilter{Size: 20, Page: 14})
	if err != nil || total != 250 || len(got) != 0 {
		t.Fatalf("page14: total=%d len=%d err=%v", total, len(got), err)
	}
	got, total, err = s.List(model.AuditFilter{Size: 20, Page: 1e15})
	if err != nil || total != 250 || len(got) != 0 {
		t.Fatalf("huge page: total=%d len=%d err=%v", total, len(got), err)
	}
}

func TestZeroTsFilled(t *testing.T) {
	s := newTestStore(t, config.Config{})
	if err := s.Save(model.AuditEvent{Actor: "alice", Action: "auth:login", Result: model.AuditResultOK}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, total, _ := s.List(model.AuditFilter{})
	if total != 1 || got[0].Ts <= 0 {
		t.Fatalf("ts not filled: total=%d ts=%d", total, got[0].Ts)
	}
}
