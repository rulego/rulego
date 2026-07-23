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
	"testing"

	"github.com/rulego/rulego/test/assert"

	"github.com/rulego/rulego/api/types"
)

// FunctionsNodeConfiguration: node configuration
type FunctionsNodeConfiguration struct {
	FunctionName string `label:"函数名称" desc:"调用的函数名称" required:"true"`
}

// FunctionsNode is implemented as a test node
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

// TestJSONTagConfiguration Tests the configuration structure of JSON tag priority
type TestJSONTagConfiguration struct {
	FieldWithJSONTag    string `json:"custom_field_name" label:"带JSON标签的字段" desc:"使用JSON标签名称"`
	FieldWithoutJSONTag string `label:"不带JSON标签的字段" desc:"使用默认字段名称"`
	FieldWithJSONOmit   string `json:"omit_field,omitempty" label:"带omitempty的字段" desc:"JSON标签包含omitempty"`
	FieldWithJSONIgnore string `json:"-" label:"忽略的字段" desc:"JSON标签为-"`
	FieldWithEmptyJSON  string `json:"" label:"空JSON标签" desc:"JSON标签为空字符串"`
}

// TestJSONTagNode tests the node of the JSON tag
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

	// Validate field count - There should only be 4 fields, because fields with json:"-" are excluded
	assert.Equal(t, 4, len(form.Fields))

	// When testing fields with JSON tags, you should use the JSON tag name
	fieldWithJSON, found := form.Fields.GetField("custom_field_name")
	assert.True(t, found)
	assert.Equal(t, "custom_field_name", fieldWithJSON.Name)
	assert.Equal(t, "带JSON标签的字段", fieldWithJSON.Label)
	assert.Equal(t, "使用JSON标签名称", fieldWithJSON.Desc)

	// For fields without JSON tags, the default field name (lowercase) should be used.
	fieldWithoutJSON, found := form.Fields.GetField("fieldWithoutJSONTag")
	assert.True(t, found)
	assert.Equal(t, "fieldWithoutJSONTag", fieldWithoutJSON.Name)
	assert.Equal(t, "不带JSON标签的字段", fieldWithoutJSON.Label)
	assert.Equal(t, "使用默认字段名称", fieldWithoutJSON.Desc)

	// For fields testing with omitempty, you should use the JSON tag name (remove omitempty).
	fieldWithOmit, found := form.Fields.GetField("omit_field")
	assert.True(t, found)
	assert.Equal(t, "omit_field", fieldWithOmit.Name)
	assert.Equal(t, "带omitempty的字段", fieldWithOmit.Label)
	assert.Equal(t, "JSON标签包含omitempty", fieldWithOmit.Desc)

	// Fields labeled as "-" in JSON should be excluded and should not appear in the results
	_, found = form.Fields.GetField("fieldWithJSONIgnore")
	assert.False(t, found)

	// Fields for the test empty JSON tag should use the default field names
	fieldWithEmpty, found := form.Fields.GetField("fieldWithEmptyJSON")
	assert.True(t, found)
	assert.Equal(t, "fieldWithEmptyJSON", fieldWithEmpty.Name)
	assert.Equal(t, "空JSON标签", fieldWithEmpty.Label)
	assert.Equal(t, "JSON标签为空字符串", fieldWithEmpty.Desc)
}

// TestPrivateFieldConfiguration tests the configuration structure of private fields
type TestPrivateFieldConfiguration struct {
	PublicField    string `label:"公有字段" desc:"这是一个公有字段"`
	privateField   string `label:"私有字段" desc:"这是一个私有字段"`
	AnotherPublic  int    `label:"另一个公有字段" desc:"这是另一个公有字段"`
	anotherPrivate bool   `label:"另一个私有字段" desc:"这是另一个私有字段"`
	IgnoredField   string `json:"-" label:"被忽略的字段" desc:"这个字段应该被忽略"`
}

