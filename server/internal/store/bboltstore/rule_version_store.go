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

// RuleVersionStore 基于 BBolt 的规则链历史版本存储。
// 版本只在链保存时写入（人工操作频率），保留裁剪随保存直接执行。
type RuleVersionStore struct {
	cfg    config.Config
	logger types.Logger
	db     *bolt.DB
	mu     sync.RWMutex
}

// NewRuleVersionStore 创建 BBolt 规则链版本存储
func NewRuleVersionStore(cfg config.Config, logger types.Logger) (*RuleVersionStore, error) {
	if logger == nil {
		logger = types.DefaultLogger()
	}
	dbPath := filepath.Join(cfg.DataDir, constants.RuleVersionDbFile)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bbolt: %w", err)
	}
	return &RuleVersionStore{cfg: cfg, logger: logger, db: db}, nil
}

// Close 关闭数据库
func (s *RuleVersionStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

const versionBucketPrefix = "rulever:"

func versionBucketName(username string) []byte {
	return []byte(versionBucketPrefix + username)
}

// versionMakeKey 生成 key：{chainId}:{reverseTimestamp}_{versionId}
// reverseTimestamp 使 B+ 树按 key 升序天然倒序（最新在前）
func versionMakeKey(chainId, versionId string, ts int64) []byte {
	return []byte(fmt.Sprintf("%s:%d_%s", chainId, math.MaxInt64-ts, versionId))
}

func versionPrefix(chainId string) []byte {
	return []byte(chainId + ":")
}

// Save 保存版本快照，超出保留上限时裁掉最旧的。
// 版本号取链内现有最大 Seq+1（事务内扫描），单调递增且裁剪不回收——
// 重排会让同一个 v5 在不同时刻指向不同快照
func (s *RuleVersionStore) Save(username string, v model.RuleVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bName := versionBucketName(username)
	key := versionMakeKey(v.ChainId, v.Id, v.Ts)

	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bName)
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		maxSeq := 0
		c := bucket.Cursor()
		prefix := versionPrefix(v.ChainId)
		for k, val := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, val = c.Next() {
			var old model.RuleVersion
			if json.Unmarshal(val, &old) == nil && old.Seq > maxSeq {
				maxSeq = old.Seq
			}
		}
		v.Seq = maxSeq + 1
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal rule version: %w", err)
		}
		return bucket.Put(key, data)
	})
	if err != nil {
		return err
	}
	return s.trimLocked(username, v.ChainId)
}

// trimLocked 裁掉保留上限之外的旧版本。key 升序=新在前，从尾部删
func (s *RuleVersionStore) trimLocked(username, chainId string) error {
	maxCount := s.cfg.RuleVersionRetentionCount
	if maxCount <= 0 {
		return nil
	}
	bName := versionBucketName(username)
	prefix := versionPrefix(chainId)
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
		if len(keys) <= maxCount {
			return nil
		}
		for _, k := range keys[maxCount:] {
			_ = bucket.Delete(k)
		}
		return nil
	})
}

// List 按链倒序列出版本（最新在前），返回值不带 Dsl
func (s *RuleVersionStore) List(username, chainId string, size, page int) ([]model.RuleVersion, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	skipStart := (page - 1) * size

	var versions []model.RuleVersion
	var total int
	bName := versionBucketName(username)
	prefix := versionPrefix(chainId)

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var ver model.RuleVersion
			if err := json.Unmarshal(v, &ver); err != nil {
				continue
			}
			total++
			if total > skipStart && len(versions) < size {
				ver.Dsl = nil
				versions = append(versions, ver)
			}
		}
		return nil
	})
	return versions, total, err
}

// Get 获取单个版本（含 Dsl）
func (s *RuleVersionStore) Get(username, chainId, versionId string) (model.RuleVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var found model.RuleVersion
	bName := versionBucketName(username)
	prefix := versionPrefix(chainId)

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bName)
		if bucket == nil {
			return nil
		}
		c := bucket.Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var ver model.RuleVersion
			if err := json.Unmarshal(v, &ver); err == nil && ver.Id == versionId {
				found = ver
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return found, err
	}
	if found.Id == "" {
		return found, store.ErrRuleVersionNotFound
	}
	return found, nil
}

// DeleteByChainId 删除指定规则链的全部版本
func (s *RuleVersionStore) DeleteByChainId(username, chainId string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bName := versionBucketName(username)
	prefix := versionPrefix(chainId)
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

var _ store.RuleVersionStore = (*RuleVersionStore)(nil)
