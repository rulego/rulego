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
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"

	"github.com/rulego/rulego/api/types"
)

// TestForNodeConcurrentMetadataAccess 测试for节点在并发场景下的元数据读写安全性
// 这个测试专门检查for节点处理元数据时是否存在并发读写问题
func TestForNodeConcurrentMetadataAccess(t *testing.T) {
	// 创建包含for节点的规则链
	forNodeRuleChain := `{
		"ruleChain": {
			"id": "test_for_concurrent",
			"name": "testForConcurrent",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "for_node",
					"type": "for",
					"name": "循环节点",
					"configuration": {
						"range": "msg.items",
						"do": "process_item",
						"mode": 1
					}
				},
				{
					"id": "process_item",
					"type": "jsTransform",
					"name": "处理项目",
					"configuration": {
						"jsScript": "metadata['processed_' + metadata._loopIndex] = 'item_' + metadata._loopItem; metadata['timestamp'] = Date.now(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "for_node",
					"toId": "process_item",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_for_concurrent", []byte(forNodeRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 并发测试参数
	concurrentCount := 50
	itemsPerMessage := 10
	var successCount int64
	var errorCount int64

	// 用于同步等待所有消息处理完成
	done := make(chan bool, 1)

	// 启动多个goroutine并发发送消息
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// 创建包含数组的消息
			items := make([]interface{}, itemsPerMessage)
			for j := 0; j < itemsPerMessage; j++ {
				items[j] = fmt.Sprintf("item_%d_%d", index, j)
			}

			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))

			itemsJSON, _ := json.Marshal(items)
			msg := types.NewMsg(0, "TEST_FOR_CONCURRENT", types.JSON, metaData, fmt.Sprintf(`{"items": %s, "batch_id": %d}`, itemsJSON, index))

			// 发送消息并等待处理完成
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
				if atomic.LoadInt64(&successCount)+atomic.LoadInt64(&errorCount) == int64(concurrentCount) {
					done <- true
				}
			}))
		}(i)
	}

	// 等待所有消息处理完成
	select {
	case <-done:
		// 所有消息处理完成
	case <-time.After(10 * time.Second):
		t.Fatal("测试超时")
	}

	// 验证结果
	if successCount != int64(concurrentCount) {
		t.Errorf("期望处理 %d 条消息，实际处理 %d 条", concurrentCount, successCount)
	}
	if errorCount != 0 {
		t.Errorf("期望0个错误，实际有 %d 个错误", errorCount)
	}

}

// TestForNodeMetadataRaceCondition 测试for节点元数据的竞态条件
// 这个测试专门检查在高并发情况下是否会出现数据竞争
func TestForNodeMetadataRaceCondition(t *testing.T) {
	// 创建一个更复杂的规则链，包含多个节点来增加竞态条件的可能性
	raceTestRuleChain := `{
		"ruleChain": {
			"id": "test_race_condition",
			"name": "testRaceCondition",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "for_node",
					"type": "for",
					"name": "循环节点",
					"configuration": {
						"range": "1..100",
						"do": "concurrent_processor",
						"mode": 3
					}
				},
				{
					"id": "concurrent_processor",
					"type": "jsTransform",
					"name": "并发处理器",
					"configuration": {
						"jsScript": "var key = 'race_test_' + metadata._loopIndex; metadata[key] = metadata._loopItem + '_processed'; metadata['global_counter'] = (metadata['global_counter'] || 0) + 1; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "for_node",
					"toId": "concurrent_processor",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_race_condition", []byte(raceTestRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 高并发测试
	concurrentCount := 100
	var wg sync.WaitGroup
	var processedCount int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动多个goroutine同时发送消息
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			metaData := types.NewMetadata()
			metaData.PutValue("test_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))

			msg := types.NewMsg(0, "RACE_TEST", types.JSON, metaData, `{"test": "data"}`)

			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					t.Errorf("处理消息时出错: %v", err)
				} else {
					atomic.AddInt64(&processedCount, 1)
				}
			}))
		}(i)
	}

	// 等待所有goroutine完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:

	case <-ctx.Done():
		currentProcessed := atomic.LoadInt64(&processedCount)
		t.Errorf("测试超时: 只处理了 %d 条消息", currentProcessed)
	}

	// 验证至少处理了一些消息
	finalProcessedCount := atomic.LoadInt64(&processedCount)
	if finalProcessedCount <= 0 {
		t.Errorf("应该至少处理一些消息，实际处理: %d", finalProcessedCount)
	}
}

