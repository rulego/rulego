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

package node_pool

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/mqtt"
	"strings"
	"testing"
	"time"
)

func TestLoadFromRuleChain(t *testing.T) {
	var dsl = []byte(`
{
  "ruleChain": {
    "id": "test_node_pool",
    "name": "测试通过规则链初始化共享组件",
    "debugMode": true,
    "root": true,
    "additionalInfo": {
    }
  },
  "metadata": {
    "endpoints": [
      {
        "id": "node_2",
        "type": "endpoint/http",
        "name": "ddd",
        "configuration": {
          "server": ":6334"
        }
      }
    ],
    "nodes": [
		{
	       "id": "my_mqtt_client01",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }
    ],
    "connections": []
  }
}
`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	ctx, err := pool.Load(dsl)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)
	assert.Nil(t, err)
	//assert.True(t, ctx.(*sharedNodeCtx)
	_, ok := pool.Get("my_mqtt_client01")
	assert.True(t, ok)
	_, ok = pool.Get("node_2")
	assert.True(t, ok)

	assert.Equal(t, 2, len(pool.GetAll()))

	client, err := pool.GetInstance("my_mqtt_client01")
	_, ok = client.(*mqtt.Client)
	assert.True(t, ok)
	assert.NotNil(t, client)
	assert.Nil(t, err)

	client, err = pool.GetInstance("node_2")
	assert.NotNil(t, client)
	assert.Nil(t, err)
	_, ok = client.(*rest.Rest)
	assert.True(t, ok)

	client, err = pool.GetInstance("my_mqtt_client02")
	assert.Nil(t, client)
	assert.NotNil(t, err)

}
func TestEndpointPool(t *testing.T) {
	var dsl1 = []byte(`
		{
	       "id": "endpoint_my_mqtt_client01",
	       "type": "endpoint/mqtt",
	       "name": "mqtt客户端",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	var dsl2 = []byte(`
		{
	       "id": "endpoint_my_mqtt_client02",
	       "type": "endpoint/mqtt",
	       "name": "mqtt客户端",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	var def types.EndpointDsl
	_ = json.Unmarshal(dsl1, &def)
	ctx, err := pool.NewFromEndpoint(def)

	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	_ = json.Unmarshal(dsl2, &def)
	ctx, err = pool.NewFromEndpoint(def)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	_, ok := pool.Get("endpoint_my_mqtt_client01")
	assert.True(t, ok)
	_, ok = pool.Get("endpoint_my_mqtt_client02")
	assert.True(t, ok)

	assert.Equal(t, 2, len(pool.GetAll()))
	pool.Del("endpoint_my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()))

	pool.Del("endpoint_my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()))

	client, err := pool.GetInstance("endpoint_my_mqtt_client01")
	assert.NotNil(t, client)
	assert.Nil(t, err)

	client, err = pool.GetInstance("endpoint_my_mqtt_client02")
	assert.Nil(t, client)
	assert.NotNil(t, err)

	items := pool.GetAll()
	assert.Equal(t, 1, len(items))

	var notNetNodeDsl = []byte(`
		{
	       "id": "my_jsFilter",
	       "type": "jsFilter",
	       "name": "过滤器",
	       "debugMode": false,
	       "configuration": {
	       }
	     }`)
	_ = json.Unmarshal(notNetNodeDsl, &def)
	ctx, err = pool.NewFromEndpoint(def)

	assert.NotNil(t, err)
	assert.Equal(t, 1, len(pool.GetAll()))
}

func TestRuleNodePool(t *testing.T) {
	var dsl1 = []byte(`
		{
	       "id": "my_mqtt_client01",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	var dsl2 = []byte(`
		{
	       "id": "my_mqtt_client02",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	nodeDef, err := config.Parser.DecodeRuleNode(dsl1)
	ctx, err := pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	nodeDef, err = config.Parser.DecodeRuleNode(dsl2)
	ctx, err = pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)
	//assert.True(t, ctx.(*sharedNodeCtx)
	_, ok := pool.Get("my_mqtt_client01")
	assert.True(t, ok)
	_, ok = pool.Get("my_mqtt_client02")
	assert.True(t, ok)

	assert.Equal(t, 2, len(pool.GetAll()))
	pool.Del("my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()))

	pool.Del("my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()))

	client, err := pool.GetInstance("my_mqtt_client01")
	assert.NotNil(t, client)
	assert.Nil(t, err)

	client, err = pool.GetInstance("my_mqtt_client02")
	assert.Nil(t, client)
	assert.NotNil(t, err)

	items := pool.GetAll()
	assert.Equal(t, 1, len(items))

	var notNetNodeDsl = []byte(`
		{
	       "id": "my_jsFilter",
	       "type": "jsFilter",
	       "name": "过滤器",
	       "debugMode": false,
	       "configuration": {
	       }
	     }`)
	nodeDef, err = config.Parser.DecodeRuleNode(notNetNodeDsl)
	ctx, err = pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, err)
	assert.Equal(t, 1, len(pool.GetAll()))
	length, err := pool.GetAllDef()
	assert.True(t, len(length) > 0)
}

func TestEngineFromNetPool(t *testing.T) {
	var dsl1 = []byte(`
		{
	       "id": "my_mqtt_client01",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	nodeDef, err := config.Parser.DecodeRuleNode(dsl1)
	ctx, err := pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	ruleChainFile := `
		{
		"ruleChain": {
		  "id": "netSourcePoolRule01",
		  "name": "netSourcePoolRule01"
		  },
		"metadata": {
		  "nodes": [
			{
			  "id": "mqttClient",
			  "type": "mqttClient",
			  "name": "mqtt推送数据",
			  "debugMode": false,
			  "configuration": {
				"server": "ref://my_mqtt_client01"
				}
			}
         ]
		}
	}
`
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, "{\"temperature\":35}")
	//通过连接池启动规则引擎
	ruleEngine1, err := engine.New("netSourcePoolRule01", []byte(ruleChainFile), engine.WithConfig(config))
	ruleEngine2, err := engine.New("netSourcePoolRule02", []byte(ruleChainFile), engine.WithConfig(config))

	ruleEngine1.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	ruleEngine1.Stop()
	time.Sleep(time.Millisecond * 500)

	//ruleEngine1停止，不相影响ruleEngine2
	ruleEngine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	ruleEngine2.Stop()
	time.Sleep(time.Millisecond * 500)

	ruleEngine3, err := engine.New("netSourcePoolRule03", []byte(ruleChainFile), engine.WithConfig(config))
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	netResourceCtx, _ := pool.Get("my_mqtt_client01")
	//错误的连接池
	dsl1 = []byte(strings.Replace(string(dsl1), `127.0.0.1:1883`, `127.0.0.1:1884`, -1))
	err = netResourceCtx.ReloadSelf(dsl1)
	assert.Nil(t, err)
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	//修改正常的连接池
	dsl1 = []byte(strings.Replace(string(dsl1), `127.0.0.1:1884`, `127.0.0.1:1883`, -1))
	err = netResourceCtx.ReloadSelf(dsl1)
	assert.Nil(t, err)
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	//注销池
	pool.Stop()
	assert.Equal(t, 0, len(pool.GetAll()))
	//连接池已经删除，无法发送数据
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 500)
}
