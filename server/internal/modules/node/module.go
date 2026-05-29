// Package node 合并管理动态组件和共享节点池，提供组件安装、卸载和节点池 CRUD 能力。
package node

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/fs"
)

const (
	ModuleName = "node"
	Priority   = 35
)

// Module node 业务模块，负责组件和共享节点池的管理。
type Module struct {
	cfg           *config.Config
	logger        types.Logger
	engineMgr     services.EngineManager
	storeProvider store.StoreProvider
	container     *app.Container
}

// New 创建 node 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	mgr, err := app.GetAs[services.EngineManager](ctx.Container, "module.rule.engine_manager")
	if err != nil {
		return err
	}
	m.engineMgr = mgr

	storeProvider, err := app.GetAs[store.StoreProvider](ctx.Container, "store.provider")
	if err != nil {
		return err
	}
	m.storeProvider = storeProvider
	m.container = ctx.Container

	if err := ctx.Container.Register(services.KeyNodeService, services.NodeService(m)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error {
	var globalData []byte
	if m.cfg.NodePoolFile != "" {
		if buf, err := os.ReadFile(m.cfg.NodePoolFile); err == nil {
			globalData = buf
		} else if !os.IsNotExist(err) {
			m.logger.Errorf("load node pool file error: %s", err)
		}
	}
	for username := range m.cfg.Users {
		m.initUserPool(username, globalData)
	}
	if m.cfg.DefaultUsername != "" {
		if _, ok := m.cfg.Users[m.cfg.DefaultUsername]; !ok {
			m.initUserPool(m.cfg.DefaultUsername, globalData)
		}
	}
	return nil
}

// initUserPool 为用户初始化节点池：先加载全局池，再加载用户私有池
func (m *Module) initUserPool(username string, globalData []byte) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return
	}
	pool, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		return
	}
	if len(globalData) > 0 {
		if _, err := pool.Load(globalData); err != nil {
			m.logger.Errorf("load global node pool for user=%s error: %s", username, err)
		}
	}
	svc, err := m.getUserService(username)
	if err == nil {
		if err := svc.UserNodePoolService.Load(); err != nil {
			m.logger.Errorf("load user node pool for user=%s error: %s", username, err)
		}
		svc.UserComponentService.LoadComponents()
	}
}
func (m *Module) Stop(_ context.Context) error  { return nil }

// getUserService 获取用户级节点服务
func (m *Module) getUserService(username string) (*UserNodeService, error) {
	ue, err := m.engineMgr.GetOrCreate(username)
	if err != nil {
		return nil, err
	}
	pool, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		pool = node_pool.NewNodePool(ue.RuleConfig())
	}
	// MCP service is optional; retrieve from container if available
	var mcpSvc services.McpToolService
	if svc, ok := m.container.Get(constants.SvcMcpService); ok {
		mcpSvc, _ = svc.(services.McpToolService)
	}
	return m.ForUser(username, ue.RuleConfig(), pool, mcpSvc)
}

// NodeService 实现

func (m *Module) ListComponents(username, keywords string, size, page int) ([]types.RuleChain, int, error) {
	svc, err := m.getUserService(username)
	if err != nil {
		return nil, 0, err
	}
	return svc.UserComponentService.List(keywords, size, page)
}

func (m *Module) GetComponent(username, nodeType string) ([]byte, error) {
	svc, err := m.getUserService(username)
	if err != nil {
		return nil, err
	}
	return svc.UserComponentService.Get(nodeType)
}

func (m *Module) InstallComponent(username, id string, dsl []byte) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.Install(id, dsl)
}

func (m *Module) UpgradeComponent(username, id string, dsl []byte) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.Upgrade(id, dsl)
}

func (m *Module) UninstallComponent(username, nodeType string) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.Uninstall(nodeType)
}

func (m *Module) ListNodePool(username string, page, size int, keywords, category string) ([]interface{}, int, error) {
	svc, err := m.getUserService(username)
	if err != nil {
		return nil, 0, err
	}
	return svc.UserNodePoolService.List(page, size, keywords, category)
}

func (m *Module) GetNodePool(username, id, nodeType string) (*types.RuleNode, error) {
	svc, err := m.getUserService(username)
	if err != nil {
		return nil, err
	}
	return svc.UserNodePoolService.Get(id, nodeType)
}

func (m *Module) SaveNodePoolNode(username string, node types.RuleNode) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.SaveNode(node)
}

func (m *Module) SaveNodePoolEndpoint(username string, endpointDef types.EndpointDsl) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.SaveEndpoint(endpointDef)
}

