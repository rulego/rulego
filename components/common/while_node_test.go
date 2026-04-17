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
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestWhileNode(t *testing.T) {
	var targetNodeType = "while"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &WhileNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"condition": "${msg.count} < 5",
			"do":        "s1",
		}, types.Configuration{
			"condition": "${msg.count} < 5",
			"do":        "s1",
			"mode":      2, // Default mode from New() is preserved
		}, Registry)

		// Test empty condition
		_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "",
			"do":        "s1",
		}, Registry)
		assert.NotNil(t, err)

		// Test empty do
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "true",
		}, Registry)
		assert.NotNil(t, err)

		// Test invalid do format
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "true",
			"do":        "chain:s1:invalid",
		}, Registry)
		assert.NotNil(t, err)
	})

	t.Run("OnMsg", func(t *testing.T) {
		// Register a test function node that increments 'count' in metadata
		action.Functions.Register("whileLoopTest", func(ctx types.RuleContext, msg types.RuleMsg) {
			countStr := msg.Metadata.GetValue("count")
			count, _ := strconv.Atoi(countStr)
			count++
			msg.Metadata.PutValue("count", strconv.Itoa(count))
			msg.SetData(strconv.Itoa(count))
			ctx.TellSuccess(msg)
		})

		// Create the 'do' node
		doNode, err := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "whileLoopTest",
		}, action.Registry)
		assert.Nil(t, err)

		childrenNodes := map[string]types.Node{
			"node1": doNode,
		}

		// Node 1: Normal loop with ReplaceValues (2)
		// Condition: count < 3. Starts at 0. Should run for 0, 1, 2. Ends at 3.
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 3",
			"do":        "node1",
			"mode":      2,
		}, Registry)
		assert.Nil(t, err)

		// Node 2: Loop with DoNotProcess (0)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 3",
			"do":        "node1",
			"mode":      0,
		}, Registry)
		assert.Nil(t, err)

		// Node 3: Loop with MergeValues (1)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 3",
			"do":        "node1",
			"mode":      1,
		}, Registry)
		assert.Nil(t, err)

		// Node 4: Loop with default configuration (should be mode 2 because Map2Struct preserves New() defaults)
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 3",
			"do":        "node1",
		}, Registry)
		assert.Nil(t, err)

		// Node 5: Loop condition evaluation error
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "invalid syntax!!",
			"do":        "node1",
			"mode":      2,
		}, Registry)
		assert.NotNil(t, err)

		// Node 6: Loop with node execution error
		action.Functions.Register("errorLoopTest", func(ctx types.RuleContext, msg types.RuleMsg) {
			ctx.TellFailure(msg, errors.New("test error"))
		})
		doNodeErr, _ := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "errorLoopTest",
		}, action.Registry)
		childrenNodes["nodeErr"] = doNodeErr

		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 3",
			"do":        "nodeErr",
			"mode":      2,
		}, Registry)
		assert.Nil(t, err)

		// Node 7: Loop condition evaluation error at runtime
		node7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.not_exists) < 3",
			"do":        "node1",
			"mode":      2,
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{
			{
				MetaData: types.BuildMetadata(map[string]string{
					"count": "0",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "0",
				AfterSleep: time.Millisecond * 20,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Nil(t, err)
					// Should end when count is 3
					assert.Equal(t, "3", msg.Metadata.GetValue("count"))
					assert.Equal(t, "3", msg.GetData())
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Nil(t, err)
					// Should end when count is 3, but msg data should be the original "0"
					assert.Equal(t, "0", msg.GetData())
					// Original msg metadata is preserved
					assert.Equal(t, "0", msg.Metadata.GetValue("count"))
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Nil(t, err)
					// Should end when count is 3, and data should be merged array
					assert.Equal(t, "3", msg.Metadata.GetValue("count"))
					assert.Equal(t, "[1,2,3]", msg.GetData()) // 1, 2, 3 were the outputs of each iteration
				},
			},
			{
				Node:    node4,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Nil(t, err)
					// Mode 2 default fallback. Should be identical to Node 1
					assert.Equal(t, "3", msg.Metadata.GetValue("count"))
					assert.Equal(t, "3", msg.GetData())
				},
			},
			{
				Node:    node7,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					// Condition template execute error
					assert.NotNil(t, err)
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node6,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					// Inner loop node execution error
					assert.NotNil(t, err)
					assert.Equal(t, "test error", err.Error())
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}

		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, childrenNodes, item.Callback)
		}
	})

	t.Run("BreakLoop", func(t *testing.T) {
		// Register a test function node that sets break when count is 2
		action.Functions.Register("breakLoopTest", func(ctx types.RuleContext, msg types.RuleMsg) {
			countStr := msg.Metadata.GetValue("count")
			count, _ := strconv.Atoi(countStr)
			count++
			msg.Metadata.PutValue("count", strconv.Itoa(count))
			msg.SetData(strconv.Itoa(count))
			if count == 2 {
				msg.Metadata.PutValue(MdKeyBreak, MdValueBreak)
			}
			ctx.TellSuccess(msg)
		})

		doNode, err := test.CreateAndInitNode("functions", types.Configuration{
			"functionName": "breakLoopTest",
		}, action.Registry)
		assert.Nil(t, err)

		childrenNodes := map[string]types.Node{
			"node1": doNode,
		}

		// Loop condition would go up to 5, but we break at 2
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"condition": "int(metadata.count) < 5",
			"do":        "node1",
			"mode":      2,
		}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{
			{
				MetaData: types.BuildMetadata(map[string]string{
					"count": "0",
				}),
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "0",
				AfterSleep: time.Millisecond * 20,
			},
		}

		test.NodeOnMsgWithChildren(t, node, msgList, childrenNodes, func(msg types.RuleMsg, relationType string, err error) {
			assert.Nil(t, err)
			// Should stop at 2
			assert.Equal(t, "2", msg.Metadata.GetValue("count"))
			assert.Equal(t, "2", msg.GetData())
			// Check break flag is removed
			assert.Equal(t, "", msg.Metadata.GetValue(MdKeyBreak))
		})
	})
}
