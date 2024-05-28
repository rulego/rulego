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
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/endpoint/websocket"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
	"testing"
)

func TestRegistry(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	var endpointType = "test"
	var unknownType = "unknown"
	configuration := types.Configuration{
		"name": "lala",
	}

	err := Registry.Register(&testEndpoint{})
	assert.Nil(t, err)
	err = Registry.Register(&testEndpoint{})
	assert.Equal(t, "the component already exists. type=test", err.Error())
	_, err = Registry.New(endpointType, config, nil)
	assert.Nil(t, err)

	endpoint, err := Registry.New(endpointType, config, configuration)
	assert.Nil(t, err)

	endpoint, err = Registry.New(endpointType, config, configuration)
	assert.Nil(t, err)
	target, ok := endpoint.(*testEndpoint)
	assert.True(t, ok)
	assert.Equal(t, "lala", target.configuration["name"])

	endpoint, err = Registry.New(endpointType, config, struct {
		Name string
	}{Name: "lala"})

	target, ok = endpoint.(*testEndpoint)
	assert.True(t, ok)
	assert.Equal(t, "lala", target.configuration["Name"])

	_, err = Registry.New(unknownType, config, configuration)
	assert.Equal(t, "component not found. type="+unknownType, err.Error())
	err = Registry.Unregister(unknownType)
	assert.Equal(t, "component not found. type="+unknownType, err.Error())
	err = Registry.Unregister(endpointType)
	assert.Nil(t, err)
	_, err = Registry.New(endpointType, config, configuration)
	assert.Equal(t, "component not found. type="+endpointType, err.Error())
}

func TestNewFromRegistry(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	configuration := types.Configuration{
		"server": ":9090",
	}
	endpoint, err := Registry.New(rest.Type, config, configuration)
	assert.Nil(t, err)
	_, ok := endpoint.(*rest.Endpoint)
	assert.True(t, ok)

	endpoint, err = Registry.New(websocket.Type, config, configuration)
	assert.Nil(t, err)
	_, ok = endpoint.(*websocket.Endpoint)
	assert.True(t, ok)
}

// 测试endpoint
type testEndpoint struct {
	impl.BaseEndpoint
	configuration types.Configuration
}

// Type 组件类型
func (test *testEndpoint) Type() string {
	return "test"
}

func (test *testEndpoint) New() types.Node {
	return &testEndpoint{}
}

// Init 初始化
func (test *testEndpoint) Init(ruleConfig types.Config, configuration types.Configuration) error {
	test.configuration = configuration
	return nil
}

// Destroy 销毁
func (test *testEndpoint) Destroy() {
	_ = test.Close()
}

func (test *testEndpoint) Close() error {
	return nil
}

func (test *testEndpoint) Id() string {
	return "id"
}

func (test *testEndpoint) AddRouter(router endpointApi.Router, params ...interface{}) (string, error) {
	//返回任务ID，用于清除任务
	return "1", nil
}

func (test *testEndpoint) RemoveRouter(routeId string, params ...interface{}) error {
	return nil
}

func (test *testEndpoint) Start() error {
	return nil
}
