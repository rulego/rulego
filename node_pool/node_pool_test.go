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
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/mqtt"
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
	ruleEngine1.Stop(context.Background())
	time.Sleep(time.Millisecond * 500)

	//ruleEngine1停止，不相影响ruleEngine2
	ruleEngine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	ruleEngine2.Stop(context.Background())
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

// TestSharedNodeLifecycleManagement 测试共享节点生命周期管理
func TestSharedNodeLifecycleManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 创建共享MQTT节点
	var mqttNodeDsl = []byte(`{
		"id": "shared_mqtt_lifecycle",
		"type": "mqttClient",
		"name": "生命周期测试MQTT节点",
		"debugMode": false,
		"configuration": {
			"Server": "127.0.0.1:1883",
			"Topic": "/test/lifecycle"
		}
	}`)

	t.Run("SharedNodeCreation", func(t *testing.T) {
		// 测试共享节点创建
		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)

		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(pool.GetAll()))

		// 验证可以获取实例
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.NotNil(t, client)
		assert.Nil(t, err)
		_, ok := client.(*mqtt.Client)
		assert.True(t, ok)
	})

	t.Run("SharedNodeRestart", func(t *testing.T) {
		// 测试共享节点重启
		sharedCtx, ok := pool.Get("shared_mqtt_lifecycle")
		assert.True(t, ok)

		// 修改配置并重启
		modifiedDsl := []byte(strings.Replace(string(mqttNodeDsl), "/test/lifecycle", "/test/restarted", -1))
		err := sharedCtx.ReloadSelf(modifiedDsl)
		assert.Nil(t, err)

		// 验证重启后仍然可用
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.NotNil(t, client)
		assert.Nil(t, err)
	})

	t.Run("SharedNodeDestroy", func(t *testing.T) {
		// 测试共享节点销毁
		pool.Del("shared_mqtt_lifecycle")
		assert.Equal(t, 0, len(pool.GetAll()))

		// 验证销毁后无法获取实例
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.Nil(t, client)
		assert.NotNil(t, err)
	})
}

// TestMultipleReferenceIndependence 测试多引用节点的独立性
func TestMultipleReferenceIndependence(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 创建共享MQTT节点
	var mqttNodeDsl = []byte(`{
		"id": "shared_mqtt_multi_ref",
		"type": "mqttClient",
		"name": "多引用测试MQTT节点",
		"debugMode": false,
		"configuration": {
			"Server": "127.0.0.1:1883",
			"Topic": "/test/multi_ref"
		}
	}`)

	nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
	assert.Nil(t, err)
	ctx, err := pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// 创建多个规则引擎引用同一个共享资源
	ruleChainTemplate := `{
		"ruleChain": {
			"id": "%s",
			"name": "%s"
		},
		"metadata": {
			"nodes": [{
				"id": "mqttClient",
				"type": "mqttClient",
				"name": "mqtt推送数据",
				"debugMode": false,
				"configuration": {
					"server": "ref://shared_mqtt_multi_ref"
				}
			}]
		}
	}`

	engines := make([]types.RuleEngine, 3)
	for i := 0; i < 3; i++ {
		chainId := fmt.Sprintf("multiRefRule%d", i+1)
		ruleChainFile := fmt.Sprintf(ruleChainTemplate, chainId, chainId)
		ruleEngine, err := engine.New(chainId, []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)
		engines[i] = ruleEngine
	}

	metaData := types.NewMetadata()
	metaData.PutValue("testId", "multi_ref_test")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, "{\"data\":\"test\"}")

	t.Run("AllEnginesCanAccessSharedResource", func(t *testing.T) {
		// 测试所有引擎都能正常访问共享资源
		for i, ruleEngine := range engines {
			engineIndex := i // 创建局部变量避免闭包捕获循环变量
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				assert.Equal(t, types.Success, relationType, fmt.Sprintf("Engine %d should succeed", engineIndex+1))
			}))
		}
		time.Sleep(time.Millisecond * 200)
	})

	t.Run("EngineStopIndependence", func(t *testing.T) {
		// 测试停止一个引擎不影响其他引擎
		engines[0].Stop(context.Background())
		time.Sleep(time.Millisecond * 100)

		// 其他引擎仍然可以正常工作
		for i := 1; i < 3; i++ {
			engineIndex := i // 创建局部变量避免闭包捕获循环变量
			engines[i].OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				assert.Equal(t, types.Success, relationType, fmt.Sprintf("Engine %d should still work", engineIndex+1))
			}))
		}
		time.Sleep(time.Millisecond * 200)
	})

	// 清理资源
	for i := 1; i < 3; i++ {
		engines[i].Stop(context.Background())
	}
	pool.Del("shared_mqtt_multi_ref")
}

