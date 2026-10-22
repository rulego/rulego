package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
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

// envPrefix 主段标量配置的环境变量覆盖前缀，键名 = RULEGO_ + ini 键大写，
// 如 RULEGO_SERVER、RULEGO_DATA_DIR。供 Docker/K8s 免改 config.conf 调整常用项
const envPrefix = "RULEGO_"

// applyEnvOverrides 用 RULEGO_<KEY> 环境变量覆盖主段标量配置，环境变量优先于 config.conf。
// 结构体/映射等复合配置（global/pprof/mcp 等）不支持覆盖，静默跳过；标量解析失败报错，
// 避免错误的部署配置被静默吞掉
func applyEnvOverrides(cfg *Config) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("ini")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		envKey := envPrefix + strings.ToUpper(name)
		val, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}
		if err := setScalarField(v.Field(i), envKey, val); err != nil {
			return err
		}
	}
	return nil
}

func setScalarField(field reflect.Value, envKey, val string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("%s=%q: %w", envKey, val, err)
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("%s=%q: %w", envKey, val, err)
		}
		if field.OverflowInt(n) {
			return fmt.Errorf("%s=%q: out of range for %s", envKey, val, field.Type())
		}
		field.SetInt(n)
	case reflect.Pointer:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setScalarField(field.Elem(), envKey, val)
	default:
		return nil
	}
	return nil
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

	// RULEGO_<KEY> 环境变量覆盖主段键（优先级最高）
	if err := applyEnvOverrides(cfg); err != nil {
		return err
	}

	cfg.ConfigFile = path
	cfg.SyncDerivedGlobals()
	cfg.InitUserMap()

	return nil
}
