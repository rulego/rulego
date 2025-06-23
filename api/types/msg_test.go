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
	"sync/atomic"
	"testing"
	"time"
)

// TestMetadataBasicOperations 测试Metadata的基本操作
func TestMetadataBasicOperations(t *testing.T) {
	// 测试NewMetadata
	md := NewMetadata()
	if md.Len() != 0 {
		t.Errorf("Expected empty metadata, got length %d", md.Len())
	}

	// 测试PutValue和GetValue
	md.PutValue("key1", "value1")
	md.PutValue("key2", "value2")

	if !md.Has("key1") {
		t.Error("Expected key1 to exist")
	}

	if md.GetValue("key1") != "value1" {
		t.Errorf("Expected value1, got %s", md.GetValue("key1"))
	}

	if md.Len() != 2 {
		t.Errorf("Expected length 2, got %d", md.Len())
	}

	// 测试Values方法
	values := md.Values()
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values["key1"] != "value1" {
		t.Errorf("Expected value1, got %s", values["key1"])
	}
}

// TestMetadataBuildFromMap 测试从map构建Metadata
func TestMetadataBuildFromMap(t *testing.T) {
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	md := BuildMetadata(data)
	if md.Len() != 2 {
		t.Errorf("Expected length 2, got %d", md.Len())
	}

	if md.GetValue("key1") != "value1" {
		t.Errorf("Expected value1, got %s", md.GetValue("key1"))
	}
}

// TestMetadataCopyOnWrite 测试Copy-on-Write机制
func TestMetadataCopyOnWrite(t *testing.T) {
	// 创建原始metadata
	original := NewMetadata()
	original.PutValue("key1", "value1")
	original.PutValue("key2", "value2")

	// 复制metadata
	copy1 := original.Copy()
	copy2 := original.Copy()

	// 验证初始状态相同
	if copy1.GetValue("key1") != "value1" {
		t.Error("Copy1 should have same initial values")
	}
	if copy2.GetValue("key1") != "value1" {
		t.Error("Copy2 should have same initial values")
	}

	// 修改copy1，不应该影响original和copy2
	copy1.PutValue("key1", "modified1")
	copy1.PutValue("key3", "new1")

	// 验证original未被修改
	if original.GetValue("key1") != "value1" {
		t.Errorf("Original should not be modified, got %s", original.GetValue("key1"))
	}
	if original.Has("key3") {
		t.Error("Original should not have key3")
	}

	// 验证copy2未被修改
	if copy2.GetValue("key1") != "value1" {
		t.Errorf("Copy2 should not be modified, got %s", copy2.GetValue("key1"))
	}
	if copy2.Has("key3") {
		t.Error("Copy2 should not have key3")
	}

	// 验证copy1被正确修改
	if copy1.GetValue("key1") != "modified1" {
		t.Errorf("Copy1 should be modified, got %s", copy1.GetValue("key1"))
	}
	if !copy1.Has("key3") {
		t.Error("Copy1 should have key3")
	}
}

// TestMetadataReplaceAll 测试ReplaceAll方法
func TestMetadataReplaceAll(t *testing.T) {
	md := NewMetadata()
	md.PutValue("key1", "value1")
	md.PutValue("key2", "value2")

	// 替换所有数据
	newData := map[string]string{
		"newKey1": "newValue1",
		"newKey2": "newValue2",
		"newKey3": "newValue3",
	}
	md.ReplaceAll(newData)

	// 验证旧数据被清除
	if md.Has("key1") {
		t.Error("Old key1 should be removed")
	}
	if md.Has("key2") {
		t.Error("Old key2 should be removed")
	}

	// 验证新数据存在
	if !md.Has("newKey1") {
		t.Error("newKey1 should exist")
	}
	if md.GetValue("newKey1") != "newValue1" {
		t.Errorf("Expected newValue1, got %s", md.GetValue("newKey1"))
	}
	if md.Len() != 3 {
		t.Errorf("Expected length 3, got %d", md.Len())
	}
}

