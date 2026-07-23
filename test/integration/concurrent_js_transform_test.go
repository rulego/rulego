package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// TestConcurrentJsTransformInRuleChain tests the isolation of concurrent data modifications for multiple JS conversion nodes in the rule chain
// Multiple JS conversion nodes are executed in parallel through fork nodes to verify data modification isolation
func TestConcurrentJsTransformInRuleChain(t *testing.T) {
	config := rulego.NewConfig()

	// Test 1: Concurrent modification isolation test for JSON data types
	t.Run("JSONDataConcurrentModification", func(t *testing.T) {
		// Define the rule chain DSL, including fork nodes and two JS conversion nodes
		ruleChainDSL := `{
			"ruleChain": {
				"id": "concurrent_json_test",
				"name": "并发JSON测试",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"type": "fork",
						"id": "fork_node",
						"name": "并行网关"
					},
					{
						"id": "js_node_1",
						"type": "jsTransform",
						"name": "js转换1",
						"configuration": {
							"jsScript": "msg.modifiedBy='node1'; msg.node1Value=100; msg.node1Timestamp=new Date().getTime(); metadata['processedBy']='node1'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_2",
						"type": "jsTransform",
						"name": "js转换2",
						"configuration": {
							"jsScript": "msg.modifiedBy='node2'; msg.node2Value=200; msg.node2Timestamp=new Date().getTime(); metadata['processedBy']='node2'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					}
				],
				"connections": [
					{
						"fromId": "fork_node",
						"toId": "js_node_1",
						"type": "Success"
					},
					{
						"fromId": "fork_node",
						"toId": "js_node_2",
						"type": "Success"
					}
				]
			}
		}`

		// Create a rule engine
		ruleEngine, err := rulego.New("concurrent_json_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Create shared JSON data
		sharedJSONData := `{"id": 1, "name": "test", "value": 50, "originalData": true}`
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "CONCURRENT_JSON_TEST", types.JSON, originalMetadata, sharedJSONData)

		// Collect the results
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// Set message handling callbacks
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("Message processing failure: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// Wait for both JS nodes to finish processing
		wg.Add(2)
		wg.Wait()

		// Verify the results
		assert.Equal(t, 2, len(results), "应该收到两个处理结果")

		// Verify that each node's processing result is independent
		node1Found := false
		node2Found := false

		for _, result := range results {
			processedBy := result.Metadata.GetValue("processedBy")
			resultData := result.Data.Get()

			if processedBy == "node1" {
				node1Found = true
				assert.True(t, len(resultData) > 0, "Node1结果数据不应为空")
				t.Logf("Node1 Result: %s", resultData)
			} else if processedBy == "node2" {
				node2Found = true
				assert.True(t, len(resultData) > 0, "Node2结果数据不应为空")
				t.Logf("Node2 Result: %s", resultData)
			}
		}

		assert.True(t, node1Found, "应该找到node1的处理结果")
		assert.True(t, node2Found, "应该找到node2的处理结果")

		// Verify the original message data
		t.Logf("Original message data: %s", testMsg.Data.Get())
	})

	// Test 2: Concurrent modification isolation testing for binary data types
	t.Run("BinaryDataConcurrentModification", func(t *testing.T) {
		// Define the Rule Chain DSL, including fork nodes and two JS conversion nodes to process binary data
		ruleChainDSL := `{
			"ruleChain": {
				"id": "concurrent_binary_test",
				"name": "并发二进制测试",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"type": "fork",
						"id": "fork_node",
						"name": "并行网关"
					},
					{
						"id": "js_node_1",
						"type": "jsTransform",
						"name": "js转换1",
						"configuration": {
							"jsScript": "if (msg && typeof msg === 'object' && msg.length !== undefined) { msg[0] = 100; msg[1] = 101; } metadata['processedBy']='node1'; metadata['firstByte']=msg[0] ? msg[0].toString() : 'undefined'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_2",
						"type": "jsTransform",
						"name": "js转换2",
						"configuration": {
							"jsScript": "if (msg && typeof msg === 'object' && msg.length !== undefined) { msg[0] = 200; msg[1] = 201; } metadata['processedBy']='node2'; metadata['firstByte']=msg[0] ? msg[0].toString() : 'undefined'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					}
				],
				"connections": [
					{
						"fromId": "fork_node",
						"toId": "js_node_1",
						"type": "Success"
					},
					{
						"fromId": "fork_node",
						"toId": "js_node_2",
						"type": "Success"
					}
				]
			}
		}`

		// Create a rule engine
		ruleEngine, err := rulego.New("concurrent_binary_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Create shared binary data
		sharedByteData := []byte{1, 2, 3, 4, 5}
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "CONCURRENT_BINARY_TEST", types.BINARY, originalMetadata, string(sharedByteData))

		// Record the raw data
		originalData := make([]byte, len(sharedByteData))
		copy(originalData, sharedByteData)

		// Collect the results
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// Set message handling callbacks
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("Message processing failure: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// Wait for both JS nodes to finish processing
		wg.Add(2)
		wg.Wait()

		// Verify the results
		assert.Equal(t, 2, len(results), "应该收到两个处理结果")

		// Verify that each node's processing result is independent
		node1Found := false
		node2Found := false

		for _, result := range results {
			processedBy := result.Metadata.GetValue("processedBy")
			firstByte := result.Metadata.GetValue("firstByte")

			if processedBy == "node1" {
				node1Found = true
				assert.Equal(t, "100", firstByte, "Node1应该将第一个字节设置为100")
				t.Logf("Node1 first byte of processing result: %s", firstByte)
			} else if processedBy == "node2" {
				node2Found = true
				assert.Equal(t, "200", firstByte, "Node2应该将第一个字节设置为200")
				t.Logf("Node2 first byte of processing result: %s", firstByte)
			}
		}

		assert.True(t, node1Found, "应该找到node1的处理结果")
		assert.True(t, node2Found, "应该找到node2的处理结果")

		// Verify that the original byte array has not been modified (due to the replication mechanism in base.go)
		for i, b := range sharedByteData {
			assert.Equal(t, originalData[i], b, "原始字节数组第%d个字节应该未被修改", i)
		}

		t.Logf("Original byte array: %v", originalData)
		t.Logf("Processed byte array: %v", sharedByteData)
	})

	// Test 3: Data isolation testing in high-concurrency scenarios
	t.Run("HighConcurrencyDataIsolation", func(t *testing.T) {
		// Define a rule chain that contains more JS nodes
		ruleChainDSL := `{
			"ruleChain": {
				"id": "high_concurrency_test",
				"name": "高并发测试",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"type": "fork",
						"id": "fork_node",
						"name": "并行网关"
					},
					{
						"id": "js_node_1",
						"type": "jsTransform",
						"name": "js转换1",
						"configuration": {
							"jsScript": "msg.nodeId=1; msg.timestamp=new Date().getTime(); msg.randomValue=Math.floor(Math.random()*1000); metadata['nodeId']='1'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_2",
						"type": "jsTransform",
						"name": "js转换2",
						"configuration": {
							"jsScript": "msg.nodeId=2; msg.timestamp=new Date().getTime(); msg.randomValue=Math.floor(Math.random()*1000); metadata['nodeId']='2'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_3",
						"type": "jsTransform",
						"name": "js转换3",
						"configuration": {
							"jsScript": "msg.nodeId=3; msg.timestamp=new Date().getTime(); msg.randomValue=Math.floor(Math.random()*1000); metadata['nodeId']='3'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_4",
						"type": "jsTransform",
						"name": "js转换4",
						"configuration": {
							"jsScript": "msg.nodeId=4; msg.timestamp=new Date().getTime(); msg.randomValue=Math.floor(Math.random()*1000); metadata['nodeId']='4'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_5",
						"type": "jsTransform",
						"name": "js转换5",
						"configuration": {
							"jsScript": "msg.nodeId=5; msg.timestamp=new Date().getTime(); msg.randomValue=Math.floor(Math.random()*1000); metadata['nodeId']='5'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					}
				],
				"connections": [
					{"fromId": "fork_node", "toId": "js_node_1", "type": "Success"},
					{"fromId": "fork_node", "toId": "js_node_2", "type": "Success"},
					{"fromId": "fork_node", "toId": "js_node_3", "type": "Success"},
					{"fromId": "fork_node", "toId": "js_node_4", "type": "Success"},
					{"fromId": "fork_node", "toId": "js_node_5", "type": "Success"}
				]
			}
		}`

		// Create a rule engine
		ruleEngine, err := rulego.New("high_concurrency_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Create shared data
		sharedData := `{"id": 1, "name": "shared", "value": 0}`
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "HIGH_CONCURRENCY_TEST", types.JSON, originalMetadata, sharedData)

		// Collect the results
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// Set message handling callbacks
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("Message processing failure: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// Wait for all five JS nodes to finish processing
		wg.Add(5)
		wg.Wait()

		// Verify the results
		assert.Equal(t, 5, len(results), "应该收到5个处理结果")

		// Each node was verified to produce an independent result
		nodeIds := make(map[string]bool)
		for _, result := range results {
			nodeId := result.Metadata.GetValue("nodeId")
			assert.True(t, nodeId != "", "节点应该设置nodeId")
			nodeIds[nodeId] = true
		}

		// All five nodes were validated and produced results
		assert.Equal(t, 5, len(nodeIds), "应该有5个不同的节点ID")
		t.Logf("Node ID: %v generated by high concurrency processing", nodeIds)
	})

	// Test 4: Multiple runs to verify consistency
	t.Run("MultipleExecutionConsistency", func(t *testing.T) {
		// Define a simple chain of rules
		ruleChainDSL := `{
			"ruleChain": {
				"id": "consistency_test",
				"name": "一致性测试",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"type": "fork",
						"id": "fork_node",
						"name": "并行网关"
					},
					{
						"id": "js_node_1",
						"type": "jsTransform",
						"name": "js转换1",
						"configuration": {
							"jsScript": "msg.from='aa'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "js_node_2",
						"type": "jsTransform",
						"name": "js转换2",
						"configuration": {
							"jsScript": "msg.from='bb'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					}
				],
				"connections": [
					{"fromId": "fork_node", "toId": "js_node_1", "type": "Success"},
					{"fromId": "fork_node", "toId": "js_node_2", "type": "Success"}
				]
			}
		}`

		// Create a rule engine
		ruleEngine, err := rulego.New("consistency_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// The same test was performed multiple times
		for i := 0; i < 10; i++ {
			// Create test data
			testData := fmt.Sprintf(`{"id": %d, "name": "test%d", "iteration": %d}`, i, i, i)
			originalMetadata := types.BuildMetadata(make(map[string]string))
			testMsg := types.NewMsg(0, "CONSISTENCY_TEST", types.JSON, originalMetadata, testData)

			// Collect the results
			var results []types.RuleMsg
			var resultsMutex sync.Mutex
			var wg sync.WaitGroup

			// Add first, then OnMsg to avoid OnEnd callbacks executing before Add and causing WaitGroup counts to become negative
			wg.Add(2)
			// Set message handling callbacks
			ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				defer wg.Done()
				if err != nil {
					t.Errorf("%d message processing failure: %v", i, err)
					return
				}

				resultsMutex.Lock()
				results = append(results, msg)
				resultsMutex.Unlock()
			}))

			// Wait for processing to complete
			wg.Wait()

			// Verify the results
			assert.Equal(t, 2, len(results), "第%d次执行应该收到2个处理结果", i)

			// Verify data consistency
			aaFound := false
			bbFound := false
			for _, result := range results {
				resultData := result.Data.Get()
				if fmt.Sprintf(`"from":"aa"`) != "" && len(resultData) > 0 {
					var jsonData map[string]interface{}
					err := json.Unmarshal([]byte(resultData), &jsonData)
					if err == nil {
						if from, ok := jsonData["from"]; ok {
							if from == "aa" {
								aaFound = true
							} else if from == "bb" {
								bbFound = true
							}
						}
					}
				}
			}

			assert.True(t, aaFound, "第%d次执行应该找到from='aa'的结果", i)
			assert.True(t, bbFound, "第%d次执行应该找到from='bb'的结果", i)
		}

		t.Logf("Multiple conformity tests were completed, totaling 10 runs")
	})
}
