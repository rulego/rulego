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
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
)

func TestJsTransformNode(t *testing.T) {
	var targetNodeType = "jsTransform"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &JsTransformNode{}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "metadata['test']='addFromJs';msgType='MSG_TYPE_MODIFY_BY_JS';return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
		assert.Nil(t, err)
		node2, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return true`,
		}, Registry)
		node3, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return a`,
		}, Registry)
		node4, _ := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"vars": map[string]string{
				"ip": "192.168.1.1",
			},
			"jsScript": "metadata['test']='addFromJs';metadata['ip']=vars.ip;msgType='MSG_TYPE_MODIFY_BY_JS';return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
		var nodeList = []types.Node{node1, node2, node3, node4}

		for _, node := range nodeList {
			// 在测试循环开始前捕获配置，避免在回调中并发访问
			jsScript := node.(*JsTransformNode).Config.JsScript

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
					Data:       "{\"name\":\"lala\"}",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if jsScript == `return true` {
					assert.Equal(t, JsTransformReturnFormatErr.Error(), err2.Error())
				} else if jsScript == `return a` {
					assert.NotNil(t, err2)
				} else {
					assert.True(t, msg.Metadata.GetValue("ip") == "" || msg.Metadata.GetValue("ip") == "192.168.1.1")
					assert.Equal(t, "test", msg.Metadata.GetValue("productType"))
					assert.Equal(t, "addFromJs", msg.Metadata.GetValue("test"))
					assert.Equal(t, "MSG_TYPE_MODIFY_BY_JS", msg.Type)
				}

			})
		}
	})
	t.Run("OnMsgError", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "msg['add']=5+msg['test'];return {'msg':msg,'metadata':metadata,'msgType':msgType};",
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		var msgList = []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node1, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Failure, relationType)
		})
	})
}

