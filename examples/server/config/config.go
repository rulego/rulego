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
	// DataDir 数据目录
	DataDir string `ini:"data_dir"`
	// LogFile 日志文件
	LogFile string `ini:"log_file"`
	// CmdWhiteList shell命令白名单
	CmdWhiteList string `ini:"cmd_white_list"`
	// LoadLuaLibs 是否加载lua库
	LoadLuaLibs string `ini:"load_lua_libs"`
	// Server http服务器地址
	Server string `ini:"server"`
	// DefaultUsername 你们访问时候，默认用户名
	DefaultUsername string `ini:"default_username"`
	//是否把节点调试日志打印到日志文件
	Debug bool `ini:"debug"`
	//最大节点日志大小，默认40
	MaxNodeLogSize int `ini:"max_node_log_size"`
	//静态文件路径映射，例如:/ui/*filepath=/home/demo/dist,/images/*filepath=/home/demo/dist/images
	ResourceMapping string `ini:"resource_mapping"`
	// 全局自定义配置，组件可以通过${global.xxx}方式取值
	Global types.Metadata `ini:"global"`
	// 节点池文件，规则链json格式
	NodePoolFile string `ini:"node_pool_file"`
	// 是否保存运行日志到文件
	SaveRunLog bool `ini:"save_run_log"`
	// ScriptMaxExecutionTime json执行脚本的最大执行时间，单位毫秒
	ScriptMaxExecutionTime int `ini:"script_max_execution_time"`
	// EndpointEnabled 是否启用endpoint
	EndpointEnabled *bool `ini:"endpoint_enabled"`
	// SecretKey 密钥
	SecretKey *string `ini:"secret_key"`
	// EventBusChainId 核心规则链Id
	EventBusChainId string `ini:"event_bus_chain_id"`

	//RequireAuth api访问是否需要验证，默认不需要
	RequireAuth bool `ini:"require_auth"`
	// JwtSecretKey jwt密钥
	JwtSecretKey string `ini:"jwt_secret_key"`
	// JwtExpireTime jwt过期时间，单位毫秒
	JwtExpireTime int `ini:"jwt_expire_time"`
	// JwtIssuer jwt签发者
	JwtIssuer string `ini:"jwt_issuer"`
	// 用户列表
	Users types.Metadata `ini:"users"`

	//用户名和密码映射
	UserNamePasswordMap types.Metadata `ini:"-"`
	//API key和用户名映射
	ApiKeyUserNameMap types.Metadata `ini:"-"`
}

func (c *Config) InitUserMap() {
	if c.Users != nil {
		c.UserNamePasswordMap = types.Metadata{}
		for username, passwordAndApiKey := range c.Users {
			c.UserNamePasswordMap[strings.TrimSpace(username)] = strings.TrimSpace(strings.Split(passwordAndApiKey, ",")[0])
		}
		c.ApiKeyUserNameMap = types.Metadata{}
		for username, passwordAndApiKey := range c.Users {
			params := strings.Split(passwordAndApiKey, ",")
			if len(params) > 1 {
				c.ApiKeyUserNameMap[strings.TrimSpace(params[1])] = strings.TrimSpace(username)
			}
		}
	}
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

// DefaultConfig 默认配置
var DefaultConfig = Config{
	DataDir: "./data",
	//LogFile:      "./rulego.log",
	CmdWhiteList:    "cp,scp,mvn,npm,yarn,git,make,cmake,docker,kubectl,helm,ansible,puppet,pytest,python,python3,pip,go,java,dotnet,gcc,g++,ctest",
	LoadLuaLibs:     "true",
	Server:          ":9090",
	DefaultUsername: "admin",
	MaxNodeLogSize:  40,
	ResourceMapping: "/editor/*filepath=./editor,/images/*filepath=./editor/images",
	JwtSecretKey:    "r6G7qZ8xk9P0y1Q2w3E4r5T6y7U8i9O0pL7z8x9CvBnM3k2l1",
	JwtExpireTime:   43200000, //12小时
	JwtIssuer:       "rulego.cc",
	Users: types.Metadata{
		"admin": "admin",
	},
}
