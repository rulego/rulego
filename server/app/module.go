package app

import (
	"context"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
)

// Module 定义模块接口，所有业务模块需要实现此接口
// Module defines the module interface that all business modules must implement
type Module interface {
	// Name 返回模块名称
	// Name returns the module name
	Name() string

	// Priority 返回模块优先级，数值越小越先初始化
	// Priority returns the module priority, lower values initialize first
	Priority() int

	// Init 初始化模块，在此方法中注册导出服务到容器
	// Init initializes the module, export services to container in this method
	Init(ctx *ModuleContext) error

	// Start 启动模块，在此方法中启动后台任务
	// Start starts the module, launch background tasks in this method
	Start(ctx context.Context) error

	// Stop 停止模块，在此方法中释放资源
	// Stop stops the module, release resources in this method
	Stop(ctx context.Context) error
}

// ModuleContext 模块初始化上下文，包含容器、配置和日志器
// ModuleContext provides the context for module initialization, including container, config, and logger
type ModuleContext struct {
	Container *Container
	Config    *config.Config
	Logger    types.Logger
	DataDir   string
}

// modulesByPriority 实现 sort.Interface，按 Priority 升序排列
// modulesByPriority implements sort.Interface, sorting by Priority in ascending order
type modulesByPriority []Module

func (m modulesByPriority) Len() int      { return len(m) }
func (m modulesByPriority) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m modulesByPriority) Less(i, j int) bool {
	pi := m[i].Priority()
	pj := m[j].Priority()
	if pi == pj {
		return i < j
	}
	return pi < pj
}
