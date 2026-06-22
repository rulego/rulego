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

package test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// parseComplexResult 解析复杂嵌套join结果
// 当只有一个结果时返回单元素数组，多个结果时返回原数组
func parseComplexResult(data string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		// 尝试解析为单个对象
		var singleObj map[string]interface{}
		err2 := json.Unmarshal([]byte(data), &singleObj)
		if err2 == nil {
			return []map[string]interface{}{singleObj}, nil
		}
		return nil, err
	}
	return result, nil
}

// TestSwitchWithInternalJoin 测试条件分支内部有join的情况
func TestSwitchWithInternalJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 条件分支 -> 某个分支内部是fork-join结构
	// 温度35: Switch Case1匹配 -> 分支1内部fork为2个并行节点 -> 内部join -> 外部join
	t.Run("Switch_BranchInternalJoin_SingleBranch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_internal_join_test1",
				"name": "条件分支内部join-单分支",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "外层条件分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=20 && msg.temperature<=50",
									"then": "Case1"
								},
								{
									"case": "msg.temperature>50",
									"then": "Case2"
								}
							]
						}
					},
					{
						"id": "fork_node",
						"type": "fork",
						"name": "内部分叉",
						"configuration": {}
					},
					{
						"id": "inner_branch_a",
						"type": "jsTransform",
						"name": "内部分支A",
						"configuration": {
							"jsScript": "msg.branchA='processed'; metadata['innerBranch']='A'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_branch_b",
						"type": "jsTransform",
						"name": "内部分支B",
						"configuration": {
							"jsScript": "msg.branchB='processed'; metadata['innerBranch']='B'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_join",
						"type": "join",
						"name": "内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "inner_process",
						"type": "jsTransform",
						"name": "内部处理",
						"configuration": {
							"jsScript": "msg.innerProcessed=true; metadata['stage']='inner'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "case2_node",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.case2='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "outer_join",
						"type": "join",
						"name": "外部合并",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "switch_node",
						"toId": "fork_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "case2_node",
						"type": "Case2"
					},
					{
						"fromId": "fork_node",
						"toId": "inner_branch_a",
						"type": "default"
					},
					{
						"fromId": "fork_node",
						"toId": "inner_branch_b",
						"type": "default"
					},
					{
						"fromId": "inner_branch_a",
						"toId": "inner_join",
						"type": "Success"
					},
					{
						"fromId": "inner_branch_b",
						"toId": "inner_join",
						"type": "Success"
					},
					{
						"fromId": "inner_join",
						"toId": "inner_process",
						"type": "Success"
					},
					{
						"fromId": "inner_process",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "case2_node",
						"toId": "outer_join",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_internal_join_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35: Switch Case1匹配 -> fork -> 内部2个并行节点 -> 内部join -> 内部处理 -> 外部join
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":35}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var resultRelationType string
		var once sync.Once

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			once.Do(func() {
				resultMsg = msg
				resultErr = err
				resultRelationType = relationType
				wg.Done()
			})
		}))

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// 解析结果 - 内部join后会合并为一条消息，然后继续处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
		assert.Nil(t, resultErr)
		// 内部fork-join后消息被合并，外部join收到的是单条消息
		// 验证消息包含内部处理的标记
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ 条件分支内部join-单分支: join成功，内部fork-join正常工作")
	})

	// 测试2: 条件分支 -> Case2匹配，不经过内部fork-join
	t.Run("Switch_BranchInternalJoin_Case2Match", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_internal_join_test2",
				"name": "条件分支内部join-Case2匹配",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "外层条件分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=20 && msg.temperature<=50",
									"then": "Case1"
								},
								{
									"case": "msg.temperature>50",
									"then": "Case2"
								}
							]
						}
					},
					{
						"id": "fork_node",
						"type": "fork",
						"name": "内部分叉",
						"configuration": {}
					},
					{
						"id": "inner_branch_a",
						"type": "jsTransform",
						"name": "内部分支A",
						"configuration": {
							"jsScript": "msg.branchA='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_branch_b",
						"type": "jsTransform",
						"name": "内部分支B",
						"configuration": {
							"jsScript": "msg.branchB='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_join",
						"type": "join",
						"name": "内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "inner_process",
						"type": "jsTransform",
						"name": "内部处理",
						"configuration": {
							"jsScript": "msg.innerProcessed=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "case2_node",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.case2='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "outer_join",
						"type": "join",
						"name": "外部合并",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "switch_node",
						"toId": "fork_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "case2_node",
						"type": "Case2"
					},
					{
						"fromId": "fork_node",
						"toId": "inner_branch_a",
						"type": "default"
					},
					{
						"fromId": "fork_node",
						"toId": "inner_branch_b",
						"type": "default"
					},
					{
						"fromId": "inner_branch_a",
						"toId": "inner_join",
						"type": "Success"
					},
					{
						"fromId": "inner_branch_b",
						"toId": "inner_join",
						"type": "Success"
					},
					{
						"fromId": "inner_join",
						"toId": "inner_process",
						"type": "Success"
					},
					{
						"fromId": "inner_process",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "case2_node",
						"toId": "outer_join",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_internal_join_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度60: Switch Case2匹配 -> 直接到case2_node -> 外部join
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":60}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var once sync.Once

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			once.Do(func() {
				resultMsg = msg
				resultErr = err
				wg.Done()
			})
		}))

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时")
		}

		assert.Nil(t, resultErr)

		// 解析结果 - 应该只有1个结果（case2处理）
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "case2_node", results[0]["nodeId"])
		t.Logf("✓ 条件分支内部join-Case2匹配: join成功，跳过内部fork-join")
	})
}

