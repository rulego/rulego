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
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestFunctionsNode(t *testing.T) {

	//not init
	_, ok := Functions.Get("add2")
	assert.False(t, ok)

	Functions.Register("add", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("addFrom", "addFunction")
		ctx.TellNext(msg, "fromAdd")
	})

	Functions.Register("add2", func(ctx types.RuleContext, msg types.RuleMsg) {
		msg.Metadata.PutValue("addFrom", "addFunction2")
		ctx.TellNext(msg, "fromAdd2")
	})

	_, ok = Functions.Get("add2")
	assert.True(t, ok)
	Functions.UnRegister("add2")

	_, ok = Functions.Get("add2")
	assert.False(t, ok)

	var targetNodeType = "functions"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &FunctionsNode{}, types.Configuration{
			"functionName": "test",
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"functionName": "add",
		}, types.Configuration{
			"functionName": "add",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"functionName": "test",
		}, types.Configuration{
			"functionName": "test",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "add",
		}, Registry)
		assert.Nil(t, err)
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "${functionName}",
		}, Registry)
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"functionName": "aa",
		}, Registry)

		var nodeList = []types.Node{node1, node2, node3}

		for _, node := range nodeList {
			metaData := types.BuildMetadata(make(map[string]string))
			metaData.PutValue("productType", "test")
			metaData.PutValue("functionName", "add")
			var msgList = []test.Msg{
				{
					MetaData:   metaData,
					MsgType:    "ACTIVITY_EVENT",
					Data:       "AA",
					AfterSleep: time.Millisecond * 200,
				},
			}
			test.NodeOnMsg(t, node, msgList, func(msg types.RuleMsg, relationType string, err2 error) {
				if node.(*FunctionsNode).Config.FunctionName == "aa" {
					assert.Equal(t, "can not found the function=aa", err2.Error())
				} else {
					assert.Equal(t, "addFunction", msg.Metadata.GetValue("addFrom"))
					assert.Equal(t, "fromAdd", relationType)
				}

			})
		}
	})
}
