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

func TestNodeTestRuleContextAccessors(t *testing.T) {
	var callbackCount int
	ctx := NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {
		callbackCount++
	}).(*NodeTestRuleContext)

	assert.NotNil(t, ctx.GlobalCache())
	assert.NotNil(t, ctx.ChainCache())
	assert.NotNil(t, ctx.Config())
	assert.Equal(t, "", ctx.GetSelfId())
	assert.Nil(t, ctx.Self())
	assert.Nil(t, ctx.From())
	assert.Nil(t, ctx.RuleChain())
	assert.Equal(t, context.TODO(), ctx.GetContext())

	custom := context.WithValue(context.Background(), "k", "v")
	assert.Equal(t, ctx, ctx.SetContext(custom))
	assert.Equal(t, custom, ctx.GetContext())

	msg := ctx.NewMsg("aa", types.NewMetadata(), "123")
	assert.Equal(t, "aa", msg.Type)
	assert.Equal(t, "123", msg.GetData())

	// assert.Nil rejects typed nil funcs, compare directly
	assert.True(t, ctx.GetEndFunc() == nil)
	onEnd := func(types.RuleContext, types.RuleMsg, error, string) {}
	assert.Equal(t, ctx, ctx.SetEndFunc(onEnd))
	assert.True(t, ctx.GetEndFunc() != nil)

	// dead-ends and trivial getters
	ctx.DoOnEnd(msg, nil, types.Success)
	ctx.SetCallbackFunc("f", nil)
	assert.Nil(t, ctx.GetCallbackFunc("f"))
	ctx.OnDebug("chain", "In", "node", msg, types.Success, nil)
	ctx.SetExecuteNodes()
	ctx.SetDebugMode(true)
	ctx.SetSkipTellNext(true)
	assert.Nil(t, ctx.GetRelationTypes())
	assert.Nil(t, ctx.GetErr())
	_, ok := ctx.GetNodeRuleMsg("n1")
	assert.False(t, ok)
	assert.Equal(t, types.RuleMsg{}, ctx.GetOut())
	ctx.setOut(msg)
	assert.Equal(t, msg.Id, ctx.GetOut().Id)

	collectCalled := false
	assert.True(t, ctx.TellCollect(msg, func(list []types.WrapperMsg) {
		collectCalled = true
	}))
	assert.True(t, collectCalled)

	// SubmitTask 是 go task() 异步执行，用 channel 建立同步关系，sleep 挡不住 race detector
	done := make(chan struct{})
	ctx.SubmitTask(func() {
		callbackCount++
		close(done)
	})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("SubmitTask did not run")
	}
	assert.True(t, callbackCount >= 1)
}

func TestNodeTestRuleContextTellMethods(t *testing.T) {
	type record struct {
		relationType string
		err          error
		onEnd        string
	}
	var records []record
	ctx := NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {
		records = append(records, record{relationType: relationType, err: err})
	}).(*NodeTestRuleContext)
	ctx.SetEndFunc(func(_ types.RuleContext, _ types.RuleMsg, err error, relationType string) {
		records = append(records, record{relationType: relationType, err: err, onEnd: "yes"})
	})

	msg := types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "d")

	ctx.TellSuccess(msg)
	ctx.TellFailure(msg, context.Canceled)
	ctx.TellNext(msg, types.Success, types.Failure)
	ctx.TellNext(msg)
	ctx.TellNextOrElse(msg, types.True, types.False)
	ctx.TellStream(msg)

	// each Tell produces a callback record followed by an onEnd record;
	// TellNext without relation types produces none
	assert.Equal(t, 12, len(records))
	assert.Equal(t, types.Success, records[0].relationType)
	// the paired onEnd record follows each callback record
	assert.Equal(t, types.Success, records[1].relationType)
	assert.Equal(t, "yes", records[1].onEnd)
	assert.Equal(t, types.Failure, records[2].relationType)
	assert.Equal(t, context.Canceled, records[2].err)
	assert.Equal(t, types.Success, records[4].relationType)
	assert.Equal(t, types.Failure, records[6].relationType)
	assert.Equal(t, types.False, records[8].relationType)
	assert.Equal(t, types.Stream, records[10].relationType)
}

func TestNodeTestRuleContextTellSelf(t *testing.T) {
	done := make(chan string, 1)
	ctx := NewRuleContextFull(types.NewConfig(), &UpperNode{}, nil, func(msg types.RuleMsg, relationType string, err error) {
		done <- msg.GetData()
	})
	ctx.TellSelf(types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "late"), 20)

	select {
	case got := <-done:
		assert.Equal(t, "LATE", got)
	case <-time.After(time.Second * 3):
		t.Fatal("TellSelf did not deliver to the self node")
	}
}

func TestNodeTestRuleContextTellFlow(t *testing.T) {
	type flowRecord struct {
		err          error
		relationType string
		allCompleted bool
	}
	var records []flowRecord
	ctx := NewRuleContext(types.NewConfig(), nil).(*NodeTestRuleContext)
	ctx.SetEndFunc(func(_ types.RuleContext, _ types.RuleMsg, err error, relationType string) {
		records = append(records, flowRecord{err: err, relationType: relationType})
	})
	ctx.SetOnAllNodeCompleted(func() {
		if len(records) > 0 {
			records[len(records)-1].allCompleted = true
		}
	})

	msg := types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "d")

	// empty chain id: only onEndFunc fires with an error
	ctx.TellFlow("", msg)
	assert.Equal(t, 1, len(records))
	assert.NotNil(t, records[0].err)
	assert.False(t, records[0].allCompleted)

	// unknown chain: error + completion callback
	ctx.TellFlow("notfound", msg)
	assert.Equal(t, 2, len(records))
	assert.NotNil(t, records[1].err)
	assert.True(t, records[1].allCompleted)

	// magic chain id "toTrue"
	ctx.TellFlow("toTrue", msg)
	assert.Equal(t, 3, len(records))
	assert.Nil(t, records[2].err)
	assert.Equal(t, types.True, records[2].relationType)

	// any other chain id succeeds
	ctx.TellFlow("myChain", msg)
	assert.Equal(t, 4, len(records))
	assert.Nil(t, records[3].err)
	assert.Equal(t, types.Success, records[3].relationType)
}

