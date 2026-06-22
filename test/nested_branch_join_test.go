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

// parseNestedResult 解析嵌套分支join结果
func parseNestedResult(data string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

// TestSwitchNestedInclusiveWithJoin 测试条件分支嵌套包容分支后join
func TestSwitchNestedInclusiveWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 条件分支 -> 包容分支 -> join
	// 温度35: Switch的Case1匹配 -> Inclusive的Case1和Case2都匹配 -> 两个分支都执行 -> join
	t.Run("Switch_NestedInclusive_AllInnerMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_nested_inclusive_test1",
				"name": "条件分支嵌套包容分支-内部都匹配",
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
									"then": "Inner1"
								},
								{
									"case": "msg.temperature<=40",
									"then": "Inner2"
								}
							]
						}
					},
					{
						"id": "branch_high",
						"type": "jsTransform",
						"name": "高温处理",
						"configuration": {
							"jsScript": "msg.highTemp='processed'; metadata['branch']='high'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_low",
						"type": "jsTransform",
						"name": "低温处理",
						"configuration": {
							"jsScript": "msg.lowTemp='processed'; metadata['branch']='low'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_cold",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.cold='processed'; metadata['branch']='cold'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
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
						"toId": "branch_cold",
						"type": "Case2"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_high",
						"type": "Inner1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_low",
						"type": "Inner2"
					},
					{
						"fromId": "branch_high",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_low",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_cold",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_nested_inclusive_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35: Switch Case1匹配 -> Inclusive Inner1(>=30)和Inner2(<=40)都匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":35}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error
		var resultRelationType string

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
			resultRelationType = relationType
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

		// 解析结果 - 应该有2个结果（高温和低温分支）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 2, len(results), "应该有2个分支结果")

		nodeIds := make(map[string]bool)
		for _, r := range results {
			nodeIds[r["nodeId"].(string)] = true
		}
		assert.True(t, nodeIds["branch_high"])
		assert.True(t, nodeIds["branch_low"])
		t.Logf("✓ 条件分支嵌套包容分支-内部都匹配: join成功，收到%d个结果", len(results))
	})

	// 测试2: 条件分支 -> 包容分支 -> join，只有部分内部匹配
	// 温度25: Switch的Case1匹配 -> Inclusive的Inner2(<=40)匹配，Inner1(>=30)不匹配
	t.Run("Switch_NestedInclusive_PartialInnerMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_nested_inclusive_test2",
				"name": "条件分支嵌套包容分支-部分内部匹配",
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
									"then": "Inner1"
								},
								{
									"case": "msg.temperature<=40",
									"then": "Inner2"
								}
							]
						}
					},
					{
						"id": "branch_high",
						"type": "jsTransform",
						"name": "高温处理",
						"configuration": {
							"jsScript": "msg.highTemp='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_low",
						"type": "jsTransform",
						"name": "低温处理",
						"configuration": {
							"jsScript": "msg.lowTemp='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_cold",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.cold='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
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
						"toId": "branch_cold",
						"type": "Case2"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_high",
						"type": "Inner1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_low",
						"type": "Inner2"
					},
					{
						"fromId": "branch_high",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_low",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_cold",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_nested_inclusive_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度25: Switch Case1匹配 -> Inclusive只有Inner2(<=40)匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":25}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
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

		// 解析结果 - 应该只有1个结果（低温分支）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "branch_low", results[0]["nodeId"])
		t.Logf("✓ 条件分支嵌套包容分支-部分内部匹配: join成功，收到%d个结果", len(results))
	})

	// 测试3: 条件分支 -> 包容分支 -> join，外层Case2匹配
	// 温度60: Switch的Case2匹配 -> 直接到branch_cold -> join
	t.Run("Switch_NestedInclusive_OuterCase2Match", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_nested_inclusive_test3",
				"name": "条件分支嵌套包容分支-外层Case2匹配",
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
									"then": "Inner1"
								},
								{
									"case": "msg.temperature<=40",
									"then": "Inner2"
								}
							]
						}
					},
					{
						"id": "branch_high",
						"type": "jsTransform",
						"name": "高温处理",
						"configuration": {
							"jsScript": "msg.highTemp='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_low",
						"type": "jsTransform",
						"name": "低温处理",
						"configuration": {
							"jsScript": "msg.lowTemp='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch_cold",
						"type": "jsTransform",
						"name": "Case2处理",
						"configuration": {
							"jsScript": "msg.cold='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
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
						"toId": "branch_cold",
						"type": "Case2"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_high",
						"type": "Inner1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch_low",
						"type": "Inner2"
					},
					{
						"fromId": "branch_high",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_low",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch_cold",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_nested_inclusive_test3", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度60: Switch Case2匹配 -> 直接到branch_cold
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":60}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
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

		// 解析结果 - 应该只有1个结果（cold分支）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "branch_cold", results[0]["nodeId"])
		t.Logf("✓ 条件分支嵌套包容分支-外层Case2匹配: join成功，收到%d个结果", len(results))
	})
}

// TestInclusiveNestedSwitchWithJoin 测试包容分支嵌套条件分支后join
func TestInclusiveNestedSwitchWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 包容分支 -> 条件分支 -> join
	// 温度35: Inclusive的Case1和Case2都匹配 -> 每个分支内部的Switch再做条件判断
	t.Run("Inclusive_NestedSwitch_AllOuterMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_nested_switch_test1",
				"name": "包容分支嵌套条件分支-外层都匹配",
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
						"id": "switch1",
						"type": "switch",
						"name": "分支1条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=35",
									"then": "Warm"
								},
								{
									"case": "msg.temperature>35",
									"then": "Hot"
								}
							]
						}
					},
					{
						"id": "switch2",
						"type": "switch",
						"name": "分支2条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=40",
									"then": "Medium"
								},
								{
									"case": "msg.temperature>40",
									"then": "VeryHot"
								}
							]
						}
					},
					{
						"id": "warm_node",
						"type": "jsTransform",
						"name": "温暖处理",
						"configuration": {
							"jsScript": "msg.warm='processed'; metadata['level']='warm'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "hot_node",
						"type": "jsTransform",
						"name": "炎热处理",
						"configuration": {
							"jsScript": "msg.hot='processed'; metadata['level']='hot'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "medium_node",
						"type": "jsTransform",
						"name": "中等处理",
						"configuration": {
							"jsScript": "msg.medium='processed'; metadata['level']='medium'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "veryhot_node",
						"type": "jsTransform",
						"name": "极热处理",
						"configuration": {
							"jsScript": "msg.veryhot='processed'; metadata['level']='veryhot'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "inclusive_node",
						"toId": "switch1",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "switch2",
						"type": "Case2"
					},
					{
						"fromId": "switch1",
						"toId": "warm_node",
						"type": "Warm"
					},
					{
						"fromId": "switch1",
						"toId": "hot_node",
						"type": "Hot"
					},
					{
						"fromId": "switch2",
						"toId": "medium_node",
						"type": "Medium"
					},
					{
						"fromId": "switch2",
						"toId": "veryhot_node",
						"type": "VeryHot"
					},
					{
						"fromId": "warm_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "hot_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "medium_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "veryhot_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_nested_switch_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35: Inclusive Case1和Case2都匹配
		// Switch1: Warm(<=35)匹配
		// Switch2: Medium(<=40)匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":35}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
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

		// 解析结果 - 应该有2个结果（warm和medium）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 2, len(results), "应该有2个分支结果")

		nodeIds := make(map[string]bool)
		for _, r := range results {
			nodeIds[r["nodeId"].(string)] = true
		}
		assert.True(t, nodeIds["warm_node"])
		assert.True(t, nodeIds["medium_node"])
		t.Logf("✓ 包容分支嵌套条件分支-外层都匹配: join成功，收到%d个结果", len(results))
	})

	// 测试2: 包容分支 -> 条件分支 -> join，只有外层一个匹配
	// 温度25: Inclusive只有Case1匹配 -> Switch1的Warm匹配
	t.Run("Inclusive_NestedSwitch_PartialOuterMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_nested_switch_test2",
				"name": "包容分支嵌套条件分支-部分外层匹配",
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
						"id": "switch1",
						"type": "switch",
						"name": "分支1条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=35",
									"then": "Warm"
								},
								{
									"case": "msg.temperature>35",
									"then": "Hot"
								}
							]
						}
					},
					{
						"id": "switch2",
						"type": "switch",
						"name": "分支2条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=40",
									"then": "Medium"
								},
								{
									"case": "msg.temperature>40",
									"then": "VeryHot"
								}
							]
						}
					},
					{
						"id": "warm_node",
						"type": "jsTransform",
						"name": "温暖处理",
						"configuration": {
							"jsScript": "msg.warm='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "hot_node",
						"type": "jsTransform",
						"name": "炎热处理",
						"configuration": {
							"jsScript": "msg.hot='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "medium_node",
						"type": "jsTransform",
						"name": "中等处理",
						"configuration": {
							"jsScript": "msg.medium='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "veryhot_node",
						"type": "jsTransform",
						"name": "极热处理",
						"configuration": {
							"jsScript": "msg.veryhot='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "inclusive_node",
						"toId": "switch1",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "switch2",
						"type": "Case2"
					},
					{
						"fromId": "switch1",
						"toId": "warm_node",
						"type": "Warm"
					},
					{
						"fromId": "switch1",
						"toId": "hot_node",
						"type": "Hot"
					},
					{
						"fromId": "switch2",
						"toId": "medium_node",
						"type": "Medium"
					},
					{
						"fromId": "switch2",
						"toId": "veryhot_node",
						"type": "VeryHot"
					},
					{
						"fromId": "warm_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "hot_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "medium_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "veryhot_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_nested_switch_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度25: Inclusive只有Case1匹配(20<=temp<=50)，Case2不匹配(temp>30)
		// Switch1: Warm(<=35)匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":25}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
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

		// 解析结果 - 应该只有1个结果（warm）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "warm_node", results[0]["nodeId"])
		t.Logf("✓ 包容分支嵌套条件分支-部分外层匹配: join成功，收到%d个结果", len(results))
	})

	// 测试3: 包容分支 -> 条件分支 -> join，内部switch无匹配
	// 温度55: Inclusive只有Case2匹配(>30) -> Switch2需要<=40或>40，55>40匹配VeryHot
	t.Run("Inclusive_NestedSwitch_InnerSwitchMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_nested_switch_test3",
				"name": "包容分支嵌套条件分支-内部匹配",
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
						"id": "switch1",
						"type": "switch",
						"name": "分支1条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=35",
									"then": "Warm"
								},
								{
									"case": "msg.temperature>35",
									"then": "Hot"
								}
							]
						}
					},
					{
						"id": "switch2",
						"type": "switch",
						"name": "分支2条件判断",
						"configuration": {
							"cases": [
								{
									"case": "msg.temperature<=40",
									"then": "Medium"
								},
								{
									"case": "msg.temperature>40",
									"then": "VeryHot"
								}
							]
						}
					},
					{
						"id": "warm_node",
						"type": "jsTransform",
						"name": "温暖处理",
						"configuration": {
							"jsScript": "msg.warm='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "hot_node",
						"type": "jsTransform",
						"name": "炎热处理",
						"configuration": {
							"jsScript": "msg.hot='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "medium_node",
						"type": "jsTransform",
						"name": "中等处理",
						"configuration": {
							"jsScript": "msg.medium='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "veryhot_node",
						"type": "jsTransform",
						"name": "极热处理",
						"configuration": {
							"jsScript": "msg.veryhot='processed'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "join_node",
						"type": "join",
						"name": "合并节点",
						"configuration": {
							"timeout": 5
						}
					}
				],
				"connections": [
					{
						"fromId": "inclusive_node",
						"toId": "switch1",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "switch2",
						"type": "Case2"
					},
					{
						"fromId": "switch1",
						"toId": "warm_node",
						"type": "Warm"
					},
					{
						"fromId": "switch1",
						"toId": "hot_node",
						"type": "Hot"
					},
					{
						"fromId": "switch2",
						"toId": "medium_node",
						"type": "Medium"
					},
					{
						"fromId": "switch2",
						"toId": "veryhot_node",
						"type": "VeryHot"
					},
					{
						"fromId": "warm_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "hot_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "medium_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "veryhot_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_nested_switch_test3", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度55: Inclusive Case1不匹配(20<=55<=50 false)，Case2匹配(55>30)
		// Switch2: VeryHot(>40)匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.JSON, originalMetadata, `{"temperature":55}`)

		var wg sync.WaitGroup
		wg.Add(1)
		var resultMsg types.RuleMsg
		var resultErr error

		ruleEngine.OnMsg(testMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			defer wg.Done()
			resultMsg = msg
			resultErr = err
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

		// 解析结果 - 应该只有1个结果（veryhot）
		results, err := parseNestedResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "veryhot_node", results[0]["nodeId"])
		t.Logf("✓ 包容分支嵌套条件分支-内部匹配: join成功，收到%d个结果", len(results))
	})
}
