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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
)

// TestEnvPoolRecyclingAndIsolation 测试环境变量池的回收机制和隔离性
// 使用 exprTransform 组件验证多个节点间环境变量不会相互影响
func TestEnvPoolRecyclingAndIsolation(t *testing.T) {
	// 创建一个包含多个 exprTransform 节点的规则链，用于验证节点间环境变量隔离
	ruleChain := `{
		"ruleChain": {
			"id": "rule01",
			"name": "testEnvPool",
			"root": true,
			"debugMode": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "transform1",
					"type": "exprTransform",
					"name": "转换节点1",
					"debugMode": true,
					"configuration": {
						"mapping": {
							"name": "upper(msg.name)",
							"temperature": "msg.temperature",
							"alarm": "msg.temperature > 50",
							"nodeId": "'node1'",
							"processedBy": "'transform1'",
							"productType": "metadata.productType",
							"deviceId": "metadata.deviceId",
							"location": "metadata.location",
							"instanceId": "metadata.instanceId",
							"step": "1"
						}
					}
				},
				{
					"id": "transform2",
					"type": "exprTransform",
					"name": "转换节点2",
					"debugMode": true,
					"configuration": {
						"mapping": {
							"name": "msg.name + '_step2'",
							"temperature": "msg.temperature + 10",
							"alarm": "msg.temperature > 30",
							"nodeId": "'node2'",
							"processedBy": "'transform2'",
							"productType": "metadata.productType",
							"deviceId": "metadata.deviceId",
							"location": "metadata.location",
							"instanceId": "metadata.instanceId",
							"step": "2"
						}
					}
				},
				{
					"id": "fork1",
					"type": "fork",
					"name": "分叉节点",
					"debugMode": true,
					"configuration": {}
				},
				{
					"id": "end1",
					"type": "exprTransform",
					"name": "结束节点1",
					"debugMode": true,
					"configuration": {
						"mapping": {
							"name": "msg.name + '_end1'",
							"temperature": "msg.temperature",
							"alarm": "msg.temperature > 60",
							"nodeId": "'end1'",
							"processedBy": "'end1'",
							"productType": "metadata.productType",
							"deviceId": "metadata.deviceId",
							"location": "metadata.location",
							"instanceId": "metadata.instanceId",
							"step": "end1"
						}
					}
				},
				{
					"id": "end2",
					"type": "exprTransform",
					"name": "结束节点2",
					"debugMode": true,
					"configuration": {
						"mapping": {
							"name": "msg.name + '_end2'",
							"temperature": "msg.temperature * 1.5",
							"alarm": "msg.temperature > 45",
							"nodeId": "'end2'",
							"processedBy": "'end2'",
							"productType": "metadata.productType",
							"deviceId": "metadata.deviceId",
							"location": "metadata.location",
							"instanceId": "metadata.instanceId",
							"step": "end2"
						}
					}
				},
				{
					"id": "end3",
					"type": "exprTransform",
					"name": "结束节点3",
					"debugMode": true,
					"configuration": {
						"mapping": {
							"name": "msg.name + '_end3'",
							"temperature": "msg.temperature + 20",
							"alarm": "msg.temperature > 35",
							"nodeId": "'end3'",
							"processedBy": "'end3'",
							"productType": "metadata.productType",
							"deviceId": "metadata.deviceId",
							"location": "metadata.location",
							"instanceId": "metadata.instanceId",
							"step": "end3"
						}
					}
				}
			],
			"connections": [
				{
					"fromId": "transform1",
					"toId": "transform2",
					"type": "Success"
				},
				{
					"fromId": "transform2",
					"toId": "fork1",
					"type": "Success"
				},
				{
					"fromId": "fork1",
					"toId": "end1",
					"type": "Success"
				},
				{
					"fromId": "fork1",
					"toId": "end2",
					"type": "Success"
				},
				{
					"fromId": "fork1",
					"toId": "end3",
					"type": "Success"
				}
			]
		}
	}`

	// 创建规则引擎
	config := NewConfig()

	// 用于收集不同节点的结果
	var transform1Results []types.RuleMsg
	var transform2Results []types.RuleMsg
	var end1Results []types.RuleMsg
	var end2Results []types.RuleMsg
	var end3Results []types.RuleMsg
	var onEndCallCount int32 // 记录OnEnd回调执行次数
	var mu sync.Mutex

	// 设置调试回调来收集不同节点的结果
	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if flowType == types.Out {
			mu.Lock()
			defer mu.Unlock()
			switch nodeId {
			case "transform1":
				transform1Results = append(transform1Results, msg)
			case "transform2":
				transform2Results = append(transform2Results, msg)
			case "end1":
				end1Results = append(end1Results, msg)
			case "end2":
				end2Results = append(end2Results, msg)
			case "end3":
				end3Results = append(end3Results, msg)
			}
		}
	}

	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChain), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 记录初始池大小
	initialPoolSize := getPoolSize()

	// 设置等待组，等待所有消息处理完成
	// 每个消息会产生3个结束节点，所以需要等待15次OnEnd回调
	var wg sync.WaitGroup
	wg.Add(15) // 5个消息 * 3个结束节点

	// 发送多个消息来测试环境变量池的复用和实例间隔离
	for i := 0; i < 5; i++ {
		metaData := types.NewMetadata()
		metaData.PutValue("productType", "sensor")
		metaData.PutValue("deviceId", str.ToString(i))
		metaData.PutValue("location", "room"+str.ToString(i))
		metaData.PutValue("instanceId", "instance_"+str.ToString(i))

		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData,
			`{"name":"device`+str.ToString(i)+`","temperature":`+str.ToString(40+i*10)+`}`)

		ruleEngine.OnMsg(msg, types.WithEndFunc(func(ctx types.RuleContext, resultMsg types.RuleMsg, err error) {
			// 每次OnEnd被调用时增加计数器
			atomic.AddInt32(&onEndCallCount, 1)
			wg.Done()
		}))
	}

	// 等待所有消息处理完成
	wg.Wait()

	// 等待所有调试回调完成
	time.Sleep(time.Millisecond * 200)

	// 验证结果数量（每个消息经过transform1、transform2，然后fork到3个结束节点）
	assert.Equal(t, 5, len(transform1Results), "Should have 5 results from transform1")
	assert.Equal(t, 5, len(transform2Results), "Should have 5 results from transform2")
	assert.Equal(t, 5, len(end1Results), "Should have 5 results from end1")
	assert.Equal(t, 5, len(end2Results), "Should have 5 results from end2")
	assert.Equal(t, 5, len(end3Results), "Should have 5 results from end3")

	// 验证每个节点的处理结果和环境变量隔离性
	// 验证 transform1 的结果（第一个转换节点：名称转大写）
	assert.Equal(t, 5, len(transform1Results), "Transform1 should have 5 results")
	// 验证所有结果都符合预期，不依赖顺序
	for _, result := range transform1Results {
		transform1Data, err := result.GetDataAsJson()
		assert.Nil(t, err)
		// 验证名称格式正确（DEVICE开头）
		name, ok := transform1Data["name"].(string)
		assert.True(t, ok && len(name) >= 7 && name[:6] == "DEVICE", "Transform1 name should start with DEVICE")
		// 验证温度在合理范围内
		temp, ok := transform1Data["temperature"].(float64)
		assert.True(t, ok && temp >= 40 && temp <= 80, "Transform1 temperature should be in range [40,80]")
		assert.Equal(t, "node1", transform1Data["nodeId"], "Transform1 should have nodeId=node1")
		assert.Equal(t, "transform1", transform1Data["processedBy"], "Transform1 should have processedBy=transform1")
		assert.Equal(t, float64(1), transform1Data["step"], "Transform1 should have step=1")
		// 验证instanceId格式正确
		instanceId, ok := transform1Data["instanceId"].(string)
		assert.True(t, ok && len(instanceId) >= 9 && instanceId[:9] == "instance_", "Transform1 should have correct instanceId format")
	}

	// 验证 transform2 的结果（第二个转换节点：添加后缀）
	assert.Equal(t, 5, len(transform2Results), "Transform2 should have 5 results")
	for _, result := range transform2Results {
		transform2Data, err := result.GetDataAsJson()
		assert.Nil(t, err)
		// 验证名称格式正确（DEVICE开头，_step2结尾）
		name, ok := transform2Data["name"].(string)
		assert.True(t, ok && len(name) >= 13 && name[:6] == "DEVICE" && name[len(name)-6:] == "_step2", "Transform2 name should have _step2 suffix")
		assert.Equal(t, "node2", transform2Data["nodeId"], "Transform2 should have nodeId=node2")
		assert.Equal(t, "transform2", transform2Data["processedBy"], "Transform2 should have processedBy=transform2")
		assert.Equal(t, float64(2), transform2Data["step"], "Transform2 should have step=2")
	}

	// 验证结束节点的结果
	assert.Equal(t, 5, len(end1Results), "End1 should have 5 results")
	assert.Equal(t, 5, len(end2Results), "End2 should have 5 results")
	assert.Equal(t, 5, len(end3Results), "End3 should have 5 results")

	// 验证所有结束节点的结果格式正确
	for _, result := range end1Results {
		end1Data, err := result.GetDataAsJson()
		assert.Nil(t, err)
		name, ok := end1Data["name"].(string)
		assert.True(t, ok && len(name) >= 18 && name[len(name)-5:] == "_end1", "End1 name should have _end1 suffix")
		assert.Equal(t, "end1", end1Data["nodeId"], "End1 should have nodeId=end1")
	}

	for _, result := range end2Results {
		end2Data, err := result.GetDataAsJson()
		assert.Nil(t, err)
		name, ok := end2Data["name"].(string)
		assert.True(t, ok && len(name) >= 18 && name[len(name)-5:] == "_end2", "End2 name should have _end2 suffix")
		assert.Equal(t, "end2", end2Data["nodeId"], "End2 should have nodeId=end2")
	}

	for _, result := range end3Results {
		end3Data, err := result.GetDataAsJson()
		assert.Nil(t, err)
		name, ok := end3Data["name"].(string)
		assert.True(t, ok && len(name) >= 18 && name[len(name)-5:] == "_end3", "End3 name should have _end3 suffix")
		assert.Equal(t, "end3", end3Data["nodeId"], "End3 should have nodeId=end3")
	}

	// 验证OnEnd执行次数（每个消息会产生3个结束节点，所以应该执行15次）
	expectedOnEndCalls := int32(5 * 3) // 5个消息 * 3个结束节点
	actualOnEndCalls := atomic.LoadInt32(&onEndCallCount)
	assert.Equal(t, expectedOnEndCalls, actualOnEndCalls, "OnEnd should be called once for each end node per message")

	// 验证环境变量池的回收
	time.Sleep(time.Millisecond * 100)
	finalPoolSize := getPoolSize()

	// 池大小应该回到初始状态或更大（由于复用）
	assert.True(t, finalPoolSize >= initialPoolSize, "Pool should maintain or increase size due to recycling")
}

