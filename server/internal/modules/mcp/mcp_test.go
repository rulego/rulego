package mcp

import (
	"context"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/utils/str"
)

func TestMcpModuleInterface(t *testing.T) {
	m := New()
	if m.Name() != "mcp" {
		t.Errorf("Name() = %q, want %q", m.Name(), "mcp")
	}
	if m.Priority() != 25 {
		t.Errorf("Priority() = %d, want 25", m.Priority())
	}
}

func TestMcpModuleInitDisabled(t *testing.T) {
	m := New()
	container := app.NewContainer()
	cfg := config.DefaultConfig()
	cfg.MCP.Enable = false
	container.Register("core.config", &cfg)

	ctx := &app.ModuleContext{Container: container, Config: &cfg}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if _, ok := container.Get(services.KeyMcpService); !ok {
		t.Error("module.mcp.service not registered")
	}
}

func TestMcpModuleStartStop(t *testing.T) {
	m := New()
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}


func TestRegisterTool(t *testing.T) {
	m := New()
	m.cfg = &config.Config{MCP: config.MCPConfig{Enable: true}}
	m.users = make(map[string]*userMcpState)
	m.toolDefs = make(map[string]toolDefEntry)

	err := m.RegisterTool("testuser", "custom_tool", "A custom test tool",
		[]byte(`{"type":"object","properties":{"input":{"type":"string"}}}`),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			return "custom result: " + str.ToString(args["input"]), nil
		},
	)

	if err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}
	if _, ok := m.toolDefs["custom_tool"]; !ok {
		t.Error("custom_tool not found in toolDefs")
	}
}

func TestRegisterTool_Disabled(t *testing.T) {
	m := New()
	m.cfg = &config.Config{MCP: config.MCPConfig{Enable: false}}

	err := m.RegisterTool("testuser", "custom_tool", "desc", nil, nil)
	if err == nil {
		t.Error("expected error when MCP is disabled")
	}
}

func TestRegisterTool_CallTool(t *testing.T) {
	m := New()
	m.cfg = &config.Config{MCP: config.MCPConfig{Enable: true}}
	m.users = make(map[string]*userMcpState)
	m.toolDefs = make(map[string]toolDefEntry)

	_ = m.RegisterTool("testuser", "echo_tool", "Echo input",
		[]byte(`{"type":"object","properties":{"msg":{"type":"string"}},"required":["msg"]}`),
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			return str.ToString(args["msg"]), nil
		},
	)

	// 通过 CallTool 调用
	result, err := m.CallTool(context.Background(), "echo_tool", map[string]interface{}{
		"msg": "hello",
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("CallTool result = %q, want %q", result, "hello")
	}
}

func TestRegisterTool_ListDefinitions(t *testing.T) {
	m := New()
	m.cfg = &config.Config{MCP: config.MCPConfig{Enable: true}}
	m.users = make(map[string]*userMcpState)
	m.toolDefs = make(map[string]toolDefEntry)

	_ = m.RegisterTool("testuser", "tool_a", "Tool A", nil,
		func(ctx context.Context, args map[string]interface{}) (string, error) { return "a", nil },
	)
	_ = m.RegisterTool("testuser", "tool_b", "Tool B", nil,
		func(ctx context.Context, args map[string]interface{}) (string, error) { return "b", nil },
	)

	defs, err := m.ListToolDefinitions()
	if err != nil {
		t.Fatalf("ListToolDefinitions failed: %v", err)
	}
	if len(defs) != 2 {
		t.Errorf("ListToolDefinitions returned %d defs, want 2", len(defs))
	}
}
