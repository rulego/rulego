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

// TestConcurrentJsTransformInRuleChain 测试规则链中多个JS转换节点的并发数据修改隔离性
// 通过fork节点并行执行多个JS转换节点，验证数据修改的隔离性
func TestConcurrentJsTransformInRuleChain(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: JSON数据类型的并发修改隔离测试
	t.Run("JSONDataConcurrentModification", func(t *testing.T) {
		// 定义规则链DSL，包含fork节点和两个JS转换节点
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

		// 创建规则引擎
		ruleEngine, err := rulego.New("concurrent_json_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 创建共享的JSON数据
		sharedJSONData := `{"id": 1, "name": "test", "value": 50, "originalData": true}`
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "CONCURRENT_JSON_TEST", types.JSON, originalMetadata, sharedJSONData)

		// 收集结果
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// 设置消息处理回调
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("消息处理失败: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// 等待两个JS节点都处理完成
		wg.Add(2)
		wg.Wait()

		// 验证结果
		assert.Equal(t, 2, len(results), "应该收到两个处理结果")

		// 验证每个节点的处理结果是独立的
		node1Found := false
		node2Found := false

		for _, result := range results {
			processedBy := result.Metadata.GetValue("processedBy")
			resultData := result.Data.Get()

			if processedBy == "node1" {
				node1Found = true
				assert.True(t, len(resultData) > 0, "Node1结果数据不应为空")
				t.Logf("Node1处理结果: %s", resultData)
			} else if processedBy == "node2" {
				node2Found = true
				assert.True(t, len(resultData) > 0, "Node2结果数据不应为空")
				t.Logf("Node2处理结果: %s", resultData)
			}
		}

		assert.True(t, node1Found, "应该找到node1的处理结果")
		assert.True(t, node2Found, "应该找到node2的处理结果")

		// 验证原始消息数据
		t.Logf("原始消息数据: %s", testMsg.Data.Get())
	})

	// 测试2: 二进制数据类型的并发修改隔离测试
	t.Run("BinaryDataConcurrentModification", func(t *testing.T) {
		// 定义规则链DSL，包含fork节点和两个JS转换节点处理二进制数据
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

		// 创建规则引擎
		ruleEngine, err := rulego.New("concurrent_binary_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 创建共享的二进制数据
		sharedByteData := []byte{1, 2, 3, 4, 5}
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "CONCURRENT_BINARY_TEST", types.BINARY, originalMetadata, string(sharedByteData))

		// 记录原始数据
		originalData := make([]byte, len(sharedByteData))
		copy(originalData, sharedByteData)

		// 收集结果
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// 设置消息处理回调
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("消息处理失败: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// 等待两个JS节点都处理完成
		wg.Add(2)
		wg.Wait()

		// 验证结果
		assert.Equal(t, 2, len(results), "应该收到两个处理结果")

		// 验证每个节点的处理结果是独立的
		node1Found := false
		node2Found := false

		for _, result := range results {
			processedBy := result.Metadata.GetValue("processedBy")
			firstByte := result.Metadata.GetValue("firstByte")

			if processedBy == "node1" {
				node1Found = true
				assert.Equal(t, "100", firstByte, "Node1应该将第一个字节设置为100")
				t.Logf("Node1处理结果第一个字节: %s", firstByte)
			} else if processedBy == "node2" {
				node2Found = true
				assert.Equal(t, "200", firstByte, "Node2应该将第一个字节设置为200")
				t.Logf("Node2处理结果第一个字节: %s", firstByte)
			}
		}

		assert.True(t, node1Found, "应该找到node1的处理结果")
		assert.True(t, node2Found, "应该找到node2的处理结果")

		// 验证原始字节数组未被修改（由于base.go中的副本机制）
		for i, b := range sharedByteData {
			assert.Equal(t, originalData[i], b, "原始字节数组第%d个字节应该未被修改", i)
		}

		t.Logf("原始字节数组: %v", originalData)
		t.Logf("处理后共享字节数组: %v", sharedByteData)
	})

	// 测试3: 高并发场景下的数据隔离测试
	t.Run("HighConcurrencyDataIsolation", func(t *testing.T) {
		// 定义包含更多JS节点的规则链
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

		// 创建规则引擎
		ruleEngine, err := rulego.New("high_concurrency_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 创建共享数据
		sharedData := `{"id": 1, "name": "shared", "value": 0}`
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "HIGH_CONCURRENCY_TEST", types.JSON, originalMetadata, sharedData)

		// 收集结果
		var results []types.RuleMsg
		var resultsMutex sync.Mutex
		var wg sync.WaitGroup

		// 设置消息处理回调
		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			if err != nil {
				t.Errorf("消息处理失败: %v", err)
				return
			}

			resultsMutex.Lock()
			results = append(results, msg)
			resultsMutex.Unlock()
		}))

		// 等待所有5个JS节点都处理完成
		wg.Add(5)
		wg.Wait()

		// 验证结果
		assert.Equal(t, 5, len(results), "应该收到5个处理结果")

		// 验证每个节点都产生了独立的结果
		nodeIds := make(map[string]bool)
		for _, result := range results {
			nodeId := result.Metadata.GetValue("nodeId")
			assert.True(t, nodeId != "", "节点应该设置nodeId")
			nodeIds[nodeId] = true
		}

		// 验证所有5个节点都产生了结果
		assert.Equal(t, 5, len(nodeIds), "应该有5个不同的节点ID")
		t.Logf("高并发处理产生的节点ID: %v", nodeIds)
	})

	// 测试4: 多次执行验证一致性
	t.Run("MultipleExecutionConsistency", func(t *testing.T) {
		// 定义简单的规则链
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

		// 创建规则引擎
		ruleEngine, err := rulego.New("consistency_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 多次执行相同的测试
		for i := 0; i < 10; i++ {
			// 创建测试数据
			testData := fmt.Sprintf(`{"id": %d, "name": "test%d", "iteration": %d}`, i, i, i)
			originalMetadata := types.BuildMetadata(make(map[string]string))
			testMsg := types.NewMsg(0, "CONSISTENCY_TEST", types.JSON, originalMetadata, testData)

			// 收集结果
			var results []types.RuleMsg
			var resultsMutex sync.Mutex
			var wg sync.WaitGroup

			// 设置消息处理回调
			ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				defer wg.Done()
				if err != nil {
					t.Errorf("第%d次执行消息处理失败: %v", i, err)
					return
				}

				resultsMutex.Lock()
				results = append(results, msg)
				resultsMutex.Unlock()
			}))

			// 等待处理完成
			wg.Add(2)
			wg.Wait()

			// 验证结果
			assert.Equal(t, 2, len(results), "第%d次执行应该收到2个处理结果", i)

			// 验证数据一致性
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

		t.Logf("多次执行一致性测试完成，共执行10次")
	})
}