// TestPrivateFieldNode tests the node of the private field
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

	// Validate field count - There should only be 2 public fields, private fields, and the json: "-" field excluded
	assert.Equal(t, 2, len(form.Fields))

	// Verify the existence of public fields
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

	// Verify that private fields do not exist
	_, found = form.Fields.GetField("privateField")
	assert.False(t, found)

	_, found = form.Fields.GetField("anotherPrivate")
	assert.False(t, found)

	// Verify json: The "-" field does not exist
	_, found = form.Fields.GetField("ignoredField")
	assert.False(t, found)
}

// TestJSONTagsConfiguration tests the configuration structure of JSON-formatted tags
type TestJSONTagsConfiguration struct {
	// Test the rules tag in JSON format
	RequiredField string `json:"required_field" label:"必填字段" desc:"测试JSON格式rules标签" rules:"[{\"required\":true,\"message\":\"此字段为必填项\"},{\"min\":3,\"message\":\"最少3个字符\"}]"`

	// Test the component tag in JSON format
	SelectField string `json:"select_field" label:"选择字段" desc:"测试JSON格式component标签" component:"{\"type\":\"select\",\"filterable\":true,\"options\":[{\"label\":\"选项1\",\"value\":\"option1\"},{\"label\":\"选项2\",\"value\":\"option2\"}]}"`

	// Test using JSON-format rules and component tags simultaneously
	ComplexField int `json:"complex_field" label:"复杂字段" desc:"同时使用rules和component标签" rules:"[{\"required\":true,\"message\":\"必填\"},{\"min\":1,\"message\":\"最小值为1\"},{\"max\":100,\"message\":\"最大值为100\"}]" component:"{\"type\":\"number\",\"step\":1,\"placeholder\":\"请输入1-100的数字\"}"`

	// Test the combination of the required tag and the JSON-formatted rules tag
	MixedField string `json:"mixed_field" label:"混合字段" desc:"required标签与JSON格式rules标签组合" required:"true" rules:"[{\"pattern\":\"^[a-zA-Z]+$\",\"message\":\"只能包含字母\"}]"`
}

// TestJSONTagsNode Tests the node of the JSON-formatted tag
type TestJSONTagsNode struct {
	Config TestJSONTagsConfiguration
}

// Type returns the node type
func (x *TestJSONTagsNode) Type() string {
	return "testJSONTags"
}

// New creates a new node instance
func (x *TestJSONTagsNode) New() types.Node {
	return &TestJSONTagsNode{Config: TestJSONTagsConfiguration{
		RequiredField: "default",
		SelectField:   "option1",
		ComplexField:  50,
		MixedField:    "test",
	}}
}