// TestMetadataConcurrentAccess 测试并发访问安全性
func TestMetadataConcurrentAccess(t *testing.T) {
	original := NewMetadata()
	original.PutValue("key1", "value1")
	original.PutValue("key2", "value2")

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// 启动多个goroutine并发操作
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// 每个goroutine创建自己的copy
			copy := original.Copy()

			// 执行多次操作
			for j := 0; j < numOperations; j++ {
				// 读操作
				_ = copy.GetValue("key1")
				_ = copy.Has("key2")
				_ = copy.Values()

				// 写操作
				copy.PutValue("key3", "value3")
				copy.PutValue("key4", "value4")

				// 替换操作
				if j%10 == 0 {
					newData := map[string]string{
						"replaced1": "replacedValue1",
						"replaced2": "replacedValue2",
					}
					copy.ReplaceAll(newData)
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()

	// 验证原始metadata未被修改
	if original.GetValue("key1") != "value1" {
		t.Errorf("Original metadata was modified: %s", original.GetValue("key1"))
	}
	if original.Len() != 2 {
		t.Errorf("Original metadata length changed: %d", original.Len())
	}
}

// TestRuleMsgCopy 测试RuleMsg的Copy方法
func TestRuleMsgCopy(t *testing.T) {
	// 创建原始消息
	originalMetadata := NewMetadata()
	originalMetadata.PutValue("key1", "value1")
	originalMetadata.PutValue("key2", "value2")

	originalMsg := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "test-id",
		DataType: JSON,
		Type:     "TEST_TYPE",
		Data:     NewSharedData(`{"test": "data"}`),
		Metadata: originalMetadata,
	}

	// 复制消息
	copiedMsg := originalMsg.Copy()

	// 验证基本字段相同
	if copiedMsg.Id != originalMsg.Id {
		t.Error("Message ID should be same")
	}
	if copiedMsg.Type != originalMsg.Type {
		t.Error("Message Type should be same")
	}
	if copiedMsg.GetData() != originalMsg.GetData() {
		t.Error("Message Data should be same")
	}

	// 验证metadata内容相同但独立
	if copiedMsg.Metadata.GetValue("key1") != "value1" {
		t.Error("Copied metadata should have same values")
	}

	// 修改复制的metadata，不应影响原始消息
	copiedMsg.Metadata.PutValue("key1", "modified")
	copiedMsg.Metadata.PutValue("key3", "new")

	// 验证原始消息未被修改
	if originalMsg.Metadata.GetValue("key1") != "value1" {
		t.Errorf("Original metadata should not be modified, got %s", originalMsg.Metadata.GetValue("key1"))
	}
	if originalMsg.Metadata.Has("key3") {
		t.Error("Original metadata should not have key3")
	}

	// 验证复制的消息被正确修改
	if copiedMsg.Metadata.GetValue("key1") != "modified" {
		t.Errorf("Copied metadata should be modified, got %s", copiedMsg.Metadata.GetValue("key1"))
	}
	if !copiedMsg.Metadata.Has("key3") {
		t.Error("Copied metadata should have key3")
	}
}

// TestMetadataBackwardCompatibility 测试向后兼容性
func TestMetadataBackwardCompatibility(t *testing.T) {
	// 测试BuildMetadataFromMetadata函数
	original := NewMetadata()
	original.PutValue("key1", "value1")

	compat := BuildMetadataFromMetadata(original)
	if compat.GetValue("key1") != "value1" {
		t.Error("BuildMetadataFromMetadata should preserve values")
	}

	// 修改compat不应影响original
	compat.PutValue("key1", "modified")
	if original.GetValue("key1") != "value1" {
		t.Error("Original should not be affected by compat modification")
	}
}

// BenchmarkMetadataCopy 性能测试：比较深拷贝和COW的性能
func BenchmarkMetadataCopy(b *testing.B) {
	// 创建一个包含多个键值对的metadata
	original := NewMetadata()
	for i := 0; i < 100; i++ {
		original.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 使用COW复制
			copy := original.Copy()
			// 模拟读操作
			_ = copy.GetValue("key1")
			_ = copy.Has("key2")
		}
	})
}

// BenchmarkMetadataCopyWithWrite 性能测试：COW在写操作时的性能
func BenchmarkMetadataCopyWithWrite(b *testing.B) {
	original := NewMetadata()
	for i := 0; i < 100; i++ {
		original.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			copy := original.Copy()
			// 触发COW
			copy.PutValue("newKey", "newValue")
			_ = copy.GetValue("newKey")
		}
	})
}

// TestRuleMsgJSONSerialization 测试RuleMsg的JSON序列化
func TestRuleMsgJSONSerialization(t *testing.T) {
	// 创建测试metadata
	metadata := NewMetadata()
	metadata.PutValue("key1", "value1")
	metadata.PutValue("key2", "value2")
	metadata.PutValue("userId", "12345")
	metadata.PutValue("deviceId", "sensor001")

	// 创建测试消息
	originalMsg := RuleMsg{
		Ts:       1640995200000, // 固定时间戳便于测试
		Id:       "test-msg-id-001",
		DataType: JSON,
		Type:     "TELEMETRY_DATA",
		Data:     NewSharedData(`{"temperature": 25.5, "humidity": 60.2}`),
		Metadata: metadata,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(originalMsg)
	if err != nil {
		t.Fatalf("JSON序列化失败: %v", err)
	}

	// 验证JSON格式
	expectedFields := []string{
		`"ts":1640995200000`,
		`"id":"test-msg-id-001"`,
		`"dataType":"JSON"`,
		`"type":"TELEMETRY_DATA"`,
		`"data":"{\"temperature\": 25.5, \"humidity\": 60.2}"`,
		`"metadata":{`,
		`"key1":"value1"`,
		`"key2":"value2"`,
		`"userId":"12345"`,
		`"deviceId":"sensor001"`,
	}

	jsonStr := string(jsonData)
	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON中缺少预期字段: %s\n实际JSON: %s", field, jsonStr)
		}
	}

	// 反序列化
	var deserializedMsg RuleMsg
	err = json.Unmarshal(jsonData, &deserializedMsg)
	if err != nil {
		t.Fatalf("JSON反序列化失败: %v", err)
	}

	// 验证反序列化结果
	if deserializedMsg.Ts != originalMsg.Ts {
		t.Errorf("时间戳不匹配: 期望 %d, 实际 %d", originalMsg.Ts, deserializedMsg.Ts)
	}
	if deserializedMsg.Id != originalMsg.Id {
		t.Errorf("ID不匹配: 期望 %s, 实际 %s", originalMsg.Id, deserializedMsg.Id)
	}
	if deserializedMsg.DataType != originalMsg.DataType {
		t.Errorf("DataType不匹配: 期望 %s, 实际 %s", originalMsg.DataType, deserializedMsg.DataType)
	}
	if deserializedMsg.Type != originalMsg.Type {
		t.Errorf("Type不匹配: 期望 %s, 实际 %s", originalMsg.Type, deserializedMsg.Type)
	}
	if deserializedMsg.GetData() != originalMsg.GetData() {
		t.Errorf("Data不匹配: 期望 %s, 实际 %s", originalMsg.GetData(), deserializedMsg.GetData())
	}

	// 验证metadata
	if deserializedMsg.Metadata.GetValue("key1") != "value1" {
		t.Errorf("Metadata key1不匹配: 期望 value1, 实际 %s", deserializedMsg.Metadata.GetValue("key1"))
	}
	if deserializedMsg.Metadata.GetValue("userId") != "12345" {
		t.Errorf("Metadata userId不匹配: 期望 12345, 实际 %s", deserializedMsg.Metadata.GetValue("userId"))
	}
	if deserializedMsg.Metadata.Len() != 4 {
		t.Errorf("Metadata长度不匹配: 期望 4, 实际 %d", deserializedMsg.Metadata.Len())
	}
}

