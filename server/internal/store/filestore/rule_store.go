package filestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/str"
)

// RuleStore is implemented based on the rule chain storage of the file system.
// Define using JSON file storage rule chains and maintain indexed file acceleration list queries.
type RuleStore struct {
	config   config.Config
	username string
	index    RuleIndex
	sync.RWMutex
}

// RuleIndex: Rules chain index, containing only necessary metadata for quick list queries
type RuleIndex struct {
	Rules map[string]RuleMeta `json:"rules"`
}

// RuleMeta Rule Chain Metadata
type RuleMeta struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Root        bool   `json:"root"`
	Disabled    bool   `json:"disabled"`
	UpdateTime  string `json:"updateTime"`
	Category    string `json:"category"`
	SystemAgent bool   `json:"systemAgent"`
}

// NewRuleStore creates a rule chain file storage.
// If the index file does not exist, it will automatically scan the rule chain file to reconstruct the index.
func NewRuleStore(cfg config.Config, username string) (*RuleStore, error) {
	store := &RuleStore{
		config:   cfg,
		username: username,
		index:    RuleIndex{Rules: make(map[string]RuleMeta)},
	}

	indexPath := store.getIndexPath()
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return store, store.rebuildIndex()
	} else if err != nil {
		return nil, err
	}
	if err := store.loadIndex(indexPath); err != nil {
		return nil, err
	}

	return store, nil
}

// Save the rule chain defined in the file and update the index
func (d *RuleStore) Save(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	if err := d.saveRuleChain(username, chainId, def); err != nil {
		return err
	}
	d.createIndex(ruleChain)
	return d.saveIndex(d.getIndexPath())
}

// Get the original JSON data of the rule chain
func (d *RuleStore) Get(username, chainId string) ([]byte, error) {
	category := d.getCategory(chainId)
	if !isSafeCategory(category) {
		return nil, errors.New("invalid category")
	}
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	paths = append(paths, chainId+constants.RuleChainFileSuffix)
	pathStr := path.Join(paths...)
	return os.ReadFile(pathStr)
}

// GetAsRuleChain Retrieves the rule chain struct
func (d *RuleStore) GetAsRuleChain(username, chainId string) (types.RuleChain, error) {
	var ruleChain types.RuleChain
	data, err := d.Get(username, chainId)
	if err != nil {
		return ruleChain, err
	}
	if err := json.Unmarshal(data, &ruleChain); err != nil {
		return ruleChain, err
	}
	return ruleChain, nil
}

// List lists the rule chain (UI-oriented): keyword search, root/disabled filtering, category filtering, pagination sorting.
// It filters the SystemAgent chain; To start and load, please use AllChains.
func (d *RuleStore) List(username string, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error) {
	var ruleChains []types.RuleChain
	totalCount := 0
	indexList := d.getAllIndex()
	for _, meta := range indexList {
		if meta.SystemAgent {
			continue
		}
		if (root == nil || meta.Root == *root) &&
			(disabled == nil || meta.Disabled == *disabled) &&
			(category == "" || meta.Category == category) {
			if keywords == "" || strings.Contains(meta.Name, keywords) ||
				strings.Contains(meta.ID, keywords) {
				ruleChainData, err := d.GetAsRuleChain(username, meta.ID)
				if err != nil {
					continue
				}
				ruleChains = append(ruleChains, ruleChainData)
				totalCount++
			}
		}
	}

	sort.Slice(ruleChains, func(i, j int) bool {
		var iTime, jTime string
		if v, ok := ruleChains[i].RuleChain.GetAdditionalInfo(constants.KeyUpdateTime); ok {
			iTime = str.ToString(v)
		}
		if v, ok := ruleChains[j].RuleChain.GetAdditionalInfo(constants.KeyUpdateTime); ok {
			jTime = str.ToString(v)
		}
		return iTime > jTime
	})

	if page == 0 {
		return ruleChains, totalCount, nil
	}

	start := (page - 1) * size
	end := start + size
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	return ruleChains[start:end], totalCount, nil
}

// AllChains reads the IDs and DSLs (including SystemAgent) of all rule chains for that user.
// Skipping single file read failures. The order is not defined.
func (d *RuleStore) AllChains(username string) (map[string][]byte, error) {
	d.RLock()
	ids := make([]string, 0, len(d.index.Rules))
	for id := range d.index.Rules {
		ids = append(ids, id)
	}
	d.RUnlock()

	result := make(map[string][]byte, len(ids))
	for _, id := range ids {
		def, err := d.Get(username, id)
		if err != nil || len(def) == 0 {
			continue
		}
		result[id] = def
	}
	return result, nil
}

