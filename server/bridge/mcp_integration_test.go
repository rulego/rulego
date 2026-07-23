package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	// Register for AI components
	_ "github.com/rulego/rulego-components-ai/agent"
	_ "github.com/rulego/rulego-components-ai/processor"
)

// skipIfNoLLMKey skips the test if LLM_API_KEY is not set
func skipIfNoLLMKey(t *testing.T) {
	t.Helper()
	if os.Getenv("LLM_API_KEY") == "" {
		t.Skip("Skipping integration testing: LLM_API_KEY environment variable not set")
	}
}

// newIntegrationBridge creates a bridge for integration testing.
// Use a clean temporary data directory, copying only system/agents.
func newIntegrationBridge(t *testing.T) *Bridge {
	t.Helper()

	// Create a clean temporary data directory to avoid interference from old chains
	tmpData := t.TempDir()
	srcData := os.Getenv("RULEGO_DATA_DIR")
	if srcData == "" {
		srcData, _ = filepath.Abs(filepath.Join("..", "data"))
	}
	copyDir(t, filepath.Join(srcData, "system"), filepath.Join(tmpData, "system"))
	// Compatible with older paths: Copy agents to data_dir/agents/ (some older configurations use global.data_dir + '/agents/')
	copyDir(t, filepath.Join(srcData, "system", "agents"), filepath.Join(tmpData, "agents"))

	llmURL := os.Getenv("LLM_BASE_URL")
	if llmURL == "" {
		llmURL = "https://api.deepseek.com/v1"
	}
	llmModel := os.Getenv("LLM_MODEL")
	if llmModel == "" {
		llmModel = "deepseek-chat"
	}

	cfgContent := "server = :0\n" +
		"data_dir = " + tmpData + "\n" +
		"default_username = admin\n" +
		"require_auth = false\n" +
		"[users]\nadmin = admin,2af255ea5618467d914c67a8beeca31d\n" +
		"[mcp]\nenable = true\n" +
		"[global]\n" +
		"data_dir = " + tmpData + "\n" +
		"llm_url = " + llmURL + "\n" +
		"llm_api_key = " + os.Getenv("LLM_API_KEY") + "\n" +
		"llm_model = " + llmModel + "\n"

	cfgFile := filepath.Join(t.TempDir(), "config.conf")
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("Configuration file writing failure: %v", err)
	}

	// Fixed parameter compatibility in generator-lite (some models do not support frequencyPenalty/presencePenalty)
	patchAgentParams(t, filepath.Join(tmpData, "system", "agents", "generator-lite", "generator-lite.json"))
	patchAgentParams(t, filepath.Join(tmpData, "system", "agents", "generator", "generator.json"))

	br, err := NewBridgeWithDefaults(cfgFile)
	if err != nil {
		t.Fatalf("Failed to create Bridge: %v", err)
	}
	return br
}

// patchAgentParams Fixes parameter compatibility in agent configuration
func patchAgentParams(t *testing.T, jsonPath string) {
	t.Helper()
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Logf("patchAgentParams: Skip %s: %v", jsonPath, err)
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Logf("patchAgentParams: Parsing failure %s: %v", jsonPath, err)
		return
	}
	metadata, _ := cfg["metadata"].(map[string]interface{})
	if metadata == nil {
		return
	}
	nodes, _ := metadata["nodes"].([]interface{})
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		config, _ := node["configuration"].(map[string]interface{})
		if config == nil {
			continue
		}
		params, _ := config["params"].(map[string]interface{})
		if params == nil {
			continue
		}
		// Some models do not support these parameters; deleting them can avoid errors
		delete(params, "frequencyPenalty")
		delete(params, "presencePenalty")
		t.Logf("patchAgentParams: 已清理 %s 中节点 %v 的 penalty 参数", jsonPath, node["id"])
	}
	patched, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(jsonPath, patched, 0644); err != nil {
		t.Logf("patchAgentParams: Write failure %s: %v", jsonPath, err)
	}
}

// copyDir recursively copies the directory
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return
	}
	os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyDir(t, sp, dp)
		} else {
			data, err := os.ReadFile(sp)
			if err != nil {
				continue
			}
			os.WriteFile(dp, data, 0644)
		}
	}
}

// chatRequest sends a chat request to the generator endpoint
func chatRequest(t *testing.T, handler http.Handler, token string, messages []map[string]string, stream bool) *http.Response {
	t.Helper()
	body := map[string]interface{}{
		"messages": messages,
		"stream":   stream,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/generator/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result()
}

// getRuleChainViaAPI retrieves the saved rule chain via the REST API
func getRuleChainViaAPI(t *testing.T, handler http.Handler, token, chainId string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules/"+chainId, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// parseResponse extracts the final message content from non-streaming responses
func parseResponse(t *testing.T, resp *http.Response) string {
	t.Helper()
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return string(bodyBytes)
	}
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok {
					return content
				}
			}
		}
	}
	return string(bodyBytes)
}

// ---- Validation Auxiliary Functions ----

func findNodesByType(chainDef map[string]interface{}, nodeType string) []map[string]interface{} {
	metadata, _ := chainDef["metadata"].(map[string]interface{})
	nodes, _ := metadata["nodes"].([]interface{})
	var result []map[string]interface{}
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		if node["type"] == nodeType {
			result = append(result, node)
		}
	}
	return result
}

