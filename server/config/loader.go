package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/ini.v1"
)

// envPattern 匹配 ${ENV_VAR} 或 ${ENV_VAR:-default} 格式
var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// expandEnv 替换字符串中的 ${ENV_VAR} 为环境变量值。
// 支持 ${VAR:-default} 语法，环境变量未设置时使用默认值。
func expandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := envPattern.FindStringSubmatch(match)
		name := sub[1]
		defVal := sub[2]
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return defVal
	})
}

// trimQuotes 去除字符串首尾的双引号，支持 `"value"` 和 `value` 两种写法
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// expandProperties 对 map 中所有值执行环境变量替换并去除首尾引号
func expandProperties(m map[string]string) {
	for k, v := range m {
		v = expandEnv(v)
		m[k] = trimQuotes(v)
	}
}

// Load 从 INI 文件加载配置，INI 文件中的值覆盖 cfg 中的已有值。
// 调用方可先用 DefaultConfig() 初始化 cfg 以获取默认值。
func Load(path string, cfg *Config) error {
	file, err := ini.Load(path)
	if err != nil {
		return fmt.Errorf("load config file %s: %w", path, err)
	}

	if err := file.MapTo(cfg); err != nil {
		return fmt.Errorf("map config: %w", err)
	}

	// MapTo 无法自动映射 types.Properties，需要手动加载
	if section, err := file.GetSection("global"); err == nil {
		cfg.Global = section.KeysHash()
	}
	if section, err := file.GetSection("users"); err == nil {
		cfg.Users = section.KeysHash()
	}
	// 加载 MCP 分组配置
	if section, err := file.GetSection("mcp.groups"); err == nil {
		if cfg.MCP.Groups == nil {
			cfg.MCP.Groups = make(map[string]string)
		}
		for key, value := range section.KeysHash() {
			cfg.MCP.Groups[key] = value
		}
	}

	// 环境变量替换：支持 ${ENV_VAR} 和 ${ENV_VAR:-default} 语法
	expandProperties(cfg.Global)
	expandProperties(cfg.Users)

	// JWT 密钥支持环境变量
	cfg.JwtSecretKey = trimQuotes(expandEnv(cfg.JwtSecretKey))
	cfg.SkillPath = trimQuotes(expandEnv(cfg.SkillPath))

	cfg.ConfigFile = path
	cfg.SyncDerivedGlobals()
	cfg.InitUserMap()

	return nil
}