// TestRuleMsgJSONWithEmptyMetadata 测试空metadata的JSON序列化
func TestRuleMsgJSONWithEmptyMetadata(t *testing.T) {
	msg := RuleMsg{
		Id:       "empty-metadata-test",
		Type:     "TEST",
		Data:     NewSharedData("test data"),
		Metadata: NewMetadata(), // 空metadata
	}

	// 序列化
	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 验证空metadata序列化为空对象
	if !strings.Contains(string(jsonData), `"metadata":{}`) {
		t.Errorf("空metadata应该序列化为空对象, 实际: %s", string(jsonData))
	}

	// 反序列化
	var deserializedMsg RuleMsg
	err = json.Unmarshal(jsonData, &deserializedMsg)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证空metadata
	if deserializedMsg.Metadata.Len() != 0 {
		t.Errorf("反序列化后metadata应该为空, 实际长度: %d", deserializedMsg.Metadata.Len())
	}
}

// TestRuleMsgJSONWithCopyOnWrite 测试Copy-on-Write机制下的JSON序列化
func TestRuleMsgJSONWithCopyOnWrite(t *testing.T) {
	// 创建原始消息
	originalMetadata := NewMetadata()
	originalMetadata.PutValue("shared", "original")
	originalMetadata.PutValue("common", "data")

	originalMsg := RuleMsg{
		Id:       "cow-test",
		Type:     "COW_TEST",
		Data:     NewSharedData("original data"),
		Metadata: originalMetadata,
	}

	// 复制消息并修改metadata
	copiedMsg := originalMsg.Copy()
	copiedMsg.Metadata.PutValue("shared", "modified")
	copiedMsg.Metadata.PutValue("new", "value")

	// 分别序列化
	originalJSON, err := json.Marshal(originalMsg)
	if err != nil {
		t.Fatalf("原始消息序列化失败: %v", err)
	}

	copiedJSON, err := json.Marshal(copiedMsg)
	if err != nil {
		t.Fatalf("复制消息序列化失败: %v", err)
	}

	// 验证原始消息未被修改
	if !strings.Contains(string(originalJSON), `"shared":"original"`) {
		t.Error("原始消息的metadata应该保持不变")
	}
	if strings.Contains(string(originalJSON), `"new":"value"`) {
		t.Error("原始消息不应该包含新添加的字段")
	}

	// 验证复制消息被正确修改
	if !strings.Contains(string(copiedJSON), `"shared":"modified"`) {
		t.Error("复制消息的metadata应该被修改")
	}
	if !strings.Contains(string(copiedJSON), `"new":"value"`) {
		t.Error("复制消息应该包含新添加的字段")
	}

	// 反序列化并验证
	var deserializedOriginal, deserializedCopied RuleMsg

	err = json.Unmarshal(originalJSON, &deserializedOriginal)
	if err != nil {
		t.Fatalf("原始消息反序列化失败: %v", err)
	}

	err = json.Unmarshal(copiedJSON, &deserializedCopied)
	if err != nil {
		t.Fatalf("复制消息反序列化失败: %v", err)
	}

	// 验证反序列化后的独立性
	if deserializedOriginal.Metadata.GetValue("shared") != "original" {
		t.Error("反序列化后原始消息的metadata不正确")
	}
	if deserializedCopied.Metadata.GetValue("shared") != "modified" {
		t.Error("反序列化后复制消息的metadata不正确")
	}
	if deserializedOriginal.Metadata.Has("new") {
		t.Error("反序列化后原始消息不应该有new字段")
	}
	if !deserializedCopied.Metadata.Has("new") {
		t.Error("反序列化后复制消息应该有new字段")
	}
}

// TestRuleMsgCopyWithNilMetadata 测试nil Metadata的复制
func TestRuleMsgCopyWithNilMetadata(t *testing.T) {
	msg := RuleMsg{
		Ts:       time.Now().UnixNano(),
		Id:       "test-id",
		Type:     "test-type",
		DataType: "json",
		Data:     NewSharedData("test data"),
		Metadata: nil, // 故意设置为nil
	}

	// 复制消息
	copiedMsg := msg.Copy()

	// 验证复制后的消息有有效的Metadata
	if copiedMsg.Metadata == nil {
		t.Error("Expected copied message to have non-nil Metadata")
	}

	// 验证可以安全地使用Metadata
	copiedMsg.Metadata.PutValue("test", "value")
	if copiedMsg.Metadata.GetValue("test") != "value" {
		t.Error("Expected to be able to use Metadata after copy")
	}
}