func hasNodeType(chainDef map[string]interface{}, nodeType string) bool {
	return len(findNodesByType(chainDef, nodeType)) > 0
}

func getNodeConfig(chainDef map[string]interface{}, nodeType string) map[string]interface{} {
	nodes := findNodesByType(chainDef, nodeType)
	if len(nodes) == 0 {
		return nil
	}
	config, _ := nodes[0]["configuration"].(map[string]interface{})
	return config
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ---- Enhanced Verification Auxiliary Function ----

// ValidationResult Summary of validation results
type ValidationResult struct {
	Valid            bool
	TotalNodes       int
	TotalConnections int
	NodeTypes        map[string]int
	Errors           []string
	Warnings         []string
}

// validateChainStructure verifies the integrity of the rule chain structure
func validateChainStructure(t *testing.T, chainDef map[string]interface{}) *ValidationResult {
	t.Helper()
	result := &ValidationResult{
		Valid:     true,
		NodeTypes: make(map[string]int),
	}

	metadata, ok := chainDef["metadata"].(map[string]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, "缺少 metadata 字段")
		return result
	}

	// Verification nodes
	nodes, ok := metadata["nodes"].([]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, "metadata.nodes 不是数组或不存在")
		return result
	}
	result.TotalNodes = len(nodes)

	// Collect all node IDs and types to check for uniqueness
	nodeIDs := make(map[string]bool)

	for i, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("节点[%d] 格式无效", i))
			continue
		}

		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)

		if nodeID == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("节点[%d] 缺少 id", i))
		} else if nodeIDs[nodeID] {
			result.Errors = append(result.Errors, fmt.Sprintf("节点 ID 重复: %s", nodeID))
		} else {
			nodeIDs[nodeID] = true
		}

		if nodeType == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("节点[%d] (%s) 缺少 type", i, nodeID))
		} else {
			result.NodeTypes[nodeType]++
		}

		// Verify the node name
		if _, hasName := node["name"]; !hasName {
			result.Warnings = append(result.Warnings, fmt.Sprintf("节点 %s 缺少 name", nodeID))
		}
	}

	// Verify the connection
	connections, ok := metadata["connections"].([]interface{})
	if !ok {
		result.Warnings = append(result.Warnings, "metadata.connections 不存在或格式无效")
	} else {
		result.TotalConnections = len(connections)
		for i, c := range connections {
			conn, ok := c.(map[string]interface{})
			if !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("连接[%d] 格式无效", i))
				continue
			}

			fromID, _ := conn["fromId"].(string)
			toID, _ := conn["toId"].(string)

			if fromID == "" || toID == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("连接[%d] 缺少 fromId 或 toId", i))
			} else {
				if !nodeIDs[fromID] {
					result.Errors = append(result.Errors, fmt.Sprintf("连接引用不存在的节点: fromId=%s", fromID))
				}
				if !nodeIDs[toID] {
					result.Errors = append(result.Errors, fmt.Sprintf("连接引用不存在的节点: toId=%s", toID))
				}
			}
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// printValidationResult Prints the details of the validation result
func printValidationResult(t *testing.T, result *ValidationResult) {
	t.Helper()
	t.Logf("=== Rule chain validation results ===")
	t.Logf("Total number of nodes: %d", result.TotalNodes)
	t.Logf("Total connections: %d", result.TotalConnections)
	t.Logf("Node type distribution:")
	for nodeType, count := range result.NodeTypes {
		t.Logf("  - %s: %d", nodeType, count)
	}

	if len(result.Errors) > 0 {
		t.Logf("Error (%d):", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("  ✗ %s", err)
		}
	}

	if len(result.Warnings) > 0 {
		t.Logf("Warning (%d):", len(result.Warnings))
		for _, warn := range result.Warnings {
			t.Logf("  ⚠ %s", warn)
		}
	}

	if result.Valid {
		t.Log("Verification result: ✓ Passed")
	} else {
		t.Log("Verification result: ✗ Failed")
	}
}

