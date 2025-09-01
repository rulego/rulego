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

package filter

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestExprFilterNode(t *testing.T) {
	var targetNodeType = "exprFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExprFilterNode{}, types.Configuration{
			"expr": "",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50",
		}, types.Configuration{
			"expr": "msg.temperature > 50",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"expr": "1",
		}, types.Configuration{
			"expr": "1",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50 && msg.humidity > 20",
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": `upper(msg.name)=='AA'`,
		}, Registry)

		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "temperature==nil?true:false",
		}, Registry)

		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg)=='AA'",
		}, Registry)
		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])=='A'",
		}, Registry)
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "",
		}, Registry)
		assert.Equal(t, err.Error(), "expr can not be empty", err.Error())
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msg1 := test.Msg{
			MetaData:   metaData,
			DataType:   types.TEXT,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "AA",
			AfterSleep: time.Millisecond * 200,
		}
		msg2 := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg3 := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"bb\",\"temperature\":40}",
			AfterSleep: time.Millisecond * 200,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node1,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},

			{
				Node:    node3,
				MsgList: []test.Msg{msg2},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node3,
				MsgList: []test.Msg{msg3},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.False, relationType)
				},
			},
			{
				Node:    node4,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node5,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
			{
				Node:    node6,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Millisecond * 20)
	})

	// 测试节点依赖表达式语法 ${node1.msg.xx}
	// Test node dependency expression syntax ${node1.msg.xx}
	t.Run("NodeDependencyExpression", func(t *testing.T) {
		// 测试初始化包含节点依赖的表达式
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.temperature} > 50",
		}, Registry)
		assert.Nil(t, err)

		// 测试混合表达式：既有节点依赖又有当前消息字段
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.temperature} > 50 && msg.humidity > 20",
		}, Registry)
		assert.Nil(t, err)

		// 测试嵌套节点依赖
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.sensor.temperature} > ${node2.msg.threshold}",
		}, Registry)
		assert.Nil(t, err)

		// 测试元数据中的节点依赖
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.metadata.deviceType} == 'sensor'",
		}, Registry)
		assert.Nil(t, err)

		// 验证节点创建成功
		assert.NotNil(t, node)
		assert.NotNil(t, node2)
		assert.NotNil(t, node3)
		assert.NotNil(t, node4)
	})

	// 测试不带 ${} 的表达式仍然正常工作
	// Test expressions without ${} still work normally
	t.Run("TraditionalExpression", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature > 50",
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "TELEMETRY",
				Data:       `{"temperature":60}`,
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.True, relationType)
				},
			},
		}
		test.NodeOnMsgWithChildren(t, nodeList[0].Node, nodeList[0].MsgList, nodeList[0].ChildrenNodes, nodeList[0].Callback)
	})

	// 测试无效的节点依赖表达式
	// Test invalid node dependency expressions
	t.Run("InvalidNodeDependencyExpression", func(t *testing.T) {
		// 测试语法错误的表达式
		_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.temperature >",
		}, Registry)
		assert.NotNil(t, err)

		// 测试不完整的节点依赖语法
		_, err2 := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.temperature",
		}, Registry)
		assert.NotNil(t, err2)
	})
}
