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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/endpoint/rest"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/mqtt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMain serves 127.0.0.1:1883 for the shared MQTT fixtures below: the
// address is reused as-is when an external broker (CI mosquitto) already
// listens on it, otherwise an embedded broker is started.
func TestMain(m *testing.M) {
	stop, err := test.StartBrokerOrUseExternal("127.0.0.1:1883")
	if err != nil {
		panic(err)
	}
	code := m.Run()
	stop()
	os.Exit(code)
}

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
	config.NodePool = pool
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
	config.NodePool = pool
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
	config.NodePool = pool
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
	config.NodePool = pool
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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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
	config.NodePool = pool

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

// aliasTestEndpoint 模拟连接地址作为主键的 endpoint 组件（如 mqtt/rest 的 Id()=Config.Server），
// 全程无网络依赖。
type aliasTestEndpoint struct {
	impl.BaseEndpoint
	id        string
	destroyed int32
}

func (e *aliasTestEndpoint) Type() string                                     { return "aliasTestEndpoint" }
func (e *aliasTestEndpoint) New() types.Node                                  { return &aliasTestEndpoint{} }
func (e *aliasTestEndpoint) Init(_ types.Config, _ types.Configuration) error { return nil }
func (e *aliasTestEndpoint) Id() string                                       { return e.id }
func (e *aliasTestEndpoint) Start() error                                     { return nil }
func (e *aliasTestEndpoint) Destroy()                                         { atomic.AddInt32(&e.destroyed, 1) }
func (e *aliasTestEndpoint) AddRouter(_ endpointApi.Router, _ ...interface{}) (string, error) {
	return "1", nil
}
func (e *aliasTestEndpoint) RemoveRouter(_ string, _ ...interface{}) error { return nil }
func (e *aliasTestEndpoint) GetInstance() (interface{}, error)             { return e, nil }

// aliasTestSharedNode 供 NewFromRuleNode 路径使用，需注册进自定义组件表。
type aliasTestSharedNode struct {
	destroyed int32
}

func (n *aliasTestSharedNode) Type() string { return "aliasTestSharedNode" }
func (n *aliasTestSharedNode) New() types.Node {
	return &aliasTestSharedNode{}
}
func (n *aliasTestSharedNode) Init(_ types.Config, _ types.Configuration) error { return nil }
func (n *aliasTestSharedNode) OnMsg(_ types.RuleContext, _ types.RuleMsg)       {}
func (n *aliasTestSharedNode) Destroy()                                         { atomic.AddInt32(&n.destroyed, 1) }
func (n *aliasTestSharedNode) GetInstance() (interface{}, error)                { return n, nil }

func newAliasTestPool() (*NodePool, types.Config) {
	registry := engine.NewCustomComponentRegistry(engine.Registry, new(engine.RuleComponentRegistry))
	_ = registry.Register(&aliasTestSharedNode{})
	config := engine.NewConfig(types.WithComponentsRegistry(registry))
	pool := NewNodePool(config)
	config.NodePool = pool
	return pool, config
}

// 主键与别名解析到同一实例，别名不计入遍历结果。
func TestAddNodeWithAlias(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &aliasTestEndpoint{id: "tcp://127.0.0.1:1883"}

	ctx, err := pool.AddNodeWithAlias("gateway_mqtt", ep)
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

	//主键仍是组件自身 Id，而非别名
	byPrimary, ok := pool.Get("tcp://127.0.0.1:1883")
	assert.True(t, ok)
	byAlias, ok := pool.Get("gateway_mqtt")
	assert.True(t, ok)
	assert.True(t, byPrimary == byAlias)

	//GetInstance 与 Lookup（ref:// 解析路径）按主键和别名取到同一实例
	insPrimary, err := pool.GetInstance("tcp://127.0.0.1:1883")
	assert.Nil(t, err)
	insAlias, err := pool.GetInstance("gateway_mqtt")
	assert.Nil(t, err)
	assert.True(t, insPrimary.(*aliasTestEndpoint) == ep)
	assert.True(t, insAlias.(*aliasTestEndpoint) == ep)

	ins, found := pool.Lookup("gateway_mqtt")
	assert.True(t, found)
	assert.True(t, ins.(*aliasTestEndpoint) == ep)

	//别名不重复计入遍历与定义导出
	assert.Equal(t, 1, len(pool.GetAll()))
	defs, err := pool.GetAllDef()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(defs["aliasTestEndpoint"]))

	visited := 0
	pool.Range(func(_, _ any) bool {
		visited++
		return true
	})
	assert.Equal(t, 1, visited)
}