// waitForChainWithRetry Waits for the rule chain to complete creation (with retry)
func waitForChainWithRetry(t *testing.T, handler http.Handler, token, chainId string, maxRetries int) map[string]interface{} {
	t.Helper()
	for i := 0; i < maxRetries; i++ {
		chain := getRuleChainViaAPI(t, handler, token, chainId)
		if chain != nil {
			return chain
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// GenerationQuality generates quality assessments
type GenerationQuality struct {
	ParseSuccess       bool    // Whether the analysis was successful
	ChainCreated       bool    // Whether a rule chain is created
	NodeCount          int     // Number of nodes
	ConnectionCount    int     // Number of connections
	HasStartNode       bool    // Is there a starting node?
	HasEndNode         bool    // Is there an end node?
	ConfigCompleteness float64 // Configuration completeness (0-1)
	StructureValid     bool    // Whether the structure is effective
	Score              float64 // Overall score (0-100)
}

// evaluateGenerationQuality assesses the quality of generation
func evaluateGenerationQuality(t *testing.T, chainDef map[string]interface{}, expectedNodeTypes []string) *GenerationQuality {
	t.Helper()
	quality := &GenerationQuality{}

	if chainDef == nil {
		return quality
	}

	quality.ParseSuccess = true
	quality.ChainCreated = true

	metadata, _ := chainDef["metadata"].(map[string]interface{})
	if metadata == nil {
		return quality
	}

	nodes, _ := metadata["nodes"].([]interface{})
	connections, _ := metadata["connections"].([]interface{})
	quality.NodeCount = len(nodes)
	quality.ConnectionCount = len(connections)

	// Check the start and end nodes
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		if node == nil {
			continue
		}
		nodeType, _ := node["type"].(string)
		if strings.Contains(strings.ToLower(nodeType), "input") || nodeType == "msg" || nodeType == "mqtt" {
			quality.HasStartNode = true
		}
		if nodeType == "log" || nodeType == "restApiCall" || strings.Contains(strings.ToLower(nodeType), "output") {
			quality.HasEndNode = true
		}
	}

	// Calculate configuration completeness
	configuredNodes := 0
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		if node == nil {
			continue
		}
		if config, ok := node["configuration"].(map[string]interface{}); ok && len(config) > 0 {
			configuredNodes++
		}
	}
	if quality.NodeCount > 0 {
		quality.ConfigCompleteness = float64(configuredNodes) / float64(quality.NodeCount)
	}

	// Verify structure
	validation := validateChainStructure(t, chainDef)
	quality.StructureValid = validation.Valid

	// Check the desired node type
	foundTypes := make(map[string]bool)
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		if node != nil {
			nodeType, _ := node["type"].(string)
			foundTypes[nodeType] = true
		}
	}

	expectedFound := 0
	for _, expectedType := range expectedNodeTypes {
		if foundTypes[expectedType] {
			expectedFound++
		}
	}

	// Calculate the overall score
	score := 0.0
	if quality.ParseSuccess {
		score += 10
	}
	if quality.ChainCreated {
		score += 10
	}
	if quality.NodeCount >= 2 {
		score += 15
	}
	if quality.ConnectionCount >= 1 {
		score += 15
	}
	if quality.HasStartNode {
		score += 10
	}
	if quality.HasEndNode {
		score += 10
	}
	score += quality.ConfigCompleteness * 15
	if quality.StructureValid {
		score += 15
	}
	if len(expectedNodeTypes) > 0 {
		score += float64(expectedFound) / float64(len(expectedNodeTypes)) * 10
	}

	quality.Score = score
	return quality
}

// printGenerationQuality print generation quality assessment
func printGenerationQuality(t *testing.T, quality *GenerationQuality) {
	t.Helper()
	t.Logf("=== Generate Quality Assessment ===")
	t.Logf("Analysis successful: %v", quality.ParseSuccess)
	t.Logf("Rule chain creation: %v", quality.ChainCreated)
	t.Logf("Number of nodes: %d", quality.NodeCount)
	t.Logf("Number of connections: %d", quality.ConnectionCount)
	t.Logf("There is a starting node: %v", quality.HasStartNode)
	t.Logf("There is an end node: %v", quality.HasEndNode)
	t.Logf("Configuration completeness: %.1f%%", quality.ConfigCompleteness*100)
	t.Logf("Effective structure: %v", quality.StructureValid)
	t.Logf("Overall score: %.1f/100", quality.Score)

	if quality.Score >= 80 {
		t.Log("Quality Grade: Excellent ✓")
	} else if quality.Score >= 60 {
		t.Log("Quality grade: Good")
	} else if quality.Score >= 40 {
		t.Log("Quality Grade: Average")
	} else {
		t.Log("Quality Grade: Needs improvement ✗")
	}
}

// validateChainWithEngine validates the rule chain through engine initialization
func validateChainWithEngine(t *testing.T, chainDef map[string]interface{}) error {
	t.Helper()

	// Extract the rule chain definition
	ruleChain, ok := chainDef["ruleChain"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("缺少 ruleChain 字段")
	}

	metadata, ok := chainDef["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("缺少 metadata 字段")
	}

	// Serialized as JSON
	chainJSON, err := json.Marshal(chainDef)
	if err != nil {
		return fmt.Errorf("序列化规则链失败: %v", err)
	}

	// Validate using the rulego engine
	registry := rulego.Registry

	// Analyze the definition of the rule chain
	var def types.RuleChain
	if err := json.Unmarshal(chainJSON, &def); err != nil {
		return fmt.Errorf("解析规则链定义失败: %v", err)
	}

	// Try creating a rule chain instance
	chainId, _ := ruleChain["id"].(string)
	if chainId == "" {
		chainId = "validation_test"
	}

	// Verify whether all node types are registered
	nodes, _ := metadata["nodes"].([]interface{})
	for i, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		nodeType, _ := node["type"].(string)
		if nodeType == "" {
			continue
		}

		// Check if the component is registered - try creating it using NewNode
		_, err := registry.NewNode(nodeType)
		if err != nil {
			return fmt.Errorf("节点[%d] 类型 '%s' 未在引擎中注册: %v", i, nodeType, err)
		}
	}

	// Verify the effectiveness of the connection
	connections, _ := metadata["connections"].([]interface{})
	nodeIds := make(map[string]bool)
	for _, n := range nodes {
		node, _ := n.(map[string]interface{})
		if node != nil {
			nodeId, _ := node["id"].(string)
			if nodeId != "" {
				nodeIds[nodeId] = true
			}
		}
	}

	for i, c := range connections {
		conn, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		fromId, _ := conn["fromId"].(string)
		toId, _ := conn["toId"].(string)

		if fromId != "" && !nodeIds[fromId] {
			return fmt.Errorf("连接[%d] 引用不存在的源节点: %s", i, fromId)
		}
		if toId != "" && !nodeIds[toId] {
			return fmt.Errorf("连接[%d] 引用不存在的目标节点: %s", i, toId)
		}
	}

	t.Log("Engine validation passed: All node types are registered, and connection references are valid")
	return nil
}

