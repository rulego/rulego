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

package endpoint

import (
	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"net/textproto"
	"testing"
)

func TestRegistry(t *testing.T) {
	config := rulego.NewConfig(types.WithDefaultPool())
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

	endpoint, err := New(endpointType, config, configuration)
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

// testRequestMessage http请求消息
type testRequestMessage struct {
	headers textproto.MIMEHeader
	body    []byte
	msg     *types.RuleMsg
	err     error
}

func (r *testRequestMessage) Body() []byte {
	return r.body
}
func (r *testRequestMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *testRequestMessage) From() string {
	return ""
}

func (r *testRequestMessage) GetParam(key string) string {
	return ""
}

func (r *testRequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *testRequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))
		r.msg = &ruleMsg
	}
	return r.msg
}

func (r *testRequestMessage) SetStatusCode(statusCode int) {
}

func (r *testRequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *testRequestMessage) SetError(err error) {
	r.err = err
}

func (r *testRequestMessage) GetError() error {
	return r.err
}

// testResponseMessage 响应消息
type testResponseMessage struct {
	body    []byte
	msg     *types.RuleMsg
	headers textproto.MIMEHeader
	err     error
}

func (r *testResponseMessage) Body() []byte {
	return r.body
}

func (r *testResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *testResponseMessage) From() string {
	return ""
}

func (r *testResponseMessage) GetParam(key string) string {
	return ""
}

func (r *testResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *testResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *testResponseMessage) SetStatusCode(statusCode int) {
}

func (r *testResponseMessage) SetBody(body []byte) {
	r.body = body

}

func (r *testResponseMessage) SetError(err error) {
	r.err = err
}

func (r *testResponseMessage) GetError() error {
	return r.err
}

// 测试endpoint
type testEndpoint struct {
	BaseEndpoint
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

func (test *testEndpoint) AddRouter(router *Router, params ...interface{}) (string, error) {
	//返回任务ID，用于清除任务
	return "1", nil
}

func (test *testEndpoint) RemoveRouter(routeId string, params ...interface{}) error {
	return nil
}

func (test *testEndpoint) Start() error {
	return nil
}
