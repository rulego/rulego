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

func TestMetadataTransformNode(t *testing.T) {
	var targetNodeType = "metadataTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &MetadataTransformNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
		}, types.Configuration{
			"mapping": map[string]string{
				"name":  "msg.name",
				"tmp":   "msg.temperature",
				"alarm": "msg.temperature>50",
			},
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"temperature": "msg.temperature",
			},
		}, types.Configuration{
			"mapping": map[string]string{
				"temperature": "msg.temperature",
			},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
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
		var nodeList = []test.NodeAndCallback{
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"mapping": map[string]string{
						"id":           "id",
						"ts":           "ts",
						"dataType":     "dataType",
						"type":         "type",
						"name":         "upper(msg.name)",
						"tmp":          "msg.temperature",
						"alarm":        "msg.temperature>50",
						"test01":       "'test01'",
						"temperature2": "msg.temperature2",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "226a05f1-9464-43b6-881e-b1629f1b030d", msg.Metadata.GetValue("id"))
					assert.Equal(t, "AA", msg.Metadata.GetValue("name"))
					assert.Equal(t, "test01", msg.Metadata.GetValue("test01"))
					assert.Equal(t, "", msg.Metadata.GetValue("temperature2"))
					assert.Equal(t, "test", msg.Metadata.GetValue("productType"))
				},
			},
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"isNew": true,
					"mapping": map[string]string{
						"id":       "id",
						"ts":       "ts",
						"dataType": "dataType",
						"type":     "type",
						"name":     "upper(msg.name)",
						"tmp":      "msg.temperature",
						"alarm":    "msg.temperature>50",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "226a05f1-9464-43b6-881e-b1629f1b030d", msg.Metadata.GetValue("id"))
					assert.Equal(t, "AA", msg.Metadata.GetValue("name"))
					assert.Equal(t, "", msg.Metadata.GetValue("productType"))
				},
			},
			{
				Node: test.InitNode(targetNodeType, types.Configuration{
					"isNew": true,
					"mapping": map[string]string{
						"id": "xx+1",
					},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
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
		// 测试基本节点依赖
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"prevTemp":    "${node1.msg.temperature}",
				"currentTemp": "msg.temperature",
				"tempDiff":    "msg.temperature - ${node1.msg.temperature}",
			},
		}, Registry)
		assert.Nil(t, err)

		// 测试混合节点依赖和元数据
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"deviceInfo": "${node1.metadata.deviceType} + '_' + msg.sensorId",
				"location":   "${node1.msg.location}",
			},
		}, Registry)
		assert.Nil(t, err)

		// 测试嵌套节点依赖
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"sensor1Data": "${node1.msg.sensor.temperature}",
				"sensor2Data": "${node2.msg.sensor.humidity}",
				"combined":    "${node1.msg.sensor.temperature} + ${node2.msg.sensor.humidity}",
			},
		}, Registry)
		assert.Nil(t, err)

		// 验证节点创建成功
		assert.NotNil(t, node1)
		assert.NotNil(t, node2)
		assert.NotNil(t, node3)
	})

	// 测试不带 ${} 的传统表达式仍然正常工作
	// Test traditional expressions without ${} still work normally
	t.Run("TraditionalExpression", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"doubleTemp": "msg.temperature * 2",
				"deviceName": "upper(msg.deviceType)",
				"timestamp":  "ts",
			},
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				Ts:         1719024872741,
				MetaData:   metaData,
				MsgType:    "TELEMETRY",
				Data:       `{"temperature":25,"deviceType":"sensor"}`,
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "50", msg.Metadata.GetValue("doubleTemp"))
					assert.Equal(t, "SENSOR", msg.Metadata.GetValue("deviceName"))
					assert.Equal(t, "1719024872741", msg.Metadata.GetValue("timestamp"))
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
			"mapping": map[string]string{
				"invalid": "${node1.msg.temperature +",
			},
		}, Registry)
		assert.NotNil(t, err)

		// 测试不完整的节点依赖表达式
		_, err2 := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"incomplete": "${node1.msg.temperature",
			},
		}, Registry)
		assert.NotNil(t, err2)
	})

	// 测试表达式计算
	// Test expression evaluation
	t.Run("ExpressionEvaluation", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"mapping": map[string]string{
				"deviceName": "upper(msg.deviceType)",
				"isHot":      "msg.temperature > 30",
				"tempLevel":  "msg.temperature >= 50 ? 'high' : 'normal'",
			},
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "TELEMETRY",
				Data:       `{"deviceType":"thermometer","temperature":60}`,
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "THERMOMETER", msg.Metadata.GetValue("deviceName"))
					assert.Equal(t, "true", msg.Metadata.GetValue("isHot"))
					assert.Equal(t, "high", msg.Metadata.GetValue("tempLevel"))
				},
			},
		}
		test.NodeOnMsgWithChildren(t, nodeList[0].Node, nodeList[0].MsgList, nodeList[0].ChildrenNodes, nodeList[0].Callback)
	})
}

// TestMetadataTransformNodeDestroy 测试销毁节点
// TestMetadataTransformNodeDestroy tests destroying the node.
func TestMetadataTransformNodeDestroy(t *testing.T) {
	var node MetadataTransformNode
	var configuration = make(types.Configuration)
	configuration["mapping"] = map[string]string{
		"deviceType": "msg.deviceType",
		"timestamp":  "ts",
	}
	err := node.Init(types.NewConfig(), configuration)
	assert.Nil(t, err)

	// 调用 Destroy 方法不应该引发错误
	node.Destroy()
}