// printEngineValidationResult Prints the engine validation result
func printEngineValidationResult(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Logf("Engine validation result: ✗ Failed")
		t.Logf("Mistake: %v", err)
	} else {
		t.Log("Engine validation result: ✓ Passed")
	}
}

// ---- Field Name Validation ----

// componentFieldSpec Component field specification
type componentFieldSpec struct {
	RequiredFields []string          // Required fields
	OptionalFields []string          // Optional fields
	WrongFields    map[string]string // Common error fields -> Correct field mapping
}

// getComponentFieldSpec obtains the component field specification
func getComponentFieldSpec(nodeType string) *componentFieldSpec {
	specs := map[string]*componentFieldSpec{
		"jsFilter": {
			RequiredFields: []string{"jsScript"},
			WrongFields:    map[string]string{"script": "jsScript", "code": "jsScript"},
		},
		"jsTransform": {
			RequiredFields: []string{"jsScript"},
			WrongFields:    map[string]string{"script": "jsScript", "code": "jsScript"},
		},
		"restApiCall": {
			RequiredFields: []string{"restEndpointUrlPattern"},
			OptionalFields: []string{"requestMethod", "headers", "body", "maxParallelRequestsCount"},
			WrongFields:    map[string]string{"url": "restEndpointUrlPattern", "endpoint": "restEndpointUrlPattern"},
		},
		"net": {
			RequiredFields: []string{"server"},
			OptionalFields: []string{"protocol", "connectTimeout", "heartbeatInterval"},
			WrongFields:    map[string]string{"host": "server", "address": "server", "url": "server"},
		},
		"log": {
			RequiredFields: []string{"jsScript"},
			WrongFields:    map[string]string{"script": "jsScript", "message": "jsScript"},
		},
		"x/python": {
			RequiredFields: []string{"script"},
			WrongFields:    map[string]string{"code": "script", "jsScript": "script"},
		},
		"x/redisPub": {
			RequiredFields: []string{"channel"},
			OptionalFields: []string{"server", "password"},
		},
		"x/streamAggregator": {
			RequiredFields: []string{"sql"},
			WrongFields:    map[string]string{"query": "sql"},
		},
	}

	if spec, ok := specs[nodeType]; ok {
		return spec
	}
	return nil
}

// FieldValidationError field validation error
type FieldValidationError struct {
	NodeID       string
	NodeType     string
	WrongField   string
	CorrectField string
}

// validateComponentFields verifies whether the component field name is correct
func validateComponentFields(t *testing.T, chainDef map[string]interface{}) []FieldValidationError {
	t.Helper()
	var errors []FieldValidationError

	metadata, ok := chainDef["metadata"].(map[string]interface{})
	if !ok {
		return errors
	}

	nodes, ok := metadata["nodes"].([]interface{})
	if !ok {
		return errors
	}

	for _, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}

		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)
		config, _ := node["configuration"].(map[string]interface{})

		if config == nil {
			continue
		}

		spec := getComponentFieldSpec(nodeType)
		if spec == nil {
			continue
		}

		// Check for incorrect field names
		for wrongField, correctField := range spec.WrongFields {
			if _, hasWrong := config[wrongField]; hasWrong {
				// Check if the correct fields exist at the same time
				if _, hasCorrect := config[correctField]; !hasCorrect {
					errors = append(errors, FieldValidationError{
						NodeID:       nodeID,
						NodeType:     nodeType,
						WrongField:   wrongField,
						CorrectField: correctField,
					})
				}
			}
		}

		// Check if the required fields exist
		for _, requiredField := range spec.RequiredFields {
			if _, has := config[requiredField]; !has {
				t.Logf("Warning: Node %s (%s) missing required field: %s", nodeID, nodeType, requiredField)
			}
		}
	}

	return errors
}

// printFieldValidationErrors prints field validation errors
func printFieldValidationErrors(t *testing.T, errors []FieldValidationError) {
	t.Helper()
	if len(errors) == 0 {
		t.Log("Field validation result: ✓ Passed - All field names are correct")
		return
	}

	t.Logf("Field validation result: ✗ Failure - %d errors found", len(errors))
	for _, err := range errors {
		t.Logf("✗ Node %s (%s): Uses the error field '%s', which should be '%s'",
			err.NodeID, err.NodeType, err.WrongField, err.CorrectField)
	}
}

// ---- Lite response format validation----

