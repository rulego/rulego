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

	// 验证字段数量 - 应该只有4个字段，因为json:"-"的字段被排除了
	assert.Equal(t, 4, len(form.Fields))

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

	// 测试JSON标签为"-"的字段，应该被排除，不应该出现在结果中
	_, found = form.Fields.GetField("fieldWithJSONIgnore")
	assert.False(t, found)

	// 测试空JSON标签的字段，应该使用默认字段名称
	fieldWithEmpty, found := form.Fields.GetField("fieldWithEmptyJSON")
	assert.True(t, found)
	assert.Equal(t, "fieldWithEmptyJSON", fieldWithEmpty.Name)
	assert.Equal(t, "空JSON标签", fieldWithEmpty.Label)
	assert.Equal(t, "JSON标签为空字符串", fieldWithEmpty.Desc)
}

// TestPrivateFieldConfiguration 测试私有字段的配置结构
type TestPrivateFieldConfiguration struct {
	PublicField  string `label:"公有字段" desc:"这是一个公有字段"`
	privateField string `label:"私有字段" desc:"这是一个私有字段"`
	AnotherPublic int   `label:"另一个公有字段" desc:"这是另一个公有字段"`
	anotherPrivate bool `label:"另一个私有字段" desc:"这是另一个私有字段"`
	IgnoredField string `json:"-" label:"被忽略的字段" desc:"这个字段应该被忽略"`
}

// TestPrivateFieldNode 测试私有字段的节点
type TestPrivateFieldNode struct {
	Config TestPrivateFieldConfiguration
}

func (x *TestPrivateFieldNode) Type() string {
	return "testPrivateField"
}

func (x *TestPrivateFieldNode) New() types.Node {
	return &TestPrivateFieldNode{}
}

func (x *TestPrivateFieldNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *TestPrivateFieldNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *TestPrivateFieldNode) Destroy() {}

func TestGetFieldsExcludePrivateFields(t *testing.T) {
	node := &TestPrivateFieldNode{}
	form := GetComponentForm(node)

	// 验证字段数量 - 应该只有2个公有字段，私有字段和json:"-"字段被排除
	assert.Equal(t, 2, len(form.Fields))

	// 验证公有字段存在
	publicField, found := form.Fields.GetField("publicField")
	assert.True(t, found)
	assert.Equal(t, "publicField", publicField.Name)
	assert.Equal(t, "公有字段", publicField.Label)
	assert.Equal(t, "这是一个公有字段", publicField.Desc)

	anotherPublicField, found := form.Fields.GetField("anotherPublic")
	assert.True(t, found)
	assert.Equal(t, "anotherPublic", anotherPublicField.Name)
	assert.Equal(t, "另一个公有字段", anotherPublicField.Label)
	assert.Equal(t, "这是另一个公有字段", anotherPublicField.Desc)

	// 验证私有字段不存在
	_, found = form.Fields.GetField("privateField")
	assert.False(t, found)

	_, found = form.Fields.GetField("anotherPrivate")
	assert.False(t, found)

	// 验证json:"-"字段不存在
	_, found = form.Fields.GetField("ignoredField")
	assert.False(t, found)
}

// TestJSONTagsConfiguration 测试JSON格式标签的配置结构
type TestJSONTagsConfiguration struct {
	// 测试JSON格式的rules标签
	RequiredField string `json:"required_field" label:"必填字段" desc:"测试JSON格式rules标签" rules:"[{\"required\":true,\"message\":\"此字段为必填项\"},{\"min\":3,\"message\":\"最少3个字符\"}]"`
	
	// 测试JSON格式的component标签
	SelectField string `json:"select_field" label:"选择字段" desc:"测试JSON格式component标签" component:"{\"type\":\"select\",\"filterable\":true,\"options\":[{\"label\":\"选项1\",\"value\":\"option1\"},{\"label\":\"选项2\",\"value\":\"option2\"}]}"`
	
	// 测试同时使用JSON格式的rules和component标签
	ComplexField int `json:"complex_field" label:"复杂字段" desc:"同时使用rules和component标签" rules:"[{\"required\":true,\"message\":\"必填\"},{\"min\":1,\"message\":\"最小值为1\"},{\"max\":100,\"message\":\"最大值为100\"}]" component:"{\"type\":\"number\",\"step\":1,\"placeholder\":\"请输入1-100的数字\"}"`
	
	// 测试required标签与JSON格式rules标签的组合
	MixedField string `json:"mixed_field" label:"混合字段" desc:"required标签与JSON格式rules标签组合" required:"true" rules:"[{\"pattern\":\"^[a-zA-Z]+$\",\"message\":\"只能包含字母\"}]"`
}

