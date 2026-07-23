package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/rulego/rulego/api/types"
)

// HookPhase Lifecycle
// HookPhase represents the lifecycle hook phase
type HookPhase int

const (
	// BeforeInit is executed before all module Inits
	// BeforeInit executes before all modules' Init phase
	BeforeInit HookPhase = iota
	// AfterInit executes after all module Inits
	// AfterInit executes after all modules' Init phase
	AfterInit
	// BeforeStart is executed before all modules start
	// BeforeStart executes before all modules' Start phase
	BeforeStart
	// AfterStart is executed after all modules start
	// AfterStart executes after all modules' Start phase
	AfterStart
	// OnStop is executed when the app stops
	// OnStop executes when the App is stopping
	OnStop
)

// String returns the stage name
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

// Hook lifecycle hook interface
// Hook defines the lifecycle hook interface
type Hook interface {
	// Name returns the hook name
	// Name returns the hook name
	Name() string
	// Phase: Returns the hook to execute the phase
	// Phase returns the hook execution phase
	Phase() HookPhase
	// Priority: Returns the hook priority; the smaller the value, the earlier it is executed
	// Priority returns the hook priority, lower values execute first
	Priority() int
	// Execute executes hook logic
	// Execute runs the hook logic
	Execute(ctx context.Context, appCtx *ModuleContext) error
}

// FuncHook is a simple hook implementation based on functions
// FuncHook is a simple function-based hook implementation
type FuncHook struct {
	name     string
	phase    HookPhase
	priority int
	fn       func(ctx context.Context, appCtx *ModuleContext) error
}

// NewFuncHook creates a function-based hook
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

// hookManager Lifecycle Hook Manager manages hooks by stage and priority
// hookManager manages lifecycle hooks by phase and priority
type hookManager struct {
	hooks map[HookPhase][]Hook
}

// newHookManager creates a hook manager
// newHookManager creates a new hook manager
func newHookManager() *hookManager {
	return &hookManager{
		hooks: make(map[HookPhase][]Hook),
	}
}

// Add a hook
// Add adds a hook
func (hm *hookManager) Add(hook Hook) {
	phase := hook.Phase()
	hm.hooks[phase] = append(hm.hooks[phase], hook)
}

// executePhase executes all hooks for the specified phase
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