// validateLiteResponseFormat Validates the Lite response format (should only include JSON)
func validateLiteResponseFormat(t *testing.T, content string) (bool, string) {
	t.Helper()

	// Try parsing directly as JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		// Check whether the rule chain structure is included
		if _, hasRuleChain := parsed["ruleChain"]; hasRuleChain {
			return true, "标准规则链格式"
		}
		if meta, hasMeta := parsed["metadata"].(map[string]interface{}); hasMeta {
			if _, hasNodes := meta["nodes"]; hasNodes {
				return true, "包含 metadata.nodes 的规则链"
			}
		}
	}

	// If it's not pure JSON, check if JSON blocks are included
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonContent := content[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonContent), &parsed); err == nil {
			// Check for any extra text
			prefix := strings.TrimSpace(content[:jsonStart])
			suffix := strings.TrimSpace(content[jsonEnd+1:])

			if len(prefix) > 0 || len(suffix) > 0 {
				return false, fmt.Sprintf("JSON 周围有多余文本 (前缀: %d字符, 后缀: %d字符)", len(prefix), len(suffix))
			}
			return true, "嵌入式 JSON"
		}
	}

	return false, "无法解析为有效的规则链 JSON"
}

// ---- Rule Chain Execution Validation ----

// executeRuleChain executes the rule chain via API
func executeRuleChain(t *testing.T, handler http.Handler, token, chainId string, msgData string) (int, string) {
	t.Helper()

	body := map[string]interface{}{
		"data":     msgData,
		"dataType": "JSON",
		"msgType":  "TEST",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/"+chainId+"/execute", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return resp.StatusCode, string(respBody)
}

// ---- Basic Test Cases ----

func TestIntegration_SimpleFilterChain(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	resp := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收温度数据，使用jsFilter节点过滤掉温度低于20的消息。规则链ID设为test_filter。"},
	}, false)

	content := parseResponse(t, resp)
	t.Logf("Agent Response: %s", truncate(content, 500))

	// Use a retry mechanism to wait for the rule chain to be created
	chain := waitForChainWithRetry(t, handler, token, "test_filter", 10)
	if chain == nil {
		t.Fatal("The generated rule chain test_filter not found")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"jsFilter"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify required nodes
	if !hasNodeType(chain, "jsFilter") {
		t.Error("No jsFilter node found in the rule chain")
	} else {
		// Verify the jsFilter configuration
		filterConfig := getNodeConfig(chain, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "temperature") || strings.Contains(script, "msg") {
					t.Log("Validation passed: jsFilter contains relevant script logic")
				} else {
					t.Error("jsFilter The script does not include temperature-related logic")
				}
			}
		}
		t.Log("Verification passed: Includes jsFilter nodes")
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

