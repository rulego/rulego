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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// TestMetadataOperations Tests the basic operations and COW mechanisms of Metadata
func TestMetadataOperations(t *testing.T) {
	// Basic operations
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

	// COW mechanism testing
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

// TestMetadataConcurrentAccess Tests the security of Metadata concurrent access
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

// TestRuleMsgOperations tests the basic operations and replication mechanisms of RuleMsg
func TestRuleMsgOperations(t *testing.T) {
	// Basic operations
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

	// COW mechanism testing
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

	// nil metadata processing
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

// TestSharedDataOperations Tests the core features of SharedData
func TestSharedDataOperations(t *testing.T) {
	// Basic operations
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

	// COW Mechanism
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

	// Byte array tests can be modified
	msd := NewSharedData("Hello World")
	mutableBytes := msd.GetMutableBytes()
	mutableBytes[0] = 'h'
	assert.Equal(t, "Hello World", msd.Get())
	assert.Equal(t, "hello World", string(mutableBytes))
	msd.SetBytes(mutableBytes)
	assert.Equal(t, "hello World", msd.Get())
}

// TestRuleMsgZeroCopyAndSharedData tests the zero-copy API and SharedData access
func TestRuleMsgZeroCopyAndSharedData(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, NewMetadata(), "")

	// Zero-copy API testing
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

	// SharedData provides direct access to testing
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

	// Data sharing between messages
	msg3 := NewMsg(0, "TEST2", JSON, NewMetadata(), "")
	sharedDataCopy := sharedData.Copy()
	msg3.SetSharedData(sharedDataCopy)

	assert.Equal(t, "Mriginal Data", msg3.GetData())
	assert.Equal(t, int64(2), sharedData.GetRefCount())

	msg3.SetData("Modified Data")
	assert.Equal(t, "Mriginal Data", msg2.GetData())
	assert.Equal(t, "Modified Data", msg3.GetData())
	assert.Equal(t, int64(1), sharedData.GetRefCount())

	// nil SharedData processing
	msg2.SetSharedData(nil)
	newSharedData := msg2.GetSharedData()
	assert.NotNil(t, newSharedData)
	assert.Equal(t, "", newSharedData.Get())
}

// TestJSONSerialization Tests JSON serialization
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

// TestJSONParsingAndCache tests JSON parsing and caching mechanisms
func TestJSONParsingAndCache(t *testing.T) {
	// Basic JSON parsing
	msg := NewMsg(0, "TEST", JSON, nil, `{"test": "data"}`)
	jsonData1, err := msg.GetJsonData()
	assert.Nil(t, err)
	jsonMap1, ok := jsonData1.(map[string]interface{})
	assert.True(t, ok, "Expected JSON object")
	assert.Equal(t, "data", jsonMap1["test"])

	// Cache testing
	jsonData2, err := msg.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, jsonData1, jsonData2, "Second call should return cached result")

	// Data modifications should be cleared from the cache
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

	// JSON array support
	arrayMsg := NewMsg(0, "TEST", JSON, nil, `["apple", "banana", "cherry"]`)
	arrayData, err := arrayMsg.GetJsonData()
	assert.Nil(t, err)
	arraySlice, ok := arrayData.([]interface{})
	assert.True(t, ok, "Expected JSON array")
	assert.Equal(t, 3, len(arraySlice))
	assert.Equal(t, "apple", arraySlice[0])

	// Nested JSON
	nestedMsg := NewMsg(0, "TEST", JSON, nil, `[{"name": "Alice"}, {"name": "Bob"}]`)
	nestedData, err := nestedMsg.GetJsonData()
	assert.Nil(t, err)
	nestedSlice, ok := nestedData.([]interface{})
	assert.True(t, ok)
	assert.Equal(t, 2, len(nestedSlice))
	firstItem, ok := nestedSlice[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Alice", firstItem["name"])

	// SharedData-level caching
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

	// Commissioned testing
	jsonMsg := NewMsgWithJsonData(`{"delegated": true}`)
	msgData, err := jsonMsg.GetJsonData()
	assert.Nil(t, err)
	sharedData := jsonMsg.GetSharedData()
	sdData, err := sharedData.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, msgData, sdData)

	// nil data processing
	emptyMsg := RuleMsg{Id: "empty", Type: "test"}
	emptyData, err := emptyMsg.GetJsonData()
	assert.Nil(t, err)
	emptyMap, ok := emptyData.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 0, len(emptyMap))
}

