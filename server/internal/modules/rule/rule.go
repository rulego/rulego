package rule

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/engine"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/maps"
)

const (
	ModuleName = "rule"
	Priority   = 30
)

// Module rule 业务模块
type Module struct {
	cfg                *config.Config
	logger             types.Logger
	engine             *engine.Manager
	lifecycleListeners []services.ChainLifecycleListener
	listenersMu        sync.RWMutex
	// opLocks 按 (username, chainId) 分条串行化生命周期操作（Save/Deploy/Undeploy/Delete），
	// 防止并发交错产生「已删链被复活」「停链未落盘」等存储态与内存态背离；
	// 分条免清理，条内冲突概率 1/256 可忽略
	opLocks [256]sync.Mutex
}

// New 创建 rule 模块
func New() *Module {
	return &Module{}
}

// lockChainOp 对同一条链的生命周期操作加锁，返回解锁函数。
// sync.Mutex 不可重入：SaveAndLoad 持锁期间须调用 deployLocked/undeployLocked，
// 新增内部调用若再进公共方法会自死锁。
func (m *Module) lockChainOp(username, chainId string) func() {
	var h uint32 = 2166136261
	for i := 0; i < len(username); i++ {
		h ^= uint32(username[i])
		h *= 16777619
	}
	h ^= '/'
	for i := 0; i < len(chainId); i++ {
		h ^= uint32(chainId[i])
		h *= 16777619
	}
	l := &m.opLocks[h%256]
	l.Lock()
	return l.Unlock
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	storeProvider, err := app.GetAs[store.StoreProvider](ctx.Container, "store.provider")
	if err != nil {
		return err
	}

	m.engine = engine.NewManager(m.cfg, m.logger, storeProvider)
	// 注入服务容器，供全局 OnRuleChainCompleted 回调懒取 RunLogService（覆盖所有触发源）
	m.engine.SetContainer(ctx.Container)

	if err := ctx.Container.Register(services.KeyRuleCatalog, services.ChainCatalog(m)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyRuleExecutor, services.ChainExecutor(m)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyRuleManager, services.RuleAdminService(m)); err != nil {
		return err
	}
	if err := ctx.Container.Register(services.KeyEngineManager, services.EngineManager(m.engine)); err != nil {
		return err
	}

	return nil
}

func (m *Module) Start(_ context.Context) error {
	if err := m.engine.InitUserEngines(); err != nil {
		return err
	}
	return m.deploySystemAgents()
}

func (m *Module) Stop(_ context.Context) error {
	m.engine.Stop()
	return nil
}

func (m *Module) getUserEngine(username string) (services.UserEngine, error) {
	return m.engine.GetOrCreate(username)
}

// ChainCatalog 实现

func (m *Module) List(username, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error) {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return nil, 0, err
	}
	return ue.RuleStore().List(username, keywords, root, disabled, category, size, page)
}

func (m *Module) Get(username, chainId string) ([]byte, error) {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return nil, err
	}
	return ue.RuleStore().Get(username, chainId)
}

func (m *Module) GetAsRuleChain(username, chainId string) (types.RuleChain, error) {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return types.RuleChain{}, err
	}
	return ue.RuleStore().GetAsRuleChain(username, chainId)
}

// ChainExecutor 实现

func (m *Module) Execute(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	if e, ok := ue.GetEngine(chainId); ok {
		e.OnMsg(msg, opts...)
		return nil
	}
	// 当前用户 pool 未找到时，仅系统智能体允许回退到 DefaultUsername 的 pool 执行（避免访问他人私有链）
	// 注：纯匿名免登陆请求 username 已是 DefaultUsername，不会进入此分支
	if username != m.cfg.DefaultUsername {
		defaultUe, err := m.getUserEngine(m.cfg.DefaultUsername)
		if err != nil {
			return err
		}
		if e, ok := defaultUe.GetEngine(chainId); ok {
			if m.isSystemAgentEngine(e) {
				e.OnMsg(msg, opts...)
				return nil
			}
		}
	}
	return fmt.Errorf("chain not found: %s", chainId)
}

func (m *Module) ExecuteAndWait(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	if e, ok := ue.GetEngine(chainId); ok {
		e.OnMsgAndWait(msg, opts...)
		return nil
	}
	// 当前用户 pool 未找到时，仅系统智能体允许回退到 DefaultUsername 的 pool 执行（避免访问他人私有链）
	// 注：纯匿名免登陆请求 username 已是 DefaultUsername，不会进入此分支
	if username != m.cfg.DefaultUsername {
		defaultUe, err := m.getUserEngine(m.cfg.DefaultUsername)
		if err != nil {
			return err
		}
		if e, ok := defaultUe.GetEngine(chainId); ok {
			if m.isSystemAgentEngine(e) {
				e.OnMsgAndWait(msg, opts...)
				return nil
			}
		}
	}
	return fmt.Errorf("chain not found: %s", chainId)
}

