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
	"strings"
	"testing"
)

// BenchmarkRuleMsgCopy 基准测试：消息复制性能
func BenchmarkRuleMsgCopy(b *testing.B) {
	// 测试不同大小的数据
	testCases := []struct {
		name string
		data string
	}{
		{"Small", "small data"},
		{"Medium", strings.Repeat("medium data ", 100)},
		{"Large", strings.Repeat("large data content ", 1000)},
		{"XLarge", strings.Repeat("extra large data content for testing ", 10000)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			original := NewMsg(0, "TEST", JSON, nil, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = original.Copy()
			}
		})
	}
}

// BenchmarkRuleMsgCopyAndModify 基准测试：复制后修改数据的性能
func BenchmarkRuleMsgCopyAndModify(b *testing.B) {
	testCases := []struct {
		name string
		data string
	}{
		{"Small", "small data"},
		{"Medium", strings.Repeat("medium data ", 100)},
		{"Large", strings.Repeat("large data content ", 1000)},
		{"XLarge", strings.Repeat("extra large data content for testing ", 10000)},
	}

	for _, tc := range testCases {
		b.Run(tc.name+"_DirectAssignment", func(b *testing.B) {
			original := NewMsg(0, "TEST", JSON, nil, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})

		b.Run(tc.name+"_COWOptimized", func(b *testing.B) {
			original := NewMsg(0, "TEST", JSON, nil, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})
	}
}

// BenchmarkMultipleCopies 基准测试：创建多个副本的性能
func BenchmarkMultipleCopies(b *testing.B) {
	largeData := strings.Repeat("benchmark data for multiple copies ", 1000)
	original := NewMsg(0, "BENCH", JSON, nil, largeData)

	b.Run("Create100Copies", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("Create100CopiesAndModifyOne", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 修改第一个副本
			copies[0].SetData("modified")
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("Create100CopiesAndModifyAll", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 修改所有副本
			for j := 0; j < 100; j++ {
				copies[j].SetData("modified")
			}
			// 防止编译器优化
			_ = copies
		}
	})
}

// BenchmarkConcurrentAccess 基准测试：并发访问性能
func BenchmarkConcurrentAccess(b *testing.B) {
	largeData := strings.Repeat("concurrent access benchmark data ", 1000)
	original := NewMsg(0, "CONCURRENT", JSON, nil, largeData)

	b.Run("ConcurrentCopy", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = original.Copy()
			}
		})
	})

	b.Run("ConcurrentCopyAndRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				copy := original.Copy()
				_ = copy.Data
			}
		})
	})

	b.Run("ConcurrentCopyAndModify", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				copy := original.Copy()
				copy.SetData("modified in goroutine")
			}
		})
	})
}

// BenchmarkMemoryUsage 基准测试：内存使用效率
func BenchmarkMemoryUsage(b *testing.B) {
	// 创建一个大数据消息
	largeData := strings.Repeat("memory usage test data ", 5000) // 约120KB
	original := NewMsg(0, "MEMORY", JSON, nil, largeData)

	b.Run("MemoryEfficiency", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 创建多个副本但不修改，应该共享内存
			copies := make([]RuleMsg, 10)
			for j := 0; j < 10; j++ {
				copies[j] = original.Copy()
			}
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("MemoryWithModification", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 创建多个副本并修改，会触发COW
			copies := make([]RuleMsg, 10)
			for j := 0; j < 10; j++ {
				copies[j] = original.Copy()
				copies[j].SetData("modified")
			}
			// 防止编译器优化
			_ = copies
		}
	})
}

// BenchmarkSharedDataOperations 基准测试：SharedData操作性能
func BenchmarkSharedDataOperations(b *testing.B) {
	testData := strings.Repeat("shared data operations test ", 100)

	b.Run("NewSharedData", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewSharedData(testData)
		}
	})

	b.Run("SharedDataCopy", func(b *testing.B) {
		sd := NewSharedData(testData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sd.Copy()
		}
	})

	b.Run("SharedDataGet", func(b *testing.B) {
		sd := NewSharedData(testData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sd.Get()
		}
	})

	b.Run("SharedDataSet", func(b *testing.B) {
		sd := NewSharedData(testData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sd.Set("new data")
		}
	})
}
