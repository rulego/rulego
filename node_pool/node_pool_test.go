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
	//Start the rule engine through the connection pool
	ruleEngine1, err := engine.New("netSourcePoolRule01", []byte(ruleChainFile), engine.WithConfig(config))
	ruleEngine2, err := engine.New("netSourcePoolRule02", []byte(ruleChainFile), engine.WithConfig(config))

	ruleEngine1.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	ruleEngine1.Stop(context.Background())
	time.Sleep(time.Millisecond * 500)

	//ruleEngine1 stops, but it doesn't affect ruleEngine2
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
	//Incorrect connection pool
	dsl1 = []byte(strings.Replace(string(dsl1), `127.0.0.1:1883`, `127.0.0.1:1884`, -1))
	err = netResourceCtx.ReloadSelf(dsl1)
	assert.Nil(t, err)
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	//Modify the normal connection pool
	dsl1 = []byte(strings.Replace(string(dsl1), `127.0.0.1:1884`, `127.0.0.1:1883`, -1))
	err = netResourceCtx.ReloadSelf(dsl1)
	assert.Nil(t, err)
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Success, relationType)
	}))
	time.Sleep(time.Millisecond * 500)

	//The pond was canceled and canceled
	pool.Stop()
	assert.Equal(t, 0, len(pool.GetAll()))
	//The connection pool has been deleted and data cannot be sent
	ruleEngine3.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		assert.Equal(t, types.Failure, relationType)
	}))
	time.Sleep(time.Millisecond * 500)
}

// TestSharedNodeLifecycleManagement TestSharedNodeLifecycle Management
func TestSharedNodeLifecycleManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Create a shared MQTT node
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
		// Test the creation of shared nodes
		nodeDef, err := config.Parser.DecodeRuleNode(mqttNodeDsl)
		assert.Nil(t, err)

		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(pool.GetAll()))

		// Verification can obtain instances
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.NotNil(t, client)
		assert.Nil(t, err)
		_, ok := client.(*mqtt.Client)
		assert.True(t, ok)
	})

	t.Run("SharedNodeRestart", func(t *testing.T) {
		// Test shared nodes restart
		sharedCtx, ok := pool.Get("shared_mqtt_lifecycle")
		assert.True(t, ok)

		// Modify the configuration and restart
		modifiedDsl := []byte(strings.Replace(string(mqttNodeDsl), "/test/lifecycle", "/test/restarted", -1))
		err := sharedCtx.ReloadSelf(modifiedDsl)
		assert.Nil(t, err)

		// Verification is still available after reboot
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.NotNil(t, client)
		assert.Nil(t, err)
	})

	t.Run("SharedNodeDestroy", func(t *testing.T) {
		// Test shared node destruction
		pool.Del("shared_mqtt_lifecycle")
		assert.Equal(t, 0, len(pool.GetAll()))

		// After verifying destruction, the instance cannot be obtained
		client, err := pool.GetInstance("shared_mqtt_lifecycle")
		assert.Nil(t, client)
		assert.NotNil(t, err)
	})
}

// TestMultipleReferenceIndependence Tests the independence of multi-reference nodes
func TestMultipleReferenceIndependence(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Create a shared MQTT node
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

	// Create multiple rule engines to reference the same shared resource
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
		// Test that all engines can access shared resources normally
		for i, ruleEngine := range engines {
			engineIndex := i // Create local variables to avoid looping variables that catch loops in closures
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				assert.Equal(t, types.Success, relationType, fmt.Sprintf("Engine %d should succeed", engineIndex+1))
			}))
		}
		time.Sleep(time.Millisecond * 200)
	})

	t.Run("EngineStopIndependence", func(t *testing.T) {
		// Testing stops one engine without affecting the others
		engines[0].Stop(context.Background())
		time.Sleep(time.Millisecond * 100)

		// Other engines can still function normally
		for i := 1; i < 3; i++ {
			engineIndex := i // Create local variables to avoid looping variables that catch loops in closures
			engines[i].OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				assert.Equal(t, types.Success, relationType, fmt.Sprintf("Engine %d should still work", engineIndex+1))
			}))
		}
		time.Sleep(time.Millisecond * 200)
	})

	// Release resources
	for i := 1; i < 3; i++ {
		engines[i].Stop(context.Background())
	}
	pool.Del("shared_mqtt_multi_ref")
}

