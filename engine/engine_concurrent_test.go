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

// TestForNodeConcurrentMetadataAccess tests the security of metadata read/write for nodes in concurrent scenarios
// This test specifically checks whether for nodes have concurrent read/write issues when processing metadata
func TestForNodeConcurrentMetadataAccess(t *testing.T) {
	// Create a rule chain containing the for node
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
		t.Fatalf("Rule engine creation failed: %v", err)
	}

	// Concurrent test parameters
	concurrentCount := 50
	itemsPerMessage := 10
	var successCount int64
	var errorCount int64

	// Used to synchronize and wait for all messages to be processed
	done := make(chan bool, 1)

	// Start multiple goroutines to send messages concurrently
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// Create a message containing an array
			items := make([]interface{}, itemsPerMessage)
			for j := 0; j < itemsPerMessage; j++ {
				items[j] = fmt.Sprintf("item_%d_%d", index, j)
			}

			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))

			itemsJSON, _ := json.Marshal(items)
			msg := types.NewMsg(0, "TEST_FOR_CONCURRENT", types.JSON, metaData, fmt.Sprintf(`{"items": %s, "batch_id": %d}`, itemsJSON, index))

			// Send a message and wait for processing to complete
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

	// Wait for all messages to be processed
	select {
	case <-done:
		// All messages are processed completely
	case <-time.After(10 * time.Second):
		t.Fatal("Test timeout")
	}

	// Verify the results
	if successCount != int64(concurrentCount) {
		t.Errorf("Expect to process %d messages, but actually process %d", concurrentCount, successCount)
	}
	if errorCount != 0 {
		t.Errorf("Expect 0 errors, but actually have %d errors", errorCount)
	}

}

// TestForNodeMetadataRaceCondition tests the race conditions for node metadata
// This test specifically checks whether data contention occurs under high concurrency conditions
func TestForNodeMetadataRaceCondition(t *testing.T) {
	// Create a more complex chain of rules with multiple nodes to increase the likelihood of race conditions
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
		t.Fatalf("Rule engine creation failed: %v", err)
	}

	// High concurrency testing
	concurrentCount := 100
	var wg sync.WaitGroup
	var processedCount int64

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Launch multiple goroutines to send messages simultaneously
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
					t.Errorf("Error when handling messages: %v", err)
				} else {
					atomic.AddInt64(&processedCount, 1)
				}
			}))
		}(i)
	}

	// Wait for all goroutines to finish or time out
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:

	case <-ctx.Done():
		currentProcessed := atomic.LoadInt64(&processedCount)
		t.Errorf("Test timeout: Only %d messages were processed", currentProcessed)
	}

	// Verification at least handled some messages
	finalProcessedCount := atomic.LoadInt64(&processedCount)
	if finalProcessedCount <= 0 {
		t.Errorf("At least some messages should be handled, and actual processing is: %d", finalProcessedCount)
	}
}