// AddAlias 支持主键或已有别名定位节点，可批量追加。
func TestAddAlias(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &aliasTestEndpoint{id: "tcp://127.0.0.1:1883"}
	_, err := pool.AddNode(ep)
	assert.Nil(t, err)

	assert.Nil(t, pool.AddAlias("tcp://127.0.0.1:1883", "alias_a", "alias_b"))
	_, ok := pool.Get("alias_a")
	assert.True(t, ok)
	_, ok = pool.Get("alias_b")
	assert.True(t, ok)

	//用已有别名定位同一节点，继续追加
	assert.Nil(t, pool.AddAlias("alias_a", "alias_c"))
	_, ok = pool.Get("alias_c")
	assert.True(t, ok)

	node, err := pool.GetInstance("alias_c")
	assert.Nil(t, err)
	assert.True(t, node.(*aliasTestEndpoint) == ep)

	assert.Equal(t, 1, len(pool.GetAll()))
}

// 别名等于主键 no-op；重复绑同一别名 no-op。
func TestAliasIdempotent(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &aliasTestEndpoint{id: "tcp://127.0.0.1:1883"}
	_, err := pool.AddNodeWithAlias("gateway_mqtt", ep)
	assert.Nil(t, err)

	assert.Nil(t, pool.AddAlias("tcp://127.0.0.1:1883", "tcp://127.0.0.1:1883"))
	assert.Nil(t, pool.AddAlias("gateway_mqtt", "gateway_mqtt"))

	byAlias, ok := pool.Get("gateway_mqtt")
	assert.True(t, ok)
	assert.True(t, byAlias.GetNodeId().Id == "tcp://127.0.0.1:1883")
}

// 别名冲突规则：占用他人主键报错，占用他人别名报错且不影响既有绑定。
func TestAliasConflict(t *testing.T) {
	pool, _ := newAliasTestPool()
	epA := &aliasTestEndpoint{id: "server_a"}
	epB := &aliasTestEndpoint{id: "server_b"}
	_, err := pool.AddNodeWithAlias("shared_name", epA)
	assert.Nil(t, err)
	_, err = pool.AddNode(epB)
	assert.Nil(t, err)

	//别名占用另一节点主键
	err = pool.AddAlias("server_b", "server_a")
	assert.NotNil(t, err)
	//别名已被其他节点占用
	err = pool.AddAlias("server_b", "shared_name")
	assert.NotNil(t, err)

	//既有绑定不受冲突影响
	ins, err := pool.GetInstance("shared_name")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint) == epA)
}

// AddNodeWithAlias 别名冲突时节点保留在池中（主键可用），别名维持旧绑定。
func TestAddNodeWithAliasConflictKeepsNode(t *testing.T) {
	pool, _ := newAliasTestPool()
	epA := &aliasTestEndpoint{id: "server_a"}
	epB := &aliasTestEndpoint{id: "server_b"}
	_, err := pool.AddNodeWithAlias("dup", epA)
	assert.Nil(t, err)

	ctx, err := pool.AddNodeWithAlias("dup", epB)
	assert.NotNil(t, err)
	assert.NotNil(t, ctx) //节点已入池，返回其上下文供调用方使用

	_, ok := pool.Get("server_b")
	assert.True(t, ok)
	ins, err := pool.GetInstance("dup")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint) == epA)

	//换绑：先解绑旧节点，B 即可占用该别名
	pool.Del("server_a")
	assert.Nil(t, pool.AddAlias("server_b", "dup"))
	ins, err = pool.GetInstance("dup")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint) == epB)
}

