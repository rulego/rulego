package app

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/store"
)

// Option Apply the configuration option function type
// Option is the application configuration option function type
type Option func(*Options)

// Options app configuration collection of options
// Options contains all application configuration options
type Options struct {
	// ConfigFile configuration file path
	// ConfigFile is the path to the configuration file
	ConfigFile string

	// Config Programmable configuration. When not nil, use it over ConfigFile and default configuration.
	// Config is a programmatic config. When non-nil, takes precedence over ConfigFile and defaults.
	Config *config.Config

	// TypesLogger logger, implementing types.Logger interface
	// Can be used to inject application-layer logging frameworks (such as Zap, Logrus, etc.)
	// TypesLogger is the types.Logger instance for the application
	// Can be used to inject application-level logging frameworks (e.g., Zap, Logrus)
	TypesLogger types.Logger

	// Modules: The list of modules that need to be loaded
	// Modules is the list of modules to load
	Modules []Module

	// ModuleOverrides Overrides by name to add modules using WithModuleOverride
	// ModuleOverrides is the list of modules to override by name, added via WithModuleOverride
	ModuleOverrides []Module

	// Hooks lifecycle hook list
	// Hooks is the list of lifecycle hooks
	Hooks []Hook

	// DisableTransport Disable the default transport layer (for embedded mode)
	// DisableTransport disables the default transport layer (for embedded mode)
	DisableTransport bool

	// Global custom global configuration, injection in embed mode, merged with configuration file [global] (injection value overwrites file value)
	// Global is custom global config for embedded mode, merged with [global] section from config file (injected values override file values)
	Global types.Properties

	// StoreProvider is a custom storage provider used to inject into databases and other custom storage implementations
	// StoreProvider is a custom store provider for injecting custom storage implementations (e.g., database-backed)
	StoreProvider store.StoreProvider

	// Does AutoMkdir automatically create data directories when Init (default true).
	// In embedded mode, it can be disabled via WithoutAutoMkdir().
	// AutoMkdir controls whether Init auto-creates data directories (default true).
	// Disable with WithoutAutoMkdir() in embedded mode.
	AutoMkdir bool
}

// DefaultOptions returns the default configuration options
// DefaultOptions returns the default configuration options
func DefaultOptions() Options {
	return Options{
		AutoMkdir: true,
	}
}

// WithConfig sets programmatic configuration, taking precedence over ConfigFile and default configuration.
// Suitable for embedding mode: host directly constructs config.Config injection, without the need to write configuration files.
// Note: The incoming Config will be merged and processed by InitUserMap/Global; The Global option value will still override Config.Global.
func WithConfig(cfg *config.Config) Option {
	return func(o *Options) {
		o.Config = cfg
	}
}

// WithConfigFile sets the configuration file path
// WithConfigFile sets the configuration file path
func WithConfigFile(path string) Option {
	return func(o *Options) {
		o.ConfigFile = path
	}
}

// WithTypesLogger sets up a logger to implement types.Logger interface
// Can be used to interface with application-layer logging frameworks (such as Zap, Logrus, etc.)
// WithTypesLogger sets the types.Logger for the application
// Use this to integrate with application-level logging frameworks (e.g., Zap, Logrus)
func WithTypesLogger(l types.Logger) Option {
	return func(o *Options) {
		o.TypesLogger = l
	}
}

// WithModules sets the modules that need to be loaded
// WithModules sets the modules to load
func WithModules(modules ...Module) Option {
	return func(o *Options) {
		o.Modules = append(o.Modules, modules...)
	}
}

// WithHooks sets lifecycle hooks
// WithHooks sets the lifecycle hooks
func WithHooks(hooks ...Hook) Option {
	return func(o *Options) {
		o.Hooks = append(o.Hooks, hooks...)
	}
}

// WithGlobal injects custom global configuration, merging with the configuration file [global] (injected value overwrites file value)
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

// WithTransportDisabled Disable the default transport layer (embedded mode)
// WithTransportDisabled disables the default transport layer (embedded mode)
func WithTransportDisabled() Option {
	return func(o *Options) {
		o.DisableTransport = true
	}
}

// WithModuleOverride Overrides registered modules by name.
// In the Init phase, if the module Name() in ModuleOverrides matches a module in ModuleOverrides,
// then replace the latter with the former; If no match is found, Init will return an error.
//
// Usage:
//
//	application := app.New(
//	    app.WithConfigFile("config.conf"),
//	    app.WithModules(bootstrap.DefaultModules()...),
//	    app.WithModuleOverride(&MyRuleModule{}), // Modules that override Name() == "rule"
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

// WithStoreProvider sets up a custom storage provider, allowing users to inject into databases and other custom storage implementations.
// If not set, file-based storage is used by default.
//
// Usage:
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

// WithoutAutoMkdir automatically creates a data directory when Init is disabled.
// Used in embedded mode, called when the host system manages directory structures itself.
// WithoutAutoMkdir disables auto-creation of data directories during Init.
// Use in embedded mode when the host manages its own directory structure.
func WithoutAutoMkdir() Option {
	return func(o *Options) {
		o.AutoMkdir = false
	}
}
