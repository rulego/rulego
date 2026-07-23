package config

import (
	"strings"

	"github.com/rulego/rulego/api/types"
)

// Config application configuration, loaded via INI file
type Config struct {
	// ConfigFile configuration file path (not INI field)
	ConfigFile string `ini:"-"`
	// DataDir data directory
	DataDir string `ini:"data_dir"`
	// The LogFile log file path is empty; if it is empty, it is only output to the console
	LogFile string `ini:"log_file"`
	// LogLevel: debug/info/warn/error, default info
	LogLevel string `ini:"log_level"`
	// LogMaxSize is the maximum size of a single log file (MB), with a default of 100
	LogMaxSize int `ini:"log_max_size"`
	// LogMaxBackups can retain the maximum number of old log files, default 30
	LogMaxBackups int `ini:"log_max_backups"`
	// LogMaxAge retains the maximum number of days old log files are retained, default is 7
	LogMaxAge int `ini:"log_max_age"`
	// CmdWhiteList shell command: whitelist multiple units separated by commas
	CmdWhiteList string `ini:"cmd_white_list"`
	// CmdMode shell command safe mode: allow (whitelist mode) or deny (blacklist mode), default is allow
	CmdMode string `ini:"cmd_mode"`
	// CmdDenyList shell command blacklist, multiple separated by commas
	CmdDenyList string `ini:"cmd_deny_list"`
	// CmdDenyArgs rejects command parameter mode, multiple separated by commas
	CmdDenyArgs string `ini:"cmd_deny_args"`
	// FilePathWhiteList allows the file path whitelist to operate
	FilePathWhiteList string `ini:"file_path_white_list"`
	// LoadLuaLibs Whether to load the lua library
	LoadLuaLibs string `ini:"load_lua_libs"`
	// Server http server address
	Server string `ini:"server"`
	// The BasePath API route the base path prefix, such as /rulego. Used in embedded modes to avoid routing conflicts
	BasePath string `ini:"base_path"`
	// DefaultUsername The default username
	DefaultUsername string `ini:"default_username"`
	// Debug: Should node debug logs be printed to log files?
	Debug bool `ini:"debug"`
	// MaxNodeLogSize: The maximum node log size
	MaxNodeLogSize int `ini:"max_node_log_size"`
	// ResourceMapping Static File Path Mapping
	ResourceMapping string `ini:"resource_mapping"`
	// Global custom configuration
	Global types.Properties `ini:"global"`
	// NodePoolFile: Node pool file
	NodePoolFile string `ini:"node_pool_file"`
	// Does SaveRunLog save the runlog?
	SaveRunLog bool `ini:"save_run_log"`
	// RunLogStoreType Runlog storage type: bbolt (default) or file (JSON Lines)
	RunLogStoreType string `ini:"run_log_store_type"`
	// RunLogRetentionCount keeps the most recent N logs, and 0 means unlimited
	RunLogRetentionCount int `ini:"run_log_retention_count"`
	// RunLogRetentionDays keeps logs from the most recent N days; 0 means there is no limit
	RunLogRetentionDays int `ini:"run_log_retention_days"`
	// ScriptMaxExecutionTime Maximum script execution time (milliseconds)
	ScriptMaxExecutionTime int `ini:"script_max_execution_time"`
	// EndpointEnabled Whether to enable Endpoint
	EndpointEnabled *bool `ini:"endpoint_enabled"`
	// SecretKey key
	SecretKey *string `ini:"secret_key"`
	// EventBusChainId Core rule Chain Id
	EventBusChainId string `ini:"event_bus_chain_id"`
	// CategoryFolderEnabled Whether to organize the rule chain by category folder
	CategoryFolderEnabled *bool `ini:"category_folder_enabled"`
	// RequireAuth API access requires authentication
	RequireAuth bool `ini:"require_auth"`
	// JwtSecretKey jwt key
	JwtSecretKey string `ini:"jwt_secret_key"`
	// JwtExpireTime: jwt expiration time (milliseconds)
	JwtExpireTime int `ini:"jwt_expire_time"`
	// JwtIssuer jwt issuer
	JwtIssuer string `ini:"jwt_issuer"`
	// Users list
	Users types.Properties `ini:"users"`
	// Pprof pprof configuration
	Pprof PprofConfig `ini:"pprof"`
	// MarketplaceBaseUrl component market root address
	MarketplaceBaseUrl string `ini:"marketplace_base_url"`
	// Is ShareHttpServer set to a shared node by default for HTTP services?
	ShareHttpServer bool `ini:"share_http_server"`
	// Does AllowCors allow cross-origin? Default is true (backward compatible)
	AllowCors bool `ini:"allow_cors"`
	// ReadTimeout HTTP reads out (seconds), default is 30
	ReadTimeout int `ini:"read_timeout"`
	// WriteTimeout HTTP writes out (seconds), default 300 (AI chat requires longer timeout)
	WriteTimeout int `ini:"write_timeout"`
	// MaxBodySize requests the maximum size of the body (MB), default is 10
	MaxBodySize int `ini:"max_body_size"`
	// MCP MCP configuration
	MCP MCPConfig `ini:"mcp"`
	// AISecurity AI tool security policy configuration
	AISecurity AISecurityConfig `ini:"ai_security"`
	// SkillPath Skill storage path
	SkillPath string `ini:"skill_path"`
	// UserNamePasswordMap Username and password mapping (runtime generation)
	UserNamePasswordMap types.Properties `ini:"-"`
	// ApiKeyUserNameMap API key and username mapping (runtime generation)
	ApiKeyUserNameMap types.Properties `ini:"-"`
}

