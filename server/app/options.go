package app

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/store"
)

// Option 应用配置选项函数类型
// Option is the application configuration option function type
type Option func(*Options)

// Options 应用配置选项集合
// Options contains all application configuration options
type Options struct {
	// ConfigFile 配置文件路径
	// ConfigFile is the path to the configuration file
	ConfigFile string

	// TypesLogger 日志器，实现 types.Logger 接口
	// 可用于注入应用层日志框架（如 Zap、Logrus 等）
	// TypesLogger is the types.Logger instance for the application
	// Can be used to inject application-level logging frameworks (e.g., Zap, Logrus)
	TypesLogger types.Logger

	// Modules 需要加载的模块列表
	// Modules is the list of modules to load
	Modules []Module

	// ModuleOverrides 按名称覆盖模块列表，用 WithModuleOverride 添加
	// ModuleOverrides is the list of modules to override by name, added via WithModuleOverride
	ModuleOverrides []Module

	// Hooks 生命周期钩子列表
	// Hooks is the list of lifecycle hooks
	Hooks []Hook

	// DisableTransport 禁用默认传输层（用于嵌入式模式）
	// DisableTransport disables the default transport layer (for embedded mode)
	DisableTransport bool

	// Global 自定义全局配置，嵌入模式下注入，与配置文件 [global] 合并（注入值覆盖文件值）
	// Global is custom global config for embedded mode, merged with [global] section from config file (injected values override file values)
	Global types.Properties

	// StoreProvider 自定义存储提供者，用于注入数据库等自定义存储实现
	// StoreProvider is a custom store provider for injecting custom storage implementations (e.g., database-backed)
	StoreProvider store.StoreProvider

	// AutoMkdir 是否在 Init 时自动创建数据目录（默认 true）。
	// 嵌入模式下可通过 WithoutAutoMkdir() 禁用。
	// AutoMkdir controls whether Init auto-creates data directories (default true).
	// Disable with WithoutAutoMkdir() in embedded mode.
	AutoMkdir bool
}

// DefaultOptions 返回默认配置选项
// DefaultOptions returns the default configuration options
func DefaultOptions() Options {
	return Options{
		AutoMkdir: true,
	}
}

// WithConfigFile 设置配置文件路径
// WithConfigFile sets the configuration file path
func WithConfigFile(path string) Option {
	return func(o *Options) {
		o.ConfigFile = path
	}
}

// WithTypesLogger 设置日志器，实现 types.Logger 接口
// 可用于对接应用层日志框架（如 Zap、Logrus 等）
// WithTypesLogger sets the types.Logger for the application
// Use this to integrate with application-level logging frameworks (e.g., Zap, Logrus)
func WithTypesLogger(l types.Logger) Option {
	return func(o *Options) {
		o.TypesLogger = l
	}
}

// WithModules 设置需要加载的模块
// WithModules sets the modules to load
func WithModules(modules ...Module) Option {
	return func(o *Options) {
		o.Modules = append(o.Modules, modules...)
	}
}

// WithHooks 设置生命周期钩子
// WithHooks sets the lifecycle hooks
func WithHooks(hooks ...Hook) Option {
	return func(o *Options) {
		o.Hooks = append(o.Hooks, hooks...)
	}
}

// WithGlobal 注入自定义全局配置，与配置文件 [global] 合并（注入值覆盖文件值）
// WithGlobal injects custom global config, merged with [global] section from config file (injected values override file values)
func WithGlobal(global types.Properties) Option {
	return func(o *Options) {
		if o.Global == nil {
			o.Global = make(types.Properties, len(global))
		}
		for k, v := range global {
			o.Global[k] = v
		}
	}
}

// WithTransportDisabled 禁用默认传输层（嵌入式模式）
// WithTransportDisabled disables the default transport layer (embedded mode)
func WithTransportDisabled() Option {
	return func(o *Options) {
		o.DisableTransport = true
	}
}

// WithModuleOverride 按名称覆盖已注册的模块。
// 在 Init 阶段，如果 ModuleOverrides 中的模块 Name() 与 Modules 中的某个模块匹配，
// 则用前者替换后者；如果没有匹配到，Init 将返回错误。
//
// 用法：
//
//	application := app.New(
//	    app.WithConfigFile("config.conf"),
//	    app.WithModules(bootstrap.DefaultModules()...),
//	    app.WithModuleOverride(&MyRuleModule{}),  // 覆盖 Name() == "rule" 的模块
//	)
//
// WithModuleOverride overrides a registered module by name.
// During Init, if a module in ModuleOverrides has the same Name() as one in Modules,
// it replaces it. If no match is found, Init returns an error.
func WithModuleOverride(module Module) Option {
	return func(o *Options) {
		o.ModuleOverrides = append(o.ModuleOverrides, module)
	}
}

// WithStoreProvider 设置自定义存储提供者，允许用户注入数据库等自定义存储实现。
// 如果不设置，默认使用基于文件的存储实现。
//
// 用法：
//
//	application := app.New(
//	    app.WithConfigFile("config.conf"),
//	    app.WithStoreProvider(&MyDbStoreProvider{db: myDb}),
//	    app.WithModules(user.New(), rule.New(), node.New(), runlog.New()),
//	)
//
// WithStoreProvider sets a custom store provider, allowing users to inject
// database-backed or other custom store implementations.
// If not set, the default file-based store is used.
func WithStoreProvider(provider store.StoreProvider) Option {
	return func(o *Options) {
		o.StoreProvider = provider
	}
}

// WithoutAutoMkdir 禁用 Init 时自动创建数据目录。
// 嵌入模式下使用，当宿主系统自行管理目录结构时调用。
// WithoutAutoMkdir disables auto-creation of data directories during Init.
// Use in embedded mode when the host manages its own directory structure.
func WithoutAutoMkdir() Option {
	return func(o *Options) {
		o.AutoMkdir = false
	}
}

