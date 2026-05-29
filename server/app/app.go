// Package app 提供应用生命周期管理、模块系统和轻量级服务容器。
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
	"github.com/rulego/rulego/server/internal/store/filestore"
	srvLogger "github.com/rulego/rulego/server/internal/logger"
)

// App 核心应用结构体，管理完整生命周期
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

// New 创建应用实例，只构造对象和应用选项，不做 I/O
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

// AddHook 注册生命周期钩子，必须在 Init 之前调用
// AddHook registers a lifecycle hook, must be called before Init
func (a *App) AddHook(hook Hook) {
	a.hooks.Add(hook)
}

// Container 返回应用的服务容器
// Container returns the application service container
func (a *App) Container() *Container {
	return a.container
}

// Config 返回应用配置
// Config returns the application config
func (a *App) Config() *config.Config {
	return a.config
}

// Logger 返回应用日志器
// Logger returns the application logger
func (a *App) Logger() types.Logger {
	return a.typesLog
}

// Init 初始化应用：加载配置、注册核心服务、初始化 runtime、按优先级初始化模块
// Init initializes the application: loads config, registers core services, initializes runtime, then initializes modules by priority
func (a *App) Init() error {
	// 优先使用注入的 logger；加载配置前需要一个临时 logger
	if a.opts.TypesLogger != nil {
		a.typesLog = a.opts.TypesLogger
	} else {
		a.typesLog = types.DefaultLogger()
	}

	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 如果未注入自定义 logger，则根据配置创建（支持文件输出和日志轮转）
	if a.opts.TypesLogger == nil && a.config != nil {
		a.typesLog = srvLogger.NewFromConfig(a.config)
	}

	a.checkSecurity()

	a.registerCoreServices()

	// 自动创建数据目录（可在嵌入模式下通过 WithoutAutoMkdir() 禁用）
	if a.opts.AutoMkdir && a.config != nil {
		a.ensureDataDirs()
	}

	// 如果用户通过 WithStoreProvider 注入了自定义 Provider，提前注册到容器
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

	// 应用模块覆盖：按 Name() 匹配替换
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

// Start 启动应用：启动传输层和后台任务，再启动模块
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

// Stop 按逆序关闭模块和传输层，带有 15 秒超时保护
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
		// 关闭存储提供者（释放 BBolt 等资源）
		if provider, err := GetAs[*filestore.FileStoreProvider](a.container, "store.provider"); err == nil {
			provider.Close()
		}

	a.started = false
	return firstErr
}

// Reload 动态重载应用：停止模块、重新加载配置、重新初始化并启动所有模块。
// 用于嵌入模式下修改配置或规则链后热更新，无需重启进程。
// Reload dynamically reloads the application: stops modules, reloads config, re-initializes and starts all modules.
// Used for hot-reloading config or rule chains in embedded mode without restarting the process.
func (a *App) Reload() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.typesLog.Infof("[reload] starting application reload...")

	// 1. 停止模块（如果正在运行）
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

	// 2. 重置容器和模块列表
	a.container = NewContainer()
	a.hooks = newHookManager()
	a.modules = append([]Module{}, a.opts.Modules...)

	// 3. 重新加载配置
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("reload: %w", err)
	}

	// 4. 根据新配置更新 logger
	if a.opts.TypesLogger == nil && a.config != nil {
		a.typesLog = srvLogger.NewFromConfig(a.config)
	}

	// 5. 重新注册核心服务
	a.registerCoreServices()
	// 5.5 重新注册钩子
	for _, h := range a.opts.Hooks {
		a.hooks.Add(h)
	}

	// 6. 重新初始化模块
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

	// 7. 启动模块
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

// applyModuleOverrides 按 Name() 匹配，用 ModuleOverrides 中的模块替换 Modules 中的同名模块。
// 如果 ModuleOverrides 中的模块没有匹配到任何已注册模块，返回错误。
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

// newModuleContext 构造当前应用状态的模块上下文
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

// Run 独立服务器模式：等价于 Init() + Start() + 等待系统信号 + Stop()。
// 会阻塞当前 goroutine 直到收到 SIGINT/SIGTERM，仅用于独立服务器 main 函数。
// 嵌入模式请使用 Init() + Start()，自行管理生命周期。
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

// loadConfig 加载配置文件。
// 查找策略：
//   1. 如果指定了 -c 且文件存在 → 从文件加载
//   2. 如果未指定 -c，自动查找当前目录的 config.conf → 找到则加载
//   3. 以上都不满足 → 使用 DefaultConfig
func (a *App) loadConfig() error {
	configFile := a.opts.ConfigFile

	// 自动查找：如果未显式指定配置文件，尝试在当前目录查找 config.conf
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

	// 合并注入的 Global 配置（注入值覆盖文件值）
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

// registerCoreServices 注册核心服务到容器
// registerCoreServices registers core services into the container
func (a *App) registerCoreServices() {
	a.container.Replace("core.container", a.container)
	if a.config != nil {
		a.container.Replace("core.config", a.config)
	}
	a.container.Replace("core.logger", a.typesLog)
}

// ensureDataDirs 创建数据目录结构
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

// defaultJwtSecretKey 默认 JWT 密钥，用于安全检查
const defaultJwtSecretKey = "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1"

// defaultAdminPassword 默认管理员密码
const defaultAdminPassword = "admin"

// checkSecurity 检查安全相关配置，对不安全的默认值打印告警
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

// splitPasswordAndApiKey 分割 "password,apiKey" 格式
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
