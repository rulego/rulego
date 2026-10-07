package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	_, err := m.UpdateConfig(map[string]interface{}{"apiKey": MaskedValue, "baseUrl": "http://b"})
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
	if _, err := m.UpdateConfig(map[string]interface{}{}); err == nil {
		t.Error("empty update should fail")
	}

	// null 值删除键：内存与持久化文件同步移除
	_, err = m.UpdateConfig(map[string]interface{}{"baseUrl": nil})
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

	// 全部被过滤（仅掩码占位）时按幂等成功处理，不落盘不触发热更
	if reload, err := m.UpdateConfig(map[string]interface{}{"apiKey": MaskedValue}); err != nil {
		t.Errorf("mask-only update should be a no-op success: %v", err)
	} else if reload != nil && (len(reload.ReloadedChains) > 0 || len(reload.FailedChains) > 0) {
		t.Errorf("mask-only update should not reload anything: %+v", reload)
	}
}

func TestUpdateConfigCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := &Module{cfg: &config.Config{DataDir: dir}}
	if _, err := m.UpdateConfig(map[string]interface{}{"a": 1}); err == nil {
		t.Error("corrupted config.json should fail the update, not be overwritten")
	}
}

func TestUpdateConfigSameValueAndEmptyNewKey(t *testing.T) {
	dir := t.TempDir()
	m := &Module{cfg: &config.Config{DataDir: dir}}
	if _, err := m.UpdateConfig(map[string]interface{}{"k1": "v1"}); err != nil {
		t.Fatal(err)
	}
	// 同值覆盖：吸收，不产生变更
	if reload, err := m.UpdateConfig(map[string]interface{}{"k1": "v1"}); err != nil {
		t.Fatal(err)
	} else if reload != nil && len(reload.ReloadedChains) > 0 {
		t.Fatalf("同值覆盖不应触发重载: %+v", reload)
	}
	// 新增空字符串值的键：不得被零值比对吞掉
	if _, err := m.UpdateConfig(map[string]interface{}{"emptyKey": ""}); err != nil {
		t.Fatal(err)
	}
	if m.cfg.Global["emptyKey"] != "" {
		t.Fatalf("空值新键应写入: %q", m.cfg.Global["emptyKey"])
	}
	var saved map[string]interface{}
	data, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	_ = json.Unmarshal(data, &saved)
	if _, ok := saved["emptyKey"]; !ok {
		t.Fatal("空值新键应落盘（键存在，值为空串）")
	}
	// 已有键改为空串：真变更，落盘
	if _, err := m.UpdateConfig(map[string]interface{}{"k1": ""}); err != nil {
		t.Fatal(err)
	}
	if old, ok := m.cfg.Global["k1"]; !ok || old != "" {
		t.Fatalf("k1 应改为空串并保留键: %q ok=%v", old, ok)
	}
}

func TestUpdateConfigReservedKeyRejected(t *testing.T) {
	dir := t.TempDir()
	m := &Module{cfg: &config.Config{DataDir: dir}}
	m.cfg.Global = types.Properties{}

	// 服务级键（config.conf 默认段）界面写入不生效，直接拒绝
	for _, k := range []string{"server", "run_log_mode", "data_dir"} {
		if _, err := m.UpdateConfig(map[string]interface{}{k: "1"}); err == nil {
			t.Errorf("reserved key %s should be rejected", k)
		}
	}
	// 删除保留键允许：清理历史遗留的无效覆盖
	if _, err := m.UpdateConfig(map[string]interface{}{"server_port": nil}); err != nil {
		t.Errorf("deleting reserved key should be allowed: %v", err)
	}
	if !config.IsReservedServerKey("run_log_mode") || config.IsReservedServerKey("llm_url") {
		t.Error("IsReservedServerKey classification wrong")
	}
}

func TestUpdateConfigEnvExpansion(t *testing.T) {
	t.Setenv("RG_TEST_URL", "http://from-env")
	dir := t.TempDir()
	m := &Module{cfg: &config.Config{DataDir: dir}}
	m.cfg.Global = types.Properties{}

	// 值里的 ${VAR}/${VAR:-默认} 引用：内存生效值展开，文件留存原始写法
	_, err := m.UpdateConfig(map[string]interface{}{
		"svcUrl":   "${RG_TEST_URL}",
		"fallback": "${NO_SUCH_VAR:-fb}",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetConfig()
	if got.Global["svcUrl"] != "http://from-env" {
		t.Errorf("svcUrl = %q, want expanded env value", got.Global["svcUrl"])
	}
	if got.Global["fallback"] != "fb" {
		t.Errorf("fallback = %q, want fb", got.Global["fallback"])
	}
	data, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var saved map[string]interface{}
	_ = json.Unmarshal(data, &saved)
	if saved["svcUrl"] != "${RG_TEST_URL}" {
		t.Errorf("file should keep raw placeholder, got %v", saved["svcUrl"])
	}
}

func TestGlobalOverridesAndRestore(t *testing.T) {
	dir := t.TempDir()
	m := &Module{cfg: &config.Config{DataDir: dir}}
	// 模拟启动合并后的状态：fileBase 是文件值快照
	m.cfg.Global = types.Properties{"nats_url": "nats://ov", "api_key": "k1", "fileOnly": "fv"}
	m.cfg.GlobalFileBase = types.Properties{"nats_url": "nats://file", "api_key": "k2"}

	// 掩码文件覆盖值的读取
	if err := os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"nats_url":"nats://ov","api_key":"k1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ov, err := m.GlobalOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if ov["nats_url"] != "nats://ov" {
		t.Errorf("nats_url override = %q", ov["nats_url"])
	}
	if ov["api_key"] != MaskedValue {
		t.Errorf("sensitive override should be masked, got %q", ov["api_key"])
	}

	// 恢复文件值：有文件值的回到文件值，没有的整键删除
	if _, err := m.RestoreGlobalKey("nats_url"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetConfig()
	if got.Global["nats_url"] != "nats://file" {
		t.Errorf("nats_url = %q, want file value", got.Global["nats_url"])
	}
	if _, err := m.RestoreGlobalKey("fileOnly"); err != nil {
		t.Fatal(err)
	}
	got, _ = m.GetConfig()
	if _, exists := got.Global["fileOnly"]; exists {
		t.Error("key without file value should be removed after restore")
	}
	// 覆盖文件同步清理
	data, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	if strings.Contains(string(data), "nats_url") {
		t.Errorf("restored key should be removed from config.json: %s", data)
	}
}
