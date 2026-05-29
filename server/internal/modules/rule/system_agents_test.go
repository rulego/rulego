package rule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyEmbeddedDir(t *testing.T) {
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "_assistant")

	if err := copyEmbeddedDir(defaultAgentsFS, "template/_assistant", dst); err != nil {
		t.Fatalf("copyEmbeddedDir failed: %v", err)
	}

	// 验证 JSON 文件存在
	jsonPath := filepath.Join(dst, "_assistant.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read _assistant.json failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("_assistant.json is empty")
	}

	// 验证包含全局变量引用
	content := string(data)
	for _, pattern := range []string{"${global.llm_url}", "${global.llm_api_key}", "${global.llm_model}"} {
		if !containsStr(content, pattern) {
			t.Errorf("_assistant.json missing %s", pattern)
		}
	}

	// 验证 AGENTS.md 存在
	if _, err := os.Stat(filepath.Join(dst, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}

	// 验证 skills 子目录存在
	skillPath := filepath.Join(dst, "skills", "streamsql", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
}

func TestEnsureDefaultAgents_CreatesMissing(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "system", "agents")
	os.MkdirAll(agentsDir, 0755)

	m := &Module{}
	m.logger = &testLogger{}

	m.ensureDefaultAgents(agentsDir)

	// 验证 _assistant 目录被创建
	jsonPath := filepath.Join(agentsDir, "_assistant", "_assistant.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("_assistant.json not auto-created: %v", err)
	}
}

func TestEnsureDefaultAgents_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "system", "agents")
	assistantDir := filepath.Join(agentsDir, "_assistant")
	os.MkdirAll(assistantDir, 0755)

	// 写入已有的自定义 JSON
	customContent := []byte(`{"ruleChain":{"id":"_assistant","name":"Custom"}}`)
	os.WriteFile(filepath.Join(assistantDir, "_assistant.json"), customContent, 0644)

	m := &Module{}
	m.logger = &testLogger{}

	m.ensureDefaultAgents(agentsDir)

	// 验证文件未被覆盖
	data, _ := os.ReadFile(filepath.Join(assistantDir, "_assistant.json"))
	if string(data) != string(customContent) {
		t.Fatal("existing _assistant.json was overwritten")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// testLogger 实现 types.Logger 接口，用于单元测试
type testLogger struct{}

func (l *testLogger) Printf(format string, args ...interface{}) {}
func (l *testLogger) Debugf(format string, args ...interface{}) {}
func (l *testLogger) Infof(format string, args ...interface{})  {}
func (l *testLogger) Warnf(format string, args ...interface{})  {}
func (l *testLogger) Errorf(format string, args ...interface{}) {}
