// Package app provides application lifecycle management, modular systems, and lightweight service containers.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	srvLogger "github.com/rulego/rulego/server/internal/logger"
	"github.com/rulego/rulego/server/internal/store/filestore"
)

// The core application structure of the app manages the entire lifecycle
// App is the core application struct managing the full lifecycle
type App struct {
	container *Container
	modules   []Module
	hooks     *hookManager
	config    *config.Config
	typesLog  types.Logger
	opts      Options
	started   bool
	mu        sync.Mutex
}

// New creates an application instance, only constructing objects and application options, without doing I/O
// New creates an application instance, only sets up options without any I/O
func New(opts ...Option) *App {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &App{
		container: NewContainer(),
		hooks:     newHookManager(),
		opts:      o,
		modules:   o.Modules,
	}
}

// AddHook registers lifecycle hooks and must be called before Init
// AddHook registers a lifecycle hook, must be called before Init
func (a *App) AddHook(hook Hook) {
	a.hooks.Add(hook)
}

// Container: Returns the application's service container
// Container returns the application service container
func (a *App) Container() *Container {
	return a.container
}

// Config returns the application configuration
// Config returns the application config
func (a *App) Config() *config.Config {
	return a.config
}

// Logger returns the application logger
// Logger returns the application logger
func (a *App) Logger() types.Logger {
	return a.typesLog
}

// Init initialization application: load configuration, register core services, initialize runtime, and initialize modules by priority
// Init initializes the application: loads config, registers core services, initializes runtime, then initializes modules by priority
func (a *App) Init() error {
	// Prioritize using the injected logger; A temporary logger is required before loading the configuration
	if a.opts.TypesLogger != nil {
		a.typesLog = a.opts.TypesLogger
	} else {
		a.typesLog = types.DefaultLogger()
	}

	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// If a custom logger is not injected, it is created according to the configuration (supports file output and log rotation).
	if a.opts.TypesLogger == nil && a.config != nil {
		a.typesLog = srvLogger.NewFromConfig(a.config)
	}

	a.checkSecurity()

	a.registerCoreServices()

	// Automatically creates data directories (can be disabled in embedded mode via WithoutAutoMkdir())
	if a.opts.AutoMkdir && a.config != nil {
		a.ensureDataDirs()
	}

	// If a user injects a custom Provider via WithStoreProvider, they can pre-register to the container
	if a.opts.StoreProvider != nil {
		if err := a.container.Register("store.provider", a.opts.StoreProvider); err != nil {
			return fmt.Errorf("register store provider: %w", err)
		}
	}

	appCtx := &ModuleContext{
		Container: a.container,
		Config:    a.config,
		Logger:    a.typesLog,
	}
	if a.config != nil {
		appCtx.DataDir = a.config.DataDir
	}

	for _, h := range a.opts.Hooks {
		a.hooks.Add(h)
	}

	if err := a.hooks.executePhase(context.Background(), BeforeInit, appCtx, a.typesLog); err != nil {
		return err
	}

	// Application module coverage: Replace by name().
	if err := a.applyModuleOverrides(); err != nil {
		return err
	}

	sort.Sort(modulesByPriority(a.modules))

	for _, m := range a.modules {
		a.typesLog.Infof("[init] initializing module: %s (priority=%d)", m.Name(), m.Priority())
		if err := m.Init(appCtx); err != nil {
			return fmt.Errorf("init module %q: %w", m.Name(), err)
		}
	}

	if err := a.hooks.executePhase(context.Background(), AfterInit, appCtx, a.typesLog); err != nil {
		return err
	}

	return nil
}

// Start the application: Start the transport layer and background tasks, then start the module
// Start starts the application: launches transport and background tasks, then starts modules
func (a *App) Start() error {
	ctx := context.Background()

	appCtx := &ModuleContext{
		Container: a.container,
		Config:    a.config,
		Logger:    a.typesLog,
	}
	if a.config != nil {
		appCtx.DataDir = a.config.DataDir
	}

	if err := a.hooks.executePhase(ctx, BeforeStart, appCtx, a.typesLog); err != nil {
		return err
	}

	for _, m := range a.modules {
		a.typesLog.Infof("[start] starting module: %s", m.Name())
		if err := m.Start(ctx); err != nil {
			return fmt.Errorf("start module %q: %w", m.Name(), err)
		}
	}

	if err := a.hooks.executePhase(ctx, AfterStart, appCtx, a.typesLog); err != nil {
		return err
	}

	a.started = true
	return nil
}

