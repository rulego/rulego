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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"strings"
	"testing"
	"time"
)

func TestNetPool(t *testing.T) {

	var dsl = []byte(`
		{
	       "id": "my_mqtt_client",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	config := NewConfig()
	pool := NewNetPool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	ctx, err := pool.New("mqttClient", "my_mqtt_client01", dsl)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	ctx, err = pool.New("mqttClient", "my_mqtt_client02", dsl)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)
	assert.True(t, ctx.(*NetResourceCtx).isInitNetResource)
	_, ok := pool.Get("mqttClient", "my_mqtt_client01")
	assert.True(t, ok)
	_, ok = pool.Get("mqttClient", "my_mqtt_client02")
	assert.True(t, ok)

	assert.Equal(t, 2, len(pool.GetAll()["mqttClient"]))
	pool.Del("mqttClient", "my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()["mqttClient"]))

	pool.Del("net", "my_mqtt_client02")
	assert.Equal(t, 1, len(pool.GetAll()["mqttClient"]))

	client, err := pool.GetNetResource("mqttClient", "my_mqtt_client01")
	assert.NotNil(t, client)
	assert.Nil(t, err)

	client, err = pool.GetNetResource("mqttClient", "my_mqtt_client02")
	assert.Nil(t, client)
	assert.NotNil(t, err)

	items := pool.GetAll()
	assert.Equal(t, 1, len(items["mqttClient"]))

	if nodeDef, err := config.Parser.DecodeRuleNode(dsl); err == nil {
		_, _ = pool.NewFromDef(nodeDef)
	}
	assert.Equal(t, 2, len(pool.GetAll()["mqttClient"]))

	var notNetNodeDsl = []byte(`
		{
	       "id": "my_jsFilter",
	       "type": "jsFilter",
	       "name": "过滤器",
	       "debugMode": false,
	       "configuration": {
	       }
	     }`)
	ctx, err = pool.New("jsFilter", "my_jsFilter01", notNetNodeDsl)
	assert.NotNil(t, err)
	assert.Equal(t, 2, len(pool.GetAll()))

}

func TestEngineFromNetPool(t *testing.T) {
	var dsl = []byte(`
		{
	       "id": "my_mqtt_client",
	       "type": "mqttClient",
	       "name": "mqtt推送数据",
	       "debugMode": false,
	       "configuration": {
	         "Server": "127.0.0.1:1883",
	         "Topic": "/device/msg"
	       }
	     }`)

	config := NewConfig()
	pool := NewNetPool(config)
	config.NetPool = pool
	assert.Equal(t, 0, len(pool.GetAll()))
	ctx, err := pool.New("mqttClient", "my_mqtt_client01", dsl)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	var notNetNodeDsl = []byte(`
		{
	       "id": "my_jsFilter",
	       "type": "jsFilter",
	       "name": "过滤器",
	       "debugMode": false,
	       "configuration": {
	       }
	     }`)
	ctx, err = pool.New("jsFilter", "my_jsFilter01", notNetNodeDsl)
	assert.NotNil(t, err)
	assert.Equal(t, 2, len(pool.GetAll()))

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
	ruleEngine1, err := New("netSourcePoolRule01", []byte(ruleChainFile), WithConfig(config))
	ruleEngine2, err := New("netSourcePoolRule02", []byte(ruleChainFile), WithConfig(config))

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

	ruleEngine3, err := New("netSourcePoolRule03", []byte(ruleChainFile), WithConfig(config))
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	netResourceCtx, _ := pool.Get("mqttClient", "my_mqtt_client01")

	err = netResourceCtx.ReloadSelf([]byte(strings.Replace(string(dsl), `127.0.0.1:1883`, `127.0.0.1:1884`, -1)))
	assert.Nil(t, err)
	//错误的连接池
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	//修改正常的连接池
	err = netResourceCtx.ReloadSelf([]byte(strings.Replace(string(dsl), `"127.0.0.1:1884`, `127.0.0.1:1883`, -1)))
	assert.Nil(t, err)
	//错误的连接池
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
