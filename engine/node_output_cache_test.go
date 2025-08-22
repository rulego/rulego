/*
 * Copyright 2025 The RuleGo Authors.
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
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
)

// TestCrossNodeAccess 测试跨节点取值功能
func TestCrossNodeAccess(t *testing.T) {
	t.Run("NodeOutputCacheConfig", testNodeOutputCacheConfig)
	t.Run("CrossNodeAccessDetection", testCrossNodeAccessDetection)
	t.Run("RuleChainCrossNodeAccess", testRuleChainCrossNodeAccess)
	t.Run("FunctionsCrossNodeAccess", testFunctionsCrossNodeAccess)
	t.Run("TemplateCrossNodeAccess", testTemplateCrossNodeAccess)
	t.Run("NodeOutputAccess", testNodeOutputAccess)

	// t.Run("RestApiCrossNodeAccess", testRestApiCrossNodeAccess)
}

// testNodeOutputCacheConfig 测试节点输出缓存配置功能
func testNodeOutputCacheConfig(t *testing.T) {
	// 测试默认情况下缓存禁用
	t.Run("DisabledByDefault", func(t *testing.T) {
		config := NewConfig()
		ruleCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
		cache := ruleCtx.GetNodeOutputCache()

		// 验证缓存为空
		assert.False(t, cache.HasOutputs(), "节点输出缓存应该是空的")

		// 尝试存储但不会存储
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"temperature":35}`)
		ruleCtx.StoreNodeOutput("node1", msg)
		assert.False(t, cache.HasOutputs(), "节点输出缓存应该仍然是空的")
	})

	// 测试通过跨节点访问启用缓存
	t.Run("EnabledByCrossNodeAccess", func(t *testing.T) {
		config := NewConfig()
		ruleCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
		cache := ruleCtx.GetNodeOutputCache()

		// 启用跨节点访问
		cache.EnableCrossNodeAccess()

		// 存储节点输出
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"temperature":35}`)
		ruleCtx.StoreNodeOutput("node1", msg)

		// 验证缓存有数据
		assert.True(t, cache.HasOutputs(), "启用跨节点访问后，缓存应该有数据")
	})
}

// DetectCrossNodeAccess 检测模板中是否包含跨节点访问
// DetectCrossNodeAccess detects if template contains cross-node access patterns
func DetectCrossNodeAccess(template string) bool {
	// 检测 ${nodeId.msg.xxx} 或 ${nodeId.metadata.xxx} 模式
	return strings.Contains(template, ".msg.") || strings.Contains(template, ".metadata.")
}

// testCrossNodeAccessDetection 测试跨节点取值检测
func testCrossNodeAccessDetection(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		expected bool
	}{
		{"Normal template", "Hello ${msg.name}", false},
		{"Cross node msg access", "Value: ${node1.msg.temperature}", true},
		{"Cross node metadata access", "Location: ${node1.metadata.location}", true},
		{"Mixed access", "Current: ${msg.status}, Node1: ${node1.msg.value}", true},
		{"No variables", "Static text", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectCrossNodeAccess(tc.template)
			assert.Equal(t, tc.expected, result, "Template: %s", tc.template)
		})
	}
}

// testRuleChainCrossNodeAccess 测试完整规则链中使用functions节点进行跨节点取值
// testRuleChainCrossNodeAccess tests cross-node access in complete rule chain using functions node
func testRuleChainCrossNodeAccess(t *testing.T) {
	// 注册测试函数
	registerCrossNodeTestFunctions()
	defer unregisterCrossNodeTestFunctions()

	// 定义规则链，使用functions节点并在配置中使用跨节点变量引用
	ruleChainFile := `{
		"ruleChain": {
			"id": "cross_node_test",
			"name": "跨节点取值测试",
			"debugMode": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "config_node",
					"type": "jsTransform",
					"name": "配置节点",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata['sensorFunc']='processSensorData'; metadata['aggregateFunc']='aggregateWithSensor'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "sensor_node",
					"type": "functions",
					"name": "传感器节点",
					"debugMode": true,
					"configuration": {
						"functionName": "${config_node.metadata.sensorFunc}"
					}
				},
				{
					"id": "aggregator_node",
					"type": "functions",
					"name": "聚合节点",
					"debugMode": true,
					"configuration": {
						"functionName": "${config_node.metadata.aggregateFunc}"
					}
				}
			],
			"connections": [
				{
					"fromId": "config_node",
					"toId": "sensor_node",
					"type": "Success"
				},
				{
					"fromId": "sensor_node",
					"toId": "aggregator_node",
					"type": "Success"
				}
			]
		}
	}`

	// 创建配置，启用节点输出缓存
	config := NewConfig()
	var aggregatorResult types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out && nodeId == "aggregator_node" {
			aggregatorResult = msg
			wg.Done()
		}
	}

	// 创建规则引擎
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 发送消息
	metaData := types.NewMetadata()
	metaData.PutValue("deviceId", "sensor001")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, `{"temperature":25.5,"humidity":60}`)

	ruleEngine.OnMsg(msg)

	// 等待处理完成
	wg.Wait()
	time.Sleep(time.Millisecond * 100)

	// 验证结果
	assert.Equal(t, "AGGREGATED", aggregatorResult.Type)
	assert.Equal(t, "sensor", aggregatorResult.Metadata.GetValue("nodeType"))

	// 验证跨节点取值成功
	data := aggregatorResult.GetData()
	assert.True(t, strings.Contains(data, "sensor"), "Expected data to contain 'sensor'")
	assert.True(t, strings.Contains(data, "temperature"), "Expected data to contain 'temperature'")
}

// testFunctionsCrossNodeAccess 测试使用 ${nodeId.msg.xx} 方式取值并通过 functions 节点动态调用函数
// testFunctionsCrossNodeAccess 测试 functions 节点的 functionName 配置是否支持跨节点变量引用
// testFunctionsCrossNodeAccess tests if functions node's functionName configuration supports cross-node variable references
func testFunctionsCrossNodeAccess(t *testing.T) {
	// 注册测试函数
	registerTestFunctions()
	defer unregisterTestFunctions()

	// 定义规则链，测试 functionName 通过 ${nodeId.x.xx} 方式动态获取函数名
	ruleChainFile := `{
		"ruleChain": {
			"id": "functions_dynamic_name_test",
			"name": "Functions动态函数名测试",
			"debugMode": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "config_node",
					"type": "jsTransform",
					"name": "配置节点",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata['targetFunction']='processData'; msg['nodeType']='config'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "dynamic_function_node",
					"type": "functions",
					"name": "动态函数节点",
					"debugMode": true,
					"configuration": {
						"functionName": "${config_node.metadata.targetFunction}"
					}
				}
			],
			"connections": [
				{
					"fromId": "config_node",
					"toId": "dynamic_function_node",
					"type": "Success"
				}
			]
		}
	}`

	// 创建配置，启用节点输出缓存以支持跨节点变量引用
	config := NewConfig()
	var functionResult types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out && nodeId == "dynamic_function_node" {
			functionResult = msg
			wg.Done()
		}

	}

	// 创建规则引擎
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 发送测试消息
	metaData := types.NewMetadata()
	metaData.PutValue("testCase", "dynamicFunctionName")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, `{"input":"test data","operation":"process"}`)

	ruleEngine.OnMsg(msg)

	// 等待处理完成
	wg.Wait()
	time.Sleep(time.Millisecond * 50)

	// 验证函数名动态解析成功
	assert.Equal(t, "PROCESSED", functionResult.Type, "Message type should be PROCESSED after function execution")
	assert.Equal(t, "processed", functionResult.Metadata.GetValue("status"), "Status should be 'processed'")

	// 验证数据被正确处理
	data := functionResult.GetData()
	assert.True(t, strings.Contains(data, "processed"), "Data should contain 'processed' indicating function was called")
	assert.True(t, strings.Contains(data, "input"), "Data should contain original 'input' field")
}

// registerTestFunctions 注册测试用的自定义函数
// registerTestFunctions registers custom functions for testing purposes.
func registerTestFunctions() {
	// 注册 processData 函数
	action.Functions.Register("processData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 处理数据并修改消息
		msg.Type = "PROCESSED"
		msg.Metadata.PutValue("status", "processed")

		// 修改消息数据
		data := msg.GetData()
		processedData := `{"processed":true,"original":` + data + `}`
		msg.Data = types.NewSharedData(processedData)

		ctx.TellSuccess(msg)
	})

	// 注册 validateData 函数
	action.Functions.Register("validateData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 验证数据
		msg.Type = "VALIDATED"
		msg.Metadata.PutValue("validation", "passed")
		ctx.TellSuccess(msg)
	})
}

// unregisterTestFunctions 注销测试用的自定义函数
// unregisterTestFunctions unregisters custom functions used for testing.
func unregisterTestFunctions() {
	action.Functions.UnRegister("processData")
	action.Functions.UnRegister("validateData")
}

// registerCrossNodeTestFunctions 注册跨节点取值测试用的自定义函数
// registerCrossNodeTestFunctions registers custom functions for cross-node access testing.
func registerCrossNodeTestFunctions() {
	// 注册 processNode1Data 函数
	action.Functions.Register("processNode1Data", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 处理node1数据并添加标识
		msg.Metadata.PutValue("nodeType", "node1")
		msg.Metadata.PutValue("status", "processed")
		ctx.TellSuccess(msg)
	})

	// 注册 aggregateWithNode1 函数
	action.Functions.Register("aggregateWithNode1", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 获取node1的输出数据
		node1Msg, found := ctx.GetNodeRuleMsg("node1")
		if found {
			// 聚合当前消息和node1的数据
			result := map[string]interface{}{
				"current":       msg.GetData(),
				"node1Data":     node1Msg.GetData(),
				"node1Metadata": node1Msg.Metadata.Values(),
			}
			resultData, _ := json.Marshal(result)
			msg.Data = types.NewSharedData(string(resultData))
		}
		msg.Type = "AGGREGATED"
		msg.Metadata.PutValue("status", "processed")
		ctx.TellSuccess(msg)
	})

	// 注册 processSensorData 函数
	action.Functions.Register("processSensorData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 处理传感器数据
		msg.Metadata.PutValue("nodeType", "sensor")
		// 在数据中添加传感器信息
		result := map[string]interface{}{
			"sensor":   "temperature",
			"value":    25.5,
			"original": msg.GetData(),
		}
		resultData, _ := json.Marshal(result)
		msg.Data = types.NewSharedData(string(resultData))
		ctx.TellSuccess(msg)
	})

	// 注册 aggregateWithSensor 函数
	action.Functions.Register("aggregateWithSensor", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 获取sensor_node的输出数据
		sensorMsg, found := ctx.GetNodeRuleMsg("sensor_node")
		if found {
			// 聚合当前消息和传感器数据
			result := map[string]interface{}{
				"current": msg.GetData(),
				"sensor":  sensorMsg.GetData(),
			}
			resultData, _ := json.Marshal(result)
			msg.Data = types.NewSharedData(string(resultData))
		}
		msg.Type = "AGGREGATED"
		ctx.TellSuccess(msg)
	})
}

// unregisterCrossNodeTestFunctions 注销跨节点取值测试用的自定义函数
// unregisterCrossNodeTestFunctions unregisters custom functions used for cross-node access testing.
func unregisterCrossNodeTestFunctions() {
	action.Functions.UnRegister("processNode1Data")
	action.Functions.UnRegister("aggregateWithNode1")
	action.Functions.UnRegister("processSensorData")
	action.Functions.UnRegister("aggregateWithSensor")
}

// testTemplateCrossNodeAccess 测试模板系统的跨节点取值功能
// testTemplateCrossNodeAccess tests cross-node access functionality in template system
func testTemplateCrossNodeAccess(t *testing.T) {
	// 创建配置，启用节点输出缓存
	config := NewConfig()
	ctx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)

	// 模拟节点输出数据
	node1Msg := types.NewMsg(0, "DATA", types.JSON, types.NewMetadata(), `{"temperature":25.5,"deviceId":"sensor001"}`)
	node1Msg.Metadata.PutValue("location", "room1")
	node1Msg.Metadata.PutValue("status", "active")

	node2Msg := types.NewMsg(0, "CONFIG", types.JSON, types.NewMetadata(), `{"threshold":30,"enabled":true}`)
	node2Msg.Metadata.PutValue("endpoint", "api.example.com")
	node2Msg.Metadata.PutValue("apiKey", "key123")

	// 添加节点为可缓存节点
	cache := ctx.GetNodeOutputCache()
	cache.AddCacheableNode("node1")
	cache.AddCacheableNode("node2")

	// 存储节点输出
	ctx.StoreNodeOutput("node1", node1Msg)
	ctx.StoreNodeOutput("node2", node2Msg)

	// 创建当前消息
	currentMsg := types.NewMsg(0, "PROCESS", types.JSON, types.NewMetadata(), `{"action":"validate"}`)
	currentMsg.Metadata.PutValue("requestId", "req001")

	// 测试GetEnv方法获取基础环境变量
	env := ctx.GetEnv(currentMsg, true)

	// 验证基础环境变量
	// 由于JSON字段处理被注释，验证msg字段包含解析后的JSON对象
	if msgMap, ok := env["msg"].(map[string]interface{}); ok {
		assert.Equal(t, "validate", msgMap["action"])
	} else {
		t.Errorf("Expected msg to be a map, got %T", env["msg"])
	}
	assert.Equal(t, "req001", env["requestId"])

	// 验证节点输出缓存功能
	node1Output, found := ctx.GetNodeRuleMsg("node1")
	assert.True(t, found, "Node1 output should be cached")
	if node1Output.Metadata != nil {
		assert.Equal(t, "room1", node1Output.Metadata.GetValue("location"))
	}

	node2Output, found := ctx.GetNodeRuleMsg("node2")
	assert.True(t, found, "Node2 output should be cached")
	if node2Output.Metadata != nil {
		assert.Equal(t, "api.example.com", node2Output.Metadata.GetValue("endpoint"))
	}

}

// testNodeOutputAccess 测试 nodeOutput 节点的跨节点取值功能
// testNodeOutputAccess tests nodeOutput node's cross-node access functionality
func testNodeOutputAccess(t *testing.T) {
	// 定义规则链，测试 nodeOutput 节点获取指定节点的输出
	ruleChainFile := `{
		"ruleChain": {
			"id": "hSyEEAxjvpdq",
			"name": "hSyEEAxjvpdq",
			"additionalInfo": {
				"noDefaultInput": false,
				"layoutX": "280",
				"layoutY": "280"
			}
		},
		"metadata": {
			"endpoints": [],
			"nodes": [
				{
					"id": "node_4",
					"type": "jsTransform",
					"name": "js转换1",
					"configuration": {
						"jsScript": "msg.from='aa'\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 495,
						"layoutY": 280
					}
				},
				{
					"id": "node_5",
					"type": "jsTransform",
					"name": "js转换2",
					"configuration": {
						"jsScript": "msg.from='bb'\nreturn {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 730,
						"layoutY": 274
					}
				},
				{
					"id": "node_7",
					"type": "fetchNodeOutput",
					"name": "取节点输出",
					"configuration": {
						"nodeId": "node_4"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 1033,
						"layoutY": 270
					}
				}
			],
			"connections": [
				{
					"fromId": "node_4",
					"toId": "node_5",
					"type": "Success"
				},
				{
					"fromId": "node_5",
					"toId": "node_7",
					"type": "Success"
				}
			]
		}
	}`

	// 创建配置，启用节点输出缓存以支持 nodeOutput 节点
	config := NewConfig()
	var nodeOutputResult types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)

	// 创建规则引擎
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 发送测试消息
	metaData := types.NewMetadata()
	metaData.PutValue("testCase", "nodeOutputAccess")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, `{"input":"test data","operation":"process"}`)

	// 使用WithEndFunc回调验证结果
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		nodeOutputResult = msg
		wg.Done()
		assert.Equal(t, types.Success, relationType)
	}))

	// 等待处理完成
	wg.Wait()
	time.Sleep(time.Millisecond * 50)

	// 验证 nodeOutput 节点成功获取了 node_4 的输出
	assert.Equal(t, "TEST_MSG", nodeOutputResult.Type, "Message type should be preserved from node_4")

	// 验证数据包含 node_4 的处理结果
	data := nodeOutputResult.GetData()
	assert.True(t, strings.Contains(data, "aa"), "Data should contain 'aa' from node_4")
	assert.True(t, strings.Contains(data, "from"), "Data should contain 'from' field from node_4")

	// 验证 nodeOutput 节点能够正确获取指定节点的输出数据
	assert.True(t, strings.Contains(data, "input"), "Data should contain original 'input' field")
	assert.True(t, strings.Contains(data, "test data"), "Data should contain original test data")
}
