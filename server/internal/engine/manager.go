// Package engine 提供多租户规则引擎池管理和用户级引擎实例。
package engine

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	rulegoEngine "github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/node_pool"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/internal/runlogutil"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/fs"

	"github.com/rulego/rulego/components/action"
)

// UserEngine 用户级规则引擎，管理引擎池和配置
type UserEngine struct {
	pool       *rulego.RuleGo
	username   string
	config     config.Config
	ruleConfig types.Config
	logger     types.Logger
	ruleStore  store.RuleStore
	setStore   store.SettingStore
	container  *app.Container // 服务容器，供全局回调按需懒取 RunLogService 等服务；nil 表示不可用
	locker     sync.RWMutex
	mainEngine types.RuleEngine
}

// Manager 管理多租户用户引擎池
type Manager struct {
	pool          map[string]*UserEngine
	locker        sync.RWMutex
	cfg           *config.Config
	logger        types.Logger
	storeProvider store.StoreProvider
	systemEp      types.Node     // 共享给用户池的系统端点（如主 HTTP server），由 SetSystemEndpoint 注入；nil 表示不注入
	container     *app.Container // 服务容器，由 SetContainer 注入，供全局回调懒取 RunLogService
}

// NewManager 创建引擎管理器
func NewManager(cfg *config.Config, logger types.Logger, storeProvider store.StoreProvider) *Manager {
	return &Manager{
		pool:          make(map[string]*UserEngine),
		cfg:           cfg,
		logger:        logger,
		storeProvider: storeProvider,
	}
}

// SetSystemEndpoint 设置要注入到每个用户池的系统端点（如开启 share_http_server 时的主 HTTP server）。
// 设置后新建用户引擎时会把该端点加入用户节点池，供用户规则链通过 ref:// 引用。
func (m *Manager) SetSystemEndpoint(ep types.Node) {
	m.systemEp = ep
}

// SetContainer 注入服务容器，供全局 OnRuleChainCompleted 回调懒取 RunLogService。
// 应在 InitUserEngines 之前调用（由 rule 模块在 Init 阶段注入）。
func (m *Manager) SetContainer(c *app.Container) {
	m.container = c
}

// userExists 判断用户是否仍有效，用于在 InitUserEngines 时识别已删用户的残留目录。
// 判定优先级：default_username 始终有效（开箱即用账号可能尚未落 store）；
// 其次 config 内置账号；最后查 UserStore。storeErr/userStore==nil 时对非内置账号
// 保守放行，避免在 store 不可用时误伤正常用户。
func (m *Manager) userExists(username string, userStore store.UserStore, storeErr error) bool {
	if username == m.cfg.DefaultUsername {
		return true
	}
	if m.cfg.CheckUserExists(username) {
		return true
	}
	if storeErr != nil || userStore == nil {
		return true
	}
	_, ok := userStore.GetUser(username)
	return ok
}

// GetOrCreate 获取或创建用户引擎，使用 double-check locking 防止竞态
func (m *Manager) GetOrCreate(username string) (services.UserEngine, error) {
	if ue, ok := m.get(username); ok {
		return ue, nil
	}
	m.locker.Lock()
	defer m.locker.Unlock()
	// 拿到写锁后再次检查，防止并发创建
	if ue, ok := m.pool[username]; ok {
		return ue, nil
	}
	ue, err := m.newUserEngine(username)
	if err != nil {
		return nil, err
	}
	m.pool[username] = ue
	return ue, nil
}

// Get 获取已有用户引擎
func (m *Manager) Get(username string) (services.UserEngine, bool) {
	return m.get(username)
}

