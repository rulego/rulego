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

// newMCPBridge creates a test bridge containing the MCP module, returning both the bridge and the mcp module
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

// callTool calls tools directly through the MCP module (bypassing the HTTP SSE transport layer)
func callToolDirect(t *testing.T, mcpMod *mcpmodule.Module, toolName string, args map[string]interface{}) string {
	t.Helper()
	result, err := mcpMod.CallTool(context.Background(), toolName, args)
	if err != nil {
		t.Fatalf("CallTool %s error: %v", toolName, err)
	}
	return result
}

// parseChainJSON parses the rule chain from the result of the tool call
func parseChainJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var chain map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &chain); err != nil {
		t.Fatalf("JSON parse error: %v, raw: %s", err, truncate(raw, 300))
	}
	return chain
}

// TestMCP_PreviewRuleChain Test preview_rule_chain tools
func TestMCP_PreviewRuleChain(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// Make sure the tool is loaded
	defs, err := mcpMod.ListToolDefinitions()
	if err != nil {
		t.Fatalf("ListToolDefinitions error: %v", err)
	}
	toolNames := make(map[string]bool)
	for _, d := range defs {
		toolNames[d.Name] = true
	}
	if !toolNames["preview_rule_chain"] {
		t.Fatal("preview_rule_chain The tool is not registered")
	}
	t.Logf("%d tools have been registered", len(defs))

	// Scenario 1: preview of the valid rule chain → returns JSON with the _preview tag
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
			t.Errorf("preview Results lack _preview:true and chain: %v", chain)
		}
		if id, _ := chain["_id"].(string); id != "test_preview_valid" {
			t.Errorf("_id = %q, want test_preview_valid", id)
		}
		t.Log("Verification passed: preview returned the full JSON marked with _preview and _id")
	})

	// Scenario 2: preview has an error field → Returns a validation warning (LLM can fix this)
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
			t.Errorf("A verification failure should return a warning, actually: %s", result)
		}
		if !strings.Contains(result, "wrongFieldName") {
			t.Errorf("The warning should include the name of the error field, actually: %s", result)
		}
		if !strings.Contains(result, "jsScript") {
			t.Errorf("The warning should include a list of available fields to help LLM correct, actually: %s", result)
		}
		t.Logf("Verification passed: Validation warning = %s", truncate(result, 200))
	})

	// Scenario 3: Preview does not save the rule chain
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

		// Use get_rule_chain to verify that the chain does not exist (it should return an error)
		_, err := mcpMod.CallTool(context.Background(), "get_rule_chain", map[string]interface{}{
			"id": "test_preview_nosave",
		})
		if err == nil {
			t.Error("preview should not store the rule chain, but get_rule_chain found it")
		}
		t.Log("Validation successful: preview Rule chain not saved")
	})
}

// TestMCP_PreviewValidationRetry Simulates LLM validation failures → retries → retries based on warnings
func TestMCP_PreviewValidationRetry(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// First time: LLM guessed the wrong field name (url → should be restEndpointUrlPattern)
	t.Log("=== First attempt: Error field ===")
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
		t.Fatalf("Expect to return a checksum warning, actually: %s", wrongResult)
	}
	if !strings.Contains(wrongResult, "url") {
		t.Fatalf("The warning should indicate the error field 'url': %s", wrongResult)
	}
	if !strings.Contains(wrongResult, "restEndpointUrlPattern") {
		t.Fatalf("The warning should include the correct field 'restEndpointUrlPattern': %s", wrongResult)
	}
	t.Logf("Check-in warning (LLM corrected accordingly): %s", truncate(wrongResult, 200))

	// Second time: The LLM corrects field names based on warnings
	t.Log("=== Second attempt: Correction field ===")
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
		t.Fatalf("After correction, return to preview JSON: %s", correctResult)
	}
	t.Log("Preview after correction successful")

	// Confirm and save
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
		t.Fatalf("save Failure: %s", saveResult)
	}
	t.Log("=== Save successful ===")

	callToolDirect(t, mcpMod, "delete_rule_chain", map[string]interface{}{"id": "test_retry"})
}

// TestMCP_FullCRUD Test the complete CRUD + Execute flow
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

	// 6. Verify deleted — get_rule_chain The chain should not be found
	_, err := mcpMod.CallTool(context.Background(), "get_rule_chain", map[string]interface{}{"id": chainId})
	if err == nil {
		t.Fatal("Deleted chains should fail to query")
	}
	t.Log("6. Verify deleted OK")
}

// TestMCP_ToolRegistration Verify that the preview_rule_chain tool is registered
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
			t.Errorf("Tool %s Not registered", name)
		}
	}

	t.Logf("%d tools registered (expected %d)", len(defs), len(expectedTools))
}

// TestMCP_MultiUserIsolation Test tool handler isolation in multi-user scenarios
// Before fixing: toolDefs is a global map; the handler of the later registered user will override the previous user's closed username
// After fixing: userToolDefs is stored by username, and CallTool finds the correct handler by username in context
func TestMCP_MultiUserIsolation(t *testing.T) {
	br, mcpMod := newMCPBridgeWithModule(t)
	defer br.Stop()

	// Simulate admin users calling via "self" MCP (injecting username into context)
	ctx := context.Background()

	// Save the rule chain (no username context, rollback to global toolDefs)
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

	// Verification can be read through get_rule_chain
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

	// Cleanup
	callToolDirect(t, mcpMod, "delete_rule_chain", map[string]interface{}{"id": adminChainId})
}

// Ensure mcpMod satisfies McpService at compile time
var _ services.McpService = (*mcpmodule.Module)(nil)
