// Package engine provides multi-tenant rule engine pool management and user-level engine instances.
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
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/internal/modules/runlog"
	"github.com/rulego/rulego/server/store"
	"github.com/rulego/rulego/utils/fs"

	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/server/services"
)

// UserEngine is a user-level rule engine that manages engine pools and configurations
type UserEngine struct {
	pool       *rulego.RuleGo
	username   string
	config     config.Config
	ruleConfig types.Config
	logger     types.Logger
	ruleStore  store.RuleStore
	setStore   store.SettingStore
	locker     sync.RWMutex
	mainEngine types.RuleEngine
}

// Manager manages the multi-tenant user engine pool
type Manager struct {
	pool          map[string]*UserEngine
	locker        sync.RWMutex
	cfg           *config.Config
	logger        types.Logger
	storeProvider store.StoreProvider
	systemEp      types.Node // System endpoints shared with the user pool (such as the main HTTP server) are injected by SetSystemEndpoint; nil means no injection
}

// NewManager creates an engine manager
func NewManager(cfg *config.Config, logger types.Logger, storeProvider store.StoreProvider) *Manager {
	return &Manager{
		pool:          make(map[string]*UserEngine),
		cfg:           cfg,
		logger:        logger,
		storeProvider: storeProvider,
	}
}

// SetSystemEndpoint sets the system endpoint to be injected into each user pool (such as the main HTTP server when share_http_server is enabled).
// When setting up a new user engine, this endpoint will be added to the user node pool for user rule chains to be referenced by the user rule chain via ref://.
func (m *Manager) SetSystemEndpoint(ep types.Node) {
	m.systemEp = ep
}

// GetOrCreate fetchs or creates a user engine, using double-check locking to prevent race states
func (m *Manager) GetOrCreate(username string) (services.UserEngine, error) {
	if ue, ok := m.get(username); ok {
		return ue, nil
	}
	m.locker.Lock()
	defer m.locker.Unlock()
	// After obtaining the write lock, check again to prevent concurrency creation
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

// Get an existing user engine
func (m *Manager) Get(username string) (services.UserEngine, bool) {
	return m.get(username)
}

// InitUserEngines initializes the engine for an existing user directory.
// Implemented in two phases:
//  1. Create all user engines (without loading the rule chain), and have modules like MCP register UDF during the Start phase
//  2. Load all rule chains uniformly; at this point, the UDF (such as mcp_tool_provider) is ready
func (m *Manager) InitUserEngines() error {
	userPath := path.Join(m.cfg.DataDir, constants.DirWorkflows)
	_ = fs.CreateDirs(userPath)

	// Phase 1: Create all user engines (no rule chains loaded)
	entries, err := os.ReadDir(userPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := m.GetOrCreate(entry.Name()); err != nil {
				m.logger.Errorf("Init %s error: %s", entry.Name(), err.Error())
			}
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

	// Phase 2: Uniformly load the rule chain
	// At this point, modules like MCP have registered UDFs (such as mcp_tool_provider) through GetOrCreate, allowing safe loading of rule chains containing AI/agent nodes.
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

// Stop: Stop all user engines
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

// newUserEngine creates user-level engine instances
func (m *Manager) newUserEngine(username string) (*UserEngine, error) {
	cfg := m.cfg
	logger := m.logger
	componentRegistry := rulegoEngine.NewCustomComponentRegistry(rulegoEngine.Registry, new(rulegoEngine.RuleComponentRegistry))
	poolConfig := rulego.NewConfig(types.WithComponentsRegistry(componentRegistry), types.WithLogger(logger))
	pool := node_pool.NewNodePool(poolConfig)
	// Inject system endpoints (such as the main HTTP server) into the user pool for user rule chains to be referenced by ref://.
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
	}

	ue.initRuleConfig()
	// Make sure the UDF map is initialized
	if ue.ruleConfig.Udf == nil {
		ue.ruleConfig.Udf = make(map[string]interface{})
	}
	ue.loadJs()
	ue.loadPlugins()
	// Note: Do not load the rule chain during the creation phase.
	// The rule chain is uniformly loaded by InitUserEngines() to ensure the UDF of modules like MCP (such as mcp_tool_provider)
	// Complete registration before the rule chain is initialized.

	return ue, nil
}

// Stop: Stop the engine
func (ue *UserEngine) Stop() {
	if ue.pool != nil {
		ue.pool.Stop()
	}
}

// Pool: Returns the underlying RuleGo pool
func (ue *UserEngine) Pool() *rulego.RuleGo {
	return ue.pool
}

// RuleConfig returns the rule engine configuration
func (ue *UserEngine) RuleConfig() types.Config {
	return ue.ruleConfig
}

// RuleStore returns the rule chain storage
func (ue *UserEngine) RuleStore() store.RuleStore {
	return ue.ruleStore
}

// SettingStore returns the settings storage.
func (ue *UserEngine) SettingStore() store.SettingStore {
	return ue.setStore
}

// Username returns the username
func (ue *UserEngine) Username() string {
	return ue.username
}

// GetEngine obtains the specified rule chain engine
func (ue *UserEngine) GetEngine(chainId string) (types.RuleEngine, bool) {
	return ue.pool.Get(chainId)
}

// LoadRule runs from the storage loading rule chain to the engine pool
func (ue *UserEngine) LoadRule(chainId string) error {
	def, err := ue.ruleStore.Get(ue.username, chainId)
	if err != nil {
		return err
	}
	return ue.loadDef(chainId, def)
}

// loadDef compiles the DSL into the engine pool (if it already exists, reload).
func (ue *UserEngine) loadDef(chainId string, def []byte) error {
	if ruleEngine, ok := ue.pool.Get(chainId); ok {
		return ruleEngine.ReloadSelf(def)
	}
	_, err := ue.pool.New(chainId, def, rulego.WithConfig(ue.ruleConfig))
	return err
}

// SetMainChainId sets the main rule chain
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

// SaveSetting saves user settings
func (ue *UserEngine) SaveSetting(key, value string) error {
	return ue.setStore.Save(key, value)
}

// GetSetting obtains user settings
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

	// OnDebug callback: Save to memory + push to WebSocket client
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
		// Stored in memory for REST API queries (used when double-clicking a node)
		runlog.DefaultDebugDataStore.Add(chainId, nodeId, logData)
		// Push to the WebSocket client
		runlog.SendDebugDataToClients(chainId, logData)
		// The sub-rule chain debugging log is synchronously pushed to the root link initiated by debugging, making it visible to the main link console
		if root := msg.Metadata.GetValue(constants.ParamRootChainId); root != "" && root != chainId {
			runlog.SendDebugDataToClients(root, logData)
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

// loadRules retrieves all of the user's rule chains at once via RuleStore.AllChains and loads them into the engine pool.
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
