// Package iotpoint 提供 IoT 采集点位模板的管理（CRUD + 内置模板）。
// 模板格式与协议无关：points 为统一的 iot_points.Point 结构（以 map 存储），
// 本模块不 import rulego-components-iot，因此在未开启 with_iot build tag 时也能编译。
package iotpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
)

const (
	ModuleName = "iotpoint"
	Priority   = 75
)

// PointTemplate 点位模板（协议无关）。
// Points 为统一 iot_points.Point 列表（以 map 存储，避免引入 iot 依赖）：
// 每个点位含 name/addr/type/scale/offset/endian 等，addr 按各协议格式（Modicon/NodeID/OID/DI）。
type PointTemplate struct {
	Id          string                   `json:"id"`
	Name        string                   `json:"name"`
	Protocol    string                   `json:"protocol"`           // modbus/opcua/s7/eip/snmp/dlt645/...
	Category    string                   `json:"category,omitempty"` // 分类（电力仪表/温湿度/...）
	Vendor      string                   `json:"vendor,omitempty"`   // 设备厂商
	Description string                   `json:"description,omitempty"`
	BuiltIn     bool                     `json:"builtIn,omitempty"` // 内置模板不可删
	Points      []map[string]interface{} `json:"points"`
}

// Module iotpoint 业务模块，负责点位模板的增删改查与内置模板初始化。
type Module struct {
	cfg *config.Config
	dir string
	mu  sync.RWMutex
}

// New 创建 iotpoint 模块
func New() *Module { return &Module{} }

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.dir = filepath.Join(ctx.Config.DataDir, "iot", "point-templates")
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	// 初始化内置模板（仅当对应文件不存在时写入，不覆盖用户修改）
	for _, tpl := range builtinTemplates() {
		if _, err := m.read(tpl.Id); err != nil {
			_ = m.write(tpl)
		}
	}
	return ctx.Container.Register(services.KeyIoTPointService, m)
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

// List 列出模板，可按协议/分类筛选。
func (m *Module) List(protocol, category string) []PointTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PointTemplate, 0)
	entries, _ := os.ReadDir(m.dir)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		var tpl PointTemplate
		if err := json.Unmarshal(b, &tpl); err != nil {
			continue
		}
		if protocol != "" && tpl.Protocol != protocol {
			continue
		}
		if category != "" && tpl.Category != category {
			continue
		}
		out = append(out, tpl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

// Get 获取单个模板。
func (m *Module) Get(id string) (PointTemplate, error) {
	if !validId(id) {
		return PointTemplate{}, errors.New("invalid template id")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.read(id)
}

// Create 创建模板（id/name 必填，已存在则报错）。
func (m *Module) Create(tpl PointTemplate) error {
	if !validId(tpl.Id) {
		return errors.New("invalid template id")
	}
	if tpl.Name == "" {
		return errors.New("template name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.read(tpl.Id); err == nil {
		return errors.New("template already exists: " + tpl.Id)
	}
	tpl.BuiltIn = false
	return m.write(tpl)
}

// Update 更新模板（保持原有 builtIn 标记）。
func (m *Module) Update(id string, tpl PointTemplate) error {
	if !validId(id) {
		return errors.New("invalid template id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	old, err := m.read(id)
	if err != nil {
		return err
	}
	tpl.Id = id
	tpl.BuiltIn = old.BuiltIn
	return m.write(tpl)
}

// Delete 删除模板（内置模板不可删）。
func (m *Module) Delete(id string) error {
	if !validId(id) {
		return errors.New("invalid template id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tpl, err := m.read(id)
	if err != nil {
		return err
	}
	if tpl.BuiltIn {
		return errors.New("builtin template cannot be deleted")
	}
	return os.Remove(m.filePath(id))
}

// read 读取模板文件（调用方需持锁）。
func (m *Module) read(id string) (PointTemplate, error) {
	b, err := os.ReadFile(m.filePath(id))
	if err != nil {
		return PointTemplate{}, err
	}
	var tpl PointTemplate
	if err := json.Unmarshal(b, &tpl); err != nil {
		return PointTemplate{}, err
	}
	return tpl, nil
}

// write 写入模板文件（调用方需持锁，或在 Init 单线程阶段调用）。
func (m *Module) write(tpl PointTemplate) error {
	b, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath(tpl.Id), b, 0o644)
}

func (m *Module) filePath(id string) string {
	return filepath.Join(m.dir, id+".json")
}

// validId 校验模板 id（防路径遍历），仅允许字母数字 - _ .
func validId(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// builtinTemplates 内置点位模板（常见设备，不可删除，可被用户复制后修改）。
func builtinTemplates() []PointTemplate {
	return []PointTemplate{
		{
			Id:          "builtin-modbus-3phase-meter",
			Name:        "三相智能电表",
			Protocol:    "modbus",
			Category:    "电力仪表",
			Description: "三相电表常用点位（Modicon 保持寄存器地址，按实际设备调整）",
			BuiltIn:     true,
			Points: []map[string]interface{}{
				{"name": "A相电压", "addr": "40001", "type": "FLOAT32", "scale": 0.1},
				{"name": "B相电压", "addr": "40003", "type": "FLOAT32", "scale": 0.1},
				{"name": "C相电压", "addr": "40005", "type": "FLOAT32", "scale": 0.1},
				{"name": "A相电流", "addr": "40007", "type": "FLOAT32", "scale": 0.001},
				{"name": "B相电流", "addr": "40009", "type": "FLOAT32", "scale": 0.001},
				{"name": "C相电流", "addr": "40011", "type": "FLOAT32", "scale": 0.001},
				{"name": "总有功功率", "addr": "40013", "type": "FLOAT32", "scale": 0.1},
				{"name": "正向有功总电能", "addr": "40100", "type": "FLOAT64", "scale": 0.01},
			},
		},
		{
			Id:          "builtin-modbus-th02",
			Name:        "温湿度传感器",
			Protocol:    "modbus",
			Category:    "环境",
			Description: "温湿度变送器（Modicon 保持寄存器，scale 按实际量程调整）",
			BuiltIn:     true,
			Points: []map[string]interface{}{
				{"name": "温度", "addr": "40001", "type": "INT16", "scale": 0.1},
				{"name": "湿度", "addr": "40002", "type": "INT16", "scale": 0.1},
			},
		},
		{
			Id:          "builtin-opcua-simulation",
			Name:        "OPC UA 仿真服务器",
			Protocol:    "opcua",
			Category:    "仿真",
			Description: "OPC UA Simulation Server 示例点位（NodeID 地址）",
			BuiltIn:     true,
			Points: []map[string]interface{}{
				{"name": "counter", "addr": "ns=3;i=1001", "type": "UINT32"},
				{"name": "sinusoid", "addr": "ns=3;i=1002", "type": "FLOAT64"},
			},
		},
	}
}