func (m *Module) DeleteNodePool(username, id, nodeType string) error {
	svc, err := m.getUserService(username)
	if err != nil {
		return err
	}
	return svc.Delete(id, nodeType)
}

func (m *Module) GetComponentForms(username string) []types.ComponentForm {
	ue, ok := m.engineMgr.Get(username)
	if !ok || ue == nil {
		return nil
	}
	return ue.RuleConfig().ComponentsRegistry.GetComponentForms().Values()
}

func (m *Module) GetNodePoolDefs(username string) (map[string][]*types.RuleNode, error) {
	svc, err := m.getUserService(username)
	if err != nil {
		return nil, err
	}
	return svc.UserNodePoolService.GetPool().GetAllDef()
}

// ForUser 为指定用户创建组件和节点池服务实例
func (m *Module) ForUser(username string, ruleConfig types.Config, pool *node_pool.NodePool, mcpSvc services.McpToolService) (*UserNodeService, error) {
	componentStore, err := m.storeProvider.GetComponentStore(username)
	if err != nil {
		return nil, err
	}
	nodePoolStore, err := m.storeProvider.GetNodePoolStore(username)
	if err != nil {
		return nil, err
	}
	return &UserNodeService{
		UserComponentService: &UserComponentService{
			username:   username,
			config:     m.cfg,
			ruleConfig: ruleConfig,
			store:      componentStore,
			mcpSvc:     mcpSvc,
		},
		UserNodePoolService: &UserNodePoolService{
			store:    nodePoolStore,
			nodePool: pool,
		},
	}, nil
}

// UserNodeService 用户级节点服务，组合了组件和节点池能力
type UserNodeService struct {
	*UserComponentService
	*UserNodePoolService
}

// UserComponentService 用户级组件服务（从原 component 模块合并）
type UserComponentService struct {
	username   string
	config     *config.Config
	ruleConfig types.Config
	store      store.ComponentStore
	mcpSvc     services.McpToolService
}

func (s *UserComponentService) GetRuleConfig() types.Config {
	return s.ruleConfig
}

func (s *UserComponentService) LoadComponents() {
	folderPath := path.Join(s.config.DataDir, constants.DirWorkflows, s.username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)
	folderPath = folderPath + "/*.json"
	paths, err := fs.GetFilePaths(folderPath)
	if err != nil {
		return
	}
	for _, p := range paths {
		fileName := p[len(folderPath)-len("*.json"):]
		chainId := fileName[:len(fileName)-len(".json")]
		if def, err := s.store.Get(s.username, chainId); err == nil {
			var ruleChain types.RuleChain
			if err = json.Unmarshal(def, &ruleChain); err != nil {
				continue
			}
			if err = s.ComponentsRegistry().Register(engine.NewDynamicNode(ruleChain.RuleChain.ID, string(def))); err != nil {
				if s.ruleConfig.Logger != nil {
					s.ruleConfig.Logger.Errorf("load component id=%s error: %s", ruleChain.RuleChain.ID, err.Error())
				}
				continue
			}
		}
	}
}

func (s *UserComponentService) ComponentsRegistry() types.ComponentRegistry {
	return s.ruleConfig.ComponentsRegistry
}

func (s *UserComponentService) List(keywords string, size, page int) ([]types.RuleChain, int, error) {
	dataList, total, err := s.store.List(s.username, keywords, size, page)
	if err != nil {
		return nil, 0, err
	}
	var ruleChains []types.RuleChain
	for _, data := range dataList {
		var ruleChain types.RuleChain
		if err := json.Unmarshal(data, &ruleChain); err != nil {
			continue
		}
		ruleChains = append(ruleChains, ruleChain)
	}
	return ruleChains, total, nil
}

func (s *UserComponentService) Get(nodeType string) ([]byte, error) {
	return s.store.Get(s.username, nodeType)
}

func (s *UserComponentService) Install(id string, dsl []byte) error {
	dynamicNode := engine.NewDynamicNode(id, string(dsl))
	err := s.ComponentsRegistry().Register(dynamicNode)
	if err != nil {
		return err
	}
	if err = s.store.Save(s.username, dynamicNode.Type(), []byte(dynamicNode.Dsl)); err != nil {
		return err
	}
	if s.mcpSvc != nil {
		s.mcpSvc.AddToolsFromComponent(s.username, dynamicNode.Type(), dynamicNode.Def())
	}
	return nil
}

