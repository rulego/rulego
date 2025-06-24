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

package types

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// TestMetadataOperations 测试Metadata的基本操作和COW机制
func TestMetadataOperations(t *testing.T) {
	// 基本操作
	md := NewMetadata()
	assert.Equal(t, 0, md.Len())

	md.PutValue("key1", "value1")
	md.PutValue("key2", "value2")
	assert.True(t, md.Has("key1"))
	assert.Equal(t, "value1", md.GetValue("key1"))
	assert.Equal(t, 2, md.Len())

	values := md.Values()
	assert.Equal(t, 2, len(values))
	assert.Equal(t, "value1", values["key1"])

	data := map[string]string{"key3": "value3", "key4": "value4"}
	md2 := BuildMetadata(data)
	assert.Equal(t, 2, md2.Len())
	assert.Equal(t, "value3", md2.GetValue("key3"))

	// COW机制测试
	original := NewMetadata()
	original.PutValue("key1", "value1")
	original.PutValue("key2", "value2")

	copy1 := original.Copy()
	copy2 := original.Copy()

	assert.Equal(t, "value1", copy1.GetValue("key1"))
	assert.Equal(t, "value1", copy2.GetValue("key1"))

	copy1.PutValue("key1", "modified1")
	copy1.PutValue("key3", "new1")

	assert.Equal(t, "value1", original.GetValue("key1"))
	assert.Equal(t, "value1", copy2.GetValue("key1"))
	assert.Equal(t, "modified1", copy1.GetValue("key1"))
	assert.False(t, original.Has("key3"))
	assert.False(t, copy2.Has("key3"))
	assert.True(t, copy1.Has("key3"))

	newData := map[string]string{"newKey1": "newValue1"}
	copy1.ReplaceAll(newData)
	assert.False(t, copy1.Has("key1"))
	assert.True(t, copy1.Has("newKey1"))
	assert.Equal(t, 1, copy1.Len())
}

// TestMetadataConcurrentAccess 测试Metadata并发访问安全性
func TestMetadataConcurrentAccess(t *testing.T) {
	original := NewMetadata()
	original.PutValue("key1", "value1")

	var wg sync.WaitGroup
	const numGoroutines = 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			copy := original.Copy()
			copy.PutValue("goroutine_key", "value")
			_ = copy.GetValue("key1")
			_ = copy.Has("key1")
			_ = copy.Values()
		}(i)
	}

	wg.Wait()

	assert.Equal(t, "value1", original.GetValue("key1"))
	assert.Equal(t, 1, original.Len())
}

// TestRuleMsgOperations 测试RuleMsg的基本操作和复制机制
func TestRuleMsgOperations(t *testing.T) {
	// 基本操作
	metadata := NewMetadata()
	metadata.PutValue("userId", "12345")

	msg := NewMsg(0, "TEST", JSON, metadata, `{"test": "data"}`)
	assert.True(t, len(msg.Id) > 0)
	assert.Equal(t, "TEST", msg.Type)
	assert.Equal(t, JSON, msg.DataType)
	assert.Equal(t, `{"test": "data"}`, msg.GetData())
	assert.Equal(t, "12345", msg.Metadata.GetValue("userId"))

	byteData := []byte("byte data")
	byteMsg := NewMsgFromBytes(12345, "BYTE_TEST", BINARY, metadata, byteData)
	assert.Equal(t, int64(12345), byteMsg.Ts)
	assert.Equal(t, "BYTE_TEST", byteMsg.Type)
	assert.Equal(t, BINARY, byteMsg.DataType)
	assert.Equal(t, string(byteData), string(byteMsg.GetBytes()))

	// COW机制测试
	original := NewMsg(0, "TEST", JSON, nil, "original data")
	copy1 := original.Copy()
	copy2 := original.Copy()

	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "original data", copy1.GetData())
	assert.Equal(t, "original data", copy2.GetData())

	copy1.SetData("modified data 1")
	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "modified data 1", copy1.GetData())
	assert.Equal(t, "original data", copy2.GetData())

	copy2.SetData("modified data 2")
	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "modified data 1", copy1.GetData())
	assert.Equal(t, "modified data 2", copy2.GetData())

	// nil metadata处理
	msgWithNilMetadata := RuleMsg{
		Id:       "test",
		Type:     "test",
		Data:     NewSharedData("test"),
		Metadata: nil,
	}
	copiedMsg := msgWithNilMetadata.Copy()
	assert.NotNil(t, copiedMsg.Metadata)
	copiedMsg.Metadata.PutValue("test", "value")
	assert.Equal(t, "value", copiedMsg.Metadata.GetValue("test"))
}

