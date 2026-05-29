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
}