// TestCacheSharedOnCopy: Tests cache sharing and COW protection during replication
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

	// Modifying the copy instance (triggering COW)
	copy.SetUnsafe(`{"original": false}`)
	originalData3, err := original.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData1, originalData3, "Original cache should remain after copy modification")

	newCopyData, err := copy.GetJsonData()
	assert.Nil(t, err)
	assert.NotEqual(t, originalData1, newCopyData, "Copy should have independent cache after modification")

	// Cache sharing testing under COW protection
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

// TestConcurrentOperations tests the security of concurrent operations
func TestConcurrentOperations(t *testing.T) {
	// RuleMsg concurrently accesses
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

	// Concurrent JSON parsing tests – Each goroutine uses an independent copy of the message
	baseJsonMsg := NewMsgWithJsonData(`{"temperature": 25.5, "humidity": 60}`)
	var wg2 sync.WaitGroup
	errors := make(chan error, 1000)

	// Concurrent reads – each goroutine uses an independent copy
	for i := 0; i < 50; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			msgCopy := baseJsonMsg.Copy() // Each goroutine uses an independent copy
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

	// Concurrent writes – each goroutine uses an independent copy
	for i := 0; i < 10; i++ {
		wg2.Add(1)
		go func(id int) {
			defer wg2.Done()
			msgCopy := baseJsonMsg.Copy() // Each goroutine uses an independent copy
			for j := 0; j < 3; j++ {
				newData := fmt.Sprintf(`{"temperature": %d, "humidity": 70}`, id)
				msgCopy.SetData(newData)
				// Verify that the data settings are successful
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
			// Passed the test
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// SharedData concurrent operation – Each goroutine uses an independent copy
	baseSd := NewSharedData(`{"counter": 0, "active": true}`)
	var wg3 sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg3.Add(1)
		go func() {
			defer wg3.Done()
			sdCopy := baseSd.Copy() // Each goroutine uses an independent copy
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
			sdCopy := baseSd.Copy() // Each goroutine uses an independent copy
			newData := fmt.Sprintf(`{"counter": %d, "active": true}`, id)
			sdCopy.SetUnsafe(newData)
			// Verify that the data settings are successful
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
		// Passed the test
	case <-time.After(5 * time.Second):
		t.Fatal("SharedData test timed out")
	}
}

// TestConcurrentModificationSafety Tests the security of multiple instances modifying concurrently
func TestConcurrentModificationSafety(t *testing.T) {
	original := NewSharedData(`{"concurrent": "test", "value": "100"}`)
	originalData, err := original.GetJsonData()
	assert.Nil(t, err)

	const numCopies = 10
	copies := make([]*SharedData, numCopies)
	for i := 0; i < numCopies; i++ {
		copies[i] = original.Copy()
	}

	// Verify shared cache
	for i, copy := range copies {
		copyData, err := copy.GetJsonData()
		assert.Nil(t, err)
		assert.Equal(t, originalData, copyData, "Copy %d should share cache", i)
	}

	var wg sync.WaitGroup
	errors := make(chan error, numCopies)
	results := make([]string, numCopies)

	// Concurrent modifications to all copies
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
			// Passed the test
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}

	// Validating independence
	for i, result := range results {
		expected := fmt.Sprintf("%d", i)
		assert.Equal(t, expected, result, "Copy %d should have independent value", i)
		assert.Equal(t, int64(1), copies[i].GetRefCount(), "Copy %d should have independent refCount", i)
	}

	// Verify that the original instance is unaffected
	originalDataAgain, err := original.GetJsonData()
	assert.Nil(t, err)
	assert.Equal(t, originalData, originalDataAgain, "Original should remain unchanged")
	assert.Equal(t, int64(1), original.GetRefCount(), "Original should have independent refCount")
}

// TestErrorHandling: Test error handling
func TestErrorHandling(t *testing.T) {
	// Invalid JSON
	msg := NewMsg(0, "ERROR_TEST", JSON, nil, "invalid json {")
	_, err := msg.GetJsonData()
	assert.NotNil(t, err)

	// Empty data JSON parsing
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

	// Fix after invalid JSON
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

// TestAPICompatibility Test API compatibility
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

// TestMemoryOptimization Testing memory optimization and leak detection
func TestMemoryOptimization(t *testing.T) {
	// Big data COW testing
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
			t.Errorf("Copy %d data is incorrect", i)
		}
		if copy.Metadata.Len() != metadata.Len() {
			t.Errorf("The copy %d metadata length is incorrect", i)
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

	// Simplified memory leak detection
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
		t.Errorf("There may be memory leaks, memory growth: %d bytes", allocDiff)
	}
}

// TestRuleMsgDataTypeJSONMarshaling: Test RuleMsg's JSON encoding under different data types
func TestRuleMsgDataTypeJSONMarshaling(t *testing.T) {
	metadata := NewMetadata()
	metadata.PutValue("deviceId", "sensor001")
	metadata.PutValue("location", "北京")

	// Test TEXT type messages
	textMsg := NewMsg(1640995200000, "TELEMETRY", TEXT, metadata, "Hello, 世界!")
	textMsgJson, err := json.Marshal(textMsg)
	assert.Nil(t, err)
	// Verify that TEXT data is properly encoded as a string
	assert.True(t, strings.Contains(string(textMsgJson), "Hello, 世界!"))
	assert.True(t, strings.Contains(string(textMsgJson), `"dataType":"TEXT"`))

	// Test JSON-type messages
	jsonData := `{"temperature": 25.5, "humidity": 60.0, "status": "正常"}`
	jsonMsg := NewMsg(1640995200000, "TELEMETRY", JSON, metadata, jsonData)
	jsonMsgJson, err := json.Marshal(jsonMsg)
	assert.Nil(t, err)
	// Verify that JSON data is encoded correctly
	assert.True(t, strings.Contains(string(jsonMsgJson), "temperature"))
	assert.True(t, strings.Contains(string(jsonMsgJson), `"dataType":"JSON"`))

	// Testing BINARY-type messages - using simulated image data
	binaryData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // JPEG file header
		0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, // JFIF
		0x80, 0x81, 0x82, 0x83, // Some binary bytes
		0xFF, 0xFF, 0xFE, 0xFD, // More binary bytes
		0x00, 0x01, 0x02, 0x03, // Control characters
	}
	binaryMsg := NewMsgFromBytes(1640995200000, "IMAGE_DATA", BINARY, metadata, binaryData)
	binaryMsgJson, err := json.Marshal(binaryMsg)
	assert.Nil(t, err)

	// Verify BINARY data using hexadecimal encoding
	expectedHex := hex.EncodeToString(binaryData)
	assert.True(t, strings.Contains(string(binaryMsgJson), expectedHex))
	assert.True(t, strings.Contains(string(binaryMsgJson), `"dataType":"BINARY"`))
	assert.True(t, strings.Contains(string(binaryMsgJson), `"type":"IMAGE_DATA"`))

	// Test contains invalid UTF-8 TEXT messages
	invalidUtf8Data := []byte{'H', 'e', 'l', 'l', 'o', 0xFF, 0xFE, 'W', 'o', 'r', 'l', 'd'}
	invalidUtf8Msg := NewMsgFromBytes(1640995200000, "CORRUPTED_TEXT", TEXT, metadata, invalidUtf8Data)
	invalidUtf8MsgJson, err := json.Marshal(invalidUtf8Msg)
	assert.Nil(t, err)

	// Verification of invalid UTF-8 TEXT data also uses hexadecimal encoding
	expectedInvalidHex := hex.EncodeToString(invalidUtf8Data)
	assert.True(t, strings.Contains(string(invalidUtf8MsgJson), expectedInvalidHex))
	assert.True(t, strings.Contains(string(invalidUtf8MsgJson), `"dataType":"TEXT"`))

	// Test the data type after message replication is maintained
	textMsgCopy := textMsg.Copy()
	textMsgCopy.SetData("Modified text data")
	textMsgCopyJson, err := json.Marshal(textMsgCopy)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(textMsgCopyJson), "Modified text data"))
	assert.True(t, strings.Contains(string(textMsgCopyJson), `"dataType":"TEXT"`))

	// Verify that the original message was not affected
	originalTextJson, err := json.Marshal(textMsg)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(originalTextJson), "Hello, 世界!"))

	// Test data settings across data types
	binaryMsgCopy := binaryMsg.Copy()
	binaryMsgCopy.SetData("Now it's text data")
	binaryMsgCopyJson, err := json.Marshal(binaryMsgCopy)
	assert.Nil(t, err)
	// The data type is still BINARY, but now it contains text data and should use hexadecimal encoding
	textDataHex := hex.EncodeToString([]byte("Now it's text data"))
	assert.True(t, strings.Contains(string(binaryMsgCopyJson), textDataHex))
	assert.True(t, strings.Contains(string(binaryMsgCopyJson), `"dataType":"BINARY"`))

	// Test JSON deserialization
	var deserializedTextMsg RuleMsg
	err = json.Unmarshal(textMsgJson, &deserializedTextMsg)
	assert.Nil(t, err)
	assert.Equal(t, textMsg.Id, deserializedTextMsg.Id)
	assert.Equal(t, textMsg.Type, deserializedTextMsg.Type)
	assert.Equal(t, textMsg.DataType, deserializedTextMsg.DataType)
	assert.Equal(t, textMsg.GetData(), deserializedTextMsg.GetData())
	assert.Equal(t, textMsg.Metadata.GetValue("deviceId"), deserializedTextMsg.Metadata.GetValue("deviceId"))

	// Demonstrate data integrity
	decodedData, err := hex.DecodeString(expectedHex)
	assert.Nil(t, err)
	assert.Equal(t, binaryData, decodedData)
}

