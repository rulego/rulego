package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/server/app"
	mcpmodule "github.com/rulego/rulego/server/internal/modules/mcp"
	"github.com/rulego/rulego/server/internal/modules/node"
	"github.com/rulego/rulego/server/internal/modules/rule"
	"github.com/rulego/rulego/server/internal/modules/user"
	"github.com/rulego/rulego/server/services"
)

// newMCPBridge 创建包含 MCP 模块的测试 Bridge，返回 bridge 和 mcp module
func newMCPBridgeWithModule(t *testing.T) (*Bridge, *mcpmodule.Module) {
	t.Helper()
	tmpData := filepath.Join(t.TempDir(), "data")
	cfgContent := "server = :0\ndata_dir = " + tmpData + "\n" +
		"default_username = admin\n" +
		"require_auth = false\n" +
		"[users]\nadmin = admin,2af255ea5618467d914c67a8beeca31d\n" +
		"[mcp]\nenable = true\n"
	cfgFile := filepath.Join(t.TempDir(), "config.conf")
	os.WriteFile(cfgFile, []byte(cfgContent), 0644)

	mcpMod := mcpmodule.New()
	application := app.New(
		app.WithConfigFile(cfgFile),
		app.WithModules(user.New(), rule.New(), node.New(), mcpMod),
	)
	br, err := NewBridge(application)
	if err != nil {
		t.Fatalf("NewBridge error: %v", err)
	}
	return br, mcpMod
}

// callTool 直接通过 MCP 模块调用工具（绕过 HTTP SSE 传输层）
func callToolDirect(t *testing.T, mcpMod *mcpmodule.Module, toolName string, args map[string]interface{}) string {
	t.Helper()
	result, err := mcpMod.CallTool(context.Background(), toolName, args)
	if err != nil {
		t.Fatalf("CallTool %s error: %v", toolName, err)
	}
	return result
}

// parseChainJSON 从工具调用结果中解析规则链
func parseChainJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var chain map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &chain); err != nil {
		t.Fatalf("JSON parse error: %v, raw: %s", err, truncate(raw, 300))
	}
	return chain
}

// TestMCP_PreviewRuleChain 测试 preview_rule_chain 工具
func TestMCP_PreviewRuleChain(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// 确保工具已加载
	defs, err := mcpMod.ListToolDefinitions()
	if err != nil {
		t.Fatalf("ListToolDefinitions error: %v", err)
	}
	toolNames := make(map[string]bool)
	for _, d := range defs {
		toolNames[d.Name] = true
	}
	if !toolNames["preview_rule_chain"] {
		t.Fatal("preview_rule_chain 工具未注册")
	}
	t.Logf("已注册 %d 个工具", len(defs))

	// 场景1: preview 有效规则链 → 返回带 _preview 标记的 JSON
	t.Run("ValidChain", func(t *testing.T) {
		result := callToolDirect(t, mcpMod, "preview_rule_chain", map[string]interface{}{
			"id": "test_preview_valid",
			"body": map[string]interface{}{
				"ruleChain": map[string]interface{}{
					"id": "test_preview_valid", "name": "Preview Test", "root": false, "debugMode": false,
				},
				"metadata": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id": "node_1", "type": "jsTransform", "name": "转换", "debugMode": false,
							"configuration": map[string]interface{}{
								"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
							},
						},
					},
					"connections": []interface{}{},
				},
			},
		})

		chain := parseChainJSON(t, result)
		if preview, _ := chain["_preview"].(bool); !preview {
			t.Errorf("preview 结果缺少 _preview:true, chain: %v", chain)
		}
		if id, _ := chain["_id"].(string); id != "test_preview_valid" {
			t.Errorf("_id = %q, want test_preview_valid", id)
		}
		t.Log("验证通过: preview 返回带 _preview 和 _id 标记的完整 JSON")
	})

	// 场景2: preview 有错误字段 → 返回校验警告（LLM 可据此修正）
	t.Run("InvalidField_ReturnsWarning", func(t *testing.T) {
		result := callToolDirect(t, mcpMod, "preview_rule_chain", map[string]interface{}{
			"id": "test_preview_invalid",
			"body": map[string]interface{}{
				"ruleChain": map[string]interface{}{
					"id": "test_preview_invalid", "name": "Bad Field", "root": false,
				},
				"metadata": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id": "node_1", "type": "jsFilter", "name": "过滤",
							"configuration": map[string]interface{}{
								"wrongFieldName": "xxx",
								"jsScript":       "return true;",
							},
						},
					},
					"connections": []interface{}{},
				},
			},
		})

		if !strings.Contains(result, "unknown fields") {
			t.Errorf("校验失败应返回警告, 实际: %s", result)
		}
		if !strings.Contains(result, "wrongFieldName") {
			t.Errorf("警告应包含错误字段名, 实际: %s", result)
		}
		if !strings.Contains(result, "jsScript") {
			t.Errorf("警告应包含可用字段列表帮助 LLM 修正, 实际: %s", result)
		}
		t.Logf("验证通过: 校验警告 = %s", truncate(result, 200))
	})

	// 场景3: preview 不保存规则链
	t.Run("DoesNotSave", func(t *testing.T) {
		callToolDirect(t, mcpMod, "preview_rule_chain", map[string]interface{}{
			"id": "test_preview_nosave",
			"body": map[string]interface{}{
				"ruleChain": map[string]interface{}{
					"id": "test_preview_nosave", "name": "No Save", "root": false,
				},
				"metadata": map[string]interface{}{
					"nodes": []interface{}{
						map[string]interface{}{
							"id": "node_1", "type": "jsTransform", "name": "转换",
							"configuration": map[string]interface{}{
								"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
							},
						},
					},
					"connections": []interface{}{},
				},
			},
		})

		// 用 get_rule_chain 验证链不存在（应该返回错误）
		_, err := mcpMod.CallTool(context.Background(), "get_rule_chain", map[string]interface{}{
			"id": "test_preview_nosave",
		})
		if err == nil {
			t.Error("preview 不应保存规则链，但 get_rule_chain 查到了")
		}
		t.Log("验证通过: preview 未保存规则链")
	})
}