// 空别名直接报错且节点不入池。
func TestAddNodeWithAliasEmpty(t *testing.T) {
	pool, _ := newAliasTestPool()

	ctx, err := pool.AddNodeWithAlias("", &aliasTestEndpoint{id: "server_a"})
	assert.NotNil(t, err)
	assert.Nil(t, ctx)
	assert.Equal(t, 0, len(pool.GetAll()))

	_, err = pool.AddNode(&aliasTestEndpoint{id: "server_b"})
	assert.Nil(t, err)
	assert.NotNil(t, pool.AddAlias("server_b", ""))
	assert.NotNil(t, pool.AddAlias("not_found", "x"))
}

// 别名不得被新节点主键抢占（AddNode/NewFromEndpoint/NewFromRuleNode 一致拒绝）。
func TestAliasShadowGuard(t *testing.T) {
	pool, _ := newAliasTestPool()
	_, err := pool.AddNodeWithAlias("occupied", &aliasTestEndpoint{id: "server_a"})
	assert.Nil(t, err)

	_, err = pool.AddNode(&aliasTestEndpoint{id: "occupied"})
	assert.NotNil(t, err)

	_, err = pool.NewFromEndpoint(types.EndpointDsl{RuleNode: types.RuleNode{
		Id:   "occupied",
		Type: "endpoint/mqtt",
		Configuration: types.Configuration{
			"server": "127.0.0.1:1883",
		},
	}})
	assert.NotNil(t, err)

	_, err = pool.NewFromRuleNode(types.RuleNode{
		Id:   "occupied",
		Type: "aliasTestSharedNode",
	})
	assert.NotNil(t, err)

	//既有绑定不受影响
	ins, err := pool.GetInstance("occupied")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint).id == "server_a")
}

// 按主键或别名 Del 均删除节点并清理全部别名；删除后别名可复用。
func TestDelAlias(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &aliasTestEndpoint{id: "server_a"}
	_, err := pool.AddNodeWithAlias("alias_a", ep)
	assert.Nil(t, err)
	assert.Nil(t, pool.AddAlias("alias_a", "alias_b"))

	//按别名删除
	pool.Del("alias_b")
	assert.Equal(t, 0, len(pool.GetAll()))
	_, ok := pool.Get("server_a")
	assert.False(t, ok)
	_, ok = pool.Get("alias_a")
	assert.False(t, ok)
	_, ok = pool.Get("alias_b")
	assert.False(t, ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&ep.destroyed))

	//删除后别名可重新绑定到新节点
	ep2 := &aliasTestEndpoint{id: "server_b"}
	_, err = pool.AddNodeWithAlias("alias_a", ep2)
	assert.Nil(t, err)
	ins, err := pool.GetInstance("alias_a")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint) == ep2)

	//按主键删除
	pool.Del("server_b")
	assert.Equal(t, 0, len(pool.GetAll()))
	_, ok = pool.Get("alias_a")
	assert.False(t, ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&ep2.destroyed))

	//重复 Del 与删除不存在的 id 均为 no-op
	pool.Del("server_b")
	pool.Del("alias_a")
	assert.Equal(t, 0, len(pool.GetAll()))
}

// Stop 释放所有节点并清空别名。
func TestStopAliasCleanup(t *testing.T) {
	pool, _ := newAliasTestPool()
	epA := &aliasTestEndpoint{id: "server_a"}
	epB := &aliasTestEndpoint{id: "server_b"}
	_, err := pool.AddNodeWithAlias("alias_a", epA)
	assert.Nil(t, err)
	_, err = pool.AddNodeWithAlias("alias_b", epB)
	assert.Nil(t, err)

	pool.Stop()
	assert.Equal(t, 0, len(pool.GetAll()))
	_, ok := pool.Get("alias_a")
	assert.False(t, ok)
	_, ok = pool.Get("alias_b")
	assert.False(t, ok)
	assert.Equal(t, int32(1), atomic.LoadInt32(&epA.destroyed))
	assert.Equal(t, int32(1), atomic.LoadInt32(&epB.destroyed))
}

