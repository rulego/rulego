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

// TestCrossNodeAccess tests the value retrieval function across nodes
func TestCrossNodeAccess(t *testing.T) {
	t.Run("NodeOutputCacheConfig", testNodeOutputCacheConfig)
	t.Run("CrossNodeAccessDetection", testCrossNodeAccessDetection)
	t.Run("RuleChainCrossNodeAccess", testRuleChainCrossNodeAccess)
	t.Run("FunctionsCrossNodeAccess", testFunctionsCrossNodeAccess)
	t.Run("TemplateCrossNodeAccess", testTemplateCrossNodeAccess)
	t.Run("NodeOutputAccess", testNodeOutputAccess)

	// t.Run("RestApiCrossNodeAccess", testRestApiCrossNodeAccess)
}

// testNodeOutputCacheConfig Tests node output cache configuration function
func testNodeOutputCacheConfig(t *testing.T) {
	// Testing with cache disabled by default
	t.Run("DisabledByDefault", func(t *testing.T) {
		config := NewConfig()
		ruleCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
		cache := ruleCtx.GetNodeOutputCache()

		// Verify that the cache is empty
		assert.False(t, cache.HasOutputs(), "节点输出缓存应该是空的")

		// Trying to store but not working
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"temperature":35}`)
		ruleCtx.StoreNodeOutput("node1", msg)
		assert.False(t, cache.HasOutputs(), "节点输出缓存应该仍然是空的")
	})

	// Testing enabled caching through cross-node access
	t.Run("EnabledByCrossNodeAccess", func(t *testing.T) {
		config := NewConfig()
		ruleCtx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)
		cache := ruleCtx.GetNodeOutputCache()

		// Enable cross-node access
		cache.EnableCrossNodeAccess()

		// Storage node output
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"temperature":35}`)
		ruleCtx.StoreNodeOutput("node1", msg)

		// Verify that cache data is present
		assert.True(t, cache.HasOutputs(), "启用跨节点访问后，缓存应该有数据")
	})
}

// DetectCrossNodeAccess detects whether the template includes cross-node access
// DetectCrossNodeAccess detects if template contains cross-node access patterns
func DetectCrossNodeAccess(template string) bool {
	// Detects ${nodeId.msg.xxx} or ${nodeId.metadata.xxx} patterns
	return strings.Contains(template, ".msg.") || strings.Contains(template, ".metadata.")
}

// testCrossNodeAccessDetection tests cross-node value detection
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

// testRuleChainCrossNodeAccess tests using functions nodes in the complete rule chain to fetch values across nodes
// testRuleChainCrossNodeAccess tests cross-node access in complete rule chain using functions node
func testRuleChainCrossNodeAccess(t *testing.T) {
	// Register the test function
	registerCrossNodeTestFunctions()
	defer unregisterCrossNodeTestFunctions()

	// Define the rule chain, use functions nodes, and configure cross-node variable references
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

	// Create a configuration and enable node output cache
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

	// Create a rule engine
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Send the message
	metaData := types.NewMetadata()
	metaData.PutValue("deviceId", "sensor001")
	msg := types.NewMsg(0, "TELEMETRY_MSG", types.JSON, metaData, `{"temperature":25.5,"humidity":60}`)

	ruleEngine.OnMsg(msg)

	// Wait for processing to complete
	wg.Wait()
	time.Sleep(time.Millisecond * 100)

	// Verify the results
	assert.Equal(t, "AGGREGATED", aggregatorResult.Type)
	assert.Equal(t, "sensor", aggregatorResult.Metadata.GetValue("nodeType"))

	// Verify successful cross-node values
	data := aggregatorResult.GetData()
	assert.True(t, strings.Contains(data, "sensor"), "Expected data to contain 'sensor'")
	assert.True(t, strings.Contains(data, "temperature"), "Expected data to contain 'temperature'")
}