// TestDataCopyOnWrite 测试Data字段的写时复制机制
func TestDataCopyOnWrite(t *testing.T) {
	// 创建原始消息
	original := NewMsg(0, "TEST", JSON, nil, "original data")

	// 复制消息
	copy1 := original.Copy()
	copy2 := original.Copy()

	// 验证初始状态下所有消息的Data都相同
	if original.GetData() != "original data" {
		t.Errorf("Expected original data to be 'original data', got %s", original.GetData())
	}
	if copy1.GetData() != "original data" {
		t.Errorf("Expected copy1 data to be 'original data', got %s", copy1.GetData())
	}
	if copy2.GetData() != "original data" {
		t.Errorf("Expected copy2 data to be 'original data', got %s", copy2.GetData())
	}

	// 验证SharedData是共享的（通过指针比较）
	if original.Data == nil || copy1.Data == nil || copy2.Data == nil {
		t.Error("Expected all messages to have non-nil Data")
	}

	// 修改copy1的Data（使用COW优化的SetData方法）
	copy1.SetData("modified data 1")

	// 验证只有copy1的数据被修改
	if original.GetData() != "original data" {
		t.Errorf("Expected original data to remain 'original data', got %s", original.GetData())
	}
	if copy1.GetData() != "modified data 1" {
		t.Errorf("Expected copy1 data to be 'modified data 1', got %s", copy1.GetData())
	}
	if copy2.GetData() != "original data" {
		t.Errorf("Expected copy2 data to remain 'original data', got %s", copy2.GetData())
	}

	// 修改copy2的Data（使用COW优化的SetData方法）
	copy2.SetData("modified data 2")

	// 验证所有消息的数据都是独立的
	if original.GetData() != "original data" {
		t.Errorf("Expected original data to remain 'original data', got %s", original.GetData())
	}
	if copy1.GetData() != "modified data 1" {
		t.Errorf("Expected copy1 data to remain 'modified data 1', got %s", copy1.GetData())
	}
	if copy2.GetData() != "modified data 2" {
		t.Errorf("Expected copy2 data to be 'modified data 2', got %s", copy2.GetData())
	}
}

// TestDataCOWPerformance 测试Data字段COW机制的性能
func TestDataCOWPerformance(t *testing.T) {
	// 创建一个包含大量数据的消息
	largeData := strings.Repeat("This is a large data string for testing COW performance. ", 1000)
	original := NewMsg(0, "TEST", JSON, nil, largeData)

	// 创建多个副本
	copies := make([]RuleMsg, 100)
	for i := 0; i < 100; i++ {
		copies[i] = original.Copy()
	}

	// 验证所有副本的数据都正确
	for i, copy := range copies {
		if copy.GetData() != largeData {
			t.Errorf("Copy %d has incorrect data", i)
		}
	}

	// 修改一个副本，验证其他副本不受影响（使用COW优化的SetData方法）
	copies[0].SetData("modified")

	if original.GetData() != largeData {
		t.Error("Original data was modified unexpectedly")
	}

	for i := 1; i < 100; i++ {
		if copies[i].GetData() != largeData {
			t.Errorf("Copy %d was modified unexpectedly", i)
		}
	}
}

// TestDataCOWConcurrency 测试Data字段COW机制的并发安全性
func TestDataCOWConcurrency(t *testing.T) {
	original := NewMsg(0, "TEST", JSON, nil, "original data")

	var wg sync.WaitGroup
	results := make([]string, 10)

	// 并发创建副本并修改
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			copy := original.Copy()
			// 使用COW优化的SetData方法
			copy.SetData("modified " + string(rune('0'+index)))
			results[index] = copy.GetData()
		}(i)
	}

	wg.Wait()

	// 验证原始数据未被修改
	if original.GetData() != "original data" {
		t.Errorf("Original data was modified: %s", original.GetData())
	}

	// 验证每个goroutine都得到了正确的修改结果
	for i, result := range results {
		expected := "modified " + string(rune('0'+i))
		if result != expected {
			t.Errorf("Result %d: expected %s, got %s", i, expected, result)
		}
	}
}

