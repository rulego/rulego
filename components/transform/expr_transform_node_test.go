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

package transform

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestExprTransformNode(t *testing.T) {
	var targetNodeType = "exprTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExprTransformNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
			"expr": "msg",
		}, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
			"expr": "msg",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"id":          "id",
				"ts":          "ts",
				"dataType":    "dataType",
				"type":        "type",
				"name":        "upper(msg.name)",
				"tmp":         "msg.temperature",
				"alarm":       "msg.temperature>50",
				"productType": "metadata.productType",
			},
		}, Registry)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg.name)",
		}, Registry)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])",
		}, Registry)
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "upper(msg[:1])",
			"mapping": map[string]string{
				"name":        "upper(msg.name)",
				"tmp":         "msg.temperature",
				"alarm":       "msg.temperature>50",
				"productType": "metadata.productType",
			},
		}, Registry)

		node5, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "replace(toJSON(msg),'name','productName')",
		}, Registry)

		node6, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.aa+'xx'",
		}, Registry)

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.aa;",
		}, Registry)
		assert.Nil(t, err)
		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name": "msg.aa;",
			},
		}, Registry)
		assert.Nil(t, err)
		node7, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name": "msg.aa+1",
			},
		}, Registry)
		node8, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		msg := test.Msg{
			Id:         "226a05f1-9464-43b6-881e-b1629f1b030d",
			Ts:         1719024872741,
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}
		msg1 := test.Msg{
			MetaData:   metaData,
			DataType:   types.TEXT,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "aa",
			AfterSleep: time.Millisecond * 200,
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, types.JSON, msg.DataType)
					assert.Equal(t, "{\"alarm\":true,\"dataType\":\"JSON\",\"id\":\"226a05f1-9464-43b6-881e-b1629f1b030d\",\"name\":\"AA\",\"productType\":\"test\",\"tmp\":60,\"ts\":1719024872741,\"type\":\"ACTIVITY_EVENT\"}", msg.GetData())
				},
			},
			{
				Node:    node2,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "AA", msg.GetData())
				},
			},
			{
				Node:    node3,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "A", msg.GetData())
				},
			},
			{
				Node:    node4,
				MsgList: []test.Msg{msg1},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "A", msg.GetData())
				},
			},
			{
				Node:    node5,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "{\n  \"humidity\": 30,\n  \"productName\": \"aa\",\n  \"temperature\": 60\n}", msg.GetData())
				},
			},
			{
				Node:    node6,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node7,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node8,
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "{}", msg.GetData())
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
		// 测试单个表达式中的节点依赖
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.msg.temperature} + 10",
		}, Registry)
		assert.Nil(t, err)

		// 测试映射中的节点依赖
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"prevTemp":    "${node1.msg.temperature}",
				"currentTemp": "msg.temperature",
				"tempDiff":    "msg.temperature - ${node1.msg.temperature}",
			},
		}, Registry)
		assert.Nil(t, err)

		// 测试混合节点依赖和元数据
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "${node1.metadata.deviceType} + '_' + msg.sensorId",
		}, Registry)
		assert.Nil(t, err)

		// 测试嵌套节点依赖
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"sensor1": "${node1.msg.sensor.temperature}",
				"sensor2": "${node2.msg.sensor.humidity}",
				"combined": "${node1.msg.sensor.temperature} + ${node2.msg.sensor.humidity}",
			},
		}, Registry)
		assert.Nil(t, err)

		// 验证节点创建成功
		assert.NotNil(t, node1)
		assert.NotNil(t, node2)
		assert.NotNil(t, node3)
		assert.NotNil(t, node4)
	})

	// 测试不带 ${} 的传统表达式仍然正常工作
	// Test traditional expressions without ${} still work normally
	t.Run("TraditionalExpression", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "msg.temperature * 2",
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "TELEMETRY",
				Data:       `{"temperature":25}`,
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "50", msg.Data.String())
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
			"expr": "${node1.msg.temperature +",
		}, Registry)
		assert.NotNil(t, err)

		// 测试映射中的无效表达式
		_, err2 := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"invalid": "${node1.msg.temperature",
			},
		}, Registry)
		assert.NotNil(t, err2)
	})

	// 测试表达式优先级（Expr 优先于 Mapping）
	// Test expression priority (Expr takes precedence over Mapping)
	t.Run("ExpressionPriority", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"expr": "'from_expr'",
			"mapping": map[string]string{
				"result": "'from_mapping'",
			},
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "TELEMETRY",
				Data:       `{"test":"data"}`,
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "from_expr", msg.Data.String())
				},
			},
		}
		test.NodeOnMsgWithChildren(t, nodeList[0].Node, nodeList[0].MsgList, nodeList[0].ChildrenNodes, nodeList[0].Callback)
	})
}

// TestExprTransformNodeDestroy 测试销毁节点
// TestExprTransformNodeDestroy tests destroying the node.
func TestExprTransformNodeDestroy(t *testing.T) {
	var node ExprTransformNode
	var configuration = make(types.Configuration)
	configuration["expr"] = "msg.temperature * 2"
	err := node.Init(types.NewConfig(), configuration)
	assert.Nil(t, err)

	// 调用 Destroy 方法不应该引发错误
	node.Destroy()
}
