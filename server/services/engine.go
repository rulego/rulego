package services

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/store"
)

// UserEngine 用户级规则引擎接口
type UserEngine interface {
	Pool() *rulego.RuleGo
	RuleConfig() types.Config
	RuleStore() store.RuleStore
	GetEngine(chainId string) (types.RuleEngine, bool)
	SetMainChainId(chainId string) error
	Username() string
	SaveSetting(key, value string) error
	GetSetting(key string) string
}

// EngineManager 多租户引擎管理器接口
type EngineManager interface {
	GetOrCreate(username string) (UserEngine, error)
	Get(username string) (UserEngine, bool)
	InitUserEngines() error
	// Remove 移除并停止指定用户的引擎，用户不存在时返回 nil（幂等）
	Remove(username string) error
	Stop()
}
