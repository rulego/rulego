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
	"strings"
	"sync"
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
		Data:     `{"test": "data"}`,
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
	if copiedMsg.Data != originalMsg.Data {
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
		Data:     `{"temperature": 25.5, "humidity": 60.2}`,
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
	if deserializedMsg.Data != originalMsg.Data {
		t.Errorf("Data不匹配: 期望 %s, 实际 %s", originalMsg.Data, deserializedMsg.Data)
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
		Data:     "test data",
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
		Data:     "original data",
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
				Data:     `{"test": "data", "number": 42}`,
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
				Metadata: NewMetadata(),
			},
		},
		{
			name: "Metadata 空",
			msg: RuleMsg{
				Id:       "metadataNil",
				Type:     "metadataNil",
				Metadata: nil,
			},
		},
		{
			name: "包含特殊字符",
			msg: RuleMsg{
				Id:   "special-chars",
				Type: "SPECIAL",
				Data: `{"message": "Hello\nWorld\t\"Test\""}`,
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
			if deserializedMsg.Data != tc.msg.Data {
				t.Errorf("Data不一致: 期望 %s, 实际 %s", tc.msg.Data, deserializedMsg.Data)
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