// TestSharedDataOperations 测试SharedData的核心功能
func TestSharedDataOperations(t *testing.T) {
	// 基本操作
	sd := NewSharedData("test data")
	assert.Equal(t, "test data", sd.Get())
	assert.Equal(t, "test data", sd.GetUnsafe())
	assert.Equal(t, []byte("test data"), sd.GetBytes())
	assert.Equal(t, int64(1), sd.GetRefCount())
	assert.Equal(t, 9, sd.Len())
	assert.False(t, sd.IsEmpty())

	sd2 := NewSharedDataFromBytes([]byte("byte data"))
	assert.Equal(t, "byte data", sd2.Get())
	assert.Equal(t, 9, sd2.Len())

	// COW机制
	copy := sd.Copy()
	assert.Equal(t, int64(2), sd.GetRefCount())
	assert.Equal(t, int64(2), copy.GetRefCount())

	copy.Set("modified data")
	assert.Equal(t, "test data", sd.Get())
	assert.Equal(t, "modified data", copy.Get())
	assert.Equal(t, int64(1), sd.GetRefCount())
	assert.Equal(t, int64(1), copy.GetRefCount())

	copy.SetBytes([]byte("new byte data"))
	assert.Equal(t, "new byte data", copy.Get())

	copy.SetUnsafe("unsafe data")
	assert.Equal(t, "unsafe data", copy.GetUnsafe())

	empty := NewSharedData("")
	assert.True(t, empty.IsEmpty())
	assert.Equal(t, 0, empty.Len())

	// 可修改字节数组测试
	msd := NewSharedData("Hello World")
	mutableBytes := msd.GetMutableBytes()
	mutableBytes[0] = 'h'
	assert.Equal(t, "Hello World", msd.Get())
	assert.Equal(t, "hello World", string(mutableBytes))
	msd.SetBytes(mutableBytes)
	assert.Equal(t, "hello World", msd.Get())
}

// TestRuleMsgZeroCopyAndSharedData 测试零拷贝API和SharedData访问
func TestRuleMsgZeroCopyAndSharedData(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, NewMetadata(), "")

	// 零拷贝API测试
	testData := "Zero Copy Test Data"
	msg.SetData(testData)
	result := msg.GetData()
	assert.Equal(t, testData, result)
	assert.Equal(t, []byte(testData), msg.GetBytes())

	newData := "Updated Data"
	msg.SetData(newData)
	assert.Equal(t, newData, msg.GetData())

	byteData := []byte("byte data")
	msg.SetBytes(byteData)
	assert.Equal(t, string(byteData), msg.GetData())
	assert.Equal(t, byteData, msg.GetBytes())

	// SharedData直接访问测试
	msg2 := NewMsg(0, "TEST", JSON, NewMetadata(), "Original Data")
	sharedData := msg2.GetSharedData()
	assert.NotNil(t, sharedData)
	assert.Equal(t, "Original Data", sharedData.Get())
	assert.Equal(t, int64(1), sharedData.GetRefCount())

	mutableBytes := sharedData.GetMutableBytes()
	mutableBytes[0] = 'M'
	assert.Equal(t, "Original Data", msg2.GetData())
	assert.Equal(t, "Mriginal Data", string(mutableBytes))

	sharedData.SetBytes(mutableBytes)
	assert.Equal(t, "Mriginal Data", msg2.GetData())

	// 消息间数据共享
	msg3 := NewMsg(0, "TEST2", JSON, NewMetadata(), "")
	sharedDataCopy := sharedData.Copy()
	msg3.SetSharedData(sharedDataCopy)

	assert.Equal(t, "Mriginal Data", msg3.GetData())
	assert.Equal(t, int64(2), sharedData.GetRefCount())

	msg3.SetData("Modified Data")
	assert.Equal(t, "Mriginal Data", msg2.GetData())
	assert.Equal(t, "Modified Data", msg3.GetData())
	assert.Equal(t, int64(1), sharedData.GetRefCount())

	// nil SharedData处理
	msg2.SetSharedData(nil)
	newSharedData := msg2.GetSharedData()
	assert.NotNil(t, newSharedData)
	assert.Equal(t, "", newSharedData.Get())
}