// testFunctionsCrossNodeAccess tests using ${nodeId.msg.xx} to take values and dynamically call functions through the Functions node
// testFunctionsCrossNodeAccess tests whether the functionName configuration of a functions node supports cross-node variable references
// testFunctionsCrossNodeAccess tests if functions node's functionName configuration supports cross-node variable references
func testFunctionsCrossNodeAccess(t *testing.T) {
	// Register the test function
	registerTestFunctions()
	defer unregisterTestFunctions()

	// Define the rule chain and test the functionName. Dynamically obtain the function name using ${nodeId.x.xx}
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

	// Create a configuration to enable node output caching to support cross-node variable references
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

	// Create a rule engine
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Send test messages
	metaData := types.NewMetadata()
	metaData.PutValue("testCase", "dynamicFunctionName")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, `{"input":"test data","operation":"process"}`)

	ruleEngine.OnMsg(msg)

	// Wait for processing to complete
	wg.Wait()
	time.Sleep(time.Millisecond * 50)

	// Verify that the function name is successfully resolved dynamically
	assert.Equal(t, "PROCESSED", functionResult.Type, "Message type should be PROCESSED after function execution")
	assert.Equal(t, "processed", functionResult.Metadata.GetValue("status"), "Status should be 'processed'")

	// Verify that data is processed correctly
	data := functionResult.GetData()
	assert.True(t, strings.Contains(data, "processed"), "Data should contain 'processed' indicating function was called")
	assert.True(t, strings.Contains(data, "input"), "Data should contain original 'input' field")
}

// registerTestFunctions: A custom function used for registering tests
// registerTestFunctions registers custom functions for testing purposes.
func registerTestFunctions() {
	// Register the processData function
	action.Functions.Register("processData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Processing data and modifying messages
		msg.Type = "PROCESSED"
		msg.Metadata.PutValue("status", "processed")

		// Modify message data
		data := msg.GetData()
		processedData := `{"processed":true,"original":` + data + `}`
		msg.Data = types.NewSharedData(processedData)

		ctx.TellSuccess(msg)
	})

	// Register the validateData function
	action.Functions.Register("validateData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Verify data
		msg.Type = "VALIDATED"
		msg.Metadata.PutValue("validation", "passed")
		ctx.TellSuccess(msg)
	})
}

// unregisterTestFunctions: A custom function used to log out of testing
// unregisterTestFunctions unregisters custom functions used for testing.
func unregisterTestFunctions() {
	action.Functions.UnRegister("processData")
	action.Functions.UnRegister("validateData")
}