// SyncDerivedGlobals keeps selected top-level config values available through
// ${global.xxx} expressions used by AI agents and templates.
func (c *Config) SyncDerivedGlobals() {
	if c.Global == nil {
		c.Global = types.Properties{}
	}
	if c.SkillPath != "" {
		c.Global["skill_path"] = c.SkillPath
	}
}

// PprofConfig pprof configuration
type PprofConfig struct {
	Enable bool   `ini:"enable"`
	Addr   string `ini:"addr"`
}

// MCPConfig MCP configuration
type MCPConfig struct {
	Enable bool `ini:"enable"`
	// Groups MCP endpoint grouping, key is the group name, value is the tool list (comma-separated)
	// Supported syntax: * means all, -prefix* means exclude, rules/components/chains indicates tool categories
	Groups map[string]string `ini:"-"`
}

// AISecurityConfig AI tool security policy configuration
type AISecurityConfig struct {
	// Enable: Whether tool security blocking is enabled is false by default
	Enable bool `ini:"enable"`
	// Mode: deny (blacklist, intercept if not on list) or allow (whitelist, intercept if not on list)
	Mode string `ini:"mode"`
	// List of tool names intercepted in DenyTools blacklist mode (comma-separatored, supports * wildcards)
	DenyTools string `ini:"deny_tools"`
	// List of tool names allowed in AllowTools whitelist mode (comma-separatored, supports * wildcards)
	AllowTools string `ini:"allow_tools"`
	// DeniedTypes tool types for interception (comma separators): builtin, mcp, rulechain, subagent
	DeniedTypes string `ini:"denied_types"`
	// CmdDenyExtra bash tool extra command blacklist (comma-separated, added above the tool's own security check)
	CmdDenyExtra string `ini:"cmd_deny_extra"`
	// AllowPaths file path whitelist (comma-separatored), the read/write/edit tool can only access these paths and their sub-paths
	// If it is empty, there are no restrictions. Lower priority than DenyPaths
	AllowPaths string `ini:"allow_paths"`
	// DenyPaths file path blacklist (comma-separatored), prohibiting access to these paths and their sub-paths
	// Priority is higher than AllowPaths. Suitable for protecting sensitive paths such as deployment directories
	DenyPaths string `ini:"deny_paths"`
}

// InitUserMap generates username-password mappings and API Key-username mappings based on the Users configuration
func (c *Config) InitUserMap() {
	if c.Users != nil {
		c.UserNamePasswordMap = types.Properties{}
		for username, passwordAndApiKey := range c.Users {
			c.UserNamePasswordMap[strings.TrimSpace(username)] = strings.TrimSpace(strings.Split(passwordAndApiKey, ",")[0])
		}
		c.ApiKeyUserNameMap = types.Properties{}
		for username, passwordAndApiKey := range c.Users {
			params := strings.Split(passwordAndApiKey, ",")
			if len(params) > 1 {
				c.ApiKeyUserNameMap[strings.TrimSpace(params[1])] = strings.TrimSpace(username)
			}
		}
	}
}

// CheckPassword: Check the password
func (c *Config) CheckPassword(username, password string) bool {
	if c.UserNamePasswordMap == nil {
		return false
	}
	return c.UserNamePasswordMap[username] == password
}

// GetUsernameByApiKey Retrieves the username through the ApiKey
func (c *Config) GetUsernameByApiKey(apikey string) string {
	if c.ApiKeyUserNameMap == nil {
		return ""
	}
	return c.ApiKeyUserNameMap[apikey]
}

// GetApiKeyByUsername Retrieves the ApiKey from the username
func (c *Config) GetApiKeyByUsername(username string) string {
	if c.UserNamePasswordMap == nil {
		return ""
	}
	for apikey, u := range c.ApiKeyUserNameMap {
		if u == username {
			return apikey
		}
	}
	return ""
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	cfg := Config{
		DataDir:           "./data",
		SkillPath:         "./skills",
		CmdWhiteList:      "cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest",
		CmdMode:           "allow",
		FilePathWhiteList: "/tmp",
		LoadLuaLibs:       "true",
		Server:            ":9090",
		DefaultUsername:   "admin",
		MaxNodeLogSize:    40,
		ResourceMapping:   "/editor/*filepath=./editor,/images/*filepath=./editor/images",
		JwtSecretKey:      "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1",
		JwtExpireTime:     43200000,
		JwtIssuer:         "rulego.cc",
		ShareHttpServer:   false,
		AllowCors:         true,
		ReadTimeout:       30,
		WriteTimeout:      300,
		MaxBodySize:       10,
		LogLevel:          "info",
		LogMaxSize:        100,
		LogMaxBackups:     30,
		LogMaxAge:         7,
		Users: types.Properties{
			"admin": "admin,2af255ea5618467d914c67a8beeca31d",
		},
		Pprof: PprofConfig{
			Enable: false,
			Addr:   "0.0.0.0:6060",
		},
		MCP: MCPConfig{
			Enable: true,
		},
	}
	cfg.SyncDerivedGlobals()
	return cfg
}