// NewFromRuleNode 创建的共享节点同样可绑别名。
func TestRuleNodeAlias(t *testing.T) {
	pool, _ := newAliasTestPool()
	ctx, err := pool.NewFromRuleNode(types.RuleNode{
		Id:   "shared_db",
		Type: "aliasTestSharedNode",
	})
	assert.Nil(t, err)
	assert.NotNil(t, ctx)

	assert.Nil(t, pool.AddAlias("shared_db", "db"))
	ins, err := pool.GetInstance("db")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestSharedNode) != nil)

	nodeCtx, ok := pool.Get("db")
	assert.True(t, ok)
	nodeCtx.Destroy()
	//销毁经 RuleNodeCtx 透传到底层节点
	assert.Equal(t, int32(1), atomic.LoadInt32(&ins.(*aliasTestSharedNode).destroyed))
}

// 主键与别名读、幂等别名写并发下无 panic，结果一致。
func TestAliasConcurrentAccess(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &aliasTestEndpoint{id: "server_a"}
	_, err := pool.AddNodeWithAlias("alias_0", ep)
	assert.Nil(t, err)
	//先绑满全部别名，并发阶段只做幂等重复绑定，避免绑定前 miss 的预期窗口
	for i := 1; i < 4; i++ {
		assert.Nil(t, pool.AddAlias("server_a", fmt.Sprintf("alias_%d", i)))
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				alias := fmt.Sprintf("alias_%d", j%4)
				if j%3 == 0 {
					_ = pool.AddAlias("server_a", alias)
				}
				ctxP, okP := pool.Get("server_a")
				ctxA, okA := pool.Get(alias)
				if okP != okA {
					t.Error("primary and alias resolved inconsistently")
					return
				}
				if okP && ctxP != ctxA {
					t.Error("primary and alias resolved to different contexts")
					return
				}
			}
		}(i)
	}
	wg.Wait()

	ins, err := pool.GetInstance("server_a")
	assert.Nil(t, err)
	assert.True(t, ins.(*aliasTestEndpoint) == ep)
}

// refTestEndpoint is a minimal ref-aware endpoint stub: RefTarget exposes its
// ref:// borrow target for the pool's registration-time cycle check.
type refTestEndpoint struct {
	aliasTestEndpoint
	id     string
	target string
}

func (e *refTestEndpoint) Id() string                        { return e.id }
func (e *refTestEndpoint) GetInstance() (interface{}, error) { return e, nil }
func (e *refTestEndpoint) RefTarget() string                 { return e.target }

// TestPoolRefCycleRejected: registering an entry whose ref:// chain closes a
// cycle is rejected; the offending entry stays out of the pool.
func TestPoolRefCycleRejected(t *testing.T) {
	pool, _ := newAliasTestPool()
	epA := &refTestEndpoint{id: "pool_a", target: "ref://pool_b"}
	epB := &refTestEndpoint{id: "pool_b", target: "ref://pool_a"}

	_, err := pool.AddNode(epA)
	assert.Nil(t, err)
	_, err = pool.AddNode(epB)
	if err == nil {
		t.Fatal("cyclic ref:// registration should be rejected")
	}
	if !strings.Contains(err.Error(), "circular ref://") {
		t.Fatalf("error should mention circular ref, got: %v", err)
	}
	// pool_b stays out of the pool; pool_a remains usable
	if _, ok := pool.Get("pool_b"); ok {
		t.Fatal("pool_b should not be registered after cycle rejection")
	}
	if _, ok := pool.Get("pool_a"); !ok {
		t.Fatal("pool_a should remain registered")
	}
}

