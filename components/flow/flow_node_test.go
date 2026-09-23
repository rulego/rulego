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

// 外部测试包：真引擎用例需要 import engine/rulego，而 engine 的注册表会播种组件包，
// 包内测试（package flow）import 它们会形成循环
package flow_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/flow"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestFlowNode(t *testing.T) {
	var targetNodeType = "flow"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &flow.ChainNode{}, types.Configuration{}, flow.Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		// Test successful initialization with empty configuration
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, flow.Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		// Test successful initialization with default configuration
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, flow.Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)
	})

	t.Run("OnMsg", func(t *testing.T) {
		// Test cases with different configurations
		testCases := []struct {
			name             string
			config           types.Configuration
			expectedRelation string
		}{
			{
				name: "RuleTarget",
				config: types.Configuration{
					"targetId": "rule01",
				},
				expectedRelation: types.Success,
			},
			{
				name: "ToTrueWithoutExtend",
				config: types.Configuration{
					"targetId": "toTrue",
					"extend":   false,
				},
				expectedRelation: types.Success,
			},
			{
				name: "ToTrueWithExtend",
				config: types.Configuration{
					"targetId": "toTrue",
					"extend":   true,
				},
				expectedRelation: types.True,
			},
			{
				name: "NotFoundWithExtend",
				config: types.Configuration{
					"targetId": "notfound",
					"extend":   true,
				},
				expectedRelation: types.Failure,
			},
		}

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		testMsg := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT2",
			Data:       "{\"temperature\":60}",
			AfterSleep: time.Millisecond * 200,
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, tc.config, flow.Registry)
				assert.Nil(t, err)

				// Use a struct to pass results through channel to avoid data race
				type testResult struct {
					relationType string
					err          error
				}
				resultChan := make(chan testResult, 1)

				callback := func(msg types.RuleMsg, relationType string, err error) {
					resultChan <- testResult{
						relationType: relationType,
						err:          err,
					}
				}

				test.NodeOnMsgWithChildrenAndConfig(t, types.NewConfig(), node, []test.Msg{testMsg}, nil, callback)

				select {
				case result := <-resultChan:
					// Use simple string comparison instead of assert.Equal to avoid circular references
					if result.relationType != tc.expectedRelation {
						t.Errorf("Expected relation type %s, got %s", tc.expectedRelation, result.relationType)
					}
					if tc.expectedRelation == types.Failure {
						assert.NotNil(t, result.err)
					}
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})
		}
	})

	t.Run("EmptyConfigInit", func(t *testing.T) {
		// Test that ChainNode can initialize with empty config (unlike RefNode)
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, flow.Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)

		chainNode := node.(*flow.ChainNode)
		// With empty config, TargetId should be empty string
		assert.Equal(t, "", chainNode.Config.TargetId)
		assert.Equal(t, false, chainNode.Config.Extend) // default value
	})
}

// TestFlowNodeTargetIdParse targetId 解析规则，解析成功的值语义由 TestFlowNodeStartNode 行为验证
func TestFlowNodeTargetIdParse(t *testing.T) {
	t.Run("ChainAndNode", func(t *testing.T) {
		_, err := test.CreateAndInitNode("flow", types.Configuration{"targetId": " chain_01 : node_02 "}, flow.Registry)
		assert.Nil(t, err)
	})
	t.Run("ThreeSegments", func(t *testing.T) {
		_, err := test.CreateAndInitNode("flow", types.Configuration{"targetId": "a:b:c"}, flow.Registry)
		assert.NotNil(t, err)
	})
	t.Run("EmptyNodePart", func(t *testing.T) {
		_, err := test.CreateAndInitNode("flow", types.Configuration{"targetId": "chain_01:"}, flow.Registry)
		assert.NotNil(t, err)
	})
	t.Run("EmptyChainPart", func(t *testing.T) {
		_, err := test.CreateAndInitNode("flow", types.Configuration{"targetId": ":node_02"}, flow.Registry)
		assert.NotNil(t, err)
	})
}

// TestChainNodePickerHint flow/ref 的 targetId 需把选择器 component hint 下发给前端表单
func TestChainNodePickerHint(t *testing.T) {
	forms := engine.Registry.GetComponentForms()
	flowField, ok := forms["flow"].Fields.GetField("targetId")
	assert.True(t, ok)
	assert.Equal(t, "RuleChainSelector", flowField.Component["type"])
	assert.Equal(t, true, flowField.Component["nodePicker"])

	refField, ok := forms["ref"].Fields.GetField("targetId")
	assert.True(t, ok)
	assert.Equal(t, "RuleChainSelector", refField.Component["type"])
	assert.Equal(t, true, refField.Component["nodePicker"])
	assert.Equal(t, true, refField.Component["allowSelf"])
	assert.Equal(t, true, refField.Component["requireNode"])
}

