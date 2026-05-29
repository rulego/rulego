package config

import (
	"os"
	"testing"
)

func TestLoadMCPGroups(t *testing.T) {
	content := `
[mcp]
enable = true

[mcp.groups]
readonly = rules,list_components,get_component_doc
full = *
no-delete = *,-delete_rule_chain
`
	tmpFile, err := os.CreateTemp("", "config_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.MCP.Enable {
		t.Error("MCP.Enable should be true")
	}

	if cfg.MCP.Groups == nil {
		t.Fatal("MCP.Groups should not be nil")
	}
	if len(cfg.MCP.Groups) != 3 {
		t.Fatalf("len(Groups) = %d, want 3", len(cfg.MCP.Groups))
	}

	tests := []struct {
		group string
		want  string
	}{
		{"readonly", "rules,list_components,get_component_doc"},
		{"full", "*"},
		{"no-delete", "*,-delete_rule_chain"},
	}
	for _, tt := range tests {
		got := cfg.MCP.Groups[tt.group]
		if got != tt.want {
			t.Errorf("Groups[%q] = %q, want %q", tt.group, got, tt.want)
		}
	}
}

func TestLoadWithoutMCPGroups(t *testing.T) {
	content := `
[mcp]
enable = true
`
	tmpFile, err := os.CreateTemp("", "config_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MCP.Groups != nil {
		t.Errorf("Groups should be nil when [mcp.groups] section not present, got %v", cfg.MCP.Groups)
	}
}

func TestLoadMCPGroupsPreservesExisting(t *testing.T) {
	content := `
[mcp.groups]
new-group = rules
`
	tmpFile, err := os.CreateTemp("", "config_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	cfg.MCP.Groups = map[string]string{
		"existing": "components",
	}
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.MCP.Groups) != 2 {
		t.Fatalf("len(Groups) = %d, want 2", len(cfg.MCP.Groups))
	}
	if cfg.MCP.Groups["existing"] != "components" {
		t.Errorf("Groups[existing] should be preserved")
	}
	if cfg.MCP.Groups["new-group"] != "rules" {
		t.Errorf("Groups[new-group] should be loaded from INI")
	}
}

// ========== 环境变量替换测试 ==========

