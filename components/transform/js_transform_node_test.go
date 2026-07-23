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
			// Capture configurations before the test loop starts to avoid concurrent access during callbacks
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

// TestJsTransformNodeJSONArraySupport tests JavaScript converter's support for JSON arrays
func TestJsTransformNodeJSONArraySupport(t *testing.T) {
	config := types.NewConfig()

	// Test 1: JSON array processing
	t.Run("JSONArrayTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Process the JSON array by adding indexes and processed flags
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

		// Create a JSON array message
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

		// Verify the results
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "3", resultMsg.Metadata.GetValue("arrayLength"))
		assert.Equal(t, "array_transformed", resultMsg.Metadata.GetValue("processed"))
	})

	// Test 2: JSON object processing
	t.Run("JSONObjectTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Process the JSON object
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

		// Create a JSON object message
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

		// Verify the results
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "object_transformed", resultMsg.Metadata.GetValue("processed"))

	})

	// Test 3: Nested JSON array processing
	t.Run("NestedJSONArrayTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Process the nested array by calculating the sum of each child array
				if (Array.isArray(msg)) {
					var result = [];
					for (var i = 0; i < msg.length; i++) {
						var item = msg[i];
						if (Array.isArray(item)) {
							// Calculate the child array's sum
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

		// Create nested JSON array messages
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

		// Verify the results
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, "true", resultMsg.Metadata.GetValue("nestedArrayProcessed"))

	})

	// Test 4: Handling mixed data types
	t.Run("MixedDataTypeTransform", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Process the value according to its data type
				metadata['originalType'] = dataType;
				
				if (String(dataType) === 'JSON') {
					if (Array.isArray(msg)) {
						metadata['jsonType'] = 'array';
						metadata['length'] = msg.length.toString();
						// Add a processed marker to the array
						var newArray = msg.slice(); // Copy the array
						newArray.push('processed_by_js');
						return {'msg': newArray, 'metadata': metadata, 'msgType': msgType};
					} else if (typeof msg === 'object') {
						metadata['jsonType'] = 'object';
						msg.processedBy = 'js_transform';
						return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
					}
				}
				
				// Return other types unchanged
				return {'msg': msg, 'metadata': metadata, 'msgType': msgType};
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Test the JSON array
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

		// Verify array processing results
		assert.Nil(t, arrayErr)
		assert.Equal(t, "JSON", arrayResult.Metadata.GetValue("originalType"))
		assert.Equal(t, "array", arrayResult.Metadata.GetValue("jsonType"))
		assert.Equal(t, "3", arrayResult.Metadata.GetValue("length"))

		// Test the JSON object
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

		// Verify object processing results
		assert.Nil(t, objectErr)
		assert.Equal(t, "JSON", objectResult.Metadata.GetValue("originalType"))
		assert.Equal(t, "object", objectResult.Metadata.GetValue("jsonType"))
	})

	// Test 5: Verify JSON serialization issues when processing DataType as strings
	t.Run("DataTypeStringProcessingIssue", func(t *testing.T) {
		// Directly test ToStringMaybeErr's handling of DataType
		dataTypeValue := types.TEXT
		result, err := str.ToStringMaybeErr(dataTypeValue)

		// Correction of expected value: ToStringMaybeErr will serialize DataType in JSON, so the result will be a string with quotes
		assert.Nil(t, err)
		assert.Equal(t, `"TEXT"`, result) // When DataType is serialized in JSON, it will be in quotes

		stringResult, err2 := str.ToStringMaybeErr(string(dataTypeValue))
		assert.Nil(t, err2)
		assert.Equal(t, "TEXT", stringResult) // Strings are not serialized by JSON
	})

	// Test 6: Reproduce JSON serialization issues caused by JS scripts returning dataType parameters
	t.Run("JSReturnDataTypeIssue", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// JS scripts directly return the original dataType parameter, which exposes the problem
			"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create test messages
		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello World")

		// Collect results using callbacks
		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		// Process the message
		node.OnMsg(ctx, testMsg)
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)
		assert.Equal(t, types.TEXT, resultMsg.DataType)
	})
}

// TestJsTransformNodeDataTypeFix: Test DataType repair
func TestJsTransformNodeDataTypeFix(t *testing.T) {
	config := types.NewConfig()

	// Test: Ensure the JS script receives the dataType parameter in string form
	t.Run("DataTypeAsStringParameter", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// The JS script returns the dataType parameter directly, which should be a string rather than a DataType type
			"jsScript": "metadata['receivedDataType'] = dataType; metadata['dataTypeType'] = typeof dataType; return {'msg':msg,'metadata':metadata,'msgType':msgType,'dataType':dataType};",
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create test messages
		metadata := types.BuildMetadata(make(map[string]string))
		testMsg := types.NewMsg(0, "TEST", types.TEXT, metadata, "Hello World")

		// Collect results using callbacks
		var resultMsg types.RuleMsg
		var resultRelationType string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			resultMsg = msg
			resultRelationType = relationType
			resultErr = err
		})

		// Process the message
		node.OnMsg(ctx, testMsg)

		// Verify the results
		assert.Nil(t, resultErr)
		assert.Equal(t, types.Success, resultRelationType)

		// Verify that the dataType received by JS is of the string type
		assert.Equal(t, "TEXT", resultMsg.Metadata.GetValue("receivedDataType"))
		assert.Equal(t, "string", resultMsg.Metadata.GetValue("dataTypeType"))

		// Verify that the returned DataType is correctly set
		assert.Equal(t, types.TEXT, resultMsg.DataType)
	})
}

// TestJsTransformNodeDataTypeFixComprehensive Comprehensive Test DataType Fix
func TestJsTransformNodeDataTypeFixComprehensive(t *testing.T) {
	config := types.NewConfig()

	// Test 1: Verify that the dataType parameter received by the JS script is a string
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

	// Test 2: Verify the handling of different DataType values
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

	// Test 3: Verify the correct handling of dataType return values
	t.Run("DataTypeReturnHandling", func(t *testing.T) {
		node := &JsTransformNode{}
		err := node.Init(config, types.Configuration{
			// The JS script modifies the dataType and returns it
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
		// Verify that DataType has been correctly changed to BINARY
		assert.Equal(t, types.BINARY, resultMsg.DataType)
	})
}