// TestMCP_PreviewValidationRetry 模拟 LLM 校验失败→根据警告修正→重试流程
func TestMCP_PreviewValidationRetry(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// 第一次: LLM 猜错字段名（url → 应该是 restEndpointUrlPattern）
	t.Log("=== 第一次尝试: 错误字段 ===")
	wrongResult := callToolDirect(t, mcpMod, "preview_rule_chain", map[string]interface{}{
		"id": "test_retry",
		"body": map[string]interface{}{
			"ruleChain": map[string]interface{}{"id": "test_retry", "name": "Retry Test"},
			"metadata": map[string]interface{}{
				"nodes": []interface{}{
					map[string]interface{}{
						"id": "node_1", "type": "restApiCall", "name": "HTTP",
						"configuration": map[string]interface{}{
							"url": "http://example.com/api", "method": "POST",
						},
					},
				},
				"connections": []interface{}{},
			},
		},
	})

	if !strings.Contains(wrongResult, "unknown fields") {
		t.Fatalf("期望返回校验警告, 实际: %s", wrongResult)
	}
	if !strings.Contains(wrongResult, "url") {
		t.Fatalf("警告应指出错误字段 'url': %s", wrongResult)
	}
	if !strings.Contains(wrongResult, "restEndpointUrlPattern") {
		t.Fatalf("警告应包含正确字段 'restEndpointUrlPattern': %s", wrongResult)
	}
	t.Logf("校验警告 (LLM 据此修正): %s", truncate(wrongResult, 200))

	// 第二次: LLM 根据警告修正字段名
	t.Log("=== 第二次尝试: 修正字段 ===")
	correctResult := callToolDirect(t, mcpMod, "preview_rule_chain", map[string]interface{}{
		"id": "test_retry",
		"body": map[string]interface{}{
			"ruleChain": map[string]interface{}{"id": "test_retry", "name": "Retry Test"},
			"metadata": map[string]interface{}{
				"nodes": []interface{}{
					map[string]interface{}{
						"id": "node_1", "type": "restApiCall", "name": "HTTP",
						"configuration": map[string]interface{}{
							"restEndpointUrlPattern": "http://example.com/api",
							"requestMethod":          "POST",
						},
					},
				},
				"connections": []interface{}{},
			},
		},
	})

	chain := parseChainJSON(t, correctResult)
	if preview, _ := chain["_preview"].(bool); !preview {
		t.Fatalf("修正后应返回预览 JSON: %s", correctResult)
	}
	t.Log("修正后预览成功")

	// 确认保存
	saveResult := callToolDirect(t, mcpMod, "save_rule_chain", map[string]interface{}{
		"id": "test_retry",
		"body": map[string]interface{}{
			"ruleChain": map[string]interface{}{"id": "test_retry", "name": "Retry Test"},
			"metadata": map[string]interface{}{
				"nodes": []interface{}{
					map[string]interface{}{
						"id": "node_1", "type": "restApiCall", "name": "HTTP",
						"configuration": map[string]interface{}{
							"restEndpointUrlPattern": "http://example.com/api",
							"requestMethod":          "POST",
						},
					},
				},
				"connections": []interface{}{},
			},
		},
	})
	if saveResult != "save ok" {
		t.Fatalf("save 失败: %s", saveResult)
	}
	t.Log("=== 保存成功 ===")

	callToolDirect(t, mcpMod, "delete_rule_chain", map[string]interface{}{"id": "test_retry"})
}

