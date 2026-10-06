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
	cfg       *config.Config
	container *app.Container
}

// New 创建 system 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	// 引擎管理器经容器惰性获取：system 的初始化先于 rule，直接取拿不到
	m.container = ctx.Container
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

func (m *Module) UpdateConfig(configMap map[string]interface{}) (*services.GlobalReloadResult, error) {
	if len(configMap) == 0 {
		return nil, errors.New("the data cannot be empty")
	}
	if err := fs.CreateDirs(m.cfg.DataDir); err != nil {
		return nil, err
	}
	var deletes []string
	upserts := make(map[string]interface{}, len(configMap))
	for k, v := range configMap {
		newVal := fmt.Sprintf("%v", v)
		switch {
		case v == nil:
			// 值为 null 表示删除该键
			deletes = append(deletes, k)
		case newVal == MaskedValue:
			// 掩码占位值不落库，防止前端把脱敏回显原样提交覆盖真值
		default:
			// 同值覆盖不进变更集，避免无谓的重载；键不存在时不比较（零值是 ""，
			// 直接比对会把新增空值键误判成同值吞掉）
			if old, exists := m.cfg.Global[k]; exists && old == newVal {
				continue
			}
			upserts[k] = v
		}
	}
	if len(upserts) == 0 && len(deletes) == 0 {
		// 掩码占位与同值覆盖都被吸收：无实际变更，按幂等成功处理
		return nil, nil
	}
	// COW 整表替换：运行中的引擎/JS 沙箱仍持旧 map 引用，只读不写；生效路径
	// 走下方热更新推送（各引擎换新 map + 重载受影响链）
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
	if err := m.saveFileData(m.cfg.DataDir, upserts, deletes); err != nil {
		return nil, err
	}
	// 热更新推送：引擎未注册时跳过，不影响保存
	changedKeys := make([]string, 0, len(upserts)+len(deletes))
	for k := range upserts {
		changedKeys = append(changedKeys, k)
	}
	changedKeys = append(changedKeys, deletes...)
	var reload *services.GlobalReloadResult
	if m.container != nil {
		if svc, ok := m.container.Get(services.KeyEngineManager); ok {
			if em, ok := svc.(services.EngineManager); ok {
				r := em.PropagateGlobal(map[string]string(newGlobal), changedKeys)
				reload = &r
			}
		}
	}
	return reload, nil
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
