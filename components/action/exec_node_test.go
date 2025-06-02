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
	"sync/atomic"
	"testing"
	"time"
)

func TestTemplateNode(t *testing.T) {
	var targetNodeType = "exec"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &ExecCommandNode{}, types.Configuration{}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"cmd":  "ls",
			"args": []string{"./data"},
		}, types.Configuration{
			"cmd":  "ls",
			"args": []string{"./data"},
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		metaData.PutValue("dir", ".")
		msg := test.Msg{
			Id:         "226a05f1-9464-43b6-881e-b1629f1b030d",
			Ts:         1719024872741,
			MetaData:   metaData,
			MsgType:    "ACTIVITY_EVENT",
			Data:       "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}",
			AfterSleep: time.Millisecond * 200,
		}

		count := int32(0)
		config := types.NewConfig()
		config.Properties.PutValue(KeyExecNodeWhitelist, "ls,cd")
		config.OnDebug = func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, types.Log, flowType)
			assert.Equal(t, "info", relationType)
			atomic.AddInt32(&count, 1)
		}
		var data1, data2 string
		var nodeList = []test.NodeAndCallback{
			{
				Node: test.InitNodeByConfig(types.NewConfig(), targetNodeType, types.Configuration{
					"cmd":  "ls",
					"args": []string{"."},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, ErrCmdNotAllowed.Error(), err.Error())
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"xx"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":  "ls",
					"args": []string{"."},
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir}"},
					"log":         true,
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir} -l"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
					data1 = msg.GetData()
				},
			},
			{
				Node: test.InitNodeByConfig(config, targetNodeType, types.Configuration{
					"cmd":         "ls",
					"args":        []string{"${dir}", "-l"},
					"replaceData": true,
				}, Registry),
				MsgList: []test.Msg{msg},
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.NotEqual(t, "{\"name\":\"aa\",\"temperature\":60,\"humidity\":30}", msg.GetData())
					assert.Equal(t, types.Success, relationType)
					data2 = msg.GetData()
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildrenAndConfig(t, config, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Second)
		assert.Equal(t, int32(1), count)
		assert.Equal(t, data1, data2)
	})
}
