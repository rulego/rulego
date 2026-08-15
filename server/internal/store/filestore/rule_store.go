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
	// SchemaVersion 索引结构版本。新增摘要字段时 +1，旧版本索引触发一次性全量重扫回填
	SchemaVersion int `json:"v,omitempty"`
}

// ruleIndexSchemaVersion 当前索引结构版本（v2 起含 Description/Message/FirstEndpointType/MTime）
const ruleIndexSchemaVersion = 2

// RuleMeta 规则链元数据
type RuleMeta struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Root        bool   `json:"root"`
	Disabled    bool   `json:"disabled"`
	UpdateTime  string `json:"updateTime"`
	Category    string `json:"category"`
	SystemAgent bool   `json:"systemAgent"`
	// Description/Message 列表摘要 additionalInfo 字段（管理页描述、启停信息）
	Description string `json:"description,omitempty"`
	Message     string `json:"message,omitempty"`
	// FirstEndpointType 第一个 endpoint 的组件类型（管理页触发器图标/文案）
	FirstEndpointType string `json:"firstEndpointType,omitempty"`
	// MTime DSL 文件 mtime（UnixNano）。reconcile 据此识别被绕过 API 覆写的文件。
	MTime int64 `json:"mtime,omitempty"`
}

// NewRuleStore 创建规则链文件存储。
// 如果索引文件不存在，会自动扫描规则链文件重建索引；
// 索引结构版本落后时全量重扫回填新字段；其余情况做一次磁盘对账（reconcile），
// 把绕过 API 的变更（手动上传/覆写/删除的 DSL 文件）同步进索引。
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
	if store.index.Rules == nil {
		store.index.Rules = make(map[string]RuleMeta)
	}
	if store.index.SchemaVersion < ruleIndexSchemaVersion {
		if err := store.rebuildIndex(); err != nil {
			return nil, err
		}
	} else if err := store.reconcileIndex(); err != nil {
		return nil, err
	}

	return store, nil
}

// Save 保存规则链定义到文件并更新索引。
// 分类变更会使落盘路径变化，写完新位置后删除旧位置的文件，
// 否则同 id 双份并存、索引裁决不明（旧副本会把列表/对账带回旧分类）。
func (d *RuleStore) Save(username, chainId string, def []byte) error {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	oldCategory := d.getCategory(chainId)
	filePath, err := d.saveRuleChain(username, chainId, def)
	if err != nil {
		return err
	}
	if oldCategory != "" {
		if newCategory, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory); ok {
			if c := strings.TrimSpace(str.ToString(newCategory)); c != oldCategory {
				_ = os.Remove(d.chainFilePath(username, chainId, oldCategory))
			}
		}
	}
	d.createIndex(ruleChain, fileModTime(filePath))
	return d.saveIndex(d.getIndexPath())
}

// Get 获取规则链原始 JSON 数据。
// 文件路径按索引反查的分类目录拼接；读不到时先做一次磁盘对账再重试，
// 覆盖手动上传后未经过任何列表请求就直接按 id 访问的场景。
func (d *RuleStore) Get(username, chainId string) ([]byte, error) {
	data, err := d.readChainFile(username, chainId)
	if err != nil {
		if reconcileErr := d.reconcileIndex(); reconcileErr == nil {
			data, err = d.readChainFile(username, chainId)
		}
	}
	return data, err
}

func (d *RuleStore) readChainFile(username, chainId string) ([]byte, error) {
	category := d.getCategory(chainId)
	if !isSafeCategory(category) {
		return nil, errors.New("invalid category")
	}
	// 索引分类可能与文件实际位置不一致（启用分类前保存的旧文件、手动挪动），
	// 分类路径读不到时回退根目录
	if data, err := os.ReadFile(d.chainFilePath(username, chainId, category)); err == nil {
		return data, nil
	}
	return os.ReadFile(d.chainFilePath(username, chainId, ""))
}