func TestIntegration_MultiTurn_Refine(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	// Round One: Create the basic rule chain
	resp1 := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收MQTT消息并打印日志。规则链ID设为test_multiturn。"},
	}, false)
	content1 := parseResponse(t, resp1)
	t.Logf("First round of response: %s", truncate(content1, 500))

	chain1 := waitForChainWithRetry(t, handler, token, "test_multiturn", 10)
	if chain1 == nil {
		t.Fatal("Round 1: The generated rule chain was not found")
	}

	// The first round of quality assessment
	quality1 := evaluateGenerationQuality(t, chain1, []string{"log"})
	t.Log("=== First round of generation mass ===")
	printGenerationQuality(t, quality1)

	if !hasNodeType(chain1, "log") {
		t.Error("No log nodes were found in the first round of the rule chain")
	}

	// Round 2: Optimize the rule chain (add jsFilter)
	resp2 := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收MQTT消息并打印日志。规则链ID设为test_multiturn。"},
		{"role": "assistant", "content": content1},
		{"role": "user", "content": "在日志节点前面增加一个jsFilter节点，只处理包含temperature字段的消息。"},
	}, false)
	content2 := parseResponse(t, resp2)
	t.Logf("Second round of response: %s", truncate(content2, 500))

	chain2 := waitForChainWithRetry(t, handler, token, "test_multiturn", 10)
	if chain2 == nil {
		t.Fatal("Second round: No updated rule chain found")
	}

	// Second round of quality assessment
	quality2 := evaluateGenerationQuality(t, chain2, []string{"jsFilter", "log"})
	t.Log("=== Second round of generating mass ===")
	printGenerationQuality(t, quality2)

	// Verify the optimization effect
	if !hasNodeType(chain2, "jsFilter") {
		t.Error("After the second round of modifications, no jsFilter node was found in the rule chain")
	} else {
		// Verify whether the jsFilter configuration includes temperature
		filterConfig := getNodeConfig(chain2, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "temperature") {
					t.Log("Verification passed: jsFilter includes temperature conditions")
				} else {
					t.Error("jsFilter The script does not include temperature conditions")
				}
			}
		}
	}

	// Comparing the quality improvements of the two rounds
	t.Logf("Mass change: %.1f -> %.1f (+%.1f)", quality1.Score, quality2.Score, quality2.Score-quality1.Score)

	// Verify structural integrity
	validation := validateChainStructure(t, chain2)
	printValidationResult(t, validation)

	// Field name validation
	fieldErrors := validateComponentFields(t, chain2)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain2)
	printEngineValidationResult(t, engineErr)

	if !validation.Valid {
		t.Error("The second round of rule chain structure verification failed")
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

// ---- Industrial Scenario Testing ----

func TestIntegration_PidFlowControl(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	prompt := `创建一个规则链，实现以下流程：
1. 使用 net 节点（TCP客户端）读取流量计数据，服务器地址为 192.168.1.100:8080
2. 通过 x/python 节点执行 PID 控制算法，Python脚本实现简单的PID计算
3. 通过 fork 节点进行并行分支
4. 三个分支分别使用 net 节点控制泵1、泵2、泵3的运行频率，地址分别为 192.168.1.201:502、192.168.1.202:502、192.168.1.203:502
规则链ID设为 pid_flow_control。`

	resp := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": prompt},
	}, false)

	content := parseResponse(t, resp)
	t.Logf("Agent Response: %s", truncate(content, 500))

	chain := waitForChainWithRetry(t, handler, token, "pid_flow_control", 10)
	if chain == nil {
		t.Fatal("The generated rule chain pid_flow_control not found")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"net", "x/python", "fork"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Number of nodes to verify net (1 input + 3 outputs = 4)
	netNodes := findNodesByType(chain, "net")
	if len(netNodes) < 4 {
		t.Errorf("Expect at least 4 net nodes, but actually %d", len(netNodes))
	} else {
		t.Logf("Verification passed: %d net nodes", len(netNodes))

		// Verify the net node configuration
		for _, node := range netNodes {
			config, _ := node["configuration"].(map[string]interface{})
			if config != nil {
				if host, ok := config["host"].(string); ok {
					t.Logf("net Node host: %s", host)
				}
			}
		}
	}

	// Verify the x/python node
	if !hasNodeType(chain, "x/python") {
		t.Error("x/python node not found")
	} else {
		pyConfig := getNodeConfig(chain, "x/python")
		if pyConfig != nil {
			if script, ok := pyConfig["script"].(string); ok && script != "" {
				t.Logf("Validation passed: x/python Includes script (length=%d)", len(script))
				// Check whether the script contains PID-related logic
				if strings.Contains(strings.ToLower(script), "pid") ||
					strings.Contains(strings.ToLower(script), "error") ||
					strings.Contains(strings.ToLower(script), "integral") {
					t.Log("Verification passed: Python script contains PID control logic")
				}
			} else {
				t.Error("x/python The node script is empty")
			}
		}
	}

	// Verify fork nodes
	if !hasNodeType(chain, "fork") {
		t.Error("fork node not found")
	}

	// Verify the connection relationship
	metadata, _ := chain["metadata"].(map[string]interface{})
	nodes, _ := metadata["nodes"].([]interface{})
	t.Logf("Total number of nodes in the rule chain: %d (expected > = 5)", len(nodes))
	if len(nodes) < 5 {
		t.Errorf("Expect at least 5 nodes, but actually %d", len(nodes))
	}

	// Field name validation (especially focusing on the server field of the NET node)
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 60 {
		t.Errorf("Generation quality score too low: %.1f/100; industrial scenarios require higher accuracy", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

func TestIntegration_DOSlidingWindow(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	prompt := `创建一个规则链，实现以下流程：
1. 使用 x/streamAggregator 节点，SQL为 SELECT AVG(do) as avg_do FROM stream GROUP BY SlidingWindow('5m','1m')
2. streamAggregator 的 window_event 输出连接到 jsTransform 节点
3. jsTransform 节点：将当前平均值存入全局缓存(GlobalCache)，获取上次平均值，计算差值，把差值放入消息
4. jsFilter 节点：过滤差值大于 2.0 的消息
5. x/redisPub 节点：将告警消息发布到 Redis 的 alarm_channel 频道
规则链ID设为 do_sliding_window。`

	resp := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": prompt},
	}, false)

	content := parseResponse(t, resp)
	t.Logf("Agent Response: %s", truncate(content, 500))

	chain := waitForChainWithRetry(t, handler, token, "do_sliding_window", 10)
	if chain == nil {
		t.Fatal("The generated rule chain do_sliding_window not found")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"x/streamAggregator", "jsTransform", "jsFilter", "x/redisPub"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify streamAggregator configuration
	if !hasNodeType(chain, "x/streamAggregator") {
		t.Error("x/streamAggregator node not found")
	} else {
		saConfig := getNodeConfig(chain, "x/streamAggregator")
		if saConfig != nil {
			sql, _ := saConfig["sql"].(string)
			sqlUpper := strings.ToUpper(sql)
			if !strings.Contains(sqlUpper, "SLIDINGWINDOW") {
				t.Errorf("SlidingWindow: %s is not included in SQL", sql)
			}
			if !strings.Contains(sqlUpper, "AVG") {
				t.Errorf("AVG: %s is not included in SQL", sql)
			}
			if !strings.Contains(sqlUpper, "DO") {
				t.Errorf("DO field not included in SQL: %s", sql)
			}
			t.Logf("Verification passed: streamAggregator SQL = %s", sql)
		} else {
			t.Error("x/streamAggregator node is configured to be empty")
		}
	}

	// Validate other required nodes
	if !hasNodeType(chain, "jsTransform") {
		t.Error("jsTransform node not found")
	}
	if !hasNodeType(chain, "jsFilter") {
		t.Error("jsFilter node not found")
	}
	if !hasNodeType(chain, "x/redisPub") {
		t.Error("x/redisPub node not found")
	} else {
		// Verify the redisPub configuration
		redisConfig := getNodeConfig(chain, "x/redisPub")
		if redisConfig != nil {
			if channel, ok := redisConfig["channel"].(string); ok {
				if strings.Contains(channel, "alarm") {
					t.Logf("Verification passed: redisPub channel = %s", channel)
				}
			}
		}
	}

	// Verify the connection relationship: streamAggregator -> jsTransform
	metadata, _ := chain["metadata"].(map[string]interface{})
	connections, _ := metadata["connections"].([]interface{})
	saNodes := findNodesByType(chain, "x/streamAggregator")
	if len(saNodes) > 0 {
		saId, _ := saNodes[0]["id"].(string)
		hasConnection := false
		for _, c := range connections {
			conn, _ := c.(map[string]interface{})
			if conn["fromId"] == saId {
				hasConnection = true
				t.Logf("streamAggregator(%s) -> %s (type=%s)", saId, conn["toId"], conn["type"])
			}
		}
		if !hasConnection {
			t.Error("streamAggregator No output connection")
		}
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check (flow vector aggregation scenarios are more complex)
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

// ---- Generator Lite Test ----

// chatRequestLite sends chat requests to generator-lite endpoints (one-time generation, no tool calls)
func chatRequestLite(t *testing.T, handler http.Handler, token string, messages []map[string]string) *http.Response {
	t.Helper()
	body := map[string]interface{}{
		"messages": messages,
		"stream":   false,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/generator-lite/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result()
}

// extractRuleChainFromText extracts the rule chain JSON from AI text responses
func extractRuleChainFromText(text string) map[string]interface{} {
	// Collect candidate JSON snippets: full text + all top-level {} objects
	candidates := []string{text}
	searchText := text
	for {
		idx := strings.Index(searchText, "{")
		if idx == -1 {
			break
		}
		depth := 0
		for i := idx; i < len(searchText); i++ {
			if searchText[i] == '{' {
				depth++
			} else if searchText[i] == '}' {
				depth--
			}
			if depth == 0 {
				candidates = append(candidates, searchText[idx:i+1])
				break
			}
		}
		if len(searchText) > idx+1 {
			searchText = searchText[idx+1:]
		} else {
			break
		}
	}

	var parsed map[string]interface{}
	for _, c := range candidates {
		if err := json.Unmarshal([]byte(c), &parsed); err != nil {
			continue
		}
		if _, ok := parsed["ruleChain"]; ok {
			return parsed
		}
		// No ruleChain wrapper but metadata.nodes → automatic wrapping
		if meta, ok := parsed["metadata"].(map[string]interface{}); ok {
			if _, hasNodes := meta["nodes"]; hasNodes {
				return normalizeChain(parsed)
			}
		}
	}
	return nil
}

// normalizeChain packages non-standard structures into standard regular chain formats
func normalizeChain(raw map[string]interface{}) map[string]interface{} {
	rc := map[string]interface{}{
		"id":             raw["id"],
		"name":           raw["name"],
		"debugMode":      false,
		"root":           false,
		"disabled":       false,
		"additionalInfo": raw["additionalInfo"],
	}
	if rc["id"] == nil {
		rc["id"] = ""
	}
	if rc["name"] == nil {
		rc["name"] = ""
	}
	if rc["additionalInfo"] == nil {
		rc["additionalInfo"] = map[string]interface{}{}
	}
	return map[string]interface{}{
		"ruleChain": rc,
		"metadata":  raw["metadata"],
	}
}

func TestLite_SimpleFilterChain(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	resp := chatRequestLite(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收温度数据，使用jsFilter节点过滤掉温度低于20的消息。规则链ID设为test_lite_filter。"},
	})

	content := parseResponse(t, resp)
	t.Logf("Lite Response length: %d characters", len(content))

	// Verify the Lite response format (should include only JSON, no extra text)
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite Response format verification: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite Response format does not meet requirements: Only the rule chain JSON should be returned, and unnecessary text should not be included")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Logf("Lite Full response text: %s", content)
		t.Fatal("Failure to extract the rule chain JSON from Lite responses")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"jsFilter"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify the jsFilter node
	nodes := findNodesByType(chain, "jsFilter")
	if len(nodes) == 0 {
		t.Error("No jsFilter node found in the rule chain")
	} else {
		config, ok := nodes[0]["configuration"].(map[string]interface{})
		if !ok || config == nil {
			t.Error("jsFilter node is configured to be empty")
		} else {
			script, _ := config["jsScript"].(string)
			t.Logf("jsFilter Script: %s", truncate(script, 300))
			if !strings.Contains(script, "temperature") {
				t.Error("jsFilter The script does not include temperature conditions")
			} else {
				t.Log("Verification passed: jsFilter includes temperature conditions")
			}
		}
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

func TestLite_SerialPipeline(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	resp := chatRequestLite(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链：接收传感器数据 → jsTransform添加时间戳和设备ID → jsFilter过滤temperature>30 → restApiCall发送告警到http://alert.example.com/notify。规则链ID设为test_lite_pipeline。"},
	})

	content := parseResponse(t, resp)
	t.Logf("Lite Response length: %d characters", len(content))

	// Verify the Lite response format (should include only JSON, no extra text)
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite Response format verification: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite Response format does not meet requirements: Only the rule chain JSON should be returned, and unnecessary text should not be included")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("Failure to extract the rule chain JSON from Lite responses")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"jsTransform", "jsFilter", "restApiCall"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify required nodes
	if !hasNodeType(chain, "jsTransform") {
		t.Error("jsTransform node not found")
	}
	if !hasNodeType(chain, "jsFilter") {
		t.Error("jsFilter node not found")
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("restApiCall node not found")
	} else {
		// Verify the restApiCall configuration
		restConfig := getNodeConfig(chain, "restApiCall")
		if restConfig != nil {
			// Check whether the correct field names are being used
			if url, hasURL := restConfig["restEndpointUrlPattern"]; hasURL {
				t.Logf("Validation passed: restApiCall use restEndpointUrlPattern = %v", url)
			} else if _, hasURL2 := restConfig["url"]; hasURL2 {
				t.Error("restApiCall uses 'url' instead of 'restEndpointUrlPattern'")
			}
		}
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

func TestLite_ParallelWithForkJoin(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	resp := chatRequestLite(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链：接收设备数据 → fork并行 → 分支A: jsTransform转换为摄氏度 → 分支B: restApiCall发送到http://api.example.com/data → 分支C: log记录日志 → join聚合 → end。规则链ID设为test_lite_parallel。"},
	})

	content := parseResponse(t, resp)
	t.Logf("Lite Response length: %d characters", len(content))

	// Verify the Lite response format (should include only JSON, no extra text)
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite Response format verification: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite Response format does not meet requirements: Only the rule chain JSON should be returned, and unnecessary text should not be included")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("Failure to extract the rule chain JSON from Lite responses")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"fork", "join", "jsTransform", "restApiCall", "log"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify required nodes
	if !hasNodeType(chain, "fork") {
		t.Error("fork node not found")
	}
	if !hasNodeType(chain, "join") {
		t.Error("join node not found")
	}
	if !hasNodeType(chain, "jsTransform") {
		t.Error("jsTransform node not found")
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("restApiCall node not found")
	}
	if !hasNodeType(chain, "log") {
		t.Error("log node not found")
	}

	// Verify that fork and join are paired
	forkCount := len(findNodesByType(chain, "fork"))
	joinCount := len(findNodesByType(chain, "join"))
	if forkCount != joinCount {
		t.Errorf("fork (%d) and join (%d) numbers do not match", forkCount, joinCount)
	} else {
		t.Logf("Verification passed: fork (%d) and join (%d) paired", forkCount, joinCount)
	}

	// Verifying connection relationships: forks should have multiple output connections
	metadata, _ := chain["metadata"].(map[string]interface{})
	connections, _ := metadata["connections"].([]interface{})
	forkNodes := findNodesByType(chain, "fork")
	if len(forkNodes) > 0 {
		forkId, _ := forkNodes[0]["id"].(string)
		outputCount := 0
		for _, c := range connections {
			conn, _ := c.(map[string]interface{})
			if conn["fromId"] == forkId {
				outputCount++
			}
		}
		if outputCount >= 3 {
			t.Logf("Verification passed: fork node has %d output branches", outputCount)
		} else {
			t.Errorf("fork Node outputs insufficient branches: Expected > = 3, actual %d", outputCount)
		}
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}

func TestLite_ConditionalBranch(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	resp := chatRequestLite(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链：接收订单数据 → jsFilter判断金额是否大于1000 → True: restApiCall发送VIP通知到http://vip.example.com/notify → False: log记录普通订单日志。规则链ID设为test_lite_branch。"},
	})

	content := parseResponse(t, resp)
	t.Logf("Lite Response length: %d characters", len(content))

	// Verify the Lite response format (should include only JSON, no extra text)
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite Response format verification: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite Response format does not meet requirements: Only the rule chain JSON should be returned, and unnecessary text should not be included")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("Failure to extract the rule chain JSON from Lite responses")
	}

	// Verify structural integrity
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// Evaluate the quality of the generation
	expectedTypes := []string{"jsFilter", "restApiCall", "log"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// Verify required nodes
	if !hasNodeType(chain, "jsFilter") {
		t.Error("jsFilter node not found")
	} else {
		// Verify the jsFilter configuration
		filterConfig := getNodeConfig(chain, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "amount") || strings.Contains(script, "1000") {
					t.Log("Verification passed: jsFilter includes amount determination logic")
				}
			}
		}
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("restApiCall node not found")
	}
	if !hasNodeType(chain, "log") {
		t.Error("log node not found")
	}

	// Verify True and False connections
	metadata, _ := chain["metadata"].(map[string]interface{})
	connections, _ := metadata["connections"].([]interface{})
	filterNodes := findNodesByType(chain, "jsFilter")
	if len(filterNodes) > 0 {
		filterId, _ := filterNodes[0]["id"].(string)
		hasTrue, hasFalse := false, false
		for _, c := range connections {
			conn, _ := c.(map[string]interface{})
			if conn["fromId"] == filterId {
				connType, _ := conn["type"].(string)
				if connType == "True" {
					hasTrue = true
				}
				if connType == "False" {
					hasFalse = true
				}
			}
		}
		if !hasTrue {
			t.Error("jsFilter Lack of True connections")
		}
		if !hasFalse {
			t.Error("jsFilter Lack of False connections")
		}
		if hasTrue && hasFalse {
			t.Log("Verification passed: jsFilter includes True and False branch connections")
		}
	}

	// Field name validation
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// Engine validation
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// Quality threshold check
	if quality.Score < 50 {
		t.Errorf("Generation quality score too low: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("There is an error in the field name; please check the generated result")
	}
}
