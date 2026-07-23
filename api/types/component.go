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
// Component type constants define different types of components in the RuleGo ecosystem.
const (
	// ComponentKindDynamic represents a dynamic component that can be loaded at runtime
	// ComponentKindDynamic represents dynamic components that can be loaded at runtime
	ComponentKindDynamic string = "dc"

	// ComponentKindNative represents a native component that is built into the system
	// ComponentKindNative refers to native components built into the system
	ComponentKindNative string = "nc"

	// ComponentKindEndpoint represents an endpoint component for input/output operations
	// ComponentKindEndpoint represents the endpoint component used for input/output operations
	ComponentKindEndpoint string = "ec"
)

// ComponentDefGetter is an optional interface that components can implement to provide
// metadata for visual configuration tools such as Label, Description, and RelationTypes.
// If not implemented, conventional rules are used to provide visual form definitions.
//
// ComponentDefGetter is an optional interface that components can implement, used to provide metadata for the visualization configuration tool,
// Such as tags, descriptions, and relationship types. If not implemented, a visual form definition is provided using convention rules.
//
// Example implementation:
// Implementation example:
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
	// def returns the component form definition used for visualizing the configuration
	Def() ComponentForm
}

// CategoryGetter is an optional interface that components can implement to provide
// category information for organizing components in visual tools.
//
// CategoryGetter is an optional interface that components can implement to provide classification information,
// Organize components within visualization tools.
type CategoryGetter interface {
	// Category returns the category name for this component
	// Category returns the category name of this component
	Category() string
}

// DescGetter is an optional interface that components can implement to provide
// a description of the component's functionality.
//
// DescGetter is an optional interface that components can implement, used to provide descriptions of component functionality.
type DescGetter interface {
	// Desc returns a description of the component
	// Desc returns the description of the component
	Desc() string
}

// ComponentFormList represents a collection of component forms indexed by component type.
// It provides methods for managing and querying component configurations.
//
// ComponentFormList represents a collection of component forms indexed by component type.
// It provides methods for managing and querying component configurations.
type ComponentFormList map[string]ComponentForm

// GetComponent retrieves a component form by its type name.
// Returns the component form and a boolean indicating whether it was found.
//
// GetComponent retrieves component forms by type name.
// Returns the component form and indicates whether the boolean value was found.
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
// Values returns all component forms sorted by category and then by type.
// This provides consistent sorting for UI display.
func (c ComponentFormList) Values() []ComponentForm {
	var values []ComponentForm
	for _, item := range c {
		values = append(values, item)
	}
	// Sort by category first, then by type
	// Sort by category first, then by type
	sort.Slice(values, func(i, j int) bool {
		// If categories are different, sort by category
		// If the categories are different, sort by category
		if values[i].Category != values[j].Category {
			return values[i].Category < values[j].Category
		}
		// Otherwise, sort by type
		// Otherwise, sort by type
		return values[i].Type < values[j].Type
	})
	return values
}

// GetByPage returns component forms with pagination support.
// Returns the forms for the specified page, total count, and any error.
//
// GetByPage returns component forms with pagination support.
// Returns the form, total number, and any errors on the specified page.
//
// Parameters:
// Parameters:
//   - page: Page number (1-based)
//   - pageSize: Number of items per page
//
// Returns:
// Returns:
//   - []ComponentForm: The component forms for the requested page
//   - int: Total number of available forms
//   - error: Any error that occurred
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
// ComponentFormFieldList represents the list of form fields for components.
// It provides methods for managing and querying field configurations.
type ComponentFormFieldList []ComponentFormField

// GetField retrieves a field by its name.
// Returns the field and a boolean indicating whether it was found.
//
// GetField retrieves fields by name.
// Returns fields and indicates whether the boolean value was found.
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
// ComponentForm represents the metadata and configuration structure of the component.
// It is used by the visualization configuration tool to generate appropriate UI forms.
type ComponentForm struct {
	// Type is the unique identifier for the component type
	// Type is the unique identifier for the component type
	Type string `json:"type"`

	// Category is the classification category for organizing components
	// Category is a subcategory used to organize components
	Category string `json:"category"`

	// Fields contains the configuration fields extracted from the component's Config struct
	// Fields contain configuration fields extracted from the component's Config structure
	Fields ComponentFormFieldList `json:"fields"`

	// Label is the display name for the component (reserved for future use)
	// Label is the displayed name of the component (reserved for future use)
	Label string `json:"label"`

	// Desc is the description of the component (reserved for future use)
	// Desc is a description of a component (reserved for future use)
	Desc string `json:"desc"`

	// Icon is the icon identifier for the component (defaults to type if empty)
	// Icon is the component's icon identifier (if empty, default is type).
	Icon string `json:"icon"`

	// RelationTypes defines the possible connection names to the next node.
	// For filter nodes, defaults to: True/False/Failure
	// For other nodes, defaults to: Success/Failure
	// If nil, users can define custom relationship types
	// RelationTypes defines possible connection names to the next node.
	// For filter nodes, the default is: True/False/Failure
	// For other nodes, the default is: Success/Failure
	// If it is nil, users can define custom relationship types
	RelationTypes *[]string `json:"relationTypes"`

	// Disabled indicates whether the component should be hidden in the editor
	// Disabled indicates whether the component should be hidden in the editor
	Disabled bool `json:"disabled"`

	// Version is the version of the component
	// Version is the version of a component
	Version string `json:"version"`

	// ComponentKind indicates the type of component: dc (dynamic), nc (native), ec (endpoint)
	// ComponentKind indicates component type: dc (dynamic), nc (native), ec (endpoint)
	ComponentKind string `json:"componentKind"`

	// RouterForm contains router configuration metadata for endpoint components.
	// Only endpoint components have this field. It describes how to configure
	// the endpoint's router (from.path meaning, whether hide=true for default router, etc.).
	// The RouterForm contains routing configuration metadata for the endpoint component.
	// Only the endpoint component has this field, which describes how to configure the routing of the endpoint.
	RouterForm *RouterForm `json:"router,omitempty"`
}

