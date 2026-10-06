package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/config"
)

func TestGetKeyFromJSON(t *testing.T) {
	m := &Module{}

	t.Run("top level key", func(t *testing.T) {
		data := map[string]interface{}{"name": "test"}
		result := m.getKeyFromJSON(data, "name")
		if result != "test" {
			t.Errorf("got %v, want test", result)
		}
	})

	t.Run("nested key", func(t *testing.T) {
		data := map[string]interface{}{
			"server": map[string]interface{}{
				"port": 8080,
			},
		}
		result := m.getKeyFromJSON(data, "server.port")
		if result != 8080 {
			t.Errorf("got %v, want 8080", result)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		data := map[string]interface{}{"name": "test"}
		result := m.getKeyFromJSON(data, "missing")
		if result != nil {
			t.Errorf("got %v, want nil", result)
		}
	})

	t.Run("deeply nested", func(t *testing.T) {
		data := map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": "deep",
				},
			},
		}
		result := m.getKeyFromJSON(data, "a.b.c")
		if result != "deep" {
			t.Errorf("got %v, want deep", result)
		}
	})

	t.Run("non-map intermediate", func(t *testing.T) {
		data := map[string]interface{}{
			"a": "string",
		}
		result := m.getKeyFromJSON(data, "a.b")
		if result != nil {
			t.Errorf("got %v, want nil", result)
		}
	})
}

func TestIsSensitiveKey(t *testing.T) {
	cases := map[string]bool{
		"apiKey": true, "db_password": true, "Access-Token": true,
		"secret": true, "credentials": true,
		"baseUrl": false, "timeout": false, "poolSize": false,
	}
	for k, want := range cases {
		if got := IsSensitiveKey(k); got != want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", k, got, want)
		}
	}
}

func TestUpdateConfigMaskAndGlobal(t *testing.T) {
	dir := t.TempDir()
	m := &Module{cfg: &config.Config{DataDir: dir}}
	m.cfg.Global = types.Properties{"apiKey": "real-secret", "baseUrl": "http://a"}

	// 掩码占位值跳过，真值保持
	err := m.UpdateConfig(map[string]interface{}{"apiKey": MaskedValue, "baseUrl": "http://b"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetConfig()
	if got.Global["apiKey"] != "real-secret" {
		t.Errorf("masked value should keep original, got %q", got.Global["apiKey"])
	}
	if got.Global["baseUrl"] != "http://b" {
		t.Errorf("baseUrl = %q, want http://b", got.Global["baseUrl"])
	}

	// 持久化文件包含变更键，掩码键不落
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]interface{}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, exists := saved["apiKey"]; exists {
		t.Error("masked key should not be persisted")
	}
	if saved["baseUrl"] != "http://b" {
		t.Errorf("persisted baseUrl = %v, want http://b", saved["baseUrl"])
	}

	// 空 map 拒绝
	if err := m.UpdateConfig(map[string]interface{}{}); err == nil {
		t.Error("empty update should fail")
	}

	// null 值删除键：内存与持久化文件同步移除
	err = m.UpdateConfig(map[string]interface{}{"baseUrl": nil})
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := m.GetConfig()
	if _, exists := got2.Global["baseUrl"]; exists {
		t.Error("deleted key should be removed from Global")
	}
	data2, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var saved2 map[string]interface{}
	_ = json.Unmarshal(data2, &saved2)
	if _, exists := saved2["baseUrl"]; exists {
		t.Error("deleted key should be removed from config.json")
	}

	// 全部被过滤（仅掩码占位）时报错
	if err := m.UpdateConfig(map[string]interface{}{"apiKey": MaskedValue}); err == nil {
		t.Error("mask-only update should fail")
	}
}

func TestUpdateConfigCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &Module{cfg: &config.Config{DataDir: dir}}
	if err := m.UpdateConfig(map[string]interface{}{"a": 1}); err == nil {
		t.Error("corrupted config.json should fail the update, not be overwritten")
	}
}
