/*
 * Copyright 2025 The RuleGo Authors.
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
)

// CustomComponentRegistry combines default and custom component registries
type CustomComponentRegistry struct {
	defaultComponents types.ComponentRegistry
	customComponents  types.ComponentRegistry
}

// NewCustomComponentRegistry creates a composite registry that checks
// custom components first, then falls back to default components.
// Parameters:
// - defaultComponents: base registry with pre-defined components
// - customComponents: registry for user-defined components
// Returns a ComponentRegistry that combines both sources
func NewCustomComponentRegistry(defaultComponents, customComponents types.ComponentRegistry) types.ComponentRegistry {
	registry := &CustomComponentRegistry{
		defaultComponents: defaultComponents,
		customComponents:  customComponents,
	}
	return registry
}

// Register adds a custom component to the registry
// Returns error if component type already exists
func (r *CustomComponentRegistry) Register(node types.Node) error {
	return r.customComponents.Register(node)
}

// RegisterPlugin loads a plugin containing components
// name: plugin name for dependency tracking
// file: plugin file path (dynamic library)
func (r *CustomComponentRegistry) RegisterPlugin(name string, file string) error {
	return r.customComponents.RegisterPlugin(name, file)
}

// Unregister removes a component from the custom registry
// Returns error if component not found
func (r *CustomComponentRegistry) Unregister(componentType string) error {
	return r.customComponents.Unregister(componentType)
}

// NewNode creates a component instance with fallback logic:
// 1. First tries to create from default components
// 2. If fails, attempts to create from custom components
// Returns:
// - component instance if found in either registry
// - error if not found in both registries
func (r *CustomComponentRegistry) NewNode(componentType string) (types.Node, error) {
	node, err := r.defaultComponents.NewNode(componentType)
	if err == nil {
		return node, err
	}
	return r.customComponents.NewNode(componentType)
}

// GetComponents returns merged view of all components:
// Default components are overridden by custom components with same type
func (r *CustomComponentRegistry) GetComponents() map[string]types.Node {
	components := make(map[string]types.Node)
	defaultComponentList := r.defaultComponents.GetComponents()
	for k, v := range defaultComponentList {
		components[k] = v
	}
	customComponentList := r.customComponents.GetComponents()
	for k, v := range customComponentList {
		components[k] = v
	}
	return components
}

// GetComponentForms returns combined component metadata
// Includes forms from both default and custom components
func (r *CustomComponentRegistry) GetComponentForms() types.ComponentFormList {
	components := make(map[string]types.ComponentForm)
	defaultComponentList := r.defaultComponents.GetComponentForms()
	for k, v := range defaultComponentList {
		components[k] = v
	}
	customComponentList := r.customComponents.GetComponentForms()
	for k, v := range customComponentList {
		components[k] = v
	}
	return components
}

func (r *CustomComponentRegistry) DefaultComponents() types.ComponentRegistry {
	return r.defaultComponents
}

func (r *CustomComponentRegistry) CustomComponents() types.ComponentRegistry {
	return r.customComponents
}
