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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// TestMetadataBasicOperations 测试Metadata的基本操作
func TestMetadataBasicOperations(t *testing.T) {
	md := NewMetadata()
	assert.Equal(t, 0, md.Len())

	// 测试基本操作
	md.PutValue("key1", "value1")
	md.PutValue("key2", "value2")
	assert.True(t, md.Has("key1"))
	assert.Equal(t, "value1", md.GetValue("key1"))
	assert.Equal(t, 2, md.Len())

	// 测试Values方法
	values := md.Values()
	assert.Equal(t, 2, len(values))
	assert.Equal(t, "value1", values["key1"])

	// 测试BuildMetadata
	data := map[string]string{"key3": "value3", "key4": "value4"}
	md2 := BuildMetadata(data)
	assert.Equal(t, 2, md2.Len())
	assert.Equal(t, "value3", md2.GetValue("key3"))
}

// TestMetadataCopyOnWrite 测试Copy-on-Write机制
func TestMetadataCopyOnWrite(t *testing.T) {
	original := NewMetadata()
	original.PutValue("key1", "value1")
	original.PutValue("key2", "value2")

	// 复制metadata
	copy1 := original.Copy()
	copy2 := original.Copy()

	// 验证初始状态
	assert.Equal(t, "value1", copy1.GetValue("key1"))
	assert.Equal(t, "value1", copy2.GetValue("key1"))

	// 修改copy1，不应该影响original和copy2
	copy1.PutValue("key1", "modified1")
	copy1.PutValue("key3", "new1")

	// 验证COW机制
	assert.Equal(t, "value1", original.GetValue("key1"))
	assert.Equal(t, "value1", copy2.GetValue("key1"))
	assert.Equal(t, "modified1", copy1.GetValue("key1"))
	assert.False(t, original.Has("key3"))
	assert.False(t, copy2.Has("key3"))
	assert.True(t, copy1.Has("key3"))

	// 测试ReplaceAll
	newData := map[string]string{"newKey1": "newValue1"}
	copy1.ReplaceAll(newData)
	assert.False(t, copy1.Has("key1"))
	assert.True(t, copy1.Has("newKey1"))
	assert.Equal(t, 1, copy1.Len())
}

// TestMetadataConcurrentAccess 测试并发访问安全性
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

			// 执行读写操作
			copy.PutValue("goroutine_key", "value")
			_ = copy.GetValue("key1")
			_ = copy.Has("key1")
			_ = copy.Values()
		}(i)
	}

	wg.Wait()

	// 验证原始metadata未被修改
	assert.Equal(t, "value1", original.GetValue("key1"))
	assert.Equal(t, 1, original.Len())
}

// TestRuleMsgBasicOperations 测试RuleMsg的基本操作
func TestRuleMsgBasicOperations(t *testing.T) {
	metadata := NewMetadata()
	metadata.PutValue("userId", "12345")

	msg := NewMsg(0, "TEST", JSON, metadata, `{"test": "data"}`)

	// 验证基本字段
	assert.True(t, len(msg.Id) > 0)
	assert.Equal(t, "TEST", msg.Type)
	assert.Equal(t, JSON, msg.DataType)
	assert.Equal(t, `{"test": "data"}`, msg.GetData())
	assert.Equal(t, "12345", msg.Metadata.GetValue("userId"))

	// 测试NewMsgFromBytes
	byteData := []byte("byte data")
	byteMsg := NewMsgFromBytes(12345, "BYTE_TEST", BINARY, metadata, byteData)
	assert.Equal(t, int64(12345), byteMsg.Ts)
	assert.Equal(t, "BYTE_TEST", byteMsg.Type)
	assert.Equal(t, BINARY, byteMsg.DataType)
	assert.Equal(t, string(byteData), string(byteMsg.GetBytes()))
}

