// Package engine 提供多租户规则引擎池管理和用户级引擎实例。
package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
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
	rulegopool "github.com/rulego/rulego/utils/pool"

	"github.com/rulego/rulego/components/action"
)

// UserEngine 用户级规则引擎，管理引擎池和配置
type UserEngine struct {
	pool       *rulego.RuleGo
	username   string
	config     config.Config
	ruleConfig types.Config
	// ruleConfigMu 保护 ruleConfig 热替换（链保存取 config 与 global 推送并发）
	ruleConfigMu sync.RWMutex
	logger       types.Logger
	ruleStore    store.RuleStore
	setStore     store.SettingStore
	container    *app.Container // 服务容器，供全局回调按需懒取 RunLogService 等服务；nil 表示不可用
	workerPool   *rulegopool.WorkerPool
	// 系统注入节点 id（share_http_server 主端点），热更重建跳过
	systemNodeId string
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

// RangeUserEngines 遍历全部用户引擎，f 返回 false 提前终止
func (m *Manager) RangeUserEngines(f func(ue *UserEngine) bool) {
	m.locker.RLock()
	engines := make([]*UserEngine, 0, len(m.pool))
	for _, ue := range m.pool {
		engines = append(engines, ue)
	}
	m.locker.RUnlock()
	for _, ue := range engines {
		if !f(ue) {
			return
		}
	}
}

// PropagateGlobal 推送新 global 表并重载受影响的链与共享节点，单引擎失败不阻断其余
func (m *Manager) PropagateGlobal(newGlobal map[string]string, changedKeys []string) services.GlobalReloadResult {
	result := services.GlobalReloadResult{
		FailedChains: map[string]string{},
		FailedNodes:  map[string]string{},
	}
	if len(changedKeys) == 0 {
		return result
	}
	m.RangeUserEngines(func(ue *UserEngine) bool {
		ue.UpdateGlobalProperties(newGlobal)
		nodes, nodeErrs := ue.ReloadSharedNodesReferencingGlobal(changedKeys)
		result.ReloadedNodes = append(result.ReloadedNodes, nodes...)
		for id, err := range nodeErrs {
			result.FailedNodes[id] = err.Error()
		}
		chains, chainErrs := ue.ReloadChainsReferencingGlobal(changedKeys)
		result.ReloadedChains = append(result.ReloadedChains, chains...)
		for id, err := range chainErrs {
			result.FailedChains[id] = err.Error()
		}
		return true
	})
	return result
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
	// DB 类存储（无 workflows/<user> 目录可扫描）经可选的 ListUsernames 发现用户，
	// 否则其引擎与规则链（含 endpoint/schedule 定时端点）不会随启动恢复——
	// 宿主以 gorm 等 DB Provider 嵌入时即此形态：重启后规则链 404、定时任务消失。
	// 类型断言可选实现，文件存储等既有 Provider 不受影响。
	if lp, ok := m.storeProvider.(interface {
		ListUsernames() ([]string, error)
	}); ok {
		if names, err := lp.ListUsernames(); err == nil {
			for _, name := range names {
				if name == "" {
					continue
				}
				if _, err := m.GetOrCreate(name); err != nil {
					m.logger.Errorf("Init %s error: %s", name, err.Error())
				}
			}
		} else {
			m.logger.Errorf("list usernames from store error: %s", err.Error())
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
	var systemNodeId string
	if m.systemEp != nil {
		if ctx, err := pool.AddNode(m.systemEp); err != nil {
			m.logger.Errorf("inject system endpoint into user=%s pool error: %s", username, err)
		} else if ctx != nil {
			systemNodeId = ctx.GetNodeId().Id
		}
	}

	// 有界工作协程池：池满后新任务在调用方协程同步执行，单用户 goroutine 总量有上界；
	// 未配置上限时保持无限池，行为与旧版一致
	var workerOpt types.Option
	var workerPool *rulegopool.WorkerPool
	if cfg.WorkerPoolMaxWorkers > 0 {
		workerPool = &rulegopool.WorkerPool{MaxWorkersCount: cfg.WorkerPoolMaxWorkers}
		workerPool.Start()
		workerOpt = types.WithPool(workerPool)
	} else {
		workerOpt = types.WithDefaultPool()
	}

	ruleConfig := rulego.NewConfig(workerOpt,
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
		pool:         rulego.NewRuleGo(),
		username:     username,
		config:       *cfg,
		ruleConfig:   ruleConfig,
		logger:       logger,
		ruleStore:    ruleStore,
		setStore:     setStore,
		container:    m.container,
		workerPool:   workerPool,
		systemNodeId: systemNodeId,
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
	if ue.workerPool != nil {
		ue.workerPool.Stop()
	}
}

// Pool 返回底层 RuleGo 池
func (ue *UserEngine) Pool() *rulego.RuleGo {
	return ue.pool
}

// RuleConfig 返回规则引擎配置
func (ue *UserEngine) RuleConfig() types.Config {
	ue.ruleConfigMu.RLock()
	defer ue.ruleConfigMu.RUnlock()
	return ue.ruleConfig
}

// UpdateGlobalProperties 用新表全量替换 Properties。必须新建 map：旧 map 仍被
// 运行中的 JS 沙箱持有引用，原地写会并发崩溃。newGlobal 取自 UpdateConfig 的
// 新表——ue.config.Global 是创建期快照。
func (ue *UserEngine) UpdateGlobalProperties(newGlobal map[string]string) {
	props := types.Properties{}
	for k, v := range newGlobal {
		props.PutValue(k, v)
	}
	ue.applyServerKeys(props)
	ue.ruleConfigMu.Lock()
	cfg := ue.ruleConfig
	cfg.Properties = props
	ue.ruleConfig = cfg
	ue.ruleConfigMu.Unlock()
	// 节点池 Init 时从自己的 Config 取值，须同步换新
	if np, ok := cfg.NodePool.(*node_pool.NodePool); ok {
		np.SetConfig(cfg)
	}
}

// globalJSRefPattern 匹配 DSL 中的 global.<标识符>：配置模板与 JS 运行时引用
// 都命中；后者在 DSL 无 ${} 痕迹、定位不到具体键
var globalJSRefPattern = regexp.MustCompile(`global\.[A-Za-z_]\w*`)

// globalRefHit 判断 DSL 是否受变更键影响：模板三种写法精确匹配；DSL 含任意
// global.<标识符> 引用时保守视为受影响——JS 运行时引用定位不到键，漏重载比
// 多重载代价大；完全不用 global 的链不受影响。
func globalRefHit(dsl string, keys []string) bool {
	if globalJSRefPattern.MatchString(dsl) {
		return true
	}
	for _, k := range keys {
		if strings.Contains(dsl, "${global."+k+"}") ||
			strings.Contains(dsl, `${global["`+k+`"]}`) ||
			strings.Contains(dsl, "${global['"+k+"']}") {
			return true
		}
	}
	return false
}

// ReloadChainsReferencingGlobal 精准重载 DSL 里引用了变更键的已加载链。
// 未引用的链不动（endpoint 不断、内存态不丢）；未加载的链保存时自然取新 config。
// 返回重载成功的链名与失败明细（单链失败不阻断其余）。
func (ue *UserEngine) ReloadChainsReferencingGlobal(keys []string) (reloaded []string, failed map[string]error) {
	failed = map[string]error{}
	cfg := ue.RuleConfig()
	ue.pool.Range(func(_, v any) bool {
		re, ok := v.(types.RuleEngine)
		if !ok {
			return true
		}
		dsl := string(re.DSL())
		if !globalRefHit(dsl, keys) {
			return true
		}
		if err := re.Reload(rulego.WithConfig(cfg)); err != nil {
			failed[re.Id()] = err
			return true
		}
		reloaded = append(reloaded, chainDisplayName(dsl, re.Id()))
		return true
	})
	return reloaded, failed
}

// chainDisplayName 从 DSL 取链名展示，解析失败回退 id
func chainDisplayName(dsl string, id string) string {
	var def struct {
		RuleChain struct {
			Name string `json:"name"`
		} `json:"ruleChain"`
	}
	if err := json.Unmarshal([]byte(dsl), &def); err == nil && def.RuleChain.Name != "" {
		return def.RuleChain.Name
	}
	return id
}

// ReloadSharedNodesReferencingGlobal 重载引用了变更键的共享节点：共享节点在
// 独立池内 Init，链重载覆盖不到。先用新 config 试 Init，成功才 Del+New，坏值
// 不动池内旧实例；不落盘（定义未变）。
func (ue *UserEngine) ReloadSharedNodesReferencingGlobal(keys []string) (reloaded []string, failed map[string]error) {
	failed = map[string]error{}
	np, ok := ue.RuleConfig().NodePool.(*node_pool.NodePool)
	if !ok {
		return
	}
	cfg := ue.RuleConfig()
	np.RangeRuleNodeDefs(func(def *types.RuleNode) bool {
		node := *def
		// 系统注入节点只读（share_http_server 主端点），重建会拉垮共享 server
		if node.Id == ue.systemNodeId {
			return true
		}
		if !globalRefHit(defToString(&node), keys) {
			return true
		}
		// 先用新 config 试 Init，坏值不动池内旧实例
		if _, err := rulegoEngine.InitNetResourceNodeCtx(cfg, nil, nil, &node); err != nil {
			failed[node.Id] = err
			return true
		}
		np.Del(node.Id)
		if _, err := np.NewFromRuleNode(node); err != nil {
			failed[node.Id] = err
			return true
		}
		reloaded = append(reloaded, node.Id)
		return true
	})
	return reloaded, failed
}

// defToString 序列化节点定义供 global 引用扫描
func defToString(def *types.RuleNode) string {
	b, err := json.Marshal(def)
	if err != nil {
		return ""
	}
	return string(b)
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
	// 先校验再落 setting：先写后查会在链不存在时留下脏 setting
	if _, ok := ue.pool.Get(chainId); !ok {
		return fmt.Errorf("please deploy rule chain first")
	}
	if err := ue.setStore.Save(constants.SettingKeyMainChainId, chainId); err != nil {
		return err
	}
	return nil
}

// SaveSetting 保存用户设置
func (ue *UserEngine) SaveSetting(key, value string) error {
	return ue.setStore.Save(key, value)
}

// GetSetting 获取用户设置
func (ue *UserEngine) GetSetting(key string) string {
	return ue.setStore.Get(key)
}

// applyServerKeys 把 server 级键灌进 Properties，与 global 键共用一个 map。
// global 热更重建 Properties 时须重放，否则 exec 白名单等配置丢失。
func (ue *UserEngine) applyServerKeys(props types.Properties) {
	props.PutValue(constants.LoadLuaLibs, ue.config.LoadLuaLibs)
	props.PutValue(action.KeyExecNodeWhitelist, ue.config.CmdWhiteList)
	props.PutValue(action.KeyExecNodeMode, ue.config.CmdMode)
	props.PutValue(action.KeyExecNodeDeny, ue.config.CmdDenyList)
	props.PutValue(action.KeyExecNodeDenyArgs, ue.config.CmdDenyArgs)
	props.PutValue(action.KeyWorkDir, ue.config.DataDir)
	if ue.config.FilePathWhiteList != "" {
		props.PutValue(constants.KeyFilePathWhitelist, ue.config.FilePathWhiteList)
	}
}

func (ue *UserEngine) initRuleConfig() {
	for k, v := range ue.config.Global {
		ue.ruleConfig.Properties.PutValue(k, fmt.Sprintf("%v", v))
	}
	ue.applyServerKeys(ue.ruleConfig.Properties)
	if ue.config.ScriptMaxExecutionTime > 0 {
		ue.ruleConfig.ScriptMaxExecutionTime = time.Millisecond * time.Duration(ue.config.ScriptMaxExecutionTime)
	}
	if ue.config.MsgMaxHops > 0 {
		ue.ruleConfig.MsgMaxHops = ue.config.MsgMaxHops
	}
	if ue.config.EndpointEnabled != nil {
		ue.ruleConfig.EndpointEnabled = *ue.config.EndpointEnabled
	}
	if ue.config.Locker != nil {
		ue.ruleConfig.Locker = ue.config.Locker
		// owner 即用户分区名：保证不同租户的同名端点与路由生成不同的锁键
		ue.ruleConfig.Owner = ue.username
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
		runlog.DefaultDebugDataStore.Add(ue.username, chainId, nodeId, logData)
		// 推送到 WebSocket 客户端
		runlog.SendDebugDataToClients(ue.username, chainId, logData)
		// 子规则链调试日志同步推送到调试发起的根链路，使主链路控制台可见
		if root := msg.Metadata.GetValue(constants.ParamRootChainId); root != "" && root != chainId {
			runlog.SendDebugDataToClients(ue.username, root, logData)
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
		// 下线链（disabled）跳过：initChain 会因 ErrEngineDisabled 报错，
		// 不跳过则每次启动对每条草稿链打一条误导性 error 日志；且下线链
		// 本就不应恢复运行（定时端点随部署/下线启停）。
		if chainDisabled(def) {
			continue
		}
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

// chainDisabled 轻量探测 DSL 的 ruleChain.disabled（只解头部字段，boot 期跳过下线链用）。
func chainDisabled(def []byte) bool {
	var head struct {
		RuleChain struct {
			Disabled bool `json:"disabled"`
		} `json:"ruleChain"`
	}
	return json.Unmarshal(def, &head) == nil && head.RuleChain.Disabled
}
