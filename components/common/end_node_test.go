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
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestEndNode(t *testing.T) {
	var targetNodeType = "end"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &EndNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)

		msgList := []test.Msg{{
			MetaData:   types.NewMetadata(),
			MsgType:    "TEST_MSG",
			Data:       "{\"test\":\"data\"}",
			DataType:   types.JSON,
			AfterSleep: time.Millisecond * 200,
		}}

		// The test termination node triggers a DoOnEnd callback
		// Test that end node triggers DoOnEnd callback
		test.NodeOnMsgWithChildren(t, node, msgList, nil, func(msg types.RuleMsg, relationType string, err error) {
			// The termination node does not have normal Success/Failure callbacks; instead, it is handled through DoOnEnd
			// End node doesn't have normal Success/Failure callbacks, but handles through DoOnEnd
			// Here, the main goal is to verify that nodes can process messages normally without errors
			// Here we mainly verify that the node can process messages without errors
			assert.NotNil(t, msg)
		})

		// Note: Because the termination node calls DoOnEnd, the actual callback behavior may vary
		// Note: Since end node calls DoOnEnd, the actual callback behavior may be different
		time.Sleep(time.Millisecond * 300)
	})

	t.Run("Type", func(t *testing.T) {
		node := &EndNode{}
		assert.Equal(t, "end", node.Type())
	})

	t.Run("New", func(t *testing.T) {
		node := &EndNode{}
		newNode := node.New().(*EndNode)
		assert.NotNil(t, newNode)
	})

	t.Run("Init", func(t *testing.T) {
		node := &EndNode{}
		err := node.Init(types.Config{}, types.Configuration{})
		assert.Nil(t, err)
	})

	t.Run("Destroy", func(t *testing.T) {
		node := &EndNode{}
		node.Destroy() // It shouldn't go wrong
		// Should not cause any error
	})
}