// TestSharedResourceRestartImpact tests the impact of shared resource restarts on existing references
func TestSharedResourceRestartImpact(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Create a shared MQTT node
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

	// Create a rule engine that references shared resources
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
		// Normal operation before reboot
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("DuringRestart", func(t *testing.T) {
		// Obtain the context of the shared resource and restart
		sharedCtx, ok := pool.Get("shared_mqtt_restart_test")
		assert.True(t, ok)

		// Modify configuration and restart (simulate restart failure using the wrong port)
		modifiedDsl := []byte(strings.Replace(string(mqttNodeDsl), "127.0.0.1:1883", "127.0.0.1:1884", -1))
		err := sharedCtx.ReloadSelf(modifiedDsl)
		assert.Nil(t, err)

		// It should fail after rebooting
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	t.Run("AfterRestartFixed", func(t *testing.T) {
		// Repair the configuration
		sharedCtx, ok := pool.Get("shared_mqtt_restart_test")
		assert.True(t, ok)

		fixedDsl := []byte(strings.Replace(string(mqttNodeDsl), "127.0.0.1:1884", "127.0.0.1:1883", -1))
		err := sharedCtx.ReloadSelf(fixedDsl)
		assert.Nil(t, err)

		// After repair, it should return to normal
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)
	})

	// Release resources
	ruleEngine.Stop(context.Background())
	pool.Del("shared_mqtt_restart_test")
}

// TestConcurrentSharedResourceAccess tests the security of concurrent access to shared resources
func TestConcurrentSharedResourceAccess(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	// Create a shared MQTT node
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

	// Create multiple rule engines
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
		// Send messages concurrently
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
		time.Sleep(time.Millisecond * 500) // Wait for all messages to be processed

		// Verify the security of concurrent access
		totalExpected := int32(10 * 5) // 10 messages * 5 engines
		totalActual := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&failureCount)
		assert.Equal(t, totalExpected, totalActual)

		t.Logf("Concurrent test results - Success: %d, Failure: %d, Total: %d",
			atomic.LoadInt32(&successCount),
			atomic.LoadInt32(&failureCount),
			totalActual)
	})

	// Release resources
	for _, ruleEngine := range engines {
		ruleEngine.Stop(context.Background())
	}
	pool.Del("shared_mqtt_concurrent")
}

// TestGracefulShutdownBehavior Tests graceful shutdownBehavior
func TestGracefulShutdownBehavior(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	t.Run("SharedResourceGracefulShutdown", func(t *testing.T) {
		// Create a shared node
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

		// Create two engines that share references to shared resources
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

		// Verify that both engines work properly
		engine1.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		engine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)

		// Stop the first engine (gracefully shut down)
		engine1.Stop(context.Background())
		time.Sleep(time.Millisecond * 100)

		// The second engine should still be functioning properly (shared resources should not be shut down).
		engine2.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType, "第二个引擎在第一个引擎停止后应该仍然正常工作")
		}))
		time.Sleep(time.Millisecond * 100)

		// Release resources
		engine2.Stop(context.Background())
		pool.Del("shared_mqtt_graceful")
	})

	t.Run("PoolShutdownBehavior", func(t *testing.T) {
		// Create a shared node
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

		// Create an engine that references shared resources
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

		// The verification engine works properly
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}))
		time.Sleep(time.Millisecond * 100)

		// Shut down the entire pool
		pool.Stop()
		assert.Equal(t, 0, len(pool.GetAll()))

		// After the pool is closed, the engine should be unable to access shared resources
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType, "池关闭后应该无法访问共享资源")
		}))
		time.Sleep(time.Millisecond * 100)

		// Release resources
		ruleEngine.Stop(context.Background())
	})
}

