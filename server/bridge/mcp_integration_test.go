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
	// 注册 AI 组件
	_ "github.com/rulego/rulego-components-ai/agent"
	_ "github.com/rulego/rulego-components-ai/processor"
)

// skipIfNoLLMKey 如果没有设置 LLM_API_KEY 则跳过测试
func skipIfNoLLMKey(t *testing.T) {
	t.Helper()
	if os.Getenv("LLM_API_KEY") == "" {
		t.Skip("跳过集成测试：未设置 LLM_API_KEY 环境变量")
	}
}

// newIntegrationBridge 创建集成测试用的 Bridge。
// 使用干净的临时数据目录，仅复制 system/agents。
func newIntegrationBridge(t *testing.T) *Bridge {
	t.Helper()

	// 创建干净的临时数据目录，避免旧链干扰
	tmpData := t.TempDir()
	srcData := os.Getenv("RULEGO_DATA_DIR")
	if srcData == "" {
		srcData, _ = filepath.Abs(filepath.Join("..", "data"))
	}
	copyDir(t, filepath.Join(srcData, "system"), filepath.Join(tmpData, "system"))
	// 兼容旧版路径：同时拷贝 agents 到 data_dir/agents/（部分旧配置使用 global.data_dir + '/agents/'）
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
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 修复 generator-lite 中的参数兼容性（某些模型不支持 frequencyPenalty/presencePenalty）
	patchAgentParams(t, filepath.Join(tmpData, "system", "agents", "generator-lite", "generator-lite.json"))
	patchAgentParams(t, filepath.Join(tmpData, "system", "agents", "generator", "generator.json"))

	br, err := NewBridgeWithDefaults(cfgFile)
	if err != nil {
		t.Fatalf("创建 Bridge 失败: %v", err)
	}
	return br
}

// patchAgentParams 修复 agent 配置中的参数兼容性
func patchAgentParams(t *testing.T, jsonPath string) {
	t.Helper()
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Logf("patchAgentParams: 跳过 %s: %v", jsonPath, err)
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Logf("patchAgentParams: 解析失败 %s: %v", jsonPath, err)
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
		// 某些模型不支持这些参数，删除避免报错
		delete(params, "frequencyPenalty")
		delete(params, "presencePenalty")
		t.Logf("patchAgentParams: 已清理 %s 中节点 %v 的 penalty 参数", jsonPath, node["id"])
	}
	patched, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(jsonPath, patched, 0644); err != nil {
		t.Logf("patchAgentParams: 写入失败 %s: %v", jsonPath, err)
	}
}

// copyDir 递归复制目录
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

// chatRequest 发送聊天请求到 generator 端点
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

// getRuleChainViaAPI 通过 REST API 获取已保存的规则链
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

// parseResponse 从非流式响应中提取最终消息内容
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

// ---- 验证辅助函数 ----

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

// ---- 增强验证辅助函数 ----

// ValidationResult 验证结果汇总
type ValidationResult struct {
	Valid            bool
	TotalNodes       int
	TotalConnections int
	NodeTypes        map[string]int
	Errors           []string
	Warnings         []string
}

// validateChainStructure 验证规则链结构完整性
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

	// 验证节点
	nodes, ok := metadata["nodes"].([]interface{})
	if !ok {
		result.Valid = false
		result.Errors = append(result.Errors, "metadata.nodes 不是数组或不存在")
		return result
	}
	result.TotalNodes = len(nodes)

	// 收集所有节点 ID 和类型，检查唯一性
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

		// 验证节点名称
		if _, hasName := node["name"]; !hasName {
			result.Warnings = append(result.Warnings, fmt.Sprintf("节点 %s 缺少 name", nodeID))
		}
	}

	// 验证连接
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

// printValidationResult 打印验证结果详情
func printValidationResult(t *testing.T, result *ValidationResult) {
	t.Helper()
	t.Logf("=== 规则链验证结果 ===")
	t.Logf("总节点数: %d", result.TotalNodes)
	t.Logf("总连接数: %d", result.TotalConnections)
	t.Logf("节点类型分布:")
	for nodeType, count := range result.NodeTypes {
		t.Logf("  - %s: %d", nodeType, count)
	}

	if len(result.Errors) > 0 {
		t.Logf("错误 (%d):", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("  ✗ %s", err)
		}
	}

	if len(result.Warnings) > 0 {
		t.Logf("警告 (%d):", len(result.Warnings))
		for _, warn := range result.Warnings {
			t.Logf("  ⚠ %s", warn)
		}
	}

	if result.Valid {
		t.Log("验证结果: ✓ 通过")
	} else {
		t.Log("验证结果: ✗ 失败")
	}
}