// InitUserEngines 初始化已有用户目录的引擎，分两阶段：
//  1. 创建所有用户引擎但不加载规则链，让 MCP 等模块在 Start 阶段先注册 UDF；
//  2. 再统一加载规则链——此时 mcp_tool_provider 等 UDF 已就绪，含 AI/agent 节点的链才能正确解析。
func (m *Manager) InitUserEngines() error {
	userPath := path.Join(m.cfg.DataDir, constants.DirWorkflows)
	_ = fs.CreateDirs(userPath)

	// Phase 1: 创建所有用户引擎（不加载规则链）
	entries, err := os.ReadDir(userPath)
	if err != nil {
		return err
	}
	userStore, storeErr := m.storeProvider.GetUserStore()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !m.userExists(name, userStore, storeErr) {
			m.logger.Infof("skip orphan data dir for removed user: %s", name)
			continue
		}
		if _, err := m.GetOrCreate(name); err != nil {
			m.logger.Errorf("Init %s error: %s", name, err.Error())
		}
	}
	for user := range m.cfg.Users {
		if _, ok := m.get(user); !ok {
			if _, err := m.GetOrCreate(user); err != nil {
				m.logger.Errorf("Init %s error: %s", user, err.Error())
			}
		}
	}
	if _, ok := m.get(m.cfg.DefaultUsername); !ok {
		if _, err := m.GetOrCreate(m.cfg.DefaultUsername); err != nil {
			m.logger.Errorf("Init %s error: %s", m.cfg.DefaultUsername, err.Error())
		}
	}

	// Phase 2: 统一加载规则链
	// 此时 MCP 等模块已通过 GetOrCreate 注册了 UDF（如 mcp_tool_provider），可以安全加载含 AI/agent 节点的规则链。
	m.locker.RLock()
	userEngines := make([]*UserEngine, 0, len(m.pool))
	for _, ue := range m.pool {
		userEngines = append(userEngines, ue)
	}
	m.locker.RUnlock()

	for _, ue := range userEngines {
		ue.loadRules()
	}
	return nil
}

// Remove 移除并停止指定用户的引擎，用户不存在时返回 nil（幂等）。
// 先加锁摘出实例并从 pool 删除，解锁后再 Stop，避免持锁做慢操作。
func (m *Manager) Remove(username string) error {
	m.locker.Lock()
	ue, ok := m.pool[username]
	if ok {
		delete(m.pool, username)
	}
	m.locker.Unlock()
	if !ok {
		return nil
	}
	ue.Stop()
	return nil
}

// Stop 停止所有用户引擎
func (m *Manager) Stop() {
	m.locker.Lock()
	defer m.locker.Unlock()
	for _, ue := range m.pool {
		ue.Stop()
	}
}

func (m *Manager) get(username string) (*UserEngine, bool) {
	m.locker.RLock()
	defer m.locker.RUnlock()
	ue, ok := m.pool[username]
	return ue, ok
}

// newUserEngine 创建用户级引擎实例
func (m *Manager) newUserEngine(username string) (*UserEngine, error) {
	cfg := m.cfg
	logger := m.logger
	componentRegistry := rulegoEngine.NewCustomComponentRegistry(rulegoEngine.Registry, new(rulegoEngine.RuleComponentRegistry))
	poolConfig := rulego.NewConfig(types.WithComponentsRegistry(componentRegistry), types.WithLogger(logger))
	pool := node_pool.NewNodePool(poolConfig)
	// 将系统端点（如主 HTTP server）注入用户池，供用户规则链通过 ref:// 引用。
	if m.systemEp != nil {
		if _, err := pool.AddNode(m.systemEp); err != nil {
			m.logger.Errorf("inject system endpoint into user=%s pool error: %s", username, err)
		}
	}

	ruleConfig := rulego.NewConfig(types.WithDefaultPool(),
		types.WithLogger(logger),
		types.WithComponentsRegistry(componentRegistry),
		types.WithNodePool(pool))

	ruleStore, err := m.storeProvider.GetRuleStore(username)
	if err != nil {
		return nil, err
	}
	setStore, err := m.storeProvider.GetSettingStore(username)
	if err != nil {
		return nil, err
	}

	ue := &UserEngine{
		pool:       rulego.NewRuleGo(),
		username:   username,
		config:     *cfg,
		ruleConfig: ruleConfig,
		logger:     logger,
		ruleStore:  ruleStore,
		setStore:   setStore,
		container:  m.container,
	}

	ue.initRuleConfig()
	// 确保 Udf map 已初始化
	if ue.ruleConfig.Udf == nil {
		ue.ruleConfig.Udf = make(map[string]interface{})
	}
	ue.loadJs()
	ue.loadPlugins()
	// 注意：不在创建阶段加载规则链。
	// 规则链由 InitUserEngines() 统一加载，确保 MCP 等模块的 UDF（如 mcp_tool_provider）
	// 在规则链初始化之前完成注册。

	return ue, nil
}