// TestRuleMsgCopyMechanism 测试消息复制机制
func TestRuleMsgCopyMechanism(t *testing.T) {
	original := NewMsg(0, "TEST", JSON, nil, "original data")

	// 复制消息
	copy1 := original.Copy()
	copy2 := original.Copy()

	// 验证初始状态
	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "original data", copy1.GetData())
	assert.Equal(t, "original data", copy2.GetData())

	// 修改copy1的数据
	copy1.SetData("modified data 1")

	// 验证COW机制
	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "modified data 1", copy1.GetData())
	assert.Equal(t, "original data", copy2.GetData())

	// 修改copy2的数据
	copy2.SetData("modified data 2")

	// 验证所有消息数据独立
	assert.Equal(t, "original data", original.GetData())
	assert.Equal(t, "modified data 1", copy1.GetData())
	assert.Equal(t, "modified data 2", copy2.GetData())

	// 测试nil metadata的复制
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

	// 从字节数组创建
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

	// 测试SetBytes
	copy.SetBytes([]byte("new byte data"))
	assert.Equal(t, "new byte data", copy.Get())

	// 测试SetUnsafe
	copy.SetUnsafe("unsafe data")
	assert.Equal(t, "unsafe data", copy.GetUnsafe())

	// 测试空数据
	empty := NewSharedData("")
	assert.True(t, empty.IsEmpty())
	assert.Equal(t, 0, empty.Len())
}

// TestSharedDataMutableBytes 测试可修改字节数组功能
func TestSharedDataMutableBytes(t *testing.T) {
	sd := NewSharedData("Hello World")

	// 获取可修改的字节副本
	mutableBytes := sd.GetMutableBytes()
	mutableBytes[0] = 'h' // Hello -> hello

	// 原始数据不受影响
	assert.Equal(t, "Hello World", sd.Get())
	assert.Equal(t, "hello World", string(mutableBytes))

	// 应用修改
	sd.SetBytes(mutableBytes)
	assert.Equal(t, "hello World", sd.Get())
}

// TestRuleMsgZeroCopyAPI 测试零拷贝API
func TestRuleMsgZeroCopyAPI(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, NewMetadata(), "")

	// 测试零拷贝设置和获取
	testData := "Zero Copy Test Data"
	msg.SetData(testData)

	result := msg.GetData()
	assert.Equal(t, testData, result)
	assert.Equal(t, []byte(testData), msg.GetBytes())

	// 测试数据更新
	newData := "Updated Data"
	msg.SetData(newData)
	assert.Equal(t, newData, msg.GetData())

	// 测试SetBytes
	byteData := []byte("byte data")
	msg.SetBytes(byteData)
	assert.Equal(t, string(byteData), msg.GetData())
	assert.Equal(t, byteData, msg.GetBytes())
}

// TestRuleMsgSharedDataAccess 测试SharedData直接访问
func TestRuleMsgSharedDataAccess(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, NewMetadata(), "Original Data")

	// 获取SharedData实例
	sharedData := msg.GetSharedData()
	assert.NotNil(t, sharedData)
	assert.Equal(t, "Original Data", sharedData.Get())
	assert.Equal(t, int64(1), sharedData.GetRefCount())

	// 测试GetMutableBytes
	mutableBytes := sharedData.GetMutableBytes()
	mutableBytes[0] = 'M'                           // Original -> Mriginal
	assert.Equal(t, "Original Data", msg.GetData()) // 原始数据不受影响
	assert.Equal(t, "Mriginal Data", string(mutableBytes))

	// 应用修改
	sharedData.SetBytes(mutableBytes)
	assert.Equal(t, "Mriginal Data", msg.GetData())

	// 测试消息间数据共享
	msg2 := NewMsg(0, "TEST2", JSON, NewMetadata(), "")
	sharedDataCopy := sharedData.Copy()
	msg2.SetSharedData(sharedDataCopy)

	assert.Equal(t, "Mriginal Data", msg2.GetData())
	assert.Equal(t, int64(2), sharedData.GetRefCount())

	// 修改一个消息，验证COW
	msg2.SetData("Modified Data")
	assert.Equal(t, "Mriginal Data", msg.GetData())
	assert.Equal(t, "Modified Data", msg2.GetData())
	assert.Equal(t, int64(1), sharedData.GetRefCount())

	// 测试设置nil SharedData
	msg.SetSharedData(nil)
	newSharedData := msg.GetSharedData()
	assert.NotNil(t, newSharedData)
	assert.Equal(t, "", newSharedData.Get())
}

