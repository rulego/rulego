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
			// Capture configurations before the test loop starts to avoid concurrent access during callbacks
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

// TestJsFilterNodeDataType tests the passing of dataType parameters
func TestJsFilterNodeDataType(t *testing.T) {
	config := types.NewConfig()

	t.Run("DataTypeParameter", func(t *testing.T) {
		// Create test messages for different data types
		testCases := []struct {
			dataType   types.DataType
			script     string
			expectTrue bool
		}{
			{types.JSON, "return String(dataType) === 'JSON';", true},
			{types.TEXT, "return String(dataType) === 'TEXT';", true},
			{types.BINARY, "return String(dataType) === 'BINARY';", true},
			{types.JSON, "return String(dataType) === 'TEXT';", false}, // Mismatch
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

				// Collect results using callbacks
				var resultRelationType string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultRelationType = relationType
					resultErr = err
				})

				// Process the message
				node.OnMsg(ctx, testMsg)

				// Verify the results
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

// TestJsFilterNodeDataTypeDebug debug the dataType parameter
func TestJsFilterNodeDataTypeDebug(t *testing.T) {
	config := types.NewConfig()

	node := &JsFilterNode{}
	err := node.Init(config, types.Configuration{
		"jsScript": "return String(dataType) === 'JSON';", // Convert to string and then check
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

// TestJsFilterNodeBinaryData tests the binary data processing of the jsFilter component
func TestJsFilterNodeBinaryData(t *testing.T) {
	config := types.NewConfig()

	t.Run("BinaryDataBasic", func(t *testing.T) {
		// Basic binary data filtering
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Check whether this is binary data longer than 4 bytes
				if (String(dataType) === 'BINARY' && msg.length > 4) {
					// Check whether the first two bytes have the expected values
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
		// Device function code filtering: simulates device data format [Device ID (2 bytes)] + [Function Code (2 bytes)] + [Data]
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Device data format: [device ID (2 bytes)] + [function code (2 bytes)] + [data length (2 bytes)] + [data]
				if (String(dataType) === 'BINARY' && msg.length >= 6) {
					// Extract the function code (bytes 3-4, indexes 2-3)
					var functionCode = (msg[2] << 8) | msg[3]; // Big-endian
					
					// Accept specific function codes: 0x0001 (read sensor) or 0x0002 (read status)
					return functionCode === 0x0001 || functionCode === 0x0002;
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		testCases := []struct {
			name         string
			deviceID     uint16 // Device ID
			functionCode uint16 // Function code
			data         []byte // Additional data
			expectTrue   bool   // Expected results
		}{
			{
				name:         "Read sensor data (0x0001)",
				deviceID:     0x1234,
				functionCode: 0x0001,
				data:         []byte{0x00, 0x04, 0x25, 0x30, 0x00, 0x64}, // Length 4 + temperature and humidity data
				expectTrue:   true,
			},
			{
				name:         "Read status (0x0002)",
				deviceID:     0x5678,
				functionCode: 0x0002,
				data:         []byte{0x00, 0x02, 0x01, 0x00}, // Length 2 + status data
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
				// Construct device data packets
				deviceData := make([]byte, 0, 6+len(tc.data))

				// Add device ID (large-end sequence)
				deviceData = append(deviceData, byte(tc.deviceID>>8), byte(tc.deviceID&0xFF))

				// Add function code (large endpoint)
				deviceData = append(deviceData, byte(tc.functionCode>>8), byte(tc.functionCode&0xFF))

				// Add additional data
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

// TestJsFilterNodeJSONArraySupport Tests JavaScript filters' support for JSON arrays
func TestJsFilterNodeJSONArraySupport(t *testing.T) {
	config := types.NewConfig()

	// Test 1: JSON array length filtering
	t.Run("ArrayLengthFilter", func(t *testing.T) {
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Accept messages whose array length is greater than 2
				if (Array.isArray(msg)) {
					return msg.length > 2;
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create an array of length 3 (should be filtered)
		metadata1 := types.BuildMetadata(make(map[string]string))
		arrayData1 := `["apple", "banana", "cherry"]`
		testMsg1 := types.NewMsg(0, "ARRAY_TEST", types.JSON, metadata1, arrayData1)

		var result1 string
		var err1 error

		ctx1 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result1 = relationType
			err1 = err
		})

		node.OnMsg(ctx1, testMsg1)

		// Verification result: length 3 should pass
		assert.Nil(t, err1)
		assert.Equal(t, types.True, result1)

		// Create an array of length 1 (which should be filtered)
		metadata2 := types.BuildMetadata(make(map[string]string))
		arrayData2 := `["single"]`
		testMsg2 := types.NewMsg(0, "ARRAY_TEST", types.JSON, metadata2, arrayData2)

		var result2 string
		var err2 error

		ctx2 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result2 = relationType
			err2 = err
		})

		node.OnMsg(ctx2, testMsg2)

		// Verification result: length 1 should be filtered
		assert.Nil(t, err2)
		assert.Equal(t, types.False, result2)
	})

	// Test 2: JSON array content filtering
	t.Run("ArrayContentFilter", func(t *testing.T) {
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Check whether the array contains a specific value
				if (Array.isArray(msg)) {
					for (var i = 0; i < msg.length; i++) {
						if (msg[i] === "target") {
							return true;
						}
					}
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create an array containing "target"
		metadata1 := types.BuildMetadata(make(map[string]string))
		arrayData1 := `["apple", "target", "cherry"]`
		testMsg1 := types.NewMsg(0, "CONTENT_TEST", types.JSON, metadata1, arrayData1)

		var result1 string
		var err1 error

		ctx1 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result1 = relationType
			err1 = err
		})

		node.OnMsg(ctx1, testMsg1)

		// Verification result: including target should pass
		assert.Nil(t, err1)
		assert.Equal(t, types.True, result1)

		// Create an array that does not contain "target"
		metadata2 := types.BuildMetadata(make(map[string]string))
		arrayData2 := `["apple", "banana", "cherry"]`
		testMsg2 := types.NewMsg(0, "CONTENT_TEST", types.JSON, metadata2, arrayData2)

		var result2 string
		var err2 error

		ctx2 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result2 = relationType
			err2 = err
		})

		node.OnMsg(ctx2, testMsg2)

		// Validation result: Targets not included should be filtered
		assert.Nil(t, err2)
		assert.Equal(t, types.False, result2)
	})

	// Test 3: Numeric filtering of digital arrays
	t.Run("NumericArrayFilter", func(t *testing.T) {
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Check whether the array contains a value greater than 50
				if (Array.isArray(msg)) {
					for (var i = 0; i < msg.length; i++) {
						if (typeof msg[i] === 'number' && msg[i] > 50) {
							return true;
						}
					}
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create an array containing values greater than 50
		metadata1 := types.BuildMetadata(make(map[string]string))
		arrayData1 := `[10, 30, 75, 20]`
		testMsg1 := types.NewMsg(0, "NUMERIC_TEST", types.JSON, metadata1, arrayData1)

		var result1 string
		var err1 error

		ctx1 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result1 = relationType
			err1 = err
		})

		node.OnMsg(ctx1, testMsg1)

		// Verification result: 75 should be accepted
		assert.Nil(t, err1)
		assert.Equal(t, types.True, result1)

		// Create arrays with no values greater than 50
		metadata2 := types.BuildMetadata(make(map[string]string))
		arrayData2 := `[10, 30, 45, 20]`
		testMsg2 := types.NewMsg(0, "NUMERIC_TEST", types.JSON, metadata2, arrayData2)

		var result2 string
		var err2 error

		ctx2 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result2 = relationType
			err2 = err
		})

		node.OnMsg(ctx2, testMsg2)

		// Verification result: None greater than 50 should be filtered
		assert.Nil(t, err2)
		assert.Equal(t, types.False, result2)
	})

	// Test 4: Mixed Type Filtering (Array vs. Object)
	t.Run("MixedTypeFilter", func(t *testing.T) {
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Filter based on the data type and structure
				if (String(dataType) === 'JSON') {
					if (Array.isArray(msg)) {
						// Array: check its length
						return msg.length >= 2;
					} else if (typeof msg === 'object') {
						// Object: check for a temperature field whose value is greater than 25
						return msg.temperature && msg.temperature > 25;
					}
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Test JSON array (length>=2)
		arrayMetadata := types.BuildMetadata(make(map[string]string))
		arrayData := `["item1", "item2", "item3"]`
		arrayMsg := types.NewMsg(0, "MIXED_TEST", types.JSON, arrayMetadata, arrayData)

		var arrayResult string
		var arrayErr error

		arrayCtx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			arrayResult = relationType
			arrayErr = err
		})

		node.OnMsg(arrayCtx, arrayMsg)

		// Verify array filtering results
		assert.Nil(t, arrayErr)
		assert.Equal(t, types.True, arrayResult)

		// Test JSON object (temperature > 25)
		objectMetadata := types.BuildMetadata(make(map[string]string))
		objectData := `{"name": "sensor", "temperature": 30}`
		objectMsg := types.NewMsg(0, "MIXED_TEST", types.JSON, objectMetadata, objectData)

		var objectResult string
		var objectErr error

		objectCtx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			objectResult = relationType
			objectErr = err
		})

		node.OnMsg(objectCtx, objectMsg)

		// Verify the object filtering results
		assert.Nil(t, objectErr)
		assert.Equal(t, types.True, objectResult)

		// Test JSON object (temperature < = 25)
		lowTempMetadata := types.BuildMetadata(make(map[string]string))
		lowTempData := `{"name": "sensor", "temperature": 20}`
		lowTempMsg := types.NewMsg(0, "MIXED_TEST", types.JSON, lowTempMetadata, lowTempData)

		var lowTempResult string
		var lowTempErr error

		lowTempCtx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			lowTempResult = relationType
			lowTempErr = err
		})

		node.OnMsg(lowTempCtx, lowTempMsg)

		// Verify that low-temperature objects are filtered
		assert.Nil(t, lowTempErr)
		assert.Equal(t, types.False, lowTempResult)
	})

	// Test 5: Nested array filtering
	t.Run("NestedArrayFilter", func(t *testing.T) {
		node := &JsFilterNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Check whether a nested array contains a child array whose sum is greater than 10
				if (Array.isArray(msg)) {
					for (var i = 0; i < msg.length; i++) {
						var item = msg[i];
						if (Array.isArray(item)) {
							var sum = 0;
							for (var j = 0; j < item.length; j++) {
								if (typeof item[j] === 'number') {
									sum += item[j];
								}
							}
							if (sum > 10) {
								return true;
							}
						}
					}
				}
				return false;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Create nested arrays containing and more than 10 subarrays
		metadata1 := types.BuildMetadata(make(map[string]string))
		nestedData1 := `[[1, 2], [5, 8], [2, 3]]` // The sum of the second subarray is 13
		testMsg1 := types.NewMsg(0, "NESTED_TEST", types.JSON, metadata1, nestedData1)

		var result1 string
		var err1 error

		ctx1 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result1 = relationType
			err1 = err
		})

		node.OnMsg(ctx1, testMsg1)

		// Verification result: If there are subarrays greater than 10, it should pass
		assert.Nil(t, err1)
		assert.Equal(t, types.True, result1)

		// Create nested arrays where the sum of all subarrays is less than or equal to 10
		metadata2 := types.BuildMetadata(make(map[string]string))
		nestedData2 := `[[1, 2], [3, 4], [2, 3]]` // All subarray sums ≤ 10
		testMsg2 := types.NewMsg(0, "NESTED_TEST", types.JSON, metadata2, nestedData2)

		var result2 string
		var err2 error

		ctx2 := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			result2 = relationType
			err2 = err
		})

		node.OnMsg(ctx2, testMsg2)

		// Verification result: No subarrays and values greater than 10 should be filtered
		assert.Nil(t, err2)
		assert.Equal(t, types.False, result2)
	})
}
