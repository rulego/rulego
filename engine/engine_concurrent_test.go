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

// TestForNodeConcurrentWithFork 测试使用fork节点并发执行，同时验证SharedData和Metadata的并发安全性
// 这个测试验证fork节点能够正确地将消息分发到多个节点并行处理，并测试数据和元数据的并发修改
func TestForNodeConcurrentWithFork(t *testing.T) {
	// 创建包含fork节点和多个并发处理节点的规则链
	forkRuleChain := `{
		"ruleChain": {
			"id": "test_fork_concurrent_data_safety",
			"name": "测试Fork并发数据安全性",
			"debugMode": true,
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
					"id": "concurrent_processor_1",
					"type": "jsTransform",
					"name": "并发处理器1",
					"configuration": {
						"jsScript": "msg.processor1_timestamp = Date.now(); msg.processor1_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor1'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "concurrent_processor_2",
					"type": "jsTransform",
					"name": "并发处理器2",
					"configuration": {
						"jsScript": "msg.processor2_timestamp = Date.now(); msg.processor2_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor2'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "concurrent_processor_3",
					"type": "jsTransform",
					"name": "并发处理器3",
					"configuration": {
						"jsScript": "msg.processor3_timestamp = Date.now(); msg.processor3_id = Math.random(); msg.concurrent_modifications = (msg.concurrent_modifications || 0) + 1; metadata['processor3'] = 'executed_' + Date.now(); metadata['total_processors'] = (parseInt(metadata['total_processors'] || '0') + 1).toString(); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				},
				{
					"id": "final_validator",
					"type": "jsTransform",
					"name": "最终验证器",
					"configuration": {
						"jsScript": "metadata['final_processed'] = 'true'; metadata['completion_time'] = Date.now(); metadata['data_integrity_check'] = (msg.concurrent_modifications >= 1 ? 'passed' : 'failed'); return {'msg': msg, 'metadata': metadata, 'msgType': msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_1",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_2",
					"type": "Success"
				},
				{
					"fromId": "fork_start",
					"toId": "concurrent_processor_3",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_1",
					"toId": "final_validator",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_2",
					"toId": "final_validator",
					"type": "Success"
				},
				{
					"fromId": "concurrent_processor_3",
					"toId": "final_validator",
					"type": "Success"
				}
			],
			"ruleChainConnections": null
		}
	}`

	config := NewConfig()
	var dataCorruptions int64
	var metadataCorruptions int64
	var refCountAnomalies int64
	var inDataValidations int64
	var outDataValidations int64
	var nodeProcessingErrors int64

	// 配置调试回调以检测并发问题和验证每个节点的IN/OUT数据准确性
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		// 处理错误情况
		if err != nil {
			atomic.AddInt64(&nodeProcessingErrors, 1)
			return
		}

		// 直接验证数据完整性
		data := msg.GetData()
		if data == "" {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// 验证JSON格式
		var jsonData map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// 验证引用计数
		if sharedData := msg.Data; sharedData != nil {
			if refCount := sharedData.GetRefCount(); refCount <= 0 {
				atomic.AddInt64(&refCountAnomalies, 1)
			}
		}

		// 验证元数据完整性
		if msg.Metadata.Len() == 0 {
			atomic.AddInt64(&metadataCorruptions, 1)
			return
		}

		// 验证每个节点的IN/OUT数据准确性
		if flowType == types.In {
			atomic.AddInt64(&inDataValidations, 1)
			// 验证输入数据的完整性
			switch nodeId {
			case "fork_start":
				// fork节点输入：应该包含原始数据
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if initialValue, exists := jsonData["initial_value"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := initialValue.(string); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证原始元数据
				batchId := msg.Metadata.GetValue("batch_id")
				if batchId == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_1", "concurrent_processor_2", "concurrent_processor_3":
				// 并发处理器输入：应该包含原始数据和fork传递的数据
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证并发修改字段的初始状态
				if modifications, exists := jsonData["concurrent_modifications"]; exists {
					if modCount, ok := modifications.(float64); ok && modCount < 0 {
						atomic.AddInt64(&dataCorruptions, 1)
					}
				}

			case "final_validator":
				// 最终验证器输入：应该包含被处理器修改过的数据
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证处理器特定字段
				processorFound := false
				for _, field := range []string{"processor1_timestamp", "processor2_timestamp", "processor3_timestamp"} {
					if _, exists := jsonData[field]; exists {
						processorFound = true
						break
					}
				}
				if !processorFound {
					atomic.AddInt64(&dataCorruptions, 1)
				}
			}

		} else if flowType == types.Out {
			atomic.AddInt64(&outDataValidations, 1)
			// 验证输出数据的完整性
			switch nodeId {
			case "fork_start":
				// fork节点输出：数据应该保持不变
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if initialValue, exists := jsonData["initial_value"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := initialValue.(string); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

			case "concurrent_processor_1":
				// 并发处理器1输出：应该包含processor1特定的字段
				if timestamp, exists := jsonData["processor1_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processorId, exists := jsonData["processor1_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := processorId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证并发修改计数增加
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// 验证元数据中的processor1字段
				if processor1 := msg.Metadata.GetValue("processor1"); processor1 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_2":
				// 并发处理器2输出验证
				if timestamp, exists := jsonData["processor2_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor2 := msg.Metadata.GetValue("processor2"); processor2 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_3":
				// 并发处理器3输出验证
				if timestamp, exists := jsonData["processor3_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor3 := msg.Metadata.GetValue("processor3"); processor3 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "final_validator":
				// 最终验证器输出：应该包含所有验证标记
				if finalProcessed := msg.Metadata.GetValue("final_processed"); finalProcessed != "true" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if completionTime := msg.Metadata.GetValue("completion_time"); completionTime == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck != "passed" && integrityCheck != "failed" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				// 验证原始数据仍然存在
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}
			}

			// 通用验证：所有输出都应该保持原始batch_id
			originalBatchId := msg.Metadata.GetValue("batch_id")
			if originalBatchId == "" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}

			// 通用验证：所有输出都应该保持原始test_type
			testType := msg.Metadata.GetValue("test_type")
			if testType != "fork_concurrent_data_safety" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}
		}
	}

	ruleEngine, err := New("test_fork_concurrent_data_safety", []byte(forkRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("创建规则引擎失败: %v", err)
	}

	// 并发测试参数
	concurrentCount := 30 // 增加并发数量以增强测试强度
	var successCount int64
	var errorCount int64
	var finalProcessorCount int64

	// 用于同步等待所有消息处理完成
	done := make(chan bool, 1)

	// 启动多个goroutine并发发送消息
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// 创建包含复杂数据的消息以增加并发修改的复杂性
			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))
			metaData.PutValue("test_type", "fork_concurrent_data_safety")
			metaData.PutValue("initial_processor_count", "0")

			// 创建包含多个字段的JSON数据，这些将被并发修改
			originalData := fmt.Sprintf(`{
				"message_id": %d, 
				"initial_value": "test_%d",
				"concurrent_modifications": 0,
				"creation_timestamp": %d,
				"processor_data": {}
			}`, index, index, time.Now().UnixNano())

			msg := types.NewMsg(0, "TEST_FORK_CONCURRENT_SAFETY", types.JSON, metaData, originalData)

			// 发送消息并等待处理完成
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)

					// 检查是否是最终处理器的结果
					if msg.Metadata.GetValue("final_processed") == "true" {
						atomic.AddInt64(&finalProcessorCount, 1)

						// 验证数据完整性
						data := msg.GetData()
						var jsonData map[string]interface{}
						if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("最终数据JSON解析失败: %v, 数据: %s", jsonErr, data)
						} else {
							// 验证原始数据是否还存在
							if messageId, exists := jsonData["message_id"]; !exists {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("原始message_id丢失，数据: %s", data)
							} else if float64(index) != messageId {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("message_id不匹配: 期望 %d, 实际 %v", index, messageId)
							}

							// 验证并发修改计数
							if modifications, exists := jsonData["concurrent_modifications"]; exists {
								if modCount, ok := modifications.(float64); ok && modCount < 1 {
									t.Errorf("并发修改计数异常: %v", modCount)
								}
							}
						}

						// 验证元数据完整性
						originalBatchId := msg.Metadata.GetValue("batch_id")
						if originalBatchId != strconv.Itoa(index) {
							atomic.AddInt64(&metadataCorruptions, 1)
							t.Errorf("batch_id不匹配: 期望 %d, 实际 %s", index, originalBatchId)
						}

						// 验证数据完整性检查结果
						if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck == "failed" {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("数据完整性检查失败")
						}
					}
				}

				// 当所有最终处理器完成时发送完成信号
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
	case <-time.After(20 * time.Second): // 增加超时时间
		t.Fatal("测试超时")
	}

	// 等待额外时间确保所有异步操作完成
	time.Sleep(time.Second)

	// 验证结果
	finalErrorCount := atomic.LoadInt64(&errorCount)
	finalDataCorruptions := atomic.LoadInt64(&dataCorruptions)
	finalMetadataCorruptions := atomic.LoadInt64(&metadataCorruptions)
	finalRefCountAnomalies := atomic.LoadInt64(&refCountAnomalies)
	finalProcessorCountResult := atomic.LoadInt64(&finalProcessorCount)
	finalInDataValidations := atomic.LoadInt64(&inDataValidations)
	finalOutDataValidations := atomic.LoadInt64(&outDataValidations)
	finalNodeProcessingErrors := atomic.LoadInt64(&nodeProcessingErrors)

	// 验证错误
	if finalNodeProcessingErrors > 0 {
		t.Errorf("期望0个节点处理错误，实际有 %d 个错误", finalNodeProcessingErrors)
	}

	if finalErrorCount > 0 {
		t.Errorf("期望0个处理错误，实际有 %d 个错误", finalErrorCount)
	}

	if finalDataCorruptions > 0 {
		t.Errorf("检测到 %d 次数据损坏", finalDataCorruptions)
	}

	if finalMetadataCorruptions > 0 {
		t.Errorf("检测到 %d 次元数据损坏", finalMetadataCorruptions)
	}

	if finalRefCountAnomalies > 0 {
		t.Errorf("检测到 %d 次引用计数异常", finalRefCountAnomalies)
	}

	// 验证数据验证次数的合理性（每个消息会产生多次IN/OUT事件）
	expectedMinValidations := int64(concurrentCount * 4) // 每个消息至少经过4个节点
	if finalInDataValidations < expectedMinValidations {
		t.Errorf("IN数据验证次数过少: 期望至少 %d 次，实际 %d 次", expectedMinValidations, finalInDataValidations)
	}

	if finalOutDataValidations < expectedMinValidations {
		t.Errorf("OUT数据验证次数过少: 期望至少 %d 次，实际 %d 次", expectedMinValidations, finalOutDataValidations)
	}

	// 验证最终处理器被调用的次数（应该等于并发数量的3倍，因为每个消息会触发3个处理器）
	expectedFinalCount := int64(concurrentCount * 3)
	if finalProcessorCountResult != expectedFinalCount {
		t.Errorf("期望最终处理器被调用 %d 次，实际调用 %d 次", expectedFinalCount, finalProcessorCountResult)
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