// TestForNodeConcurrentWithFork tests are executed concurrently using fork nodes, while verifying the concurrency security of SharedData and Metadata
// This test verifies that fork nodes can correctly distribute messages to multiple nodes for parallel processing and test concurrent modifications of data and metadata
func TestForNodeConcurrentWithFork(t *testing.T) {
	// Create a rule chain containing fork nodes and multiple concurrent processing nodes
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

	// Configure debug callbacks to detect concurrency issues and verify the accuracy of IN/OUT data for each node
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		// Handling errors
		if err != nil {
			atomic.AddInt64(&nodeProcessingErrors, 1)
			return
		}

		// Directly verify data integrity
		data := msg.GetData()
		if data == "" {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// Verify JSON format
		var jsonData map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
			atomic.AddInt64(&dataCorruptions, 1)
			return
		}

		// Verify citation counts
		if sharedData := msg.Data; sharedData != nil {
			if refCount := sharedData.GetRefCount(); refCount <= 0 {
				atomic.AddInt64(&refCountAnomalies, 1)
			}
		}

		// Verify metadata integrity
		if msg.Metadata.Len() == 0 {
			atomic.AddInt64(&metadataCorruptions, 1)
			return
		}

		// Verify the accuracy of IN/OUT data at each node
		if flowType == types.In {
			atomic.AddInt64(&inDataValidations, 1)
			// Verify the integrity of input data
			switch nodeId {
			case "fork_start":
				// fork node input: should include the original data
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

				// Verify the original metadata
				batchId := msg.Metadata.GetValue("batch_id")
				if batchId == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_1", "concurrent_processor_2", "concurrent_processor_3":
				// Concurrency processor input: should include the original data and data passed by the fork
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// Verify the initial state of the concurrent modified field
				if modifications, exists := jsonData["concurrent_modifications"]; exists {
					if modCount, ok := modifications.(float64); ok && modCount < 0 {
						atomic.AddInt64(&dataCorruptions, 1)
					}
				}

			case "final_validator":
				// Final validator input: should contain data modified by the processor
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// Validate processor-specific fields
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
			// Verify the integrity of output data
			switch nodeId {
			case "fork_start":
				// fork node output: data should remain unchanged
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
				// Concurrent processor 1 output: should include the specific fields of processor1
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

				// Validation concurrent modification count increases
				if modifications, exists := jsonData["concurrent_modifications"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if modCount, ok := modifications.(float64); !ok || modCount < 1 {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				// Validate the processor1 field in the metadata
				if processor1 := msg.Metadata.GetValue("processor1"); processor1 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_2":
				// Concurrent processor 2 outputs verification
				if timestamp, exists := jsonData["processor2_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor2 := msg.Metadata.GetValue("processor2"); processor2 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "concurrent_processor_3":
				// Concurrent processor 3 outputs verification
				if timestamp, exists := jsonData["processor3_timestamp"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := timestamp.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}

				if processor3 := msg.Metadata.GetValue("processor3"); processor3 == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

			case "final_validator":
				// Final validator output: should include all validation marks
				if finalProcessed := msg.Metadata.GetValue("final_processed"); finalProcessed != "true" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if completionTime := msg.Metadata.GetValue("completion_time"); completionTime == "" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck != "passed" && integrityCheck != "failed" {
					atomic.AddInt64(&metadataCorruptions, 1)
				}

				// The original data is still validated
				if messageId, exists := jsonData["message_id"]; !exists {
					atomic.AddInt64(&dataCorruptions, 1)
				} else if _, ok := messageId.(float64); !ok {
					atomic.AddInt64(&dataCorruptions, 1)
				}
			}

			// Universal verification: All outputs should retain their original batch_id
			originalBatchId := msg.Metadata.GetValue("batch_id")
			if originalBatchId == "" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}

			// Universal verification: All outputs should maintain their original test_type
			testType := msg.Metadata.GetValue("test_type")
			if testType != "fork_concurrent_data_safety" {
				atomic.AddInt64(&metadataCorruptions, 1)
			}
		}
	}

	ruleEngine, err := New("test_fork_concurrent_data_safety", []byte(forkRuleChain), WithConfig(config))
	if err != nil {
		t.Fatalf("Rule engine creation failed: %v", err)
	}

	// Concurrent test parameters
	concurrentCount := 30 // Increase concurrency counts to enhance test intensity
	var successCount int64
	var errorCount int64
	var finalProcessorCount int64

	// Used to synchronize and wait for all messages to be processed
	done := make(chan bool, 1)

	// Start multiple goroutines to send messages concurrently
	for i := 0; i < concurrentCount; i++ {
		go func(index int) {
			// Create messages containing complex data to increase the complexity of concurrent modifications
			metaData := types.NewMetadata()
			metaData.PutValue("batch_id", strconv.Itoa(index))
			metaData.PutValue("start_time", strconv.FormatInt(time.Now().UnixNano(), 10))
			metaData.PutValue("test_type", "fork_concurrent_data_safety")
			metaData.PutValue("initial_processor_count", "0")

			// Create JSON data containing multiple fields, which will be modified concurrently
			originalData := fmt.Sprintf(`{
				"message_id": %d, 
				"initial_value": "test_%d",
				"concurrent_modifications": 0,
				"creation_timestamp": %d,
				"processor_data": {}
			}`, index, index, time.Now().UnixNano())

			msg := types.NewMsg(0, "TEST_FORK_CONCURRENT_SAFETY", types.JSON, metaData, originalData)

			// Send a message and wait for processing to complete
			ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)

					// Check if it is the final processor result
					if msg.Metadata.GetValue("final_processed") == "true" {
						atomic.AddInt64(&finalProcessorCount, 1)

						// Verify data integrity
						data := msg.GetData()
						var jsonData map[string]interface{}
						if jsonErr := json.Unmarshal([]byte(data), &jsonData); jsonErr != nil {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("Final data JSON parsing failed: %v, Data: %s", jsonErr, data)
						} else {
							// Verify whether the original data still exists
							if messageId, exists := jsonData["message_id"]; !exists {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("Original message_id lost, data: %s", data)
							} else if float64(index) != messageId {
								atomic.AddInt64(&dataCorruptions, 1)
								t.Errorf("message_id mismatch: expectations %d, actual %v", index, messageId)
							}

							// Validate concurrent count modifications
							if modifications, exists := jsonData["concurrent_modifications"]; exists {
								if modCount, ok := modifications.(float64); ok && modCount < 1 {
									t.Errorf("Concurrent modification of count anomaly: %v", modCount)
								}
							}
						}

						// Verify metadata integrity
						originalBatchId := msg.Metadata.GetValue("batch_id")
						if originalBatchId != strconv.Itoa(index) {
							atomic.AddInt64(&metadataCorruptions, 1)
							t.Errorf("batch_id mismatch: expectations %d, actual %s", index, originalBatchId)
						}

						// Verify data integrity check results
						if integrityCheck := msg.Metadata.GetValue("data_integrity_check"); integrityCheck == "failed" {
							atomic.AddInt64(&dataCorruptions, 1)
							t.Errorf("Data integrity check failed")
						}
					}
				}

				// When all final processors are finished, a completion signal is sent
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

	// Wait for all messages to be processed
	select {
	case <-done:
		// All messages are processed completely
	case <-time.After(20 * time.Second): // Increased timeout
		t.Fatal("Test timeout")
	}

	// Wait extra time to ensure all asynchronous operations are completed
	time.Sleep(time.Second)

	// Verify the results
	finalErrorCount := atomic.LoadInt64(&errorCount)
	finalDataCorruptions := atomic.LoadInt64(&dataCorruptions)
	finalMetadataCorruptions := atomic.LoadInt64(&metadataCorruptions)
	finalRefCountAnomalies := atomic.LoadInt64(&refCountAnomalies)
	finalProcessorCountResult := atomic.LoadInt64(&finalProcessorCount)
	finalInDataValidations := atomic.LoadInt64(&inDataValidations)
	finalOutDataValidations := atomic.LoadInt64(&outDataValidations)
	finalNodeProcessingErrors := atomic.LoadInt64(&nodeProcessingErrors)

	// Verify errors
	if finalNodeProcessingErrors > 0 {
		t.Errorf("Expecting 0 nodes to handle errors, but actually having %d errors", finalNodeProcessingErrors)
	}

	if finalErrorCount > 0 {
		t.Errorf("Expect zero processing errors, but actually have %d errors", finalErrorCount)
	}

	if finalDataCorruptions > 0 {
		t.Errorf("%d data corruption detected", finalDataCorruptions)
	}

	if finalMetadataCorruptions > 0 {
		t.Errorf("%d dimensional data corruption detected", finalMetadataCorruptions)
	}

	if finalRefCountAnomalies > 0 {
		t.Errorf("%d citation count anomalies detected", finalRefCountAnomalies)
	}

	// Validate the validation count of data validation (each message generates multiple IN/OUT events)
	expectedMinValidations := int64(concurrentCount * 4) // Each message passes through at least 4 nodes
	if finalInDataValidations < expectedMinValidations {
		t.Errorf("IN Data validation is too few: Expect at least %d times, but actually %d times", expectedMinValidations, finalInDataValidations)
	}

	if finalOutDataValidations < expectedMinValidations {
		t.Errorf("OUT Data validation is too few: Expect at least %d times, but actually %d times", expectedMinValidations, finalOutDataValidations)
	}

	// Verify the number of calls made by the final processor (which should be three times the number of concurrent times, since each message triggers three processors)
	expectedFinalCount := int64(concurrentCount * 3)
	if finalProcessorCountResult != expectedFinalCount {
		t.Errorf("The final processor is expected to be called %d times, and the actual call is %d times", expectedFinalCount, finalProcessorCountResult)
	}
}

// TestForNodeAsyncModeMetadataSafety Tests metadata security under asynchronous mode for nodes
func TestForNodeAsyncModeMetadataSafety(t *testing.T) {
	// Create an asynchronous for node rule chain
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
		t.Fatalf("Rule engine creation failed: %v", err)
	}

	// Create messages containing a large number of items
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

	// Sending messages and verifying asynchronous processing does not lead to data contention
	var processedCount int64
	var errorCount int64

	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		if err != nil {
			atomic.AddInt64(&errorCount, 1)

		} else {
			atomic.AddInt64(&processedCount, 1)

		}
	}))

	// Wait a while for the asynchronous processing to complete
	time.Sleep(2 * time.Second)

	// Verify the results
	finalProcessedCount := atomic.LoadInt64(&processedCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)
	if finalProcessedCount != 1 {
		t.Errorf("Expected to process 1 message, actually processed %d", finalProcessedCount)
	}
	if finalErrorCount != 0 {
		t.Errorf("Expect 0 errors, but actually have %d errors", finalErrorCount)
	}

}

