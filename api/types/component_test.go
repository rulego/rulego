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

package types

import (
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func buildComponentFormList() ComponentFormList {
	return ComponentFormList{
		"bNode": {Type: "bNode", Category: "filter", Order: 2},
		"aNode": {Type: "aNode", Category: "filter", Order: 1},
		"cNode": {Type: "cNode", Category: "action"},
	}
}

// TestComponentFormListGetComponent 按类型名查找组件表单
func TestComponentFormListGetComponent(t *testing.T) {
	list := buildComponentFormList()
	form, ok := list.GetComponent("aNode")
	assert.True(t, ok)
	assert.Equal(t, "filter", form.Category)

	_, ok = list.GetComponent("missing")
	assert.False(t, ok)

	_, ok = ComponentFormList{}.GetComponent("aNode")
	assert.False(t, ok)
}

// TestComponentFormListValues 排序规则：类别 → Order → 类型
func TestComponentFormListValues(t *testing.T) {
	values := buildComponentFormList().Values()
	assert.Equal(t, 3, len(values))
	assert.Equal(t, "cNode", values[0].Type)
	assert.Equal(t, "aNode", values[1].Type)
	assert.Equal(t, "bNode", values[2].Type)

	sameOrder := ComponentFormList{
		"zNode": {Type: "zNode", Category: "filter"},
		"mNode": {Type: "mNode", Category: "filter"},
	}
	sorted := sameOrder.Values()
	assert.Equal(t, "mNode", sorted[0].Type)
	assert.Equal(t, "zNode", sorted[1].Type)

	assert.Equal(t, 0, len(ComponentFormList{}.Values()))
}

// TestComponentFormListGetByPage 分页与越界/非法参数
func TestComponentFormListGetByPage(t *testing.T) {
	list := buildComponentFormList()

	page, total, err := list.GetByPage(1, 2)
	assert.Nil(t, err)
	assert.Equal(t, 3, total)
	assert.Equal(t, 2, len(page))
	assert.Equal(t, "cNode", page[0].Type)

	page, total, err = list.GetByPage(2, 2)
	assert.Nil(t, err)
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, len(page))

	// 起始下标等于总数：空页不报错；超出总数：报错
	page, total, err = list.GetByPage(4, 1)
	assert.Nil(t, err)
	assert.Equal(t, 3, total)
	assert.Equal(t, 0, len(page))
	_, _, err = list.GetByPage(5, 1)
	assert.NotNil(t, err)

	_, _, err = list.GetByPage(0, 1)
	assert.NotNil(t, err)
	_, _, err = list.GetByPage(1, 0)
	assert.NotNil(t, err)

	page, total, err = ComponentFormList{}.GetByPage(1, 10)
	assert.Nil(t, err)
	assert.Equal(t, 0, total)
	assert.Nil(t, page)
}

// TestComponentFormFieldListGetField 按字段名查找
func TestComponentFormFieldListGetField(t *testing.T) {
	fields := ComponentFormFieldList{
		{Name: "path", Type: "string"},
		{Name: "port", Type: "int"},
	}
	field, ok := fields.GetField("port")
	assert.True(t, ok)
	assert.Equal(t, "int", field.Type)

	_, ok = fields.GetField("missing")
	assert.False(t, ok)
}

type noopComponent struct{}

func (n *noopComponent) Type() string { return "noop" }
func (n *noopComponent) New() Node    { return &noopComponent{} }
func (n *noopComponent) Init(ruleConfig Config, configuration Configuration) error {
	return nil
}
func (n *noopComponent) OnMsg(ctx RuleContext, msg RuleMsg) {}
func (n *noopComponent) Destroy()                           {}

// TestSafeComponentSlice 并发追加与快照读取
func TestSafeComponentSlice(t *testing.T) {
	var s SafeComponentSlice
	assert.Equal(t, 0, len(s.Components()))

	n1 := &noopComponent{}
	n2 := &noopComponent{}
	s.Add(n1)
	s.Add(n2)
	assert.Equal(t, 2, len(s.Components()))
	assert.Equal(t, n1, s.Components()[0])
}
