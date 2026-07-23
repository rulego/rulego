package app

import (
	"context"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
)

// Module defines the module interface, which all business modules need to implement
// Module defines the module interface that all business modules must implement
type Module interface {
	// Name returns the module name
	// Name returns the module name
	Name() string

	// Priority returns the module's priority; the smaller the value, the earlier it is initialized
	// Priority returns the module priority, lower values initialize first
	Priority() int

	// Init initializes the module, registering the export of services to containers in this method
	// Init initializes the module, export services to container in this method
	Init(ctx *ModuleContext) error

	// Start the module, which starts background tasks in this method
	// Start starts the module, launch background tasks in this method
	Start(ctx context.Context) error

	// Stop the module releases resources in this method
	// Stop stops the module, release resources in this method
	Stop(ctx context.Context) error
}

// The ModuleContext module initializes the context and includes containers, configurations, and loggers
// ModuleContext provides the context for module initialization, including container, config, and logger
type ModuleContext struct {
	Container *Container
	Config    *config.Config
	Logger    types.Logger
	DataDir   string
}

// modulesByPriority implements sort.Interface, arranged in ascending order of Priority
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
