/*
 * Copyright 2025 The RuleGo Authors.
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

package common

import (
	"strings"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestRefNode(t *testing.T) {
	var targetNodeType = "ref"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &RefNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		// Test successful initialization with valid targetId
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"targetId": "test_node",
		}, Registry)
		assert.Nil(t, err)
		refNode := node.(*RefNode)
		assert.Equal(t, "test_node", refNode.nodeId)
		assert.Equal(t, "", refNode.chainId)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		// Test that initialization fails with empty configuration
		_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "targetId is empty"))
	})

	t.Run("OnMsg", func(t *testing.T) {
		// Test initialization failure with empty targetId
		t.Run("EmptyTargetId", func(t *testing.T) {
			_, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
			assert.NotNil(t, err)
			assert.True(t, strings.Contains(err.Error(), "targetId is empty"))
		})

		// Test valid external chain reference
		t.Run("ExternalChainReference", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"targetId": "chain01:node01",
			}, Registry)
			assert.Nil(t, err)
			refNode := node.(*RefNode)
			assert.Equal(t, "chain01", refNode.chainId)
			assert.Equal(t, "node01", refNode.nodeId)
		})

		// Test valid local node reference
		t.Run("LocalNodeReference", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"targetId": "node01",
			}, Registry)
			assert.Nil(t, err)
			refNode := node.(*RefNode)
			assert.Equal(t, "", refNode.chainId)
			assert.Equal(t, "node01", refNode.nodeId)
		})

		// Test message handling with non-existent target nodes
		t.Run("MessageHandling", func(t *testing.T) {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")

			testMsg := test.Msg{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			}

			// Test external chain reference (should fail since chain doesn't exist)
			t.Run("ExternalChain", func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
					"targetId": "chain01:node01",
				}, Registry)
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
					assert.Equal(t, types.Failure, result.relationType)
					assert.NotNil(t, result.err)
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})

			// Test local node reference (should fail since node doesn't exist)
			t.Run("LocalNode", func(t *testing.T) {
				node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
					"targetId": "node01",
				}, Registry)
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
					assert.Equal(t, types.Failure, result.relationType)
					assert.NotNil(t, result.err)
				case <-time.After(time.Second):
					t.Fatal("Test timed out")
				}
			})
		})
	})
}