// waitForChainWithRetry 等待规则链创建完成（带重试）
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

// GenerationQuality 生成质量评估
type GenerationQuality struct {
	ParseSuccess       bool    // 是否成功解析
	ChainCreated       bool    // 规则链是否创建
	NodeCount          int     // 节点数量
	ConnectionCount    int     // 连接数量
	HasStartNode       bool    // 是否有起始节点
	HasEndNode         bool    // 是否有结束节点
	ConfigCompleteness float64 // 配置完整度 (0-1)
	StructureValid     bool    // 结构是否有效
	Score              float64 // 综合得分 (0-100)
}

// evaluateGenerationQuality 评估生成质量
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

	// 检查起始和结束节点
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

	// 计算配置完整度
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

	// 验证结构
	validation := validateChainStructure(t, chainDef)
	quality.StructureValid = validation.Valid

	// 检查期望的节点类型
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

	// 计算综合得分
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

// printGenerationQuality 打印生成质量评估
func printGenerationQuality(t *testing.T, quality *GenerationQuality) {
	t.Helper()
	t.Logf("=== 生成质量评估 ===")
	t.Logf("解析成功: %v", quality.ParseSuccess)
	t.Logf("规则链创建: %v", quality.ChainCreated)
	t.Logf("节点数量: %d", quality.NodeCount)
	t.Logf("连接数量: %d", quality.ConnectionCount)
	t.Logf("有起始节点: %v", quality.HasStartNode)
	t.Logf("有结束节点: %v", quality.HasEndNode)
	t.Logf("配置完整度: %.1f%%", quality.ConfigCompleteness*100)
	t.Logf("结构有效: %v", quality.StructureValid)
	t.Logf("综合得分: %.1f/100", quality.Score)

	if quality.Score >= 80 {
		t.Log("质量等级: 优秀 ✓")
	} else if quality.Score >= 60 {
		t.Log("质量等级: 良好")
	} else if quality.Score >= 40 {
		t.Log("质量等级: 一般")
	} else {
		t.Log("质量等级: 需改进 ✗")
	}
}

// validateChainWithEngine 通过引擎初始化验证规则链
func validateChainWithEngine(t *testing.T, chainDef map[string]interface{}) error {
	t.Helper()

	// 提取规则链定义
	ruleChain, ok := chainDef["ruleChain"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("缺少 ruleChain 字段")
	}

	metadata, ok := chainDef["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("缺少 metadata 字段")
	}

	// 序列化为 JSON
	chainJSON, err := json.Marshal(chainDef)
	if err != nil {
		return fmt.Errorf("序列化规则链失败: %v", err)
	}

	// 使用 rulego 引擎验证
	registry := rulego.Registry

	// 解析规则链定义
	var def types.RuleChain
	if err := json.Unmarshal(chainJSON, &def); err != nil {
		return fmt.Errorf("解析规则链定义失败: %v", err)
	}

	// 尝试创建规则链实例
	chainId, _ := ruleChain["id"].(string)
	if chainId == "" {
		chainId = "validation_test"
	}

	// 验证节点类型是否都已注册
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

		// 检查组件是否已注册 - 使用 NewNode 尝试创建
		_, err := registry.NewNode(nodeType)
		if err != nil {
			return fmt.Errorf("节点[%d] 类型 '%s' 未在引擎中注册: %v", i, nodeType, err)
		}
	}

	// 验证连接的有效性
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

	t.Log("引擎验证通过: 所有节点类型已注册，连接引用有效")
	return nil
}

// printEngineValidationResult 打印引擎验证结果
func printEngineValidationResult(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Logf("引擎验证结果: ✗ 失败")
		t.Logf("  错误: %v", err)
	} else {
		t.Log("引擎验证结果: ✓ 通过")
	}
}

// ---- 字段名验证 ----

// componentFieldSpec 组件字段规范
type componentFieldSpec struct {
	RequiredFields []string          // 必需字段
	OptionalFields []string          // 可选字段
	WrongFields    map[string]string // 常见错误字段 -> 正确字段映射
}

// getComponentFieldSpec 获取组件字段规范
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

// FieldValidationError 字段验证错误
type FieldValidationError struct {
	NodeID       string
	NodeType     string
	WrongField   string
	CorrectField string
}

