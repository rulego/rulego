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
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSshNode(t *testing.T) {
	var targetNodeType = "ssh"

	serverIp := os.Getenv("TEST_SERVER_IP")
	serverPort := os.Getenv("TEST_SERVER_PORT")
	if serverPort == "" {
		serverPort = "22"
	}
	serverUsername := os.Getenv("TEST_SERVER_USERNAME")
	serverPassword := os.Getenv("TEST_SERVER_PASSWORD")

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &SshNode{}, types.Configuration{
			"host":     "127.0.0.1",
			"port":     22,
			"username": "root",
			"password": "password",
		}, Registry)
	})

	port, err := strconv.Atoi(serverPort)
	if err != nil {
		port = 22
	}
	t.Run("InitNode", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		test.NodeInit(t, targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
		}, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		test.NodeInit(t, targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
		}, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo \"hello world\"",
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     "127.0.0.1",
			"Port":     22,
			"username": "root",
			"password": "password",
		}, Registry)
		assert.NotNil(t, err)

		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "",
		}, Registry)
		assert.Nil(t, err)

		node4 := &SshNode{}
		err = node4.Init(types.NewConfig(), types.Configuration{})
		assert.Equal(t, SshConfigEmptyErr.Error(), err.Error())
		ctx := test.NewRuleContextFull(types.NewConfig(), node4, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, SshClientNotInitErr.Error(), err.Error())
		})
		node4.OnMsg(ctx, types.RuleMsg{})

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {

					assert.True(t, strings.Contains(msg.Data, "hello world"))
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
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, SshCmdEmptyErr.Error(), err.Error())
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
	})
}
