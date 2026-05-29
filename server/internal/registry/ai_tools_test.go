package registry

import (
	"testing"

	aitool "github.com/rulego/rulego-components-ai/tool"
	"github.com/rulego/rulego/api/types"
)

// TestEnhanceAiToolFormsAddsSkillMetadata verifies the editor-facing skill tool
// metadata includes global-skill management hints and default scope/path info.
func TestEnhanceAiToolFormsAddsSkillMetadata(t *testing.T) {
	forms := []aitool.ToolForm{
		{
			ComponentForm: types.ComponentForm{
				Type: "skill",
				Desc: "技能调用工具",
			},
		},
	}

	enhanced := EnhanceAiToolFormsWithDefault(forms, "./skills")
	if len(enhanced) != 1 {
		t.Fatalf("len(enhanced) = %d, want 1", len(enhanced))
	}

	skillForm, ok := enhanced[0].(map[string]interface{})
	if !ok {
		t.Fatalf("enhanced[0] type = %T, want map[string]interface{}", enhanced[0])
	}

	if got := skillForm["defaultScope"]; got != "global" {
		t.Fatalf("defaultScope = %v, want global", got)
	}
	if got := skillForm["manageable"]; got != true {
		t.Fatalf("manageable = %v, want true", got)
	}
	if got := skillForm["manageTarget"]; got != "global-skills" {
		t.Fatalf("manageTarget = %v, want global-skills", got)
	}
	if got := skillForm["defaultGlobalDir"]; got != "./skills" {
		t.Fatalf("defaultGlobalDir = %v, want ./skills", got)
	}
}

func TestWithSkillFieldDefaultsFillsConfiguredPath(t *testing.T) {
	fields := types.ComponentFormFieldList{
		{Name: "globalDirs"},
		{Name: "useChinese"},
	}

	items := withSkillFieldDefaults(fields, "./skills")
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	got, ok := items[0].DefaultValue.([]string)
	if !ok {
		t.Fatalf("defaultValue type = %T, want []string", items[0].DefaultValue)
	}
	if len(got) != 1 || got[0] != "./skills" {
		t.Fatalf("defaultValue = %#v, want []string{\"./skills\"}", got)
	}
}

func TestApplySkillToolPathToFormsOverridesConfiguredPath(t *testing.T) {
	forms := []interface{}{
		map[string]interface{}{
			"type":             "skill",
			"defaultGlobalDir": "./skills",
			"fields": []interface{}{
				map[string]interface{}{
					"name":         "globalDirs",
					"defaultValue": []string{"./skills"},
				},
			},
		},
	}

	items := ApplySkillToolPathToForms(forms, "D:/custom/skills")
	skillForm := items[0].(map[string]interface{})
	if skillForm["defaultGlobalDir"] != "D:/custom/skills" {
		t.Fatalf("defaultGlobalDir = %v, want D:/custom/skills", skillForm["defaultGlobalDir"])
	}
	fields := skillForm["fields"].([]interface{})
	fieldMap := fields[0].(map[string]interface{})
	got := fieldMap["defaultValue"].([]string)
	if len(got) != 1 || got[0] != "D:/custom/skills" {
		t.Fatalf("defaultValue = %#v, want []string{\"D:/custom/skills\"}", got)
	}
}
