package services

import (
	"github.com/rulego/rulego/server/config"
)

// ConfigService configuration management service interface
type ConfigService interface {
	GetConfig() (*config.Config, error)
	UpdateConfig(configMap map[string]interface{}) error
}
