// Package auditstore 管理面操作审计的 JSONL 存储：按天分文件、追加写、后台保留清理。
// 审计是平台级数据，落 DataDir/audit/（不进用户命名空间），删用户不影响审计记录。
package auditstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
)

const (
	filePrefix = "audit-"
	fileSuffix = ".jsonl"
	dayLayout  = "20060102"

	defaultRetentionDays = 90
	defaultMaxSizeMB     = 200
	// 单次查询最多定位到第 N 条命中事件（page*size 上限），更深的页只返回总数与空页
	maxScanWindow = 60000
	// 单行解析上限；正常事件经服务端截断远小于此，超限即损坏行
	maxLineLen = 1 << 20
)

type auditFile struct {
	path string
	day  time.Time
	size int64
}

// AuditLogStore 基于按天 JSONL 文件的审计存储。
type AuditLogStore struct {
	cfg           config.Config
	logger        types.Logger
	mu            sync.RWMutex
	stopCh        chan struct{}
	retentionDays int
	maxSizeBytes  int64
}

// NewAuditLogStore 创建审计存储并启动保留清理。保留参数 0 取默认值。
func NewAuditLogStore(cfg config.Config, logger types.Logger) (*AuditLogStore, error) {
	if logger == nil {
		logger = types.DefaultLogger()
	}
	days := cfg.AuditRetentionDays
	if days <= 0 {
		days = defaultRetentionDays
	}
	maxMB := cfg.AuditMaxSizeMB
	if maxMB <= 0 {
		maxMB = defaultMaxSizeMB
	}
	s := &AuditLogStore{
		cfg:           cfg,
		logger:        logger,
		stopCh:        make(chan struct{}),
		retentionDays: days,
		maxSizeBytes:  int64(maxMB) << 20,
	}
	if err := os.MkdirAll(s.dir(), 0755); err != nil {
		return nil, fmt.Errorf("create audit dir: %w", err)
	}
	go s.retentionLoop()
	return s, nil
}

func (s *AuditLogStore) dir() string {
	return filepath.Join(s.cfg.DataDir, constants.DirAudit)
}

func (s *AuditLogStore) filePath(day time.Time) string {
	return filepath.Join(s.dir(), filePrefix+day.Format(dayLayout)+fileSuffix)
}

func (s *AuditLogStore) Close() error {
	close(s.stopCh)
	return nil
}

