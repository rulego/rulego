/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package transform

//Example of rule chain node configuration:
//{
//"id": "s1",
//"type": "text/template",
//"name": "模板转换",
//"configuration": {
//"template": "type:{{ .type}}"
//}
//}

import (
	"bytes"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/funcs"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/maps"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateName The default template name
const TemplateName = "template"

func init() {
	Registry.Add(&TemplateNode{})
}

// TemplateNodeConfiguration node configuration
type TemplateNodeConfiguration struct {
	// Template is the Go template content or file path (prefix with 'file:' to load from file).
	Template string `json:"template" label:"Template" desc:"Go text/template content or 'file:/path/to/template'. Variables: .id, .ts, .data, .msg, .metadata, .type, .dataType" required:"true"`
}

// TemplateNode parses templates using text/template
// Access the message ID via the `.id` variable
// Access message timestamps via the `.ts` variable
// Access the original message data via the `.data` variable
// Access the transformed message body via the `.msg` variable. If the message's dataType is of JSON type, you can use `msg.XX` to access the msg field. For example: `msg.temperature > 50;` `
// Access message metadata through the `.metadata` variable. For example, `metadata.customerName`
// Access message types via the `.type` variable
// Access data types via the `.dataType` variable
type TemplateNode struct {
	Config         TemplateNodeConfiguration
	templateEngine *template.Template
	templateName   string
}

// Type returns the component type
func (x *TemplateNode) Type() string {
	return "text/template"
}

func (x *TemplateNode) New() types.Node {
	return &TemplateNode{
		Config: TemplateNodeConfiguration{
			Template: `"id": "{{ .id}}"
"ts": "{{ .ts}}"
"type": "{{ .type}}"
"msgType": "{{ .msgType}}"
"data": "{{ .data | escape}}"
"dataType": "{{ .dataType}}"
`,
		},
	}
}

// Init initializes the component
func (x *TemplateNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if strings.HasPrefix(x.Config.Template, "file:") {
			// Load the template from the file path
			filePath := strings.TrimPrefix(x.Config.Template, "file:")
			x.templateName = filepath.Base(filePath)
			x.templateEngine, err = template.New(x.templateName).Funcs(funcs.TemplateFunc.GetAll()).ParseFiles(filePath)
		} else {
			x.templateName = TemplateName
			// Use template content
			x.templateEngine, err = template.New(x.templateName).Funcs(funcs.TemplateFunc.GetAll()).Parse(x.Config.Template)
		}
	}
	return err
}

// OnMsg processes a message
func (x *TemplateNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var err error
	evn := base.NodeUtils.GetEvn(ctx, msg)

	var buf bytes.Buffer
	err = x.templateEngine.ExecuteTemplate(&buf, x.templateName, evn)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	msg.SetData(buf.String())
	ctx.TellSuccess(msg)
}

// Desc returns the component description
func (x *TemplateNode) Desc() string {
	return "Transform messages using Go text/template. Variables: .id, .ts, .data, .msg, .metadata, .type, .dataType. Supports 'file:' prefix for external templates. Routes to Success/Failure"
}

// Destroy releases resources
func (x *TemplateNode) Destroy() {
}