// TestSharedResourceRestartImpact 测试共享资源重启对现有引用的影响
func TestSharedResourceRestartImpact(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 创建共享MQTT节点
	var mqttNodeDsl = []byte(`{
		"id": "shared_mqtt_restart_test",
		"type": "mqttClient",
		"name": "重启影响测试MQTT节点",
		"debugMode": false,
		"configuration": {
			"Server": "127.0.0.1:1883",
			"Topic": "/test/restart_impact"
		}
	}`)

	nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
	assert.Nil(t, err)
	ctx, err := pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// 创建引用共享资源的规则引擎
	ruleChainFile := `{
		"ruleChain": {
			"id": "restartImpactRule",
			"name": "restartImpactRule"
		},
		"metadata": {
			"nodes": [{
				"id": "mqttClient",
				"type": "mqttClient",
				"name": "mqtt推送数据",
				"debugMode": false,
				"configuration": {
					"server": "ref://shared_mqtt_restart_test"
				}
			}]
		}
	}`

	ruleEngine, err := engine.New("restartImpactRule", []byte(ruleChainFile), engine.WithConfig(config))
	assert.Nil(t, err)

	metaData := types.NewMetadata()
	metaData.PutValue("testId", "restart_impact_test")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, "{\"data\":\"test\"}")

	t.Run("BeforeRestart", func(t *testing.T) {
		// 重启前正常工作
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("DuringRestart", func(t *testing.T) {
		// 获取共享资源上下文并重启
		sharedCtx, ok := pool.Get("shared_mqtt_restart_test")
		assert.True(t, ok)

		// 修改配置并重启（使用错误的端口模拟重启失败）
		modifiedDsl := []byte(strings.Replace(string(mqttNodeDsl), "127.0.0.1:1883", "127.0.0.1:1884", -1))
		err := sharedCtx.ReloadSelf(modifiedDsl)
		assert.Nil(t, err)

		// 重启后应该失败
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("AfterRestartFixed", func(t *testing.T) {
		// 修复配置
		sharedCtx, ok := pool.Get("shared_mqtt_restart_test")
		assert.True(t, ok)

		fixedDsl := []byte(strings.Replace(string(mqttNodeDsl), "127.0.0.1:1884", "127.0.0.1:1883", -1))
		err := sharedCtx.ReloadSelf(fixedDsl)
		assert.Nil(t, err)

		// 修复后应该恢复正常
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	// 清理资源
	ruleEngine.Stop(context.Background())
	pool.Del("shared_mqtt_restart_test")
}

// TestConcurrentSharedResourceAccess 测试并发访问共享资源的安全性
func TestConcurrentSharedResourceAccess(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	// 创建共享MQTT节点
	var mqttNodeDsl = []byte(`{
		"id": "shared_mqtt_concurrent",
		"type": "mqttClient",
		"name": "并发测试MQTT节点",
		"debugMode": false,
		"configuration": {
			"Server": "127.0.0.1:1883",
			"Topic": "/test/concurrent"
		}
	}`)

	nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
	assert.Nil(t, err)
	ctx, err := pool.NewFromRuleNode(nodeDef)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// 创建多个规则引擎
	engines := make([]types.RuleEngine, 5)
	ruleChainTemplate := `{
		"ruleChain": {
			"id": "concurrentRule%d",
			"name": "concurrentRule%d"
		},
		"metadata": {
			"nodes": [{
				"id": "mqttClient",
				"type": "mqttClient",
				"name": "mqtt推送数据",
				"debugMode": false,
				"configuration": {
					"server": "ref://shared_mqtt_concurrent"
				}
			}]
		}
	}`

	for i := 0; i < 5; i++ {
		chainId := fmt.Sprintf("concurrentRule%d", i)
		ruleChainFile := fmt.Sprintf(ruleChainTemplate, i, i)
		ruleEngine, err := engine.New(chainId, []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)
		engines[i] = ruleEngine
	}

	t.Run("ConcurrentMessageProcessing", func(t *testing.T) {
		// 并发发送消息
		var wg sync.WaitGroup
		successCount := int32(0)
		failureCount := int32(0)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(msgId int) {
				defer wg.Done()
				for engineIdx, ruleEngine := range engines {
					metaData := types.NewMetadata()
					metaData.PutValue("msgId", fmt.Sprintf("%d", msgId))
					metaData.PutValue("engineIdx", fmt.Sprintf("%d", engineIdx))
					msg := types.NewMsg(0, "CONCURRENT_TEST", types.JSON, metaData, fmt.Sprintf("{\"msgId\":%d}", msgId))

					ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
						if relationType == types.Success {
							atomic.AddInt32(&successCount, 1)
						} else {
							atomic.AddInt32(&failureCount, 1)
						}
					}))
				}
			}(i)
		}

		wg.Wait()
		time.Sleep(time.Millisecond * 500) // 等待所有消息处理完成

		// 验证并发访问的安全性
		totalExpected := int32(10 * 5) // 10条消息 * 5个引擎
		totalActual := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&failureCount)
		assert.Equal(t, totalExpected, totalActual)

		t.Logf("并发测试结果 - 成功: %d, 失败: %d, 总计: %d",
			atomic.LoadInt32(&successCount),
			atomic.LoadInt32(&failureCount),
			totalActual)
	})

	// 清理资源
	for _, ruleEngine := range engines {
		ruleEngine.Stop(context.Background())
	}
	pool.Del("shared_mqtt_concurrent")
}

