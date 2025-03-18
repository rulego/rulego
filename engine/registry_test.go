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

package engine

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestGetComponentsFields(t *testing.T) {
	Registry.Register(&NoConfigNode{})
	Registry.Register(&ConfigHasPtrNode{})
	Registry.Register(&ConfigHasPtrNode2{})
	Registry.Register(&DefaultValueNode{})
	items := Registry.GetComponentForms()
	componentForm, ok := items.GetComponent("test/configHasPtr")
	assert.Equal(t, true, ok)
	componentForm.Label = "有指针配置"
	items[componentForm.Type] = componentForm
	componentForm, _ = items.GetComponent("test/configHasPtr")
	assert.Equal(t, "有指针配置", componentForm.Label)
	for _, item := range items {
		if item.Type == "test/defaultConfig" {
			assert.Equal(t, 10, len(item.Fields))
			urlField, ok := item.Fields.GetField("url")
			assert.Equal(t, true, ok)
			assert.Equal(t, "string", urlField.Type)
			assert.Equal(t, "required", urlField.Validate)
			assert.Equal(t, "服务器地址", urlField.Label)
			assert.Equal(t, "broker服务器地址", urlField.Desc)

			structField, ok := item.Fields.GetField("e")

			assert.Equal(t, true, ok)
			assert.Equal(t, "struct", structField.Type)
			assert.Equal(t, 1, len(structField.Fields))
			assert.Equal(t, "测试", structField.Fields[0].DefaultValue)
		}
	}

	nodes := Registry.GetComponents()
	length := len(nodes)
	assert.True(t, len(nodes) > 0)

	Registry.Unregister("test/configHasPtr")
	lengthNew := len(Registry.GetComponents())
	assert.Equal(t, length-1, lengthNew)
}

func TestCustomComponentRegistry(t *testing.T) {
	// 创建默认和自定义注册表
	defaultReg := new(RuleComponentRegistry)
	customReg := new(RuleComponentRegistry)
	compositeReg := NewCustomComponentRegistry(defaultReg, customReg)

	// 初始化测试组件
	defaultNode := &NoConfigNode{}
	customNode := &ConfigHasPtrNode{}

	// 测试注册逻辑
	t.Run("Register Components", func(t *testing.T) {
		// 注册默认组件
		assert.Nil(t, defaultReg.Register(defaultNode))
		// 注册自定义组件
		assert.Nil(t, compositeReg.Register(customNode))
	})

	t.Run("NewNode Fallback Logic", func(t *testing.T) {
		// 测试获取自定义组件
		node, err := compositeReg.NewNode("test/configHasPtr")
		assert.Nil(t, err)
		assert.Equal(t, customNode.Type(), node.Type())

		// 测试默认组件回退
		node, err = compositeReg.NewNode("test/noConfig")
		assert.Nil(t, err)
		assert.Equal(t, defaultNode.Type(), node.Type())

		// 测试不存在的组件
		_, err = compositeReg.NewNode("invalid_type")
		assert.NotNil(t, err)
	})

	t.Run("Component Merging", func(t *testing.T) {
		components := compositeReg.GetComponents()
		// 验证合并结果
		assert.Equal(t, 2, len(components))
		_, ok := components["test/noConfig"]
		assert.True(t, ok)
		_, ok = components["test/configHasPtr"]
		assert.True(t, ok)
	})

	t.Run("Unregister Custom", func(t *testing.T) {
		// 卸载自定义组件
		assert.Nil(t, compositeReg.Unregister("test/configHasPtr"))
		_, err := compositeReg.NewNode("test/configHasPtr")
		assert.NotNil(t, err)

		// 默认组件应仍然存在
		_, err = compositeReg.NewNode("test/noConfig")
		assert.Nil(t, err)
	})

	// 清理测试组件
	defer func() {
		defaultReg.Unregister("test/noConfig")
		customReg.Unregister("test/configHasPtr")
	}()
}

//以下是测试组件

type BaseNode struct {
}

func (n *BaseNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	return nil
}

func (n *BaseNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
}

func (n *BaseNode) Destroy() {
}

// 没配置节点
type NoConfigNode struct {
	BaseNode
}

func (n *NoConfigNode) Type() string {
	return "test/noConfig"
}
func (n *NoConfigNode) New() types.Node {
	return &NoConfigNode{}
}

// 有指针配置
type ConfigHasPtrConfig struct {
	Num *int
	Url string
}

type ConfigHasPtrNode struct {
	BaseNode
	config ConfigHasPtrConfig
}

func (n *ConfigHasPtrNode) Type() string {
	return "test/configHasPtr"
}
func (n *ConfigHasPtrNode) New() types.Node {
	return &ConfigHasPtrNode{}
}

// config 大写
type ConfigHasPtrNode2 struct {
	BaseNode
	Config ConfigHasPtrConfig
}

func (n *ConfigHasPtrNode2) Type() string {
	return "test/configHasPtr2"
}
func (n *ConfigHasPtrNode2) New() types.Node {
	return &ConfigHasPtrNode2{}
}

// 有默认值配置
type DefaultValueConfig struct {
	Num    int
	Url    string `label:"服务器地址" desc:"broker服务器地址" validate:"required" `
	IsSsl  bool
	Params []string
	A      int32
	B      int64
	C      float64
	D      map[string]string
	E      TestE
	F      uint16
}
type TestE struct {
	A string
}
type DefaultValueNode struct {
	BaseNode
	Config DefaultValueConfig
}

func (n *DefaultValueNode) Type() string {
	return "test/defaultConfig"
}

func (n *DefaultValueNode) New() types.Node {
	return &DefaultValueNode{
		Config: DefaultValueConfig{
			Url: "http://localhost:8080",
			Num: 5,
			E: TestE{
				A: "测试",
			},
		},
	}
}

func (n *DefaultValueNode) Def() types.ComponentForm {
	relationTypes := &[]string{"aa", "bb"}
	return types.ComponentForm{
		Label:         "默认测试组件",
		Desc:          "用法xxxxx",
		RelationTypes: relationTypes,
	}
}
