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
	"errors"
	"fmt"
	"plugin"
	"sync"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/components/common"
	"github.com/rulego/rulego/components/external"
	"github.com/rulego/rulego/components/filter"
	"github.com/rulego/rulego/components/flow"
	"github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/utils/reflect"
)

// PluginsSymbol is the symbol used to identify plugins in a Go plugin file.
const PluginsSymbol = "Plugins"

// Registry is the default registry for rule engine components.
var Registry = new(RuleComponentRegistry)

// init registers default components to the default component registry.
func init() {
	var components []types.Node
	// Append components from various packages to the components slice.
	components = append(components, action.Registry.Components()...)
	components = append(components, filter.Registry.Components()...)
	components = append(components, transform.Registry.Components()...)
	components = append(components, external.Registry.Components()...)
	components = append(components, flow.Registry.Components()...)
	components = append(components, common.Registry.Components()...)

	// Register all components to the default component registry.
	for _, node := range components {
		_ = Registry.Register(node)
	}
}

// RuleComponentRegistry is a registry for rule engine components.
type RuleComponentRegistry struct {
	// components is a map of rule engine node components.
	// Keys can be primary type names or aliases, both pointing to the same Node instance.
	components map[string]types.Node
	// aliases is a reverse mapping from alias to primary type name.
	// Only alias entries exist in this map.
	aliases map[string]string
	// plugins is a map of plugin components.
	plugins map[string][]types.Node
	// endpointComponents is a map of endpoint components.
	endpointComponents map[string]types.Node
	// RWMutex is a read/write mutex lock.
	sync.RWMutex
}

// Register adds a rule engine node component to the registry.
func (r *RuleComponentRegistry) Register(node types.Node) error {
	r.Lock()
	defer r.Unlock()
	if r.components == nil {
		r.components = make(map[string]types.Node)
	}
	if _, ok := r.components[node.Type()]; ok {
		return errors.New("the component already exists. componentType=" + node.Type())
	}
	r.components[node.Type()] = node

	return nil
}

// RegisterAlias registers one or more aliases for an existing component.
// This allows components to be referenced by alternative names, useful for
// backward compatibility or providing shorter/more intuitive names.
//
// Parameters:
//   - primaryType: The registered component type name
//   - aliases: One or more alternative names for the component
//
// Returns error if the primary component is not found or any alias conflicts with existing components.
func (r *RuleComponentRegistry) RegisterAlias(primaryType string, aliases ...string) error {
	r.Lock()
	defer r.Unlock()

	if r.components == nil {
		return fmt.Errorf("component not found: %s", primaryType)
	}

	node, ok := r.components[primaryType]
	if !ok {
		return fmt.Errorf("component not found: %s", primaryType)
	}

	if r.aliases == nil {
		r.aliases = make(map[string]string)
	}

	for _, alias := range aliases {
		// Allow alias to be the same as primaryType (no-op)
		if alias == primaryType {
			continue
		}
		if _, exists := r.components[alias]; exists {
			return fmt.Errorf("alias conflicts with existing component: %s", alias)
		}
		r.components[alias] = node
		r.aliases[alias] = primaryType
	}

	return nil
}

// RegisterPlugin adds a rule engine node component from a plugin file.
func (r *RuleComponentRegistry) RegisterPlugin(name string, file string) error {
	builder := &PluginComponentRegistry{name: name, file: file}
	if err := builder.Init(); err != nil {
		return err
	}
	components := builder.Components()

	r.Lock()
	defer r.Unlock()

	// Check for existing components
	for _, node := range components {
		if _, ok := r.components[node.Type()]; ok {
			return errors.New("the component already exists. componentType=" + node.Type())
		}
	}

	// Initialize maps if needed
	if r.components == nil {
		r.components = make(map[string]types.Node)
	}
	if r.plugins == nil {
		r.plugins = make(map[string][]types.Node)
	}

	// Register all components
	for _, node := range components {
		r.components[node.Type()] = node
	}
	r.plugins[name] = components
	return nil
}

// Unregister removes a component from the registry by its type or plugin name.
// When removing a primary component, all its aliases are also removed.
// When removing an alias, only that alias is removed, the primary and other aliases remain.
func (r *RuleComponentRegistry) Unregister(componentType string) error {
	r.Lock()
	defer r.Unlock()
	var removed = false

	// 1. If removing an alias, only remove that alias entry
	if r.aliases != nil {
		if _, ok := r.aliases[componentType]; ok {
			delete(r.components, componentType)
			delete(r.aliases, componentType)
			return nil
		}
	}

	// 2. If removing a primary type, also remove all its aliases
	if r.aliases != nil {
		for alias, primary := range r.aliases {
			if primary == componentType {
				delete(r.components, alias)
				delete(r.aliases, alias)
			}
		}
	}

	// 3. Check if it's a plugin name
	if nodes, ok := r.plugins[componentType]; ok {
		for _, node := range nodes {
			// Delete all components of this plugin
			delete(r.components, node.Type())
		}
		delete(r.plugins, componentType)
		removed = true
	}

	// 4. Check if it's a component type
	if _, ok := r.components[componentType]; ok {
		// Delete the component
		delete(r.components, componentType)
		removed = true
	}

	if !removed {
		return fmt.Errorf("component not found. componentType=%s", componentType)
	} else {
		return nil
	}
}

// NewNode creates a new instance of a rule engine node component by its type.
func (r *RuleComponentRegistry) NewNode(componentType string) (types.Node, error) {
	r.RLock()
	defer r.RUnlock()

	if node, ok := r.components[componentType]; !ok {
		return nil, fmt.Errorf("component not found. componentType=%s", componentType)
	} else {
		return node.New(), nil
	}
}

// GetComponents returns a map of all registered components.
func (r *RuleComponentRegistry) GetComponents() map[string]types.Node {
	r.RLock()
	defer r.RUnlock()
	var components = map[string]types.Node{}
	for k, v := range r.components {
		components[k] = v
	}
	return components
}

// GetComponentForms returns a list of component forms for all registered components.
func (r *RuleComponentRegistry) GetComponentForms() types.ComponentFormList {
	r.RLock()
	defer r.RUnlock()

	var components = make(types.ComponentFormList)
	for _, component := range r.components {
		components[component.Type()] = reflect.GetComponentForm(component.New())
	}
	return components
}

// PluginComponentRegistry is an initializer for Go plugin components.
type PluginComponentRegistry struct {
	name     string
	file     string
	registry types.PluginRegistry
}

// Init initializes the plugin component registry by loading the plugin from a file.
func (p *PluginComponentRegistry) Init() error {
	pluginRegistry, err := loadPlugin(p.file)
	if err != nil {
		return err
	} else {
		p.registry = pluginRegistry
		return nil
	}
}

// Components returns a slice of components provided by the plugin.
func (p *PluginComponentRegistry) Components() []types.Node {
	if p.registry != nil {
		return p.registry.Components()
	}
	return nil
}

// loadPlugin loads a plugin from a file and registers it with a given name
func loadPlugin(file string) (types.PluginRegistry, error) {
	// Use the plugin package to open the file and look up the exported symbol "Plugin"
	p, err := plugin.Open(file)
	if err != nil {
		return nil, err
	}
	sym, err := p.Lookup(PluginsSymbol)
	if err != nil {
		return nil, err
	}
	// Use type assertion to check if the symbol is a Plugin interface implementation
	plugin, ok := sym.(types.PluginRegistry)
	if !ok {
		return nil, errors.New("invalid plugin")
	}
	// Register the plugin with the name
	//pm.plugins[name] = plugin
	return plugin, nil
}
