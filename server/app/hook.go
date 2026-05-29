package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/rulego/rulego/api/types"
)

// HookPhase 生命周期钩子阶段
// HookPhase represents the lifecycle hook phase
type HookPhase int

const (
	// BeforeInit 在所有模块 Init 之前执行
	// BeforeInit executes before all modules' Init phase
	BeforeInit HookPhase = iota
	// AfterInit 在所有模块 Init 之后执行
	// AfterInit executes after all modules' Init phase
	AfterInit
	// BeforeStart 在所有模块 Start 之前执行
	// BeforeStart executes before all modules' Start phase
	BeforeStart
	// AfterStart 在所有模块 Start 之后执行
	// AfterStart executes after all modules' Start phase
	AfterStart
	// OnStop 在 App 停止时执行
	// OnStop executes when the App is stopping
	OnStop
)

// String 返回阶段名称
// String returns the phase name
func (p HookPhase) String() string {
	switch p {
	case BeforeInit:
		return "before_init"
	case AfterInit:
		return "after_init"
	case BeforeStart:
		return "before_start"
	case AfterStart:
		return "after_start"
	case OnStop:
		return "on_stop"
	default:
		return "unknown"
	}
}

// Hook 生命周期钩子接口
// Hook defines the lifecycle hook interface
type Hook interface {
	// Name 返回钩子名称
	// Name returns the hook name
	Name() string
	// Phase 返回钩子执行阶段
	// Phase returns the hook execution phase
	Phase() HookPhase
	// Priority 返回钩子优先级，数值越小越先执行
	// Priority returns the hook priority, lower values execute first
	Priority() int
	// Execute 执行钩子逻辑
	// Execute runs the hook logic
	Execute(ctx context.Context, appCtx *ModuleContext) error
}

// FuncHook 基于函数的简单钩子实现
// FuncHook is a simple function-based hook implementation
type FuncHook struct {
	name     string
	phase    HookPhase
	priority int
	fn       func(ctx context.Context, appCtx *ModuleContext) error
}

// NewFuncHook 创建一个基于函数的钩子
// NewFuncHook creates a function-based hook
func NewFuncHook(name string, phase HookPhase, priority int, fn func(ctx context.Context, appCtx *ModuleContext) error) *FuncHook {
	return &FuncHook{
		name:     name,
		phase:    phase,
		priority: priority,
		fn:       fn,
	}
}

func (h *FuncHook) Name() string     { return h.name }
func (h *FuncHook) Phase() HookPhase { return h.phase }
func (h *FuncHook) Priority() int    { return h.priority }
func (h *FuncHook) Execute(ctx context.Context, appCtx *ModuleContext) error {
	return h.fn(ctx, appCtx)
}

// hookManager 生命周期钩子管理器，按阶段和优先级管理钩子
// hookManager manages lifecycle hooks by phase and priority
type hookManager struct {
	hooks map[HookPhase][]Hook
}

// newHookManager 创建钩子管理器
// newHookManager creates a new hook manager
func newHookManager() *hookManager {
	return &hookManager{
		hooks: make(map[HookPhase][]Hook),
	}
}

// Add 添加一个钩子
// Add adds a hook
func (hm *hookManager) Add(hook Hook) {
	phase := hook.Phase()
	hm.hooks[phase] = append(hm.hooks[phase], hook)
}

// executePhase 执行指定阶段的所有钩子
// executePhase executes all hooks for the given phase
func (hm *hookManager) executePhase(ctx context.Context, phase HookPhase, appCtx *ModuleContext, logger types.Logger) error {
	hooks := hm.hooks[phase]
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority() < hooks[j].Priority()
	})
	for _, hook := range hooks {
		logger.Infof("[%s] executing hook: %s (priority=%d)", phase, hook.Name(), hook.Priority())
		if err := hook.Execute(ctx, appCtx); err != nil {
			return fmt.Errorf("hook %q[%s] failed: %w", hook.Name(), phase, err)
		}
	}
	return nil
}
