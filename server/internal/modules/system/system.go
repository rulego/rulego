package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rulego/rulego/api/types"
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

// MaskedValue GET 脱敏返回的占位值；POST 收到该值时视为「保持原值」不落库
const MaskedValue = "******"

var sensitiveKeyWords = []string{"key", "secret", "token", "password", "passwd", "auth", "credential"}

// IsSensitiveKey 按键名命名约定判定敏感（global 常用于存放密钥类配置）
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, w := range sensitiveKeyWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

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
	var deletes []string
	upserts := make(map[string]interface{}, len(configMap))
	for k, v := range configMap {
		switch {
		case v == nil:
			// 值为 null 表示删除该键
			deletes = append(deletes, k)
		case fmt.Sprintf("%v", v) == MaskedValue:
			// 掩码占位值不落库，防止前端把脱敏回显原样提交覆盖真值
		default:
			upserts[k] = v
		}
	}
	if len(upserts) == 0 && len(deletes) == 0 {
		return errors.New("no effective changes")
	}
	// COW 整表替换：已存在引擎在创建时各自拷贝了 Properties，本更新只影响
	// 后续新建引擎与重启后的进程，不触碰运行中的 map（并发写会 panic）
	newGlobal := make(types.Properties, len(m.cfg.Global)+len(upserts))
	for k, v := range m.cfg.Global {
		newGlobal[k] = v
	}
	for k := range deletes {
		delete(newGlobal, deletes[k])
	}
	for k, v := range upserts {
		newGlobal[k] = fmt.Sprintf("%v", v)
	}
	m.cfg.Global = newGlobal
	return m.saveFileData(m.cfg.DataDir, upserts, deletes)
}

func (m *Module) saveFileData(dataDir string, upserts map[string]interface{}, deletes []string) error {
	filePath := filepath.Join(dataDir, "config.json")
	if err := fs.CreateDirs(dataDir); err != nil {
		return err
	}
	var mergedConfig map[string]interface{}
	data, err := os.ReadFile(filePath)
	if err == nil {
		if err := json.Unmarshal(data, &mergedConfig); err != nil {
			// 损坏文件不能当空文件覆写，否则既有运行时键全部丢失
			return fmt.Errorf("parse %s: %w", filePath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if mergedConfig == nil {
		mergedConfig = map[string]interface{}{}
	}
	for k, v := range upserts {
		mergedConfig[k] = v
	}
	for _, k := range deletes {
		delete(mergedConfig, k)
	}
	return m.writeFileData(filePath, mergedConfig)
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
