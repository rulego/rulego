/*
 * Copyright 2024 The RuleGo Authors.
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
	"strings"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/str"
)

func BenchmarkChainNotChangeMetadata(b *testing.B) {
	b.ResetTimer()
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkChainChangeMetadataAndMsg(b *testing.B) {

	config := NewConfig()

	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	if err != nil {
		b.Error(err)
	}
	//modify s1 node content
	_ = ruleEngine.ReloadChild("s2", []byte(modifyMetadataAndMsgNode))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "test01")
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"temperature\":35}")
		ruleEngine.OnMsg(msg)
	}
}

func BenchmarkCallRestApiNodeGo(b *testing.B) {
	//不使用协程池
	config := NewConfig()
	ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

func BenchmarkCallRestApiNodeWorkerPool(b *testing.B) {
	//使用协程池
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callRestApiNode(ruleEngine)
	}
}

//	func BenchmarkCallRestApiNodeAnts(b *testing.B) {
//		defaultAntsPool, _ := ants.NewPool(200000)
//		//使用协程池
//		config := NewConfig(types.WithPool(defaultAntsPool))
//		ruleEngine, _ := New(str.RandomStr(10), loadFile("./chain_call_rest_api.json"), WithConfig(config))
//		b.ResetTimer()
//		for i := 0; i < b.N; i++ {
//			callRestApiNode(ruleEngine)
//		}
//	}
func callRestApiNode(ruleEngine types.RuleEngine) {
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"aa\":\"aaaaaaaaaaaaaa\"}")
	ruleEngine.OnMsg(msg)
}

// BenchmarkRuleMsgDataCOW 基准测试：RuleMsg Data字段的写时复制性能
func BenchmarkRuleMsgDataCOW(b *testing.B) {
	// 测试不同大小的数据
	testCases := []struct {
		name string
		data string
	}{
		{"Small", "small data"},
		{"Medium", strings.Repeat("medium data ", 100)},
		{"Large", strings.Repeat("large data content ", 1000)},
	}

	for _, tc := range testCases {
		b.Run(tc.name+"_Copy", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = original.Copy()
			}
		})

		b.Run(tc.name+"_CopyAndModifyDirect", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})

		b.Run(tc.name+"_CopyAndModifyCOW", func(b *testing.B) {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			original := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, tc.data)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy := original.Copy()
				copy.SetData("modified data")
			}
		})
	}
}

// BenchmarkRuleMsgDataCOWMultipleCopies 基准测试：多副本创建和修改性能
func BenchmarkRuleMsgDataCOWMultipleCopies(b *testing.B) {
	largeData := strings.Repeat("benchmark data for multiple copies ", 1000)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	original := types.NewMsg(0, "BENCH", types.JSON, metaData, largeData)

	b.Run("Create100Copies", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]types.RuleMsg, 100)
			for j := 0; j < 100; j++ {
				copies[j] = original.Copy()
			}
			// 防止编译器优化
			_ = copies
		}
	})

	b.Run("Create100CopiesAndModifyOne", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			copies := make([]types.RuleMsg, 100)
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
			copies := make([]types.RuleMsg, 100)
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

// BenchmarkRuleMsgDataCOWInRuleChain 基准测试：规则链中的Data COW性能
func BenchmarkRuleMsgDataCOWInRuleChain(b *testing.B) {
	// 创建一个会修改Data的规则链
	dataModifyRuleChain := `{
		"ruleChain": {
			"id": "test_data_cow",
			"name": "testDataCOW",
			"debugMode": false,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "transform1",
					"type": "jsTransform",
					"name": "数据转换1",
					"configuration": {
						"jsScript": "msg.processed_by = 'transform1'; msg.timestamp = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "transform2",
					"type": "jsTransform",
					"name": "数据转换2",
					"configuration": {
						"jsScript": "msg.processed_by = 'transform2'; msg.counter = (msg.counter || 0) + 1; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "transform1",
					"toId": "transform2",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(dataModifyRuleChain), WithConfig(config))
	if err != nil {
		b.Error(err)
	}

	b.Run("RuleChainWithDataModification", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			metaData := types.NewMetadata()
			metaData.PutValue("productType", "test01")
			msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, `{"temperature":35,"humidity":60}`)
			ruleEngine.OnMsg(msg)
		}
	})
}

// BenchmarkRuleMsgDataCOWConcurrent 基准测试：并发场景下的Data COW性能
func BenchmarkRuleMsgDataCOWConcurrent(b *testing.B) {
	largeData := strings.Repeat("concurrent test data ", 1000)
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	original := types.NewMsg(0, "CONCURRENT", types.JSON, metaData, largeData)

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
				_ = copy.GetData()
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
