package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/services"
)

const (
	defaultAssistantID = "_assistant"
)

type assistantPromptPayload struct {
	Content string `json:"content"`
}

type assistantModelPayload struct {
	Provider            string               `json:"provider"`
	URL                 string               `json:"url"`
	Key                 string               `json:"key"`
	Model               string               `json:"model"`
	MaxStep             int                  `json:"maxStep"`
	MaxToolOutputLength int                  `json:"maxToolOutputLength"`
	Params              assistantModelParams `json:"params"`
}

type assistantModelParams struct {
	Temperature      float64 `json:"temperature"`
	TopP             float64 `json:"topP"`
	FrequencyPenalty float64 `json:"frequencyPenalty"`
	PresencePenalty  float64 `json:"presencePenalty"`
	MaxTokens        int     `json:"maxTokens"`
}
type assistantRuleReloader interface {
	SaveAndLoad(username, chainId string, def []byte) error
}

func (s *Server) registerAIRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	ep.GET(endpoint.NewRouter().From(base + "/system/agents/:id/prompt").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		agentID, err := assistantIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		content, err := readAssistantPrompt(s.config.DataDir, agentID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				exchange.Out.SetBody([]byte(`{"error":"assistant prompt not found"}`))
				return false
			}
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{
			"agentId": agentID,
			"content": content,
		})
		return true
	}).End())

	ep.POST(endpoint.NewRouter().From(base + "/system/agents/:id/prompt").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		agentID, err := assistantIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		var req assistantPromptPayload
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		if err := writeAssistantPrompt(s.config.DataDir, agentID, req.Content); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		if err := reloadAssistantRuleChain(s.config.DataDir, s.config.DefaultUsername, agentID, admin); err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{
			"agentId": agentID,
			"content": req.Content,
		})
		return true
	}).End())

	ep.GET(endpoint.NewRouter().From(base + "/system/agents/:id/model").Process(s.authWithPermission("config", "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		agentID, err := assistantIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		modelCfg, err := readAssistantModelConfig(s.config.DataDir, agentID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				exchange.Out.SetStatusCode(http.StatusNotFound)
				exchange.Out.SetBody([]byte(`{"error":"assistant model config not found"}`))
				return false
			}
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{
			"agentId": agentID,
			"model":   modelCfg,
		})
		return true
	}).End())

	ep.POST(endpoint.NewRouter().From(base + "/system/agents/:id/model").Process(s.authWithPermission("config", "write")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		admin, ok := getService[services.RuleAdminService](s, exchange, services.KeyRuleManager)
		if !ok {
			return false
		}
		agentID, err := assistantIDFromExchange(exchange)
		if err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		var req assistantModelPayload
		if err := json.Unmarshal(exchange.In.Body(), &req); err != nil {
			writeBadRequest(exchange, err)
			return false
		}
		modelCfg, err := writeAssistantModelConfig(s.config.DataDir, s.config.DefaultUsername, agentID, req, admin)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		writeJSON(exchange, map[string]interface{}{
			"agentId": agentID,
			"model":   modelCfg,
		})
		return true
	}).End())
}

// assistantIDFromExchange resolves the target assistant id while keeping the
// fixed first-version default aligned with the editor UI.
func assistantIDFromExchange(exchange *endpointApi.Exchange) (string, error) {
	agentID := strings.TrimSpace(metadataValue(exchange, constants.KeyId))
	if agentID == "" {
		agentID = defaultAssistantID
	}
	if !validateId(agentID) {
		return "", errors.New("invalid assistant id")
	}
	return agentID, nil
}

// assistantPromptFilePath returns the canonical AGENTS.md path for the target
// built-in assistant after validating the assistant identifier.
func assistantPromptFilePath(dataDir, agentID string) (string, error) {
	if !validateId(agentID) {
		return "", errors.New("invalid assistant id")
	}
	return filepath.Join(dataDir, constants.DirSystemAgents, agentID, "AGENTS.md"), nil
}

// assistantRuleChainFilePath returns the canonical built-in assistant rule
// chain JSON path for the target assistant identifier.
func assistantRuleChainFilePath(dataDir, agentID string) (string, error) {
	if !validateId(agentID) {
		return "", errors.New("invalid assistant id")
	}
	return filepath.Join(dataDir, constants.DirSystemAgents, agentID, agentID+constants.RuleChainFileSuffix), nil
}