// TestForNodeConcurrentWithFork 测试使用fork节点并发执行多个for节点的场景
// 这个测试验证fork节点能够正确地将消息分发到多个for节点并行处理
func TestForNodeConcurrentWithFork(t *testing.T) {
	// 创建包含fork节点和多个for节点的规则链
	forkForRuleChain := `{
		"ruleChain": {
			"id": "test_fork_for_concurrent",
			"name": "testForkForConcurrent",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "fork_start",
					"type": "fork",
					"name": "并行网关"
				},
				{
					"id": "for_node_1",
					"type": "for",
					"name": "循环节点1",
					"configuration": {
						"range": "msg.items1",
						"do": "process_item_1",
						"mode": 1
					}
				},
				{
					"id": "for_node_2",
					"type": "for",
					"name": "循环节点2",
					"configuration": {
						"range": "msg.items2",
						"do": "process_item_2",
						"mode": 2
					}
				},
				{
					"id": "for_node_3",
					"type": "for",
					"name": "循环节点3",
					"configuration": {
						"range": "msg.items3",
						"do": "process_item_3",
						"mode": 3
					}
				},
				{
					"id": "process_item_1",
					"type": "jsTransform",
					"name": "处理项目1",
					"configuration": {
						"jsScript": "metadata['processed_1_' + metadata._loopIndex] = 'item1_' + metadata._loopItem; metadata['processor'] = 'processor_1'; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "process_item_2",
					"type": "jsTransform",
					"name": "处理项目2",
					"configuration": {
						"jsScript": "metadata['processed_2_' + metadata._loopIndex] = 'item2_' + metadata._loopItem; metadata['processor'] = 'processor_2'; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "process_item_3",
					"type": "jsTransform",
					"name": "处理项目3",
					"configuration": {
						"jsScript": "metadata['processed_3_' + metadata._loopIndex] = 'item3_' + metadata._loopItem; metadata['processor'] = 'processor_3'; return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "final_processor",
					"type": "jsTransform",
					"name": "最终处理器",
					"configuration": {
						"jsScript": "metadata['final_processed'] = true; metadata['completion_time'] = Date.now(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "fork_start",
					"toId": "for_node_1",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "for_node_2",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "for_node_3",
					"type": "Success"
				},
				{
					"fromId": "for_node_1",
					"toId": "final_processor",
					"type": "Success"
				},
				{
					"fromId": "for_node_2",
					"toId": "final_processor",
					"type": "Success"
				},
				{
					"fromId": "for_node_3",
					"toId": "final_processor",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_fork_for_concurrent", []byte(forkForRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 并发测试参数
	concurrentCount := 20
	itemsPerArray := 5
	var successCount int64
	var errorCount int64
	var finalProcessorCount int64

	// 用于同步等待所有消息处理完成
	done := make(chan bool, 1)

	// 启动多个goroutine并发发送消息
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// 创建包含三个数组的消息，每个数组对应一个for节点
			items1 := make([]interface{}, itemsPerArray)
			items2 := make([]interface{}, itemsPerArray)
			items3 := make([]interface{}, itemsPerArray)
			for j := 0; j < itemsPerArray; j++ {
				items1[j] = fmt.Sprintf("batch_%d_item1_%d", index, j)
				items2[j] = fmt.Sprintf("batch_%d_item2_%d", index, j)
				items3[j] = fmt.Sprintf("batch_%d_item3_%d", index, j)
			}

			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))
			metaData.PutValue("test_type", "fork_for_concurrent")

			items1JSON, _ := json.Marshal(items1)
			items2JSON, _ := json.Marshal(items2)
			items3JSON, _ := json.Marshal(items3)
			msg := types.NewMsg(0, "TEST_FORK_FOR_CONCURRENT", types.JSON, metaData,
				fmt.Sprintf(`{"items1": %s, "items2": %s, "items3": %s, "batch_id": %d}`,
					items1JSON, items2JSON, items3JSON, index))

			// 发送消息并等待处理完成
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)

				} else {
					atomic.AddInt64(&successCount, 1)
					// 检查是否是最终处理器的结果
					if msg.Metadata.GetValue("final_processed") == "true" {
						atomic.AddInt64(&finalProcessorCount, 1)
					}

				}
				// 只计算最终处理器的调用次数来判断完成
				if msg.Metadata.GetValue("final_processed") == "true" {
					if atomic.LoadInt64(&finalProcessorCount) >= int64(concurrentCount*3) {
						select {
						case done <- true:
						default:
						}
					}
				}
			}))
		}(i)
	}

	// 等待所有消息处理完成
	select {
	case <-done:
		// 所有消息处理完成
	case <-time.After(15 * time.Second):
		t.Fatal("测试超时")
	}

	// 验证结果
	if errorCount > 0 {
		t.Errorf("期望0个错误，实际有 %d 个错误", errorCount)
	}

	// 验证最终处理器被调用的次数（应该等于并发数量的3倍，因为每个消息会触发3个for节点）
	expectedFinalCount := int64(concurrentCount * 3)
	if finalProcessorCount != expectedFinalCount {
		t.Errorf("期望最终处理器被调用 %d 次，实际调用 %d 次", expectedFinalCount, finalProcessorCount)
	}

}

// TestForNodeAsyncModeMetadataSafety 测试for节点异步模式下的元数据安全性
func TestForNodeAsyncModeMetadataSafety(t *testing.T) {
	// 创建异步模式的for节点规则链
	asyncRuleChain := `{
		"ruleChain": {
			"id": "test_async_safety",
			"name": "testAsyncSafety",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "async_for",
					"type": "for",
					"name": "异步循环",
					"configuration": {
						"range": "msg.items",
						"do": "async_processor",
						"mode": 3
					}
				},
				{
					"id": "async_processor",
					"type": "jsTransform",
					"name": "异步处理器",
					"configuration": {
						"jsScript": "metadata['async_processed_' + metadata._loopIndex] = metadata._loopItem; metadata['process_time'] = Date.now(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "async_for",
					"toId": "async_processor",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	ruleEngine, err := New("test_async_safety", []byte(asyncRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 创建包含大量项目的消息
	itemsCount := 50
	items := make([]interface{}, itemsCount)
	for i := 0; i < itemsCount; i++ {
		items[i] = fmt.Sprintf("async_item_%d", i)
	}

	itemsJSON, _ := json.Marshal(items)
	msgData := fmt.Sprintf(`{"items": %s}`, itemsJSON)
	metaData := types.NewMetadata()
	metaData.PutValue("test_type", "async_safety")
	metaData.PutValue("items_count", strconv.Itoa(itemsCount))

	msg := types.NewMsg(0, "ASYNC_TEST", types.JSON, metaData, msgData)

	// 发送消息并验证异步处理不会导致数据竞争
	var processedCount int64
	var errorCount int64

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		if err != nil {
			atomic.AddInt64(&errorCount, 1)

		} else {
			atomic.AddInt64(&processedCount, 1)

		}
	}))

	// 等待一段时间让异步处理完成
	time.Sleep(2 * time.Second)

	// 验证结果
	finalProcessedCount := atomic.LoadInt64(&processedCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)
	if finalProcessedCount != 1 {
		t.Errorf("期望处理1条消息，实际处理 %d 条", finalProcessedCount)
	}
	if finalErrorCount != 0 {
		t.Errorf("期望0个错误，实际有 %d 个错误", finalErrorCount)
	}

}

// TestConcurrentRaceCondition 测试getEnv并发竞态条件
func TestConcurrentGetEnv(t *testing.T) {
	// 规则链DSL - 一个节点分叉到两个并发节点
	ruleChainDSL := `{
		"ruleChain": {
			"id": "kOPFwceGDK9p",
			"name": "测试并发",
			"root": true,
			"debugMode": true,
			"additionalInfo": {
				"description": "",
				"layoutX": "280",
				"layoutY": "280"
			},
			"configuration": {}
		},
		"metadata": {
			"endpoints": [],
			"nodes": [
				{
					"id": "node_2",
					"type": "restApiCall",
					"name": "并发1",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 480,
						"layoutY": 280
					}
				},
				{
					"id": "node_3",
					"type": "restApiCall",
					"name": "并发2",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 750,
						"layoutY": 200
					}
				},
				{
					"id": "node_4",
					"type": "restApiCall",
					"name": "并发3",
					"configuration": {
						"requestMethod": "GET",
						"headers": {
							"Content-Type": "application/json",
							"Token": "${metadata.token}"
						},
						"readTimeoutMs": 2000,
						"insecureSkipVerify": true,
						"maxParallelRequestsCount": 200,
						"proxyPort": 0,
						"restEndpointUrlPattern": "https://aa/delay/1"
					},
					"debugMode": false,
					"additionalInfo": {
						"layoutX": 750,
						"layoutY": 350
					}
				}
			],
			"connections": [
			{
				"fromId": "node_2",
				"toId": "node_3",
				"type": "Success"
			},
			{
				"fromId": "node_2",
				"toId": "node_3",
				"type": "Failure"
			},
			{
				"fromId": "node_2",
				"toId": "node_4",
				"type": "Success"
			},
			{
				"fromId": "node_2",
				"toId": "node_4",
				"type": "Failure"
			}
			]
		}
	}`

	// 创建规则引擎
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test", []byte(ruleChainDSL), WithConfig(config))
	assert.Nil(t, err)

	// 并发测试参数
	concurrentCount := 50 // 并发数量
	messageCount := 1     // 每个协程发送的消息数量

	var wg sync.WaitGroup
	wg.Add(concurrentCount * messageCount * 2)
	// 启动多个协程并发执行规则链
	for i := 0; i < concurrentCount; i++ {
		go func(routineID int) {

			// 每个协程发送多条消息
			for j := 0; j < messageCount; j++ {
				metadata := types.NewMetadata()
				// 创建消息
				msg := types.NewMsg(0, "TEST", types.JSON, metadata, fmt.Sprintf(`{"id":%d,"count":%d}`, routineID, j))

				// 设置metadata，包含token用于模板替换
				msg.Metadata.PutValue("token", fmt.Sprintf("token_%d_%d", routineID, j))
				msg.Metadata.PutValue("routineID", fmt.Sprintf("%d", routineID))
				msg.Metadata.PutValue("messageID", fmt.Sprintf("%d", j))

				// 执行规则链
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					wg.Done()
				}))

				// 添加小延迟，增加并发竞争的可能性
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	// 等待所有协程完成
	wg.Wait()

}