// TestJSONTagsNode 测试JSON格式标签的节点
type TestJSONTagsNode struct {
	Config TestJSONTagsConfiguration
}

// Type 返回节点类型
func (x *TestJSONTagsNode) Type() string {
	return "testJSONTags"
}

// New 创建新的节点实例
func (x *TestJSONTagsNode) New() types.Node {
	return &TestJSONTagsNode{Config: TestJSONTagsConfiguration{
		RequiredField: "default",
		SelectField:   "option1",
		ComplexField:  50,
		MixedField:    "test",
	}}
}

// Init 初始化节点
func (x *TestJSONTagsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

// OnMsg 处理消息
func (x *TestJSONTagsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

// Destroy 销毁节点
func (x *TestJSONTagsNode) Destroy() {}

// TestGetFieldsWithJSONTags 测试JSON格式的rules和component标签解析
func TestGetFieldsWithJSONTags(t *testing.T) {
	node := &TestJSONTagsNode{}
	componentForm := GetComponentForm(node)
	
	// 验证字段数量
	assert.Equal(t, 4, len(componentForm.Fields))
	
	// 测试RequiredField的JSON格式rules标签
	requiredField := componentForm.Fields[0]
	assert.Equal(t, "required_field", requiredField.Name)
	assert.Equal(t, "必填字段", requiredField.Label)
	assert.Equal(t, 2, len(requiredField.Rules))
	
	// 验证第一个规则
	rule1 := requiredField.Rules[0]
	assert.Equal(t, true, rule1["required"])
	assert.Equal(t, "此字段为必填项", rule1["message"])
	
	// 验证第二个规则
	rule2 := requiredField.Rules[1]
	assert.Equal(t, float64(3), rule2["min"]) // JSON解析数字为float64
	assert.Equal(t, "最少3个字符", rule2["message"])
	
	// 测试SelectField的JSON格式component标签
	selectField := componentForm.Fields[1]
	assert.Equal(t, "select_field", selectField.Name)
	assert.Equal(t, "选择字段", selectField.Label)
	assert.NotNil(t, selectField.Component)
	
	// 验证component配置
	component := selectField.Component
	assert.Equal(t, "select", component["type"])
	assert.Equal(t, true, component["filterable"])
	
	// 验证options数组
	options, ok := component["options"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(options))
	
	option1, ok := options[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "选项1", option1["label"])
	assert.Equal(t, "option1", option1["value"])
	
	// 测试ComplexField的复杂配置
	complexField := componentForm.Fields[2]
	assert.Equal(t, "complex_field", complexField.Name)
	assert.Equal(t, "复杂字段", complexField.Label)
	assert.Equal(t, 3, len(complexField.Rules))
	assert.NotNil(t, complexField.Component)
	
	// 验证复杂字段的component配置
	complexComponent := complexField.Component
	assert.Equal(t, "number", complexComponent["type"])
	assert.Equal(t, float64(1), complexComponent["step"])
	assert.Equal(t, "请输入1-100的数字", complexComponent["placeholder"])
	
	// 测试MixedField的required标签与JSON格式rules标签组合
	mixedField := componentForm.Fields[3]
	assert.Equal(t, "mixed_field", mixedField.Name)
	assert.Equal(t, "混合字段", mixedField.Label)
	assert.Equal(t, 2, len(mixedField.Rules)) // required标签生成的规则 + JSON格式rules标签的规则
	
	// 验证required标签生成的规则
	requiredRule := mixedField.Rules[0]
	assert.Equal(t, true, requiredRule["required"])
	assert.Equal(t, "This field is required", requiredRule["message"])
	
	// 验证JSON格式rules标签的规则
	patternRule := mixedField.Rules[1]
	assert.Equal(t, "^[a-zA-Z]+$", patternRule["pattern"])
	assert.Equal(t, "只能包含字母", patternRule["message"])
}