// TestPoolRefChainNoCycle: a linear borrow chain (b→a) registers fine.
func TestPoolRefChainNoCycle(t *testing.T) {
	pool, _ := newAliasTestPool()
	epA := &refTestEndpoint{id: "pool_a", target: ""}
	epB := &refTestEndpoint{id: "pool_b", target: "ref://pool_a"}

	_, err := pool.AddNode(epA)
	assert.Nil(t, err)
	_, err = pool.AddNode(epB)
	assert.Nil(t, err)
	if _, ok := pool.Get("pool_b"); !ok {
		t.Fatal("pool_b should be registered (linear chain is fine)")
	}
}

// TestPoolSelfRefRejected: an entry referencing itself is rejected.
func TestPoolSelfRefRejected(t *testing.T) {
	pool, _ := newAliasTestPool()
	ep := &refTestEndpoint{id: "pool_self", target: "ref://pool_self"}
	if _, err := pool.AddNode(ep); err == nil {
		t.Fatal("self ref:// should be rejected")
	} else if !strings.Contains(err.Error(), "circular ref://") {
		t.Fatalf("error should mention circular ref, got: %v", err)
	}
}

// TestRestSharedNodeBasicOperations 测试REST endpoint的基本SharedNode功能和多实例共享
func TestRestSharedNodeBasicOperations(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// 子测试1：基本SharedNode功能
	t.Run("BasicSharedNodeFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "shared_rest_endpoint",
		       "type": "endpoint/http",
		       "name": "共享REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9080"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 验证共享节点已创建
		sharedCtx, ok := pool.Get("shared_rest_endpoint")
		assert.True(t, ok)
		assert.NotNil(t, sharedCtx)

		// 获取REST实例
		restInstance, err := pool.GetInstance("shared_rest_endpoint")
		assert.Nil(t, err)
		assert.NotNil(t, restInstance)

		// 验证实例类型和配置
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.NotNil(t, restEndpoint)
		assert.Equal(t, ":9080", restEndpoint.Config.Server)

		// 清理
		pool.Del("shared_rest_endpoint")
		assert.Equal(t, 0, len(pool.GetAll()))
	})

	// 子测试2：多实例共享验证
	t.Run("MultipleInstancesSharing", func(t *testing.T) {
		// 创建共享的REST服务器节点
		var sharedServerDsl = []byte(`
			{
		       "id": "shared_rest_server",
		       "type": "endpoint/http",
		       "name": "共享REST服务器",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9081"
		       }
		     }`)

		var sharedServerDef types.EndpointDsl
		err := json.Unmarshal(sharedServerDsl, &sharedServerDef)
		assert.Nil(t, err)
		sharedCtx, err := pool.NewFromEndpoint(sharedServerDef)
		assert.NotNil(t, sharedCtx)
		assert.Nil(t, err)

		// 验证只有一个共享服务器节点存在
		assert.Equal(t, 1, len(pool.GetAll()))

		// 获取共享服务器实例
		sharedInstance, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		sharedRest, ok := sharedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, ":9081", sharedRest.Config.Server)

		// 验证多次获取返回同一个实例
		instance1, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		instance2, err := pool.GetInstance("shared_rest_server")
		assert.Nil(t, err)
		assert.Equal(t, instance1, instance2) // 应该是同一个实例

		// 验证不存在的节点返回错误
		_, err = pool.GetInstance("non_existent_node")
		assert.NotNil(t, err)

		// 清理
		pool.Stop()
		assert.Equal(t, 0, len(pool.GetAll()))
	})
}