// validateComponentFields 验证组件字段名是否正确
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

		// 检查是否有错误字段名
		for wrongField, correctField := range spec.WrongFields {
			if _, hasWrong := config[wrongField]; hasWrong {
				// 检查是否同时有正确字段
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

		// 检查必需字段是否存在
		for _, requiredField := range spec.RequiredFields {
			if _, has := config[requiredField]; !has {
				t.Logf("警告: 节点 %s (%s) 缺少必需字段: %s", nodeID, nodeType, requiredField)
			}
		}
	}

	return errors
}

// printFieldValidationErrors 打印字段验证错误
func printFieldValidationErrors(t *testing.T, errors []FieldValidationError) {
	t.Helper()
	if len(errors) == 0 {
		t.Log("字段验证结果: ✓ 通过 - 所有字段名正确")
		return
	}

	t.Logf("字段验证结果: ✗ 失败 - 发现 %d 个错误", len(errors))
	for _, err := range errors {
		t.Logf("  ✗ 节点 %s (%s): 使用了错误字段 '%s'，应为 '%s'",
			err.NodeID, err.NodeType, err.WrongField, err.CorrectField)
	}
}

// ---- Lite 响应格式验证 ----

// validateLiteResponseFormat 验证 Lite 响应格式（应该只包含 JSON）
func validateLiteResponseFormat(t *testing.T, content string) (bool, string) {
	t.Helper()

	// 尝试直接解析为 JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		// 检查是否包含规则链结构
		if _, hasRuleChain := parsed["ruleChain"]; hasRuleChain {
			return true, "标准规则链格式"
		}
		if meta, hasMeta := parsed["metadata"].(map[string]interface{}); hasMeta {
			if _, hasNodes := meta["nodes"]; hasNodes {
				return true, "包含 metadata.nodes 的规则链"
			}
		}
	}

	// 如果不是纯 JSON，检查是否包含 JSON 块
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonContent := content[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonContent), &parsed); err == nil {
			// 检查是否有额外的文本
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

// ---- 规则链运行验证 ----

// executeRuleChain 通过 API 执行规则链
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

// ---- 基础测试用例 ----

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
	t.Logf("Agent 响应: %s", truncate(content, 500))

	// 使用重试机制等待规则链创建
	chain := waitForChainWithRetry(t, handler, token, "test_filter", 10)
	if chain == nil {
		t.Fatal("未找到生成的规则链 test_filter")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"jsFilter"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证必需节点
	if !hasNodeType(chain, "jsFilter") {
		t.Error("规则链中未找到 jsFilter 节点")
	} else {
		// 验证 jsFilter 配置
		filterConfig := getNodeConfig(chain, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "temperature") || strings.Contains(script, "msg") {
					t.Log("验证通过: jsFilter 包含相关脚本逻辑")
				} else {
					t.Error("jsFilter 脚本中未包含 temperature 相关逻辑")
				}
			}
		}
		t.Log("验证通过: 包含 jsFilter 节点")
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
	}
}

