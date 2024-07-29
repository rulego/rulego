package config

import (
	"github.com/rulego/rulego/api/types"
	"time"
)

var C Config

func Get() *Config {
	return &C
}

func Set(c Config) {
	C = c
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
	// Mqtt mqtt配置
	Mqtt Mqtt `ini:"mqtt"`
	// 全局自定义配置，组件可以通过${global.xxx}方式取值
	Global types.Metadata `ini:"global"`
}
type Mqtt struct {
	//是否启用mqtt
	Enabled bool `ini:"enabled"`
	//mqtt broker 地址
	Server string `ini:"server"`
	//用户名
	Username string `ini:"username"`
	//密码
	Password string `ini:"password"`
	//重连重试间隔
	MaxReconnectInterval time.Duration `ini:"max_reconnec_tinterval"`
	QOS                  uint8         `ini:"qos"`
	CleanSession         bool          `ini:"clean_session"`
	//client Id
	ClientID    string `ini:"client_id"`
	CAFile      string `ini:"ca_file"`
	CertFile    string `ini:"cert_file"`
	CertKeyFile string `ini:"cert_key_file"`
	//订阅主题,多个与逗号隔开
	Topics string `ini:"topics"`
	//订阅主题对应交给那个规则链id处理
	ToChainId string `ini:"to_chain_id"`
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
	Mqtt: Mqtt{
		Server:       "172.0.0.1:1883",
		CleanSession: true,
		ToChainId:    "chain_call_rest_api",
	},
}
