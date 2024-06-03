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
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/components/external"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	config := engine.NewConfig(types.WithDefaultPool())
	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		//assert.Equal(t, "ok", msg.Data)
	})
	msg1 := ctx.NewMsg("TEST_MSG_TYPE_AA", types.NewMetadata(), "{\"name\":\"lala\"}")

	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")
	_, err = engine.New("test01", ruleDsl)
	if err != nil {
		t.Fatal(err)
	}

	pool := NewPool()
	defer pool.Stop()
	assert.True(t, reflect.DeepEqual(pool.factory, pool.Factory()))

	ep, err := New("e_http", []byte(""), endpoint.DynamicEndpointOptions.WithConfig(config))
	assert.NotNil(t, err)

	endpointBuf, err := os.ReadFile(testEndpointsFolder + "/http_01.json")
	if err != nil {
		t.Fatal(err)
	}
	ep, err = New("e_http", endpointBuf, endpoint.DynamicEndpointOptions.WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}
	err = ep.Start()
	time.Sleep(time.Millisecond * 200)

	var def types.EndpointDsl
	_ = json.Unmarshal(endpointBuf, &def)
	v, _ := json.Marshal(def)
	dsl := strings.Replace(string(v), " ", "", -1)

	assert.Equal(t, dsl, strings.Replace(string(ep.DSL()), " ", "", -1))
	assert.True(t, reflect.DeepEqual(def, ep.Definition()))
	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))

	endpointBuf, err = os.ReadFile(testEndpointsFolder + "/http_02.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = ep.Reload(endpointBuf)
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Failure)
	}))
	sendMsg(t, "http://127.0.0.1:9090/api/v2/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))

	_ = ep.Reload(endpointBuf, endpoint.DynamicEndpointOptions.WithRestart(true))
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "http://127.0.0.1:9090/api/v1/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Failure)
	}))
	sendMsg(t, "http://127.0.0.1:9090/api/v2/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))

	router1 := `
  {
      "id":"r1",
      "params": [
        "post"
      ],
      "from": {
        "path": "/api/v3/test/:chainId",
        "configuration": {
        }
      },
      "to": {
        "path": "${chainId}"
      }
    }`
	_ = ep.AddOrReloadRouter([]byte(router1))
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "http://127.0.0.1:9090/api/v2/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Failure)
	}))
	sendMsg(t, "http://127.0.0.1:9090/api/v3/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))

	endpointBuf, err = os.ReadFile(testEndpointsFolder + "/http_02.json")
	if err != nil {
		t.Fatal(err)
	}
	newDsl := strings.Replace(string(endpointBuf), ":9090", ":9091", -1)
	ep, ok := Get("e_http")
	assert.True(t, ok)
	_ = ep.Reload([]byte(newDsl))
	time.Sleep(time.Millisecond * 200)

	sendMsg(t, "http://127.0.0.1:9090/api/v2/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Failure)
	}))
	sendMsg(t, "http://127.0.0.1:9091/api/v2/test/test01", "POST", msg1, test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err2 error) {
		assert.Equal(t, relationType, types.Success)
	}))
	time.Sleep(time.Millisecond * 200)

	Reload()

	Range(func(key, value any) bool {
		assert.Equal(t, "e_http", key)
		return true
	})

	Del("e_http")
	ep, ok = Get("e_http")
	assert.False(t, ok)

	var count = 0
	Range(func(key, value any) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)
	Stop()
}

func TestFactory(t *testing.T) {
	endpointBuf, err := os.ReadFile(testEndpointsFolder + "/http_01.json")
	if err != nil {
		t.Fatal(err)
	}
	ruleDsl, err := os.ReadFile(testRulesFolder + "/filter_node.json")

	_, err = engine.New("test01", ruleDsl)

	assert.True(t, reflect.DeepEqual(DefaultPool.factory, DefaultPool.Factory()))

	ep1, err := DefaultPool.factory.NewFromDsl(endpointBuf)
	assert.Nil(t, err)

	def := ep1.Definition()
	assert.Equal(t, "e1", def.Id)

	ep1.Destroy()

	ep2, err := DefaultPool.factory.NewFromDef(def)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(def, ep2.Definition()))

	config := engine.NewConfig(types.WithDefaultPool())
	ep3, err := DefaultPool.factory.NewFromType("http", config, struct {
		Name string
	}{Name: "lala"})

	assert.Nil(t, err)

	assert.Equal(t, "http", ep3.Type())
}

// SendMsg 发送消息到rest服务器
func sendMsg(t *testing.T, url, method string, msg types.RuleMsg, ctx types.RuleContext) types.Node {
	node := &external.RestApiCallNode{}
	var configuration = make(types.Configuration)
	configuration["restEndpointUrlPattern"] = url
	configuration["requestMethod"] = method
	config := types.NewConfig()
	err := node.Init(config, configuration)
	if err != nil {
		t.Fatal(err)
	}
	//发送消息
	node.OnMsg(ctx, msg)
	return node
}