func TestExpandEnv(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		envKey string
		envVal string
		want   string
	}{
		{
			name:  "plain value without env var",
			input: "hello",
			want:  "hello",
		},
		{
			name:   "env var set",
			input:  "${TEST_RULEGO_KEY}",
			envKey: "TEST_RULEGO_KEY",
			envVal: "my-secret",
			want:   "my-secret",
		},
		{
			name:  "env var not set, no default",
			input: "${TEST_RULEGO_MISSING}",
			want:  "",
		},
		{
			name:  "env var not set, with default",
			input: "${TEST_RULEGO_MISSING:-fallback}",
			want:  "fallback",
		},
		{
			name:   "env var set, default ignored",
			input:  "${TEST_RULEGO_SET:-ignored}",
			envKey: "TEST_RULEGO_SET",
			envVal: "real-value",
			want:   "real-value",
		},
		{
			name:  "empty default",
			input: "${TEST_RULEGO_MISSING:-}",
			want:  "",
		},
		{
			name:   "env var in middle of string",
			input:  "prefix-${TEST_RULEGO_MID}-suffix",
			envKey: "TEST_RULEGO_MID",
			envVal: "middle",
			want:   "prefix-middle-suffix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envKey != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			}
			got := expandEnv(tt.input)
			if got != tt.want {
				t.Errorf("expandEnv(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadEnvVarExpansion(t *testing.T) {
	os.Setenv("TEST_RULEGO_JWT", "jwt-from-env")
	os.Setenv("TEST_RULEGO_LLM_KEY", "llm-key-from-env")
	defer os.Unsetenv("TEST_RULEGO_JWT")
	defer os.Unsetenv("TEST_RULEGO_LLM_KEY")

	content := []byte("jwt_secret_key = ${TEST_RULEGO_JWT}\n" +
		"require_auth = true\n\n" +
		"[global]\n" +
		"llm_api_key = ${TEST_RULEGO_LLM_KEY}\n" +
		"llm_url = ${TEST_RULEGO_URL:-http://localhost:11434}\n\n" +
		"[users]\n" +
		"admin = admin,api-key-123\n")

	tmpFile, err := os.CreateTemp("", "config_env_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// JWT 密钥应从环境变量替换
	if cfg.JwtSecretKey != "jwt-from-env" {
		t.Errorf("JwtSecretKey = %q, want %q", cfg.JwtSecretKey, "jwt-from-env")
	}
	// Global 中 llm_api_key 应从环境变量替换
	if cfg.Global["llm_api_key"] != "llm-key-from-env" {
		t.Errorf("Global[llm_api_key] = %q, want %q", cfg.Global["llm_api_key"], "llm-key-from-env")
	}
	// 环境变量未设置时使用默认值
	if cfg.Global["llm_url"] != "http://localhost:11434" {
		t.Errorf("Global[llm_url] = %q, want %q", cfg.Global["llm_url"], "http://localhost:11434")
	}
}

func TestLoadEnvVarWithDefault(t *testing.T) {
	// 不设置环境变量，验证默认值生效
	os.Unsetenv("TEST_RULEGO_JWT_NOT_SET")

	content := []byte("jwt_secret_key = ${TEST_RULEGO_JWT_NOT_SET:-default-secret}\n")

	tmpFile, err := os.CreateTemp("", "config_default_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.JwtSecretKey != "default-secret" {
		t.Errorf("JwtSecretKey = %q, want %q", cfg.JwtSecretKey, "default-secret")
	}
}

func TestLoadNewConfigFields(t *testing.T) {
	// 验证新增的 CORS、超时、body 限制配置能正确读取
	content := []byte("allow_cors = false\n" +
		"read_timeout = 60\n" +
		"write_timeout = 600\n" +
		"max_body_size = 50\n")

	tmpFile, err := os.CreateTemp("", "config_fields_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AllowCors != false {
		t.Errorf("AllowCors = %v, want false", cfg.AllowCors)
	}
	if cfg.ReadTimeout != 60 {
		t.Errorf("ReadTimeout = %d, want 60", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 600 {
		t.Errorf("WriteTimeout = %d, want 600", cfg.WriteTimeout)
	}
	if cfg.MaxBodySize != 50 {
		t.Errorf("MaxBodySize = %d, want 50", cfg.MaxBodySize)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.AllowCors != true {
		t.Errorf("default AllowCors = %v, want true", cfg.AllowCors)
	}
	if cfg.ReadTimeout != 30 {
		t.Errorf("default ReadTimeout = %d, want 30", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 300 {
		t.Errorf("default WriteTimeout = %d, want 300", cfg.WriteTimeout)
	}
	if cfg.MaxBodySize != 10 {
		t.Errorf("default MaxBodySize = %d, want 10", cfg.MaxBodySize)
	}
	if cfg.SkillPath != "./skills" {
		t.Errorf("default SkillPath = %q, want %q", cfg.SkillPath, "./skills")
	}
	if cfg.Global["skill_path"] != "./skills" {
		t.Errorf("default Global[skill_path] = %q, want %q", cfg.Global["skill_path"], "./skills")
	}
}

func TestLoadSyncsSkillPathToGlobal(t *testing.T) {
	content := []byte("skill_path = ./custom-skills\n")

	tmpFile, err := os.CreateTemp("", "config_skill_path_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.SkillPath != "./custom-skills" {
		t.Errorf("SkillPath = %q, want %q", cfg.SkillPath, "./custom-skills")
	}
	if cfg.Global["skill_path"] != "./custom-skills" {
		t.Errorf("Global[skill_path] = %q, want %q", cfg.Global["skill_path"], "./custom-skills")
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello"`, "hello"},
		{`hello`, "hello"},
		{`"hello`, `"hello`},
		{`hello"`, `hello"`},
		{`""`, ""},
		{`"he"llo"`, `he"llo`},
		{`"`, `"`},
		{``, ""},
	}
	for _, tt := range tests {
		got := trimQuotes(tt.input)
		if got != tt.want {
			t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadQuotedValues(t *testing.T) {
	content := []byte("jwt_secret_key = \"my-secret\"\n\n" +
		"[global]\n" +
		"llm_url = \"http://localhost:11434\"\n" +
		"llm_api_key = ${TEST_RULEGO_QKEY:-\"fallback-key\"}\n\n" +
		"[users]\n" +
		"admin = \"admin\",\"api-key-123\"\n")

	tmpFile, err := os.CreateTemp("", "config_quote_test_*.conf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	tmpFile.Close()

	cfg := DefaultConfig()
	if err := Load(tmpFile.Name(), &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.JwtSecretKey != "my-secret" {
		t.Errorf("JwtSecretKey = %q, want %q", cfg.JwtSecretKey, "my-secret")
	}
	if cfg.Global["llm_url"] != "http://localhost:11434" {
		t.Errorf("Global[llm_url] = %q, want %q", cfg.Global["llm_url"], "http://localhost:11434")
	}
	if cfg.Global["llm_api_key"] != "fallback-key" {
		t.Errorf("Global[llm_api_key] = %q, want %q", cfg.Global["llm_api_key"], "fallback-key")
	}
}
