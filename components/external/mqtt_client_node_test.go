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

package external

import (
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"testing"
	"time"
)

func TestMqttClientNode(t *testing.T) {
	var targetNodeType = "mqttClient"

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &MqttClientNode{}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qOS":                  uint8(0),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qOS":                  uint8(1),
			"MaxReconnectInterval": 60,
		}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qOS":                  uint8(1),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qOS":                  uint8(1),
			"MaxReconnectInterval": 60,
		}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qOS":                  uint8(1),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"maxReconnectInterval": -1,
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"topic":  "/device/msg",
			"server": "127.0.0.1:1884",
		}, Registry)
		assert.Nil(t, err)

		nodeClientFromPool, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"server":               types.NodeConfigurationPrefixInstanceId + "127.0.0.1:1883",
			"topic":                "/device/msg",
			"maxReconnectInterval": -1,
		}, Registry)
		assert.Nil(t, err)
		assert.Equal(t, targetNodeType, nodeClientFromPool.(*MqttClientNode).Type())
		assert.Equal(t, "127.0.0.1:1883", nodeClientFromPool.(*MqttClientNode).InstanceId)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData: metaData,
				MsgType:  "ACTIVITY_EVENT1",
				Data:     "AA",
			},
			{
				MetaData: metaData,
				MsgType:  "ACTIVITY_EVENT2",
				Data:     "{\"temperature\":60}",
			},
		}
		_ = node1
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    nodeClientFromPool,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Second * 2)
	})
}
