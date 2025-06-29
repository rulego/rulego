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

// Component kind constants define the different types of components in the RuleGo ecosystem.
// 组件类型常量定义了 RuleGo 生态系统中不同类型的组件。
const (
	// ComponentKindDynamic represents a dynamic component that can be loaded at runtime
	// ComponentKindDynamic 表示可以在运行时加载的动态组件
	ComponentKindDynamic string = "dc"

	// ComponentKindNative represents a native component that is built into the system
	// ComponentKindNative 表示内置在系统中的原生组件
	ComponentKindNative string = "nc"

	// ComponentKindEndpoint represents an endpoint component for input/output operations
	// ComponentKindEndpoint 表示用于输入/输出操作的端点组件
	ComponentKindEndpoint string = "ec"
)

// ComponentDefGetter is an optional interface that components can implement to provide
// metadata for visual configuration tools such as Label, Description, and RelationTypes.
// If not implemented, conventional rules are used to provide visual form definitions.
//
// ComponentDefGetter 是组件可以实现的可选接口，用于为可视化配置工具提供元数据，
// 如标签、描述和关系类型。如果未实现，则使用约定规则提供可视化表单定义。
//
// Example implementation:
// 实现示例：
//
//	func (n *MyNode) Def() ComponentForm {
//		return ComponentForm{
//			Type:     "myNode",
//			Category: "transform",
//			Label:    "My Custom Node",
//			Desc:     "A custom transformation node",
//		}
//	}
type ComponentDefGetter interface {
	// Def returns the component form definition for visual configuration
	// Def 返回用于可视化配置的组件表单定义
	Def() ComponentForm
}

// CategoryGetter is an optional interface that components can implement to provide
// category information for organizing components in visual tools.
//
// CategoryGetter 是组件可以实现的可选接口，用于提供分类信息，
// 在可视化工具中组织组件。
type CategoryGetter interface {
	// Category returns the category name for this component
	// Category 返回此组件的类别名称
	Category() string
}

// DescGetter is an optional interface that components can implement to provide
// a description of the component's functionality.
//
// DescGetter 是组件可以实现的可选接口，用于提供组件功能的描述。
type DescGetter interface {
	// Desc returns a description of the component
	// Desc 返回组件的描述
	Desc() string
}

// ComponentFormList represents a collection of component forms indexed by component type.
// It provides methods for managing and querying component configurations.
//
// ComponentFormList 表示按组件类型索引的组件表单集合。
// 它提供了管理和查询组件配置的方法。
type ComponentFormList map[string]ComponentForm

// GetComponent retrieves a component form by its type name.
// Returns the component form and a boolean indicating whether it was found.
//
// GetComponent 通过类型名称检索组件表单。
// 返回组件表单和表示是否找到的布尔值。
func (c ComponentFormList) GetComponent(name string) (ComponentForm, bool) {
	for _, item := range c {
		if item.Type == name {
			return item, true
		}
	}
	return ComponentForm{}, false
}

// Values returns all component forms sorted by category and then by type.
// This provides a consistent ordering for UI display purposes.
//
// Values 返回按类别然后按类型排序的所有组件表单。
// 这为 UI 显示提供了一致的排序。
func (c ComponentFormList) Values() []ComponentForm {
	var values []ComponentForm
	for _, item := range c {
		values = append(values, item)
	}
	// Sort by category first, then by type
	// 先按类别排序，再按类型排序
	sort.Slice(values, func(i, j int) bool {
		// If categories are different, sort by category
		// 如果类别不同，按类别排序
		if values[i].Category != values[j].Category {
			return values[i].Category < values[j].Category
		}
		// Otherwise, sort by type
		// 否则，按类型排序
		return values[i].Type < values[j].Type
	})
	return values
}

// GetByPage returns component forms with pagination support.
// Returns the forms for the specified page, total count, and any error.
//
// GetByPage 返回带分页支持的组件表单。
// 返回指定页面的表单、总数和任何错误。
//
// Parameters:
// 参数：
//   - page: Page number (1-based)  页码（从1开始）
//   - pageSize: Number of items per page  每页项目数
//
// Returns:
// 返回：
//   - []ComponentForm: The component forms for the requested page  请求页面的组件表单
//   - int: Total number of available forms  可用表单的总数
//   - error: Any error that occurred  发生的任何错误
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

// ComponentFormFieldList represents a list of component form fields.
// It provides methods for managing and querying field configurations.
//
// ComponentFormFieldList 表示组件表单字段列表。
// 它提供了管理和查询字段配置的方法。
type ComponentFormFieldList []ComponentFormField

// GetField retrieves a field by its name.
// Returns the field and a boolean indicating whether it was found.
//
// GetField 通过名称检索字段。
// 返回字段和表示是否找到的布尔值。
func (c ComponentFormFieldList) GetField(name string) (ComponentFormField, bool) {
	for _, field := range c {
		if field.Name == name {
			return field, true
		}
	}
	return ComponentFormField{}, false
}

