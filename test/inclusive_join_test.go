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
	"sync"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// TestInclusiveBranchWithJoin 测试包容分支接不通的分支后增加join能不能顺利join和结束
func TestInclusiveBranchWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 包容分支 - 分支1有1个节点，分支2有1个节点
	// 场景: temperature=35 时，Case1 (20<=temp<=50) 和 Case2 (temp>50) 中只有 Case1 匹配
	// Case2 分支不会执行，但join应该能够正常完成
	t.Run("InclusiveBranch_SingleNodePerBranch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_join_test1",
				"name": "包容分支join测试-每分支单节点",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "包容分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node",
						"type": "jsTransform",
						"name": "分支2处理",
						"configuration": {
							"jsScript": "msg.branch2='processed'; metadata['branch2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch2_node",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_join_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35，只有Case1匹配，Case2不匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "INCLUSIVE_TEST", types.JSON, originalMetadata, `{"temperature":35}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证只有分支1被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})

	// 测试2: 包容分支 - 分支1有1个节点，分支2有2个节点
	t.Run("InclusiveBranch_MultipleNodesInBranch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_join_test2",
				"name": "包容分支join测试-分支2多节点",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "包容分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node1",
						"type": "jsTransform",
						"name": "分支2处理-步骤1",
						"configuration": {
							"jsScript": "msg.branch2_step1='processed'; metadata['branch2_step1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node2",
						"type": "jsTransform",
						"name": "分支2处理-步骤2",
						"configuration": {
							"jsScript": "msg.branch2_step2='processed'; metadata['branch2_step2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch2_node1",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node1",
						"toId": "branch2_node2",
						"type": "Success"
					},
					{
						"fromId": "branch2_node2",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_join_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35，只有Case1匹配，Case2不匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "INCLUSIVE_TEST", types.JSON, originalMetadata, `{"temperature":35}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证只有分支1被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})

	// 测试3: 包容分支 - 两个分支都匹配的情况
	t.Run("InclusiveBranch_BothBranchesMatch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "inclusive_join_test3",
				"name": "包容分支join测试-两分支都匹配",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "inclusive_node",
						"type": "inclusive",
						"name": "包容分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node",
						"type": "jsTransform",
						"name": "分支2处理",
						"configuration": {
							"jsScript": "msg.branch2='processed'; metadata['branch2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "inclusive_node",
						"toId": "branch2_node",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("inclusive_join_test3", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35，Case1 (20<=temp<=50) 和 Case2 (temp>30) 都匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "INCLUSIVE_TEST", types.JSON, originalMetadata, `{"temperature":35}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证两个分支都被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})
}

// TestSwitchBranchWithJoin 测试条件分支接不通的分支后增加join能不能顺利join和结束
func TestSwitchBranchWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// 测试1: 条件分支 - 分支1有1个节点，分支2有1个节点
	// 场景: temperature=35 时，Case1 (20<=temp<=50) 匹配，Case2 (temp>50) 不匹配
	t.Run("SwitchBranch_SingleNodePerBranch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_join_test1",
				"name": "条件分支join测试-每分支单节点",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "条件分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node",
						"type": "jsTransform",
						"name": "分支2处理",
						"configuration": {
							"jsScript": "msg.branch2='processed'; metadata['branch2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "branch2_node",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_join_test1", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35，只有Case1匹配，Case2不匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "SWITCH_TEST", types.JSON, originalMetadata, `{"temperature":35}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证只有分支1被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})

	// 测试2: 条件分支 - 分支1有1个节点，分支2有2个节点
	t.Run("SwitchBranch_MultipleNodesInBranch", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_join_test2",
				"name": "条件分支join测试-分支2多节点",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "条件分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node1",
						"type": "jsTransform",
						"name": "分支2处理-步骤1",
						"configuration": {
							"jsScript": "msg.branch2_step1='processed'; metadata['branch2_step1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node2",
						"type": "jsTransform",
						"name": "分支2处理-步骤2",
						"configuration": {
							"jsScript": "msg.branch2_step2='processed'; metadata['branch2_step2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "branch2_node1",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node1",
						"toId": "branch2_node2",
						"type": "Success"
					},
					{
						"fromId": "branch2_node2",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_join_test2", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度35，只有Case1匹配，Case2不匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "SWITCH_TEST", types.JSON, originalMetadata, `{"temperature":35}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证只有分支1被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})

	// 测试3: 条件分支 - 温度60，Case2匹配
	t.Run("SwitchBranch_Case2Match", func(t *testing.T) {
		ruleChainDSL := `{
			"ruleChain": {
				"id": "switch_join_test3",
				"name": "条件分支join测试-Case2匹配",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"nodes": [
					{
						"id": "switch_node",
						"type": "switch",
						"name": "条件分支",
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
						"id": "branch1_node",
						"type": "jsTransform",
						"name": "分支1处理",
						"configuration": {
							"jsScript": "msg.branch1='processed'; metadata['branch1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node1",
						"type": "jsTransform",
						"name": "分支2处理-步骤1",
						"configuration": {
							"jsScript": "msg.branch2_step1='processed'; metadata['branch2_step1']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
						}
					},
					{
						"id": "branch2_node2",
						"type": "jsTransform",
						"name": "分支2处理-步骤2",
						"configuration": {
							"jsScript": "msg.branch2_step2='processed'; metadata['branch2_step2']='done'; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};"
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
						"toId": "branch1_node",
						"type": "Case1"
					},
					{
						"fromId": "switch_node",
						"toId": "branch2_node1",
						"type": "Case2"
					},
					{
						"fromId": "branch1_node",
						"toId": "join_node",
						"type": "Success"
					},
					{
						"fromId": "branch2_node1",
						"toId": "branch2_node2",
						"type": "Success"
					},
					{
						"fromId": "branch2_node2",
						"toId": "join_node",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := rulego.New("switch_join_test3", []byte(ruleChainDSL), rulego.WithConfig(config))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 温度60，只有Case2匹配，Case1不匹配
		originalMetadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "SWITCH_TEST", types.JSON, originalMetadata, `{"temperature":60}`)

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

		// 等待处理完成
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// 处理完成
		case <-time.After(10 * time.Second):
			t.Fatal("测试超时：join节点未能在规定时间内完成")
		}

		// 验证结果
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// 验证只有分支2被处理
		t.Logf("结果数据: %s", resultMsg.GetData())
		t.Logf("结果关系类型: %s", resultRelationType)
	})
}
