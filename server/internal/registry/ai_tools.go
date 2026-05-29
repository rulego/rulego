package registry

import (
	"encoding/json"

	aitool "github.com/rulego/rulego-components-ai/tool"
	"github.com/rulego/rulego/api/types"
)

const (
	aiSkillDefaultScope = "global"
	aiSkillManageTarget = "global-skills"
)

// EnhanceAiToolForms injects lightweight editor metadata for selected built-in
// AI tools without changing the underlying runtime tool contract.
func EnhanceAiToolForms(forms []aitool.ToolForm) []interface{} {
	return ApplySkillToolPathToForms(EnhanceAiToolFormsWithDefault(forms, "./skills"), "./skills")
}

// EnhanceAiToolFormsWithDefault converts built-in tool forms into editor-facing
// metadata while allowing callers to control the configured global skill path.
func EnhanceAiToolFormsWithDefault(forms []aitool.ToolForm, globalDir string) []interface{} {
	result := make([]interface{}, 0, len(forms))
	for _, form := range forms {
		data := toolFormToMap(form)
		if form.Type == "skill" {
			applySkillToolDefaults(data, form.Fields, globalDir)
		}
		result = append(result, data)
	}
	return result
}

// toolFormToMap converts the tool form into a generic JSON-friendly map so the
// editor can receive extra metadata fields beyond the base schema.
func toolFormToMap(form aitool.ToolForm) map[string]interface{} {
	raw, err := json.Marshal(form)
	if err != nil {
		return map[string]interface{}{
			"type":  form.Type,
			"label": form.Label,
			"desc":  form.Desc,
		}
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return map[string]interface{}{
			"type":  form.Type,
			"label": form.Label,
			"desc":  form.Desc,
		}
	}
	return data
}

// applySkillToolDefaults adds UI metadata and a default global skill path so
// the first-version editor can directly manage and use shared skills.
func applySkillToolDefaults(data map[string]interface{}, fields types.ComponentFormFieldList, globalDir string) {
	data["defaultScope"] = aiSkillDefaultScope
	data["manageable"] = true
	data["manageTarget"] = aiSkillManageTarget
	data["defaultGlobalDir"] = globalDir
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
		if items[index].Name == "globalDirs" && items[index].DefaultValue == nil {
			items[index].DefaultValue = []string{globalDir}
		}
	}
	return items
}

// ApplySkillToolPathToForms updates existing editor-facing tool form maps with
// the current configured global skill path.
func ApplySkillToolPathToForms(forms []interface{}, globalDir string) []interface{} {
	for index, item := range forms {
		data, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if data["type"] != "skill" {
			continue
		}
		data["defaultGlobalDir"] = globalDir
		if rawFields, ok := data["fields"].([]interface{}); ok {
			for fieldIndex, rawField := range rawFields {
				fieldMap, ok := rawField.(map[string]interface{})
				if !ok || fieldMap["name"] != "globalDirs" {
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