// RouterForm describes the router configuration for an endpoint component.
// Compatible with the frontend endpoint.js router structure.
//
// RouterForm describes the routing configuration of the endpoint component.
type RouterForm struct {
	// Hide indicates whether the endpoint uses a default router.
	// When true, the agent should auto-generate router with from.path="*".
	// Hide indicates whether the endpoint uses the default route.
	// If true, the agent should automatically generate routes with from.path="*".
	Hide bool `json:"hide,omitempty"`

	// From provides metadata about the router source configuration.
	// From provides metadata for routing source configurations.
	From *RouterFormField `json:"from,omitempty"`

	// To provides metadata about the router target configuration.
	// To provide metadata configured for routing targets.
	To *RouterFormField `json:"to,omitempty"`

	// Params provides metadata about the router `params` field.
	// nil means the router takes no params.
	// `params` provide metadata for routing the 'params' field.
	// nil means the route does not accept params.
	Params *ComponentFormField `json:"params,omitempty"`

	// DefaultValue provides default router entries.
	// DefaultValue provides default routing entries.
	DefaultValue []map[string]interface{} `json:"defaultValue,omitempty"`
}

// RouterFormField describes the fields in a router's from/to configuration.
// RouterFormField describes the field in the routing from/to configuration.
type RouterFormField struct {
	// Path describes the path/topic/expression field metadata.
	// Path describes metadata for path/topic/expression fields.
	Path ComponentFormField `json:"path"`

	// Processors describes the processors field metadata.
	// Processors describes metadata of the processor field.
	Processors *RouterProcessorsField `json:"processors,omitempty"`
}

// RouterProcessorsField describes the processors selector configuration.
// RouterProcessorsField describes the processor selector configuration.
type RouterProcessorsField struct {
	// Hide indicates whether to hide the processors selector.
	// hide indicates whether the processor selector is hidden.
	Hide bool `json:"hide,omitempty"`
}

// ComponentFormField represents a single configuration field in a component form.
// It contains metadata about the field type, validation rules, and UI presentation.
//
// ComponentFormField represents a single configuration field in the component form.
// It contains metadata about field types, validation rules, and UI representations.
type ComponentFormField struct {
	// Name is the field name corresponding to the struct field
	// Name is the field name corresponding to the structure field
	Name string `json:"name"`

	// Type is the data type of the field (string, int, bool, etc.)
	// Type is the data type of a field (string, int, bool, etc.)
	Type string `json:"type"`

	// DefaultValue is the default value provided by the component's New() method
	// DefaultValue is the default value provided by the component's New() method
	DefaultValue interface{} `json:"defaultValue"`

	// Label is the display name for the field, extracted from the 'label' tag
	// Label is the display name of a field, extracted from the 'label' tag
	Label string `json:"label"`

	// Desc is the description of the field, extracted from the 'desc' tag
	// Desc is the description of a field, extracted from the 'desc' tag
	Desc string `json:"desc"`

	// Validate contains validation rules, extracted from the 'validate' tag
	// Deprecated: Use Rules instead
	// Validate contains validation rules extracted from the 'validate' tag
	// Deprecated: Use Rules instead
	Validate string `json:"validate"`

	// Rules contains frontend validation rules
	// Rules include frontend validation rules
	// Example: [{"required": true, "message": "This field is required"}]
	// Example: [{"required": true, "message": "This field is required"}]
	Rules []map[string]interface{} `json:"rules"`

	// Fields contains nested fields for complex objects
	// Fields contain nested fields of complex objects
	Fields ComponentFormFieldList `json:"fields"`

	// Component contains UI component configuration for rendering
	// Component contains the UI component configuration used for rendering
	// Example: {"type": "codeEditor", "language": "javascript"}
	// Example: {"type": "codeEditor", "language": "javascript"}
	Component map[string]interface{} `json:"component"`

	// Required indicates whether the field is mandatory, extracted from the 'required' tag
	// Required indicates whether the field is required and is extracted from the 'required' tab
	Required bool `json:"required"`

	// Ref indicates the field's relationship to shared node pool:
	// "primary" = the ref:// field (e.g., server, dsn)
	// "shared" = provided by shared node (hidden when ref:// is selected, shown when creating shared node)
	// "" = component-specific field (always shown in editor, hidden in shared node form)
	// Ref represents the relationship between the field and the shared node pool:
	// "primary" = ref:// field (e.g., server, dsn)
	// "shared" = provided by shared nodes (hide after selecting ref:// in the editor, show when creating shared nodes)
	// "" = Component business field (always shown in the editor, hidden when creating shared nodes)
	Ref string `json:"ref,omitempty"`
}

// SafeComponentSlice provides a thread-safe slice for storing Node components.
// It uses mutex synchronization to ensure safe concurrent access.
//
// SafeComponentSlice provides thread-safe slices for storing Node components.
// It uses mutex synchronization to ensure secure concurrent access.
type SafeComponentSlice struct {
	// components holds the list of Node components
	// components: Stores a list of Node components
	components []Node
	sync.Mutex
}

// Add safely appends one or more Node components to the slice.
// This method is thread-safe and can be called concurrently.
//
// Add securely appends one or more Node components to the slice.
// This method is thread-safe and can be called concurrently.
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
// Components returns a copy of the current component list.
// This method is thread-safe and returns snapshots of components.
func (p *SafeComponentSlice) Components() []Node {
	p.Lock()
	defer p.Unlock()
	return p.components
}