// registerCrossNodeTestFunctions Custom functions for registering cross-node value testing
// registerCrossNodeTestFunctions registers custom functions for cross-node access testing.
func registerCrossNodeTestFunctions() {
	// Register the processNode1Data function
	action.Functions.Register("processNode1Data", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Process node1 data and add identifiers
		msg.Metadata.PutValue("nodeType", "node1")
		msg.Metadata.PutValue("status", "processed")
		ctx.TellSuccess(msg)
	})

	// Register the aggregateWithNode1 function
	action.Functions.Register("aggregateWithNode1", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Obtain the output data of node1
		node1Msg, found := ctx.GetNodeRuleMsg("node1")
		if found {
			// Aggregate current messages and node1 data
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

	// Register the processSensorData function
	action.Functions.Register("processSensorData", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Processing sensor data
		msg.Metadata.PutValue("nodeType", "sensor")
		// Add sensor information to the data
		result := map[string]interface{}{
			"sensor":   "temperature",
			"value":    25.5,
			"original": msg.GetData(),
		}
		resultData, _ := json.Marshal(result)
		msg.Data = types.NewSharedData(string(resultData))
		ctx.TellSuccess(msg)
	})

	// Register the aggregateWithSensor function
	action.Functions.Register("aggregateWithSensor", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Obtain the output data of the sensor_node
		sensorMsg, found := ctx.GetNodeRuleMsg("sensor_node")
		if found {
			// Aggregate current news and sensor data
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

// unregisterCrossNodeTestFunctions Delete a custom function used for cross-node value testing
// unregisterCrossNodeTestFunctions unregisters custom functions used for cross-node access testing.
func unregisterCrossNodeTestFunctions() {
	action.Functions.UnRegister("processNode1Data")
	action.Functions.UnRegister("aggregateWithNode1")
	action.Functions.UnRegister("processSensorData")
	action.Functions.UnRegister("aggregateWithSensor")
}

// testTemplateCrossNodeAccess is a cross-node value retrieval function for test template systems
// testTemplateCrossNodeAccess tests cross-node access functionality in template system
func testTemplateCrossNodeAccess(t *testing.T) {
	// Create a configuration and enable node output cache
	config := NewConfig()
	ctx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, nil, nil)

	// Simulated node outputs data
	node1Msg := types.NewMsg(0, "DATA", types.JSON, types.NewMetadata(), `{"temperature":25.5,"deviceId":"sensor001"}`)
	node1Msg.Metadata.PutValue("location", "room1")
	node1Msg.Metadata.PutValue("status", "active")

	node2Msg := types.NewMsg(0, "CONFIG", types.JSON, types.NewMetadata(), `{"threshold":30,"enabled":true}`)
	node2Msg.Metadata.PutValue("endpoint", "api.example.com")
	node2Msg.Metadata.PutValue("apiKey", "key123")

	// Add nodes as cacheable nodes
	cache := ctx.GetNodeOutputCache()
	cache.AddCacheableNode("node1")
	cache.AddCacheableNode("node2")

	// Storage node output
	ctx.StoreNodeOutput("node1", node1Msg)
	ctx.StoreNodeOutput("node2", node2Msg)

	// Create the current message
	currentMsg := types.NewMsg(0, "PROCESS", types.JSON, types.NewMetadata(), `{"action":"validate"}`)
	currentMsg.Metadata.PutValue("requestId", "req001")

	// Test the GetEnv method to obtain the base environment variables
	env := ctx.GetEnv(currentMsg, true)

	// Verify the underlying environment variables
	// Since JSON field processing is commented, verify that the msg field contains the parsed JSON object
	if msgMap, ok := env["msg"].(map[string]interface{}); ok {
		assert.Equal(t, "validate", msgMap["action"])
	} else {
		t.Errorf("Expected msg to be a map, got %T", env["msg"])
	}
	assert.Equal(t, "req001", env["requestId"])

	// Verify node output cache functionality
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

// testNodeOutputAccess tests the cross-node value retrieval function of nodeOutput
// testNodeOutputAccess tests nodeOutput node's cross-node access functionality
func testNodeOutputAccess(t *testing.T) {
	// Define the rule chain and test nodeOutput. The nodeOutput node gets the output of the specified node
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

	// Create a configuration to enable node output caching to support nodeOutput nodes
	config := NewConfig()
	var nodeOutputResult types.RuleMsg
	var wg sync.WaitGroup
	wg.Add(1)

	// Create a rule engine
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Send test messages
	metaData := types.NewMetadata()
	metaData.PutValue("testCase", "nodeOutputAccess")
	msg := types.NewMsg(0, "TEST_MSG", types.JSON, metaData, `{"input":"test data","operation":"process"}`)

	// Use WithEndFunc callbacks to verify results
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		nodeOutputResult = msg
		wg.Done()
		assert.Equal(t, types.Success, relationType)
	}))

	// Wait for processing to complete
	wg.Wait()
	time.Sleep(time.Millisecond * 50)

	// Verify nodeOutput: The node has successfully obtained the output of node_4
	assert.Equal(t, "TEST_MSG", nodeOutputResult.Type, "Message type should be preserved from node_4")

	// Verify that the data contains the processing results of node_4
	data := nodeOutputResult.GetData()
	assert.True(t, strings.Contains(data, "aa"), "Data should contain 'aa' from node_4")
	assert.True(t, strings.Contains(data, "from"), "Data should contain 'from' field from node_4")

	// Verify that nodeOutput node correctly retrieves the output data of the specified node
	assert.True(t, strings.Contains(data, "input"), "Data should contain original 'input' field")
	assert.True(t, strings.Contains(data, "test data"), "Data should contain original test data")
}