func (d *RuleStore) chainFilePath(username, chainId, category string) string {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	paths = append(paths, chainId+constants.RuleChainFileSuffix)
	return path.Join(paths...)
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

// List 列出规则链（面向 UI）：关键字搜索、root/disabled 过滤、category 过滤、分页排序。
// 过滤 SystemAgent 链；启动加载请用 AllChains。
// 从索引构造摘要项，不读 DSL 文件（完整 DSL 在打开链时按 id 单独拉取）；
// 每次调用先做磁盘对账，同步绕过 API 的文件变更。
func (d *RuleStore) List(username string, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error) {
	_ = d.reconcileIndex()
	var metas []RuleMeta
	for _, meta := range d.getAllIndex() {
		if meta.SystemAgent {
			continue
		}
		if (root == nil || meta.Root == *root) &&
			(disabled == nil || meta.Disabled == *disabled) &&
			categoryMatches(meta.Category, category) {
			if keywords == "" || strings.Contains(meta.Name, keywords) ||
				strings.Contains(meta.ID, keywords) {
				metas = append(metas, meta)
			}
		}
	}

	// 按 updateTime 字符串倒序（最近更新的在前）
	sort.Slice(metas, func(i, j int) bool { return metas[i].UpdateTime > metas[j].UpdateTime })

	totalCount := len(metas)
	if page == 0 {
		size = totalCount
		if size == 0 {
			size = 1
		}
		page = 1
	}
	start := (page - 1) * size
	end := start + size
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	ruleChains := make([]types.RuleChain, 0, end-start)
	for _, meta := range metas[start:end] {
		ruleChains = append(ruleChains, meta.summaryRuleChain())
	}
	return ruleChains, totalCount, nil
}

// summaryRuleChain 由索引元数据构造列表摘要项，不含完整 DSL。
// 字段集对齐消费方：资源树/切换器用 id/name/root/disabled/category/updateTime，
// 管理页用 description/message 和首个 endpoint 类型。
func (m RuleMeta) summaryRuleChain() types.RuleChain {
	addi := make(map[string]interface{}, 4)
	if m.Category != "" {
		addi[constants.KeyCategory] = m.Category
	}
	if m.UpdateTime != "" {
		addi[constants.KeyUpdateTime] = m.UpdateTime
	}
	if m.Description != "" {
		addi[constants.AddiKeyDescription] = m.Description
	}
	if m.Message != "" {
		addi[constants.AddiKeyMessage] = m.Message
	}
	summary := types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{
			ID:       m.ID,
			Name:     m.Name,
			Root:     m.Root,
			Disabled: m.Disabled,
		},
	}
	if len(addi) > 0 {
		summary.RuleChain.AdditionalInfo = addi
	}
	if m.FirstEndpointType != "" {
		summary.Metadata.Endpoints = []*types.EndpointDsl{{RuleNode: types.RuleNode{Type: m.FirstEndpointType}}}
	}
	return summary
}

// AllChains 读取该用户所有规则链的 ID 和 DSL（含 SystemAgent）。
// 单个文件读取失败跳过。顺序未定义。
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