// Stop 停止引擎
func (ue *UserEngine) Stop() {
	if ue.pool != nil {
		ue.pool.Stop()
	}
}

// Pool 返回底层 RuleGo 池
func (ue *UserEngine) Pool() *rulego.RuleGo {
	return ue.pool
}

// RuleConfig 返回规则引擎配置
func (ue *UserEngine) RuleConfig() types.Config {
	return ue.ruleConfig
}

// RuleStore 返回规则链存储
func (ue *UserEngine) RuleStore() store.RuleStore {
	return ue.ruleStore
}

// SettingStore 返回设置存储
func (ue *UserEngine) SettingStore() store.SettingStore {
	return ue.setStore
}

// Username 返回用户名
func (ue *UserEngine) Username() string {
	return ue.username
}

// GetEngine 获取指定规则链引擎
func (ue *UserEngine) GetEngine(chainId string) (types.RuleEngine, bool) {
	return ue.pool.Get(chainId)
}

// LoadRule 从存储加载规则链到引擎池
func (ue *UserEngine) LoadRule(chainId string) error {
	def, err := ue.ruleStore.Get(ue.username, chainId)
	if err != nil {
		return err
	}
	return ue.loadDef(chainId, def)
}

// loadDef 把 DSL 编译进引擎池（已存在则 reload）。
func (ue *UserEngine) loadDef(chainId string, def []byte) error {
	if ruleEngine, ok := ue.pool.Get(chainId); ok {
		return ruleEngine.ReloadSelf(def)
	}
	_, err := ue.pool.New(chainId, def, rulego.WithConfig(ue.ruleConfig))
	return err
}

// SetMainChainId 设置主规则链
func (ue *UserEngine) SetMainChainId(chainId string) error {
	if chainId == "" {
		return fmt.Errorf("chainId is empty")
	}
	if err := ue.setStore.Save(constants.SettingKeyMainChainId, chainId); err != nil {
		return err
	}
	if e, ok := ue.pool.Get(chainId); !ok {
		return fmt.Errorf("please deploy rule chain first")
	} else {
		ue.mainEngine = e
		return nil
	}
}

// SaveSetting 保存用户设置
func (ue *UserEngine) SaveSetting(key, value string) error {
	return ue.setStore.Save(key, value)
}

// GetSetting 获取用户设置
func (ue *UserEngine) GetSetting(key string) string {
	return ue.setStore.Get(key)
}

