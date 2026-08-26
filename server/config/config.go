package config

import (
	"strings"

	"github.com/rulego/rulego/api/types"
)

// Config 应用配置，通过 INI 文件加载
type Config struct {
	// ConfigFile 配置文件路径（非 INI 字段）
	ConfigFile string `ini:"-"`
	// DataDir 数据目录
	DataDir string `ini:"data_dir"`
	// LogFile 日志文件路径，为空则仅输出到控制台
	LogFile string `ini:"log_file"`
	// LogLevel 日志级别：debug/info/warn/error，默认 info
	LogLevel string `ini:"log_level"`
	// LogMaxSize 单个日志文件最大大小（MB），默认 100
	LogMaxSize int `ini:"log_max_size"`
	// LogMaxBackups 保留的旧日志文件最大数量，默认 30
	LogMaxBackups int `ini:"log_max_backups"`
	// LogMaxAge 保留旧日志文件的最大天数，默认 7
	LogMaxAge int `ini:"log_max_age"`
	// CmdWhiteList shell命令白名单，多个用逗号分隔
	CmdWhiteList string `ini:"cmd_white_list"`
	// CmdMode shell命令安全模式：allow(白名单模式) 或 deny(黑名单模式)，默认 allow
	CmdMode string `ini:"cmd_mode"`
	// CmdDenyList shell命令黑名单，多个用逗号分隔
	CmdDenyList string `ini:"cmd_deny_list"`
	// CmdDenyArgs 拒绝的命令参数模式，多个用逗号分隔
	CmdDenyArgs string `ini:"cmd_deny_args"`
	// FilePathWhiteList 允许操作的文件路径白名单
	FilePathWhiteList string `ini:"file_path_white_list"`
	// LoadLuaLibs 是否加载lua库
	LoadLuaLibs string `ini:"load_lua_libs"`
	// Server http服务器地址
	Server string `ini:"server"`
	// BasePath API路由基础路径前缀，例如 /rulego。用于嵌入式模式避免路由冲突
	BasePath string `ini:"base_path"`
	// DefaultUsername 默认用户名
	DefaultUsername string `ini:"default_username"`
	// Debug 是否把节点调试日志打印到日志文件
	Debug bool `ini:"debug"`
	// MaxNodeLogSize 最大节点日志大小
	MaxNodeLogSize int `ini:"max_node_log_size"`
	// ResourceMapping 静态文件路径映射
	ResourceMapping string `ini:"resource_mapping"`
	// DisableGzip 关闭内置 gzip（静态资源与 JSON API），默认开启。
	// 前置 nginx 已开启压缩时可置 true；两层同时开启不出错，仅多耗一层 CPU。
	DisableGzip bool `ini:"disable_gzip"`
	// Global 全局自定义配置
	Global types.Properties `ini:"global"`
	// NodePoolFile 节点池文件
	NodePoolFile string `ini:"node_pool_file"`
	// RunLogMode 运行记录级别：off（不记录，默认）/ summary（摘要）/ detail（完整节点日志）
	// 替代原 save_run_log。链级 additionalInfo.runLogMode 可覆盖此全局配置。
	RunLogMode string `ini:"run_log_mode"`
	// RunLogStoreType 运行日志存储类型：bbolt（默认）或 file（JSON Lines）
	RunLogStoreType string `ini:"run_log_store_type"`
	// RunLogRetentionCount 每条规则链保留最近 N 条运行记录，0 表示不限制。默认 500
	RunLogRetentionCount int `ini:"run_log_retention_count"`
	// RunLogRetentionDays 保留最近 N 天运行记录，0 表示不限制。默认 7
	RunLogRetentionDays int `ini:"run_log_retention_days"`
	// ScriptMaxExecutionTime 脚本最大执行时间（毫秒）
	ScriptMaxExecutionTime int `ini:"script_max_execution_time"`
	// EndpointEnabled 是否启用endpoint
	EndpointEnabled *bool `ini:"endpoint_enabled"`
	// SecretKey 密钥
	SecretKey *string `ini:"secret_key"`
	// EventBusChainId 核心规则链Id
	EventBusChainId string `ini:"event_bus_chain_id"`
	// CategoryFolderEnabled 是否按分类文件夹组织规则链
	CategoryFolderEnabled *bool `ini:"category_folder_enabled"`
	// RequireAuth api访问是否需要验证
	RequireAuth bool `ini:"require_auth"`
	// DisableLocalAuth 禁用 rulego-server 自身的登录/用户管理路由（/api/v1/login、/users*）。
	// 嵌入模式使用：认证完全由宿主通过 app.WithAuthenticator 注入的 SPI 承担，
	// 本地账号体系（config Users / UserStore）不再暴露，避免默认口令/双账号体系泄露面。
	// DisableLocalAuth disables rulego-server's own login/user-management routes
	// (/api/v1/login, /users*). For embedded mode: authentication is fully delegated to
	// the host-injected Authenticator SPI, keeping the local account system unexposed.
	DisableLocalAuth bool `ini:"disable_local_auth"`
	// JwtSecretKey jwt密钥
	JwtSecretKey string `ini:"jwt_secret_key"`
	// JwtExpireTime jwt过期时间（毫秒）
	JwtExpireTime int `ini:"jwt_expire_time"`
	// JwtIssuer jwt签发者
	JwtIssuer string `ini:"jwt_issuer"`
	// Users 用户列表
	Users types.Properties `ini:"users"`
	// Pprof pprof配置
	Pprof PprofConfig `ini:"pprof"`
	// MarketplaceBaseUrl 组件市场根地址
	MarketplaceBaseUrl string `ini:"marketplace_base_url"`
	// ShareHttpServer 是否默认HTTP服务设置成共享节点
	ShareHttpServer bool `ini:"share_http_server"`
	// AllowCors 是否允许跨域，默认 true（向后兼容）
	AllowCors bool `ini:"allow_cors"`
	// ReadTimeout HTTP 读超时（秒），默认 30
	ReadTimeout int `ini:"read_timeout"`
	// WriteTimeout HTTP 写超时（秒），默认 300（AI 聊天需要较长超时）
	WriteTimeout int `ini:"write_timeout"`
	// MaxBodySize 请求体最大大小（MB），默认 10
	MaxBodySize int `ini:"max_body_size"`
	// MCP MCP配置
	MCP MCPConfig `ini:"mcp"`
	// AISecurity AI 工具安全策略配置
	AISecurity AISecurityConfig `ini:"ai_security"`
	// SkillPath 技能存储路径
	SkillPath string `ini:"skill_path"`
	// UserNamePasswordMap 用户名和密码映射（运行期生成）
	UserNamePasswordMap types.Properties `ini:"-"`
	// ApiKeyUserNameMap API key和用户名映射（运行期生成）
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

// PprofConfig pprof 配置
type PprofConfig struct {
	Enable bool   `ini:"enable"`
	Addr   string `ini:"addr"`
}

// MCPConfig MCP 配置
type MCPConfig struct {
	Enable bool `ini:"enable"`
	// Groups MCP 端点分组配置，key 为组名，value 为工具列表（逗号分隔）
	// 支持语法：* 表示全部，-prefix* 表示排除，rules/components/chains 表示工具类别
	Groups map[string]string `ini:"-"`
}

// AISecurityConfig AI 工具安全策略配置
type AISecurityConfig struct {
	// Enable 是否启用工具安全拦截，默认 false
	Enable bool `ini:"enable"`
	// Mode 拦截模式：deny（黑名单，命中则拦截）或 allow（白名单，不在列表则拦截）
	Mode string `ini:"mode"`
	// DenyTools 黑名单模式下拦截的工具名称列表（逗号分隔，支持 * 通配符）
	DenyTools string `ini:"deny_tools"`
	// AllowTools 白名单模式下允许的工具名称列表（逗号分隔，支持 * 通配符）
	AllowTools string `ini:"allow_tools"`
	// DeniedTypes 拦截的工具类型（逗号分隔）：builtin, mcp, rulechain, subagent
	DeniedTypes string `ini:"denied_types"`
	// CmdDenyExtra bash 工具额外命令黑名单（逗号分隔，在工具自身安全检查之上追加）
	CmdDenyExtra string `ini:"cmd_deny_extra"`
	// AllowPaths 文件路径白名单（逗号分隔），read/write/edit 工具只能访问这些路径及其子路径
	// 为空则不限制。优先级低于 DenyPaths
	AllowPaths string `ini:"allow_paths"`
	// DenyPaths 文件路径黑名单（逗号分隔），禁止访问这些路径及其子路径
	// 优先级高于 AllowPaths。适合保护部署目录等敏感路径
	DenyPaths string `ini:"deny_paths"`
}

// InitUserMap 根据 Users 配置生成用户名-密码映射和 API Key-用户名映射
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

// CheckUserExists 检查用户是否在配置的内置账号里
func (c *Config) CheckUserExists(username string) bool {
	if c.UserNamePasswordMap == nil {
		return false
	}
	_, ok := c.UserNamePasswordMap[username]
	return ok
}

// CheckPassword 检查密码
func (c *Config) CheckPassword(username, password string) bool {
	if c.UserNamePasswordMap == nil {
		return false
	}
	return c.UserNamePasswordMap[username] == password
}

// GetUsernameByApiKey 通过ApiKey获取用户名
func (c *Config) GetUsernameByApiKey(apikey string) string {
	if c.ApiKeyUserNameMap == nil {
		return ""
	}
	return c.ApiKeyUserNameMap[apikey]
}

// GetApiKeyByUsername 通过用户名获取ApiKey
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

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	cfg := Config{
		DataDir:              "./data",
		SkillPath:            "./skills",
		CmdWhiteList:         "cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest",
		CmdMode:              "allow",
		FilePathWhiteList:    "/tmp",
		LoadLuaLibs:          "true",
		Server:               ":9090",
		DefaultUsername:      "admin",
		MaxNodeLogSize:       40,
		RunLogRetentionCount: 500,
		RunLogRetentionDays:  7,
		ResourceMapping:      "/editor/*filepath=./editor,/images/*filepath=./editor/images",
		JwtSecretKey:         "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1",
		JwtExpireTime:        43200000,
		JwtIssuer:            "rulego.cc",
		ShareHttpServer:      false,
		AllowCors:            true,
		ReadTimeout:          30,
		WriteTimeout:         300,
		MaxBodySize:          10,
		LogLevel:             "info",
		LogMaxSize:           100,
		LogMaxBackups:        30,
		LogMaxAge:            7,
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