// TestJsTransformNodeJSONArraySupport 测试JavaScript转换器对JSON数组的支持
func TestJsTransformNodeJSONArraySupport(t *testing.T) {
	config := types.NewConfig()

	// 测试1: JSON数组处理
	t.Run("JSONArrayTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 对JSON数组进行处理：添加索引和处理标志
				if (Array.isArray(msg)) {
					var result = [];
					for (var i = 0; i < msg.length; i++) {
						result.push({
							index: i,
							value: msg[i],
							processed: true
						});
					}
					metadata['arrayLength'] = msg.length.toString();
					metadata['processed'] = 'array_transformed';
					return {'msg': result, 'metadata': metadata, 'msgType': msgType};
				}
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建JSON数组消息
		metadata := types.BuildMetadata(make(map[string]string))
		arrayData := `["apple", "banana", "cherry"]`
		testMsg := types.NewMsg(0, "ARRAY_TEST", types.JSON, metadata, arrayData)

		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		// 验证结果
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "3", resultMsg.Metadata.GetValue("arrayLength"))
		assert.Equal(t, "array_transformed", resultMsg.Metadata.GetValue("processed"))
	})

	// 测试2: JSON对象处理
	t.Run("JSONObjectTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 对JSON对象进行处理
				if (typeof msg === 'object' && !Array.isArray(msg)) {
					msg.processed = true;
					msg.timestamp = new Date().getTime();
					metadata['processed'] = 'object_transformed';
				}
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建JSON对象消息
		metadata := types.BuildMetadata(make(map[string]string))
		objectData := `{"name": "test", "value": 123}`
		testMsg := types.NewMsg(0, "OBJECT_TEST", types.JSON, metadata, objectData)

		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		// 验证结果
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "object_transformed", resultMsg.Metadata.GetValue("processed"))

	})

	// 测试3: 嵌套JSON数组处理
	t.Run("NestedJSONArrayTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 处理嵌套数组：计算每个子数组的和
				if (Array.isArray(msg)) {
					var result = [];
					for (var i = 0; i < msg.length; i++) {
						var item = msg[i];
						if (Array.isArray(item)) {
							// 计算子数组的和
							var sum = 0;
							for (var j = 0; j < item.length; j++) {
								sum += item[j];
							}
							result.push({
								original: item,
								sum: sum,
								count: item.length
							});
						} else {
							result.push(item);
						}
					}
					metadata['nestedArrayProcessed'] = 'true';
					return {'msg': result, 'metadata': metadata, 'msgType': msgType};
				}
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建嵌套JSON数组消息
		metadata := types.BuildMetadata(make(map[string]string))
		nestedArrayData := `[[1, 2, 3], [4, 5, 6], [7, 8, 9]]`
		testMsg := types.NewMsg(0, "NESTED_ARRAY_TEST", types.JSON, metadata, nestedArrayData)

		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		// 验证结果
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "true", resultMsg.Metadata.GetValue("nestedArrayProcessed"))

	})

	// 测试4: 混合数据类型处理
	t.Run("MixedDataTypeTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 根据数据类型进行不同处理
				metadata['originalType'] = dataType;
				
				if (String(dataType) === 'JSON') {
					if (Array.isArray(msg)) {
						metadata['jsonType'] = 'array';
						metadata['length'] = msg.length.toString();
						// 为数组添加处理标记
						var newArray = msg.slice(); // 复制数组
						newArray.push('processed_by_js');
						return {'msg': newArray, 'metadata': metadata, 'msgType': msgType};
					} else if (typeof msg === 'object') {
						metadata['jsonType'] = 'object';
						msg.processedBy = 'js_transform';
						return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
					}
				}
				
				// 其他类型直接返回
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 测试JSON数组
		arrayMetadata := types.BuildMetadata(make(map[string]string))
		arrayData := `["item1", "item2", "item3"]`
		arrayMsg := types.NewMsg(0, "MIXED_TEST", types.JSON, arrayMetadata, arrayData)

		var arrayResult types.RuleMsg
		var arrayErr error

		arrayCtx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			arrayResult = msg
			arrayErr = err
		})

		node.OnMsg(arrayCtx, arrayMsg)

		// 验证数组处理结果
		assert.Nil(t, arrayErr)
		assert.Equal(t, "JSON", arrayResult.Metadata.GetValue("originalType"))
		assert.Equal(t, "array", arrayResult.Metadata.GetValue("jsonType"))
		assert.Equal(t, "3", arrayResult.Metadata.GetValue("length"))

		// 测试JSON对象
		objectMetadata := types.BuildMetadata(make(map[string]string))
		objectData := `{"name": "test", "id": 456}`
		objectMsg := types.NewMsg(0, "MIXED_TEST", types.JSON, objectMetadata, objectData)

		var objectResult types.RuleMsg
		var objectErr error

		objectCtx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			objectResult = msg
			objectErr = err
		})

		node.OnMsg(objectCtx, objectMsg)

		// 验证对象处理结果
		assert.Nil(t, objectErr)
		assert.Equal(t, "JSON", objectResult.Metadata.GetValue("originalType"))
		assert.Equal(t, "object", objectResult.Metadata.GetValue("jsonType"))
	})

	// 测试5: 验证DataType作为字符串处理时的JSON序列化问题
	t.Run("DataTypeStringProcessingIssue", func(t *testing.T) {
		// 直接测试ToStringMaybeErr对DataType的处理
		dataTypeValue := types.TEXT
		result, err := str.ToStringMaybeErr(dataTypeValue)

		// 这里会显示问题：DataType被JSON序列化后带引号
		t.Logf("DataType value: %v, converted string: %s", dataTypeValue, result)

		// 修正期望值：ToStringMaybeErr会对DataType进行JSON序列化，所以结果会是带引号的字符串
		assert.Nil(t, err)
		assert.Equal(t, `"TEXT"`, result) // DataType被JSON序列化后会带引号

		// 这就是为什么我们需要在JS transform中传递string(msg.DataType)而不是msg.DataType
		// 正确的做法是先转换为字符串
		stringResult, err2 := str.ToStringMaybeErr(string(dataTypeValue))
		assert.Nil(t, err2)
		assert.Equal(t, "TEXT", stringResult) // 字符串不会被JSON序列化
	})

	// 测试6: 重现JS脚本返回dataType参数导致的JSON序列化问题
	t.Run("JSReturnDataTypeIssue", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// JS脚本直接返回原始dataType参数，这会暴露问题
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建测试消息
		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello World")

		// 使用回调收集结果
		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		// 处理消息
		node.OnMsg(ctx, testMsg)

		// 输出调试信息
		t.Logf("Original DataType: %v (%T)", testMsg.DataType, testMsg.DataType)
		if resultErr == nil {
			t.Logf("Result DataType: %v (%T)", resultMsg.DataType, resultMsg.DataType)
			t.Logf("Result DataType string: %s", string(resultMsg.DataType))
		}

		// 验证结果
		if resultErr != nil {
			t.Logf("Error occurred: %v", resultErr)
		}
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		// 这里可能会失败，因为DataType可能变成了"\"TEXT\""而不是"TEXT"
		assert.Equal(t, types.TEXT, resultMsg.DataType)
	})
}