// readAssistantPrompt loads the assistant prompt markdown from disk.
func readAssistantPrompt(dataDir, agentID string) (string, error) {
	path, err := assistantPromptFilePath(dataDir, agentID)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// writeAssistantPrompt persists the assistant prompt markdown to disk.
func writeAssistantPrompt(dataDir, agentID, content string) error {
	path, err := assistantPromptFilePath(dataDir, agentID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// readAssistantModelConfig loads the ai/agent node model settings from the
// built-in assistant rule chain definition.
func readAssistantModelConfig(dataDir, agentID string) (assistantModelPayload, error) {
	path, err := assistantRuleChainFilePath(dataDir, agentID)
	if err != nil {
		return assistantModelPayload{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return assistantModelPayload{}, err
	}
	return extractAssistantModelPayload(data)
}

// writeAssistantModelConfig updates the ai/agent node model settings, persists
// the rule chain JSON, and reloads the assistant in memory immediately.
func writeAssistantModelConfig(dataDir, username, agentID string, payload assistantModelPayload, reloader assistantRuleReloader) (assistantModelPayload, error) {
	path, err := assistantRuleChainFilePath(dataDir, agentID)
	if err != nil {
		return assistantModelPayload{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return assistantModelPayload{}, err
	}
	updatedDef, updatedPayload, err := updateAssistantModelDefinition(data, payload)
	if err != nil {
		return assistantModelPayload{}, err
	}
	if err := os.WriteFile(path, updatedDef, 0644); err != nil {
		return assistantModelPayload{}, err
	}
	if err := reloader.SaveAndLoad(username, agentID, updatedDef); err != nil {
		return assistantModelPayload{}, err
	}
	return updatedPayload, nil
}

// reloadAssistantRuleChain reloads the assistant rule chain from disk after
// prompt or model assets are updated.
func reloadAssistantRuleChain(dataDir, username, agentID string, reloader assistantRuleReloader) error {
	path, err := assistantRuleChainFilePath(dataDir, agentID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return reloader.SaveAndLoad(username, agentID, data)
}

// extractAssistantModelPayload parses the first ai/agent node configuration
// into a compact API payload for the editor.
func extractAssistantModelPayload(def []byte) (assistantModelPayload, error) {
	node, err := findAssistantAgentNode(def)
	if err != nil {
		return assistantModelPayload{}, err
	}
	params := mapValue(node.Configuration, "params")
	return assistantModelPayload{
		Provider:            stringValue(node.Configuration["provider"]),
		URL:                 stringValue(node.Configuration["url"]),
		Key:                 stringValue(node.Configuration["key"]),
		Model:               stringValue(node.Configuration["model"]),
		MaxStep:             intValue(node.Configuration["maxStep"]),
		MaxToolOutputLength: intValue(node.Configuration["maxToolOutputLength"]),
		Params: assistantModelParams{
			Temperature:      floatValue(params["temperature"]),
			TopP:             floatValue(params["topP"]),
			FrequencyPenalty: floatValue(params["frequencyPenalty"]),
			PresencePenalty:  floatValue(params["presencePenalty"]),
			MaxTokens:        intValue(params["maxTokens"]),
		},
	}, nil
}

func mapValue(source types.Configuration, key string) map[string]interface{} {
	value, ok := source[key].(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return value
}

// updateAssistantModelDefinition patches the ai/agent node configuration while
// preserving all other parts of the rule chain definition (connections, ruleChain,
// additionalInfo, debugMode, etc.) using the framework's types.RuleChain model.
func updateAssistantModelDefinition(def []byte, payload assistantModelPayload) ([]byte, assistantModelPayload, error) {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return nil, assistantModelPayload{}, err
	}
	firstAgentUpdated := false
	for _, node := range ruleChain.Metadata.Nodes {
		if node.Type != "ai/agent" {
			continue
		}
		if node.Configuration == nil {
			node.Configuration = make(types.Configuration)
		}
		node.Configuration["url"] = payload.URL
		node.Configuration["key"] = payload.Key
		node.Configuration["model"] = payload.Model
		if !firstAgentUpdated {
			if payload.Provider != "" {
				node.Configuration["provider"] = payload.Provider
			} else {
				delete(node.Configuration, "provider")
			}
			node.Configuration["maxStep"] = payload.MaxStep
			node.Configuration["maxToolOutputLength"] = payload.MaxToolOutputLength
			node.Configuration["params"] = map[string]interface{}{
				"temperature":      payload.Params.Temperature,
				"topP":             payload.Params.TopP,
				"frequencyPenalty": payload.Params.FrequencyPenalty,
				"presencePenalty":  payload.Params.PresencePenalty,
				"maxTokens":        payload.Params.MaxTokens,
			}
			firstAgentUpdated = true
		}
	}
	if !firstAgentUpdated {
		return nil, assistantModelPayload{}, errors.New("assistant agent node not found")
	}
	updatedDef, err := json.MarshalIndent(ruleChain, "", "  ")
	if err != nil {
		return nil, assistantModelPayload{}, err
	}
	updatedPayload, err := extractAssistantModelPayload(updatedDef)
	if err != nil {
		return nil, assistantModelPayload{}, err
	}
	return updatedDef, updatedPayload, nil
}

// findAssistantAgentNode locates the first ai/agent node in the rule chain
// definition.
func findAssistantAgentNode(def []byte) (*types.RuleNode, error) {
	var ruleChain types.RuleChain
	if err := json.Unmarshal(def, &ruleChain); err != nil {
		return nil, err
	}
	for i := range ruleChain.Metadata.Nodes {
		if ruleChain.Metadata.Nodes[i].Type == "ai/agent" {
			return ruleChain.Metadata.Nodes[i], nil
		}
	}
	return nil, errors.New("assistant agent node not found")
}

// stringValue converts JSON scalar values to strings.
func stringValue(value interface{}) string {
	str, _ := value.(string)
	return str
}

// floatValue converts JSON number values to float64.
func floatValue(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// intValue converts JSON number values to int.
func intValue(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
