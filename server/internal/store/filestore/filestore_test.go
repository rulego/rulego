package filestore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
)

func newTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		DataDir: t.TempDir(),
	}
}

func TestNewUserStore(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewUserStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("store should not be nil")
	}
}

func TestUserStoreCreateAndValidate(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewUserStore(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.CreateUser(model.User{Username: "testuser", Password: "testpass"}); err != nil {
		t.Fatal(err)
	}

	// Test Save and Validate
	if err := store.fs.Save("", "testuser", "testpass"); err != nil {
		t.Fatal(err)
	}

	if !store.ValidatePassword("testuser", "testpass") {
		t.Error("ValidatePassword should return true")
	}
	if store.ValidatePassword("testuser", "wrong") {
		t.Error("ValidatePassword with wrong password should return false")
	}
	if store.ValidatePassword("nonexistent", "x") {
		t.Error("ValidatePassword with nonexistent user should return false")
	}
}

func TestUserStoreList(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewUserStore(cfg)
	if err != nil {
		t.Fatal(err)
	}

	store.fs.Save("", "user1", "pass1")
	store.fs.Save("", "user2", "pass2")

	users := store.List()
	if len(users) < 2 {
		t.Errorf("List returned %d users, want at least 2", len(users))
	}
}

func TestUserStoreDelete(t *testing.T) {
	cfg := newTestConfig(t)
	store, err := NewUserStore(cfg)
	if err != nil {
		t.Fatal(err)
	}

	store.fs.Save("", "deluser", "delpass")
	if !store.ValidatePassword("deluser", "delpass") {
		t.Fatal("user should exist before delete")
	}

	if err := store.Delete("deluser"); err != nil {
		t.Fatal(err)
	}
	if store.ValidatePassword("deluser", "delpass") {
		t.Error("user should not exist after delete")
	}
}

func TestNewSettingStore(t *testing.T) {
	cfg := newTestConfig(t)
	userDir := filepath.Join(cfg.DataDir, "workflows", "testuser")
	os.MkdirAll(userDir, 0755)

	store, err := NewSettingStore(cfg, userDir)
	if err != nil {
		t.Fatal(err)
	}

	// Test Save and Get
	if err := store.Save("key1", "value1"); err != nil {
		t.Fatal(err)
	}
	if v := store.Get("key1"); v != "value1" {
		t.Errorf("Get(key1) = %q, want value1", v)
	}
	if v := store.Get("nonexistent"); v != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", v)
	}

	// Test Delete
	if err := store.Delete("key1"); err != nil {
		t.Fatal(err)
	}
	if v := store.Get("key1"); v != "" {
		t.Errorf("Get after delete = %q, want empty", v)
	}
}

func TestNewRuleStore(t *testing.T) {
	cfg := newTestConfig(t)
	// Create user directories
	userDir := filepath.Join(cfg.DataDir, "workflows", "testuser", "rules")
	os.MkdirAll(userDir, 0755)

	store, err := NewRuleStore(cfg, "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("store should not be nil")
	}
}

func TestRuleStoreSaveAndGet(t *testing.T) {
	cfg := newTestConfig(t)
	userDir := filepath.Join(cfg.DataDir, "workflows", "testuser", "rules")
	os.MkdirAll(userDir, 0755)

	store, err := NewRuleStore(cfg, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	ruleDef := `{"ruleChain":{"id":"test-chain","name":"Test Chain","root":true,"disabled":false,"additionalInfo":{"username":"testuser","updateTime":"2026/01/01 00:00:00"}}}`
	if err := store.Save("testuser", "test-chain", []byte(ruleDef)); err != nil {
		t.Fatal(err)
	}

	data, err := store.Get("testuser", "test-chain")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("Get should return data")
	}

	// List
	chains, total, err := store.List("testuser", "", nil, nil, "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("List total = %d, want 1", total)
	}
	if len(chains) != 1 {
		t.Errorf("List count = %d, want 1", len(chains))
	}
}

func TestRuleStoreDelete(t *testing.T) {
	cfg := newTestConfig(t)
	userDir := filepath.Join(cfg.DataDir, "workflows", "testuser", "rules")
	os.MkdirAll(userDir, 0755)

	store, err := NewRuleStore(cfg, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	ruleDef := `{"ruleChain":{"id":"del-chain","name":"Delete Test","root":false,"disabled":false}}`
	store.Save("testuser", "del-chain", []byte(ruleDef))

	if err := store.Delete("testuser", "del-chain"); err != nil {
		t.Fatal(err)
	}

	_, err = store.Get("testuser", "del-chain")
	if err == nil {
		t.Error("Get after delete should return error")
	}
}

func TestNodePoolStore(t *testing.T) {
	cfg := newTestConfig(t)
	userDir := filepath.Join(cfg.DataDir, "workflows", "testuser")
	os.MkdirAll(userDir, 0755)

	store, err := NewNodePoolStore(cfg, "testuser")
	if err != nil {
		t.Fatal(err)
	}

	// Get when empty
	data, err := store.Get()
	if err != nil {
		t.Fatal(err)
	}
	if data != nil {
		t.Error("Get on empty store should return nil")
	}

	// Save
	testData := []byte(`{"ruleChain":{"id":"node_pool"},"metadata":{"nodes":[]}}`)
	if err := store.Save(testData); err != nil {
		t.Fatal(err)
	}

	// Get after save
	data, err = store.Get()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(testData) {
		t.Errorf("Get data mismatch")
	}
}