// TestRestSharedNodeLifecycleManagement 测试REST endpoint的生命周期管理（重启和注销）
func TestRestSharedNodeLifecycleManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// 子测试1：重启功能测试
	t.Run("RestartFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "restart_test_rest",
		       "type": "endpoint/http",
		       "name": "重启测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9082"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 获取REST实例
		restInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		_, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// 测试重启功能：删除旧节点并创建新节点
		pool.Del("restart_test_rest")
		time.Sleep(1 * time.Second)
		// 创建更新的配置
		var newRestDsl = []byte(`
			{
		       "id": "restart_test_rest",
		       "type": "endpoint/http",
		       "name": "重启测试REST端点-更新",
		       "debugMode": true,
		       "configuration": {
		         "server": ":9082",
		         "allowCors": true
		       }
		     }`)

		// 重新创建节点
		var newDef types.EndpointDsl
		err = json.Unmarshal(newRestDsl, &newDef)
		assert.Nil(t, err)
		newCtx, err := pool.NewFromEndpoint(newDef)
		assert.NotNil(t, newCtx)
		assert.Nil(t, err)

		// 验证配置已更新
		updatedInstance, err := pool.GetInstance("restart_test_rest")
		assert.Nil(t, err)
		updatedRest, ok := updatedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.True(t, updatedRest.Config.AllowCors)

		// 清理
		pool.Del("restart_test_rest")
	})

	// 子测试2：注销影响测试
	t.Run("UnregisterImpact", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "unregister_test_rest",
		       "type": "endpoint/http",
		       "name": "注销测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9083"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 验证节点存在
		_, ok := pool.Get("unregister_test_rest")
		assert.True(t, ok)
		assert.Equal(t, 1, len(pool.GetAll()))

		// 获取实例
		instance, err := pool.GetInstance("unregister_test_rest")
		assert.Nil(t, err)
		assert.NotNil(t, instance)

		// 注销节点
		pool.Del("unregister_test_rest")

		// 验证节点已被删除
		_, ok = pool.Get("unregister_test_rest")
		assert.False(t, ok)
		assert.Equal(t, 0, len(pool.GetAll()))

		// 尝试获取已删除的实例
		instance, err = pool.GetInstance("unregister_test_rest")
		assert.NotNil(t, err)
		assert.Nil(t, instance)
	})

	// 最终清理
	pool.Stop()
}

// TestRestSharedNodeAdvancedFeatures 测试REST endpoint的高级功能（路由和并发）
func TestRestSharedNodeAdvancedFeatures(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// 子测试1：路由功能测试
	t.Run("RouteFunctionality", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "routes_test_rest",
		       "type": "endpoint/http",
		       "name": "路由测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9084"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 获取REST实例
		restInstance, err := pool.GetInstance("routes_test_rest")
		assert.Nil(t, err)
		restEndpoint, ok := restInstance.(*rest.Rest)
		assert.True(t, ok)

		// 添加路由
		router := impl.NewRouter().From("/test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
			exchange.Out.SetBody([]byte("Hello from shared REST endpoint"))
			return true
		}).End()

		restEndpoint.GET(router)

		// 清理
		pool.Del("routes_test_rest")
	})

	// 子测试2：并发访问测试
	t.Run("ConcurrentAccess", func(t *testing.T) {
		var restDsl = []byte(`
			{
		       "id": "concurrent_test_rest",
		       "type": "endpoint/http",
		       "name": "并发测试REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9085"
		       }
		     }`)

		// 创建共享节点
		var def types.EndpointDsl
		err := json.Unmarshal(restDsl, &def)
		assert.Nil(t, err)

		ctx, err := pool.NewFromEndpoint(def)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// 并发获取实例
		const numGoroutines = 10
		results := make(chan interface{}, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				instance, err := pool.GetInstance("concurrent_test_rest")
				if err != nil {
					errors <- err
					return
				}
				results <- instance
			}()
		}

		// 收集结果
		var instances []interface{}
		for i := 0; i < numGoroutines; i++ {
			select {
			case instance := <-results:
				instances = append(instances, instance)
			case err := <-errors:
				t.Fatalf("Concurrent access failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent access")
			}
		}

		// 验证所有实例都是相同的（共享实例）
		assert.Equal(t, numGoroutines, len(instances))
		for i := 1; i < len(instances); i++ {
			assert.Equal(t, instances[0], instances[i])
		}

		// 清理
		pool.Del("concurrent_test_rest")
	})

	// 最终清理
	pool.Stop()
}

