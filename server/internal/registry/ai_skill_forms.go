package registry

import (
	"github.com/rulego/rulego/api/types"
)

const (
	aiSkillDefaultScope  = "global"
	aiSkillManageTarget  = "global-skills"
	toolTypeSkill        = "skill"
	fieldDefaultGlobalDir = "defaultGlobalDir"
	fieldGlobalDirs      = "globalDirs"
)

// ApplySkillToolPathToForms updates existing editor-facing tool form maps with
// the current configured global skill path.
func ApplySkillToolPathToForms(forms []interface{}, globalDir string) []interface{} {
	for index, item := range forms {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if data["type"] != toolTypeSkill {
			continue
		}
		data[fieldDefaultGlobalDir] = globalDir
		if rawFields, ok := data["fields"].([]interface{}); ok {
			for fieldIndex, rawField := range rawFields {
				fieldMap, ok := rawField.(map[string]interface{})
				if !ok || fieldMap["name"] != fieldGlobalDirs {
					continue
				}
				fieldMap["defaultValue"] = []string{globalDir}
				rawFields[fieldIndex] = fieldMap
			}
			data["fields"] = rawFields
		}
		forms[index] = data
	}
	return forms
}

// ApplySkillToolDefaults adds UI metadata and a default global skill path so
// the first-version editor can directly manage and use shared skills.
func ApplySkillToolDefaults(data map[string]interface{}, fields types.ComponentFormFieldList, globalDir string) {
	data["defaultScope"] = aiSkillDefaultScope
	data["manageable"] = true
	data["manageTarget"] = aiSkillManageTarget
	data[fieldDefaultGlobalDir] = globalDir
	data["fields"] = withSkillFieldDefaults(fields, globalDir)
}

// withSkillFieldDefaults clones the form fields and fills the globalDirs
// default value so selecting the skill tool works out of the box.
func withSkillFieldDefaults(fields types.ComponentFormFieldList, globalDir string) []types.ComponentFormField {
	if len(fields) == 0 {
		return nil
	}
	items := make([]types.ComponentFormField, len(fields))
	copy(items, fields)
	for index := range items {
		if items[index].Name == fieldGlobalDirs && items[index].DefaultValue == nil {
			items[index].DefaultValue = []string{globalDir}
		}
	}
	return items
}