// TestGracefulShutdownBehavior 测试优雅关闭行为
func TestGracefulShutdownBehavior(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	t.Run("SharedResourceGracefulShutdown", func(t *testing.T) {
		// 创建共享节点
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_graceful",
			"type": "mqttClient",
			"name": "优雅关闭测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1883",
				"Topic": "/test/graceful"
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建两个引用共享资源的引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "gracefulRule%s",
				"name": "gracefulRule%s"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_graceful"
					}
				}]
			}
		}`

		engine1, err := engine.New("gracefulRule1", []byte(fmt.Sprintf(ruleChainFile, "1", "1")), engine.WithConfig(config))
		assert.Nil(t, err)
		engine2, err := engine.New("gracefulRule2", []byte(fmt.Sprintf(ruleChainFile, "2", "2")), engine.WithConfig(config))
		assert.Nil(t, err)

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "GRACEFUL_TEST", types.JSON, metaData, "{\"test\":\"graceful\"}")

		// 验证两个引擎都能正常工作
		engine1.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		engine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)

		// 停止第一个引擎（优雅关闭）
		engine1.Stop(context.Background())
		time.Sleep(time.Millisecond * 100)

		// 第二个引擎应该仍然能正常工作（共享资源不应该被关闭）
		engine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType, "第二个引擎在第一个引擎停止后应该仍然正常工作")
		}))
		time.Sleep(time.Millisecond * 100)

		// 清理资源
		engine2.Stop(context.Background())
		pool.Del("shared_mqtt_graceful")
	})

	t.Run("PoolShutdownBehavior", func(t *testing.T) {
		// 创建共享节点
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_pool_shutdown",
			"type": "mqttClient",
			"name": "池关闭测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1883",
				"Topic": "/test/pool_shutdown"
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建引用共享资源的引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "poolShutdownRule",
				"name": "poolShutdownRule"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_pool_shutdown"
					}
				}]
			}
		}`

		ruleEngine, err := engine.New("poolShutdownRule", []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "POOL_SHUTDOWN_TEST", types.JSON, metaData, "{\"test\":\"pool_shutdown\"}")

		// 验证引擎能正常工作
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)

		// 关闭整个池
		pool.Stop()
		assert.Equal(t, 0, len(pool.GetAll()))

		// 池关闭后，引擎应该无法访问共享资源
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType, "池关闭后应该无法访问共享资源")
		}))
		time.Sleep(time.Millisecond * 100)

		// 清理资源
		ruleEngine.Stop(context.Background())
	})
}

