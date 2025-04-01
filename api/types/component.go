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
	"fmt"
	"sort"
	"sync"
)

const (
	// ComponentKindDynamic Dynamic component
	ComponentKindDynamic string = "dc"
	// ComponentKindNative Native component
	ComponentKindNative string = "nc"
	// ComponentKindEndpoint Endpoint component
	ComponentKindEndpoint string = "ec"
)

// ComponentDefGetter 该接口是可选的，组件可以实现该接口，提供可视化需要的信息，
// 例如：Label,Desc,RelationTypes。否则使用约定规则提供可视化表单定义
type ComponentDefGetter interface {
	Def() ComponentForm
}

// CategoryGetter 该接口是可选的，组件可以实现该接口，提供分类，
type CategoryGetter interface {
	Category() string
}

// DescGetter 该接口是可选的，组件可以实现该接口，提供组件描述，
type DescGetter interface {
	Desc() string
}

// ComponentFormList 组件表单列表
type ComponentFormList map[string]ComponentForm

func (c ComponentFormList) GetComponent(name string) (ComponentForm, bool) {
	for _, item := range c {
		if item.Type == name {
			return item, true
		}
	}
	return ComponentForm{}, false
}

func (c ComponentFormList) Values() []ComponentForm {
	var values []ComponentForm
	for _, item := range c {
		values = append(values, item)
	}
	// 先按Pkg排序，再按Type排序
	sort.Slice(values, func(i, j int) bool {
		// 如果两个元素的Pkg不同，就按Pkg的字典序比较
		if values[i].Category != values[j].Category {
			return values[i].Category < values[j].Category
		}
		// 否则，就按Type的字典序比较
		return values[i].Type < values[j].Type
	})
	return values
}

// GetByPage 根据分页获取数据
func (c ComponentFormList) GetByPage(page, pageSize int) ([]ComponentForm, int, error) {
	if page < 1 || pageSize < 1 {
		return nil, 0, fmt.Errorf("invalid page or pageSize")
	}

	values := c.Values()
	total := len(values)
	if total == 0 {
		return nil, 0, nil
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start > total {
		return nil, 0, fmt.Errorf("page out of range")
	}

	if end > total {
		end = total
	}

	return values[start:end], total, nil
}

// ComponentFormFieldList 字段列表类型
type ComponentFormFieldList []ComponentFormField

func (c ComponentFormFieldList) GetField(name string) (ComponentFormField, bool) {
	for _, field := range c {
		if field.Name == name {
			return field, true
		}
	}
	return ComponentFormField{}, false
}

// ComponentForm 组件表单，用于可视化加载组件表单
type ComponentForm struct {
	//Type 组件类型
	Type string `json:"type"`
	//Category 组件分类
	Category string `json:"category"`
	//配置字段,获取组件`Config`字段的所有公有字段
	Fields ComponentFormFieldList `json:"fields"`
	//Label 组件展示名称，预留，目前没值
	Label string `json:"label"`
	//Desc 组件说明，预留，目前没值
	Desc string `json:"desc"`
	//Icon 图标，预留，如果没值则取type。
	Icon string `json:"icon"`
	//RelationTypes 和下一个节点能产生的连接名称列表，
	//过滤器节点类型默认是：True/False/Failure；其他节点类型默认是Success/Failure
	//如果是空，表示用户可以自定义连接关系
	RelationTypes *[]string `json:"relationTypes"`
	//是否禁用，如果禁用在editor不显示
	Disabled bool `json:"disabled"`
	//版本
	Version string `json:"version"`
	// 组件种类，dc:动态组件 nc:原生组件 ec:endpoint组件
	ComponentKind string `json:"componentKind"`
}

// ComponentFormField 组件配置字段
type ComponentFormField struct {
	//Name 字段名称
	Name string `json:"name"`
	//Type 字段类型
	Type string `json:"type"`
	//默认值，组件实现的方法node.New(), Config对应的字段，提供了默认值会填充到该值
	DefaultValue interface{} `json:"defaultValue"`
	//Label 字段展示名称，通过tag:label获取
	Label string `json:"label"`
	//Desc 字段说明，通过tag:desc获取
	Desc string `json:"desc"`
	//Validate 校验规则，通过tag:validate获取
	//Deprecated: 使用 Rules 代替
	Validate string `json:"validate"`
	//Rules 前端界面校验规则
	// 示例: [
	// {required: true, message: '该字段是必须的'},
	// ]
	Rules []map[string]interface{} `json:"rules"`
	//Fields 嵌套字段
	Fields ComponentFormFieldList `json:"fields"`
	//表单组件配置
	//示例:{
	//	"type": "codeEditor",
	//}
	Component map[string]interface{} `json:"component"`
	//是否必填，通过tag:required获取
	Required bool `json:"required"`
}

// SafeComponentSlice 安全的组件列表切片
type SafeComponentSlice struct {
	//组件列表
	components []Node
	sync.Mutex
}

// Add 线程安全地添加元素
func (p *SafeComponentSlice) Add(nodes ...Node) {
	p.Lock()
	defer p.Unlock()
	for _, node := range nodes {
		p.components = append(p.components, node)
	}
}

// Components 获取组件列表
func (p *SafeComponentSlice) Components() []Node {
	p.Lock()
	defer p.Unlock()
	return p.components
}