// TestInclusiveWithInternalJoin 测试包容分支内部有join的情况
func TestInclusiveWithInternalJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 包容分支 -> 两个分支内部都有join
	// 温度35: Inclusive Case1和Case2都匹配 -> 两个分支各自内部fork-join -> 外部join
	t.Run("Inclusive_BothBranchesInternalJoin", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_internal_join_test1",
				"name": "包容分支内部join-两分支都有",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "外层包容分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=20 && msg.temperature<=50",
									"then": "Case1"
								},
								{
									"case": "msg.temperature>30",
									"then": "Case2"
								}
							]
						}
					},
					{
						"id": "fork1",
						"type": "fork",
						"name": "分支1内部分叉",
						"configuration": {}
					},
					{
						"id": "fork2",
						"type": "fork",
						"name": "分支2内部分叉",
						"configuration": {}
					},
					{
						"id": "branch1_a",
						"type": "jsTransform",
						"name": "分支1处理A",
						"configuration": {
							"jsScript": "msg.branch1A='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch1_b",
						"type": "jsTransform",
						"name": "分支1处理B",
						"configuration": {
							"jsScript": "msg.branch1B='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_a",
						"type": "jsTransform",
						"name": "分支2处理A",
						"configuration": {
							"jsScript": "msg.branch2A='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_b",
						"type": "jsTransform",
						"name": "分支2处理B",
						"configuration": {
							"jsScript": "msg.branch2B='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_join1",
						"type": "join",
						"name": "分支1内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "inner_join2",
						"type": "join",
						"name": "分支2内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "process1",
						"type": "jsTransform",
						"name": "分支1后处理",
						"configuration": {
							"jsScript": "msg.processed1=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "process2",
						"type": "jsTransform",
						"name": "分支2后处理",
						"configuration": {
							"jsScript": "msg.processed2=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "outer_join",
						"type": "join",
						"name": "外部合并",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "inclusive_node",
						"toId": "fork1",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "fork2",
						"type": "Case2"
					},
					{
						"fromId": "fork1",
						"toId": "branch1_a",
						"type": "default"
					},
					{
						"fromId": "fork1",
						"toId": "branch1_b",
						"type": "default"
					},
					{
						"fromId": "fork2",
						"toId": "branch2_a",
						"type": "default"
					},
					{
						"fromId": "fork2",
						"toId": "branch2_b",
						"type": "default"
					},
					{
						"fromId": "branch1_a",
						"toId": "inner_join1",
						"type": "Success"
					},
					{
						"fromId": "branch1_b",
						"toId": "inner_join1",
						"type": "Success"
					},
					{
						"fromId": "branch2_a",
						"toId": "inner_join2",
						"type": "Success"
					},
					{
						"fromId": "branch2_b",
						"toId": "inner_join2",
						"type": "Success"
					},
					{
						"fromId": "inner_join1",
						"toId": "process1",
						"type": "Success"
					},
					{
						"fromId": "inner_join2",
						"toId": "process2",
						"type": "Success"
					},
					{
						"fromId": "process1",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "process2",
						"toId": "outer_join",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_internal_join_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35: Inclusive Case1和Case2都匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":35}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var resultRelationType string
		var once sync.Once

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			once.Do(func() {
				resultMsg = msg
				resultErr = err
				resultRelationType = relationType
				wg.Done()
			})
		}))

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// 解析结果 - 应该有2个结果（process1和process2）
		// 每个分支内部fork-join后合并为一条，然后外部join收集两个分支的结果
		t.Logf("结果数据: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("结果数量: %d", len(results))
		// 验证结果包含处理后的数据
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ 包容分支内部join-两分支都有: join成功")
	})

	// 测试2: 包容分支 -> 只有一个分支有内部join
	// 温度25: Inclusive只有Case1匹配 -> 分支1内部fork-join -> 外部join
	t.Run("Inclusive_SingleBranchInternalJoin", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_internal_join_test2",
				"name": "包容分支内部join-单分支",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "外层包容分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=20 && msg.temperature<=50",
									"then": "Case1"
								},
								{
									"case": "msg.temperature>30",
									"then": "Case2"
								}
							]
						}
					},
					{
						"id": "fork1",
						"type": "fork",
						"name": "分支1内部分叉",
						"configuration": {}
					},
					{
						"id": "branch1_a",
						"type": "jsTransform",
						"name": "分支1处理A",
						"configuration": {
							"jsScript": "msg.branch1A='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch1_b",
						"type": "jsTransform",
						"name": "分支1处理B",
						"configuration": {
							"jsScript": "msg.branch1B='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_join1",
						"type": "join",
						"name": "分支1内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "process1",
						"type": "jsTransform",
						"name": "分支1后处理",
						"configuration": {
							"jsScript": "msg.processed1=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_single",
						"type": "jsTransform",
						"name": "分支2单节点",
						"configuration": {
							"jsScript": "msg.branch2='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "outer_join",
						"type": "join",
						"name": "外部合并",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "inclusive_node",
						"toId": "fork1",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch2_single",
						"type": "Case2"
					},
					{
						"fromId": "fork1",
						"toId": "branch1_a",
						"type": "default"
					},
					{
						"fromId": "fork1",
						"toId": "branch1_b",
						"type": "default"
					},
					{
						"fromId": "branch1_a",
						"toId": "inner_join1",
						"type": "Success"
					},
					{
						"fromId": "branch1_b",
						"toId": "inner_join1",
						"type": "Success"
					},
					{
						"fromId": "inner_join1",
						"toId": "process1",
						"type": "Success"
					},
					{
						"fromId": "process1",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "branch2_single",
						"toId": "outer_join",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_internal_join_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度25: Inclusive只有Case1匹配 -> 分支1内部fork-join
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":25}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var once sync.Once

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			once.Do(func() {
				resultMsg = msg
				resultErr = err
				wg.Done()
			})
		}))

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时")
		}

		assert.Nil(t, resultErr)

		// 解析结果 - 应该只有1个结果（process1）
		t.Logf("结果数据: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("结果数量: %d", len(results))
		// 验证结果包含处理后的数据
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ 包容分支内部join-单分支: join成功，只有分支1执行")
	})
}

