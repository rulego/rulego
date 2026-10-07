package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

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

// ExpandString 展开 ${VAR} / ${VAR:-默认值} 环境变量引用。供 data/config.json
// 运行时覆盖等非 ini 加载路径复用，保证与 config.conf 值的处理一致
func ExpandString(s string) string { return expandEnv(s) }

// reservedServerKeys 默认段（服务级）配置键集合。/config/global 只面向 [global] 段，
// 服务级键经界面写入不会生效，据此直接拒绝，避免「改了没反应」的错觉
var reservedServerKeys = func() map[string]struct{} {
	m := map[string]struct{}{}
	t := reflect.TypeOf(Config{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("ini")
		if tag == "" || tag == "-" {
			continue
		}
		if name := strings.Split(tag, ",")[0]; name != "" {
			m[name] = struct{}{}
		}
	}
	return m
}()

// IsReservedServerKey 键是否属于服务主配置（config.conf 默认段）
func IsReservedServerKey(k string) bool {
	_, ok := reservedServerKeys[k]
	return ok
}

// SensitiveKeyWords 敏感键名子串（不区分大小写），global 常用于存放密钥类配置
var SensitiveKeyWords = []string{"key", "secret", "token", "password", "passwd", "auth", "credential"}

// IsSensitiveKey 按键名命名约定判定敏感
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, w := range SensitiveKeyWords {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
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
