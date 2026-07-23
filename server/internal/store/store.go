package store

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/store/filestore"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/fs"
)

var (
	ruleStoreCache      = make(map[string]store.RuleStore)
	ruleStoreLock       sync.RWMutex
	settingStoreCache   = make(map[string]store.SettingStore)
	settingStoreLock    sync.RWMutex
	componentStoreCache = make(map[string]store.ComponentStore)
	componentStoreLock  sync.RWMutex
	nodePoolStoreCache  = make(map[string]store.NodePoolStore)
	nodePoolStoreLock   sync.RWMutex
	runLogStoreInstance store.RunLogStore
	runLogStoreOnce     sync.Once
)

// GetRuleStore Retrieves the rule chain storage instance (with cache)
func GetRuleStore(cfg *config.Config, username string) (store.RuleStore, error) {
	key := cfg.DataDir + ":" + username
	ruleStoreLock.RLock()
	if s, ok := ruleStoreCache[key]; ok {
		ruleStoreLock.RUnlock()
		return s, nil
	}
	ruleStoreLock.RUnlock()

	folderPath := path.Join(cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsRule)
	_ = fs.CreateDirs(folderPath)

	s, err := filestore.NewRuleStore(*cfg, username)
	if err != nil {
		return nil, err
	}

	ruleStoreLock.Lock()
	ruleStoreCache[key] = s
	ruleStoreLock.Unlock()
	return s, nil
}

// GetSettingStore Retrieves user settings storage instances (with cache)
func GetSettingStore(cfg *config.Config, username string) (store.SettingStore, error) {
	key := cfg.DataDir + ":" + username
	settingStoreLock.RLock()
	if s, ok := settingStoreCache[key]; ok {
		settingStoreLock.RUnlock()
		return s, nil
	}
	settingStoreLock.RUnlock()

	dirPath := path.Join(cfg.DataDir, constants.DirWorkflows, username)
	_ = fs.CreateDirs(dirPath)

	s, err := filestore.NewSettingStore(*cfg, dirPath)
	if err != nil {
		return nil, err
	}

	settingStoreLock.Lock()
	settingStoreCache[key] = s
	settingStoreLock.Unlock()
	return s, nil
}

// GetComponentStore Retrieves component storage instances (with cache)
func GetComponentStore(cfg *config.Config, username string) (store.ComponentStore, error) {
	key := cfg.DataDir + ":" + username
	componentStoreLock.RLock()
	if s, ok := componentStoreCache[key]; ok {
		componentStoreLock.RUnlock()
		return s, nil
	}
	componentStoreLock.RUnlock()

	folderPath := path.Join(cfg.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)

	s, err := filestore.NewComponentStore(*cfg, username)
	if err != nil {
		return nil, err
	}

	componentStoreLock.Lock()
	componentStoreCache[key] = s
	componentStoreLock.Unlock()
	return s, nil
}

// GetNodePoolStore Gets node pool storage instances (with cache)
func GetNodePoolStore(cfg *config.Config, username string) (store.NodePoolStore, error) {
	key := cfg.DataDir + ":" + username
	nodePoolStoreLock.RLock()
	if s, ok := nodePoolStoreCache[key]; ok {
		nodePoolStoreLock.RUnlock()
		return s, nil
	}
	nodePoolStoreLock.RUnlock()

	s, err := filestore.NewNodePoolStore(*cfg, username)
	if err != nil {
		return nil, err
	}

	nodePoolStoreLock.Lock()
	nodePoolStoreCache[key] = s
	nodePoolStoreLock.Unlock()
	return s, nil
}

// SetRunLogStore sets the runtime log storage instance (injected at application startup)
func SetRunLogStore(s store.RunLogStore) {
	runLogStoreInstance = s
}

// GetRunLogStore retrieves the runtime log storage instance
func GetRunLogStore() (store.RunLogStore, error) {
	if runLogStoreInstance == nil {
		return nil, fmt.Errorf("run log store not initialized")
	}
	return runLogStoreInstance, nil
}

// GetUserStore Retrieves user storage instances (global singletons)
func GetUserStore(cfg *config.Config) (store.UserStore, error) {
	return filestore.NewUserStore(*cfg)
}

// CleanUserCache clears the storage cache of the specified user
func CleanUserCache(cfg *config.Config, username string) {
	key := cfg.DataDir + ":" + username
	ruleStoreLock.Lock()
	delete(ruleStoreCache, key)
	ruleStoreLock.Unlock()

	settingStoreLock.Lock()
	delete(settingStoreCache, key)
	settingStoreLock.Unlock()

	componentStoreLock.Lock()
	delete(componentStoreCache, key)
	componentStoreLock.Unlock()

	nodePoolStoreLock.Lock()
	delete(nodePoolStoreCache, key)
	nodePoolStoreLock.Unlock()

	// Delete the user data directory
	_ = os.RemoveAll(path.Join(cfg.DataDir, constants.DirWorkflows, username))
}
