package system

import (
	"testing"
)

func TestMergeConfigs(t *testing.T) {
	m := &Module{}

	t.Run("merges new keys", func(t *testing.T) {
		existing := map[string]interface{}{"a": 1, "b": 2}
		updates := map[string]interface{}{"c": 3}
		result := m.mergeConfigs(existing, updates)
		if len(result) != 3 {
			t.Errorf("len = %d, want 3", len(result))
		}
		if result["c"] != 3 {
			t.Errorf("c = %v, want 3", result["c"])
		}
	})

	t.Run("overwrites existing keys", func(t *testing.T) {
		existing := map[string]interface{}{"a": 1}
		updates := map[string]interface{}{"a": 2}
		result := m.mergeConfigs(existing, updates)
		if result["a"] != 2 {
			t.Errorf("a = %v, want 2", result["a"])
		}
	})

	t.Run("empty updates", func(t *testing.T) {
		existing := map[string]interface{}{"a": 1}
		updates := map[string]interface{}{}
		result := m.mergeConfigs(existing, updates)
		if len(result) != 1 {
			t.Errorf("len = %d, want 1", len(result))
		}
	})

	t.Run("nil existing", func(t *testing.T) {
		result := m.mergeConfigs(nil, map[string]interface{}{"a": 1})
		if result["a"] != 1 {
			t.Errorf("a = %v, want 1", result["a"])
		}
	})
}

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
