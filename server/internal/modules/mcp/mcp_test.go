package mcp

import (
	"context"
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/modules/user"
	"github.com/rulego/rulego/server/model"
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

type stubUserAdmin struct {
	roles map[string][]string
}

func (s *stubUserAdmin) List() []model.User                     { return nil }
func (s *stubUserAdmin) Get(username string) (model.User, bool) { return model.User{}, false }
func (s *stubUserAdmin) Save(user model.User) error             { return nil }
func (s *stubUserAdmin) Delete(username string) error           { return nil }
func (s *stubUserAdmin) RolesOf(username string) []string       { return s.roles[username] }

// mcpAuthorize 走与 REST 同一套授权器：viewer 拒写、admin 放行；
// 宿主未注册授权器（嵌入模式）时放行
func TestMcpAuthorize(t *testing.T) {
	m := New()
	m.container = app.NewContainer()
	m.container.Register(services.KeyAuthorizer, user.NewDefaultAuthorizer())
	m.container.Register(services.KeyUserAdmin, &stubUserAdmin{
		roles: map[string][]string{"viewer1": {"viewer"}, "admin1": {"admin"}},
	})
	if err := m.mcpAuthorize("viewer1", "rule", "write"); err == nil {
		t.Error("viewer should be denied on rule:write")
	}
	if err := m.mcpAuthorize("admin1", "rule", "delete"); err != nil {
		t.Errorf("admin should pass rule:delete, got %v", err)
	}

	bare := New()
	bare.container = app.NewContainer()
	if err := bare.mcpAuthorize("anyone", "rule", "delete"); err != nil {
		t.Errorf("no authorizer registered should allow, got %v", err)
	}
}