func (s *UserComponentService) Upgrade(id string, dsl []byte) error {
	_ = s.ComponentsRegistry().Unregister(id)
	return s.Install(id, dsl)
}

func (s *UserComponentService) Uninstall(nodeType string) error {
	if s.mcpSvc != nil {
		s.mcpSvc.DeleteTools(s.username, nodeType)
	}
	_ = s.ComponentsRegistry().Unregister(nodeType)
	return s.store.Delete(s.username, nodeType)
}

// UserNodePoolService 用户级节点池服务（从原 nodepool 模块合并）
type UserNodePoolService struct {
	store    store.NodePoolStore
	nodePool *node_pool.NodePool
}

func (s *UserNodePoolService) Load() error {
	data, err := s.store.Get()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	_, err = s.nodePool.Load(data)
	return err
}

func (s *UserNodePoolService) GetPool() *node_pool.NodePool {
	return s.nodePool
}

func (s *UserNodePoolService) List(page, size int, keywords, category string) ([]interface{}, int, error) {
	list := make([]interface{}, 0)
	all := s.nodePool.GetAll()

	type item struct {
		Ctx        types.SharedNodeCtx
		Def        types.RuleNode
		IsEndpoint bool
	}

	var items []item
	for _, ctx := range all {
		var def types.RuleNode
		if err := json.Unmarshal(ctx.DSL(), &def); err != nil {
			continue
		}
		isEndpoint := false
		if _, ok := ctx.GetNode().(endpointApi.Endpoint); ok {
			isEndpoint = true
		}
		if category == "endpoint" && !isEndpoint {
			continue
		}
		if category == "node" && isEndpoint {
			continue
		}
		if keywords != "" {
			if !strings.Contains(def.Id, keywords) && !strings.Contains(def.Name, keywords) {
				continue
			}
		}
		items = append(items, item{Ctx: ctx, Def: def, IsEndpoint: isEndpoint})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Def.Id < items[j].Def.Id
	})

	total := len(items)
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}

	start := (page - 1) * size
	end := start + size
	if start >= total {
		return []interface{}{}, total, nil
	}
	if end > total {
		end = total
	}

	for _, it := range items[start:end] {
		if it.IsEndpoint {
			var endpointDef types.EndpointDsl
			_ = json.Unmarshal(it.Ctx.DSL(), &endpointDef)
			list = append(list, endpointDef)
		} else {
			list = append(list, it.Def)
		}
	}

	return list, total, nil
}

func (s *UserNodePoolService) SaveNode(node types.RuleNode) error {
	s.nodePool.Del(node.Id)
	if _, err := s.nodePool.NewFromRuleNode(node); err != nil {
		return err
	}
	return s.saveState()
}

func (s *UserNodePoolService) SaveEndpoint(endpoint types.EndpointDsl) error {
	s.nodePool.Del(endpoint.Id)
	if _, err := s.nodePool.NewFromEndpoint(endpoint); err != nil {
		return err
	}
	return s.saveState()
}

func (s *UserNodePoolService) Delete(id, nodeType string) error {
	s.nodePool.Del(id)
	return s.saveState()
}

func (s *UserNodePoolService) Get(id, nodeType string) (*types.RuleNode, error) {
	defs, err := s.nodePool.GetAllDef()
	if err != nil {
		return nil, err
	}
	for _, nodes := range defs {
		for _, node := range nodes {
			if node.Id == id {
				return node, nil
			}
		}
	}
	return nil, nil
}

func (s *UserNodePoolService) saveState() error {
	all := s.nodePool.GetAll()

	dsl := types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{
			ID:   "node_pool",
			Name: "Shared Node Pool",
		},
		Metadata: types.RuleMetadata{
			Nodes:     []*types.RuleNode{},
			Endpoints: []*types.EndpointDsl{},
		},
	}

	for _, ctx := range all {
		raw := ctx.DSL()
		if _, ok := ctx.GetNode().(endpointApi.Endpoint); ok {
			var ep types.EndpointDsl
			if err := json.Unmarshal(raw, &ep); err == nil {
				dsl.Metadata.Endpoints = append(dsl.Metadata.Endpoints, &ep)
			}
		} else {
			var node types.RuleNode
			if err := json.Unmarshal(raw, &node); err == nil {
				dsl.Metadata.Nodes = append(dsl.Metadata.Nodes, &node)
			}
		}
	}

	bytes, err := json.MarshalIndent(dsl, "", "  ")
	if err != nil {
		return err
	}
	return s.store.Save(bytes)
}
