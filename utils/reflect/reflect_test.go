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