// TestSharedDataJSONRoundTrip: Fixes SharedDataJSON's serialization/deserialization round trip
// This test is used to verify the bug previously fixed: UnmarshalJSON correctly handles hex encoding data generated by MarshalJSON
func TestSharedDataJSONRoundTrip(t *testing.T) {
	// Test 1: Round-trip serialization of BINARY data types
	t.Run("BINARY data round-trip", func(t *testing.T) {
		// Create SharedData containing binary data
		binaryData := []byte{0x01, 0x02, 0xFF, 0xFE, 0x80, 0x7F, 0x00, 0x0A, 0x0D, 0x1F}
		original := NewSharedDataFromBytesWithType(binaryData, BINARY)

		// Verify the original data
		assert.Equal(t, BINARY, original.dataType)
		assert.Equal(t, binaryData, original.GetBytes())
		assert.Equal(t, string(binaryData), original.Get())

		// JSON serialization
		jsonData, err := json.Marshal(original)
		assert.Nil(t, err)

		// Verify that serialization results include hex encoding
		expectedHex := hex.EncodeToString(binaryData)
		jsonStr := string(jsonData)
		assert.True(t, strings.Contains(jsonStr, expectedHex), "JSON should contain hex-encoded data")

		// JSON deserialization
		var deserialized SharedData
		err = json.Unmarshal(jsonData, &deserialized)
		assert.Nil(t, err)

		// Verify the correctness of the deserialized data
		assert.Equal(t, binaryData, deserialized.GetBytes(), "Binary data should be correctly decoded from hex")
		assert.Equal(t, BINARY, deserialized.dataType, "DataType should be inferred as BINARY")
		assert.Equal(t, original.Get(), deserialized.Get(), "String representation should match")
		assert.Equal(t, int64(1), deserialized.GetRefCount(), "RefCount should be initialized")
	})

	// Test 2: Serialization of TEXT data containing invalid UTF-8 data for round trips
	t.Run("Invalid UTF-8 TEXT data round-trip", func(t *testing.T) {
		// Create TEXT data containing invalid UTF-8 bytes
		invalidUtf8Data := []byte{'H', 'e', 'l', 'l', 'o', 0xFF, 0xFE, 'W', 'o', 'r', 'l', 'd'}
		original := NewSharedDataFromBytesWithType(invalidUtf8Data, TEXT)

		// Verify the original data
		assert.Equal(t, TEXT, original.dataType)
		assert.Equal(t, invalidUtf8Data, original.GetBytes())

		// JSON serialization
		jsonData, err := json.Marshal(original)
		assert.Nil(t, err)

		// Verify that serialization results contain hex encoding (because they contain invalid UTF-8)
		expectedHex := hex.EncodeToString(invalidUtf8Data)
		jsonStr := string(jsonData)
		assert.True(t, strings.Contains(jsonStr, expectedHex), "Invalid UTF-8 should be hex-encoded")

		// JSON deserialization
		var deserialized SharedData
		err = json.Unmarshal(jsonData, &deserialized)
		assert.Nil(t, err)

		// Verify the correctness of the deserialized data
		assert.Equal(t, invalidUtf8Data, deserialized.GetBytes(), "Invalid UTF-8 data should be correctly decoded")
		assert.Equal(t, BINARY, deserialized.dataType, "DataType should be inferred as BINARY for invalid UTF-8")
	})

	// Test 3: Reciprocating and serializing TEXT data for valid UTF-8
	t.Run("Valid UTF-8 TEXT data round-trip", func(t *testing.T) {
		textData := "Hello, 世界! This is valid UTF-8 text."
		original := NewSharedDataWithType(textData, TEXT)

		// Verify the original data
		assert.Equal(t, TEXT, original.dataType)
		assert.Equal(t, textData, original.Get())

		// JSON serialization
		jsonData, err := json.Marshal(original)
		assert.Nil(t, err)

		// Verify serialization results directly containing text (no hex encoding required)
		jsonStr := string(jsonData)
		assert.True(t, strings.Contains(jsonStr, textData), "Valid UTF-8 should be stored as plain text")

		// JSON deserialization
		var deserialized SharedData
		err = json.Unmarshal(jsonData, &deserialized)
		assert.Nil(t, err)

		// Verify the correctness of the deserialized data
		assert.Equal(t, textData, deserialized.Get(), "Text data should be preserved")
		assert.Equal(t, TEXT, deserialized.dataType, "DataType should be inferred as TEXT")
	})

	// Test 4: Serialization of JSON data round-trip
	t.Run("JSON data round-trip", func(t *testing.T) {
		jsonData := `{"name": "Alice", "age": 30, "active": true}`
		original := NewSharedDataWithType(jsonData, JSON)

		// Verify the original data
		assert.Equal(t, JSON, original.dataType)
		assert.Equal(t, jsonData, original.Get())

		// JSON serialization
		serializedData, err := json.Marshal(original)
		assert.Nil(t, err)

		// JSON deserialization
		var deserialized SharedData
		err = json.Unmarshal(serializedData, &deserialized)
		assert.Nil(t, err)

		// Verify the correctness of the deserialized data
		assert.Equal(t, jsonData, deserialized.Get(), "JSON data should be preserved")
		assert.Equal(t, JSON, deserialized.dataType, "DataType should be inferred as JSON")

		// Verify that the JSON parsing function is normal
		parsedData, err := deserialized.GetJsonData()
		assert.Nil(t, err)
		parsedMap, ok := parsedData.(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Alice", parsedMap["name"])
		assert.Equal(t, float64(30), parsedMap["age"])
	})

	// Test 5: hex detects the boundary state of heuristic algorithms
	t.Run("Hex detection heuristics", func(t *testing.T) {
		// Short valid hex strings (should be recognized as plain text)
		shortHex := "cafe"
		sd1 := NewSharedDataWithType(shortHex, TEXT)
		jsonData1, _ := json.Marshal(sd1)

		var deserialized1 SharedData
		json.Unmarshal(jsonData1, &deserialized1)
		assert.Equal(t, shortHex, deserialized1.Get(), "Short hex-like string should be treated as text")
		assert.Equal(t, TEXT, deserialized1.dataType, "Should be inferred as TEXT")

		// Long valid hex strings (should be recognized as hex encoding)
		longHex := "0123456789abcdef0123456789abcdef"
		sd2 := NewSharedData(longHex)
		jsonData2, _ := json.Marshal(sd2)

		var deserialized2 SharedData
		json.Unmarshal(jsonData2, &deserialized2)
		// This should be decoded into binary data
		expectedBytes, _ := hex.DecodeString(longHex)
		assert.Equal(t, expectedBytes, deserialized2.GetBytes(), "Long hex string should be decoded")
		assert.Equal(t, BINARY, deserialized2.dataType, "Should be inferred as BINARY")

		// Odd-length hex string (not a valid hex)
		oddHex := "123456789abcde1" // 15 characters, odd length
		sd3 := NewSharedDataWithType(oddHex, TEXT)
		jsonData3, _ := json.Marshal(sd3)

		var deserialized3 SharedData
		json.Unmarshal(jsonData3, &deserialized3)
		assert.Equal(t, oddHex, deserialized3.Get(), "Odd length hex should be treated as text")
		assert.Equal(t, TEXT, deserialized3.dataType, "Should be inferred as TEXT")

		// Strings containing non-hex characters
		nonHex := "123456789abcdefg"
		sd4 := NewSharedDataWithType(nonHex, TEXT)
		jsonData4, _ := json.Marshal(sd4)

		var deserialized4 SharedData
		json.Unmarshal(jsonData4, &deserialized4)
		assert.Equal(t, nonHex, deserialized4.Get(), "Non-hex string should be treated as text")
		assert.Equal(t, TEXT, deserialized4.dataType, "Should be inferred as TEXT")
	})

	// Test 6: Integration test with RuleMsg
	t.Run("Integration with RuleMsg", func(t *testing.T) {
		// Create a RuleMsg containing binary data
		binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG file header
		metadata := NewMetadata()
		metadata.PutValue("fileType", "png")

		originalMsg := NewMsgFromBytes(12345, "IMAGE_UPLOAD", BINARY, metadata, binaryData)

		// Serialize the entire RuleMsg
		msgJsonData, err := json.Marshal(originalMsg)
		assert.Nil(t, err)

		// Deserialization of RuleMsg
		var deserializedMsg RuleMsg
		err = json.Unmarshal(msgJsonData, &deserializedMsg)
		assert.Nil(t, err)

		// Verify that all fields are correctly restored
		assert.Equal(t, originalMsg.Id, deserializedMsg.Id)
		assert.Equal(t, originalMsg.Type, deserializedMsg.Type)
		assert.Equal(t, originalMsg.DataType, deserializedMsg.DataType)
		assert.Equal(t, binaryData, deserializedMsg.GetBytes(), "Binary data should be preserved in RuleMsg")
		assert.Equal(t, "png", deserializedMsg.Metadata.GetValue("fileType"))

		// Verify that SharedData's dataType is inferred correctly
		// Note: In RuleMsg, the dataType field is serialized individually, but the DataType of SharedData needs to be inferred
		sharedData := deserializedMsg.GetSharedData()
		// Since the original data is binary, SharedData should be inferred to be BINARY type
		assert.Equal(t, BINARY, sharedData.dataType, "SharedData should infer BINARY type")
	})
}
