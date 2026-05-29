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

// RuleStore 基于文件系统的规则链存储实现。
// 使用 JSON 文件存储规则链定义，并维护索引文件加速列表查询。
type RuleStore struct {
	config   config.Config
	username string
	index    RuleIndex
	sync.RWMutex
}

// RuleIndex 规则链索引，仅包含必要元数据用于快速列表查询
type RuleIndex struct {
	Rules map[string]RuleMeta `json:"rules"`
}

// RuleMeta 规则链元数据
type RuleMeta struct {
	Name         string `json:"name"`
	ID           string `json:"id"`
	Root         bool   `json:"root"`
	Disabled     bool   `json:"disabled"`
	UpdateTime   string `json:"updateTime"`
	Category     string `json:"category"`
	SystemAgent  bool   `json:"systemAgent"`
}

// NewRuleStore 创建规则链文件存储。
// 如果索引文件不存在，会自动扫描规则链文件重建索引。
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

// Save 保存规则链定义到文件并更新索引
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

// Get 获取规则链原始 JSON 数据
func (d *RuleStore) Get(username, chainId string) ([]byte, error) {
	category := d.getCategory(chainId)
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	paths = append(paths, chainId+constants.RuleChainFileSuffix)
	pathStr := path.Join(paths...)
	return os.ReadFile(pathStr)
}

// GetAsRuleChain 获取规则链结构体
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

// List 列出规则链，支持关键字搜索、root/disabled 过滤、category 过滤、分页排序
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

// Delete 删除规则链文件并从索引中移除
func (d *RuleStore) Delete(username, chainId string) error {
	category := d.getCategory(chainId)
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

func (d *RuleStore) saveRuleChain(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	category := ""
	if err := json.Unmarshal(def, &ruleChain); err == nil {
		if cat, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory); ok {
			category = strings.TrimSpace(str.ToString(cat))
		}
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
