package system

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/utils/fs"
)

const (
	ModuleName = "system"
	Priority   = 65
)

// Module system 业务模块，负责系统配置的读写。
type Module struct {
	cfg *config.Config
}

// New 创建 system 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	if err := ctx.Container.Register(services.KeyConfigService, services.ConfigService(m)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

func (m *Module) GetConfig() (*config.Config, error) {
	return m.cfg, nil
}

func (m *Module) UpdateConfig(configMap map[string]interface{}) error {
	if len(configMap) == 0 {
		return errors.New("the data cannot be empty")
	}
	if err := fs.CreateDirs(m.cfg.DataDir); err != nil {
		return err
	}
	return m.saveFileData(m.cfg.DataDir, configMap, configMap)
}

func (m *Module) saveFileData(dataDir string, configMap map[string]interface{}, originalMap map[string]interface{}) error {
	filePath := filepath.Join(dataDir, "config.json")
	if err := fs.CreateDirs(dataDir); err != nil {
		return err
	}
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return m.writeFileData(filePath, originalMap)
	} else if err != nil {
		return err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var existingConfig map[string]interface{}
	if err := json.Unmarshal(data, &existingConfig); err != nil {
		return err
	}
	mergedConfig := m.mergeConfigs(existingConfig, configMap)
	return m.writeFileData(filePath, mergedConfig)
}

func (m *Module) mergeConfigs(existing, updates map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range existing {
		result[k] = v
	}
	for k, v := range updates {
		result[k] = v
	}
	return result
}

func (m *Module) writeFileData(filePath string, data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return fs.SaveFile(filePath, jsonData)
}

func (m *Module) getKeyFromJSON(data map[string]interface{}, key string) interface{} {
	keys := strings.Split(key, ".")
	current := data
	for i, k := range keys {
		if i == len(keys)-1 {
			if value, exists := current[k]; exists {
				return value
			}
			return nil
		}
		if next, exists := current[k]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return nil
}