// TestJSONSerialization 测试JSON序列化
func TestJSONSerialization(t *testing.T) {
	metadata := NewMetadata()
	metadata.PutValue("key1", "value1")
	metadata.PutValue("userId", "12345")

	originalMsg := RuleMsg{
		Ts:       1640995200000,
		Id:       "test-msg-id",
		DataType: JSON,
		Type:     "TEST_TYPE",
		Data:     NewSharedData(`{"temperature": 25.5}`),
		Metadata: metadata,
	}

	jsonData, err := json.Marshal(originalMsg)
	assert.Nil(t, err)

	jsonStr := string(jsonData)
	assert.True(t, strings.Contains(jsonStr, `"ts":1640995200000`))
	assert.True(t, strings.Contains(jsonStr, `"id":"test-msg-id"`))
	assert.True(t, strings.Contains(jsonStr, `"type":"TEST_TYPE"`))
	assert.True(t, strings.Contains(jsonStr, `"key1":"value1"`))

	var deserializedMsg RuleMsg
	err = json.Unmarshal(jsonData, &deserializedMsg)
	assert.Nil(t, err)

	assert.Equal(t, originalMsg.Ts, deserializedMsg.Ts)
	assert.Equal(t, originalMsg.Id, deserializedMsg.Id)
	assert.Equal(t, originalMsg.Type, deserializedMsg.Type)
	assert.Equal(t, originalMsg.GetData(), deserializedMsg.GetData())
	assert.Equal(t, "value1", deserializedMsg.Metadata.GetValue("key1"))
	assert.Equal(t, "12345", deserializedMsg.Metadata.GetValue("userId"))

	msgWithEmptyMetadata := RuleMsg{
		Id:       "empty-metadata",
		Type:     "TEST",
		Data:     NewSharedData("test"),
		Metadata: NewMetadata(),
	}
	emptyMetadataJSON, err := json.Marshal(msgWithEmptyMetadata)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(emptyMetadataJSON), `"metadata":{}`))
}

// TestJSONParsingAndCache 测试JSON解析和缓存机制
func TestJSONParsingAndCache(t *testing.T) {
	// 基本JSON解析
	msg := NewMsg(0, "TEST", JSON, nil, `{"test": "data"}`)
	jsonData1, err := msg.GetJsonData()
	assert.Nil(t, err)
	jsonMap1, ok := jsonData1.(map[string]interface{})
	assert.True(t, ok, "Expected JSON object")
	assert.Equal(t, "data", jsonMap1["test"])

	// 缓存测试
	jsonData2, err := msg.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, jsonData1, jsonData2, "Second call should return cached result")

	// 数据修改应清除缓存
	msg.SetData(`{"test": "modified"}`)
	newJsonData, err := msg.GetJsonData()
	assert.Nil(t, err)
	newJsonMap, ok := newJsonData.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "modified", newJsonMap["test"])

	msg.SetBytes([]byte(`{"test": "from_bytes"}`))
	bytesJsonData, err := msg.GetJsonData()
	assert.Nil(t, err)
	bytesJsonMap, ok := bytesJsonData.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "from_bytes", bytesJsonMap["test"])

	// JSON数组支持
	arrayMsg := NewMsg(0, "TEST", JSON, nil, `["apple", "banana", "cherry"]`)
	arrayData, err := arrayMsg.GetJsonData()
	assert.Nil(t, err)
	arraySlice, ok := arrayData.([]interface{})
	assert.True(t, ok, "Expected JSON array")
	assert.Equal(t, 3, len(arraySlice))
	assert.Equal(t, "apple", arraySlice[0])

	// 嵌套JSON
	nestedMsg := NewMsg(0, "TEST", JSON, nil, `[{"name": "Alice"}, {"name": "Bob"}]`)
	nestedData, err := nestedMsg.GetJsonData()
	assert.Nil(t, err)
	nestedSlice, ok := nestedData.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(nestedSlice))
	firstItem, ok := nestedSlice[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Alice", firstItem["name"])

	// SharedData级别缓存
	sd := NewSharedData(`{"name": "Alice", "age": 30}`)
	data1, err := sd.GetJsonData()
	assert.Nil(t, err)
	data2, err := sd.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, data1, data2, "SharedData should cache JSON parsing")

	sd.SetUnsafe(`{"name": "Bob", "age": 25}`)
	data3, err := sd.GetJsonData()
	assert.Nil(t, err)
	assert.NotEqual(t, data1, data3, "Cache should be cleared after data modification")

	// 委托测试
	jsonMsg := NewMsgWithJsonData(`{"delegated": true}`)
	msgData, err := jsonMsg.GetJsonData()
	assert.Nil(t, err)
	sharedData := jsonMsg.GetSharedData()
	sdData, err := sharedData.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, msgData, sdData)

	// nil Data处理
	emptyMsg := RuleMsg{Id: "empty", Type: "test"}
	emptyData, err := emptyMsg.GetJsonData()
	assert.Nil(t, err)
	emptyMap, ok := emptyData.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 0, len(emptyMap))
}

