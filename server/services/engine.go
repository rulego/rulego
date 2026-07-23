package services

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/store"
)

// UserEngine user-level rule engine interface
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

// EngineManager multi-tenant engine manager interface
type EngineManager interface {
	GetOrCreate(username string) (UserEngine, error)
	Get(username string) (UserEngine, bool)
	InitUserEngines() error
	Stop()
}