func TestIntegration_MultiTurn_Refine(t *testing.T) {
	skipIfNoLLMKey(t)
	br := newIntegrationBridge(t)
	defer br.Stop()

	handler := br.Handler()
	token := loginAndGetToken(t, br)

	// 第一轮：创建基础规则链
	resp1 := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收MQTT消息并打印日志。规则链ID设为test_multiturn。"},
	}, false)
	content1 := parseResponse(t, resp1)
	t.Logf("第一轮响应: %s", truncate(content1, 500))

	chain1 := waitForChainWithRetry(t, handler, token, "test_multiturn", 10)
	if chain1 == nil {
		t.Fatal("第一轮：未找到生成的规则链")
	}

	// 第一轮质量评估
	quality1 := evaluateGenerationQuality(t, chain1, []string{"log"})
	t.Log("=== 第一轮生成质量 ===")
	printGenerationQuality(t, quality1)

	if !hasNodeType(chain1, "log") {
		t.Error("第一轮规则链中未找到 log 节点")
	}

	// 第二轮：优化规则链（增加 jsFilter）
	resp2 := chatRequest(t, handler, token, []map[string]string{
		{"role": "user", "content": "创建一个规则链，接收MQTT消息并打印日志。规则链ID设为test_multiturn。"},
		{"role": "assistant", "content": content1},
		{"role": "user", "content": "在日志节点前面增加一个jsFilter节点，只处理包含temperature字段的消息。"},
	}, false)
	content2 := parseResponse(t, resp2)
	t.Logf("第二轮响应: %s", truncate(content2, 500))

	chain2 := waitForChainWithRetry(t, handler, token, "test_multiturn", 10)
	if chain2 == nil {
		t.Fatal("第二轮：未找到更新的规则链")
	}

	// 第二轮质量评估
	quality2 := evaluateGenerationQuality(t, chain2, []string{"jsFilter", "log"})
	t.Log("=== 第二轮生成质量 ===")
	printGenerationQuality(t, quality2)

	// 验证优化效果
	if !hasNodeType(chain2, "jsFilter") {
		t.Error("第二轮修改后规则链中未找到 jsFilter 节点")
	} else {
		// 验证 jsFilter 配置是否包含 temperature
		filterConfig := getNodeConfig(chain2, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "temperature") {
					t.Log("验证通过: jsFilter 包含 temperature 条件")
				} else {
					t.Error("jsFilter 脚本中未包含 temperature 条件")
				}
			}
		}
	}

	// 比较两轮的质量提升
	t.Logf("质量变化: %.1f -> %.1f (+%.1f)", quality1.Score, quality2.Score, quality2.Score-quality1.Score)

	// 验证结构完整性
	validation := validateChainStructure(t, chain2)
	printValidationResult(t, validation)

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain2)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain2)
	printEngineValidationResult(t, engineErr)

	if !validation.Valid {
		t.Error("第二轮规则链结构验证失败")
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
	}
}

// ---- 工业场景测试 ----

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
	t.Logf("Agent 响应: %s", truncate(content, 500))

	chain := waitForChainWithRetry(t, handler, token, "pid_flow_control", 10)
	if chain == nil {
		t.Fatal("未找到生成的规则链 pid_flow_control")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"net", "x/python", "fork"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证 net 节点数量（1个输入 + 3个输出 = 4个）
	netNodes := findNodesByType(chain, "net")
	if len(netNodes) < 4 {
		t.Errorf("期望至少 4 个 net 节点，实际 %d 个", len(netNodes))
	} else {
		t.Logf("验证通过: %d 个 net 节点", len(netNodes))

		// 验证 net 节点配置
		for _, node := range netNodes {
			config, _ := node["configuration"].(map[string]interface{})
			if config != nil {
				if host, ok := config["host"].(string); ok {
					t.Logf("  net 节点 host: %s", host)
				}
			}
		}
	}

	// 验证 x/python 节点
	if !hasNodeType(chain, "x/python") {
		t.Error("未找到 x/python 节点")
	} else {
		pyConfig := getNodeConfig(chain, "x/python")
		if pyConfig != nil {
			if script, ok := pyConfig["script"].(string); ok && script != "" {
				t.Logf("验证通过: x/python 包含脚本 (长度=%d)", len(script))
				// 检查脚本是否包含 PID 相关逻辑
				if strings.Contains(strings.ToLower(script), "pid") ||
					strings.Contains(strings.ToLower(script), "error") ||
					strings.Contains(strings.ToLower(script), "integral") {
					t.Log("验证通过: Python 脚本包含 PID 控制逻辑")
				}
			} else {
				t.Error("x/python 节点脚本为空")
			}
		}
	}

	// 验证 fork 节点
	if !hasNodeType(chain, "fork") {
		t.Error("未找到 fork 节点")
	}

	// 验证连接关系
	metadata, _ := chain["metadata"].(map[string]interface{})
	nodes, _ := metadata["nodes"].([]interface{})
	t.Logf("规则链总节点数: %d (期望 >= 5)", len(nodes))
	if len(nodes) < 5 {
		t.Errorf("期望至少 5 个节点，实际 %d 个", len(nodes))
	}

	// 字段名验证（特别关注 net 节点的 server 字段）
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 60 {
		t.Errorf("生成质量得分过低: %.1f/100，工业场景需要更高准确度", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
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
	t.Logf("Agent 响应: %s", truncate(content, 500))

	chain := waitForChainWithRetry(t, handler, token, "do_sliding_window", 10)
	if chain == nil {
		t.Fatal("未找到生成的规则链 do_sliding_window")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"x/streamAggregator", "jsTransform", "jsFilter", "x/redisPub"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证 streamAggregator 配置
	if !hasNodeType(chain, "x/streamAggregator") {
		t.Error("未找到 x/streamAggregator 节点")
	} else {
		saConfig := getNodeConfig(chain, "x/streamAggregator")
		if saConfig != nil {
			sql, _ := saConfig["sql"].(string)
			sqlUpper := strings.ToUpper(sql)
			if !strings.Contains(sqlUpper, "SLIDINGWINDOW") {
				t.Errorf("SQL 中未包含 SlidingWindow: %s", sql)
			}
			if !strings.Contains(sqlUpper, "AVG") {
				t.Errorf("SQL 中未包含 AVG: %s", sql)
			}
			if !strings.Contains(sqlUpper, "DO") {
				t.Errorf("SQL 中未包含 DO 字段: %s", sql)
			}
			t.Logf("验证通过: streamAggregator SQL = %s", sql)
		} else {
			t.Error("x/streamAggregator 节点配置为空")
		}
	}

	// 验证其他必需节点
	if !hasNodeType(chain, "jsTransform") {
		t.Error("未找到 jsTransform 节点")
	}
	if !hasNodeType(chain, "jsFilter") {
		t.Error("未找到 jsFilter 节点")
	}
	if !hasNodeType(chain, "x/redisPub") {
		t.Error("未找到 x/redisPub 节点")
	} else {
		// 验证 redisPub 配置
		redisConfig := getNodeConfig(chain, "x/redisPub")
		if redisConfig != nil {
			if channel, ok := redisConfig["channel"].(string); ok {
				if strings.Contains(channel, "alarm") {
					t.Logf("验证通过: redisPub channel = %s", channel)
				}
			}
		}
	}

	// 验证连接关系：streamAggregator -> jsTransform
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
			t.Error("streamAggregator 没有输出连接")
		}
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查（流式聚合场景较复杂）
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
	}
}

// ---- Generator Lite 测试 ----

// chatRequestLite 发送聊天请求到 generator-lite 端点（单次生成，无工具调用）
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

// extractRuleChainFromText 从 AI 文本响应中提取规则链 JSON
func extractRuleChainFromText(text string) map[string]interface{} {
	// 收集候选 JSON 片段：全文 + 所有顶层 {} 对象
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
		// 没有ruleChain包装但有metadata.nodes → 自动包装
		if meta, ok := parsed["metadata"].(map[string]interface{}); ok {
			if _, hasNodes := meta["nodes"]; hasNodes {
				return normalizeChain(parsed)
			}
		}
	}
	return nil
}