// RuleAdminService 实现

func (m *Module) SaveAndLoad(username, chainId string, def []byte) error {
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	// 保护服务端字段：禁止通过 SaveAndLoad 注入 systemAgent 标记
	// （否则任意链可伪装为不可删除的系统智能体）。系统智能体仅由服务端
	// 在 DefaultUsername 命名空间下部署（system_agents.go 经 markSystemAgent 标记）；
	// 普通用户命名空间一律剥离该标记，杜绝越权伪装。
	if ue.Username() != m.cfg.DefaultUsername && ruleChain.RuleChain.AdditionalInfo != nil {
		delete(ruleChain.RuleChain.AdditionalInfo, constants.KeySystemAgent)
	}
	// 系统智能体不更新最后操作规则链ID
	if !m.isSystemAgent(ruleChain) {
		_ = ue.SaveSetting(constants.SettingKeyLatestChainId, chainId)
	}
	m.fillAdditionalInfo(ue, &ruleChain)
	b, err := json.Marshal(ruleChain)
	if err != nil {
		return err
	}
	if err = ue.RuleStore().Save(username, chainId, b); err != nil {
		return err
	}
	m.fireSaved(username, chainId, b)
	if ruleChain.RuleChain.Disabled {
		return m.undeployLocked(username, chainId)
	}
	return m.deployLocked(username, chainId)
}

func (m *Module) Deploy(username, chainId string) error {
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	return m.deployLocked(username, chainId)
}

func (m *Module) deployLocked(username, chainId string) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	def, err := ue.RuleStore().Get(username, chainId)
	if err != nil {
		return err
	}
	var ruleChain types.RuleChain
	if err = json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	ruleChain.RuleChain.Disabled = false
	def, err = json.Marshal(ruleChain)
	if err != nil {
		return err
	}
	ruleEngine, ok := ue.GetEngine(chainId)
	if ok {
		err = ruleEngine.ReloadSelf(def)
	} else {
		_, err = ue.Pool().New(chainId, def, rulego.WithConfig(ue.RuleConfig()))
	}
	m.saveRuleChain(ue, ruleChain, err)
	if err == nil {
		m.fireDeployed(username, chainId, def)
	}
	return err
}

func (m *Module) Undeploy(username, chainId string) error {
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	return m.undeployLocked(username, chainId)
}

func (m *Module) undeployLocked(username, chainId string) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	def, err := ue.RuleStore().Get(username, chainId)
	if err != nil {
		return err
	}
	var ruleChain types.RuleChain
	if err = json.Unmarshal(def, &ruleChain); err != nil {
		return err
	}
	ue.Pool().Del(chainId)
	ruleChain.RuleChain.Disabled = true
	b, err := json.Marshal(ruleChain)
	if err != nil {
		return err
	}
	if err = ue.RuleStore().Save(username, chainId, b); err != nil {
		return err
	}
	m.fireUndeployed(username, chainId, b)
	return nil
}

// AddLifecycleListener 注册链生命周期监听器（线程安全），须在 App.Start() 之前调用。
func (m *Module) AddLifecycleListener(listener services.ChainLifecycleListener) {
	m.listenersMu.Lock()
	defer m.listenersMu.Unlock()
	m.lifecycleListeners = append(m.lifecycleListeners, listener)
}

func (m *Module) fireDeployed(username, chainId string, dsl []byte) {
	m.broadcast(services.ChainLifecycleEvent{Username: username, ChainId: chainId, DSL: dsl},
		func(l services.ChainLifecycleListener, e services.ChainLifecycleEvent) { l.OnDeployed(e) })
}

func (m *Module) fireUndeployed(username, chainId string, dsl []byte) {
	m.broadcast(services.ChainLifecycleEvent{Username: username, ChainId: chainId, DSL: dsl},
		func(l services.ChainLifecycleListener, e services.ChainLifecycleEvent) { l.OnUndeployed(e) })
}

func (m *Module) fireSaved(username, chainId string, dsl []byte) {
	m.broadcast(services.ChainLifecycleEvent{Username: username, ChainId: chainId, DSL: dsl},
		func(l services.ChainLifecycleListener, e services.ChainLifecycleEvent) { l.OnSaved(e) })
}

