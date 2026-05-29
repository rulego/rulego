package endpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// assistantRuleChainDocument 是测试用的规则链文档结构
type assistantRuleChainDocument struct {
	RuleChain struct {
		ID string `json:"id"`
	} `json:"ruleChain"`
	Metadata struct {
		Nodes []struct {
			ID            string                 `json:"id"`
			Type          string                 `json:"type"`
			Configuration map[string]interface{} `json:"configuration"`
		} `json:"nodes"`
	} `json:"metadata"`
}

// TestReadAssistantModelConfig verifies model settings can be extracted from
// the built-in assistant rule chain file.
func TestReadAssistantModelConfig(t *testing.T) {
	dataDir := t.TempDir()
	writeAssistantRuleChainFixture(t, dataDir)

	cfg, err := readAssistantModelConfig(dataDir, defaultAssistantID)
	if err != nil {
		t.Fatalf("readAssistantModelConfig() error = %v", err)
	}
	if cfg.URL != "https://api.deepseek.com" {
		t.Fatalf("URL = %q, want %q", cfg.URL, "https://api.deepseek.com")
	}
	if cfg.Model != "deepseek-chat" {
		t.Fatalf("Model = %q, want %q", cfg.Model, "deepseek-chat")
	}
	if cfg.Params.MaxTokens != 8192 {
		t.Fatalf("MaxTokens = %d, want 8192", cfg.Params.MaxTokens)
	}
}

// TestWriteAssistantModelConfig verifies model updates persist to disk and
// trigger a rule chain reload.
func TestWriteAssistantModelConfig(t *testing.T) {
	dataDir := t.TempDir()
	writeAssistantRuleChainFixture(t, dataDir)
	reloader := &mockAssistantReloader{}

	payload := assistantModelPayload{
		Provider:            "qwen",
		URL:                 "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Key:                 "sk-test",
		Model:               "qwen-plus",
		MaxStep:             12,
		MaxToolOutputLength: 4096,
		Params: assistantModelParams{
			Temperature:      0.3,
			TopP:             0.8,
			FrequencyPenalty: 0.1,
			PresencePenalty:  0.2,
			MaxTokens:        4096,
		},
	}

	updated, err := writeAssistantModelConfig(dataDir, "system", defaultAssistantID, payload, reloader)
	if err != nil {
		t.Fatalf("writeAssistantModelConfig() error = %v", err)
	}
	if updated.Provider != "qwen" {
		t.Fatalf("Provider = %q, want %q", updated.Provider, "qwen")
	}
	if reloader.username != "system" || reloader.chainID != defaultAssistantID {
		t.Fatalf("unexpected reload target = (%q, %q)", reloader.username, reloader.chainID)
	}
	if len(reloader.definition) == 0 {
		t.Fatal("expected reload definition to be captured")
	}

	reloaded, err := readAssistantModelConfig(dataDir, defaultAssistantID)
	if err != nil {
		t.Fatalf("readAssistantModelConfig() after update error = %v", err)
	}
	if reloaded.Model != "qwen-plus" {
		t.Fatalf("Model = %q, want %q", reloaded.Model, "qwen-plus")
	}
	if reloaded.MaxStep != 12 {
		t.Fatalf("MaxStep = %d, want 12", reloaded.MaxStep)
	}

	secondary, err := readAssistantAgentNodeConfig(dataDir, defaultAssistantID, "node_secondary")
	if err != nil {
		t.Fatalf("readAssistantAgentNodeConfig() error = %v", err)
	}
	if secondary["url"] != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("secondary url = %v, want updated value", secondary["url"])
	}
	if secondary["model"] != "qwen-plus" {
		t.Fatalf("secondary model = %v, want qwen-plus", secondary["model"])
	}
	if secondary["maxStep"] != float64(8) {
		t.Fatalf("secondary maxStep = %v, want preserved 8", secondary["maxStep"])
	}
}

type mockAssistantReloader struct {
	username   string
	chainID    string
	definition []byte
}

// SaveAndLoad captures the updated rule chain definition for assertions.
func (m *mockAssistantReloader) SaveAndLoad(username, chainID string, def []byte) error {
	m.username = username
	m.chainID = chainID
	m.definition = append([]byte(nil), def...)
	return nil
}

// writeAssistantRuleChainFixture stores a minimal assistant rule chain file
// used by the endpoint helper tests.
func writeAssistantRuleChainFixture(t *testing.T, dataDir string) {
	t.Helper()
	path := filepath.Join(dataDir, "system", "agents", defaultAssistantID, defaultAssistantID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{
  "ruleChain": { "id": "_assistant" },
  "metadata": {
    "nodes": [
      {
        "id": "node_agent",
        "type": "ai/agent",
        "configuration": {
          "url": "https://api.deepseek.com",
          "key": "sk-old",
          "model": "deepseek-chat",
          "maxStep": 25,
          "maxToolOutputLength": 50000,
          "params": {
            "temperature": 0.7,
            "topP": 0.9,
            "frequencyPenalty": 0.5,
            "presencePenalty": 0.5,
            "maxTokens": 8192
          },
          "tools": [
            {
              "type": "builtin",
              "name": "skill",
              "config": {
                "globalDirs": ["${global.skill_path}"]
              }
            }
          ]
        }
      },
      {
        "id": "node_secondary",
        "type": "ai/agent",
        "configuration": {
          "url": "https://api.deepseek.com",
          "key": "sk-old-2",
          "model": "deepseek-chat",
          "maxStep": 8,
          "params": {
            "temperature": 0.1,
            "topP": 0.5,
            "frequencyPenalty": 0.0,
            "presencePenalty": 0.0,
            "maxTokens": 1024
          }
        }
      }
    ]
  }
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

// readAssistantAgentNodeConfig returns a raw node configuration for assertions.
func readAssistantAgentNodeConfig(dataDir, agentID, nodeID string) (map[string]interface{}, error) {
	path := filepath.Join(dataDir, "system", "agents", agentID, agentID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc assistantRuleChainDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for _, node := range doc.Metadata.Nodes {
		if node.ID == nodeID {
			return node.Configuration, nil
		}
	}
	return nil, os.ErrNotExist
}