// Stop closes modules and transport layers in reverse order, with 15-second timeout protection
// Stop shuts down modules in reverse order and transport layer, with 15s timeout
func (a *App) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var firstErr error

	appCtx := &ModuleContext{
		Container: a.container,
		Config:    a.config,
		Logger:    a.typesLog,
	}
	if a.config != nil {
		appCtx.DataDir = a.config.DataDir
	}

	if err := a.hooks.executePhase(ctx, OnStop, appCtx, a.typesLog); err != nil && firstErr == nil {
		firstErr = err
	}

	for i := len(a.modules) - 1; i >= 0; i-- {
		m := a.modules[i]
		a.typesLog.Infof("[stop] stopping module: %s", m.Name())
		if err := m.Stop(ctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("stop module %q: %w", m.Name(), err)
			}
			a.typesLog.Errorf("[stop] error stopping module %s: %v", m.Name(), err)
		}
	}
	// Shutting down storage providers (releasing resources like BBolt)
	if provider, err := GetAs[*filestore.FileStoreProvider](a.container, "store.provider"); err == nil {
		provider.Close()
	}

	a.started = false
	return firstErr
}

// Reload Dynamic Reload of Applications: Stop modules, reload configurations, reinitialize, and start all modules.
// Used for hot updates after modifying configurations or rule chains in embedded mode, without restarting the process.
// Reload dynamically reloads the application: stops modules, reloads config, re-initializes and starts all modules.
// Used for hot-reloading config or rule chains in embedded mode without restarting the process.
func (a *App) Reload() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.typesLog.Infof("[reload] starting application reload...")

	// 1. Stop the module (if running)
	if a.started {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		appCtx := a.newModuleContext()

		_ = a.hooks.executePhase(ctx, OnStop, appCtx, a.typesLog)

		for i := len(a.modules) - 1; i >= 0; i-- {
			m := a.modules[i]
			a.typesLog.Infof("[reload] stopping module: %s", m.Name())
			if err := m.Stop(ctx); err != nil {
				a.typesLog.Errorf("[reload] error stopping module %s: %v", m.Name(), err)
			}
		}
		a.started = false
	}

	// 2. Reset the container and module list
	a.container = NewContainer()
	a.hooks = newHookManager()
	a.modules = append([]Module{}, a.opts.Modules...)

	// 3. Reload the configuration
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}

	// 4. Update the logger according to the new configuration
	if a.opts.TypesLogger == nil && a.config != nil {
		a.typesLog = srvLogger.NewFromConfig(a.config)
	}

	// 5. Re-register core services
	a.registerCoreServices()
	// 5.5 Re-registering the hook
	for _, h := range a.opts.Hooks {
		a.hooks.Add(h)
	}

	// 6. Reinitialize the module
	appCtx := a.newModuleContext()

	if err := a.hooks.executePhase(context.Background(), BeforeInit, appCtx, a.typesLog); err != nil {
		return err
	}

	sort.Sort(modulesByPriority(a.modules))
	for _, m := range a.modules {
		a.typesLog.Infof("[reload] initializing module: %s (priority=%d)", m.Name(), m.Priority())
		if err := m.Init(appCtx); err != nil {
			return fmt.Errorf("reload init module %q: %w", m.Name(), err)
		}
	}

	if err := a.hooks.executePhase(context.Background(), AfterInit, appCtx, a.typesLog); err != nil {
		return err
	}

	// 7. Startup module
	ctx := context.Background()
	if err := a.hooks.executePhase(ctx, BeforeStart, appCtx, a.typesLog); err != nil {
		return err
	}

	for _, m := range a.modules {
		a.typesLog.Infof("[reload] starting module: %s", m.Name())
		if err := m.Start(ctx); err != nil {
			return fmt.Errorf("reload start module %q: %w", m.Name(), err)
		}
	}

	if err := a.hooks.executePhase(ctx, AfterStart, appCtx, a.typesLog); err != nil {
		return err
	}

	a.started = true
	a.typesLog.Infof("[reload] application reloaded successfully")
	return nil
}

