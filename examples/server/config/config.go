package config

import (
	"examples/server/internal/constants"
	"strings"

	"github.com/rulego/rulego/api/types"
)

var C Config

func Get() *Config {
	return &C
}

func Set(c Config) {
	C = c
	if C.EventBusChainId == "" {
		C.EventBusChainId = constants.KeyDefaultIntegrationChainId
	}
}

type Config struct {
	// DataDir data directory
	DataDir string `ini:"data_dir"`
	// LogFile
	LogFile string `ini:"log_file"`
	// CmdWhiteList shell command: whitelist multiple units separated by commas
	CmdWhiteList string `ini:"cmd_white_list"`
	// FilePathWhiteList allows file path whitelists to operate on, used to control file node privileges. Supports wildcard formats, such as: /data/*/output Multiple paths separated by commas.
	FilePathWhiteList string `ini:"file_path_white_list"`

	// LoadLuaLibs Whether to load the lua library
	LoadLuaLibs string `ini:"load_lua_libs"`
	// Server http server address
	Server string `ini:"server"`
	// DefaultUsername The default username
	DefaultUsername string `ini:"default_username"`
	//Whether node debug logs are printed into log files
	Debug bool `ini:"debug"`
	//Maximum node log size, default 40
	MaxNodeLogSize int `ini:"max_node_log_size"`
	//Static file path mapping, for example: /ui/*filepath=/home/demo/dist, /images/*filepath=/home/demo/dist/images
	ResourceMapping string `ini:"resource_mapping"`
	// Global custom configuration, components can take values using ${global.xxx}
	Global types.Properties `ini:"global"`
	// Node pool files, rule chain in JSON format
	NodePoolFile string `ini:"node_pool_file"`
	// Whether to save the runtime log to a file
	SaveRunLog bool `ini:"save_run_log"`
	// ScriptMaxExecutionTime: The maximum execution time for a script in milliseconds
	ScriptMaxExecutionTime int `ini:"script_max_execution_time"`
	// EndpointEnabled Whether to enable Endpoint
	EndpointEnabled *bool `ini:"endpoint_enabled"`
	// SecretKey key
	SecretKey *string `ini:"secret_key"`
	// EventBusChainId Core rule Chain Id
	EventBusChainId string `ini:"event_bus_chain_id"`

	//Whether authentication is required for RequireAuth API access, by default it is not required
	RequireAuth bool `ini:"require_auth"`
	// JwtSecretKey jwt key
	JwtSecretKey string `ini:"jwt_secret_key"`
	// JwtExpireTime, jwt expiration time, in milliseconds
	JwtExpireTime int `ini:"jwt_expire_time"`
	// JwtIssuer jwt issuer
	JwtIssuer string `ini:"jwt_issuer"`
	// User list
	Users types.Properties `ini:"users"`
	// Pprof pprof configuration
	Pprof Pprof `ini:"pprof"`
	// Module market root address
	MarketplaceBaseUrl string `ini:"marketplace_base_url"`
	// Is the HTTP service set to a shared node by default?
	ShareHttpServer bool `ini:"share_http_server"`
	// MCP configuration
	MCP MCP `ini:"mcp"`
	//Username and password mapping
	UserNamePasswordMap types.Properties `ini:"-"`
	//API key and username mapping
	ApiKeyUserNameMap types.Properties `ini:"-"`
}
type Pprof struct {
	Enable bool   `ini:"enable"`
	Addr   string `ini:"addr"`
}

type MCP struct {
	// Whether to enable MCP service is set to true by default
	Enable bool `ini:"enable"`
	// Whether to use components as MCP tools defaults to true
	LoadComponentsAsTool bool `ini:"load_components_as_tool"`
	// Whether to use the rule chain as an MCP tool, default is true
	LoadChainsAsTool bool `ini:"load_chains_as_tool"`
	// Whether to set the API seat MCP tool to true by default
	LoadApisAsTool bool `ini:"load_apis_as_tool"`
	// Rule chain IDs for exclusion, multiple separated by commas. Supports *wildcards, for example: *Filter
	ExcludeChains string `ini:"exclude_chains"`
	// Excluded components, multiple separated by commas. Supports *wildcards, for example: *Filter
	ExcludeComponents string `ini:"exclude_components"`
}

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

// DefaultConfig The default configuration
var DefaultConfig = Config{
	DataDir: "./data",
	//LogFile:      "./rulego.log",
	CmdWhiteList:       "cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest",
	FilePathWhiteList:  "/tmp",
	LoadLuaLibs:        "true",
	Server:             ":9090",
	DefaultUsername:    "admin",
	MaxNodeLogSize:     40,
	ResourceMapping:    "/editor/*filepath=./editor,/images/*filepath=./editor/images",
	JwtSecretKey:       "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1",
	JwtExpireTime:      43200000, //12 hours
	JwtIssuer:          "rulego.cc",
	MarketplaceBaseUrl: "http://8.134.32.225:9090/api/v1",
	ShareHttpServer:    true,
	Users: types.Properties{
		"admin": "admin,2af255ea5618467d914c67a8beeca31d",
	},
	Pprof: Pprof{
		Enable: false,
		Addr:   "0.0.0.0:6060",
	},
	MCP: MCP{
		Enable:               true,
		LoadComponentsAsTool: true,
		LoadChainsAsTool:     true,
		LoadApisAsTool:       true,
		ExcludeComponents:    "comment,iterator,delay,groupAction,ref,fork,join,for,*Filter",
	},
}