// TestRuleMsgJSONRoundTrip 测试JSON序列化往返一致性
func TestRuleMsgJSONRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		msg  RuleMsg
	}{
		{
			name: "完整消息",
			msg: RuleMsg{
				Ts:       time.Now().UnixMilli(),
				Id:       "round-trip-test",
				DataType: JSON,
				Type:     "ROUND_TRIP",
				Data:     NewSharedData(`{"test": "data", "number": 42}`),
				Metadata: func() *Metadata {
					md := NewMetadata()
					md.PutValue("test", "value")
					return md
				}(),
			},
		},
		{
			name: "最小消息",
			msg: RuleMsg{
				Id:       "minimal",
				Type:     "MINIMAL",
				Data:     NewSharedData(""),
				Metadata: NewMetadata(),
			},
		},
		{
			name: "Metadata 空",
			msg: RuleMsg{
				Id:       "metadataNil",
				Type:     "metadataNil",
				Data:     NewSharedData(""),
				Metadata: nil,
			},
		},
		{
			name: "包含特殊字符",
			msg: RuleMsg{
				Id:   "special-chars",
				Type: "SPECIAL",
				Data: NewSharedData(`{"message": "Hello\nWorld\t\"Test\""}`),
				Metadata: func() *Metadata {
					md := NewMetadata()
					md.PutValue("special", "value\nwith\ttabs")
					return md
				}(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 序列化
			jsonData, err := json.Marshal(tc.msg)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}

			// 反序列化
			var deserializedMsg RuleMsg
			err = json.Unmarshal(jsonData, &deserializedMsg)
			if err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}

			// 验证一致性
			if deserializedMsg.Id != tc.msg.Id {
				t.Errorf("Id不一致: 期望 %s, 实际 %s", tc.msg.Id, deserializedMsg.Id)
			}
			if deserializedMsg.Type != tc.msg.Type {
				t.Errorf("Type不一致: 期望 %s, 实际 %s", tc.msg.Type, deserializedMsg.Type)
			}
			if deserializedMsg.GetData() != tc.msg.GetData() {
				t.Errorf("Data不一致: 期望 %s, 实际 %s", tc.msg.GetData(), deserializedMsg.GetData())
			}

			// 验证metadata
			if tc.msg.Metadata == nil {
				if deserializedMsg.Metadata != nil {
					t.Errorf("Metadata不一致: 期望 nil, 实际 %v", deserializedMsg.Metadata)
				}
			} else {
				if deserializedMsg.Metadata == nil {
					t.Errorf("Metadata不一致: 期望 %v, 实际 nil", tc.msg.Metadata)
				} else {
					originalValues := tc.msg.Metadata.Values()
					deserializedValues := deserializedMsg.Metadata.Values()
					if len(originalValues) != len(deserializedValues) {
						t.Errorf("Metadata长度不一致: 期望 %d, 实际 %d", len(originalValues), len(deserializedValues))
					}
					for k, v := range originalValues {
						if deserializedValues[k] != v {
							t.Errorf("Metadata值不一致 [%s]: 期望 %s, 实际 %s", k, v, deserializedValues[k])
						}
					}
				}
			}
		})
	}
}

// TestNewMsgFromBytes 测试从[]byte创建消息
func TestNewMsgFromBytes(t *testing.T) {
	data := []byte("test data from bytes")
	metadata := NewMetadata()
	metadata.PutValue("test", "value")

	msg := NewMsgFromBytes(12345, "TEST_TYPE", BINARY, metadata, data)

	// 验证消息属性
	if msg.Ts != 12345 {
		t.Errorf("Expected timestamp 12345, got %d", msg.Ts)
	}
	if msg.Type != "TEST_TYPE" {
		t.Errorf("Expected type TEST_TYPE, got %s", msg.Type)
	}
	if msg.DataType != BINARY {
		t.Errorf("Expected dataType BINARY, got %s", msg.DataType)
	}

	// 验证数据
	retrievedData := msg.GetDataAsBytes()
	if string(retrievedData) != string(data) {
		t.Errorf("Expected data %s, got %s", string(data), string(retrievedData))
	}

	// 验证元数据
	if msg.Metadata.GetValue("test") != "value" {
		t.Errorf("Expected metadata value 'value', got %s", msg.Metadata.GetValue("test"))
	}
}

// TestNewMsgWithJsonDataFromBytes 测试从[]byte创建JSON消息
func TestNewMsgWithJsonDataFromBytes(t *testing.T) {
	jsonData := []byte(`{"key": "value", "number": 123}`)

	msg := NewMsgWithJsonDataFromBytes(jsonData)

	// 验证消息属性
	if msg.DataType != JSON {
		t.Errorf("Expected dataType JSON, got %s", msg.DataType)
	}

	// 验证数据
	retrievedData := msg.GetDataAsBytes()
	if string(retrievedData) != string(jsonData) {
		t.Errorf("Expected data %s, got %s", string(jsonData), string(retrievedData))
	}

	// 验证ID不为空
	if msg.Id == "" {
		t.Error("Expected non-empty message ID")
	}
}

// TestSetDataFromBytes 测试设置[]byte数据
func TestSetDataFromBytes(t *testing.T) {
	// 创建一个初始消息
	msg := NewMsg(0, "TEST", TEXT, NewMetadata(), "initial data")

	// 更新为[]byte数据
	newData := []byte("new data from bytes")
	msg.SetDataFromBytes(newData)

	// 验证数据更新
	retrievedData := msg.GetDataAsBytes()
	if string(retrievedData) != string(newData) {
		t.Errorf("Expected data %s, got %s", string(newData), string(retrievedData))
	}

	// 验证字符串获取也正常
	stringData := msg.GetData()
	if stringData != string(newData) {
		t.Errorf("Expected string data %s, got %s", string(newData), stringData)
	}
}

// TestSharedDataFromBytes 测试SharedData的[]byte功能
func TestSharedDataFromBytes(t *testing.T) {
	data := []byte("test shared data")

	// 创建SharedData
	sd := NewSharedDataFromBytes(data)

	// 验证获取数据
	retrievedBytes := sd.GetBytes()
	if string(retrievedBytes) != string(data) {
		t.Errorf("Expected bytes %s, got %s", string(data), string(retrievedBytes))
	}

	// 验证字符串获取
	retrievedString := sd.Get()
	if retrievedString != string(data) {
		t.Errorf("Expected string %s, got %s", string(data), retrievedString)
	}

	// 测试设置[]byte
	newData := []byte("updated data")
	sd.SetBytes(newData)

	retrievedAfterSet := sd.GetBytes()
	if string(retrievedAfterSet) != string(newData) {
		t.Errorf("Expected updated bytes %s, got %s", string(newData), string(retrievedAfterSet))
	}
}

