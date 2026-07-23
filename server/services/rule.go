package services

import (
	"github.com/rulego/rulego/api/types"
)

// ChainCatalog rules the Chain Directory service interface, available for use by other modules (such as mcp).
type ChainCatalog interface {
	List(username, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error)
	Get(username, chainId string) ([]byte, error)
	GetAsRuleChain(username, chainId string) (types.RuleChain, error)
}

// ChainExecutor rules, chain executor interface
type ChainExecutor interface {
	Execute(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error
	ExecuteAndWait(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error
}

// ChainLifecycleEvent Chain Lifecycle Event.
type ChainLifecycleEvent struct {
	Username string // The user of the chain (username in multi-tenancy models)
	ChainId  string
	DSL      []byte // The chain at the time of the event is defined as JSON
}

// ChainLifecycleListener is a chain lifecycle listener for embedded hosts to sense changes in the chain.
// The monitor should be idempotent and not blocking; Panic is captured by the caller and does not affect the main process.
type ChainLifecycleListener interface {
	// OnSaved chains are triggered after being saved to storage (covering creation and updates).
	OnSaved(event ChainLifecycleEvent)
	// OnDeployed chain is triggered after deployment to the engine pool.
	OnDeployed(event ChainLifecycleEvent)
	// OnUndeployed chain is triggered after removal from the engine pool.
	OnUndeployed(event ChainLifecycleEvent)
	// The OnDeleted chain is triggered after permanent deletion from storage.
	OnDeleted(event ChainLifecycleEvent)
}

// BaseChainLifecycleListener is an empty implementation of all events; after embedding, only the method of concern is covered.
type BaseChainLifecycleListener struct{}

func (BaseChainLifecycleListener) OnSaved(ChainLifecycleEvent)      {}
func (BaseChainLifecycleListener) OnDeployed(ChainLifecycleEvent)   {}
func (BaseChainLifecycleListener) OnUndeployed(ChainLifecycleEvent) {}
func (BaseChainLifecycleListener) OnDeleted(ChainLifecycleEvent)    {}

// RuleAdminService Rule chain management service interface
type RuleAdminService interface {
	SaveAndLoad(username, chainId string, def []byte) error
	Deploy(username, chainId string) error
	Undeploy(username, chainId string) error
	Delete(username, chainId string) error
	SaveBaseInfo(username, chainId string, baseInfo types.RuleChainBaseInfo) error
	SaveConfiguration(username, chainId string, key string, configuration interface{}) error
	SetMainChainId(username, chainId string) error
	GetEngine(username, chainId string) (types.RuleEngine, bool)
	GetRuleConfig(username string) types.Config
	GetSetting(username, key string) string
	// AddLifecycleListener registers the chain lifecycle listener and must be called before App.Start().
	AddLifecycleListener(listener ChainLifecycleListener)
}
