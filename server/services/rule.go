package services

import (
	"github.com/rulego/rulego/api/types"
)

// ChainCatalog 规则链目录服务接口，供其他模块（如 mcp）使用
type ChainCatalog interface {
	List(username, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error)
	Get(username, chainId string) ([]byte, error)
	GetAsRuleChain(username, chainId string) (types.RuleChain, error)
}

// ChainExecutor 规则链执行器接口
type ChainExecutor interface {
	Execute(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error
	ExecuteAndWait(username, chainId string, msg types.RuleMsg, opts ...types.RuleContextOption) error
}

// ChainLifecycleEvent 链生命周期事件。
type ChainLifecycleEvent struct {
	Username string // 链所属用户（多租户模型下即 username）
	ChainId  string
	DSL      []byte // 事件发生时的链定义 JSON
}

// ChainLifecycleListener 链生命周期监听器，供嵌入式宿主感知链变更。
// 监听器应幂等且非阻塞；panic 由调用方捕获，不影响主流程。
type ChainLifecycleListener interface {
	// OnSaved 链保存到存储后触发（涵盖创建和更新）。
	OnSaved(event ChainLifecycleEvent)
	// OnDeployed 链部署到引擎池后触发。
	OnDeployed(event ChainLifecycleEvent)
	// OnUndeployed 链从引擎池移除后触发。
	OnUndeployed(event ChainLifecycleEvent)
	// OnDeleted 链从存储永久删除后触发。
	OnDeleted(event ChainLifecycleEvent)
}

// BaseChainLifecycleListener 全部事件的空实现，嵌入后只覆盖关心的方法即可。
type BaseChainLifecycleListener struct{}

func (BaseChainLifecycleListener) OnSaved(ChainLifecycleEvent)      {}
func (BaseChainLifecycleListener) OnDeployed(ChainLifecycleEvent)   {}
func (BaseChainLifecycleListener) OnUndeployed(ChainLifecycleEvent) {}
func (BaseChainLifecycleListener) OnDeleted(ChainLifecycleEvent)    {}

// RuleAdminService 规则链管理服务接口
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
	// AddLifecycleListener 注册链生命周期监听器，须在 App.Start() 之前调用。
	AddLifecycleListener(listener ChainLifecycleListener)
}