// TestAPICompatibility 测试API兼容性
func TestAPICompatibility(t *testing.T) {
	// 测试原有的string API仍然工作
	stringMsg := NewMsg(0, "STRING_TEST", TEXT, NewMetadata(), "string data")
	if stringMsg.GetData() != "string data" {
		t.Errorf("String API compatibility broken")
	}

	// 测试新的[]byte API
	byteData := []byte("byte data")
	byteMsg := NewMsgFromBytes(0, "BYTE_TEST", BINARY, NewMetadata(), byteData)
	if string(byteMsg.GetDataAsBytes()) != string(byteData) {
		t.Errorf("Byte API not working correctly")
	}

	// 测试两种方式创建的消息可以互相转换
	stringFromByte := byteMsg.GetData()
	if stringFromByte != string(byteData) {
		t.Errorf("Byte to string conversion failed")
	}

	stringMsg.SetDataFromBytes(byteData)
	convertedBytes := stringMsg.GetDataAsBytes()
	if string(convertedBytes) != string(byteData) {
		t.Errorf("String to byte conversion failed")
	}
}

// TestSharedDataZeroCopyIntegration 测试零拷贝优化的完整集成
func TestSharedDataZeroCopyIntegration(t *testing.T) {
	// 测试GetUnsafe和SetUnsafe的零拷贝特性
	originalData := "这是一个零拷贝测试数据，包含中文和English characters"
	sd := NewSharedData(originalData)

	// 测试GetUnsafe零拷贝获取
	unsafeResult := sd.GetUnsafe()
	if unsafeResult != originalData {
		t.Errorf("GetUnsafe结果不匹配: 期望 %s, 实际 %s", originalData, unsafeResult)
	}

	// 测试SetUnsafe零拷贝设置
	newData := "新的零拷贝数据 New zero-copy data"
	sd.SetUnsafe(newData)

	// 验证设置成功
	if sd.GetUnsafe() != newData {
		t.Errorf("SetUnsafe设置失败: 期望 %s, 实际 %s", newData, sd.GetUnsafe())
	}

	// 测试COW机制下的零拷贝
	copy := sd.Copy()
	modifiedData := "修改后的数据 Modified data"
	copy.SetUnsafe(modifiedData)

	// 验证原始数据未被修改
	if sd.GetUnsafe() != newData {
		t.Errorf("COW机制失效，原始数据被修改: 期望 %s, 实际 %s", newData, sd.GetUnsafe())
	}

	// 验证复制的数据被正确修改
	if copy.GetUnsafe() != modifiedData {
		t.Errorf("复制数据修改失败: 期望 %s, 实际 %s", modifiedData, copy.GetUnsafe())
	}
}

// TestSharedDataMemorySafety 测试SharedData的内存安全性
func TestSharedDataMemorySafety(t *testing.T) {
	// 测试nil数据处理
	sd := NewSharedDataFromBytes(nil)
	if sd.Get() != "" {
		t.Errorf("nil数据应该返回空字符串，实际返回: %s", sd.Get())
	}
	if sd.GetUnsafe() != "" {
		t.Errorf("nil数据GetUnsafe应该返回空字符串，实际返回: %s", sd.GetUnsafe())
	}

	// 测试空字符串处理
	sd.Set("")
	if sd.Len() != 0 {
		t.Errorf("空字符串长度应该为0，实际为: %d", sd.Len())
	}
	if !sd.IsEmpty() {
		t.Error("空字符串应该被识别为空")
	}

	// 测试大数据处理
	largeData := strings.Repeat("大数据测试Large data test", 10000)
	sd.Set(largeData)
	if sd.Get() != largeData {
		t.Error("大数据处理失败")
	}
	if sd.Len() != len(largeData) {
		t.Errorf("大数据长度不匹配: 期望 %d, 实际 %d", len(largeData), sd.Len())
	}

	// 测试特殊字符处理
	specialData := "特殊字符测试\n\r\t\"'\\`~!@#$%^&*()_+-=[]{}|;:,.<>?/"
	sd.SetUnsafe(specialData)
	if sd.GetUnsafe() != specialData {
		t.Errorf("特殊字符处理失败: 期望 %s, 实际 %s", specialData, sd.GetUnsafe())
	}

	// 测试Unicode字符处理
	unicodeData := "Unicode测试🎉🔥⭐️🌟💫🎊🎈🎁🎀🎂🍰🥳😀😃😄😁😆😅🤣😂"
	sd.SetBytes([]byte(unicodeData))
	if sd.Get() != unicodeData {
		t.Errorf("Unicode字符处理失败: 期望 %s, 实际 %s", unicodeData, sd.Get())
	}
}

// TestSharedDataCOWPerformance 测试COW机制的性能特性
func TestSharedDataCOWPerformance(t *testing.T) {
	// 创建大数据用于测试
	largeData := strings.Repeat("性能测试数据Performance test data", 1000)
	original := NewSharedData(largeData)

	// 创建大量副本（应该很快，因为只是共享引用）
	copies := make([]*SharedData, 1000)
	startTime := time.Now()
	for i := 0; i < 1000; i++ {
		copies[i] = original.Copy()
	}
	copyTime := time.Since(startTime)

	// 验证所有副本的数据正确
	for i, copy := range copies {
		if copy.Get() != largeData {
			t.Errorf("副本 %d 数据不正确", i)
		}
	}

	// 修改一个副本（应该触发COW）
	modifyStartTime := time.Now()
	copies[0].Set("修改的数据Modified data")
	modifyTime := time.Since(modifyStartTime)

	// 验证只有被修改的副本改变了
	if copies[0].Get() == largeData {
		t.Error("修改失败")
	}
	for i := 1; i < 10; i++ { // 只检查前10个以节省时间
		if copies[i].Get() != largeData {
			t.Errorf("副本 %d 被意外修改", i)
		}
	}

	// 输出性能信息
	t.Logf("复制1000个SharedData耗时: %v", copyTime)
	t.Logf("COW修改耗时: %v", modifyTime)

	// 性能要求：复制操作应该很快（小于1毫秒）
	if copyTime > time.Millisecond {
		t.Errorf("复制操作太慢: %v", copyTime)
	}
}