// Delete deletes the rule chain file and removes it from the index
func (d *RuleStore) Delete(username, chainId string) error {
	category := d.getCategory(chainId)
	if !isSafeCategory(category) {
		return errors.New("invalid category")
	}
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	pathStr := path.Join(paths...)
	file := filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix)
	if err := os.RemoveAll(file); err != nil {
		return err
	}
	return d.deleteIndex(chainId)
}

func (d *RuleStore) getCategory(chainId string) string {
	d.RLock()
	defer d.RUnlock()
	if meta, ok := d.index.Rules[chainId]; ok {
		return meta.Category
	}
	return ""
}

// isSafeCategory checks that the category will not escape from the storage root directory, preventing path traversal.
// category comes from client-controlled additionalInfo and is directly typed into the storage path, so verification is required.
// Multi-level classifications like "a/b" are allowed, but ".." segments and absolute paths are prohibited.
func isSafeCategory(category string) bool {
	if category == "" {
		return true
	}
	cleaned := filepath.Clean(filepath.ToSlash(category))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return false
	}
	return true
}

func (d *RuleStore) saveRuleChain(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	category := ""
	if err := json.Unmarshal(def, &ruleChain); err == nil {
		if cat, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory); ok {
			category = strings.TrimSpace(str.ToString(cat))
		}
	}
	if !isSafeCategory(category) {
		return errors.New("invalid category")
	}
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	pathStr := path.Join(paths...)
	_ = fs.CreateDirs(pathStr)
	var buf bytes.Buffer
	if err := json.Indent(&buf, def, "", "  "); err != nil {
		return err
	}
	return fs.SaveFile(filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix), buf.Bytes())
}

func (d *RuleStore) getIndexPath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsRule, constants.FileNameIndex)
}

func (d *RuleStore) rebuildIndex() error {
	var paths []string
	paths = append(paths, d.config.DataDir, constants.DirWorkflows)
	paths = append(paths, d.username, constants.DirWorkflowsRule)
	basePath := filepath.Join(paths...)
	d.scanDirectory(basePath, "")
	return d.saveIndex(d.getIndexPath())
}

func (d *RuleStore) scanDirectory(dirPath string, folderCategory string) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}
	for _, file := range files {
		if file.IsDir() {
			subCategory := file.Name()
			if folderCategory != "" {
				subCategory = folderCategory + "/" + file.Name()
			}
			d.scanDirectory(filepath.Join(dirPath, file.Name()), subCategory)
			continue
		}
		if filepath.Ext(strings.ToLower(file.Name())) == constants.RuleChainFileSuffix {
			filePath := filepath.Join(dirPath, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			var ruleChain types.RuleChain
			if err := json.Unmarshal(data, &ruleChain); err != nil {
				continue
			}
			if d.isCategoryFolderEnabled() && folderCategory != "" {
				if ruleChain.RuleChain.AdditionalInfo == nil {
					ruleChain.RuleChain.AdditionalInfo = make(map[string]interface{})
				}
				ruleChain.RuleChain.AdditionalInfo[constants.KeyCategory] = folderCategory
			}
			d.createIndex(ruleChain)
		}
	}
}

func (d *RuleStore) loadIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&d.index)
}

func (d *RuleStore) createIndex(ruleChain types.RuleChain) {
	updateTime, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime)
	category, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory)
	var systemAgent bool
	if v, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
		systemAgent, _ = v.(bool)
	}
	chainId := ruleChain.RuleChain.ID
	meta := RuleMeta{
		Name:        ruleChain.RuleChain.Name,
		ID:          chainId,
		Root:        ruleChain.RuleChain.Root,
		Disabled:    ruleChain.RuleChain.Disabled,
		UpdateTime:  str.ToString(updateTime),
		Category:    str.ToString(category),
		SystemAgent: systemAgent,
	}
	d.Lock()
	defer d.Unlock()
	d.index.Rules[chainId] = meta
}

func (d *RuleStore) deleteIndex(chainId string) error {
	d.Lock()
	delete(d.index.Rules, chainId)
	d.Unlock()
	return d.saveIndex(d.getIndexPath())
}

func (d *RuleStore) saveIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(d.index)
}

func (d *RuleStore) getAllIndex() []RuleMeta {
	d.RLock()
	defer d.RUnlock()
	var items []RuleMeta
	for _, v := range d.index.Rules {
		items = append(items, v)
	}
	return items
}

func (d *RuleStore) isCategoryFolderEnabled() bool {
	if d.config.CategoryFolderEnabled == nil {
		return true
	}
	return *d.config.CategoryFolderEnabled
}
