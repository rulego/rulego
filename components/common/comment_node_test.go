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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestCommentNode(t *testing.T) {

	var targetNodeType = "comment"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &CommentNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{}, types.Configuration{}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{}, Registry)
		assert.Nil(t, err)
		var msgList = []test.Msg{
			{
				MsgType:    "ACTIVITY_EVENT",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
		}
		test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
			assert.Equal(t, types.Success, relationType)
		})
	})
}
