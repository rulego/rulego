package config

import (
	"testing"

	"github.com/rulego/rulego/api/types"
)

func TestConfig_InitUserMap(t *testing.T) {
	cfg := Config{
		Users: types.Properties{
			"admin":   "password123,apikey123",
			"viewer":  "viewpass",
			"editor ": " editorpass , editorkey ",
		},
	}
	cfg.InitUserMap()

	// 验证用户名-密码映射
	if cfg.UserNamePasswordMap == nil {
		t.Fatal("UserNamePasswordMap should not be nil")
	}
	if cfg.UserNamePasswordMap["admin"] != "password123" {
		t.Errorf("admin password = %q, want %q", cfg.UserNamePasswordMap["admin"], "password123")
	}
	if cfg.UserNamePasswordMap["viewer"] != "viewpass" {
		t.Errorf("viewer password = %q, want %q", cfg.UserNamePasswordMap["viewer"], "viewpass")
	}

	// 验证 API Key-用户名映射
	if cfg.ApiKeyUserNameMap == nil {
		t.Fatal("ApiKeyUserNameMap should not be nil")
	}
	if cfg.ApiKeyUserNameMap["apikey123"] != "admin" {
		t.Errorf("apikey123 -> %q, want %q", cfg.ApiKeyUserNameMap["apikey123"], "admin")
	}
	// viewer 没有 API key，不应该出现在映射中
	if _, ok := cfg.ApiKeyUserNameMap[""]; ok {
		t.Error("empty apikey should not be in map")
	}
}

func TestConfig_InitUserMap_Nil(t *testing.T) {
	cfg := Config{Users: nil}
	cfg.InitUserMap()
	if cfg.UserNamePasswordMap != nil {
		t.Error("UserNamePasswordMap should be nil when Users is nil")
	}
}

func TestConfig_InitUserMap_TrimSpace(t *testing.T) {
	cfg := Config{
		Users: types.Properties{
			" user ": " pass , key ",
		},
	}
	cfg.InitUserMap()
	if cfg.UserNamePasswordMap["user"] != "pass" {
		t.Errorf("trim username = %q, want %q", cfg.UserNamePasswordMap["user"], "pass")
	}
	if cfg.ApiKeyUserNameMap["key"] != "user" {
		t.Errorf("trim apikey -> %q, want %q", cfg.ApiKeyUserNameMap["key"], "user")
	}
}

func TestConfig_CheckPassword(t *testing.T) {
	cfg := Config{
		Users: types.Properties{
			"admin": "secret",
		},
	}
	cfg.InitUserMap()

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"correct", "admin", "secret", true},
		{"wrong password", "admin", "wrong", false},
		{"nonexistent user", "nobody", "secret", false},
		{"empty password", "admin", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.CheckPassword(tt.username, tt.password); got != tt.want {
				t.Errorf("CheckPassword(%q, %q) = %v, want %v", tt.username, tt.password, got, tt.want)
			}
		})
	}
}

func TestConfig_CheckPassword_NilMap(t *testing.T) {
	cfg := Config{}
	if cfg.CheckPassword("admin", "pass") {
		t.Error("CheckPassword with nil map should return false")
	}
}

func TestConfig_GetUsernameByApiKey(t *testing.T) {
	cfg := Config{
		Users: types.Properties{
			"admin":  "pass,ak-123",
			"editor": "epass,ak-456",
		},
	}
	cfg.InitUserMap()

	if v := cfg.GetUsernameByApiKey("ak-123"); v != "admin" {
		t.Errorf("GetUsernameByApiKey(ak-123) = %q, want %q", v, "admin")
	}
	if v := cfg.GetUsernameByApiKey("ak-456"); v != "editor" {
		t.Errorf("GetUsernameByApiKey(ak-456) = %q, want %q", v, "editor")
	}
	if v := cfg.GetUsernameByApiKey("nonexistent"); v != "" {
		t.Errorf("GetUsernameByApiKey(nonexistent) = %q, want empty", v)
	}
}

func TestConfig_GetUsernameByApiKey_NilMap(t *testing.T) {
	cfg := Config{}
	if v := cfg.GetUsernameByApiKey("key"); v != "" {
		t.Errorf("GetUsernameByApiKey with nil map should return empty, got %q", v)
	}
}

func TestConfig_GetApiKeyByUsername(t *testing.T) {
	cfg := Config{
		Users: types.Properties{
			"admin":  "pass,ak-123",
			"editor": "epass,ak-456",
		},
	}
	cfg.InitUserMap()

	if v := cfg.GetApiKeyByUsername("admin"); v != "ak-123" {
		t.Errorf("GetApiKeyByUsername(admin) = %q, want %q", v, "ak-123")
	}
	if v := cfg.GetApiKeyByUsername("editor"); v != "ak-456" {
		t.Errorf("GetApiKeyByUsername(editor) = %q, want %q", v, "ak-456")
	}
	if v := cfg.GetApiKeyByUsername("nouser"); v != "" {
		t.Errorf("GetApiKeyByUsername(nouser) = %q, want empty", v)
	}
}

func TestConfig_GetApiKeyByUsername_NilMap(t *testing.T) {
	cfg := Config{}
	if v := cfg.GetApiKeyByUsername("admin"); v != "" {
		t.Errorf("GetApiKeyByUsername with nil map should return empty, got %q", v)
	}
}

func TestConfig_SyncDerivedGlobals(t *testing.T) {
	t.Run("nil global", func(t *testing.T) {
		cfg := Config{SkillPath: "/skills"}
		cfg.SyncDerivedGlobals()
		if cfg.Global["skill_path"] != "/skills" {
			t.Errorf("skill_path = %q, want %q", cfg.Global["skill_path"], "/skills")
		}
	})
	t.Run("empty skill path", func(t *testing.T) {
		cfg := Config{Global: types.Properties{}}
		cfg.SyncDerivedGlobals()
		if _, ok := cfg.Global["skill_path"]; ok {
			t.Error("skill_path should not be set when SkillPath is empty")
		}
	})
	t.Run("preserve existing", func(t *testing.T) {
		cfg := Config{
			Global:    types.Properties{"existing": "value"},
			SkillPath: "/new-skills",
		}
		cfg.SyncDerivedGlobals()
		if cfg.Global["existing"] != "value" {
			t.Error("existing global should be preserved")
		}
		if cfg.Global["skill_path"] != "/new-skills" {
			t.Error("skill_path should be added")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultUsername != "admin" {
		t.Errorf("DefaultUsername = %q, want %q", cfg.DefaultUsername, "admin")
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.JwtExpireTime <= 0 {
		t.Error("JwtExpireTime should be positive")
	}
	if cfg.JwtSecretKey == "" {
		t.Error("JwtSecretKey should not be empty")
	}
	if cfg.ReadTimeout <= 0 {
		t.Error("ReadTimeout should be positive")
	}
	if cfg.WriteTimeout <= 0 {
		t.Error("WriteTimeout should be positive")
	}
	if cfg.MaxBodySize <= 0 {
		t.Error("MaxBodySize should be positive")
	}
	if cfg.Global["skill_path"] != cfg.SkillPath {
		t.Error("SyncDerivedGlobals should have been called in DefaultConfig")
	}
}
