package dao

import (
	"bytes"
	"encoding/json"
	"errors"
	"examples/server/config"
	"examples/server/internal/constants"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/str"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type ComponentDao struct {
	config   config.Config
	username string
	index    Index
	sync.RWMutex
}

func NewComponentDao(config config.Config, username string) (*ComponentDao, error) {
	dao := &ComponentDao{
		config:   config,
		username: username,
		index:    Index{Rules: make(map[string]RuleMeta)},
	}

	// Load or initialize the index
	indexPath := dao.getIndexPath()
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return dao, dao.rebuildIndex()
	} else if err != nil {
		return nil, err
	} else {
		if err := dao.loadIndex(indexPath); err != nil {
			return nil, err
		}
	}

	return dao, nil
}
func (d *ComponentDao) List(username string, keywords string, root *bool, disabled *bool, size, page int) ([]types.RuleChain, int, error) {
	var ruleChains []types.RuleChain
	totalCount := 0
	indexList := d.getAllIndex()
	// 遍历索引中的元数据
	for _, meta := range indexList {
		if (root == nil || meta.Root == *root) &&
			(disabled == nil || meta.Disabled == *disabled) {
			if keywords == "" || strings.Contains(meta.Name, keywords) ||
				strings.Contains(meta.ID, keywords) {
				// 根据元数据加载完整的规则链数据
				ruleChainData, err := d.GetAsRuleChain(username, meta.ID)
				if err != nil {
					continue
				}
				ruleChains = append(ruleChains, ruleChainData)
				totalCount++
			}
		}
	}

	// 排序逻辑
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

func (d *ComponentDao) Get(username, chainId string) ([]byte, error) {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent, chainId+constants.RuleChainFileSuffix)
	pathStr := path.Join(paths...)
	return os.ReadFile(pathStr)
}

func (d *ComponentDao) GetAsRuleChain(username, chainId string) (types.RuleChain, error) {
	// 根据ID加载规则链DSL数据
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

func (d *ComponentDao) Save(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	if err := d.saveRuleChain(username, chainId, def); err != nil {
		return err
	}
	//创建索引
	d.createIndex(ruleChain)
	// 保存索引到文件
	return d.saveIndex(d.getIndexPath())
}

func (d *ComponentDao) saveRuleChain(username, chainId string, def []byte) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent)
	pathStr := path.Join(paths...)
	//创建文件夹
	_ = fs.CreateDirs(pathStr)
	//保存到文件
	var buf bytes.Buffer
	err := json.Indent(&buf, def, "", "  ")
	if err != nil {
		return err
	}

	//保存规则链到文件
	return fs.SaveFile(filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix), buf.Bytes())
}
func (d *ComponentDao) Delete(username, chainId string) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent)
	pathStr := path.Join(paths...)
	file := filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix)
	if err := os.RemoveAll(file); err != nil {
		return err
	}
	return d.deleteIndex(chainId)
}

func (d *ComponentDao) getIndexPath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsComponent, constants.FileNameIndex)
}
func (d *ComponentDao) rebuildIndex() error {
	var paths []string
	paths = append(paths, d.config.DataDir, constants.DirWorkflows)
	paths = append(paths, d.username, constants.DirWorkflowsComponent)

	// 构建完整的路径
	basePath := filepath.Join(paths...)

	// 读取目录下的所有文件
	files, err := os.ReadDir(basePath)
	if err != nil {
		return err
	}

	// 遍历文件
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(strings.ToLower(file.Name())) == constants.RuleChainFileSuffix {
			// 构建文件的完整路径
			filePath := filepath.Join(basePath, file.Name())

			// 读取文件内容
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			// 解析 JSON 数据到 RuleChain 结构体
			var ruleChain types.RuleChain
			err = json.Unmarshal(data, &ruleChain)
			if err != nil {
				continue
			}
			d.createIndex(ruleChain)
		}
	}
	return d.saveIndex(d.getIndexPath())
}
func (d *ComponentDao) loadIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&d.index); err != nil {
		return err
	}

	return nil
}

func (d *ComponentDao) createIndex(ruleChain types.RuleChain) {
	updateTime, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime)
	chainId := ruleChain.RuleChain.ID
	// 更新索引
	meta := RuleMeta{
		Name:       ruleChain.RuleChain.Name,
		ID:         chainId,
		Root:       ruleChain.RuleChain.Root,
		Disabled:   ruleChain.RuleChain.Disabled,
		UpdateTime: str.ToString(updateTime),
	}
	d.Lock()
	defer d.Unlock()
	d.index.Rules[chainId] = meta
}

func (d *ComponentDao) deleteIndex(chainId string) error {
	d.Lock()
	delete(d.index.Rules, chainId)
	d.Unlock()
	return d.saveIndex(d.getIndexPath())
}
func (d *ComponentDao) saveIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(d.index); err != nil {
		return err
	}
	return nil
}
func (d *ComponentDao) getAllIndex() []RuleMeta {
	d.RLock()
	defer d.RUnlock()
	var items []RuleMeta
	for _, v := range d.index.Rules {
		items = append(items, v)
	}
	return items
}
