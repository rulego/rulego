// Package store 定义存储层接口，具体实现由 internal/store 提供。
package store

import (
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/model"
)

// RuleStore 规则链存储接口
type RuleStore interface {
	// Save 保存规则链定义
	Save(username, chainId string, def []byte) error
	// Get 获取规则链原始 JSON 数据
	Get(username, chainId string) ([]byte, error)
	// GetAsRuleChain 获取规则链结构体
	GetAsRuleChain(username, chainId string) (types.RuleChain, error)
	// List 列出规则链，支持关键字搜索、root/disabled 过滤、category 过滤、分页排序
	List(username, keywords string, root *bool, disabled *bool, category string, size, page int) ([]types.RuleChain, int, error)
	// Delete 删除规则链
	Delete(username, chainId string) error
}

// UserStore 用户存储接口
type UserStore interface {
	// CreateUser 创建用户
	CreateUser(user model.User) error
	// ValidatePassword 验证密码
	ValidatePassword(username, password string) bool
	// Delete 删除用户
	Delete(username string) error
	// List 列出所有用户
	List() []model.User
}

// SettingStore 用户设置存储接口
type SettingStore interface {
	// Save 保存设置
	Save(key, value string) error
	// Get 获取设置值
	Get(key string) string
	// Delete 删除设置
	Delete(key string) error
	// Setting 获取完整的用户设置
	Setting() model.UserSetting
}

// RunLogStore 运行日志存储接口
type RunLogStore interface {
	// Save 保存运行日志
	Save(username string, event model.Event) error
	// List 列出运行日志，支持按 chainId 和时间范围过滤，分页
	List(username, chainId string, startTime, endTime time.Time, size, page int) ([]model.Event, int, error)
	// Get 获取单条运行日志
	Get(username, logId string) (model.Event, error)
	// Delete 删除运行日志
	Delete(username, logId string) error
	// DeleteByChainId 删除指定规则链的所有运行日志
	DeleteByChainId(username, chainId string) error
}

// ComponentStore 组件存储接口
type ComponentStore interface {
	// Save 保存组件定义
	Save(username, componentId string, def []byte) error
	// Get 获取组件定义
	Get(username, componentId string) ([]byte, error)
	// List 列出组件
	List(username, keywords string, size, page int) ([][]byte, int, error)
	// Delete 删除组件
	Delete(username, componentId string) error
}

// NodePoolStore 节点池存储接口
type NodePoolStore interface {
	// Save 保存节点池数据
	Save(data []byte) error
	// Get 获取节点池数据
	Get() ([]byte, error)
}

// LocaleStore 国际化存储接口
type LocaleStore interface {
	// Save 保存语言包
	Save(lang string, data []byte) error
	// Get 获取语言包
	Get(lang string) ([]byte, error)
	// List 列出所有语言
	List() ([]string, error)
}

// StoreProvider 存储提供者接口，用于创建和获取各类 Store 实例。
// 默认实现为 filestore.FileStoreProvider（基于文件的存储）。
// 用户可通过 app.WithStoreProvider 注入自定义实现（如数据库存储）。
type StoreProvider interface {
	// GetRuleStore 获取指定用户的规则链存储（per-user，带缓存）
	GetRuleStore(username string) (RuleStore, error)
	// GetSettingStore 获取指定用户的设置存储（per-user，带缓存）
	GetSettingStore(username string) (SettingStore, error)
	// GetComponentStore 获取指定用户的组件存储（per-user，带缓存）
	GetComponentStore(username string) (ComponentStore, error)
	// GetNodePoolStore 获取指定用户的节点池存储（per-user，带缓存）
	GetNodePoolStore(username string) (NodePoolStore, error)
	// GetUserStore 获取用户存储（全局单例）
	GetUserStore() (UserStore, error)
	// GetRunLogStore 获取运行日志存储（全局单例）
	GetRunLogStore() (RunLogStore, error)
}
