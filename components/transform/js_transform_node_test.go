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

// TestJsTransformNodeDataTypeSimple 简单测试 - 避免并发问题
func TestJsTransformNodeDataTypeSimple(t *testing.T) {
	// 创建规则引擎配置
	config := types.NewConfig()

	// 测试1: dataType参数传递
	t.Run("DataTypeParameter", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": "metadata['receivedDataType'] = dataType; return {'msg':msg,'metadata':metadata,'msgType':msgType};",
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
		assert.Equal(t, "TEXT", resultMsg.Metadata.GetValue("receivedDataType"))
	})

	// 测试2: dataType修改
	t.Run("DataTypeModification", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':'BINARY'};",
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
		assert.Equal(t, types.BINARY, resultMsg.DataType)
	})

	// 测试3: 字节数组处理 - 完整的二进制数据处理流程
	t.Run("ByteArrayProcessing", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// 完整的二进制数据处理：传入BINARY数据，在JS中修改，然后返回
				if (String(dataType) === 'BINARY') {
					// msg在BINARY模式下是Uint8Array，包含原始数据
					// 创建新的字节数组，在原数据前添加4字节头部
					var header = [0xAA, 0xBB, 0xCC, 0xDD]; // 4字节头部
					var newBytes = new Array(header.length + msg.length);
					
					// 复制头部
					for (var i = 0; i < header.length; i++) {
						newBytes[i] = header[i];
					}
					
					// 复制原始数据
					for (var i = 0; i < msg.length; i++) {
						newBytes[header.length + i] = msg[i];
					}
					
					metadata['processed'] = 'binary_modified';
					metadata['originalLength'] = msg.length.toString();
					metadata['newLength'] = newBytes.length.toString();
					metadata['headerAdded'] = 'true';
					
					return {'msg': newBytes, 'metadata': metadata, 'msgType': msgType, 'dataType': 'BINARY'};
				}
				
				// 非BINARY数据直接返回
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建包含二进制数据的测试消息
		metadata := types.BuildMetadata(make(map[string]string))
		originalData := []byte{0x01, 0x02, 0x03, 0x04, 0x05} // 原始二进制数据
		testMsg := types.NewMsgFromBytes(0, "BINARY_TEST", types.BINARY, metadata, originalData)

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
		assert.Equal(t, types.BINARY, resultMsg.DataType)

		// 验证元数据
		assert.Equal(t, "binary_modified", resultMsg.Metadata.GetValue("processed"))
		assert.Equal(t, "5", resultMsg.Metadata.GetValue("originalLength"))
		assert.Equal(t, "9", resultMsg.Metadata.GetValue("newLength")) // 5原始 + 4头部 = 9
		assert.Equal(t, "true", resultMsg.Metadata.GetValue("headerAdded"))

		// 验证输出数据：应该包含4字节头部 + 原始5字节数据
		outputData := []byte(resultMsg.GetData())
		assert.Equal(t, 9, len(outputData))

		// 检查头部字节
		assert.Equal(t, byte(0xAA), outputData[0])
		assert.Equal(t, byte(0xBB), outputData[1])
		assert.Equal(t, byte(0xCC), outputData[2])
		assert.Equal(t, byte(0xDD), outputData[3])

		// 检查原始数据部分
		assert.Equal(t, byte(0x01), outputData[4])
		assert.Equal(t, byte(0x02), outputData[5])
		assert.Equal(t, byte(0x03), outputData[6])
		assert.Equal(t, byte(0x04), outputData[7])
		assert.Equal(t, byte(0x05), outputData[8])
	})

	// 测试4: 简单字节数组创建（保留原来的测试逻辑）
	t.Run("CreateByteArray", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": "var bytes = [72, 101, 108, 108, 111]; return {'msg': bytes, 'metadata': metadata, 'msgType': msgType, 'dataType': 'BINARY'};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// 创建测试消息
		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "original data")

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
		assert.Equal(t, types.BINARY, resultMsg.DataType)
		assert.Equal(t, "Hello", resultMsg.GetData())
	})
}

// TestJsTransformNodeDebug 调试测试
func TestJsTransformNodeDebug(t *testing.T) {
	config := types.NewConfig()

	node := &JsTransformNode{}
	err := node.Init(config, types.Configuration{
		"jsScript": `
			var bytes = [72, 101, 108, 108, 111];
			console.log("bytes type:", typeof bytes);
			console.log("bytes:", bytes);
			console.log("bytes constructor:", bytes.constructor.name);
			return {'msg': bytes, 'metadata': metadata, 'msgType': msgType, 'dataType': 'BINARY'};
		`,
	})
	assert.Nil(t, err)
	defer node.Destroy()

	metadata := types.BuildMetadata(make(map[string]string))
	testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "original")

	var resultMsg types.RuleMsg
	var resultErr error

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		resultMsg = msg
		resultErr = err
	})

	node.OnMsg(ctx, testMsg)

	// 打印实际结果用于调试
	t.Logf("Result error: %v", resultErr)
	t.Logf("Result data: %s", resultMsg.GetData())
	t.Logf("Result dataType: %s", resultMsg.DataType)
}

// TestJsTransformNodeDebugOutput 调试JavaScript输出格式
func TestJsTransformNodeDebugOutput(t *testing.T) {
	config := types.NewConfig()

	t.Run("DebugJavaScriptOutput", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				var bytes = [72, 101, 108, 108, 111]; 
				metadata['arrayType'] = typeof bytes;
				metadata['arrayLength'] = bytes.length.toString();
				metadata['firstElement'] = bytes[0].toString();
				metadata['arrayString'] = JSON.stringify(bytes);
				return {'msg': bytes, 'metadata': metadata, 'msgType': msgType, 'dataType': 'BINARY'};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "original")

		var resultMsg types.RuleMsg
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultErr = err
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		t.Logf("输出数据: %s", resultMsg.GetData())
		t.Logf("数组类型: %s", resultMsg.Metadata.GetValue("arrayType"))
		t.Logf("数组长度: %s", resultMsg.Metadata.GetValue("arrayLength"))
		t.Logf("第一个元素: %s", resultMsg.Metadata.GetValue("firstElement"))
		t.Logf("数组字符串: %s", resultMsg.Metadata.GetValue("arrayString"))
		t.Logf("DataType: %s", resultMsg.DataType)
	})
}
