package skill

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rulego/rulego/server/config"
)

// TestImportSkillsImportsNestedZip verifies a zip archive containing nested
// skill folders can be imported into the configured global skill directory.
func TestImportSkillsImportsNestedZip(t *testing.T) {
	tmpDir := t.TempDir()
	module := &Module{
		cfg: &config.Config{
			SkillPath: filepath.Join(tmpDir, "skills"),
		},
	}

	archive := buildSkillArchive(t, map[string]string{
		"alpha/SKILL.md":           "---\nname: alpha\ndescription: alpha desc\n---\n\n# Alpha\n",
		"nested/beta/SKILL.md":     "---\nname: beta\ndescription: beta desc\n---\n\n# Beta\n",
		"nested/ignored/readme.md": "ignore me",
	})

	items, err := module.ImportSkills("tester", "global", archive)
	if err != nil {
		t.Fatalf("ImportSkills() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	for _, name := range []string{"alpha", "beta"} {
		skillFile := filepath.Join(tmpDir, "skills", name, "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Fatalf("expected imported skill file %s: %v", skillFile, err)
		}
	}
}

// buildSkillArchive creates an in-memory zip used by import tests.
func buildSkillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}