// Delete 删除规则链文件并从索引中移除
func (d *RuleStore) Delete(username, chainId string) error {
	category := d.getCategory(chainId)
	if !isSafeCategory(category) {
		return errors.New("invalid category")
	}
	// 索引分类可能与实际位置不一致（历史遗留），两个候选位置都清理；
	// 只删索引分类路径会漏掉根目录文件，残留文件会被对账复活
	for _, cat := range []string{category, ""} {
		if err := os.RemoveAll(d.chainFilePath(username, chainId, cat)); err != nil {
			return err
		}
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

// isSafeCategory 校验 category 不会逃逸出存储根目录，防止路径穿越。
// category 来自客户端可控的 additionalInfo，会被直接拼入存储路径，必须校验。
// 允许 "a/b" 这类多级分类，但禁止 ".." 段和绝对路径。
// categoryMatches 判断规则链的 category 是否落在查询分类之下。
//
// category 是路径式的（如 "collect/modbus"），存储时按 "/" 分段建目录，
// 所以查询父级要能命中子级：query="collect" 命中 "collect/modbus"。
//
// 必须在 "/" 边界上比，不能用裸 strings.HasPrefix ——
// 否则 query="collect" 会误命中兄弟分类 "collection"。
// query 两端的 "/" 先规整掉，避免 "collect/" 与 "collect" 行为不一致。
func categoryMatches(itemCategory, query string) bool {
	q := strings.Trim(strings.TrimSpace(query), "/")
	if q == "" {
		return true
	}
	c := strings.Trim(itemCategory, "/")
	return c == q || strings.HasPrefix(c, q+"/")
}

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

func (d *RuleStore) saveRuleChain(username, chainId string, def []byte) (string, error) {
	var ruleChain types.RuleChain
	category := ""
	if err := json.Unmarshal(def, &ruleChain); err == nil {
		if cat, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory); ok {
			category = strings.TrimSpace(str.ToString(cat))
		}
	}
	if !isSafeCategory(category) {
		return "", errors.New("invalid category")
	}
	pathStr := d.ruleFilePath(username, chainId, category)
	_ = fs.CreateDirs(pathStr)
	var buf bytes.Buffer
	if err := json.Indent(&buf, def, "", "  "); err != nil {
		return "", err
	}
	filePath := filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix)
	return filePath, fs.SaveFile(filePath, buf.Bytes())
}

// ruleFilePath 规则链 DSL 所在目录（含分类子目录）
func (d *RuleStore) ruleFilePath(username, chainId, category string) string {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	if d.isCategoryFolderEnabled() && category != "" {
		paths = append(paths, category)
	}
	return path.Join(paths...)
}

// fileModTime 取文件 mtime（UnixNano），取不到返回 0
func fileModTime(filePath string) int64 {
	if info, err := os.Stat(filePath); err == nil {
		return info.ModTime().UnixNano()
	}
	return 0
}

func (d *RuleStore) getIndexPath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsRule, constants.FileNameIndex)
}

func (d *RuleStore) rebuildIndex() error {
	// 清空重建：版本升级路径复用此方法，不能残留旧条目
	d.Lock()
	d.index.Rules = make(map[string]RuleMeta)
	d.index.SchemaVersion = ruleIndexSchemaVersion
	d.Unlock()
	basePath := d.ruleBasePath()
	d.scanDirectory(basePath, "")
	return d.saveIndex(d.getIndexPath())
}

// ruleBasePath 当前用户规则链 DSL 根目录
func (d *RuleStore) ruleBasePath() string {
	return filepath.Join(d.config.DataDir, constants.DirWorkflows, d.username, constants.DirWorkflowsRule)
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
			info, err := file.Info()
			if err != nil {
				continue
			}
			d.indexChainFile(filepath.Join(dirPath, file.Name()), folderCategory, info.ModTime().UnixNano())
		}
	}
}

// indexChainFile 读取单个 DSL 文件并写入索引。读取/解析失败静默跳过（与扫描重建同策略）。
func (d *RuleStore) indexChainFile(filePath, folderCategory string, mtime int64) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var ruleChain types.RuleChain
	if err := json.Unmarshal(data, &ruleChain); err != nil {
		return
	}
	if d.isCategoryFolderEnabled() && folderCategory != "" {
		if ruleChain.RuleChain.AdditionalInfo == nil {
			ruleChain.RuleChain.AdditionalInfo = make(map[string]interface{})
		}
		ruleChain.RuleChain.AdditionalInfo[constants.KeyCategory] = folderCategory
	}
	d.createIndex(ruleChain, mtime)
}

// diskChainFile 磁盘上发现的一个 DSL 文件
type diskChainFile struct {
	id             string
	mtime          int64
	path           string
	folderCategory string
}

