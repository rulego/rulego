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
	"net"
	"testing"
	"time"
)

func TestNetNode(t *testing.T) {
	var targetNodeType = "net"
	config := NetNodeConfiguration{
		Protocol: "tcp",
		Server:   "127.0.0.1:9999",
	}
	stop := make(chan struct{})
	//启动服务
	go createNetServer(config, stop)
	time.Sleep(time.Millisecond * 200)

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &NetNode{}, types.Configuration{
			"protocol":          "tcp",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"server":            "127.0.0.1:9999",
			"connectTimeout":    -1,
			"heartbeatInterval": -1,
		}, types.Configuration{
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:9999",
			"connectTimeout":    60,
			"heartbeatInterval": 60,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"server":            "127.0.0.1:9999",
			"heartbeatInterval": 1,
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"protocol":          "tcp",
			"server":            "127.0.0.1:6666",
			"connectTimeout":    60,
			"heartbeatInterval": 1,
		}, Registry)
		assert.Nil(t, err)

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT1",
				Data:       "AA",
				AfterSleep: time.Millisecond * 200,
			},
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Second * 3,
			},
		}

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
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

		//time.Sleep(time.Second * 10)
		stop <- struct{}{}

	})
}

// 创建net服务
func createNetServer(config NetNodeConfiguration, stop chan struct{}) {
	//var err error
	// 根据配置的协议和地址，创建一个服务器监听器
	listener, err := net.Listen(config.Protocol, config.Server)
	if err != nil {
		return
	}
	go func() {
		for {
			select {
			case <-stop:
				// 接收到中断信号，退出循环
				listener.Close()
				return
			default:
			}
		}
	}()
	// 循环接受客户端的连接请求
	for {
		// 从监听器中获取一个客户端连接，返回连接对象和错误信息
		_, err := listener.Accept()
		if err != nil {
			if opError, ok := err.(*net.OpError); ok && opError.Err == net.ErrClosed {
				return
			} else {
				continue
			}
		}
	}
}
