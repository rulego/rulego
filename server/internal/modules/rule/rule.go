package rule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/engine"
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
	cfg    *config.Config
	logger types.Logger
	engine *engine.Manager
}

// New 创建 rule 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string     { return ModuleName }
func (m *Module) Priority() int    { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	storeProvider, err := app.GetAs[store.StoreProvider](ctx.Container, "store.provider")
	if err != nil {
		return err
	}

	m.engine = engine.NewManager(m.cfg, m.logger, storeProvider)

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
	// 当前用户 pool 未找到时，回退到 DefaultUsername 的 pool（支持系统智能体等共享链）
	if username != m.cfg.DefaultUsername {
		defaultUe, err := m.getUserEngine(m.cfg.DefaultUsername)
		if err != nil {
			return err
		}
		if e, ok := defaultUe.GetEngine(chainId); ok {
			e.OnMsg(msg, opts...)
			return nil
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
	// 当前用户 pool 未找到时，回退到 DefaultUsername 的 pool（支持系统智能体等共享链）
	if username != m.cfg.DefaultUsername {
		defaultUe, err := m.getUserEngine(m.cfg.DefaultUsername)
		if err != nil {
			return err
		}
		if e, ok := defaultUe.GetEngine(chainId); ok {
			e.OnMsgAndWait(msg, opts...)
			return nil
		}
	}
	return fmt.Errorf("chain not found: %s", chainId)
}

// RuleAdminService 实现

func (m *Module) SaveAndLoad(username, chainId string, def []byte) error {
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return err
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
	if ruleChain.RuleChain.Disabled {
		return m.Undeploy(username, chainId)
	}
	return m.Deploy(username, chainId)
}

func (m *Module) Deploy(username, chainId string) error {
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
	return err
}

func (m *Module) Undeploy(username, chainId string) error {
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
	return ue.RuleStore().Save(username, chainId, b)
}

func (m *Module) Delete(username, chainId string) error {
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
	return ue.RuleStore().Delete(username, chainId)
}

func (m *Module) SaveBaseInfo(username, chainId string, baseInfo types.RuleChainBaseInfo) error {
	if chainId == "" {
		return errors.New("chainId is empty")
	}
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	_ = ue.SaveSetting(constants.SettingKeyLatestChainId, chainId)
	ruleEngine, ok := ue.GetEngine(chainId)
	if ok {
		def := ruleEngine.RootRuleChainCtx().Definition()
		def.RuleChain.AdditionalInfo = baseInfo.AdditionalInfo
		def.RuleChain.Name = baseInfo.Name
		def.RuleChain.Root = baseInfo.Root
		def.RuleChain.DebugMode = baseInfo.DebugMode
		_ = maps.Map2Struct(baseInfo.Configuration, &def.RuleChain.Configuration)
		m.fillAdditionalInfo(ue, def)
		defBytes, err := json.Format(ruleEngine.DSL())
		if err != nil {
			return err
		}
		return ue.RuleStore().Save(username, chainId, defBytes)
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
	ue, err := m.getUserEngine(username)
	if err != nil {
		return err
	}
	_ = ue.SaveSetting(constants.SettingKeyLatestChainId, chainId)
	ruleEngine, ok := ue.GetEngine(chainId)
	if !ok {
		return errors.New("chain not found: " + chainId)
	}
	self := ruleEngine.RootRuleChainCtx().Definition()
	if self.RuleChain.Configuration == nil {
		self.RuleChain.Configuration = make(types.Configuration)
	}
	self.RuleChain.Configuration[key] = configuration
	m.fillAdditionalInfo(ue, self)
	if err := ruleEngine.ReloadSelf(ruleEngine.DSL()); err != nil {
		return err
	}
	def, err := json.Format(ruleEngine.DSL())
	if err != nil {
		return err
	}
	return ue.RuleStore().Save(username, chainId, def)
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

func (m *Module) saveRuleChain(ue services.UserEngine, ruleChain types.RuleChain, whenErr error) {
	if whenErr != nil {
		ruleChain.RuleChain.PutAdditionalInfo(constants.AddiKeyMessage, whenErr.Error())
	}
	if def, err := json.Marshal(ruleChain); err == nil {
		_ = ue.RuleStore().Save(ue.Username(), ruleChain.RuleChain.ID, def)
	}
}