// TestCacheSharedOnCopy 测试复制时的缓存共享和COW保护
func TestCacheSharedOnCopy(t *testing.T) {
	original := NewSharedData(`{"original": true}`)
	originalData1, err := original.GetJsonData()
	assert.Nil(t, err)

	copy := original.Copy()
	copyData, err := copy.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData1, copyData, "Copy should share cache with original")

	originalData2, err := original.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData1, originalData2, "Original should use cache")

	// 修改复制实例（触发COW）
	copy.SetUnsafe(`{"original": false}`)
	originalData3, err := original.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData1, originalData3, "Original cache should remain after copy modification")

	newCopyData, err := copy.GetJsonData()
	assert.Nil(t, err)
	assert.NotEqual(t, originalData1, newCopyData, "Copy should have independent cache after modification")

	// COW保护下的缓存共享测试
	original2 := NewSharedData(`{"user": "Alice", "score": 100}`)
	originalData, err := original2.GetJsonData()
	assert.Nil(t, err)

	copy1 := original2.Copy()
	copy2 := original2.Copy()

	copyData1, err := copy1.GetJsonData()
	assert.Nil(t, err)
	copyData2, err := copy2.GetJsonData()
	assert.Nil(t, err)

	assert.Equal(t, originalData, copyData1, "Copy1 should share parsed cache")
	assert.Equal(t, originalData, copyData2, "Copy2 should share parsed cache")

	copy1.SetUnsafe(`{"user": "Bob", "score": 200}`)
	copy1NewData, err := copy1.GetJsonData()
	assert.Nil(t, err)
	assert.NotEqual(t, originalData, copy1NewData, "Copy1 should have new independent cache after modification")

	originalDataAgain, err := original2.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData, originalDataAgain, "Original cache should remain unchanged")

	copyData2Again, err := copy2.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData, copyData2Again, "Copy2 cache should remain unchanged")
}