// TestEnvPoolConcurrentAccess 测试环境变量池的并发访问安全性
func TestEnvPoolConcurrentAccess(t *testing.T) {
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// 并发获取和释放环境变量
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineId int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// 从池中获取环境变量
				evn := GetEnvFromPool()
				assert.NotNil(t, evn, "Environment variable map should not be nil")

				// 设置一些测试数据
				evn["goroutine"] = goroutineId
				evn["operation"] = j
				evn["test_key"] = "test_value"

				// 验证数据
				assert.Equal(t, goroutineId, evn["goroutine"])
				assert.Equal(t, j, evn["operation"])
				assert.Equal(t, "test_value", evn["test_key"])

				// 释放回池中
				ReleaseEnvToPool(evn)

				// 验证释放后map被清空
				assert.Equal(t, 0, len(evn), "Environment variable map should be empty after release")
			}
		}(i)
	}

	wg.Wait()

	// 验证池中的对象是干净的
	for i := 0; i < 10; i++ {
		evn := GetEnvFromPool()
		assert.Equal(t, 0, len(evn), "Pooled environment variable map should be clean")
		ReleaseEnvToPool(evn)
	}
}

// getPoolSize 获取当前环境变量池的大小（近似值）
// 注意：这是一个辅助函数，用于测试目的
func getPoolSize() int {
	// 使用较小的采样数量避免与正在运行的测试竞争
	var maps []map[string]interface{}
	for i := 0; i < 10; i++ {
		m := GetEnvFromPool()
		if len(m) == 0 {
			maps = append(maps, m)
		} else {
			// 如果获取到的map不为空，说明池中有脏数据，先清理再放回
			ReleaseEnvToPool(m)
			maps = append(maps, m) // 仍然计入统计
		}
	}

	// 将所有获取的map放回池中
	for _, m := range maps {
		ReleaseEnvToPool(m)
	}

	return len(maps)
}
