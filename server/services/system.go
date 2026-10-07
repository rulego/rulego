package services

import (
	"github.com/rulego/rulego/server/config"
)

// ConfigService 配置管理服务接口
type ConfigService interface {
	GetConfig() (*config.Config, error)
	// UpdateConfig 保存全局配置并热更新：推送新表到各引擎、精准重载受影响的链与共享节点
	UpdateConfig(configMap map[string]interface{}) (*GlobalReloadResult, error)
	// GlobalOverrides 返回 data/config.json 留存的运行时覆盖（原始写法，敏感键掩码）
	GlobalOverrides() (map[string]string, error)
	// RestoreGlobalKey 删除某键的运行时覆盖并恢复为 config.conf 文件值
	RestoreGlobalKey(key string) (*GlobalReloadResult, error)
}
