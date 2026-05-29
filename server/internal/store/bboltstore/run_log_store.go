package bboltstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/store"
	bolt "go.etcd.io/bbolt"
)

const (
	// bucketPrefix 运行日志 bucket 前缀
	bucketPrefix = "runlog:"
)

// RunLogStore 基于 BBolt 的运行日志存储实现。
type RunLogStore struct {
	cfg    config.Config
	logger types.Logger
	db     *bolt.DB
	mu     sync.RWMutex
	stopCh chan struct{}
}

// NewRunLogStore 创建 BBolt 运行日志存储
func NewRunLogStore(cfg config.Config, logger types.Logger) (*RunLogStore, error) {
	if logger == nil {
		logger = types.DefaultLogger()
	}
	dbPath := filepath.Join(cfg.DataDir, constants.RunLogDbFile)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt: %w", err)
	}
	s := &RunLogStore{
		cfg:    cfg,
		logger: logger,
		db:     db,
		stopCh: make(chan struct{}),
	}
	go s.retentionLoop()
	return s, nil
}

// Close 关闭数据库，停止后台 goroutine
func (s *RunLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.stopCh)
	return s.db.Close()
}

func bucketName(username string) []byte {
	return []byte(bucketPrefix + username)
}

// makeKey 生成 BBolt key：{chainId}:{reverseTimestamp}_{snapshotId}
// reverseTimestamp 使 BBolt B+ 树按 key 有序时天然倒序（最新在前）
func makeKey(chainId, snapshotId string) []byte {
	ts := math.MaxInt64 - time.Now().UnixNano()
	return []byte(fmt.Sprintf("%s:%d_%s", chainId, ts, snapshotId))
}

// Save 保存运行日志
func (s *RunLogStore) Save(username string, event model.Event) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bName := bucketName(username)
	key := makeKey(event.ChainId, event.Id)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bName)
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		if err := bucket.Put(key, data); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.lazyRetainCount(username, event.ChainId)
	return nil
}

// List 列出运行日志，支持按 chainId、时间范围过滤和分页
func (s *RunLogStore) List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bName := bucketName(username)
	var events []model.Event
	var total int

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		skipStart := (page - 1) * size

		if chainId != "" {
			prefix := []byte(chainId + ":")
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				e, err := unmarshalEvent(v)
				if err != nil {
					continue
				}
				if !startTime.IsZero() && e.StartTs < startTime.UnixMilli() {
					break // 按 reverseTimestamp 倒序，后续全比 startTime 更早
				}
				if !endTime.IsZero() && e.StartTs > endTime.UnixMilli() {
					continue
				}
				total++
				if total > skipStart && len(events) < size {
					events = append(events, e)
				}
			}
		} else {
			for k, v := c.First(); k != nil; k, v = c.Next() {
				e, err := unmarshalEvent(v)
				if err != nil {
					continue
				}
				if !startTime.IsZero() && e.StartTs < startTime.UnixMilli() {
					continue
				}
				if !endTime.IsZero() && e.StartTs > endTime.UnixMilli() {
					continue
				}
				total++
				if total > skipStart && len(events) < size {
					events = append(events, e)
				}
			}
		}
		return nil
	})

	return events, total, err
}

// Get 获取单条运行日志
func (s *RunLogStore) Get(username, logId string) (model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bName := bucketName(username)
	var event model.Event

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if e, err := unmarshalEvent(v); err == nil && e.Id == logId {
				event = e
				return nil
			}
		}
		return nil
	})

	return event, err
}

// Delete 删除运行日志
func (s *RunLogStore) Delete(username, logId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bName := bucketName(username)
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if e, err := unmarshalEvent(v); err == nil && e.Id == logId {
				return bucket.Delete(k)
			}
		}
		return nil
	})
}

// DeleteByChainId 删除指定规则链的所有运行日志
func (s *RunLogStore) DeleteByChainId(username, chainId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bName := bucketName(username)
	prefix := []byte(chainId + ":")

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		var keys [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keys = append(keys, k)
		}
		for _, k := range keys {
			_ = bucket.Delete(k)
		}
		return nil
	})
}

// lazyRetainCount 惰性清理：当 chainId 的记录数超过限制时，删除最旧的记录。
// key 格式 {chainId}:{reverseTs}_{id}，reverseTs 使 key 升序排列时最新在前。
// 所以 keys[0] 是最新的，keys[len-1] 是最旧的，应从尾部删除。
func (s *RunLogStore) lazyRetainCount(username, chainId string) {
	maxCount := s.cfg.RunLogRetentionCount
	if maxCount <= 0 {
		return
	}

	bName := bucketName(username)
	prefix := []byte(chainId + ":")

	_ = s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		var keys [][]byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keys = append(keys, k)
		}
		// 超出限制，从尾部删除最旧的记录
		if len(keys) > maxCount {
			deleteCount := len(keys) - maxCount
			for i := len(keys) - deleteCount; i < len(keys); i++ {
				_ = bucket.Delete(keys[i])
			}
		}
		return nil
	})
}

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

// cleanExpired 清理过期数据，加写锁保护
func (s *RunLogStore) cleanExpired() {
	maxDays := s.cfg.RunLogRetentionDays
	if maxDays <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -maxDays)

	_ = s.db.Update(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, bucket *bolt.Bucket) error {
			if !bytes.HasPrefix(name, []byte(bucketPrefix)) {
				return nil
			}
			c := bucket.Cursor()
			var keys [][]byte
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if e, err := unmarshalEvent(v); err == nil && e.StartTs < cutoff.UnixMilli() {
					keys = append(keys, k)
				}
			}
			for _, k := range keys {
				_ = bucket.Delete(k)
			}
			return nil
		})
	})
}

func unmarshalEvent(data []byte) (model.Event, error) {
	var event model.Event
	if len(data) == 0 {
		return event, nil
	}
	err := json.Unmarshal(data, &event)
	return event, err
}

var _ store.RunLogStore = (*RunLogStore)(nil)
