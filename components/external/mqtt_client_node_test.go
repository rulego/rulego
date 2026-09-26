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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

func TestMqttClientNode(t *testing.T) {
	var targetNodeType = "mqttClient"

	// in-process broker so the happy path works without an external mosquitto
	broker, err := test.NewMqttBroker("127.0.0.1:0")
	assert.Nil(t, err)
	defer broker.Close()
	brokerAddr := broker.Addr()

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &MqttClientNode{}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qos":                  uint8(0),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qos":                  uint8(1),
			"MaxReconnectInterval": 60,
		}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qos":                  uint8(1),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qos":                  uint8(1),
			"MaxReconnectInterval": 60,
		}, types.Configuration{
			"topic":                "/device/msg",
			"server":               "127.0.0.1:1883",
			"qos":                  uint8(1),
			"maxReconnectInterval": 60,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"topic":                "/device/msg",
			"server":               brokerAddr,
			"maxReconnectInterval": -1,
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"topic":  "/device/msg",
			"server": "127.0.0.1:1884",
		}, Registry)
		assert.Nil(t, err)

		nodeClientFromPool, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"server":               types.NodeConfigurationPrefixInstanceId + brokerAddr,
			"topic":                "/device/msg",
			"maxReconnectInterval": -1,
		}, Registry)
		assert.Nil(t, err)
		assert.Equal(t, targetNodeType, nodeClientFromPool.(*MqttClientNode).Type())
		assert.Equal(t, brokerAddr, nodeClientFromPool.(*MqttClientNode).InstanceId)

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
		// count callbacks so the test waits for the dead-server node instead of
		// letting its late failure assert fire after the test completes
		var pending int32 = int32(len(msgList) * 3)
		allDone := make(chan struct{})
		expect := func(want string) func(types.RuleMsg, string, error) {
			return func(msg types.RuleMsg, relationType string, err error) {
				assert.Equal(t, want, relationType)
				if atomic.AddInt32(&pending, -1) == 0 {
					close(allDone)
				}
			}
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:     node1,
				MsgList:  msgList,
				Callback: expect(types.Success),
			},
			{
				Node:     node2,
				MsgList:  msgList,
				Callback: expect(types.Failure),
			},
			{
				Node:     nodeClientFromPool,
				MsgList:  msgList,
				Callback: expect(types.Failure),
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		select {
		case <-allDone:
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for node callbacks")
		}
		node1.Destroy()
		node2.Destroy()
		nodeClientFromPool.Destroy()
	})
}