// TestJsTransformNodeDataTypeFix 测试DataType修复
func TestJsTransformNodeDataTypeFix(t *testing.T) {
	config := types.NewConfig()

	// 测试: 确保JS脚本接收到字符串形式的dataType参数
	t.Run("DataTypeAsStringParameter", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// JS脚本直接返回dataType参数，应该是字符串而不是DataType类型
			"jsScript": "metadata['receivedDataType'] = dataType; metadata['dataTypeType'] = typeof dataType; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建测试消息
		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello World")

		// 使用回调收集结果
		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		// 处理消息
		node.OnMsg(ctx, testMsg)

		// 验证结果
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// 验证JS接收到的dataType是字符串类型
		assert.Equal(t, "TEXT", resultMsg.Metadata.GetValue("receivedDataType"))
		assert.Equal(t, "string", resultMsg.Metadata.GetValue("dataTypeType"))

		// 验证返回的DataType正确设置
		assert.Equal(t, types.TEXT, resultMsg.DataType)
	})
}

// TestJsTransformNodeDataTypeFixComprehensive 综合测试DataType修复
func TestJsTransformNodeDataTypeFixComprehensive(t *testing.T) {
	config := types.NewConfig()

	// 测试1: 验证JS脚本接收到的dataType参数是字符串
	t.Run("DataTypeParameterIsString", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				metadata['dataType_value'] = dataType;
				metadata['dataType_type'] = typeof dataType;
				metadata['dataType_equality'] = (dataType === 'TEXT').toString();
				return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello")

		var resultMsg types.RuleMsg
		var resultErr error
		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		assert.Equal(t, "TEXT", resultMsg.Metadata.GetValue("dataType_value"))
		assert.Equal(t, "string", resultMsg.Metadata.GetValue("dataType_type"))
		assert.Equal(t, "true", resultMsg.Metadata.GetValue("dataType_equality"))
		assert.Equal(t, types.TEXT, resultMsg.DataType)
	})

	// 测试2: 验证不同DataType值的处理
	t.Run("DifferentDataTypes", func(t *testing.T) {
		testCases := []struct {
			name     string
			dataType types.DataType
			expected string
		}{
			{"JSON", types.JSON, "JSON"},
			{"TEXT", types.TEXT, "TEXT"},
			{"BINARY", types.BINARY, "BINARY"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				node := &JsTransformNode{}
				err := node.Init(config, types.Configuration{
					"jsScript": "metadata['received_dataType'] = dataType; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
				})
				assert.Nil(t, err)
				defer node.Destroy()

				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsg(0, "TEST", tc.dataType, metadata, "test data")

				var resultMsg types.RuleMsg
				var resultErr error
				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultMsg = msg
					resultErr = err
				})

				node.OnMsg(ctx, testMsg)

				assert.Nil(t, resultErr)
				assert.Equal(t, tc.expected, resultMsg.Metadata.GetValue("received_dataType"))
				assert.Equal(t, tc.dataType, resultMsg.DataType)
			})
		}
	})

	// 测试3: 验证dataType返回值的正确处理
	t.Run("DataTypeReturnHandling", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// JS脚本修改dataType并返回
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':'BINARY'};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello")

		var resultMsg types.RuleMsg
		var resultErr error
		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		// 验证DataType被正确修改为BINARY
		assert.Equal(t, types.BINARY, resultMsg.DataType)
	})
}
