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

import (
	"bytes"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/builtin/funcs"
	"github.com/rulego/rulego/components"
	"github.com/rulego/rulego/utils/maps"
	"path/filepath"
	"strings"
	"text/template"
)

const TemplateName = "template"

func init() {
	Registry.Add(&TemplateNode{})
}

// TemplateNodeConfiguration 节点配置
type TemplateNodeConfiguration struct {
	// Template 模板内容或文件路径
	Template string
}

// TemplateNode 使用 text/template 解析模板
type TemplateNode struct {
	Config         TemplateNodeConfiguration
	templateEngine *template.Template
	templateName   string
}

// Type 组件类型
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

// Init 初始化
func (x *TemplateNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		if strings.HasPrefix(x.Config.Template, "file:") {
			// 从文件路径加载模板
			filePath := strings.TrimPrefix(x.Config.Template, "file:")
			x.templateName = filepath.Base(filePath)
			x.templateEngine, err = template.New(x.templateName).Funcs(funcs.TemplateFuncMap).ParseFiles(filePath)
		} else {
			x.templateName = TemplateName
			// 使用模板内容
			x.templateEngine, err = template.New(x.templateName).Funcs(funcs.TemplateFuncMap).Parse(x.Config.Template)
		}
	}
	return err
}

// OnMsg 处理消息
func (x *TemplateNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	evn, err := components.NodeUtils.GetEvn(ctx, msg)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	var buf bytes.Buffer
	err = x.templateEngine.ExecuteTemplate(&buf, x.templateName, evn)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	msg.Data = buf.String()
	ctx.TellSuccess(msg)
}

// Destroy 销毁
func (x *TemplateNode) Destroy() {
}