// ComponentForm represents the metadata and configuration structure for a component.
// It is used by visual configuration tools to generate appropriate UI forms.
//
// ComponentForm 表示组件的元数据和配置结构。
// 它被可视化配置工具用来生成适当的 UI 表单。
type ComponentForm struct {
	// Type is the unique identifier for the component type
	// Type 是组件类型的唯一标识符
	Type string `json:"type"`

	// Category is the classification category for organizing components
	// Category 是用于组织组件的分类类别
	Category string `json:"category"`

	// Fields contains the configuration fields extracted from the component's Config struct
	// Fields 包含从组件的 Config 结构体中提取的配置字段
	Fields ComponentFormFieldList `json:"fields"`

	// Label is the display name for the component (reserved for future use)
	// Label 是组件的显示名称（保留供将来使用）
	Label string `json:"label"`

	// Desc is the description of the component (reserved for future use)
	// Desc 是组件的描述（保留供将来使用）
	Desc string `json:"desc"`

	// Icon is the icon identifier for the component (defaults to type if empty)
	// Icon 是组件的图标标识符（如果为空则默认为类型）
	Icon string `json:"icon"`

	// RelationTypes defines the possible connection names to the next node.
	// For filter nodes, defaults to: True/False/Failure
	// For other nodes, defaults to: Success/Failure
	// If nil, users can define custom relationship types
	// RelationTypes 定义与下一个节点的可能连接名称。
	// 对于过滤器节点，默认为：True/False/Failure
	// 对于其他节点，默认为：Success/Failure
	// 如果为 nil，用户可以定义自定义关系类型
	RelationTypes *[]string `json:"relationTypes"`

	// Disabled indicates whether the component should be hidden in the editor
	// Disabled 表示组件是否应在编辑器中隐藏
	Disabled bool `json:"disabled"`

	// Version is the version of the component
	// Version 是组件的版本
	Version string `json:"version"`

	// ComponentKind indicates the type of component: dc (dynamic), nc (native), ec (endpoint)
	// ComponentKind 表示组件类型：dc（动态）、nc（原生）、ec（端点）
	ComponentKind string `json:"componentKind"`
}

// ComponentFormField represents a single configuration field in a component form.
// It contains metadata about the field type, validation rules, and UI presentation.
//
// ComponentFormField 表示组件表单中的单个配置字段。
// 它包含有关字段类型、验证规则和 UI 表示的元数据。
type ComponentFormField struct {
	// Name is the field name corresponding to the struct field
	// Name 是对应于结构体字段的字段名称
	Name string `json:"name"`

	// Type is the data type of the field (string, int, bool, etc.)
	// Type 是字段的数据类型（string、int、bool 等）
	Type string `json:"type"`

	// DefaultValue is the default value provided by the component's New() method
	// DefaultValue 是组件的 New() 方法提供的默认值
	DefaultValue interface{} `json:"defaultValue"`

	// Label is the display name for the field, extracted from the 'label' tag
	// Label 是字段的显示名称，从 'label' 标签中提取
	Label string `json:"label"`

	// Desc is the description of the field, extracted from the 'desc' tag
	// Desc 是字段的描述，从 'desc' 标签中提取
	Desc string `json:"desc"`

	// Validate contains validation rules, extracted from the 'validate' tag
	// Deprecated: Use Rules instead
	// Validate 包含验证规则，从 'validate' 标签中提取
	// 已废弃：使用 Rules 代替
	Validate string `json:"validate"`

	// Rules contains frontend validation rules
	// Rules 包含前端验证规则
	// Example: [{"required": true, "message": "This field is required"}]
	// 示例：[{"required": true, "message": "This field is required"}]
	Rules []map[string]interface{} `json:"rules"`

	// Fields contains nested fields for complex objects
	// Fields 包含复杂对象的嵌套字段
	Fields ComponentFormFieldList `json:"fields"`

	// Component contains UI component configuration for rendering
	// Component 包含用于渲染的 UI 组件配置
	// Example: {"type": "codeEditor", "language": "javascript"}
	// 示例：{"type": "codeEditor", "language": "javascript"}
	Component map[string]interface{} `json:"component"`

	// Required indicates whether the field is mandatory, extracted from the 'required' tag
	// Required 表示字段是否为必填项，从 'required' 标签中提取
	Required bool `json:"required"`
}

// SafeComponentSlice provides a thread-safe slice for storing Node components.
// It uses mutex synchronization to ensure safe concurrent access.
//
// SafeComponentSlice 提供了用于存储 Node 组件的线程安全切片。
// 它使用互斥锁同步来确保安全的并发访问。
type SafeComponentSlice struct {
	// components holds the list of Node components
	// components 保存 Node 组件列表
	components []Node
	sync.Mutex
}

// Add safely appends one or more Node components to the slice.
// This method is thread-safe and can be called concurrently.
//
// Add 安全地将一个或多个 Node 组件追加到切片中。
// 此方法是线程安全的，可以并发调用。
func (p *SafeComponentSlice) Add(nodes ...Node) {
	p.Lock()
	defer p.Unlock()
	for _, node := range nodes {
		p.components = append(p.components, node)
	}
}

// Components returns a copy of the current component list.
// This method is thread-safe and returns a snapshot of the components.
//
// Components 返回当前组件列表的副本。
// 此方法是线程安全的，返回组件的快照。
func (p *SafeComponentSlice) Components() []Node {
	p.Lock()
	defer p.Unlock()
	return p.components
}