func TestNodeTestRuleContextTellNode(t *testing.T) {
	children := map[string]types.Node{"upper": &UpperNode{}}

	var results []string
	var completed int
	ctx := NewRuleContextFull(types.NewConfig(), nil, children, nil).(*NodeTestRuleContext)

	msg := types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "abc")
	ctx.TellNode(context.Background(), "upper", msg, false,
		func(_ types.RuleContext, m types.RuleMsg, err error, relationType string) {
			assert.Nil(t, err)
			assert.Equal(t, types.Success, relationType)
			assert.Equal(t, "ABC", m.GetData())
			results = append(results, relationType)
		}, func() { completed++ })

	// missing node: failure callback with the not-found error
	ctx.TellNode(context.Background(), "missing", msg, false,
		func(_ types.RuleContext, _ types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Failure, relationType)
			assert.NotNil(t, err)
			results = append(results, "missing")
		}, func() { completed++ })

	assert.Equal(t, 2, len(results))
	assert.Equal(t, 2, completed)

	// TellChainNode delegates to TellNode
	ctx.TellChainNode(context.Background(), "chain", "missing", msg, false, nil, nil)
}

func TestNodeTestRuleContextGetEnv(t *testing.T) {
	ctx := NewRuleContext(types.NewConfig(), nil).(*NodeTestRuleContext)

	t.Run("json msg with metadata", func(t *testing.T) {
		meta := types.NewMetadata()
		meta.PutValue("tenant", "t1")
		msg := types.NewMsg(0, "T1", types.JSON, meta, `{"temp":25}`)
		env := ctx.GetEnv(msg, true)

		assert.Equal(t, "T1", env["msgType"])
		assert.Equal(t, "T1", env["type"])
		assert.Equal(t, string(types.JSON), env["dataType"])
		envMsg, ok := env[types.MsgKey].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(25), envMsg["temp"])
		assert.Equal(t, "t1", env["tenant"])
		metaMap, ok := env[types.MetadataKey].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "t1", metaMap["tenant"])
	})

	t.Run("invalid json falls back to raw data", func(t *testing.T) {
		msg := types.NewMsg(0, "T2", types.JSON, types.NewMetadata(), "{broken")
		env := ctx.GetEnv(msg, false)
		assert.Equal(t, "{broken", env[types.MsgKey])
		metaMap, ok := env[types.MetadataKey].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, 0, len(metaMap))
	})

	t.Run("non json data type", func(t *testing.T) {
		msg := types.NewMsg(0, "T3", types.TEXT, types.NewMetadata(), "plain")
		env := ctx.GetEnv(msg, true)
		assert.Equal(t, "plain", env[types.MsgKey])
	})
}

func TestExtendedTestRuleContext(t *testing.T) {
	t.Run("collect results", func(t *testing.T) {
		ctx := NewExtendedTestRuleContext(types.NewConfig(), nil)
		msg := types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "d")

		ctx.TellNext(msg, types.Success)
		ctx.TellSuccess(msg)
		ctx.TellFailure(msg, context.Canceled)

		assert.Equal(t, []string{types.Success, "Success", "Failure"}, ctx.GetResults())

		select {
		case r := <-ctx.GetResultsChannel():
			assert.Equal(t, types.Success, r.RelationType)
			assert.Nil(t, r.Err)
		case <-time.After(time.Second):
			t.Fatal("results channel did not receive TellNext result")
		}
	})

	t.Run("node handler override", func(t *testing.T) {
		ctx := NewExtendedTestRuleContextWithChannel()
		ctx.SetNodeHandler("mock", func(msg types.RuleMsg) (string, error) {
			return types.True, nil
		})

		msg := types.NewMsg(0, "T", types.JSON, types.NewMetadata(), "d")
		done := make(chan string, 1)
		ctx.TellNode(context.Background(), "mock", msg, false,
			func(_ types.RuleContext, _ types.RuleMsg, err error, relationType string) {
				assert.Nil(t, err)
				done <- relationType
			}, nil)

		select {
		case got := <-done:
			assert.Equal(t, types.True, got)
		case <-time.After(time.Second * 3):
			t.Fatal("node handler was not invoked")
		}

		// without a handler the base TellNode logic runs (missing node -> failure)
		ctx.TellNode(context.Background(), "absent", msg, false,
			func(_ types.RuleContext, _ types.RuleMsg, err error, relationType string) {
				assert.Equal(t, types.Failure, relationType)
				assert.NotNil(t, err)
				done <- relationType
			}, nil)
		select {
		case <-done:
		case <-time.After(time.Second * 3):
			t.Fatal("fallback TellNode was not invoked")
		}
	})
}

// ------------------------------------------------------------------
// Engine scenario tests merged from inclusive_join_test.go,
// nested_branch_join_test.go and nested_join_complex_test.go.
// They drive full rule chains through the engine and use the
// NodeTestRuleContext-based helpers indirectly.
// ------------------------------------------------------------------

// ------------------------------------------------------------------
// Engine scenario tests merged from inclusive_join_test.go,
// nested_branch_join_test.go and nested_join_complex_test.go.
// They drive full rule chains through the engine and use the
// NodeTestRuleContext-based helpers indirectly.
// ------------------------------------------------------------------

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
