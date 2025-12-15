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

package action

import (
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestFunctionsNode(t *testing.T) {

	//not init
	_, ok := Functions.Get("add2")
	assert.False(t, ok)

	Functions.Register("add", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("addFrom", "addFunction")
		ctx.TellNext(msg, "fromAdd")
	})

	Functions.Register("add2", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("addFrom", "addFunction2")
		ctx.TellNext(msg, "fromAdd2")
	})

	_, ok = Functions.Get("add2")
	assert.True(t, ok)
	Functions.UnRegister("add2")

	_, ok = Functions.Get("add2")
	assert.False(t, ok)

	var targetNodeType = "functions"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &FunctionsNode{}, types.Configuration{
			"functionName": "test",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"functionName": "add",
		}, types.Configuration{
			"functionName": "add",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"functionName": "test",
		}, types.Configuration{
			"functionName": "test",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "add",
		}, Registry)
		assert.Nil(t, err)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "${functionName}",
		}, Registry)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "aa",
		}, Registry)

		var nodeList = []types.Node{node1, node2, node3}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")
			metaData.PutValue("functionName", "add")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "AA",
					AfterSleep: time.Millisecond * 200,
				},
			}
			functionName := node.(*FunctionsNode).Config.FunctionName
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if functionName == "aa" {
					assert.Equal(t, "can not found the function=aa", err2.Error())
				} else {
					assert.Equal(t, "addFunction", msg.Metadata.GetValue("addFrom"))
					assert.Equal(t, "fromAdd", relationType)
				}

			})
		}
	})

	t.Run("RegisterDef", func(t *testing.T) {
		Functions.RegisterDef(FunctionDef{
			Name:  "testDef",
			Label: "测试函数",
			Desc:  "这是一个测试函数",
			F: func(ctx types.RuleContext, msg types.RuleMsg) {
				ctx.TellSuccess(msg)
			},
		})

		f, ok := Functions.Get("testDef")
		assert.True(t, ok)
		assert.NotNil(t, f)

		defs := Functions.List()
		var found bool
		for _, def := range defs {
			if def.Name == "testDef" {
				assert.Equal(t, "测试函数", def.Label)
				assert.Equal(t, "这是一个测试函数", def.Desc)
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("RegisterWithParams", func(t *testing.T) {
		Functions.Register("testParams", func(ctx types.RuleContext, msg types.RuleMsg) {
			ctx.TellSuccess(msg)
		}, "测试函数Params", "这是一个测试函数Params")

		f, ok := Functions.Get("testParams")
		assert.True(t, ok)
		assert.NotNil(t, f)

		defs := Functions.List()
		var found bool
		for _, def := range defs {
			if def.Name == "testParams" {
				assert.Equal(t, "测试函数Params", def.Label)
				assert.Equal(t, "这是一个测试函数Params", def.Desc)
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("RegisterOrder", func(t *testing.T) {
		// Clean up previous registrations for this test
		Functions.UnRegister("order1")
		Functions.UnRegister("order2")
		Functions.UnRegister("order3")

		Functions.Register("order1", nil)
		Functions.Register("order2", nil)
		Functions.Register("order3", nil)

		names := Functions.Names()
		var foundOrder1, foundOrder2, foundOrder3 bool
		var index1, index2, index3 int
		for i, name := range names {
			if name == "order1" {
				foundOrder1 = true
				index1 = i
			}
			if name == "order2" {
				foundOrder2 = true
				index2 = i
			}
			if name == "order3" {
				foundOrder3 = true
				index3 = i
			}
		}
		assert.True(t, foundOrder1)
		assert.True(t, foundOrder2)
		assert.True(t, foundOrder3)
		assert.True(t, index1 < index2)
		assert.True(t, index2 < index3)

		// Test UnRegister order
		Functions.UnRegister("order2")
		names = Functions.Names()
		foundOrder2 = false
		for _, name := range names {
			if name == "order2" {
				foundOrder2 = true
			}
		}
		assert.False(t, foundOrder2)

		// Re-register order2, it should be at the end
		Functions.Register("order2", nil)
		names = Functions.Names()
		for i, name := range names {
			if name == "order1" {
				index1 = i
			}
			if name == "order2" {
				index2 = i
			}
			if name == "order3" {
				index3 = i
			}
		}
		assert.True(t, index1 < index3)
		assert.True(t, index3 < index2)
	})

	t.Run("Param", func(t *testing.T) {
		// Register a function that returns the received data
		Functions.Register("echo", func(ctx types.RuleContext, msg types.RuleMsg) {
			ctx.TellSuccess(msg)
		})

		// Test with static param
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "echo",
			"param":        "static_param",
		}, Registry)
		assert.Nil(t, err)

		// Test with dynamic param from metadata
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "echo",
			"param":        "${metadata.key}",
		}, Registry)
		assert.Nil(t, err)

		// Test with default (empty param)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "echo",
		}, Registry)
		assert.Nil(t, err)

		// Test with json param
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "echo",
			"param":        "{\"name\":\"${metadata.key}\"}",
		}, Registry)
		assert.Nil(t, err)

		var nodeList = []types.Node{node1, node2, node3, node4}

		for i, node := range nodeList {
			// fix data race
			i := i
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("key", "dynamic_value")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "original_data",
					AfterSleep: time.Millisecond * 200,
				},
			}

			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if i == 0 {
					assert.Equal(t, "static_param", msg.GetData())
				} else if i == 1 {
					assert.Equal(t, "dynamic_value", msg.GetData())
				} else if i == 2 {
					assert.Equal(t, "original_data", msg.GetData())
				} else if i == 3 {
					assert.Equal(t, "{\"name\":\"dynamic_value\"}", msg.GetData())
				}
				assert.Equal(t, types.Success, relationType)
			})
		}
	})
}
