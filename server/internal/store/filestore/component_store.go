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

// ComponentStore is implemented based on the component storage of the file system.
// Component definitions are stored as JSON files, using index files to accelerate list queries.
type ComponentStore struct {
	config   config.Config
	username string
	index    ComponentIndex
	sync.RWMutex
}

// ComponentIndex component index
type ComponentIndex struct {
	Components map[string]ComponentMeta `json:"rules"`
}

// ComponentMeta component metadata
type ComponentMeta struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	UpdateTime string `json:"updateTime"`
}

// NewComponentStore creates component file storage
func NewComponentStore(cfg config.Config, username string) (*ComponentStore, error) {
	store := &ComponentStore{
		config:   cfg,
		username: username,
		index:    ComponentIndex{Components: make(map[string]ComponentMeta)},
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

// Save the component defined in the file and update the index
func (d *ComponentStore) Save(username, componentId string, def []byte) error {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	if err := d.saveFile(username, componentId, def); err != nil {
		return err
	}
	d.createIndex(ruleChain)
	return d.saveIndex(d.getIndexPath())
}

// Get the original JSON data defined by the component
func (d *ComponentStore) Get(username, componentId string) ([]byte, error) {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent, componentId+constants.RuleChainFileSuffix)
	return os.ReadFile(path.Join(paths...))
}

// List lists, component listing, supports keyword search and pagination
func (d *ComponentStore) List(username, keywords string, size, page int) ([][]byte, int, error) {
	var results [][]byte
	totalCount := 0
	indexList := d.getAllIndex()
	for _, meta := range indexList {
		if keywords == "" || strings.Contains(meta.Name, keywords) || strings.Contains(meta.ID, keywords) {
			data, err := d.Get(username, meta.ID)
			if err != nil {
				continue
			}
			results = append(results, data)
			totalCount++
		}
	}
	sort.Slice(results, func(i, j int) bool {
		var iChain, jChain types.RuleChain
		_ = json.Unmarshal(results[i], &iChain)
		_ = json.Unmarshal(results[j], &jChain)
		var iTime, jTime string
		if v, ok := iChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime); ok {
			iTime = str.ToString(v)
		}
		if v, ok := jChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime); ok {
			jTime = str.ToString(v)
		}
		return iTime > jTime
	})
	if page == 0 {
		return results, totalCount, nil
	}
	start := (page - 1) * size
	end := start + size
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	return results[start:end], totalCount, nil
}

// Delete deletes component files and removes them from the index
func (d *ComponentStore) Delete(username, componentId string) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent)
	filePath := filepath.Join(path.Join(paths...), componentId+constants.RuleChainFileSuffix)
	if err := os.RemoveAll(filePath); err != nil {
		return err
	}
	return d.deleteIndex(componentId)
}

func (d *ComponentStore) saveFile(username, componentId string, def []byte) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsComponent)
	pathStr := path.Join(paths...)
	_ = fs.CreateDirs(pathStr)
	var buf bytes.Buffer
	if err := json.Indent(&buf, def, "", "  "); err != nil {
		return err
	}
	return fs.SaveFile(filepath.Join(pathStr, componentId+constants.RuleChainFileSuffix), buf.Bytes())
}

func (d *ComponentStore) getIndexPath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsComponent, constants.FileNameIndex)
}

func (d *ComponentStore) rebuildIndex() error {
	var paths []string
	paths = append(paths, d.config.DataDir, constants.DirWorkflows)
	paths = append(paths, d.username, constants.DirWorkflowsComponent)
	basePath := filepath.Join(paths...)
	files, err := os.ReadDir(basePath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if filepath.Ext(strings.ToLower(f.Name())) == constants.RuleChainFileSuffix {
			filePath := filepath.Join(basePath, f.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			var ruleChain types.RuleChain
			if err := json.Unmarshal(data, &ruleChain); err != nil {
				continue
			}
			d.createIndex(ruleChain)
		}
	}
	return d.saveIndex(d.getIndexPath())
}

func (d *ComponentStore) loadIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&d.index)
}

func (d *ComponentStore) createIndex(ruleChain types.RuleChain) {
	updateTime, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime)
	chainId := ruleChain.RuleChain.ID
	meta := ComponentMeta{
		Name:       ruleChain.RuleChain.Name,
		ID:         chainId,
		UpdateTime: str.ToString(updateTime),
	}
	d.Lock()
	defer d.Unlock()
	d.index.Components[chainId] = meta
}

func (d *ComponentStore) deleteIndex(chainId string) error {
	d.Lock()
	delete(d.index.Components, chainId)
	d.Unlock()
	return d.saveIndex(d.getIndexPath())
}

func (d *ComponentStore) saveIndex(indexPath string) error {
	d.Lock()
	defer d.Unlock()
	file, err := os.Create(indexPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(d.index)
}

func (d *ComponentStore) getAllIndex() []ComponentMeta {
	d.RLock()
	defer d.RUnlock()
	var items []ComponentMeta
	for _, v := range d.index.Components {
		items = append(items, v)
	}
	return items
}
