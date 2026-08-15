package jsonlstore

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
	"github.com/rulego/rulego/server/internal/utils/file"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
)

const lineSeparator = '\n'

// RunLogStore 基于 JSON Lines 文件的运行日志存储，每个 chainId 一个文件。
type RunLogStore struct {
	cfg    config.Config
	logger types.Logger
	mu     sync.RWMutex
	stopCh chan struct{}
}

func NewRunLogStore(cfg config.Config, logger types.Logger) (*RunLogStore, error) {
	if logger == nil {
		logger = types.DefaultLogger()
	}
	s := &RunLogStore{
		cfg:    cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	go s.retentionLoop()
	return s, nil
}

func (s *RunLogStore) Close() error {
	close(s.stopCh)
	return nil
}

func (s *RunLogStore) filePath(username, chainId string) string {
	return filepath.Join(s.cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRun, chainId+constants.RunLogFileSuffix)
}

func (s *RunLogStore) userDir(username string) string {
	return filepath.Join(s.cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRun)
}

// Save 保存运行日志（仅 append 写入，不做清理，清理由后台 goroutine 负责）
func (s *RunLogStore) Save(username string, event model.Event) error {
	if !constants.IsSafeId(event.ChainId) {
		return fmt.Errorf("invalid chain id: %s", event.ChainId)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	fp := s.filePath(username, event.ChainId)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	line, err := marshalLine(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
}

// List 列出运行日志，支持按 chainId、时间范围过滤和分页
func (s *RunLogStore) List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	if chainId != "" && !constants.IsSafeId(chainId) {
		return nil, 0, fmt.Errorf("invalid chain id: %s", chainId)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if chainId != "" {
		return s.listByChain(username, chainId, startTime, endTime, size, page)
	}
	return s.listAll(username, startTime, endTime, size, page)
}

func (s *RunLogStore) listByChain(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	fp := s.filePath(username, chainId)
	events, total, err := s.readEventsReverse(fp, page*size, startTime, endTime)
	if err != nil {
		return nil, 0, nil
	}
	start := (page - 1) * size
	if start >= total {
		return nil, total, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return events[start:end], total, nil
}

func (s *RunLogStore) listAll(username string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	dir := s.userDir(username)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, nil
	}

	var allEvents []model.Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), constants.RunLogFileSuffix) {
			continue
		}
		fp := filepath.Join(dir, entry.Name())
		events, err := s.readFile(fp)
		if err != nil {
			continue
		}
		allEvents = append(allEvents, events...)
	}

	allEvents = filterByDate(allEvents, startTime, endTime)

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].StartTs > allEvents[j].StartTs
	})

	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	total := len(allEvents)
	start := (page - 1) * size
	if start >= total {
		return nil, total, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	return allEvents[start:end], total, nil
}

// Get 获取单条运行日志
func (s *RunLogStore) Get(username, logId string) (model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := s.userDir(username)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return model.Event{}, store.ErrRunLogNotFound
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), constants.RunLogFileSuffix) {
			continue
		}
		fp := filepath.Join(dir, entry.Name())
		events, err := s.readFile(fp)
		if err != nil {
			continue
		}
		for _, e := range events {
			if e.Id == logId {
				return e, nil
			}
		}
	}
	return model.Event{}, store.ErrRunLogNotFound
}

// Delete 删除运行日志
func (s *RunLogStore) Delete(username, logId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.userDir(username)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), constants.RunLogFileSuffix) {
			continue
		}
		fp := filepath.Join(dir, entry.Name())
		events, err := s.readFile(fp)
		if err != nil {
			continue
		}
		for i, e := range events {
			if e.Id == logId {
				events = append(events[:i], events[i+1:]...)
				return s.writeFile(fp, events)
			}
		}
	}
	return nil
}

// DeleteByChainId 删除指定规则链的所有运行日志
func (s *RunLogStore) DeleteByChainId(username, chainId string) error {
	if !constants.IsSafeId(chainId) {
		return fmt.Errorf("invalid chain id: %s", chainId)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	fp := s.filePath(username, chainId)
	return os.Remove(fp)
}

// readFile 读取 jsonl 文件中的所有事件（倒序返回）
// maxLineSize 单行事件上限：超过则跳过该行。detail 级事件可能超过
// bufio.Scanner 的行缓冲上限，Scanner 遇超长行直接中断导致整表丢弃。
const maxLineSize = 10 * 1024 * 1024

func (s *RunLogStore) readFile(fp string) ([]model.Event, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []model.Event
	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		line, rerr := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) > 0 && len(trimmed) <= maxLineSize {
				if event, uerr := unmarshalLine(trimmed); uerr == nil {
					events = append(events, event)
				}
			}
		}
		if rerr != nil {
			break
		}
	}

	// 倒序（最新的在前）
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

