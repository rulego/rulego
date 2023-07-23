package rulego

import (
	"errors"
	"fmt"
	"plugin"
	"rulego/api/types"
	"rulego/components/action"
	"rulego/components/filter"
	"rulego/components/transform"
	"sync"
)

//PluginsSymbol 插件检查点 Symbol
const PluginsSymbol = "Plugins"

//Registry 规则引擎组件默认注册器
var Registry = new(RuleComponentRegistry)

//注册默认组件
func init() {
	var components []types.Node
	components = append(components, action.Registry.Components()...)
	components = append(components, filter.Registry.Components()...)
	components = append(components, transform.Registry.Components()...)

	//把组件注册到默认组件库
	for _, node := range components {
		_ = Registry.Register(node)
	}
}

//RuleComponentRegistry 组件注册器
type RuleComponentRegistry struct {
	//规则引擎节点组件列表
	components map[string]types.Node
	//插件列表
	plugins map[string][]types.Node
	sync.RWMutex
}

//Register 注册规则引擎节点组件
func (r *RuleComponentRegistry) Register(node types.Node) error {
	r.Lock()
	defer r.Unlock()
	if r.components == nil {
		r.components = make(map[string]types.Node)
	}
	if _, ok := r.components[node.Type()]; ok {
		return errors.New("the component already exists. nodeType=" + node.Type())
	}
	r.components[node.Type()] = node

	return nil
}

//RegisterPlugin 注册规则引擎节点组件
func (r *RuleComponentRegistry) RegisterPlugin(name string, file string) error {
	builder := &PluginComponentRegistry{name: name, file: file}
	if err := builder.Init(); err != nil {
		return err
	}
	components := builder.Components()
	for _, node := range components {
		if _, ok := r.components[node.Type()]; ok {
			return errors.New("the component already exists. nodeType=" + node.Type())
		}
	}
	for _, node := range components {
		if err := r.Register(node); err != nil {
			return err
		}
	}

	r.Lock()
	defer r.Unlock()
	if r.plugins == nil {
		r.plugins = make(map[string][]types.Node)
	}
	r.plugins[name] = components
	return nil
}

func (r *RuleComponentRegistry) Unregister(componentType string) error {
	r.RLock()
	defer r.RUnlock()
	var removed = false
	// Check if the plugin exists
	if nodes, ok := r.plugins[componentType]; ok {
		for _, node := range nodes {
			// Delete the plugin from the map
			delete(r.components, node.Type())
		}
		delete(r.plugins, componentType)
		removed = true
	}

	// Check if the plugin exists
	if _, ok := r.components[componentType]; ok {
		// Delete the plugin from the map
		delete(r.components, componentType)
		removed = true
	}

	if !removed {
		return fmt.Errorf("component not found.componentType=%s", componentType)
	} else {
		return nil
	}
}

//NewNode 获取规则引擎节点组件
func (r *RuleComponentRegistry) NewNode(nodeType string) (types.Node, error) {
	r.RLock()
	defer r.RUnlock()

	if node, ok := r.components[nodeType]; !ok {
		return nil, fmt.Errorf("component not found.componentType=%s", nodeType)
	} else {
		return node.New(), nil
	}
}

func (r *RuleComponentRegistry) GetComponents() map[string]types.Node {
	r.RLock()
	defer r.RUnlock()
	var components = map[string]types.Node{}
	for k, v := range r.components {
		components[k] = v
	}
	return components
}

//PluginComponentRegistry go plugin组件初始化器
type PluginComponentRegistry struct {
	name     string
	file     string
	registry types.PluginRegistry
}

func (p *PluginComponentRegistry) Init() error {
	pluginRegistry, err := loadPlugin(p.file)
	if err != nil {
		return err
	} else {
		p.registry = pluginRegistry
		return nil
	}
}

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
