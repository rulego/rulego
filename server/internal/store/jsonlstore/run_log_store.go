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
	s.mu.RLock()
	defer s.mu.RUnlock()

	if chainId != "" {
		return s.listByChain(username, chainId, startTime, endTime, size, page)
	}
	return s.listAll(username, startTime, endTime, size, page)
}

func (s *RunLogStore) listByChain(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	fp := s.filePath(username, chainId)
	events, err := s.readFile(fp)
	if err != nil {
		return nil, 0, nil
	}
	events = filterByDate(events, startTime, endTime)
	total := len(events)
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
		return model.Event{}, nil
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
	return model.Event{}, nil
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
	s.mu.Lock()
	defer s.mu.Unlock()

	fp := s.filePath(username, chainId)
	return os.Remove(fp)
}

// readFile 读取 jsonl 文件中的所有事件（倒序返回）
func (s *RunLogStore) readFile(fp string) ([]model.Event, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []model.Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		event, err := unmarshalLine(line)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	// 倒序（最新的在前）
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, scanner.Err()
}

// writeFile 重写 jsonl 文件
func (s *RunLogStore) writeFile(fp string, events []model.Event) error {
	if len(events) == 0 {
		return os.Remove(fp)
	}
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, e := range events {
		line, err := marshalLine(e)
		if err != nil {
			continue
		}
		if _, err := f.Write(line); err != nil {
			return err
		}
	}
	return nil
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
