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

package flow

import (
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestFlowNode(t *testing.T) {
	var targetNodeType = "flow"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ChainNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		// Test successful initialization with empty configuration
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		// Test successful initialization with default configuration
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)
	})

	t.Run("OnMsg", func(t *testing.T) {
		// Test cases with different configurations
		testCases := []struct {
			name             string
			config           types.Configuration
			expectedRelation string
		}{
			{
				name: "RuleTarget",
				config: types.Configuration{
					"targetId": "rule01",
				},
				expectedRelation: types.Success,
			},
			{
				name: "ToTrueWithoutExtend",
				config: types.Configuration{
					"targetId": "toTrue",
					"extend":   false,
				},
				expectedRelation: types.Success,
			},
			{
				name: "ToTrueWithExtend",
				config: types.Configuration{
					"targetId": "toTrue",
					"extend":   true,
				},
				expectedRelation: types.True,
			},
			{
				name: "NotFoundWithExtend",
				config: types.Configuration{
					"targetId": "notfound",
					"extend":   true,
				},
				expectedRelation: types.Failure,
			},
		}

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		testMsg := test.Msg{
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT2",
			Data:       "{\"temperature\":60}",
			AfterSleep: time.Millisecond * 200,
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, tc.config, Registry)
				assert.Nil(t, err)

				// Use a struct to pass results through channel to avoid data race
				type testResult struct {
					relationType string
					err          error
				}
				resultChan := make(chan testResult, 1)

				callback := func(msg types.RuleMsg, relationType string, err error) {
					resultChan <- testResult{
						relationType: relationType,
						err:          err,
					}
				}

				test.NodeOnMsgWithChildrenAndConfig(t, types.NewConfig(), node, []test.Msg{testMsg}, nil, callback)

				select {
				case result := <-resultChan:
					// Use simple string comparison instead of assert.Equal to avoid circular references
					if result.relationType != tc.expectedRelation {
						t.Errorf("Expected relation type %s, got %s", tc.expectedRelation, result.relationType)
					}
					if tc.expectedRelation == types.Failure {
						assert.NotNil(t, result.err)
					}
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})
		}
	})

	t.Run("EmptyConfigInit", func(t *testing.T) {
		// Test that ChainNode can initialize with empty config (unlike RefNode)
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)
		assert.NotNil(t, node)

		chainNode := node.(*ChainNode)
		// With empty config, TargetId should be empty string
		assert.Equal(t, "", chainNode.Config.TargetId)
		assert.Equal(t, false, chainNode.Config.Extend) // default value
	})
}