// TestRuleMsgJSONSerialization 测试JSON序列化
func TestRuleMsgJSONSerialization(t *testing.T) {
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

	// 序列化
	jsonData, err := json.Marshal(originalMsg)
	assert.Nil(t, err)

	// 验证JSON包含预期字段
	jsonStr := string(jsonData)
	assert.True(t, strings.Contains(jsonStr, `"ts":1640995200000`))
	assert.True(t, strings.Contains(jsonStr, `"id":"test-msg-id"`))
	assert.True(t, strings.Contains(jsonStr, `"type":"TEST_TYPE"`))
	assert.True(t, strings.Contains(jsonStr, `"key1":"value1"`))

	// 反序列化
	var deserializedMsg RuleMsg
	err = json.Unmarshal(jsonData, &deserializedMsg)
	assert.Nil(t, err)

	// 验证数据一致性
	assert.Equal(t, originalMsg.Ts, deserializedMsg.Ts)
	assert.Equal(t, originalMsg.Id, deserializedMsg.Id)
	assert.Equal(t, originalMsg.Type, deserializedMsg.Type)
	assert.Equal(t, originalMsg.GetData(), deserializedMsg.GetData())
	assert.Equal(t, "value1", deserializedMsg.Metadata.GetValue("key1"))
	assert.Equal(t, "12345", deserializedMsg.Metadata.GetValue("userId"))

	// 测试空metadata序列化
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

// TestRuleMsgJSONCache 测试JSON解析缓存
func TestRuleMsgJSONCache(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, nil, `{"test": "data"}`)

	// 初始状态下没有缓存
	assert.Nil(t, msg.parsedData)

	// 解析JSON数据
	jsonData, err := msg.GetAsJson()
	assert.Nil(t, err)
	assert.Equal(t, "data", jsonData["test"])
	assert.NotNil(t, msg.parsedData) // 缓存应该被创建

	// 再次获取应该使用缓存
	jsonData2, err := msg.GetAsJson()
	assert.Nil(t, err)
	assert.Equal(t, "data", jsonData2["test"])

	// 修改数据应该清除缓存
	msg.SetData(`{"test": "modified"}`)
	assert.Nil(t, msg.parsedData) // 缓存应该被清除

	// 重新解析应该得到新数据
	newJsonData, err := msg.GetAsJson()
	assert.Nil(t, err)
	assert.Equal(t, "modified", newJsonData["test"])

	// 使用SetBytes也应该触发回调
	msg.SetBytes([]byte(`{"test": "from_bytes"}`))
	assert.Nil(t, msg.parsedData)

	bytesJsonData, err := msg.GetAsJson()
	assert.Nil(t, err)
	assert.Equal(t, "from_bytes", bytesJsonData["test"])
}

// TestConcurrentDataAccess 测试并发数据访问
func TestConcurrentDataAccess(t *testing.T) {
	msg := NewMsg(0, "CONCURRENT_TEST", JSON, NewMetadata(), "Concurrent Test Data")
	originalSharedData := msg.GetSharedData()

	var wg sync.WaitGroup
	results := make([]string, 10)

	// 并发创建副本并修改
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

	// 验证原始数据不受影响
	assert.Equal(t, "Concurrent Test Data", msg.GetData())

	// 验证每个goroutine的结果
	for i, result := range results {
		expected := string(byte('A'+i)) + "oncurrent Test Data"
		assert.Equal(t, expected, result)
	}
}

