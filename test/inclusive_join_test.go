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

// TestInclusiveBranchWithJoin tests whether the inclusion branch can join smoothly and close after adding branches that cannot connect
func TestInclusiveBranchWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// Test 1: Include branches - Branch 1 has 1 node, Branch 2 has 1 node
	// Scenario: When temperature=35, only Case1 matches in Case1 (20<=temp<=50) and Case2 (temp>50).
	// Case2: The branch will not execute, but the join should be able to complete normally
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

		// Temperature 35, only Case1 matches, Case2 does not
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Only branch 1 is processed for verification
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})

	// Test 2: Include branches – Branch 1 has 1 node, Branch 2 has 2 nodes
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

		// Temperature 35, only Case1 matches, Case2 does not
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Only branch 1 is processed for verification
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})

	// Test 3: Inclusion of branches – a case where both branches match
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

		// Temperature 35, Case1 (20< = temp< = 50) and Case2 (temp> 30) both match
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Verify that both branches are processed
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})
}

// TestSwitchBranchWithJoin Test Conditions Branch WithJoin Adds joins after branches that fail to connect, whether the join joins smoothly and closes
func TestSwitchBranchWithJoin(t *testing.T) {
	config := rulego.NewConfig()

	// Test 1: Conditional branch - Branch 1 has 1 node, branch 2 has 1 node
	// Scenario: When temperature=35, Case1 (20< = temp < = 50) matches, Case2 (temp > 50) does not match
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

		// Temperature 35, only Case1 matches, Case2 does not
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Only branch 1 is processed for verification
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})

	// Test 2: Conditional branch - Branch 1 has 1 node, Branch 2 has 2 nodes
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

		// Temperature 35, only Case1 matches, Case2 does not
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Only branch 1 is processed for verification
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})

	// Test 3: Conditional branch - temperature 60, Case2 matches
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

		// Temperature 60°C, only Case2 matches, Case1 does not match
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

		// Wait for processing to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Processing complete
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout: join node fails to complete within the specified time")
		}

		// Verify the results
		assert.Nil(t, resultErr, "不应该有错误")
		assert.Equal(t, types.Success, resultRelationType, "应该是Success关系")

		// Only branch 2 is processed for verification
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
	})
}