// readEventsReverse 从文件尾部向前扫描，返回时间新→旧、落在 [startTime,endTime]
// 内的前 max 条，以及范围内总条数。total 需要精确计数因此会扫到文件尾
//（保留策略下文件行数有限，IO 可接受），但只保留最近 max 条在内存——
// detail 级事件单条可达 MB，全量持有才是 OOM 根源。文件按完成顺序追加，
// 倒序即近似时间倒序；结果再按 StartTs 排序兜住乱序写入，startTime
// 早于范围时提前终止。
func (s *RunLogStore) readEventsReverse(fp string, max int, startTime, endTime time.Time) ([]model.Event, int, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, 0, nil // 文件不存在按空处理
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return nil, 0, err
	}
	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	var carry []byte
	var matched []model.Event
	total := 0
	pos := st.Size()
	process := func(line []byte) bool { // 返回 false 表示可停止扫描
		trimmed := bytes.TrimRight(line, "\r")
		if len(trimmed) == 0 || len(trimmed) > maxLineSize {
			return true
		}
		e, uerr := unmarshalLine(trimmed)
		if uerr != nil {
			return true
		}
		if !startTime.IsZero() && e.StartTs < startTime.UnixMilli() {
			return false
		}
		if !endTime.IsZero() && e.StartTs > endTime.UnixMilli() {
			return true
		}
		total++
		if len(matched) < max {
			matched = append(matched, e)
		}
		return true
	}
	for pos > 0 {
		n := int64(chunkSize)
		if n > pos {
			n = pos
		}
		pos -= n
		if _, rerr := f.ReadAt(buf[:n], pos); rerr != nil {
			return nil, 0, rerr
		}
		data := buf[:n]
		if carry != nil {
			data = append(data, carry...)
		}
		lines := bytes.Split(data, []byte{'\n'})
		if pos == 0 {
			carry = nil
		} else {
			carry = append(carry[:0], lines[0]...)
		}
		stop := false
		for i := len(lines) - 1; i >= 1 && !stop; i-- {
			if !process(lines[i]) {
				stop = true
			}
		}
		if pos == 0 && !stop {
			process(lines[0])
		}
		if stop {
			break
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].StartTs > matched[j].StartTs })
	return matched, total, nil
}

// writeFile 重写 jsonl 文件（原子替换，避免清理线程崩溃时整文件损坏）
func (s *RunLogStore) writeFile(fp string, events []model.Event) error {
	if len(events) == 0 {
		return os.Remove(fp)
	}
	var buf bytes.Buffer
	for _, e := range events {
		line, err := marshalLine(e)
		if err != nil {
			continue
		}
		buf.Write(line)
	}
	return file.WriteFileAtomic(fp, buf.Bytes(), 0o644)
}

// retentionLoop 后台定期清理：按天数和按条数
func (s *RunLogStore) retentionLoop() {
	ticker := time.NewTicker(1 * time.Hour)
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

// cleanExpired 后台清理过期和超量数据，全程加写锁保证一致
func (s *RunLogStore) cleanExpired() {
	maxDays := s.cfg.RunLogRetentionDays
	maxCount := s.cfg.RunLogRetentionCount
	if maxDays <= 0 && maxCount <= 0 {
		return
	}

	workflowsDir := filepath.Join(s.cfg.DataDir, constants.DirWorkflows)
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var cutoff time.Time
	if maxDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -maxDays)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		username := entry.Name()
		runsDir := filepath.Join(workflowsDir, username, constants.DirWorkflowsRun)
		runEntries, err := os.ReadDir(runsDir)
		if err != nil {
			continue
		}
		for _, re := range runEntries {
			if re.IsDir() || !strings.HasSuffix(re.Name(), ".jsonl") {
				continue
			}
			fp := filepath.Join(runsDir, re.Name())
			s.cleanFile(fp, cutoff, maxCount)
		}
	}
}

func (s *RunLogStore) cleanFile(fp string, cutoff time.Time, maxCount int) {
	events, err := s.readFile(fp)
	if err != nil {
		return
	}

	var kept []model.Event
	for _, e := range events {
		if maxCount > 0 && len(kept) >= maxCount {
			break
		}
		if !cutoff.IsZero() && e.StartTs < cutoff.UnixMilli() {
			continue
		}
		kept = append(kept, e)
	}

	if len(kept) < len(events) {
		_ = s.writeFile(fp, kept)
	}
}

func marshalLine(event model.Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return append(data, lineSeparator), nil
}

func filterByDate(events []model.Event, startTime, endTime time.Time) []model.Event {
	if startTime.IsZero() && endTime.IsZero() {
		return events
	}
	filtered := make([]model.Event, 0, len(events))
	for _, e := range events {
		if !startTime.IsZero() && e.StartTs < startTime.UnixMilli() {
			continue
		}
		if !endTime.IsZero() && e.StartTs > endTime.UnixMilli() {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func unmarshalLine(data []byte) (model.Event, error) {
	var event model.Event
	data = bytes.TrimRight(data, "\r\n")
	if len(data) == 0 {
		return event, nil
	}
	err := json.Unmarshal(data, &event)
	return event, err
}

var _ store.RunLogStore = (*RunLogStore)(nil)
