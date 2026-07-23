package filestore

import (
	"fmt"
	"path"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/fs"
)

// FileStoreProvider is a default StoreProvider implementation based on files.
// Built-in per-user cache prevents duplicate Store instance creation.
type FileStoreProvider struct {
	cfg    config.Config
	logger types.Logger

	ruleStoreMu         sync.RWMutex
	ruleStoreCache      map[string]store.RuleStore
	settingStoreMu      sync.RWMutex
	settingStoreCache   map[string]store.SettingStore
	componentStoreMu    sync.RWMutex
	componentStoreCache map[string]store.ComponentStore
	nodePoolStoreMu     sync.RWMutex
	nodePoolStoreCache  map[string]store.NodePoolStore

	userStoreOnce sync.Once
	userStore     store.UserStore
	userStoreErr  error

	runLogStore store.RunLogStore
}

// NewFileStoreProvider creates a file storage provider
func NewFileStoreProvider(cfg config.Config, logger types.Logger) *FileStoreProvider {
	return &FileStoreProvider{
		cfg:                 cfg,
		logger:              logger,
		ruleStoreCache:      make(map[string]store.RuleStore),
		settingStoreCache:   make(map[string]store.SettingStore),
		componentStoreCache: make(map[string]store.ComponentStore),
		nodePoolStoreCache:  make(map[string]store.NodePoolStore),
	}
}

func (p *FileStoreProvider) GetRuleStore(username string) (store.RuleStore, error) {
	key := p.cfg.DataDir + ":" + username
	p.ruleStoreMu.RLock()
	if s, ok := p.ruleStoreCache[key]; ok {
		p.ruleStoreMu.RUnlock()
		return s, nil
	}
	p.ruleStoreMu.RUnlock()

	folderPath := path.Join(p.cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule)
	_ = fs.CreateDirs(folderPath)

	s, err := NewRuleStore(p.cfg, username)
	if err != nil {
		return nil, err
	}
	p.ruleStoreMu.Lock()
	p.ruleStoreCache[key] = s
	p.ruleStoreMu.Unlock()
	return s, nil
}

func (p *FileStoreProvider) GetSettingStore(username string) (store.SettingStore, error) {
	key := p.cfg.DataDir + ":" + username
	p.settingStoreMu.RLock()
	if s, ok := p.settingStoreCache[key]; ok {
		p.settingStoreMu.RUnlock()
		return s, nil
	}
	p.settingStoreMu.RUnlock()

	dirPath := path.Join(p.cfg.DataDir, constants.DirWorkflows, username)
	_ = fs.CreateDirs(dirPath)

	s, err := NewSettingStore(p.cfg, dirPath)
	if err != nil {
		return nil, err
	}
	p.settingStoreMu.Lock()
	p.settingStoreCache[key] = s
	p.settingStoreMu.Unlock()
	return s, nil
}

func (p *FileStoreProvider) GetComponentStore(username string) (store.ComponentStore, error) {
	key := p.cfg.DataDir + ":" + username
	p.componentStoreMu.RLock()
	if s, ok := p.componentStoreCache[key]; ok {
		p.componentStoreMu.RUnlock()
		return s, nil
	}
	p.componentStoreMu.RUnlock()

	folderPath := path.Join(p.cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)

	s, err := NewComponentStore(p.cfg, username)
	if err != nil {
		return nil, err
	}
	p.componentStoreMu.Lock()
	p.componentStoreCache[key] = s
	p.componentStoreMu.Unlock()
	return s, nil
}

func (p *FileStoreProvider) GetNodePoolStore(username string) (store.NodePoolStore, error) {
	key := p.cfg.DataDir + ":" + username
	p.nodePoolStoreMu.RLock()
	if s, ok := p.nodePoolStoreCache[key]; ok {
		p.nodePoolStoreMu.RUnlock()
		return s, nil
	}
	p.nodePoolStoreMu.RUnlock()

	s, err := NewNodePoolStore(p.cfg, username)
	if err != nil {
		return nil, err
	}
	p.nodePoolStoreMu.Lock()
	p.nodePoolStoreCache[key] = s
	p.nodePoolStoreMu.Unlock()
	return s, nil
}

func (p *FileStoreProvider) GetUserStore() (store.UserStore, error) {
	p.userStoreOnce.Do(func() {
		p.userStore, p.userStoreErr = NewUserStore(p.cfg)
	})
	if p.userStore == nil && p.userStoreErr != nil {
		return nil, fmt.Errorf("user store not initialized: %w", p.userStoreErr)
	}
	return p.userStore, p.userStoreErr
}

// SetRunLogStore sets the externally injected RunLogStore
func (p *FileStoreProvider) SetRunLogStore(s store.RunLogStore) {
	p.runLogStore = s
}

func (p *FileStoreProvider) GetRunLogStore() (store.RunLogStore, error) {
	if p.runLogStore != nil {
		return p.runLogStore, nil
	}
	return nil, fmt.Errorf("run log store not configured")
}

// Close: Close Closing RunLogStore that can be closed (such as BBolt)
func (p *FileStoreProvider) Close() {
	if p.runLogStore != nil {
		if c, ok := p.runLogStore.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
}
