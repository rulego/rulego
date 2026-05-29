package skill

import (
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "my-skill", false},
		{"valid with underscore", "my_skill", false},
		{"valid with numbers", "skill123", false},
		{"valid complex", "my-skill_v2", false},
		{"empty name", "", true},
		{"spaces", "my skill", true},
		{"special chars", "skill@name", true},
		{"dot", "skill.name", true},
		{"slash", "skill/name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSkillName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	t.Run("with frontmatter", func(t *testing.T) {
		content := `---
name: "Test Skill"
description: "A test skill"
---

Body content here.`
		fm, body := parseFrontmatter(content)
		if fm.Name != "Test Skill" {
			t.Errorf("Name = %q, want %q", fm.Name, "Test Skill")
		}
		if fm.Description != "A test skill" {
			t.Errorf("Description = %q, want %q", fm.Description, "A test skill")
		}
		if body != "Body content here." {
			t.Errorf("body = %q, want %q", body, "Body content here.")
		}
	})

	t.Run("without frontmatter", func(t *testing.T) {
		content := "# My Skill\n\nSome content."
		fm, body := parseFrontmatter(content)
		if fm.Name != "" {
			t.Errorf("Name should be empty, got %q", fm.Name)
		}
		if body != content {
			t.Errorf("body should be unchanged when no frontmatter")
		}
	})

	t.Run("name without quotes", func(t *testing.T) {
		content := "---\nname: PlainName\n---\n\nBody."
		fm, body := parseFrontmatter(content)
		if fm.Name != "PlainName" {
			t.Errorf("Name = %q, want %q", fm.Name, "PlainName")
		}
		if body != "Body." {
			t.Errorf("body = %q, want %q", body, "Body.")
		}
	})
}

func TestParseFromContent(t *testing.T) {
	t.Run("extracts name from heading", func(t *testing.T) {
		content := "# My Skill\n\nSome text."
		fm := parseFromContent(content)
		if fm.Name != "My Skill" {
			t.Errorf("Name = %q, want %q", fm.Name, "My Skill")
		}
	})

	t.Run("extracts description from blockquote", func(t *testing.T) {
		content := "# My Skill\n> A description\n\nBody."
		fm := parseFromContent(content)
		if fm.Description != "A description" {
			t.Errorf("Description = %q, want %q", fm.Description, "A description")
		}
	})

	t.Run("no heading", func(t *testing.T) {
		content := "Just plain text."
		fm := parseFromContent(content)
		if fm.Name != "" {
			t.Errorf("Name should be empty, got %q", fm.Name)
		}
	})
}

func TestGenerateSkillContent(t *testing.T) {
	t.Run("generates frontmatter", func(t *testing.T) {
		result := generateSkillContent("Test", "A test", "Body text")
		if result == "Body text" {
			t.Error("should wrap in frontmatter when body doesn't start with ---")
		}
		if !contains(result, "name: Test") {
			t.Error("should contain name field")
		}
		if !contains(result, "A test") {
			t.Error("should contain description field")
		}
		if !contains(result, "Body text") {
			t.Error("should contain body text")
		}
	})

	t.Run("preserves existing frontmatter", func(t *testing.T) {
		body := "---\nname: Existing\n---\n\nContent."
		result := generateSkillContent("Test", "Desc", body)
		if result != body {
			t.Error("should not re-wrap content that already has frontmatter")
		}
	})
}

func TestExtractBody(t *testing.T) {
	t.Run("extracts body from frontmatter", func(t *testing.T) {
		content := "---\nname: Test\n---\n\nExtracted body."
		body := extractBody(content)
		if body != "Extracted body." {
			t.Errorf("body = %q, want %q", body, "Extracted body.")
		}
	})

	t.Run("returns full content without frontmatter", func(t *testing.T) {
		content := "No frontmatter here."
		body := extractBody(content)
		if body != content {
			t.Error("should return unchanged content")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
