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

package common

import (
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestRefNode(t *testing.T) {
	var targetNodeType = "ref"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &RefNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		// Test successful initialization with valid targetId
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"targetId": "test_node",
		}, Registry)
		assert.Nil(t, err)
		refNode := node.(*RefNode)
		assert.Equal(t, "test_node", refNode.Config.TargetId)
		assert.NotNil(t, refNode.targetIdTemplate)
		assert.False(t, refNode.targetIdTemplate.HasVar())
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		// Test that initialization fails with empty configuration
		_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "targetId is empty"))
	})

	t.Run("OnMsg", func(t *testing.T) {
		// Test initialization failure with empty targetId
		t.Run("EmptyTargetId", func(t *testing.T) {
			_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "targetId is empty"))
		})

		// Test valid external chain reference
		t.Run("ExternalChainReference", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"targetId": "chain01:node01",
			}, Registry)
			assert.Nil(t, err)
			refNode := node.(*RefNode)
			assert.Equal(t, "chain01:node01", refNode.Config.TargetId)
		})

		// Test valid local node reference
		t.Run("LocalNodeReference", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"targetId": "node01",
			}, Registry)
			assert.Nil(t, err)
			refNode := node.(*RefNode)
			assert.Equal(t, "node01", refNode.Config.TargetId)
		})

		// Test message handling with non-existent target nodes
		t.Run("MessageHandling", func(t *testing.T) {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")

			testMsg := test.Msg{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			}

			// Test external chain reference (should fail since chain doesn't exist)
			t.Run("ExternalChain", func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
					"targetId": "chain01:node01",
				}, Registry)
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
					assert.Equal(t, types.Failure, result.relationType)
					assert.NotNil(t, result.err)
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})

			// Test local node reference (should fail since node doesn't exist)
			t.Run("LocalNode", func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
					"targetId": "node01",
				}, Registry)
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
					assert.Equal(t, types.Failure, result.relationType)
					assert.NotNil(t, result.err)
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})
		})
	})
}

// TestRefNodeDynamicTargetIdInit 含 ${} 的 targetId 部署期只编译模板，执行时求值
func TestRefNodeDynamicTargetIdInit(t *testing.T) {
	node, err := test.CreateAndInitNode("ref", types.Configuration{
		"targetId": "${metadata.targetChain}:node_x",
	}, Registry)
	assert.Nil(t, err)
	refNode := node.(*RefNode)
	assert.NotNil(t, refNode.targetIdTemplate)
	assert.True(t, refNode.targetIdTemplate.HasVar())
}

// TestRefNodeResolveTargetId 执行时求值：metadata 驱动 chainId/nodeId
func TestRefNodeResolveTargetId(t *testing.T) {
	node, err := test.CreateAndInitNode("ref", types.Configuration{
		"targetId": "${metadata.targetChain}:node_x",
	}, Registry)
	assert.Nil(t, err)
	refNode := node.(*RefNode)

	ctx := test.NewRuleContextFull(types.NewConfig(), refNode, nil, nil)
	metadata := types.NewMetadata()
	metadata.PutValue("targetChain", "chain_a")
	chainId, nodeId, err := refNode.resolveTargetId(ctx, types.NewMsg(0, "T", types.JSON, metadata, "{}"))
	assert.Nil(t, err)
	assert.Equal(t, "chain_a", chainId)
	assert.Equal(t, "node_x", nodeId)

	//变量缺失：占位符求值为空，解析报错
	metadata = types.NewMetadata()
	_, _, err = refNode.resolveTargetId(ctx, types.NewMsg(0, "T", types.JSON, metadata, "{}"))
	assert.NotNil(t, err)
}

// TestRefNodeDynamicLocalRef 本链引用同样支持 ${} 动态指定节点
func TestRefNodeDynamicLocalRef(t *testing.T) {
	node, err := test.CreateAndInitNode("ref", types.Configuration{
		"targetId": "${metadata.targetNodeId}",
	}, Registry)
	assert.Nil(t, err)

	testMsg := test.Msg{
		MetaData:   types.BuildMetadata(map[string]string{"targetNodeId": "node_x"}),
		MsgType:    "ACTIVITY_EVENT2",
		Data:       "{\"temperature\":60}",
		AfterSleep: time.Millisecond * 200,
	}

	type testResult struct {
		relationType string
		msg          types.RuleMsg
	}
	resultChan := make(chan testResult, 1)
	callback := func(msg types.RuleMsg, relationType string, err error) {
		resultChan <- testResult{relationType: relationType, msg: msg}
	}

	test.NodeOnMsgWithChildrenAndConfig(t, types.NewConfig(), node, []test.Msg{testMsg},
		map[string]types.Node{"node_x": &markNode{}}, callback)

	select {
	case result := <-resultChan:
		assert.Equal(t, types.Success, result.relationType)
		assert.Equal(t, "X", result.msg.Metadata.GetValue("touched"))
	case <-time.After(time.Second):
		t.Fatal("Test timed out")
	}
}

//markNode 测试桩：向 metadata.touched 追加标记后走 Success
type markNode struct{}

func (n *markNode) Type() string                                              { return "markNode" }
func (n *markNode) New() types.Node                                           { return &markNode{} }
func (n *markNode) Init(_ types.Config, _ types.Configuration) error          { return nil }
func (n *markNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	msg.Metadata.PutValue("touched", msg.Metadata.GetValue("touched")+"X")
	ctx.TellSuccess(msg)
}
func (n *markNode) Destroy()                                                 {}
func (n *markNode) Def() types.ComponentForm                                 { return types.ComponentForm{} }

// TestParseRefTargetId 静态 targetId 拆分
func TestParseRefTargetId(t *testing.T) {
	chainId, nodeId := "", ""
	assert.Nil(t, parseRefTargetId("chain01:node01", &chainId, &nodeId))
	assert.Equal(t, "chain01", chainId)
	assert.Equal(t, "node01", nodeId)

	assert.Nil(t, parseRefTargetId("node01", &chainId, &nodeId))
	assert.Equal(t, "", chainId)
	assert.Equal(t, "node01", nodeId)

	assert.NotNil(t, parseRefTargetId("a:b:c", &chainId, &nodeId))
	assert.NotNil(t, parseRefTargetId(":node01", &chainId, &nodeId))
}
