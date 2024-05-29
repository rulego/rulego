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
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/mqtt"
	"github.com/rulego/rulego/endpoint/net"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/endpoint/schedule"
	"github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/maps"
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
	engine.RuleComponentRegistry
}

// Register adds a new endpoint component to the registry.
func (r *ComponentRegistry) Register(component endpoint.Endpoint) error {
	return r.RuleComponentRegistry.Register(component)
}

// New creates a new instance of an endpoint based on the component type.
// The configuration parameter can be either types.Configuration or the corresponding Config type for the endpoint.
func (r *ComponentRegistry) New(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	newNode, err := r.RuleComponentRegistry.NewNode(componentType)
	if err != nil {
		return nil, err
	}

	var config = make(types.Configuration)
	if configuration != nil {
		if c, ok := configuration.(types.Configuration); ok {
			config = c
		} else if err = maps.Map2Struct(configuration, config); err != nil {
			return nil, err
		}
	}

	if ep, ok := newNode.(endpoint.Endpoint); ok {
		if err = ep.Init(ruleConfig, config); err != nil {
			return nil, err
		} else {
			return ep, nil
		}
	} else {
		return nil, fmt.Errorf("%s not type of Endpoint", componentType)
	}
}
