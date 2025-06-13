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

package action

import (
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestJsLogNode(t *testing.T) {
	var targetNodeType = "log"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &LogNode{}, types.Configuration{
			"jsScript": `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return 'Incoming message1:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, types.Configuration{
			"jsScript": `return 'Incoming message1:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"jsScript": `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, types.Configuration{
			"jsScript": `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return 'Incoming message:\n' + JSON.stringify(msg) + '\nIncoming metadata:\n' + JSON.stringify(metadata);`,
		}, Registry)
		assert.Nil(t, err)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return true`,
		}, Registry)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"jsScript": `return a`,
		}, Registry)

		var nodeList = []types.Node{node1, node2, node3}

		for _, node := range nodeList {
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
			jsScript := node.(*LogNode).Config.JsScript
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if jsScript == `return true` {
					assert.Equal(t, JsLogReturnFormatErr.Error(), err2.Error())
				} else if jsScript == `return a` {
					assert.NotNil(t, err2)
				} else {
					assert.Equal(t, "test", msg.Metadata.GetValue("productType"))
				}

			})
		}
	})
}
