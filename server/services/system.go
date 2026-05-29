package services

import (
	"github.com/rulego/rulego/server/config"
)

// ConfigService 配置管理服务接口
type ConfigService interface {
	GetConfig() (*config.Config, error)
	UpdateConfig(configMap map[string]interface{}) error
}