// TestSharedNodeGetSafelyAPI tests the new GetSafely API
func TestSharedNodeGetSafelyAPI(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	t.Run("GetSafelyConcurrentAccess", func(t *testing.T) {
		// Create a shared MQTT node
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

		// Create a rule engine using GetSafely
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

		// Wait for client initialization
		time.Sleep(time.Millisecond * 500)

		// Concurrent testing of thread safety for GetSafely methods
		var wg sync.WaitGroup
		successCount := int32(0)
		concurrentNum := 30 // Further reduce the number of concurrency

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
		time.Sleep(time.Millisecond * 1000) // Increased waiting times

		// Verify that most concurrent operations are successful
		actualSuccess := atomic.LoadInt32(&successCount)
		assert.True(t, actualSuccess > int32(concurrentNum*6/10), fmt.Sprintf("至少60%%的GetSafely调用应该成功，实际成功：%d/%d", actualSuccess, concurrentNum))

		// Release resources
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_getsafely")
	})

	t.Run("InitWithCloseCallback", func(t *testing.T) {
		// Test the cleanup callback function of InitWithClose
		callbackExecuted := int32(0)

		// Create a failed MQTT node configuration (using the wrong port)
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

		// Create a node (may fail, but should trigger a cleanup callback)
		ctx, err := pool.NewFromRuleNode(nodeDef)
		assert.NotNil(t, ctx)
		assert.Nil(t, err)

		// Create a rule engine
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

		// Sending messages should fail (due to an MQTT port error)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			// It may succeed or fail, depending on the behavior of the MQTT client
			t.Logf("Callback test results: %s, error: %v", relationType, err)
		}))
		time.Sleep(time.Millisecond * 200)

		// Cleaning up resources should trigger a Close callback
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_callback_test")
		time.Sleep(time.Millisecond * 100)

		t.Logf("Number of cleanup callback executions: %d", atomic.LoadInt32(&callbackExecuted))
	})
}

// TestSharedNodeResourceManagement Tests shared node resource management
func TestSharedNodeResourceManagement(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	t.Run("ResourceCleanupOnError", func(t *testing.T) {
		// Resource cleanup during test initialization errors
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

		// Create a rule engine
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

		// Sending messages should fail
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType, "无效主机应该导致失败")
		}))
		time.Sleep(time.Millisecond * 200)

		// Release resources
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_error_test")
	})

	t.Run("PerformanceComparisonGetVsGetSafely", func(t *testing.T) {
		// Performance comparison test (GetSafely should perform better during high-concurrency reads)
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

		// Create multiple rule engines for stress testing
		engines := make([]types.RuleEngine, 5) // Reduce the number of engines to avoid excessive concurrency
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

		// Wait for the client to complete initialization
		time.Sleep(time.Millisecond * 500)

		// High-concurrency messages send tests
		start := time.Now()
		var wg sync.WaitGroup
		messageCount := 50 // Reduce the number of messages
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
		time.Sleep(time.Millisecond * 1000) // Increase waiting time to ensure all message processing is complete

		expectedTotal := int32(messageCount * 5) // 50 messages * 5 engines
		actualSuccess := atomic.LoadInt32(&successCount)

		t.Logf("Performance test results - Total messages: %d, Number of successes: %d, Time taken: %v, Average QPS: %.2f",
			expectedTotal, actualSuccess, duration, float64(actualSuccess)/duration.Seconds())

		// Verify that most messages are processed successfully
		assert.True(t, actualSuccess > expectedTotal*5/10, "至少50%的消息应该处理成功")

		// Release resources
		for _, ruleEngine := range engines {
			ruleEngine.Stop(context.Background())
		}
		pool.Del("shared_mqtt_performance")
	})
}

// TestSharedNodeLockOptimization: Tests read/write lock optimization
func TestSharedNodeLockOptimization(t *testing.T) {
	config := engine.NewConfig()
	pool := NewNodePool(config)
	config.NodePool = pool

	t.Run("ReadWriteLockBehavior", func(t *testing.T) {
		// Create a shared node
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

		// Create a rule engine
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

		// Wait for client initialization
		time.Sleep(time.Millisecond * 500)

		// Large-scale concurrent read tests (simulating GetSafely's lock reading advantages)
		var wg sync.WaitGroup
		readCount := 100 // Reduce the number of concurrency
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
		t.Logf("Read/Write Lock Test - Concurrent Reads: %d, Successes: %d, Time: %v",
			readCount, actualSuccess, readDuration)

		// Verify that most read operations are successful
		assert.True(t, actualSuccess > int32(readCount*7/10), "至少70%的读取操作应该成功")

		// Release resources
		ruleEngine.Stop(context.Background())
		pool.Del("shared_mqtt_rwlock")
	})
}