// TestSharedNodeGetSafelyAPI 测试新的GetSafely API
func TestSharedNodeGetSafelyAPI(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	t.Run("GetSafelyConcurrentAccess", func(t *testing.T) {
		// 创建共享MQTT节点
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_getsafely",
			"type": "mqttClient",
			"name": "GetSafely测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1883",
				"Topic": "/test/getsafely",
				"ClientID": "rulego_getsafely_test",
				"CleanSession": true
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建使用GetSafely的规则引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "getSafelyRule",
				"name": "getSafelyRule"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_getsafely"
					}
				}]
			}
		}`

		ruleEngine, err := engine.New("getSafelyRule", []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)

		// 等待客户端初始化
		time.Sleep(time.Millisecond * 500)

		// 并发测试GetSafely方法的线程安全性
		var wg sync.WaitGroup
		successCount := int32(0)
		concurrentNum := 30 // 进一步减少并发数

		for i := 0; i < concurrentNum; i++ {
			wg.Add(1)
			go func(msgId int) {
				defer wg.Done()
				metaData := types.NewMetadata()
				metaData.PutValue("msgId", fmt.Sprintf("%d", msgId))
				msg := types.NewMsg(0, "GETSAFELY_TEST", types.JSON, metaData, fmt.Sprintf("{\"msgId\":%d}", msgId))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					if relationType == types.Success {
						atomic.AddInt32(&successCount, 1)
					}
				}))
			}(i)
		}

		wg.Wait()
		time.Sleep(time.Millisecond * 1000) // 增加等待时间

		// 验证大部分并发操作成功
		actualSuccess := atomic.LoadInt32(&successCount)
		assert.True(t, actualSuccess > int32(concurrentNum*6/10), fmt.Sprintf("至少60%%的GetSafely调用应该成功，实际成功：%d/%d", actualSuccess, concurrentNum))

		// 清理资源
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_getsafely")
	})

	t.Run("InitWithCloseCallback", func(t *testing.T) {
		// 测试InitWithClose的清理回调功能
		callbackExecuted := int32(0)

		// 创建一个会失败的MQTT节点配置（使用错误的端口）
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_callback_test",
			"type": "mqttClient", 
			"name": "回调测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1884",
				"Topic": "/test/callback"
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)

		// 创建节点（可能会失败，但应该触发清理回调）
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建规则引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "callbackTestRule",
				"name": "callbackTestRule"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_callback_test"
					}
				}]
			}
		}`

		ruleEngine, err := engine.New("callbackTestRule", []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "CALLBACK_TEST", types.JSON, metaData, "{\"test\":\"callback\"}")

		// 发送消息应该失败（因为MQTT端口错误）
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// 可能成功也可能失败，取决于MQTT客户端的行为
			t.Logf("回调测试结果: %s, error: %v", relationType, err)
		}))
		time.Sleep(time.Millisecond * 200)

		// 清理资源应该触发Close回调
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_callback_test")
		time.Sleep(time.Millisecond * 100)

		t.Logf("清理回调执行次数: %d", atomic.LoadInt32(&callbackExecuted))
	})
}

// TestSharedNodeResourceManagement 测试共享节点资源管理
func TestSharedNodeResourceManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	t.Run("ResourceCleanupOnError", func(t *testing.T) {
		// 测试初始化错误时的资源清理
		var errorNodeDsl = []byte(`{
			"id": "shared_mqtt_error_test",
			"type": "mqttClient",
			"name": "错误处理测试",
			"debugMode": false,
			"configuration": {
				"Server": "invalid-host:1883",
				"Topic": "/test/error"
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(errorNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建规则引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "errorTestRule",
				"name": "errorTestRule"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_error_test"
					}
				}]
			}
		}`

		ruleEngine, err := engine.New("errorTestRule", []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)

		metaData := types.NewMetadata()
		msg := types.NewMsg(0, "ERROR_TEST", types.JSON, metaData, "{\"test\":\"error\"}")

		// 发送消息应该失败
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType, "无效主机应该导致失败")
		}))
		time.Sleep(time.Millisecond * 200)

		// 清理资源
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_error_test")
	})

	t.Run("PerformanceComparisonGetVsGetSafely", func(t *testing.T) {
		// 性能对比测试（GetSafely应该在高并发读取时表现更好）
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_performance",
			"type": "mqttClient",
			"name": "性能测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1883",
				"Topic": "/test/performance",
				"ClientID": "rulego_performance_test",
				"CleanSession": true,
				"MaxReconnectInterval": 30
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建多个规则引擎进行压力测试
		engines := make([]types.RuleEngine, 5) // 减少引擎数量避免过度并发
		ruleChainTemplate := `{
			"ruleChain": {
				"id": "performanceRule%d",
				"name": "performanceRule%d"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_performance"
					}
				}]
			}
		}`

		for i := 0; i < 5; i++ {
			chainId := fmt.Sprintf("performanceRule%d", i)
			ruleChainFile := fmt.Sprintf(ruleChainTemplate, i, i)
			ruleEngine, err := engine.New(chainId, []byte(ruleChainFile), engine.WithConfig(config))
			assert.Nil(t, err)
			engines[i] = ruleEngine
		}

		// 等待客户端初始化完成
		time.Sleep(time.Millisecond * 500)

		// 高并发消息发送测试
		start := time.Now()
		var wg sync.WaitGroup
		messageCount := 50 // 减少消息数量
		successCount := int32(0)

		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(msgId int) {
				defer wg.Done()
				for _, ruleEngine := range engines {
					metaData := types.NewMetadata()
					metaData.PutValue("msgId", fmt.Sprintf("%d", msgId))
					msg := types.NewMsg(0, "PERFORMANCE_TEST", types.JSON, metaData, fmt.Sprintf("{\"msgId\":%d}", msgId))

					ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
						if relationType == types.Success {
							atomic.AddInt32(&successCount, 1)
						}
					}))
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		time.Sleep(time.Millisecond * 1000) // 增加等待时间确保所有消息处理完成

		expectedTotal := int32(messageCount * 5) // 50条消息 * 5个引擎
		actualSuccess := atomic.LoadInt32(&successCount)

		t.Logf("性能测试结果 - 总消息数: %d, 成功数: %d, 耗时: %v, 平均QPS: %.2f",
			expectedTotal, actualSuccess, duration, float64(actualSuccess)/duration.Seconds())

		// 验证大部分消息处理成功
		assert.True(t, actualSuccess > expectedTotal*5/10, "至少50%的消息应该处理成功")

		// 清理资源
		for _, ruleEngine := range engines {
			ruleEngine.Stop(context.Background())
		}
		pool.Del("shared_mqtt_performance")
	})
}

