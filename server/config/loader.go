package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/ini.v1"
)

// envPattern matches the ${ENV_VAR} or ${ENV_VAR:-default} format
var envPattern = regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

// expandEnv replaces ${ENV_VAR} in the string as the environment variable value.
// Supports the ${VAR:-default} syntax; the default value is used when the environment variable is not set.
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

// trimQuotes removes surrounding double quotes and supports both `"value"` and `value` forms
func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// expandProperties: Performs environment variable replacements on all values in the map and removes quotes
func expandProperties(m map[string]string) {
	for k, v := range m {
		v = expandEnv(v)
		m[k] = trimQuotes(v)
	}
}

// Load loads the configuration from the INI file, and the values in the INI file override existing values in the cfg.
// You can first use DefaultConfig() to initialize the cfg to get the default value.
func Load(path string, cfg *Config) error {
	file, err := ini.Load(path)
	if err != nil {
		return fmt.Errorf("load config file %s: %w", path, err)
	}

	if err := file.MapTo(cfg); err != nil {
		return fmt.Errorf("map config: %w", err)
	}

	// MapTo cannot automatically map types.Properties, which need to be loaded manually
	if section, err := file.GetSection("global"); err == nil {
		cfg.Global = section.KeysHash()
	}
	if section, err := file.GetSection("users"); err == nil {
		cfg.Users = section.KeysHash()
	}
	// Load MCP packet configuration
	if section, err := file.GetSection("mcp.groups"); err == nil {
		if cfg.MCP.Groups == nil {
			cfg.MCP.Groups = make(map[string]string)
		}
		for key, value := range section.KeysHash() {
			cfg.MCP.Groups[key] = value
		}
	}

	// Environment variable replacement: supports ${ENV_VAR} and ${ENV_VAR:-default} syntax
	expandProperties(cfg.Global)
	expandProperties(cfg.Users)

	// JWT keys support environment variables
	cfg.JwtSecretKey = trimQuotes(expandEnv(cfg.JwtSecretKey))
	cfg.SkillPath = trimQuotes(expandEnv(cfg.SkillPath))

	cfg.ConfigFile = path
	cfg.SyncDerivedGlobals()
	cfg.InitUserMap()

	return nil
}
