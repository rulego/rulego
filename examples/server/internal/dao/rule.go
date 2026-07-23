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

// IndexKeySpe key linker
var IndexKeySpe = ":"

type RuleDao struct {
	config   config.Config
	username string
	index    Index
	sync.RWMutex
}

// Index defines the index structure, containing only the necessary metadata
type Index struct {
	// key=chainId
	Rules map[string]RuleMeta `json:"rules"`
}

type RuleMeta struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Root       bool   `json:"root"`
	Disabled   bool   `json:"disabled"`
	UpdateTime string `json:"updateTime"`
}

func NewRuleDao(config config.Config, username string) (*RuleDao, error) {
	dao := &RuleDao{
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
func (d *RuleDao) List(username string, keywords string, root *bool, disabled *bool, size, page int) ([]types.RuleChain, int, error) {
	var ruleChains []types.RuleChain
	totalCount := 0
	indexList := d.getAllIndex()
	// Traverse metadata in the index
	for _, meta := range indexList {
		if (root == nil || meta.Root == *root) &&
			(disabled == nil || meta.Disabled == *disabled) {
			if keywords == "" || strings.Contains(meta.Name, keywords) ||
				strings.Contains(meta.ID, keywords) {
				// Load complete rule chain data based on metadata
				ruleChainData, err := d.GetAsRuleChain(username, meta.ID)
				if err != nil {
					continue
				}
				ruleChains = append(ruleChains, ruleChainData)
				totalCount++
			}
		}
	}

	// Sorting logic
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

func (d *RuleDao) Get(username, chainId string) ([]byte, error) {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule, chainId+constants.RuleChainFileSuffix)
	pathStr := path.Join(paths...)
	return os.ReadFile(pathStr)
}

func (d *RuleDao) GetAsRuleChain(username, chainId string) (types.RuleChain, error) {
	// Load the rule chain DSL data according to the ID
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

func (d *RuleDao) Save(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	if err := d.saveRuleChain(username, chainId, def); err != nil {
		return err
	}
	//Create indexes
	d.createIndex(ruleChain)
	// Save the index to the file
	return d.saveIndex(d.getIndexPath())
}

func (d *RuleDao) saveRuleChain(username, chainId string, def []byte) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	pathStr := path.Join(paths...)
	//Create a folder
	_ = fs.CreateDirs(pathStr)
	//Save to file
	var buf bytes.Buffer
	err := json.Indent(&buf, def, "", "  ")
	if err != nil {
		return err
	}

	//Save the rule chain to the file
	return fs.SaveFile(filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix), buf.Bytes())
}
func (d *RuleDao) Delete(username, chainId string) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	pathStr := path.Join(paths...)
	file := filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix)
	if err := os.RemoveAll(file); err != nil {
		return err
	}
	return d.deleteIndex(chainId)
}

func (d *RuleDao) getIndexPath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsRule, constants.FileNameIndex)
}
func (d *RuleDao) rebuildIndex() error {
	var paths []string
	paths = append(paths, d.config.DataDir, constants.DirWorkflows)
	paths = append(paths, d.username, constants.DirWorkflowsRule)

	// Build a complete path
	basePath := filepath.Join(paths...)

	// Read all files in the directory
	files, err := os.ReadDir(basePath)
	if err != nil {
		return err
	}

	// Traverse the file
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(strings.ToLower(file.Name())) == constants.RuleChainFileSuffix {
			// Complete path to the build file
			filePath := filepath.Join(basePath, file.Name())

			// Read the file content
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			// Parse JSON data to the RuleChain structure
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
func (d *RuleDao) loadIndex(indexPath string) error {
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

func (d *RuleDao) createIndex(ruleChain types.RuleChain) {
	updateTime, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime)
	chainId := ruleChain.RuleChain.ID
	// Update the index
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

func (d *RuleDao) deleteIndex(chainId string) error {
	d.Lock()
	delete(d.index.Rules, chainId)
	d.Unlock()
	return d.saveIndex(d.getIndexPath())
}
func (d *RuleDao) saveIndex(indexPath string) error {
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
func (d *RuleDao) getAllIndex() []RuleMeta {
	d.RLock()
	defer d.RUnlock()
	var items []RuleMeta
	for _, v := range d.index.Rules {
		items = append(items, v)
	}
	return items
}