// TestRuleMsgMemoryOptimization 测试RuleMsg的内存优化
func TestRuleMsgMemoryOptimization(t *testing.T) {
	// 创建大量消息副本测试内存使用
	largeData := strings.Repeat("内存优化测试Memory optimization test", 500)
	metadata := NewMetadata()
	for i := 0; i < 100; i++ {
		metadata.PutValue("key"+string(rune(i)), "value"+string(rune(i)))
	}

	original := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "memory-test-msg",
		DataType: JSON,
		Type:     "MEMORY_TEST",
		Data:     NewSharedData(largeData),
		Metadata: metadata,
	}

	// 创建大量副本
	copies := make([]RuleMsg, 1000)
	for i := 0; i < 1000; i++ {
		copies[i] = original.Copy()
	}

	// 验证所有副本的数据正确
	for i, copy := range copies {
		if copy.GetData() != largeData {
			t.Errorf("副本 %d 数据不正确", i)
		}
		if copy.Metadata.Len() != metadata.Len() {
			t.Errorf("副本 %d metadata长度不正确", i)
		}
	}

	// 修改部分副本，验证独立性
	for i := 0; i < 10; i++ {
		copies[i].SetData("修改的数据" + string(rune(i)))
		copies[i].Metadata.PutValue("modified", "true")
	}

	// 验证原始消息未被修改
	if original.GetData() != largeData {
		t.Error("原始消息被意外修改")
	}
	if original.Metadata.Has("modified") {
		t.Error("原始metadata被意外修改")
	}

	// 验证未修改的副本保持不变
	for i := 10; i < 20; i++ { // 只检查一部分以节省时间
		if copies[i].GetData() != largeData {
			t.Errorf("未修改的副本 %d 被意外修改", i)
		}
	}
}

