package registry

import (
	"testing"

	"github.com/rulego/rulego/api/types"
)

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
