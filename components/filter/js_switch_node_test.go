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

func TestJsSwitchNode(t *testing.T) {
	var targetNodeType = "jsSwitch"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &JsSwitchNode{}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, types.Configuration{
			"jsScript": `return ['msgType1','msgType2'];`,
		}, Registry)
	})

	t.Run("InitNodeDefault", func(t *testing.T) {
		config := types.NewConfig()
		node := JsSwitchNode{}
		err := node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, types.DefaultRelationType)

		config.Properties.PutValue(types.DefaultRelationTypeKey, "Default")
		err = node.Init(config, types.Configuration{})
		assert.Nil(t, err)
		assert.Equal(t, node.defaultRelationType, "Default")
	})
	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": "return ['one','two'];",
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
			jsScript := node.(*JsSwitchNode).Config.JsScript

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
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if jsScript == `return 1` {
					assert.Equal(t, JsSwitchReturnFormatErr.Error(), err2.Error())
				} else if jsScript == `return a` {
					assert.NotNil(t, err2)
				} else {
					assert.True(t, relationType == "one" || relationType == "two")
				}

			})
		}
	})
}

// TestJsSwitchNodeDataType tests passing dataType parameters
func TestJsSwitchNodeDataType(t *testing.T) {
	config := types.NewConfig()

	t.Run("DataTypeParameter", func(t *testing.T) {
		// Create test messages for different data types
		testCases := []struct {
			dataType types.DataType
			expected string
		}{
			{types.JSON, "JSON"},
			{types.TEXT, "TEXT"},
			{types.BINARY, "BINARY"},
		}

		for _, tc := range testCases {
			t.Run(string(tc.dataType), func(t *testing.T) {
				node := &JsSwitchNode{}
				err := node.Init(config, types.Configuration{
					"jsScript": "return [String(dataType)];", // After converting to a string, it returns as a route
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
				assert.Equal(t, tc.expected, resultRelationType) // The routing type should equal dataType
			})
		}
	})
}

// TestJsSwitchNodeJSONArraySupport Tests JavaScript switch component support for JSON arrays
func TestJsSwitchNodeJSONArraySupport(t *testing.T) {
	config := types.NewConfig()

	// Test 1: Routing based on JSON array length
	t.Run("ArrayLengthSwitch", func(t *testing.T) {
		node := &JsSwitchNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Select the route based on the array length
				if (Array.isArray(msg)) {
					if (msg.length <= 2) {
						return ['small_array'];
					} else if (msg.length <= 5) {
						return ['medium_array'];
					} else {
						return ['large_array'];
					}
				}
				return ['unknown'];
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		testCases := []struct {
			name          string
			arrayData     string
			expectedRoute string
		}{
			{
				name:          "Small array (length 2)",
				arrayData:     `["a", "b"]`,
				expectedRoute: "small_array",
			},
			{
				name:          "Medium array (length 4)",
				arrayData:     `["a", "b", "c", "d"]`,
				expectedRoute: "medium_array",
			},
			{
				name:          "Large array (length 7)",
				arrayData:     `["a", "b", "c", "d", "e", "f", "g"]`,
				expectedRoute: "large_array",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsg(0, "ARRAY_SWITCH", types.JSON, metadata, tc.arrayData)

				var resultRelationType string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					resultRelationType = relationType
					resultErr = err
				})

				node.OnMsg(ctx, testMsg)

				assert.Nil(t, resultErr)
				assert.Equal(t, tc.expectedRoute, resultRelationType)
			})
		}
	})

	// Test 2: Multi-routing based on JSON array content
	t.Run("ArrayContentMultiSwitch", func(t *testing.T) {
		node := &JsSwitchNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Select multiple routes based on the array contents
				var routes = [];
				
				if (Array.isArray(msg)) {
					// Check whether the array contains a number
					var hasNumber = false;
					var hasString = false;
					var hasObject = false;
					
					for (var i = 0; i < msg.length; i++) {
						if (typeof msg[i] === 'number') {
							hasNumber = true;
						} else if (typeof msg[i] === 'string') {
							hasString = true;
						} else if (typeof msg[i] === 'object') {
							hasObject = true;
						}
					}
					
					if (hasNumber) routes.push('has_numbers');
					if (hasString) routes.push('has_strings');
					if (hasObject) routes.push('has_objects');
				}
				
				return routes.length > 0 ? routes : ['empty'];
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Test the mixed type array
		metadata := types.BuildMetadata(make(map[string]string))
		mixedArrayData := `[42, "hello", {"key": "value"}]`
		testMsg := types.NewMsg(0, "MIXED_SWITCH", types.JSON, metadata, mixedArrayData)

		var routes []string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			if err == nil {
				routes = append(routes, relationType)
			} else {
				resultErr = err
			}
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		// Manually check whether the route contains the expected value
		hasNumbers := false
		hasStrings := false
		hasObjects := false
		for _, route := range routes {
			if route == "has_numbers" {
				hasNumbers = true
			} else if route == "has_strings" {
				hasStrings = true
			} else if route == "has_objects" {
				hasObjects = true
			}
		}
		assert.True(t, hasNumbers, "Expected route 'has_numbers' not found")
		assert.True(t, hasStrings, "Expected route 'has_strings' not found")
		assert.True(t, hasObjects, "Expected route 'has_objects' not found")
		assert.Equal(t, 3, len(routes))
	})

	// Test 3: Digital array range routing
	t.Run("NumericArrayRangeSwitch", func(t *testing.T) {
		node := &JsSwitchNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Route based on the numeric array's value range
				if (Array.isArray(msg)) {
					var routes = [];
					var hasLow = false;    // < 50
					var hasMedium = false; // 50-100
					var hasHigh = false;   // > 100
					
					for (var i = 0; i < msg.length; i++) {
						if (typeof msg[i] === 'number') {
							if (msg[i] < 50) {
								hasLow = true;
							} else if (msg[i] <= 100) {
								hasMedium = true;
							} else {
								hasHigh = true;
							}
						}
					}
					
					if (hasLow) routes.push('low_values');
					if (hasMedium) routes.push('medium_values');
					if (hasHigh) routes.push('high_values');
					
					return routes;
				}
				return ['non_array'];
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Testing arrays containing values of different ranges
		metadata := types.BuildMetadata(make(map[string]string))
		numericArrayData := `[25, 75, 150, 10, 200]`
		testMsg := types.NewMsg(0, "NUMERIC_SWITCH", types.JSON, metadata, numericArrayData)

		var routes []string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			if err == nil {
				routes = append(routes, relationType)
			} else {
				resultErr = err
			}
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		// Manually check the numerical range of the route
		hasLow := false
		hasMedium := false
		hasHigh := false
		for _, route := range routes {
			if route == "low_values" {
				hasLow = true
			} else if route == "medium_values" {
				hasMedium = true
			} else if route == "high_values" {
				hasHigh = true
			}
		}
		assert.True(t, hasLow, "Expected route 'low_values' not found")       // 25, 10
		assert.True(t, hasMedium, "Expected route 'medium_values' not found") // 75
		assert.True(t, hasHigh, "Expected route 'high_values' not found")     // 150, 200
		assert.Equal(t, 3, len(routes))
	})

	// Test 4: Nested array structure routing
	t.Run("NestedArrayStructureSwitch", func(t *testing.T) {
		node := &JsSwitchNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Analyze the nested array structure and route it
				if (Array.isArray(msg)) {
					var routes = [];
					var maxDepth = 0;
					var totalElements = 0;
					
					function analyzeArray(arr, depth) {
						if (depth > maxDepth) maxDepth = depth;
						
						for (var i = 0; i < arr.length; i++) {
							totalElements++;
							if (Array.isArray(arr[i])) {
								analyzeArray(arr[i], depth + 1);
							}
						}
					}
					
					analyzeArray(msg, 1);
					
					// Route based on the analysis result
					routes.push('depth_' + maxDepth);
					
					if (totalElements <= 5) {
						routes.push('small_data');
					} else if (totalElements <= 15) {
						routes.push('medium_data');
					} else {
						routes.push('large_data');
					}
					
					return routes;
				}
				return ['invalid'];
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		// Test nested arrays
		metadata := types.BuildMetadata(make(map[string]string))
		nestedArrayData := `[[1, 2], [3, [4, 5]], [6, 7, 8]]`
		testMsg := types.NewMsg(0, "NESTED_SWITCH", types.JSON, metadata, nestedArrayData)

		var routes []string
		var resultErr error

		ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
			if err == nil {
				routes = append(routes, relationType)
			} else {
				resultErr = err
			}
		})

		node.OnMsg(ctx, testMsg)

		assert.Nil(t, resultErr)
		// Manually check the nested array routes
		hasDepth3 := false
		hasMediumData := false
		for _, route := range routes {
			if route == "depth_3" {
				hasDepth3 = true
			} else if route == "medium_data" {
				hasMediumData = true
			}
		}
		assert.True(t, hasDepth3, "Expected route 'depth_3' not found")         // The maximum depth is 3
		assert.True(t, hasMediumData, "Expected route 'medium_data' not found") // The total number of elements is in the medium range
		assert.Equal(t, 2, len(routes))
	})

	// Test 5: Conditional Routing (Array vs. Object)
	t.Run("ConditionalTypeSwitch", func(t *testing.T) {
		node := &JsSwitchNode{}
		err := node.Init(config, types.Configuration{
			"jsScript": `
				// Route based on the data type and conditions
				var routes = [];
				
				if (String(dataType) === 'JSON') {
					if (Array.isArray(msg)) {
						routes.push('json_array');
						
						// Analyze the array further
						if (msg.length === 0) {
							routes.push('empty_array');
						} else if (msg.every(function(item) { return typeof item === 'string'; })) {
							routes.push('string_array');
						} else if (msg.every(function(item) { return typeof item === 'number'; })) {
							routes.push('numeric_array');
						} else {
							routes.push('mixed_array');
						}
					} else if (typeof msg === 'object') {
						routes.push('json_object');
						
						// Check specific fields
						if (msg.hasOwnProperty('temperature')) {
							routes.push('sensor_data');
						}
						if (msg.hasOwnProperty('error')) {
							routes.push('error_data');
						}
					}
				} else {
					routes.push('non_json');
				}
				
				return routes;
			`,
		})
		assert.Nil(t, err)
		defer node.Destroy()

		testCases := []struct {
			name           string
			dataType       types.DataType
			data           string
			expectedRoutes []string
		}{
			{
				name:           "String array",
				dataType:       types.JSON,
				data:           `["apple", "banana", "cherry"]`,
				expectedRoutes: []string{"json_array", "string_array"},
			},
			{
				name:           "Numeric array",
				dataType:       types.JSON,
				data:           `[1, 2, 3, 4, 5]`,
				expectedRoutes: []string{"json_array", "numeric_array"},
			},
			{
				name:           "Mixed array",
				dataType:       types.JSON,
				data:           `[1, "text", true]`,
				expectedRoutes: []string{"json_array", "mixed_array"},
			},
			{
				name:           "Sensor object",
				dataType:       types.JSON,
				data:           `{"temperature": 25, "humidity": 60}`,
				expectedRoutes: []string{"json_object", "sensor_data"},
			},
			{
				name:           "Error object",
				dataType:       types.JSON,
				data:           `{"error": "connection failed", "code": 500}`,
				expectedRoutes: []string{"json_object", "error_data"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				metadata := types.BuildMetadata(make(map[string]string))
				testMsg := types.NewMsg(0, "CONDITIONAL_SWITCH", tc.dataType, metadata, tc.data)

				var routes []string
				var resultErr error

				ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
					if err == nil {
						routes = append(routes, relationType)
					} else {
						resultErr = err
					}
				})

				node.OnMsg(ctx, testMsg)

				assert.Nil(t, resultErr)
				// Manually check all expected routes
				for _, expectedRoute := range tc.expectedRoutes {
					found := false
					for _, route := range routes {
						if route == expectedRoute {
							found = true
							break
						}
					}
					assert.True(t, found, "Missing expected route: %s", expectedRoute)
				}
			})
		}
	})
}