// normalizeChain 将非标准结构包装为标准规则链格式
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
	t.Logf("Lite 响应长度: %d 字符", len(content))

	// 验证 Lite 响应格式（应该只包含 JSON，无多余文本）
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite 响应格式验证: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite 响应格式不符合要求：应只返回规则链 JSON，不应包含多余文本")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Logf("Lite 响应全文: %s", content)
		t.Fatal("未能从 Lite 响应中提取规则链 JSON")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"jsFilter"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证 jsFilter 节点
	nodes := findNodesByType(chain, "jsFilter")
	if len(nodes) == 0 {
		t.Error("规则链中未找到 jsFilter 节点")
	} else {
		config, ok := nodes[0]["configuration"].(map[string]interface{})
		if !ok || config == nil {
			t.Error("jsFilter 节点配置为空")
		} else {
			script, _ := config["jsScript"].(string)
			t.Logf("jsFilter 脚本: %s", truncate(script, 300))
			if !strings.Contains(script, "temperature") {
				t.Error("jsFilter 脚本中未包含 temperature 条件")
			} else {
				t.Log("验证通过: jsFilter 包含 temperature 条件")
			}
		}
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
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
	t.Logf("Lite 响应长度: %d 字符", len(content))

	// 验证 Lite 响应格式（应该只包含 JSON，无多余文本）
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite 响应格式验证: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite 响应格式不符合要求：应只返回规则链 JSON，不应包含多余文本")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("未能从 Lite 响应中提取规则链 JSON")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"jsTransform", "jsFilter", "restApiCall"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证必需节点
	if !hasNodeType(chain, "jsTransform") {
		t.Error("未找到 jsTransform 节点")
	}
	if !hasNodeType(chain, "jsFilter") {
		t.Error("未找到 jsFilter 节点")
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("未找到 restApiCall 节点")
	} else {
		// 验证 restApiCall 配置
		restConfig := getNodeConfig(chain, "restApiCall")
		if restConfig != nil {
			// 检查是否使用了正确的字段名
			if url, hasURL := restConfig["restEndpointUrlPattern"]; hasURL {
				t.Logf("验证通过: restApiCall 使用 restEndpointUrlPattern = %v", url)
			} else if _, hasURL2 := restConfig["url"]; hasURL2 {
				t.Error("restApiCall 使用了 'url' 而非 'restEndpointUrlPattern'")
			}
		}
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
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
	t.Logf("Lite 响应长度: %d 字符", len(content))

	// 验证 Lite 响应格式（应该只包含 JSON，无多余文本）
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite 响应格式验证: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite 响应格式不符合要求：应只返回规则链 JSON，不应包含多余文本")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("未能从 Lite 响应中提取规则链 JSON")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"fork", "join", "jsTransform", "restApiCall", "log"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证必需节点
	if !hasNodeType(chain, "fork") {
		t.Error("未找到 fork 节点")
	}
	if !hasNodeType(chain, "join") {
		t.Error("未找到 join 节点")
	}
	if !hasNodeType(chain, "jsTransform") {
		t.Error("未找到 jsTransform 节点")
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("未找到 restApiCall 节点")
	}
	if !hasNodeType(chain, "log") {
		t.Error("未找到 log 节点")
	}

	// 验证 fork 和 join 成对
	forkCount := len(findNodesByType(chain, "fork"))
	joinCount := len(findNodesByType(chain, "join"))
	if forkCount != joinCount {
		t.Errorf("fork(%d) 和 join(%d) 数量不匹配", forkCount, joinCount)
	} else {
		t.Logf("验证通过: fork(%d) 和 join(%d) 成对", forkCount, joinCount)
	}

	// 验证连接关系：fork 应该有多个输出连接
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
			t.Logf("验证通过: fork 节点有 %d 个输出分支", outputCount)
		} else {
			t.Errorf("fork 节点输出分支不足: 期望 >= 3，实际 %d", outputCount)
		}
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
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
	t.Logf("Lite 响应长度: %d 字符", len(content))

	// 验证 Lite 响应格式（应该只包含 JSON，无多余文本）
	isValidFormat, formatDesc := validateLiteResponseFormat(t, content)
	t.Logf("Lite 响应格式验证: %v - %s", isValidFormat, formatDesc)
	if !isValidFormat {
		t.Error("Lite 响应格式不符合要求：应只返回规则链 JSON，不应包含多余文本")
	}

	chain := extractRuleChainFromText(content)
	if chain == nil {
		t.Fatal("未能从 Lite 响应中提取规则链 JSON")
	}

	// 验证结构完整性
	validation := validateChainStructure(t, chain)
	printValidationResult(t, validation)

	// 评估生成质量
	expectedTypes := []string{"jsFilter", "restApiCall", "log"}
	quality := evaluateGenerationQuality(t, chain, expectedTypes)
	printGenerationQuality(t, quality)

	// 验证必需节点
	if !hasNodeType(chain, "jsFilter") {
		t.Error("未找到 jsFilter 节点")
	} else {
		// 验证 jsFilter 配置
		filterConfig := getNodeConfig(chain, "jsFilter")
		if filterConfig != nil {
			if script, ok := filterConfig["jsScript"].(string); ok {
				if strings.Contains(script, "amount") || strings.Contains(script, "1000") {
					t.Log("验证通过: jsFilter 包含金额判断逻辑")
				}
			}
		}
	}
	if !hasNodeType(chain, "restApiCall") {
		t.Error("未找到 restApiCall 节点")
	}
	if !hasNodeType(chain, "log") {
		t.Error("未找到 log 节点")
	}

	// 验证 True 和 False 连接
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
			t.Error("jsFilter 缺少 True 连接")
		}
		if !hasFalse {
			t.Error("jsFilter 缺少 False 连接")
		}
		if hasTrue && hasFalse {
			t.Log("验证通过: jsFilter 包含 True 和 False 分支连接")
		}
	}

	// 字段名验证
	fieldErrors := validateComponentFields(t, chain)
	printFieldValidationErrors(t, fieldErrors)

	// 引擎验证
	engineErr := validateChainWithEngine(t, chain)
	printEngineValidationResult(t, engineErr)

	// 质量阈值检查
	if quality.Score < 50 {
		t.Errorf("生成质量得分过低: %.1f/100", quality.Score)
	}
	if len(fieldErrors) > 0 {
		t.Error("存在字段名错误，请检查生成结果")
	}
}