func (m *Module) fireDeleted(username, chainId string, dsl []byte) {
	m.broadcast(services.ChainLifecycleEvent{Username: username, ChainId: chainId, DSL: dsl},
		func(l services.ChainLifecycleListener, e services.ChainLifecycleEvent) { l.OnDeleted(e) })
}

// broadcast 向所有监听器派发事件，单个监听器 panic 不影响其他。
func (m *Module) broadcast(event services.ChainLifecycleEvent, invoke func(services.ChainLifecycleListener, services.ChainLifecycleEvent)) {
	for _, l := range m.snapshotListeners() {
		m.safeNotify(l, func(l services.ChainLifecycleListener) { invoke(l, event) })
	}
}

func (m *Module) snapshotListeners() []services.ChainLifecycleListener {
	m.listenersMu.RLock()
	defer m.listenersMu.RUnlock()
	snapshot := make([]services.ChainLifecycleListener, len(m.lifecycleListeners))
	copy(snapshot, m.lifecycleListeners)
	return snapshot
}

// safeNotify 调用单个监听器，捕获 panic 避免影响其他监听器或主流程。
func (m *Module) safeNotify(l services.ChainLifecycleListener, invoke func(services.ChainLifecycleListener)) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Errorf("chain lifecycle listener panic: %v", r)
		}
	}()
	invoke(l)
}

func (m *Module) Delete(username, chainId string) error {
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	chain, err := ue.RuleStore().GetAsRuleChain(username, chainId)
	if err != nil {
		return err
	}
	if v, ok := chain.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
		if b, ok := v.(bool); ok && b {
			return fmt.Errorf("系统内置智能体不允许删除")
		}
	}
	ue.Pool().Del(chainId)
	if err = ue.RuleStore().Delete(username, chainId); err != nil {
		return err
	}
	// 链已删除，调试数据不再有意义，随链清理避免内存滞留
	runlog.DefaultDebugDataStore.Clear(username, chainId)
	m.fireDeleted(username, chainId, nil)
	return nil
}

func (m *Module) SaveBaseInfo(username, chainId string, baseInfo types.RuleChainBaseInfo) error {
	if chainId == "" {
		return errors.New("chainId is empty")
	}
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	// 保护服务端字段：禁止通过基础信息注入 systemAgent 标记（否则任意链可伪装为不可删除的系统智能体）
	if baseInfo.AdditionalInfo != nil {
		delete(baseInfo.AdditionalInfo, constants.KeySystemAgent)
	}
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	_ = ue.SaveSetting(constants.SettingKeyLatestChainId, chainId)
	ruleEngine, ok := ue.GetEngine(chainId)
	if ok {
		// 在 DSL 快照的副本上修改：直接改运行中引擎的 Definition 会与消息执行路径
		// 并发读写 AdditionalInfo map，触发进程级 fatal（不可 recover）
		def, err := snapshotDefinition(ruleEngine)
		if err != nil {
			return err
		}
		// 保留原有 systemAgent 标记（系统智能体编辑后仍保持受保护），其余以提交的 additionalInfo 为准
		sysAgent, _ := def.RuleChain.GetAdditionalInfo(constants.KeySystemAgent)
		def.RuleChain.AdditionalInfo = baseInfo.AdditionalInfo
		if def.RuleChain.AdditionalInfo == nil {
			def.RuleChain.AdditionalInfo = make(map[string]interface{})
		}
		if sysAgent != nil {
			def.RuleChain.AdditionalInfo[constants.KeySystemAgent] = sysAgent
		}
		def.RuleChain.Name = baseInfo.Name
		def.RuleChain.Root = baseInfo.Root
		def.RuleChain.DebugMode = baseInfo.DebugMode
		_ = maps.Map2Struct(baseInfo.Configuration, &def.RuleChain.Configuration)
		m.fillAdditionalInfo(ue, def)
		defBytes, err := json.Marshal(def)
		if err != nil {
			return err
		}
		if err := ruleEngine.ReloadSelf(defBytes); err != nil {
			return err
		}
		formatted, err := json.Format(defBytes)
		if err != nil {
			return err
		}
		return ue.RuleStore().Save(username, chainId, formatted)
	}
	def := types.RuleChain{RuleChain: baseInfo}
	m.fillAdditionalInfo(ue, &def)
	defBytes, err := json.Marshal(def)
	if err != nil {
		return err
	}
	if _, err := ue.Pool().New(chainId, defBytes, rulego.WithConfig(ue.RuleConfig())); err != nil {
		return err
	}
	return ue.RuleStore().Save(username, chainId, defBytes)
}