// TestConcurrentOperations 测试并发操作安全性
func TestConcurrentOperations(t *testing.T) {
	// RuleMsg并发访问
	msg := NewMsg(0, "CONCURRENT_TEST", JSON, NewMetadata(), "Concurrent Test Data")
	originalSharedData := msg.GetSharedData()

	var wg sync.WaitGroup
	results := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sharedDataCopy := originalSharedData.Copy()
			mutableBytes := sharedDataCopy.GetMutableBytes()
			mutableBytes[0] = byte('A' + index)
			sharedDataCopy.SetBytes(mutableBytes)
			results[index] = sharedDataCopy.GetUnsafe()
		}(i)
	}

	wg.Wait()

	assert.Equal(t, "Concurrent Test Data", msg.GetData())
	for i, result := range results {
		expected := string(byte('A'+i)) + "oncurrent Test Data"
		assert.Equal(t, expected, result)
	}

	// 并发JSON解析测试 - 每个goroutine使用独立的消息副本
	baseJsonMsg := NewMsgWithJsonData(`{"temperature": 25.5, "humidity": 60}`)
	var wg2 sync.WaitGroup
	errors := make(chan error, 1000)

	// 并发读取 - 每个goroutine使用独立副本
	for i := 0; i < 50; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			msgCopy := baseJsonMsg.Copy() // 每个goroutine使用独立副本
			for j := 0; j < 5; j++ {
				_, err := msgCopy.GetJsonData()
				if err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
			}
		}()
	}

	// 并发写入 - 每个goroutine使用独立副本
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func(id int) {
			defer wg2.Done()
			msgCopy := baseJsonMsg.Copy() // 每个goroutine使用独立副本
			for j := 0; j < 3; j++ {
				newData := fmt.Sprintf(`{"temperature": %d, "humidity": 70}`, id)
				msgCopy.SetData(newData)
				// 验证数据设置成功
				if msgCopy.GetData() != newData {
					select {
					case errors <- fmt.Errorf("data not set correctly"):
					default:
					}
					return
				}
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg2.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errors:
			t.Fatalf("Concurrent operation failed: %v", err)
		default:
			// 测试通过
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// SharedData并发操作 - 每个goroutine使用独立副本
	baseSd := NewSharedData(`{"counter": 0, "active": true}`)
	var wg3 sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg3.Add(1)
		go func() {
			defer wg3.Done()
			sdCopy := baseSd.Copy() // 每个goroutine使用独立副本
			for j := 0; j < 5; j++ {
				_, err := sdCopy.GetJsonData()
				if err != nil {
					select {
					case errors <- err:
					default:
					}
					return
				}
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg3.Add(1)
		go func(id int) {
			defer wg3.Done()
			sdCopy := baseSd.Copy() // 每个goroutine使用独立副本
			newData := fmt.Sprintf(`{"counter": %d, "active": true}`, id)
			sdCopy.SetUnsafe(newData)
			// 验证数据设置成功
			if sdCopy.GetUnsafe() != newData {
				select {
				case errors <- fmt.Errorf("shared data not set correctly"):
				default:
				}
				return
			}
		}(i)
	}

	done3 := make(chan struct{})
	go func() {
		wg3.Wait()
		close(done3)
	}()

	select {
	case <-done3:
		// 测试通过
	case <-time.After(5 * time.Second):
		t.Fatal("SharedData test timed out")
	}
}

// TestConcurrentModificationSafety 测试多个实例并发修改的安全性
func TestConcurrentModificationSafety(t *testing.T) {
	original := NewSharedData(`{"concurrent": "test", "value": "100"}`)
	originalData, err := original.GetJsonData()
	assert.Nil(t, err)

	const numCopies = 10
	copies := make([]*SharedData, numCopies)
	for i := 0; i < numCopies; i++ {
		copies[i] = original.Copy()
	}

	// 验证共享缓存
	for i, copy := range copies {
		copyData, err := copy.GetJsonData()
		assert.Nil(t, err)
		assert.Equal(t, originalData, copyData, "Copy %d should share cache", i)
	}

	var wg sync.WaitGroup
	errors := make(chan error, numCopies)
	results := make([]string, numCopies)

	// 并发修改所有副本
	for i := 0; i < numCopies; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			newData := fmt.Sprintf(`{"concurrent": "modified", "value": "%d", "index": %d}`, index, index)
			copies[index].SetUnsafe(newData)

			modifiedData, err := copies[index].GetJsonData()
			if err != nil {
				select {
				case errors <- err:
				default:
				}
				return
			}

			modifiedMap := modifiedData.(map[string]interface{})
			results[index] = modifiedMap["value"].(string)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errors:
			t.Fatalf("Concurrent modification failed: %v", err)
		default:
			// 测试通过
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}

	// 验证独立性
	for i, result := range results {
		expected := fmt.Sprintf("%d", i)
		assert.Equal(t, expected, result, "Copy %d should have independent value", i)
		assert.Equal(t, int64(1), copies[i].GetRefCount(), "Copy %d should have independent refCount", i)
	}

	// 验证原始实例不受影响
	originalDataAgain, err := original.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData, originalDataAgain, "Original should remain unchanged")
	assert.Equal(t, int64(1), original.GetRefCount(), "Original should have independent refCount")
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	// 无效JSON
	msg := NewMsg(0, "ERROR_TEST", JSON, nil, "invalid json {")
	_, err := msg.GetJsonData()
	assert.NotNil(t, err)

	// 空数据JSON解析
	emptyMsg := NewMsg(0, "EMPTY_TEST", JSON, nil, "")
	emptyJson, err := emptyMsg.GetJsonData()
	assert.Nil(t, err)
	emptyMap, ok := emptyJson.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 0, len(emptyMap))

	// nil metadata
	copy := BuildMetadataFromMetadata(nil)
	assert.NotNil(t, copy)
	assert.Equal(t, 0, copy.Len())

	// 无效JSON后修复
	invalidSD := NewSharedData(`{"invalid": json}`)
	_, err = invalidSD.GetJsonData()
	assert.NotNil(t, err)

	invalidSD.SetUnsafe(`{"valid": "json"}`)
	data, err := invalidSD.GetJsonData()
	assert.Nil(t, err)
	jsonMap, ok := data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "json", jsonMap["valid"])
}

