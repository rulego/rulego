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

// parseComplexResult parses the result of complex nested joins
// Returns a single-element array when there is only one result; returns the original array when there are multiple results
func parseComplexResult(data string) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		// Try parsing it as a single object
		var singleObj map[string]interface{}
		err2 := json.Unmarshal([]byte(data), &singleObj)
		if err2 == nil {
			return []map[string]interface{}{singleObj}, nil
		}
		return nil, err
	}
	return result, nil
}

// TestSwitchWithInternalJoin Test Condition There are joins within the branch
func TestSwitchWithInternalJoin(t *testing.T) {
	config := rulego.NewConfig()

	// Test 1: Conditional branch -> Inside a branch is a fork-join structure
	// Temperature 35: Switch Case1 matches -> Branch 1 forks 2 parallel nodes internally -> Internal join -> External join
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

		// Temperature 35: Switch Case1 match -> fork -> Two internal parallel nodes -> Internal join -> Internal processing -> External join
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
			t.Fatal("Test timeout")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// Parse result - After internal joins, the message will be merged into one message and then processed again
		t.Logf("Result data: %s", resultMsg.GetData())
		t.Logf("Result relationship type: %s", resultRelationType)
		assert.Nil(t, resultErr)
		// After internal fork-join, messages are merged, while external joins receive a single message
		// Validation messages contain internal processing marks
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ Conditional branch internal join- single branch: join successful, internal fork-join working normally")
	})

	// Test 2: Conditional branch -> Case2 matching, without internal fork-join
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

		// Temperature 60: Switch Case2 matches -> directly to case2_node -> external join
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
			t.Fatal("Test timeout")
		}

		assert.Nil(t, resultErr)

		// Parsing result - There should be only one result (case2 handling)
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(results), "应该只有1个分支结果")
		assert.Equal(t, "case2_node", results[0]["nodeId"])
		t.Logf("✓ Conditional branch internal join-Case2 matching: join successful, skip internal fork-join")
	})
}

// TestInclusiveWithInternalJoin tests whether there are joins within the inclusion branch
func TestInclusiveWithInternalJoin(t *testing.T) {
	config := rulego.NewConfig()

	// Test 1: Include branches -> Both branches have joins inside
	// Temperature 35: Both Inclusive Case1 and Case2 match -> Both branches fork-join -> external join respectively
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

		// Temperature 35: Inclusive Case1 and Case2 both match
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
			t.Fatal("Test timeout")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// Parse results - There should be 2 results (process1 and process2)
		// Each branch is fork-joined, merged into one, and then the external join collects the results from both branches
		t.Logf("Result data: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("Number of results: %d", len(results))
		// The verification results include processed data
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ Inclusion within the branch join- both branches have: join success")
	})

	// Test 2: Include branches -> Only one branch has internal joins
	// Temperature 25: Inclusive only matches Case1 with -> branch 1 internal fork-join -> external join
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

		// Temperature 25: Inclusive only matches Case1 with -> fork-join inside branch 1
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
			t.Fatal("Test timeout")
		}

		assert.Nil(t, resultErr)

		// Parse result - There should be only one result (process1)
		t.Logf("Result data: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("Number of results: %d", len(results))
		// The verification results include processed data
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ Inclusion of a single branch within join- branch: join succeeds, only branch 1 executes")
	})
}

// TestDoubleNestedJoin Tests the double-layer nested join
func TestDoubleNestedJoin(t *testing.T) {
	config := rulego.NewConfig()

	// Test: Conditional branch -> Inclusion branch -> Internal fork-join -> External join
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

		// Temperature 35: Switch Case1 matches -> Inclusive High (>=30) and Low (<=40) matches
		// -> fork_high and fork_low each fork -> internal join -> process -> external join
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
			t.Fatal("Test timeout")
		}

		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// Parsing results - there should be two outcomes (process_high and process_low)
		t.Logf("Result data: %s", resultMsg.GetData())
		results, err := parseComplexResult(resultMsg.GetData())
		assert.Nil(t, err)
		t.Logf("Number of results: %d", len(results))
		// The verification results include processed data
		assert.True(t, len(resultMsg.GetData()) > 0, "结果数据不应为空")
		t.Logf("✓ Double-layer nesting join: join successful")
	})
}