func (ue *UserEngine) initRuleConfig() {
	for k, v := range ue.config.Global {
		ue.ruleConfig.Properties.PutValue(k, fmt.Sprintf("%v", v))
	}
	ue.ruleConfig.Properties.PutValue(constants.LoadLuaLibs, ue.config.LoadLuaLibs)
	ue.ruleConfig.Properties.PutValue(action.KeyExecNodeWhitelist, ue.config.CmdWhiteList)
	ue.ruleConfig.Properties.PutValue(action.KeyExecNodeMode, ue.config.CmdMode)
	ue.ruleConfig.Properties.PutValue(action.KeyExecNodeDeny, ue.config.CmdDenyList)
	ue.ruleConfig.Properties.PutValue(action.KeyExecNodeDenyArgs, ue.config.CmdDenyArgs)
	ue.ruleConfig.Properties.PutValue(action.KeyWorkDir, ue.config.DataDir)
	if ue.config.FilePathWhiteList != "" {
		ue.ruleConfig.Properties.PutValue(constants.KeyFilePathWhitelist, ue.config.FilePathWhiteList)
	}
	if ue.config.ScriptMaxExecutionTime > 0 {
		ue.ruleConfig.ScriptMaxExecutionTime = time.Millisecond * time.Duration(ue.config.ScriptMaxExecutionTime)
	}
	if ue.config.EndpointEnabled != nil {
		ue.ruleConfig.EndpointEnabled = *ue.config.EndpointEnabled
	}
	if ue.config.SecretKey != nil && *ue.config.SecretKey != "" {
		ue.ruleConfig.SecretKey = *ue.config.SecretKey
	}

	// OnDebug 回调：存到内存 + 推送到 WebSocket 客户端
	ue.ruleConfig.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		logData := map[string]interface{}{
			"chainId":      chainId,
			"flowType":     flowType,
			"nodeId":       nodeId,
			"relationType": relationType,
			"err":          errStr,
			"msg":          msg,
			"msgId":        msg.Id,
			"ts":           time.Now().UnixMilli(),
		}
		// 存到内存，供 REST API 查询（双击节点时使用）
		runlog.DefaultDebugDataStore.Add(chainId, nodeId, logData)
		// 推送到 WebSocket 客户端
		runlog.SendDebugDataToClients(chainId, logData)
		// 子规则链调试日志同步推送到调试发起的根链路，使主链路控制台可见
		if root := msg.Metadata.GetValue(constants.ParamRootChainId); root != "" && root != chainId {
			runlog.SendDebugDataToClients(root, logData)
		}
	}

	// 注册全局完成回调以落运行记录。仅当全局级别非 Off 时才启用——
	// RunLogMode 必须设进 ruleConfig，引擎据此决定是否收集逐节点日志。
	if globalLevel := runlogutil.ParseLevel(ue.config.RunLogMode); globalLevel != runlogutil.LevelOff {
		ue.ruleConfig.RunLogMode = types.RunLogMode(ue.config.RunLogMode)
		ue.ruleConfig.OnRuleChainCompleted = func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			// 单链可能在自己的 additionalInfo 里覆盖级别，需按链重新解析
			level := runlogutil.ResolveLevel(ue.config.RunLogMode, ctx)
			if level == runlogutil.LevelOff {
				return
			}
			username := runlogutil.UsernameFromCtx(ctx)
			source := ""
			if out := ctx.GetOut(); out.Metadata != nil {
				source = out.Metadata.GetValue(constants.ParamTriggerSource)
			}
			// RunLogService 此时未必已注册，按需从容器懒取以避开模块 init 时序
			if ue.container != nil {
				if runLogSvc, err := app.GetAs[services.RunLogService](ue.container, services.KeyRunLogService); err == nil {
					_ = runLogSvc.SaveRunLog(username, ctx, snapshot, level, source)
				}
			}
		}
	}
}

func (ue *UserEngine) loadJs() {
	jsPath := path.Join(ue.config.DataDir, "js")
	_ = fs.CreateDirs(jsPath)
	paths, err := fs.GetFilePaths(jsPath + "/*.js")
	if err != nil {
		return
	}
	for _, file := range paths {
		if b := fs.LoadFile(file); b != nil {
			if p, err := goja.Compile(file, string(b), true); err != nil {
				ue.logger.Errorf("Compile js file=%s err=%s", file, err.Error())
			} else {
				ue.ruleConfig.RegisterUdf(path.Base(file), types.Script{
					Type:    types.Js,
					Content: p,
				})
			}
		}
	}
}

func (ue *UserEngine) loadPlugins() {
	pluginsPath := path.Join(ue.config.DataDir, "plugins")
	_ = fs.CreateDirs(pluginsPath)
	paths, err := fs.GetFilePaths(pluginsPath + "/*.so")
	if err != nil {
		return
	}
	for _, file := range paths {
		if err := rulego.Registry.RegisterPlugin(path.Base(file), file); err != nil {
			ue.logger.Errorf("load plugin=%s error=%s", file, err.Error())
		}
	}
}

// loadRules 通过 RuleStore.AllChains 一次取回该用户所有规则链并加载到引擎池。
func (ue *UserEngine) loadRules() {
	chains, err := ue.ruleStore.AllChains(ue.username)
	if err != nil {
		ue.logger.Errorf("loadRules(%s): load chains failed: %s",
			ue.username, err.Error())
		return
	}

	var count int
	for chainId, def := range chains {
		if err := ue.loadDef(chainId, def); err != nil {
			ue.logger.Errorf("load rule chain id:%s error: %s",
				chainId, err.Error())
		} else {
			count++
		}
	}
	ue.logger.Infof("%s number of rule chains loaded: %d", ue.username, count)

	if mainChainId := ue.setStore.Get(constants.SettingKeyMainChainId); mainChainId != "" {
		if err := ue.SetMainChainId(mainChainId); err != nil {
			ue.logger.Errorf("load %s main rule chain error: %s",
				ue.username, err.Error())
		}
	}
}