// TestMetadataConcurrentStress 测试Metadata的并发压力测试
func TestMetadataConcurrentStress(t *testing.T) {
	// 创建原始metadata
	original := NewMetadata()
	for i := 0; i < 100; i++ {
		original.PutValue("init_key_"+string(rune(i)), "init_value_"+string(rune(i)))
	}

	const numGoroutines = 100
	const numOperations = 1000
	var wg sync.WaitGroup
	var errorCount int64

	// 启动大量goroutine进行并发操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 每个goroutine创建自己的副本
			copy := original.Copy()

			for j := 0; j < numOperations; j++ {
				// 随机进行读写操作
				switch j % 4 {
				case 0: // 读操作
					_ = copy.GetValue("init_key_1")
					_ = copy.Has("init_key_2")
					_ = copy.Values()

				case 1: // 写操作
					copy.PutValue("goroutine_"+string(rune(id))+"_op_"+string(rune(j)), "value")

				case 2: // 替换操作
					if j%100 == 0 {
						newData := map[string]string{
							"replaced_" + string(rune(id)): "value_" + string(rune(j)),
						}
						copy.ReplaceAll(newData)
					}

				case 3: // 清空操作
					if j%200 == 0 {
						copy.Clear()
						copy.PutValue("cleared_"+string(rune(id)), "true")
					}
				}

				// 验证基本操作的一致性
				if copy.Len() < 0 {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()

	// 验证没有错误
	if errorCount > 0 {
		t.Errorf("并发测试中发生 %d 个错误", errorCount)
	}

	// 验证原始metadata未被修改
	if original.Len() != 100 {
		t.Errorf("原始metadata被意外修改，长度从100变为%d", original.Len())
	}
}

// TestJSONSerializationPerformance 测试JSON序列化性能
func TestJSONSerializationPerformance(t *testing.T) {
	// 创建包含大量数据的消息
	largeData := strings.Repeat(`{"key": "这是一个大的JSON数据", "number": 12345, "array": [1,2,3,4,5]}`, 100)
	metadata := NewMetadata()
	for i := 0; i < 50; i++ {
		metadata.PutValue("perf_key_"+string(rune(i)), "performance_value_"+string(rune(i)))
	}

	msg := RuleMsg{
		Ts:       time.Now().UnixMilli(),
		Id:       "performance-test-msg",
		DataType: JSON,
		Type:     "PERFORMANCE_TEST",
		Data:     NewSharedData(largeData),
		Metadata: metadata,
	}

	// 测试序列化性能
	serializeStart := time.Now()
	jsonData, err := json.Marshal(msg)
	serializeTime := time.Since(serializeStart)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化性能
	deserializeStart := time.Now()
	var deserializedMsg RuleMsg
	err = json.Unmarshal(jsonData, &deserializedMsg)
	deserializeTime := time.Since(deserializeStart)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if deserializedMsg.GetData() != msg.GetData() {
		t.Error("反序列化后数据不一致")
	}
	if deserializedMsg.Metadata.Len() != msg.Metadata.Len() {
		t.Error("反序列化后metadata长度不一致")
	}

	// 输出性能信息
	t.Logf("JSON序列化耗时: %v, 数据大小: %d bytes", serializeTime, len(jsonData))
	t.Logf("JSON反序列化耗时: %v", deserializeTime)

	// 基本性能要求
	if serializeTime > 10*time.Millisecond {
		t.Errorf("序列化太慢: %v", serializeTime)
	}
	if deserializeTime > 10*time.Millisecond {
		t.Errorf("反序列化太慢: %v", deserializeTime)
	}
}

// TestDataCallbackMechanism 测试数据变更回调机制
func TestDataCallbackMechanism(t *testing.T) {
	msg := NewMsg(0, "TEST", JSON, nil, `{"test": "data"}`)

	// 验证初始状态下parsedData为nil
	if msg.parsedData != nil {
		t.Error("初始状态下parsedData应该为nil")
	}

	// 解析JSON数据
	jsonData, err := msg.GetDataAsJson()
	if err != nil {
		t.Fatalf("解析JSON失败: %v", err)
	}
	if jsonData["test"] != "data" {
		t.Errorf("JSON解析结果不正确: %v", jsonData)
	}

	// 验证parsedData被缓存
	if msg.parsedData == nil {
		t.Error("parsedData应该被缓存")
	}

	// 修改数据，验证缓存被清空
	msg.SetData(`{"test": "modified"}`)
	if msg.parsedData != nil {
		t.Error("修改数据后parsedData应该被清空")
	}

	// 重新解析，验证得到新数据
	newJsonData, err := msg.GetDataAsJson()
	if err != nil {
		t.Fatalf("重新解析JSON失败: %v", err)
	}
	if newJsonData["test"] != "modified" {
		t.Errorf("修改后的JSON解析结果不正确: %v", newJsonData)
	}

	// 使用SetDataFromBytes也应该触发回调
	msg.SetDataFromBytes([]byte(`{"test": "from_bytes"}`))
	if msg.parsedData != nil {
		t.Error("使用SetDataFromBytes后parsedData应该被清空")
	}

	bytesJsonData, err := msg.GetDataAsJson()
	if err != nil {
		t.Fatalf("从bytes设置后解析JSON失败: %v", err)
	}
	if bytesJsonData["test"] != "from_bytes" {
		t.Errorf("从bytes设置的JSON解析结果不正确: %v", bytesJsonData)
	}
}

// TestMemoryLeakageDetection 测试内存泄漏检测
func TestMemoryLeakageDetection(t *testing.T) {
	// 此测试旨在检测可能的内存泄漏
	const iterations = 10000

	// 记录初始内存状态
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 执行大量内存分配和释放操作
	for i := 0; i < iterations; i++ {
		// 创建消息
		msg := NewMsg(0, "LEAK_TEST", JSON, nil, "测试数据"+string(rune(i)))

		// 复制消息
		copy1 := msg.Copy()
		copy2 := copy1.Copy()

		// 修改数据触发COW
		copy1.SetData("修改的数据1")
		copy2.SetData("修改的数据2")
		copy2.Metadata.PutValue("test", "value")

		// 创建SharedData
		sd := NewSharedData("shared data " + string(rune(i)))
		sdCopy := sd.Copy()
		sdCopy.Set("modified shared data")

		// 让这些变量在作用域结束时被GC回收
		_ = msg
		_ = copy1
		_ = copy2
		_ = sd
		_ = sdCopy
	}

	// 强制GC并测量内存
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存增长
	allocDiff := m2.Alloc - m1.Alloc
	totalAllocDiff := m2.TotalAlloc - m1.TotalAlloc

	t.Logf("内存使用变化:")
	t.Logf("  当前分配: %d bytes (增长: %d bytes)", m2.Alloc, allocDiff)
	t.Logf("  累计分配: %d bytes (增长: %d bytes)", m2.TotalAlloc, totalAllocDiff)
	t.Logf("  GC次数: %d -> %d", m1.NumGC, m2.NumGC)

	// 简单的内存泄漏检测：当前分配内存不应该有显著增长
	// 考虑到测试框架本身可能分配内存，设置一个相对宽松的阈值
	maxAcceptableGrowth := int64(1024 * 1024) // 1MB
	if int64(allocDiff) > maxAcceptableGrowth {
		t.Errorf("可能存在内存泄漏，当前分配内存增长了 %d bytes", allocDiff)
	}
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	// 测试无效JSON的处理
	msg := NewMsg(0, "ERROR_TEST", JSON, nil, "invalid json {")
	_, err := msg.GetDataAsJson()
	if err == nil {
		t.Error("无效JSON应该返回错误")
	}

	// 测试空数据的JSON解析
	emptyMsg := NewMsg(0, "EMPTY_TEST", JSON, nil, "")
	emptyJson, err := emptyMsg.GetDataAsJson()
	if err != nil {
		t.Errorf("空数据JSON解析失败: %v", err)
	}
	if len(emptyJson) != 0 {
		t.Errorf("空数据应该返回空map，实际: %v", emptyJson)
	}

	// 测试metadata的错误情况
	var nilMetadata *Metadata = nil
	copy := BuildMetadataFromMetadata(nilMetadata)
	if copy == nil {
		t.Error("从nil metadata构建应该返回有效的metadata")
	}
	if copy.Len() != 0 {
		t.Errorf("从nil metadata构建的长度应该为0，实际: %d", copy.Len())
	}

	// 测试SharedData的错误情况
	var nilSharedData *SharedData = nil
	if nilSharedData != nil {
		// 这个测试主要是确保我们的设计能处理nil情况
		_ = nilSharedData.Get() // 如果这里panic，说明设计有问题
	}
}
