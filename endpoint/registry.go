/*
 * Copyright 2024 The RuleGo Authors.
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

package endpoint

import (
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/mqtt"
	"github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/endpoint/schedule"
	"github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/utils/maps"
	"sync"
)

// init registers the available endpoint components with the Registry.
func init() {
	_ = Registry.Register(&mqtt.Endpoint{})
	_ = Registry.Register(&rest.Endpoint{})
	_ = Registry.Register(&net.Endpoint{})
	_ = Registry.Register(&websocket.Endpoint{})
	_ = Registry.Register(&schedule.Endpoint{})
}

// Registry is the default registry for endpoint components.
var Registry = new(ComponentRegistry)

// ComponentRegistry is a registry for endpoint components.
type ComponentRegistry struct {
	// components holds the registered endpoint components.
	components map[string]endpoint.Endpoint
	sync.RWMutex
}

// Register adds a new endpoint component to the registry.
func (r *ComponentRegistry) Register(component endpoint.Endpoint) error {
	r.Lock()
	defer r.Unlock()
	if r.components == nil {
		r.components = make(map[string]endpoint.Endpoint)
	}
	if _, ok := r.components[component.Type()]; ok {
		return errors.New("the component already exists. type=" + component.Type())
	}
	r.components[component.Type()] = component

	return nil
}

// Unregister removes an endpoint component from the registry.
func (r *ComponentRegistry) Unregister(componentType string) error {
	r.RLock()
	defer r.RUnlock()
	if _, ok := r.components[componentType]; ok {
		delete(r.components, componentType)
		return nil
	} else {
		return fmt.Errorf("component not found. type=%s", componentType)
	}
}

// New creates a new instance of an endpoint based on the component type.
// The configuration parameter can be either types.Configuration or the corresponding Config type for the endpoint.
func (r *ComponentRegistry) New(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	r.RLock()
	defer r.RUnlock()

	if node, ok := r.components[componentType]; !ok {
		return nil, fmt.Errorf("component not found. type=%s", componentType)
	} else {
		var err error
		var config = make(types.Configuration)
		if configuration != nil {
			if c, ok := configuration.(types.Configuration); ok {
				config = c
			} else if err = maps.Map2Struct(configuration, config); err != nil {
				return nil, err
			}
		}

		//创建新的实例
		newNode := node.New()

		if endpoint, ok := newNode.(endpoint.Endpoint); ok {
			if err = endpoint.Init(ruleConfig, config); err != nil {
				return nil, err
			} else {
				return endpoint, nil
			}
		} else {
			return nil, fmt.Errorf("%s not type of Endpoint", componentType)
		}
	}
}