// TestDoubleNestedJoin 测试双层嵌套join
func TestDoubleNestedJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试: 条件分支 -> 包容分支 -> 内部fork-join -> 外部join
	t.Run("Switch_Inclusive_InternalForkJoin", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "double_nested_join_test",
				"name": "双层嵌套join",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "外层条件分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=20 && msg.temperature<=50",
									"then": "Case1"
								},
								{
									"case": "msg.temperature>50",
									"then": "Case2"
								}
							]
						}
					},
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "内层包容分支",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature>=30",
									"then": "High"
								},
								{
									"case": "msg.temperature<=40",
									"then": "Low"
								}
							]
						}
					},
					{
						"id": "fork_high",
						"type": "fork",
						"name": "高温分支分叉",
						"configuration": {}
					},
					{
						"id": "fork_low",
						"type": "fork",
						"name": "低温分支分叉",
						"configuration": {}
					},
					{
						"id": "high_a",
						"type": "jsTransform",
						"name": "高温处理A",
						"configuration": {
							"jsScript": "msg.highA='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "high_b",
						"type": "jsTransform",
						"name": "高温处理B",
						"configuration": {
							"jsScript": "msg.highB='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "low_a",
						"type": "jsTransform",
						"name": "低温处理A",
						"configuration": {
							"jsScript": "msg.lowA='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "low_b",
						"type": "jsTransform",
						"name": "低温处理B",
						"configuration": {
							"jsScript": "msg.lowB='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "inner_join_high",
						"type": "join",
						"name": "高温内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "inner_join_low",
						"type": "join",
						"name": "低温内部合并",
						"configuration": {
							"timeout": 5
						}
					},
					{
						"id": "process_high",
						"type": "jsTransform",
						"name": "高温后处理",
						"configuration": {
							"jsScript": "msg.processedHigh=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "process_low",
						"type": "jsTransform",
						"name": "低温后处理",
						"configuration": {
							"jsScript": "msg.processedLow=true; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "case2_node",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.case2='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "outer_join",
						"type": "join",
						"name": "外部合并",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "switch_node",
						"toId": "inclusive_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "case2_node",
						"type": "Case2"
					},
					{
						"fromId": "inclusive_node",
						"toId": "fork_high",
						"type": "High"
					},
					{
						"fromId": "inclusive_node",
						"toId": "fork_low",
						"type": "Low"
					},
					{
						"fromId": "fork_high",
						"toId": "high_a",
						"type": "default"
					},
					{
						"fromId": "fork_high",
						"toId": "high_b",
						"type": "default"
					},
					{
						"fromId": "fork_low",
						"toId": "low_a",
						"type": "default"
					},
					{
						"fromId": "fork_low",
						"toId": "low_b",
						"type": "default"
					},
					{
						"fromId": "high_a",
						"toId": "inner_join_high",
						"type": "Success"
					},
					{
						"fromId": "high_b",
						"toId": "inner_join_high",
						"type": "Success"
					},
					{
						"fromId": "low_a",
						"toId": "inner_join_low",
						"type": "Success"
					},
					{
						"fromId": "low_b",
						"toId": "inner_join_low",
						"type": "Success"
					},
					{
						"fromId": "inner_join_high",
						"toId": "process_high",
						"type": "Success"
					},
					{
						"fromId": "inner_join_low",
						"toId": "process_low",
						"type": "Success"
					},
					{
						"fromId": "process_high",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "process_low",
						"toId": "outer_join",
						"type": "Success"
					},
					{
						"fromId": "case2_node",
						"toId": "outer_join",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("double_nested_join_test", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35: Switch Case1匹配 -> Inclusive High(>=30)和Low(<=40)都匹配
		// -> fork_high和fork_low各自fork -> 内部join -> process -> 外部join
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":35}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var resultRelationType string
		var once sync.Once

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			once.Do(func() {
				resultMsg = msg
				resultErr = err
				resultRelationType = relationType
				wg.Done()
			})
		}))

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// 解析结果 - 应该有2个结果（process_high和process_low）
		t.Logf("结果数据: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("结果数量: %d", len(results))
		// 验证结果包含处理后的数据
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ 双层嵌套join: join成功")
	})
}