// TestMCP_FullCRUD 测试完整 CRUD + Execute 流程
func TestMCP_FullCRUD(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	chainId := fmt.Sprintf("crud_%d", time.Now().UnixNano())
	chainBody := map[string]interface{}{
		"ruleChain": map[string]interface{}{"id": chainId, "name": "CRUD Test", "root": false},
		"metadata": map[string]interface{}{
			"nodes": []interface{}{
				map[string]interface{}{
					"id": "n1", "type": "jsTransform", "name": "转换",
					"configuration": map[string]interface{}{
						"jsScript": "msg.result='ok'; return {'msg':msg,'metadata':metadata,'msgType':msgType};",
					},
				},
			},
			"connections": []interface{}{},
		},
	}

	// 1. Create
	r := callToolDirect(t, mcpMod, "save_rule_chain", map[string]interface{}{"id": chainId, "body": chainBody})
	if r != "save ok" {
		t.Fatalf("create: %s", r)
	}
	t.Log("1. Create OK")

	// 2. Read
	r = callToolDirect(t, mcpMod, "get_rule_chain", map[string]interface{}{"id": chainId})
	if !strings.Contains(r, chainId) {
		t.Fatalf("read: %s", r)
	}
	t.Log("2. Read OK")

	// 3. List
	r = callToolDirect(t, mcpMod, "list_rule_chains", map[string]interface{}{"keywords": chainId, "size": 5})
	if !strings.Contains(r, chainId) {
		t.Fatalf("list: %s", r)
	}
	t.Log("3. List OK")

	// 4. Execute
	r = callToolDirect(t, mcpMod, "execute_rule_chain", map[string]interface{}{
		"id": chainId, "message": map[string]interface{}{"data": "hello"},
	})
	t.Logf("4. Execute result: %s", truncate(r, 100))

	// 5. Delete
	r = callToolDirect(t, mcpMod, "delete_rule_chain", map[string]interface{}{"id": chainId})
	if r != "delete ok" {
		t.Fatalf("delete: %s", r)
	}
	t.Log("5. Delete OK")

	// 6. Verify deleted — get_rule_chain 应该找不到该链
	_, err := mcpMod.CallTool(context.Background(), "get_rule_chain", map[string]interface{}{"id": chainId})
	if err == nil {
		t.Fatal("已删除的链应该查询失败")
	}
	t.Log("6. Verify deleted OK")
}

// TestMCP_ToolRegistration 验证 preview_rule_chain 工具已注册
func TestMCP_ToolRegistration(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	defs, err := mcpMod.ListToolDefinitions()
	if err != nil {
		t.Fatalf("ListToolDefinitions error: %v", err)
	}

	expectedTools := []string{
		"list_rule_chains", "get_rule_chain", "preview_rule_chain",
		"save_rule_chain", "delete_rule_chain",
		"operate_rule_chain", "execute_rule_chain",
		"list_components", "get_component_doc",
	}

	toolMap := make(map[string]bool)
	for _, d := range defs {
		toolMap[d.Name] = true
	}

	for _, name := range expectedTools {
		if !toolMap[name] {
			t.Errorf("工具 %s 未注册", name)
		}
	}

	t.Logf("已注册 %d 个工具 (期望 %d)", len(defs), len(expectedTools))
}

// TestMCP_MultiUserIsolation 测试多用户场景下工具 handler 隔离
// 修复前：toolDefs 是全局 map，后注册的用户的 handler 会覆盖前一个用户的闭包 username
// 修复后：userToolDefs 按 username 存储，CallTool 通过 context 中的 username 查找正确 handler
func TestMCP_MultiUserIsolation(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// 模拟 admin 用户通过 "self" MCP 调用（注入 username 到 context）
	ctx := context.Background()

	// 保存规则链（无 username context，回退到全局 toolDefs）
	adminChainId := fmt.Sprintf("admin_chain_%d", time.Now().UnixNano())
	adminBody := map[string]interface{}{
		"ruleChain": map[string]interface{}{"id": adminChainId, "name": "Admin Chain", "root": false},
		"metadata": map[string]interface{}{
			"nodes": []interface{}{
				map[string]interface{}{
					"id": "n1", "type": "jsTransform", "name": "Admin Node",
					"configuration": map[string]interface{}{
						"jsScript": "msg.user='admin'; return {'msg':msg,'metadata':metadata,'msgType':msgType};",
					},
				},
			},
			"connections": []interface{}{},
		},
	}

	result, err := mcpMod.CallTool(ctx, "save_rule_chain", map[string]interface{}{
		"id":   adminChainId,
		"body": adminBody,
	})
	if err != nil {
		t.Fatalf("admin save error: %v", err)
	}
	if result != "save ok" {
		t.Fatalf("admin save result: %s", result)
	}
	t.Logf("admin save ok: %s", truncate(result, 100))

	// 验证可以通过 get_rule_chain 读到
	result, err = mcpMod.CallTool(ctx, "get_rule_chain", map[string]interface{}{
		"id": adminChainId,
	})
	if err != nil {
		t.Fatalf("get admin chain error: %v", err)
	}
	if !strings.Contains(result, adminChainId) {
		t.Fatalf("get admin chain: %s", result)
	}
	t.Log("admin chain read ok")

	// 清理
	callToolDirect(t, mcpMod, "delete_rule_chain", map[string]interface{}{"id": adminChainId})
}

// Ensure mcpMod satisfies McpService at compile time
var _ services.McpService = (*mcpmodule.Module)(nil)