// TestConcurrentRaceCondition tests getEnv concurrent race condition
func TestConcurrentGetEnv(t *testing.T) {
	// Rule Chain DSL - One node forks into two concurrent nodes
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

	// Create a rule engine
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("test", []byte(ruleChainDSL), WithConfig(config))
	assert.Nil(t, err)

	// Concurrent test parameters
	concurrentCount := 50 // Concurrent quantity
	messageCount := 1     // The number of messages sent per coroutine

	var wg sync.WaitGroup
	wg.Add(concurrentCount * messageCount * 2)
	// Initiate multiple coroutines to concurrently execute the rule chain
	for i := 0; i < concurrentCount; i++ {
		go func(routineID int) {

			// Each coroutine sends multiple messages
			for j := 0; j < messageCount; j++ {
				metadata := types.NewMetadata()
				// Create a message
				msg := types.NewMsg(0, "TEST", types.JSON, metadata, fmt.Sprintf(`{"id":%d,"count":%d}`, routineID, j))

				// Set up metadata, including tokens for template replacement
				msg.Metadata.PutValue("token", fmt.Sprintf("token_%d_%d", routineID, j))
				msg.Metadata.PutValue("routineID", fmt.Sprintf("%d", routineID))
				msg.Metadata.PutValue("messageID", fmt.Sprintf("%d", j))

				// Execute the rule chain
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					wg.Done()
				}))

				// Adding small latency increases the likelihood of concurrent competition
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	// Wait for all coroutines to complete
	wg.Wait()

}
