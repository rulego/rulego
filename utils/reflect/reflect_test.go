/*
 * Copyright 2023 The RuleGo Authors.
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

package reflect

import (
	"github.com/rulego/rulego/test/assert"
	"testing"

	"github.com/rulego/rulego/api/types"
)

// FunctionsNodeConfiguration 节点配置
type FunctionsNodeConfiguration struct {
	FunctionName string `label:"函数名称" desc:"调用的函数名称" required:"true"`
}

// FunctionsNode 测试节点实现
type FunctionsNode struct {
	Config  FunctionsNodeConfiguration
	HasVars bool
}

func (x *FunctionsNode) Type() string {
	return "functions"
}

func (x *FunctionsNode) New() types.Node {
	return &FunctionsNode{Config: FunctionsNodeConfiguration{
		FunctionName: "test",
	}}
}

func (x *FunctionsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *FunctionsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *FunctionsNode) Destroy() {}

func TestGetComponentConfig(t *testing.T) {
	node := &FunctionsNode{}
	ty, configField, _ := GetComponentConfig(node)

	assert.Equal(t, "FunctionsNode", ty.Name())
	assert.Equal(t, "Config", configField.Name)
}

func TestGetComponentForm(t *testing.T) {
	node := &FunctionsNode{
		Config: FunctionsNodeConfiguration{
			FunctionName: "test",
		},
	}
	form := GetComponentForm(node)

	assert.Equal(t, "functions", form.Type)
	assert.Equal(t, "FunctionsNode", form.Label)
	assert.Equal(t, 1, len(form.Fields))
	assert.Equal(t, "functionName", form.Fields[0].Name)
	assert.Equal(t, "string", form.Fields[0].Type)
	assert.Equal(t, "test", form.Fields[0].DefaultValue)
	assert.Equal(t, "函数名称", form.Fields[0].Label)
	assert.Equal(t, "调用的函数名称", form.Fields[0].Desc)
	assert.True(t, form.Fields[0].Rules[0]["required"].(bool))
}

// TestJSONTagConfiguration 测试JSON tag优先级的配置结构
type TestJSONTagConfiguration struct {
	FieldWithJSONTag    string `json:"custom_field_name" label:"带JSON标签的字段" desc:"使用JSON标签名称"`
	FieldWithoutJSONTag string `label:"不带JSON标签的字段" desc:"使用默认字段名称"`
	FieldWithJSONOmit   string `json:"omit_field,omitempty" label:"带omitempty的字段" desc:"JSON标签包含omitempty"`
	FieldWithJSONIgnore string `json:"-" label:"忽略的字段" desc:"JSON标签为-"`
	FieldWithEmptyJSON  string `json:"" label:"空JSON标签" desc:"JSON标签为空字符串"`
}

// TestJSONTagNode 测试JSON tag的节点
type TestJSONTagNode struct {
	Config TestJSONTagConfiguration
}

func (x *TestJSONTagNode) Type() string {
	return "testJSONTag"
}

func (x *TestJSONTagNode) New() types.Node {
	return &TestJSONTagNode{}
}

func (x *TestJSONTagNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *TestJSONTagNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *TestJSONTagNode) Destroy() {}

func TestGetFieldsWithJSONTag(t *testing.T) {
	node := &TestJSONTagNode{}
	form := GetComponentForm(node)

	// 验证字段数量
	assert.Equal(t, 5, len(form.Fields))

	// 测试带JSON标签的字段，应该使用JSON标签名称
	fieldWithJSON, found := form.Fields.GetField("custom_field_name")
	assert.True(t, found)
	assert.Equal(t, "custom_field_name", fieldWithJSON.Name)
	assert.Equal(t, "带JSON标签的字段", fieldWithJSON.Label)
	assert.Equal(t, "使用JSON标签名称", fieldWithJSON.Desc)

	// 测试不带JSON标签的字段，应该使用默认字段名称（首字母小写）
	fieldWithoutJSON, found := form.Fields.GetField("fieldWithoutJSONTag")
	assert.True(t, found)
	assert.Equal(t, "fieldWithoutJSONTag", fieldWithoutJSON.Name)
	assert.Equal(t, "不带JSON标签的字段", fieldWithoutJSON.Label)
	assert.Equal(t, "使用默认字段名称", fieldWithoutJSON.Desc)

	// 测试带omitempty的字段，应该使用JSON标签名称（去掉omitempty）
	fieldWithOmit, found := form.Fields.GetField("omit_field")
	assert.True(t, found)
	assert.Equal(t, "omit_field", fieldWithOmit.Name)
	assert.Equal(t, "带omitempty的字段", fieldWithOmit.Label)
	assert.Equal(t, "JSON标签包含omitempty", fieldWithOmit.Desc)

	// 测试JSON标签为"-"的字段，应该使用默认字段名称
	fieldWithIgnore, found := form.Fields.GetField("fieldWithJSONIgnore")
	assert.True(t, found)
	assert.Equal(t, "fieldWithJSONIgnore", fieldWithIgnore.Name)
	assert.Equal(t, "忽略的字段", fieldWithIgnore.Label)
	assert.Equal(t, "JSON标签为-", fieldWithIgnore.Desc)

	// 测试空JSON标签的字段，应该使用默认字段名称
	fieldWithEmpty, found := form.Fields.GetField("fieldWithEmptyJSON")
	assert.True(t, found)
	assert.Equal(t, "fieldWithEmptyJSON", fieldWithEmpty.Name)
	assert.Equal(t, "空JSON标签", fieldWithEmpty.Label)
	assert.Equal(t, "JSON标签为空字符串", fieldWithEmpty.Desc)
}