// TestFlowNodeStartNode targetId 带 {chainId}:{nodeId} 起点时从指定节点执行子链
func TestFlowNodeStartNode(t *testing.T) {
	const subChainId = "flow_start_test_sub"
	const parentChainId = "flow_start_test_parent"

	engine.DefaultPool.Del(subChainId)
	engine.DefaultPool.Del(parentChainId)
	t.Cleanup(func() {
		engine.DefaultPool.Del(subChainId)
		engine.DefaultPool.Del(parentChainId)
	})

	//n_a/n_b 分别向 metadata.touched 追加 A/B，用于判断哪些节点执行过
	subDSL := `{
		"ruleChain": {"id": "flow_start_test_sub"},
		"metadata": {
			"nodes": [
				{"id": "n_a", "type": "jsTransform", "name": "A", "configuration": {"jsScript": "metadata['touched']=(metadata['touched']||'')+'A'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"}},
				{"id": "n_b", "type": "jsTransform", "name": "B", "configuration": {"jsScript": "metadata['touched']=(metadata['touched']||'')+'B'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"}}
			],
			"connections": [{"fromId": "n_a", "toId": "n_b", "type": "Success"}]
		}
	}`
	_, err := rulego.New(subChainId, []byte(subDSL))
	assert.Nil(t, err)

	type chainResult struct {
		relationType string
		err          error
		msg          types.RuleMsg
	}

	runParent := func(t *testing.T, targetId string, extend bool) []chainResult {
		//同 ID 重复 New 不会重建引擎，先删保证每个用例的 DSL 都生效
		engine.DefaultPool.Del(parentChainId)
		parentDSL := fmt.Sprintf(`{
			"ruleChain": {"id": "flow_start_test_parent"},
			"metadata": {
				"nodes": [
					{"id": "s_flow", "type": "flow", "name": "flow", "configuration": {"targetId": "%s", "extend": %t}}
				],
				"connections": []
			}
		}`, targetId, extend)
		_, err := rulego.New(parentChainId, []byte(parentDSL))
		assert.Nil(t, err)

		metadata := types.NewMetadata()
		metadata.PutValue("productType", "test")
		msg := types.NewMsg(0, "START_TEST", types.JSON, metadata, "{\"temperature\":60}")

		resultChan := make(chan chainResult, 8)
		parentEngine, _ := engine.DefaultPool.Get(parentChainId)
		parentEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			resultChan <- chainResult{relationType: relationType, err: err, msg: onEndMsg}
		}))

		var results []chainResult
		timeout := time.After(3 * time.Second)
		select {
		case r := <-resultChan:
			results = append(results, r)
		case <-timeout:
			t.Fatalf("timeout waiting for onEnd, got %d results", len(results))
		}
		return results
	}

	t.Run("WholeChain", func(t *testing.T) {
		results := runParent(t, subChainId, false)
		assert.Equal(t, types.Success, results[0].relationType)
		assert.Equal(t, "AB", results[0].msg.Metadata.GetValue("touched"))
	})

	t.Run("StartFromNode", func(t *testing.T) {
		results := runParent(t, subChainId+":n_b", false)
		assert.Equal(t, types.Success, results[0].relationType)
		//从 n_b 开始，n_a 不应执行
		assert.Equal(t, "B", results[0].msg.Metadata.GetValue("touched"))
		//来源键：调用方链与 flow 节点
		assert.Equal(t, parentChainId, results[0].msg.Metadata.GetValue(types.KeyFromChainId))
		assert.Equal(t, "s_flow", results[0].msg.Metadata.GetValue(types.KeyFromNodeId))
	})

	t.Run("StartFromNodeExtend", func(t *testing.T) {
		results := runParent(t, subChainId+":n_b", true)
		assert.Equal(t, types.Success, results[0].relationType)
		assert.Equal(t, "B", results[0].msg.Metadata.GetValue("touched"))
	})

	t.Run("StartNodeNotFound", func(t *testing.T) {
		results := runParent(t, subChainId+":n_zzz", false)
		assert.Equal(t, types.Failure, results[0].relationType)
		assert.NotNil(t, results[0].err)
	})

	t.Run("CallerMsgNotPolluted", func(t *testing.T) {
		metadata := types.NewMetadata()
		metadata.PutValue("productType", "test")
		msg := types.NewMsg(0, "START_TEST", types.JSON, metadata, "{\"temperature\":60}")

		parentDSL := fmt.Sprintf(`{
			"ruleChain": {"id": "flow_start_test_parent"},
			"metadata": {
				"nodes": [
					{"id": "s_flow", "type": "flow", "name": "flow", "configuration": {"targetId": "%s"}}
				],
				"connections": []
			}
		}`, subChainId+":n_b")
		engine.DefaultPool.Del(parentChainId)
		_, err := rulego.New(parentChainId, []byte(parentDSL))
		assert.Nil(t, err)

		resultChan := make(chan chainResult, 1)
		parentEngine, _ := engine.DefaultPool.Get(parentChainId)
		parentEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, onEndMsg types.RuleMsg, err error, relationType string) {
			resultChan <- chainResult{relationType: relationType}
		}))
		select {
		case r := <-resultChan:
			assert.Equal(t, types.Success, r.relationType)
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for onEnd")
		}
		//来源键写入的是消息副本，调用方原消息不应被污染
		assert.Equal(t, "", msg.Metadata.GetValue(types.KeyFromChainId))
		assert.Equal(t, "", msg.Metadata.GetValue(types.KeyFromNodeId))
	})
}