// TestAPICompatibility 测试API兼容性
func TestAPICompatibility(t *testing.T) {
	stringMsg := NewMsg(0, "STRING_TEST", TEXT, NewMetadata(), "string data")
	assert.Equal(t, "string data", stringMsg.GetData())

	byteData := []byte("byte data")
	byteMsg := NewMsgFromBytes(0, "BYTE_TEST", BINARY, NewMetadata(), byteData)
	assert.Equal(t, string(byteData), string(byteMsg.GetBytes()))
	assert.Equal(t, string(byteData), byteMsg.GetData())

	stringMsg.SetBytes(byteData)
	assert.Equal(t, string(byteData), string(stringMsg.GetBytes()))

	jsonData := []byte(`{"key": "value"}`)
	jsonMsg := NewMsgWithJsonDataFromBytes(jsonData)
	assert.Equal(t, JSON, jsonMsg.DataType)
	assert.Equal(t, string(jsonData), string(jsonMsg.GetBytes()))
	assert.True(t, len(jsonMsg.Id) > 0)
}

// TestMemoryOptimization 测试内存优化和泄漏检测
func TestMemoryOptimization(t *testing.T) {
	// 大数据COW测试
	largeData := strings.Repeat("Large data test ", 1000)
	metadata := NewMetadata()
	for i := 0; i < 50; i++ {
		metadata.PutValue(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	original := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "memory-test",
		DataType: JSON,
		Type:     "MEMORY_TEST",
		Data:     NewSharedData(largeData),
		Metadata: metadata,
	}

	copies := make([]RuleMsg, 100)
	for i := 0; i < 100; i++ {
		copies[i] = original.Copy()
	}

	for i, copy := range copies {
		if copy.GetData() != largeData {
			t.Errorf("副本 %d 数据不正确", i)
		}
		if copy.Metadata.Len() != metadata.Len() {
			t.Errorf("副本 %d metadata长度不正确", i)
		}
	}

	for i := 0; i < 10; i++ {
		copies[i].SetData(fmt.Sprintf("modified %d", i))
		copies[i].Metadata.PutValue("modified", "true")
	}

	assert.Equal(t, largeData, original.GetData())
	assert.False(t, original.Metadata.Has("modified"))

	for i := 10; i < 20; i++ {
		assert.Equal(t, largeData, copies[i].GetData())
		assert.False(t, copies[i].Metadata.Has("modified"))
	}

	// 简化内存泄漏检测
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	for i := 0; i < 1000; i++ {
		msg := NewMsg(0, "LEAK_TEST", JSON, nil, "test data")
		copy1 := msg.Copy()
		copy2 := copy1.Copy()

		copy1.SetData("modified1")
		copy2.SetData("modified2")
		copy2.Metadata.PutValue("test", "value")

		sd := NewSharedData("shared data")
		sdCopy := sd.Copy()
		sdCopy.Set("modified shared")
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocDiff := int64(m2.Alloc - m1.Alloc)
	maxAcceptableGrowth := int64(1024 * 1024) // 1MB

	if allocDiff > maxAcceptableGrowth {
		t.Errorf("可能存在内存泄漏，内存增长: %d bytes", allocDiff)
	}
}