func (m *Module) SaveConfiguration(username, chainId string, key string, configuration interface{}) error {
	if chainId == "" {
		return errors.New("chainId is empty")
	}
	unlock := m.lockChainOp(username, chainId)
	defer unlock()
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	_ = ue.SaveSetting(constants.SettingKeyLatestChainId, chainId)
	ruleEngine, ok := ue.GetEngine(chainId)
	if !ok {
		return errors.New("chain not found: " + chainId)
	}
	self, err := snapshotDefinition(ruleEngine)
	if err != nil {
		return err
	}
	if self.RuleChain.Configuration == nil {
		self.RuleChain.Configuration = make(types.Configuration)
	}
	self.RuleChain.Configuration[key] = configuration
	m.fillAdditionalInfo(ue, self)
	defBytes, err := json.Marshal(self)
	if err != nil {
		return err
	}
	if err := ruleEngine.ReloadSelf(defBytes); err != nil {
		return err
	}
	formatted, err := json.Format(defBytes)
	if err != nil {
		return err
	}
	return ue.RuleStore().Save(username, chainId, formatted)
}

// snapshotDefinition 从引擎 DSL 快照反序列化出可安全修改的定义副本。
func snapshotDefinition(ruleEngine types.RuleEngine) (*types.RuleChain, error) {
	var def types.RuleChain
	if err := json.Unmarshal(ruleEngine.DSL(), &def); err != nil {
		return nil, err
	}
	return &def, nil
}

func (m *Module) SetMainChainId(username, chainId string) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	return ue.SetMainChainId(chainId)
}

func (m *Module) GetEngine(username, chainId string) (types.RuleEngine, bool) {
	ue, ok := m.engine.Get(username)
	if !ok {
		return nil, false
	}
	return ue.GetEngine(chainId)
}

func (m *Module) GetRuleConfig(username string) types.Config {
	ue, ok := m.engine.Get(username)
	if !ok {
		return types.Config{}
	}
	return ue.RuleConfig()
}

func (m *Module) GetSetting(username, key string) string {
	ue, ok := m.engine.Get(username)
	if !ok {
		return ""
	}
	return ue.GetSetting(key)
}

func (m *Module) ComponentService(username string) services.UserEngine {
	ue, ok := m.engine.Get(username)
	if !ok {
		return nil
	}
	return ue
}

// EngineManager 返回引擎管理器
func (m *Module) EngineManager() services.EngineManager {
	return m.engine
}

func (m *Module) MCPService(username string) interface{} {
	return (*mcpServiceStub)(nil)
}

// mcpServiceStub 占位实现，由 with_ai 构建标签下的真实实现替换
type mcpServiceStub struct{}

func (s *mcpServiceStub) Name() string { return "mcp (not enabled)" }

func (m *Module) fillAdditionalInfo(ue services.UserEngine, def *types.RuleChain) {
	if def.RuleChain.AdditionalInfo == nil {
		def.RuleChain.AdditionalInfo = make(map[string]interface{})
	}
	def.RuleChain.AdditionalInfo[constants.KeyUsername] = ue.Username()
	nowStr := time.Now().Format("2006/01/02 15:04:05")
	if _, ok := def.RuleChain.AdditionalInfo["createTime"]; !ok {
		def.RuleChain.AdditionalInfo["createTime"] = nowStr
	}
	def.RuleChain.AdditionalInfo["updateTime"] = nowStr
}

func (m *Module) isSystemAgent(ruleChain types.RuleChain) bool {
	if v, ok := ruleChain.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	return false
}

// isSystemAgentEngine 判断引擎对应的规则链是否为系统智能体。
// 用于跨用户执行回退的鉴权：仅系统智能体（部署在 DefaultUsername 名下的共享链）
// 允许被其他用户执行，避免访问 admin 的私有链。
// 注：纯匿名免登陆请求 username 已是 DefaultUsername，不会进入调用此方法的回退分支。
func (m *Module) isSystemAgentEngine(e types.RuleEngine) bool {
	if e == nil {
		return false
	}
	if def := e.RootRuleChainCtx().Definition(); def != nil {
		if v, ok := def.RuleChain.GetAdditionalInfo(constants.KeySystemAgent); ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return false
}

func (m *Module) saveRuleChain(ue services.UserEngine, ruleChain types.RuleChain, whenErr error) {
	if whenErr != nil {
		ruleChain.RuleChain.PutAdditionalInfo(constants.AddiKeyMessage, whenErr.Error())
	}
	if def, err := json.Marshal(ruleChain); err == nil {
		_ = ue.RuleStore().Save(ue.Username(), ruleChain.RuleChain.ID, def)
	}
}