// Save 追加一行 JSON。不逐条 fsync：掉电丢最后几秒对运维审计可接受，
// 读侧跳过解析失败的行（掉电撕裂的末行）。
func (s *AuditLogStore) Save(event model.AuditEvent) error {
	if event.Ts <= 0 {
		event.Ts = time.Now().UnixMilli()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.filePath(time.UnixMilli(event.Ts)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

// List 倒序流式扫描：只物化目标分页窗口内的事件，内存上界是单页大小而不是
// 全量（保留上限 200MB 的数据全量解析会顶出数百 MB 堆）。
func (s *AuditLogStore) List(filter model.AuditFilter) ([]model.AuditEvent, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	size := filter.Size
	if size <= 0 {
		size = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	// 超深页窗口置空：只返回总数与空页，封顶查询内存并避开 page*size 溢出
	var start, end int
	if p := int64(page-1) * int64(size); p <= maxScanWindow {
		start, end = int(p), int(p)+size
	}

	files, err := s.filesInRange(filter.StartTime, filter.EndTime)
	if err != nil {
		return nil, 0, err
	}

	events := make([]model.AuditEvent, 0, size)
	var seen int64
	// files 升序，倒序遍历使整体结果为新→旧
	for i := len(files) - 1; i >= 0; i-- {
		count, err := s.scanFile(files[i].path, filter, nil)
		if err != nil {
			return nil, 0, err
		}
		// 本文件命中事件占全局序号 [seen, seen+count)，与窗口的交集才需要物化
		if len(events) < end-start {
			if lo, hi := start-int(seen), end-int(seen); lo < hi {
				var batch []model.AuditEvent
				m := 0
				_, err = s.scanFile(files[i].path, filter, func(e model.AuditEvent) {
					if rank := int(count) - 1 - m; rank >= lo && rank < hi {
						batch = append(batch, e)
					}
					m++
				})
				if err != nil {
					return nil, 0, err
				}
				for j := len(batch) - 1; j >= 0; j-- {
					events = append(events, batch[j])
				}
			}
		}
		seen += count
	}
	return events, seen, nil
}

// filesInRange 返回时间范围命中的天文件（升序，List 侧倒序遍历）。
func (s *AuditLogStore) filesInRange(start, end time.Time) ([]auditFile, error) {
	files, err := s.listFiles()
	if err != nil {
		return nil, err
	}
	if start.IsZero() && end.IsZero() {
		return files, nil
	}
	var out []auditFile
	for _, af := range files {
		dayEnd := af.day.Add(24 * time.Hour)
		if !start.IsZero() && dayEnd.UnixMilli() < start.UnixMilli() {
			continue
		}
		if !end.IsZero() && af.day.UnixMilli() > end.UnixMilli() {
			continue
		}
		out = append(out, af)
	}
	return out, nil
}

// listFiles 列出审计目录全部天文件，按日期升序。
func (s *AuditLogStore) listFiles() ([]auditFile, error) {
	entries, err := os.ReadDir(s.dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []auditFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), filePrefix) || !strings.HasSuffix(entry.Name(), fileSuffix) {
			continue
		}
		day, err := time.ParseInLocation(dayLayout, strings.TrimSuffix(strings.TrimPrefix(entry.Name(), filePrefix), fileSuffix), time.Local)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, auditFile{path: filepath.Join(s.dir(), entry.Name()), day: day, size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].day.Before(files[j].day) })
	return files, nil
}

// scanFile 流式扫描单个天文件，对命中事件逐条回调 visit（可为 nil 仅计数），
// 返回命中总数。损坏行（掉电撕裂）跳过；文件已被保留策略删除按 0 处理。
func (s *AuditLogStore) scanFile(fp string, filter model.AuditFilter, visit func(model.AuditEvent)) (int64, error) {
	f, err := os.Open(fp)
	if err != nil {
		return 0, nil
	}
	defer f.Close()

	var n int64
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineLen)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e model.AuditEvent
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if !matchAuditEvent(filter, e) {
			continue
		}
		n++
		if visit != nil {
			visit(e)
		}
	}
	return n, nil
}

func matchAuditEvent(f model.AuditFilter, e model.AuditEvent) bool {
	if !f.StartTime.IsZero() && e.Ts < f.StartTime.UnixMilli() {
		return false
	}
	if !f.EndTime.IsZero() && e.Ts > f.EndTime.UnixMilli() {
		return false
	}
	if f.Actor != "" && e.Actor != f.Actor {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if f.Result != "" && e.Result != f.Result {
		return false
	}
	if f.Target != "" && !strings.HasPrefix(e.Target, f.Target) {
		return false
	}
	return true
}

func (s *AuditLogStore) retentionLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanExpired()
		}
	}
}

// cleanExpired 先删超期天文件，再按总量从最旧删起；当天的活跃文件不删。
func (s *AuditLogStore) cleanExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := s.listFiles()
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	today := time.Now().Format(dayLayout)
	var kept []auditFile
	var total int64
	for _, af := range files {
		if af.day.Before(cutoff) {
			s.removeFile(af.path)
			continue
		}
		kept = append(kept, af)
		total += af.size
	}
	for i := 0; i < len(kept) && total > s.maxSizeBytes; i++ {
		if kept[i].day.Format(dayLayout) == today {
			continue
		}
		total -= kept[i].size
		s.removeFile(kept[i].path)
	}
}

func (s *AuditLogStore) removeFile(fp string) {
	if err := os.Remove(fp); err != nil {
		s.logger.Debugf("remove audit file %s: %s", fp, err.Error())
	}
}