// TestMemoryOptimization 测试内存优化
func TestMemoryOptimization(t *testing.T) {
	// 创建大数据消息
	largeData := strings.Repeat("Large data test ", 1000)
	metadata := NewMetadata()
	for i := 0; i < 50; i++ {
		metadata.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	original := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "memory-test",
		DataType: JSON,
		Type:     "MEMORY_TEST",
		Data:     NewSharedData(largeData),
		Metadata: metadata,
	}

	// 创建多个副本
	copies := make([]RuleMsg, 100)
	for i := 0; i < 100; i++ {
		copies[i] = original.Copy()
	}

	// 验证所有副本数据正确
	for i, copy := range copies {
		if copy.GetData() != largeData {
			t.Errorf("副本 %d 数据不正确", i)
		}
		if copy.Metadata.Len() != metadata.Len() {
			t.Errorf("副本 %d metadata长度不正确", i)
		}
	}

	// 修改部分副本
	for i := 0; i < 10; i++ {
		copies[i].SetData("modified " + string(rune('0'+i)))
		copies[i].Metadata.PutValue("modified", "true")
	}

	// 验证原始消息未被修改
	assert.Equal(t, largeData, original.GetData())
	assert.False(t, original.Metadata.Has("modified"))

	// 验证未修改的副本保持不变
	for i := 10; i < 20; i++ {
		assert.Equal(t, largeData, copies[i].GetData())
		assert.False(t, copies[i].Metadata.Has("modified"))
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	// 测试无效JSON
	msg := NewMsg(0, "ERROR_TEST", JSON, nil, "invalid json {")
	_, err := msg.GetAsJson()
	assert.NotNil(t, err)

	// 测试空数据JSON解析
	emptyMsg := NewMsg(0, "EMPTY_TEST", JSON, nil, "")
	emptyJson, err := emptyMsg.GetAsJson()
	assert.Nil(t, err)
	assert.Equal(t, 0, len(emptyJson))

	// 测试nil metadata
	copy := BuildMetadataFromMetadata(nil)
	assert.NotNil(t, copy)
	assert.Equal(t, 0, copy.Len())
}

// TestAPICompatibility 测试API兼容性
func TestAPICompatibility(t *testing.T) {
	// 测试string API
	stringMsg := NewMsg(0, "STRING_TEST", TEXT, NewMetadata(), "string data")
	assert.Equal(t, "string data", stringMsg.GetData())

	// 测试[]byte API
	byteData := []byte("byte data")
	byteMsg := NewMsgFromBytes(0, "BYTE_TEST", BINARY, NewMetadata(), byteData)
	assert.Equal(t, string(byteData), string(byteMsg.GetBytes()))

	// 测试类型转换
	assert.Equal(t, string(byteData), byteMsg.GetData())

	stringMsg.SetBytes(byteData)
	assert.Equal(t, string(byteData), string(stringMsg.GetBytes()))

	// 测试NewMsgWithJsonDataFromBytes
	jsonData := []byte(`{"key": "value"}`)
	jsonMsg := NewMsgWithJsonDataFromBytes(jsonData)
	assert.Equal(t, JSON, jsonMsg.DataType)
	assert.Equal(t, string(jsonData), string(jsonMsg.GetBytes()))
	assert.True(t, len(jsonMsg.Id) > 0)
}

// TestMemoryLeakage 简化的内存泄漏检测
func TestMemoryLeakage(t *testing.T) {
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 执行大量操作
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

	// 简单检测：当前分配内存不应该有显著增长
	allocDiff := int64(m2.Alloc - m1.Alloc)
	maxAcceptableGrowth := int64(1024 * 1024) // 1MB

	if allocDiff > maxAcceptableGrowth {
		t.Errorf("可能存在内存泄漏，内存增长: %d bytes", allocDiff)
	}
}

// BenchmarkMetadataCopy COW性能基准测试
func BenchmarkMetadataCopy(b *testing.B) {
	original := NewMetadata()
	for i := 0; i < 100; i++ {
		original.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			copy := original.Copy()
			_ = copy.GetValue("key1")
		}
	})
}

// BenchmarkSharedDataCopy SharedData COW性能基准测试
func BenchmarkSharedDataCopy(b *testing.B) {
	largeData := strings.Repeat("benchmark test data ", 1000)
	original := NewSharedData(largeData)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			copy := original.Copy()
			_ = copy.Get()
		}
	})
}

// BenchmarkRuleMsgCopy RuleMsg复制性能基准测试
func BenchmarkRuleMsgCopy(b *testing.B) {
	metadata := NewMetadata()
	for i := 0; i < 50; i++ {
		metadata.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	largeData := strings.Repeat("benchmark msg data ", 500)
	original := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "benchmark-msg",
		DataType: JSON,
		Type:     "BENCHMARK",
		Data:     NewSharedData(largeData),
		Metadata: metadata,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			copy := original.Copy()
			_ = copy.GetData()
		}
	})
}
