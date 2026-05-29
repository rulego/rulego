package mcp

import (
	"testing"

	"github.com/rulego/rulego/server/config"
)

func TestParseToolFilter(t *testing.T) {
	tests := []struct {
		name           string
		tools          []string
		wantAllowed    map[string]bool
		wantExcluded   []string
		wantAllAllowed bool
	}{
		{
			name:           "empty means all allowed",
			tools:          []string{},
			wantAllowed:    nil,
			wantExcluded:   nil,
			wantAllAllowed: true,
		},
		{
			name:           "wildcard means all allowed",
			tools:          []string{"*"},
			wantAllowed:    nil,
			wantExcluded:   nil,
			wantAllAllowed: true,
		},
		{
			name:           "specific tools",
			tools:          []string{"save_rule_chain", "get_rule_chain"},
			wantAllowed:    map[string]bool{"save_rule_chain": true, "get_rule_chain": true},
			wantExcluded:   nil,
			wantAllAllowed: false,
		},
		{
			name:           "exclude prefix",
			tools:          []string{"*", "-test_*"},
			wantAllowed:    nil,
			wantExcluded:   []string{"test_"},
			wantAllAllowed: false,
		},
		{
			name:           "mixed include and exclude",
			tools:          []string{"save_rule_chain", "get_rule_chain", "-internal_*"},
			wantAllowed:    map[string]bool{"save_rule_chain": true, "get_rule_chain": true},
			wantExcluded:   []string{"internal_"},
			wantAllAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, excluded := parseToolFilter(tt.tools)

			if tt.wantAllAllowed {
				if allowed != nil {
					t.Errorf("parseToolFilter() allowed = %v, want nil for all allowed", allowed)
				}
				return
			}

			if len(tt.wantAllowed) > 0 {
				if len(allowed) != len(tt.wantAllowed) {
					t.Errorf("parseToolFilter() allowed count = %d, want %d", len(allowed), len(tt.wantAllowed))
				}
				for k := range tt.wantAllowed {
					if !allowed[k] {
						t.Errorf("parseToolFilter() missing allowed tool: %s", k)
					}
				}
			}

			if len(tt.wantExcluded) > 0 {
				if len(excluded) != len(tt.wantExcluded) {
					t.Errorf("parseToolFilter() excluded count = %d, want %d", len(excluded), len(tt.wantExcluded))
				}
			}
		})
	}
}

func TestIsToolAllowed(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		allowedTools   map[string]bool
		excludedPrefix []string
		want           bool
	}{
		{
			name:           "no filter means all allowed",
			toolName:       "any_tool",
			allowedTools:   nil,
			excludedPrefix: nil,
			want:           true,
		},
		{
			name:           "in allowed list",
			toolName:       "save_rule_chain",
			allowedTools:   map[string]bool{"save_rule_chain": true, "get_rule_chain": true},
			excludedPrefix: nil,
			want:           true,
		},
		{
			name:           "not in allowed list",
			toolName:       "delete_rule_chain",
			allowedTools:   map[string]bool{"save_rule_chain": true, "get_rule_chain": true},
			excludedPrefix: nil,
			want:           false,
		},
		{
			name:           "excluded by prefix",
			toolName:       "test_chain1",
			allowedTools:   nil,
			excludedPrefix: []string{"test_"},
			want:           false,
		},
		{
			name:           "not excluded by prefix",
			toolName:       "my_chain",
			allowedTools:   nil,
			excludedPrefix: []string{"test_"},
			want:           true,
		},
		{
			name:           "in allowed but excluded",
			toolName:       "test_save",
			allowedTools:   map[string]bool{"test_save": true},
			excludedPrefix: []string{"test_"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolAllowed(tt.toolName, tt.allowedTools, tt.excludedPrefix)
			if got != tt.want {
				t.Errorf("isToolAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestContainsToolType(t *testing.T) {
	tests := []struct {
		name       string
		allowed    map[string]bool
		toolType   string
		want       bool
	}{
		{
			name:     "nil means all",
			allowed:  nil,
			toolType: "rules",
			want:     true,
		},
		{
			name:     "empty means all",
			allowed:  map[string]bool{},
			toolType: "rules",
			want:     true,
		},
		{
			name:     "contains type",
			allowed:  map[string]bool{"rules": true, "components": true},
			toolType: "rules",
			want:     true,
		},
		{
			name:     "not contains type",
			allowed:  map[string]bool{"rules": true},
			toolType: "components",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsToolType(tt.allowed, tt.toolType)
			if got != tt.want {
				t.Errorf("containsToolType(%v, %q) = %v, want %v", tt.allowed, tt.toolType, got, tt.want)
			}
		})
	}
}

func TestInitGroups(t *testing.T) {
	m := New()
	m.cfg = &config.Config{
		MCP: config.MCPConfig{
			Groups: map[string]string{
				"rules":      "save_rule_chain,get_rule_chain,list_rule_chains",
				"components": "*,-test_*",
				"chains":     "*",
			},
		},
	}

	m.initGroups()

	if len(m.groups) != 3 {
		t.Errorf("initGroups() created %d groups, want 3", len(m.groups))
	}

	rulesGroup, ok := m.groups["rules"]
	if !ok {
		t.Fatal("rules group not found")
	}
	if len(rulesGroup.Tools) != 3 {
		t.Errorf("rules group has %d tools, want 3", len(rulesGroup.Tools))
	}

	componentsGroup, ok := m.groups["components"]
	if !ok {
		t.Fatal("components group not found")
	}
	if len(componentsGroup.Tools) != 2 {
		t.Errorf("components group has %d tools, want 2", len(componentsGroup.Tools))
	}
}

func TestIsGroupExists(t *testing.T) {
	m := New()
	m.groups = map[string]*MCPGroup{
		"rules":      {Name: "rules", Tools: []string{"*"}},
		"components": {Name: "components", Tools: []string{"*"}},
	}

	tests := []struct {
		name      string
		groupName string
		want      bool
	}{
		{"default always exists", "default", true},
		{"existing group", "rules", true},
		{"non-existing group", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.isGroupExists(tt.groupName)
			if got != tt.want {
				t.Errorf("isGroupExists(%q) = %v, want %v", tt.groupName, got, tt.want)
			}
		})
	}
}