// collectDiskChains 递归收集当前用户规则链目录下的 DSL 文件（只列目录不读文件内容）。
// 同 id 多副本（改分类保存未清理旧位置的历史遗留）时保留 mtime 新的，
// 旧副本作为 stale 返回，由对账清除。
func (d *RuleStore) collectDiskChains() (map[string]diskChainFile, []diskChainFile) {
	out := make(map[string]diskChainFile)
	var stale []diskChainFile
	var walk func(dirPath, folderCategory string)
	walk = func(dirPath, folderCategory string) {
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				sub := entry.Name()
				if folderCategory != "" {
					sub = folderCategory + "/" + entry.Name()
				}
				walk(filepath.Join(dirPath, entry.Name()), sub)
				continue
			}
			if filepath.Ext(strings.ToLower(entry.Name())) != constants.RuleChainFileSuffix {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			id := strings.TrimSuffix(entry.Name(), constants.RuleChainFileSuffix)
			file := diskChainFile{
				id:             id,
				mtime:          info.ModTime().UnixNano(),
				path:           filepath.Join(dirPath, entry.Name()),
				folderCategory: folderCategory,
			}
			if prev, ok := out[id]; ok {
				if file.mtime >= prev.mtime {
					out[id] = file
					stale = append(stale, prev)
				} else {
					stale = append(stale, file)
				}
			} else {
				out[id] = file
			}
		}
	}
	walk(d.ruleBasePath(), "")
	return out, stale
}

// reconcileIndex 磁盘对账：同步绕过 API 的文件变更——新增（手动上传）、删除、
// 覆写（mtime 变化）、同 id 多副本收敛（以最新为准并清除旧副本）。
// 正常 Save/Delete 已同步索引；无差异时零文件读取。
func (d *RuleStore) reconcileIndex() error {
	disk, stale := d.collectDiskChains()
	d.RLock()
	var toRead []diskChainFile
	var removed []string
	for id, f := range disk {
		cur, ok := d.index.Rules[id]
		if !ok || cur.MTime != f.mtime {
			toRead = append(toRead, f)
		}
	}
	for id := range d.index.Rules {
		if _, ok := disk[id]; !ok {
			removed = append(removed, id)
		}
	}
	d.RUnlock()
	if len(toRead) == 0 && len(removed) == 0 && len(stale) == 0 {
		return nil
	}
	for _, f := range toRead {
		d.indexChainFile(f.path, f.folderCategory, f.mtime)
	}
	if len(removed) > 0 {
		d.Lock()
		for _, id := range removed {
			delete(d.index.Rules, id)
		}
		d.Unlock()
	}
	for _, f := range stale {
		_ = os.Remove(f.path)
	}
	if len(toRead) == 0 && len(removed) == 0 {
		return nil
	}
	return d.saveIndex(d.getIndexPath())
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

func (d *RuleStore) createIndex(ruleChain types.RuleChain, mtime int64) {
	updateTime, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyUpdateTime)
	category, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.KeyCategory)
	description, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.AddiKeyDescription)
	message, _ := ruleChain.RuleChain.GetAdditionalInfo(constants.AddiKeyMessage)
	var systemAgent bool
	if v, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
		systemAgent, _ = v.(bool)
	}
	firstEndpointType := ""
	if len(ruleChain.Metadata.Endpoints) > 0 {
		firstEndpointType = ruleChain.Metadata.Endpoints[0].Type
	}
	chainId := ruleChain.RuleChain.ID
	meta := RuleMeta{
		Name:              ruleChain.RuleChain.Name,
		ID:                chainId,
		Root:              ruleChain.RuleChain.Root,
		Disabled:          ruleChain.RuleChain.Disabled,
		UpdateTime:        str.ToString(updateTime),
		Category:          str.ToString(category),
		Description:       str.ToString(description),
		Message:           str.ToString(message),
		FirstEndpointType: firstEndpointType,
		MTime:             mtime,
		SystemAgent:       systemAgent,
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
	// 父目录可能尚未创建（新建用户首次写入索引时），os.Create 不会自动建目录。
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
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