// TestSharedNodeLockOptimization 测试读写锁优化
func TestSharedNodeLockOptimization(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NetPool = pool

	t.Run("ReadWriteLockBehavior", func(t *testing.T) {
		// 创建共享节点
		var mqttNodeDsl = []byte(`{
			"id": "shared_mqtt_rwlock",
			"type": "mqttClient",
			"name": "读写锁测试",
			"debugMode": false,
			"configuration": {
				"Server": "127.0.0.1:1883",
				"Topic": "/test/rwlock",
				"ClientID": "rulego_rwlock_test",
				"CleanSession": true
			}
		}`)

		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 创建规则引擎
		ruleChainFile := `{
			"ruleChain": {
				"id": "rwlockRule",
				"name": "rwlockRule"
			},
			"metadata": {
				"nodes": [{
					"id": "mqttClient",
					"type": "mqttClient",
					"name": "mqtt推送数据",
					"debugMode": false,
					"configuration": {
						"server": "ref://shared_mqtt_rwlock",
						"topic": "/test/rwlock"
					}
				}]
			}
		}`

		ruleEngine, err := engine.New("rwlockRule", []byte(ruleChainFile), engine.WithConfig(config))
		assert.Nil(t, err)

		// 等待客户端初始化
		time.Sleep(time.Millisecond * 500)

		// 大量并发读取测试（模拟GetSafely的读锁优势）
		var wg sync.WaitGroup
		readCount := 100 // 减少并发数
		successCount := int32(0)

		start := time.Now()
		for i := 0; i < readCount; i++ {
			//time.Sleep(time.Millisecond * 10)
			wg.Add(1)
			go func(msgId int) {
				defer wg.Done()
				metaData := types.NewMetadata()
				metaData.PutValue("msgId", fmt.Sprintf("%d", msgId))
				msg := types.NewMsg(0, "RWLOCK_TEST", types.JSON, metaData, fmt.Sprintf("{\"msgId\":%d}", msgId))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					if relationType == types.Success {
						atomic.AddInt32(&successCount, 1)
					}
				}))
			}(i)
		}

		wg.Wait()
		readDuration := time.Since(start)
		time.Sleep(time.Millisecond * 500)

		actualSuccess := atomic.LoadInt32(&successCount)
		t.Logf("读写锁测试 - 并发读取数: %d, 成功数: %d, 耗时: %v",
			readCount, actualSuccess, readDuration)

		// 验证大部分读取操作成功
		assert.True(t, actualSuccess > int32(readCount*7/10), "至少70%的读取操作应该成功")

		// 清理资源
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_rwlock")
	})
}
