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

// 注册组件
func init() {
	_ = Registry.Register(&mqtt.Endpoint{})
	_ = Registry.Register(&rest.Endpoint{})
	_ = Registry.Register(&net.Endpoint{})
	_ = Registry.Register(&websocket.Endpoint{})
	_ = Registry.Register(&schedule.Endpoint{})
}

// Registry endpoint组件默认注册器
var Registry = new(ComponentRegistry)

// ComponentRegistry 组件注册器
type ComponentRegistry struct {
	//endpoint组件
	components map[string]endpoint.Endpoint
	sync.RWMutex
}

// Register 注册规则引擎节点组件
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

// New 创建一个新的endpoint实例
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

// New 创建指定类型的endpoint实例
// componentType endpoint类型
// ruleConfig rulego配置
// configuration endpoint配置参数，可以是types.Configuration和endpoint对应Config的类型
func New(componentType string, ruleConfig types.Config, configuration interface{}) (endpoint.Endpoint, error) {
	return Registry.New(componentType, ruleConfig, configuration)
}