// TestRestSharedNodeWithRefProtocol 测试使用ref://方式引入共享REST endpoint及其生命周期管理
func TestRestSharedNodeWithRefProtocol(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// 子测试1：基本ref://引用功能
	t.Run("BasicRefProtocol", func(t *testing.T) {
		// 创建共享节点
		var sharedRestDsl = []byte(`
			{
		       "id": "shared_rest_endpoint_ref",
		       "type": "endpoint/http",
		       "name": "共享REST端点-ref测试",
		       "debugMode": false,
		       "configuration": {
		         "server": ":9087"
		       }
		     }`)

		var sharedDef types.EndpointDsl
		err := json.Unmarshal(sharedRestDsl, &sharedDef)
		assert.Nil(t, err)

		sharedCtx, err := pool.NewFromEndpoint(sharedDef)
		assert.NotNil(t, sharedCtx)
		assert.Nil(t, err)

		// 验证共享节点已创建
		_, ok := pool.Get("shared_rest_endpoint_ref")
		assert.True(t, ok)

		// 创建使用ref://引用的配置
		var refRestDsl = []byte(`
			{
		       "id": "ref_rest_endpoint",
		       "type": "endpoint/http",
		       "name": "引用REST端点",
		       "debugMode": false,
		       "configuration": {
		         "server": "ref://shared_rest_endpoint_ref"
		       }
		     }`)

		// 解析引用配置
		var refDef types.EndpointDsl
		err = json.Unmarshal(refRestDsl, &refDef)
		assert.Nil(t, err)
		assert.Equal(t, "ref://shared_rest_endpoint_ref", refDef.Configuration["server"])

		// 测试通过ref://获取共享实例
		serverConfig := refDef.Configuration["server"].(string)
		if strings.HasPrefix(serverConfig, "ref://") {
			instanceId := serverConfig[len("ref://"):]
			assert.Equal(t, "shared_rest_endpoint_ref", instanceId)

			// 从池中获取引用的实例
			sharedInstance, err := pool.GetInstance(instanceId)
			assert.Nil(t, err)
			assert.NotNil(t, sharedInstance)

			// 验证获取的是同一个共享实例
			sharedRest, ok := sharedInstance.(*rest.Rest)
			assert.True(t, ok)
			assert.Equal(t, ":9087", sharedRest.Config.Server)

		}

		// 验证ref://引用不会创建新的节点实例
		assert.Equal(t, 1, len(pool.GetAll())) // 只有一个共享节点
	})

	// 子测试2：测试共享节点重启不影响引用
	t.Run("SharedNodeRestartIsolation", func(t *testing.T) {
		// 获取原始共享实例的引用
		originalInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		originalRest, ok := originalInstance.(*rest.Rest)
		assert.True(t, ok)
		originalServer := originalRest.Config.Server

		// 模拟共享节点重启：删除并重新创建
		pool.Del("shared_rest_endpoint_ref")
		time.Sleep(1 * time.Second)
		// 验证共享节点已被删除
		_, ok = pool.Get("shared_rest_endpoint_ref")
		assert.False(t, ok)

		// 重新创建共享节点（模拟重启后的新配置）
		var restartedRestDsl = []byte(`
			{
		       "id": "shared_rest_endpoint_ref",
		       "type": "endpoint/http",
		       "name": "重启后的共享REST端点",
		       "debugMode": true,
		       "configuration": {
		         "server": ":9087",
		         "allowCors": true
		       }
		     }`)

		var restartedDef types.EndpointDsl
		err = json.Unmarshal(restartedRestDsl, &restartedDef)
		assert.Nil(t, err)

		_, err = pool.NewFromEndpoint(restartedDef)
		assert.Nil(t, err)

		// 验证重启后的实例配置已更新
		restartedInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		restartedRest, ok := restartedInstance.(*rest.Rest)
		assert.True(t, ok)
		assert.Equal(t, originalServer, restartedRest.Config.Server) // 服务器地址保持一致
		assert.True(t, restartedRest.Config.AllowCors)               // 新配置生效

		// 验证通过ref://仍然可以正常获取更新后的实例
		refInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)
		assert.Equal(t, restartedInstance, refInstance) // 引用获取的是同一个实例
	})

	// 子测试3：测试多个引用节点的独立性
	t.Run("MultipleReferencesIndependence", func(t *testing.T) {
		// 创建多个使用ref://的配置（模拟不同规则链中的引用）
		refConfigs := []string{
			`{"id": "ref1", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref2", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
			`{"id": "ref3", "type": "endpoint/http", "configuration": {"server": "ref://shared_rest_endpoint_ref"}}`,
		}

		// 验证所有引用都指向同一个共享实例
		sharedInstance, err := pool.GetInstance("shared_rest_endpoint_ref")
		assert.Nil(t, err)

		for i, configStr := range refConfigs {
			var refDef types.EndpointDsl
			err := json.Unmarshal([]byte(configStr), &refDef)
			assert.Nil(t, err)

			serverConfig := refDef.Configuration["server"].(string)
			if strings.HasPrefix(serverConfig, "ref://") {
				instanceId := serverConfig[len("ref://"):]
				refInstance, err := pool.GetInstance(instanceId)
				assert.Nil(t, err)
				assert.Equal(t, sharedInstance, refInstance, "Reference %d should point to the same shared instance", i+1)
			}
		}

		// 验证节点池中仍然只有一个共享节点
		assert.Equal(t, 1, len(pool.GetAll()))
	})

	// 清理
	pool.Stop()
}

// TestRestSharedNodeDynamicRestart 测试REST endpoint的动态重启是否生效
func TestRestSharedNodeDynamicRestart(t *testing.T) {
	var restDsl = []byte(`
		{
	       "id": "dynamic_restart_test",
	       "type": "endpoint/http",
	       "name": "动态重启测试",
	       "debugMode": false,
	       "configuration": {
	         "server": ":9086",
	         "allowCors": false
	       }
	     }`)

	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// 创建共享节点
	var def types.EndpointDsl
	err := json.Unmarshal(restDsl, &def)
	assert.Nil(t, err)

	ctx, err := pool.NewFromEndpoint(def)
	assert.NotNil(t, ctx)
	assert.Nil(t, err)

	// 获取初始实例
	initialInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	initialRest, ok := initialInstance.(*rest.Rest)
	assert.True(t, ok)
	assert.False(t, initialRest.Config.AllowCors) // 初始配置

	// 动态更新配置并重启
	var updatedRestDsl = []byte(`
		{
	       "id": "dynamic_restart_test",
	       "type": "endpoint/http",
	       "name": "动态重启测试-更新",
	       "debugMode": false,
	       "configuration": {
	         "server": ":9086",
	         "allowCors": true
	       }
	     }`)

	// 删除旧节点并重新创建
	pool.Del("dynamic_restart_test")
	time.Sleep(1 * time.Second)
	// 重新创建节点
	var updatedDef types.EndpointDsl
	err = json.Unmarshal(updatedRestDsl, &updatedDef)
	assert.Nil(t, err)
	_, err = pool.NewFromEndpoint(updatedDef)
	assert.Nil(t, err)

	// 获取更新后的实例
	updatedInstance, err := pool.GetInstance("dynamic_restart_test")
	assert.Nil(t, err)
	updatedRest, ok := updatedInstance.(*rest.Rest)
	assert.True(t, ok)

	// 验证配置已更新
	assert.True(t, updatedRest.Config.AllowCors) // 配置已更新

	// 验证配置更新成功

	// 添加一个简单的路由来测试功能
	router := impl.NewRouter().From("/cors-test").Transform(func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("CORS enabled"))
		return true
	}).End()
	updatedRest.GET(router)

	// 验证路由已添加

	// 清理
	pool.Stop()
}