// applyModuleOverrides matches by Name(), replacing modules with the same name in Modules with modules in ModuleOverrides.
// If a module in ModuleOverrides does not match any registered module, an error is returned.
func (a *App) applyModuleOverrides() error {
	if len(a.opts.ModuleOverrides) == 0 {
		return nil
	}
	for _, override := range a.opts.ModuleOverrides {
		name := override.Name()
		found := false
		for i, m := range a.modules {
			if m.Name() == name {
				a.typesLog.Infof("[init] overriding module: %s", name)
				a.modules[i] = override
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("module override failed: no module named %q to override", name)
		}
	}
	return nil
}

// newModuleContext constructs the module context for the current application state
func (a *App) newModuleContext() *ModuleContext {
	appCtx := &ModuleContext{
		Container: a.container,
		Config:    a.config,
		Logger:    a.typesLog,
	}
	if a.config != nil {
		appCtx.DataDir = a.config.DataDir
	}
	return appCtx
}

// Run Independent Server Mode: Equivalent to Init() + Start() + Wait for System Signal + Stop().
// It blocks the current goroutine until SIGINT/SIGTERM is received, and is only used for the main function of the standalone server.
// For embedding mode, use Init() + Start() to manage the lifecycle yourself.
// Run is standalone server mode: Init() + Start() + wait for system signal + Stop().
// Blocks the current goroutine until SIGINT/SIGTERM. Only use in standalone server main.
// For embedded mode, use Init() + Start() and manage the lifecycle yourself.
func (a *App) Run() error {
	if err := a.Init(); err != nil {
		return err
	}
	if err := a.Start(); err != nil {
		return err
	}

	a.typesLog.Infof("[run] server started, waiting for signal...")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	a.typesLog.Infof("[run] received signal: %v, shutting down...", sig)

	return a.Stop()
}

// loadConfig loads the configuration file.
// Find Strategies:
//  1. If -c is specified and the file contains → load from the file
//  2. If -c is not specified, it will automatically search for config.conf in the current directory → load when found
//  3. If none of the above are met, use DefaultConfig →
func (a *App) loadConfig() error {
	// Programmatic configuration priority (direct injection by embedding mode host)
	if a.opts.Config != nil {
		a.config = a.opts.Config
		if a.config.Users == nil {
			a.config.Users = make(types.Properties)
		}
		a.config.InitUserMap()
		a.typesLog.Infof("[config] loaded from programmatic Config")
	} else {
		configFile := a.opts.ConfigFile

		// Auto Search: If the configuration file is not explicitly specified, try searching config.conf in the current directory
		if configFile == "" {
			if _, err := os.Stat("config.conf"); err == nil {
				configFile = "config.conf"
			}
		}

		if configFile == "" {
			cfg := config.DefaultConfig()
			a.config = &cfg
			cfg.InitUserMap()
		} else {
			cfg := config.DefaultConfig()
			if err := config.Load(configFile, &cfg); err != nil {
				return err
			}
			a.config = &cfg
			a.typesLog.Infof("[config] loaded from %s", configFile)
		}
	}

	// Merge the global configuration of the injection (the injection value overwrites the file value)
	if len(a.opts.Global) > 0 {
		if a.config.Global == nil {
			a.config.Global = make(types.Properties, len(a.opts.Global))
		}
		for k, v := range a.opts.Global {
			a.config.Global[k] = v
		}
	}

	return nil
}

// registerCoreServices Register core services into containers
// registerCoreServices registers core services into the container
func (a *App) registerCoreServices() {
	a.container.Replace("core.container", a.container)
	if a.config != nil {
		a.container.Replace("core.config", a.config)
	}
	a.container.Replace("core.logger", a.typesLog)
}

// ensureDataDirs creates a data directory structure
func (a *App) ensureDataDirs() {
	dataDir := a.config.DataDir
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "workflows"),
		filepath.Join(dataDir, "js"),
		filepath.Join(dataDir, "plugins"),
		filepath.Join(dataDir, "system", "agents"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			a.typesLog.Errorf("[init] failed to create directory %s: %v", dir, err)
		}
	}
}

// defaultJwtSecretKey is the default JWT key used for security checks
const defaultJwtSecretKey = "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1"

// defaultAdminPassword: The default administrator password
const defaultAdminPassword = "admin"

// checkSecurity: Checks security-related configurations and prints alerts for default values that are not secure
func (a *App) checkSecurity() {
	if a.config != nil && a.config.JwtSecretKey == defaultJwtSecretKey {
		a.typesLog.Warnf("[security] WARNING: using default JWT secret key, please change it in config for production use")
	}
	if a.config != nil {
		for username, passwordAndApiKey := range a.config.Users {
			parts := splitPasswordAndApiKey(passwordAndApiKey)
			if parts[0] == defaultAdminPassword {
				a.typesLog.Warnf("[security] WARNING: user %q has default password, please change it for production use", username)
			}
		}
	}
}

// splitPasswordAndApiKey splits the "password,apiKey" format
func splitPasswordAndApiKey(s string) []string {
	parts := make([]string, 2)
	for i, c := range s {
		if c == ',' {
			parts[0] = s[:i]
			if i+1 < len(s) {
				parts[1] = s[i+1:]
			}
			return parts
		}
	}
	parts[0] = s
	return parts
}