// Init initializes the node
func (x *TestJSONTagsNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

// OnMsg processes a message
func (x *TestJSONTagsNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

// Destroy the node
func (x *TestJSONTagsNode) Destroy() {}

// TestGetFieldsWithJSONTags tests the parsing of JSON rules and component tags
func TestGetFieldsWithJSONTags(t *testing.T) {
	node := &TestJSONTagsNode{}
	componentForm := GetComponentForm(node)

	// Verify the number of fields
	assert.Equal(t, 4, len(componentForm.Fields))

	// Test the JSON-formatted rules tag of the RequiredField
	requiredField := componentForm.Fields[0]
	assert.Equal(t, "required_field", requiredField.Name)
	assert.Equal(t, "必填字段", requiredField.Label)
	assert.Equal(t, 2, len(requiredField.Rules))

	// Verify the first rule
	rule1 := requiredField.Rules[0]
	assert.Equal(t, true, rule1["required"])
	assert.Equal(t, "此字段为必填项", rule1["message"])

	// Verify the second rule
	rule2 := requiredField.Rules[1]
	assert.Equal(t, float64(3), rule2["min"]) // The JSON parsing number is float64
	assert.Equal(t, "最少3个字符", rule2["message"])

	// Test the JSON-formatted component tag of SelectField
	selectField := componentForm.Fields[1]
	assert.Equal(t, "select_field", selectField.Name)
	assert.Equal(t, "选择字段", selectField.Label)
	assert.NotNil(t, selectField.Component)

	// Verify component configuration
	component := selectField.Component
	assert.Equal(t, "select", component["type"])
	assert.Equal(t, true, component["filterable"])

	// Validate the options array
	options, ok := component["options"].([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(options))

	// Verification options[0]
	option1, ok := options[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "选项1", option1["label"])
	assert.Equal(t, "option1", option1["value"])

	// Test the complex configuration of ComplexField
	complexField := componentForm.Fields[2]
	assert.Equal(t, "complex_field", complexField.Name)
	assert.Equal(t, "复杂字段", complexField.Label)
	assert.Equal(t, 3, len(complexField.Rules))
	assert.NotNil(t, complexField.Component)

	// Verify the component configuration for complex fields
	complexComponent := complexField.Component
	assert.Equal(t, "number", complexComponent["type"])
	assert.Equal(t, float64(1), complexComponent["step"])
	assert.Equal(t, "请输入1-100的数字", complexComponent["placeholder"])

	// Test the combination of the MixedField required tag and the JSON-format rules tag
	mixedField := componentForm.Fields[3]
	assert.Equal(t, "mixed_field", mixedField.Name)
	assert.Equal(t, "混合字段", mixedField.Label)
	assert.Equal(t, 2, len(mixedField.Rules)) // Rules generated by the required tag + rules for the JSON-format tag

	// Validate the rules generated by the required tag
	requiredRule := mixedField.Rules[0]
	assert.Equal(t, true, requiredRule["required"])
	assert.Equal(t, "This field is required", requiredRule["message"])

	// Verify the rules of the JSON format rules tag
	patternRule := mixedField.Rules[1]
	assert.Equal(t, "^[a-zA-Z]+$", patternRule["pattern"])
	assert.Equal(t, "只能包含字母", patternRule["message"])
}

// SquashConfig is used to test the configuration structure of squash tags
type SquashConfig struct {
	BaseConfig `mapstructure:",squash"`
	OtherField string `json:"otherField" label:"其他字段"`
}

type BaseConfig struct {
	BaseField string `json:"baseField" label:"基础字段"`
}

// SquashNode tests the node of the squash tag
type SquashNode struct {
	Config SquashConfig
}

func (x *SquashNode) Type() string {
	return "squash"
}

func (x *SquashNode) New() types.Node {
	return &SquashNode{
		Config: SquashConfig{
			BaseConfig: BaseConfig{
				BaseField: "base",
			},
			OtherField: "other",
		},
	}
}

func (x *SquashNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *SquashNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *SquashNode) Destroy() {}

func TestGetFieldsWithSquash(t *testing.T) {
	node := &SquashNode{}
	form := GetComponentForm(node)

	// Number of validated fields - should be 2 fields, BaseField and OtherField, BaseConfig is tiled
	assert.Equal(t, 2, len(form.Fields))

	// Verify that BaseField exists and is at the top level
	baseField, found := form.Fields.GetField("baseField")
	assert.True(t, found)
	assert.Equal(t, "baseField", baseField.Name)
	assert.Equal(t, "基础字段", baseField.Label)

	// Verify the existence of OtherField
	otherField, found := form.Fields.GetField("otherField")
	assert.True(t, found)
	assert.Equal(t, "otherField", otherField.Name)
	assert.Equal(t, "其他字段", otherField.Label)
}

// JsonSquashConfig is used to test the configuration structure of json squash tags
type JsonSquashConfig struct {
	BaseConfig `json:",squash"`
	OtherField string `json:"otherField" label:"其他字段"`
}

// JsonSquashNode tests the node of the json squash tag
type JsonSquashNode struct {
	Config JsonSquashConfig
}

func (x *JsonSquashNode) Type() string {
	return "jsonSquash"
}

func (x *JsonSquashNode) New() types.Node {
	return &JsonSquashNode{
		Config: JsonSquashConfig{
			BaseConfig: BaseConfig{
				BaseField: "base",
			},
			OtherField: "other",
		},
	}
}

func (x *JsonSquashNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *JsonSquashNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *JsonSquashNode) Destroy() {}

func TestGetFieldsWithJsonSquash(t *testing.T) {
	node := &JsonSquashNode{}
	form := GetComponentForm(node)

	// Number of validated fields - should be 2 fields, BaseField and OtherField, BaseConfig is tiled
	assert.Equal(t, 2, len(form.Fields))

	// Verify that BaseField exists and is at the top level
	baseField, found := form.Fields.GetField("baseField")
	assert.True(t, found)
	assert.Equal(t, "baseField", baseField.Name)
	assert.Equal(t, "基础字段", baseField.Label)

	// Verify the existence of OtherField
	otherField, found := form.Fields.GetField("otherField")
	assert.True(t, found)
	assert.Equal(t, "otherField", otherField.Name)
	assert.Equal(t, "其他字段", otherField.Label)
}

// TestRefTagConfiguration Tests the configuration structure of ref tags
type TestRefTagConfiguration struct {
	Server   string `json:"server" label:"Server" ref:"primary"`
	Username string `json:"username" label:"Username" ref:"shared"`
	Password string `json:"password" label:"Password" ref:"shared"`
	Topic    string `json:"topic" label:"Topic"`
}

// TestRefTagNode tests the node of ref tag
type TestRefTagNode struct {
	Config TestRefTagConfiguration
}

func (x *TestRefTagNode) Type() string {
	return "testRefTag"
}

func (x *TestRefTagNode) New() types.Node {
	return &TestRefTagNode{Config: TestRefTagConfiguration{
		Server: "127.0.0.1:1883",
		Topic:  "/device/msg",
	}}
}

func (x *TestRefTagNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *TestRefTagNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *TestRefTagNode) Destroy() {}

func TestGetFieldsWithRefTag(t *testing.T) {
	node := &TestRefTagNode{}
	form := GetComponentForm(node)

	assert.Equal(t, "testRefTag", form.Type)
	assert.Equal(t, 4, len(form.Fields))

	// server: ref=primary
	serverField, found := form.Fields.GetField("server")
	assert.True(t, found)
	assert.Equal(t, "primary", serverField.Ref)

	// username: ref=shared
	usernameField, found := form.Fields.GetField("username")
	assert.True(t, found)
	assert.Equal(t, "shared", usernameField.Ref)

	// password: ref=shared
	passwordField, found := form.Fields.GetField("password")
	assert.True(t, found)
	assert.Equal(t, "shared", passwordField.Ref)

	// topic: No ref mark
	topicField, found := form.Fields.GetField("topic")
	assert.True(t, found)
	assert.Equal(t, "", topicField.Ref)
}

// TestNoRefTagConfiguration tests without a ref tag configuration
type TestNoRefTagConfiguration struct {
	Server string `json:"server" label:"Server"`
}

type TestNoRefTagNode struct {
	Config TestNoRefTagConfiguration
}

func (x *TestNoRefTagNode) Type() string {
	return "testNoRefTag"
}

func (x *TestNoRefTagNode) New() types.Node { return &TestNoRefTagNode{} }

func (x *TestNoRefTagNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (x *TestNoRefTagNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {}

func (x *TestNoRefTagNode) Destroy() {}

func TestRefTagEmptyWhenNotSet(t *testing.T) {
	node := &TestNoRefTagNode{}
	form := GetComponentForm(node)

	assert.Equal(t, 1, len(form.Fields))
	assert.Equal(t, "", form.Fields[0].Ref)
}
