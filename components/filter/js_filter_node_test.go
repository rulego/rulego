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
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestJsFilterNode(t *testing.T) {
	var targetNodeType = "jsFilter"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &JsFilterNode{}, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "return msg.temperature > 50;",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return 1`,
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return a`,
		}, Registry)

		var nodeList = []types.Node{node1, node2, node3}

		for _, node := range nodeList {
			// 在测试循环开始前捕获配置，避免在回调中并发访问
			jsScript := node.(*JsFilterNode).Config.JsScript

			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "AA",
					AfterSleep: time.Millisecond * 200,
				},
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature\":60}",
					AfterSleep: time.Millisecond * 200,
				},
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "{\"temperature\":40}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if jsScript == `return 1` {
					assert.Equal(t, "False", relationType)
				} else if jsScript == `return a` {
					assert.NotNil(t, err2)
				} else if msg.GetData() == "{\"temperature\":60}" {

					assert.Equal(t, "True", relationType)
				} else {
					assert.Equal(t, "False", relationType)
				}

			})
		}
	})
}

// TestJsFilterNodeDataType 测试dataType参数传递
func TestJsFilterNodeDataType(t *testing.T) {
	config := types.NewConfig()

	t.Run("DataTypeParameter", func(t *testing.T) {
		// 创建不同数据类型的测试消息
		testCases := []struct {
			dataType   types.DataType
			script     string
			expectTrue bool
		}{
			{types.JSON, "return String(dataType) === 'JSON';", true},
			{types.TEXT, "return String(dataType) === 'TEXT';", true},
			{types.BINARY, "return String(dataType) === 'BINARY';", true},
			{types.JSON, "return String(dataType) === 'TEXT';", false}, // 错误匹配
		}

		for _, tc := range testCases {
			t.Run(string(tc.dataType), func(t *testing.T) {
				node := &JsFilterNode{}
				err := node.Init(config, types.Configuration{
					"jsScript": tc.script,
				})
				assert.Nil(t, err)
				defer node.Destroy()

				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsg(0, "TEST", tc.dataType, metadata, "test data")

				// 使用回调收集结果
				var resultRelationType string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultRelationType = relationType
					resultErr = err
				})

				// 处理消息
				node.OnMsg(ctx, testMsg)

				// 验证结果
				assert.Nil(t, resultErr)
				if tc.expectTrue {
					assert.Equal(t, types.True, resultRelationType)
				} else {
					assert.Equal(t, types.False, resultRelationType)
				}
			})
		}
	})
}

// TestJsFilterNodeDataTypeDebug 调试dataType参数
func TestJsFilterNodeDataTypeDebug(t *testing.T) {
	config := types.NewConfig()

	node := &JsFilterNode{}
	err := node.Init(config, types.Configuration{
		"jsScript": "return String(dataType) === 'JSON';", // 转换为字符串后检查
	})
	assert.Nil(t, err)
	defer node.Destroy()

	metadata := types.BuildMetadata(make(map[string]string))
	testMsg := types.NewMsg(0, "TEST", types.JSON, metadata, "test data")

	var resultRelationType string
	var resultErr error

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		resultRelationType = relationType
		resultErr = err
	})

	node.OnMsg(ctx, testMsg)

	assert.Nil(t, resultErr)
	t.Logf("Result relation type: %s", resultRelationType)
}

// TestJsFilterNodeBinaryData 测试jsFilter组件的二进制数据处理
func TestJsFilterNodeBinaryData(t *testing.T) {
	config := types.NewConfig()

	t.Run("BinaryDataBasic", func(t *testing.T) {
		// 基础二进制数据过滤
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 检查是否为二进制数据且长度大于4字节
				if (String(dataType) === 'BINARY' && msg.length > 4) {
					// 检查前两个字节是否为特定值
					return msg[0] === 0xAA && msg[1] === 0xBB;
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		testCases := []struct {
			name       string
			data       []byte
			expectTrue bool
		}{
			{
				name:       "Valid header with sufficient length",
				data:       []byte{0xAA, 0xBB, 0x01, 0x02, 0x03},
				expectTrue: true,
			},
			{
				name:       "Invalid header",
				data:       []byte{0xFF, 0xEE, 0x01, 0x02, 0x03},
				expectTrue: false,
			},
			{
				name:       "Too short data",
				data:       []byte{0xAA, 0xBB, 0x01},
				expectTrue: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsgFromBytes(0, "DEVICE_DATA", types.BINARY, metadata, tc.data)

				var resultRelationType string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultRelationType = relationType
					resultErr = err
				})

				node.OnMsg(ctx, testMsg)

				assert.Nil(t, resultErr)
				if tc.expectTrue {
					assert.Equal(t, types.True, resultRelationType)
				} else {
					assert.Equal(t, types.False, resultRelationType)
				}
			})
		}
	})

	t.Run("DeviceFunctionCodeFilter", func(t *testing.T) {
		// 设备功能码过滤：模拟设备数据格式 [设备ID(2字节)] + [功能码(2字节)] + [数据]
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 设备数据格式：[设备ID(2字节)] + [功能码(2字节)] + [数据长度(2字节)] + [数据内容]
				if (String(dataType) === 'BINARY' && msg.length >= 6) {
					// 提取功能码 (第3-4字节，索引2-3)
					var functionCode = (msg[2] << 8) | msg[3]; // 大端序
					
					// 过滤特定功能码：0x0001(读取传感器) 或 0x0002(读取状态)
					return functionCode === 0x0001 || functionCode === 0x0002;
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		testCases := []struct {
			name         string
			deviceID     uint16 // 设备ID
			functionCode uint16 // 功能码
			data         []byte // 附加数据
			expectTrue   bool   // 期望结果
		}{
			{
				name:         "Read sensor data (0x0001)",
				deviceID:     0x1234,
				functionCode: 0x0001,
				data:         []byte{0x00, 0x04, 0x25, 0x30, 0x00, 0x64}, // 长度4 + 温度湿度数据
				expectTrue:   true,
			},
			{
				name:         "Read status (0x0002)",
				deviceID:     0x5678,
				functionCode: 0x0002,
				data:         []byte{0x00, 0x02, 0x01, 0x00}, // 长度2 + 状态数据
				expectTrue:   true,
			},
			{
				name:         "Write command (0x0010) - should be filtered out",
				deviceID:     0x9ABC,
				functionCode: 0x0010,
				data:         []byte{0x00, 0x02, 0xFF, 0x00},
				expectTrue:   false,
			},
			{
				name:         "Unknown function code (0xFFFF)",
				deviceID:     0xDEAD,
				functionCode: 0xFFFF,
				data:         []byte{0x00, 0x01, 0x55},
				expectTrue:   false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// 构造设备数据包
				deviceData := make([]byte, 0, 6+len(tc.data))

				// 添加设备ID (大端序)
				deviceData = append(deviceData, byte(tc.deviceID>>8), byte(tc.deviceID&0xFF))

				// 添加功能码 (大端序)
				deviceData = append(deviceData, byte(tc.functionCode>>8), byte(tc.functionCode&0xFF))

				// 添加附加数据
				deviceData = append(deviceData, tc.data...)

				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsg(0, "DEVICE_PACKET", types.BINARY, metadata, string(deviceData))

				var resultRelationType string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultRelationType = relationType
					resultErr = err
				})

				node.OnMsg(ctx, testMsg)

				assert.Nil(t, resultErr)
				if tc.expectTrue {
					assert.Equal(t, types.True, resultRelationType)
				} else {
					assert.Equal(t, types.False, resultRelationType)
				}
			})
		}
	})
}
