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
	"testing"
	"time"
)

func TestRestApiCallNode(t *testing.T) {
	var targetNodeType = "restApiCall"
	headers := map[string]string{"Content-Type": "application/json"}
	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &RestApiCallNode{}, types.Configuration{
			"requestMethod":            "POST",
			"maxParallelRequestsCount": 200,
			"readTimeoutMs":            2000,
			"headers":                  headers,
		}, Registry)
	})

	t.Run("InitNode", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"requestMethod":            "GET",
			"maxParallelRequestsCount": 100,
			"readTimeoutMs":            0,
			"withoutRequestBody":       true,
			"headers":                  headers,
		}, types.Configuration{
			"requestMethod":            "GET",
			"maxParallelRequestsCount": 100,
			"readTimeoutMs":            0,
			"withoutRequestBody":       true,
			"headers":                  headers,
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		test.NodeInit(t, targetNodeType, types.Configuration{
			"requestMethod":            "POST",
			"maxParallelRequestsCount": 200,
			"readTimeoutMs":            2000,
			"headers":                  headers,
		}, types.Configuration{
			"requestMethod":            "POST",
			"maxParallelRequestsCount": 200,
			"readTimeoutMs":            2000,
			"headers":                  headers,
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc/notfound",
			"requestMethod":          "POST",
		}, Registry)
		assert.Nil(t, err)

		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc/",
			"requestMethod":          "GET",
		}, Registry)
		assert.Nil(t, err)

		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.xx/",
			"requestMethod":          "GET",
			"enableProxy":            true,
			"proxyScheme":            "http",
			"proxyHost":              "127.0.0.1",
			"proxyPor":               "10809",
		}, Registry)

		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc/",
			"requestMethod":          "GET",
			"withoutRequestBody":     true,
		}, Registry)

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
					code := msg.Metadata.GetValue(statusCodeMetadataKey)
					assert.Equal(t, "405", code)
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
				},
			},
			{
				Node:    node4,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
	})

	//SSE(Server-Sent Events)流式请求
	t.Run("SSEOnMsg", func(t *testing.T) {
		sseServer := os.Getenv("TEST_SSE_SERVER")
		if sseServer == "" {
			return
		}
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"headers":                map[string]string{"Content-Type": "application/json", "Accept": "text/event-stream"},
			"restEndpointUrlPattern": sseServer,
			"requestMethod":          "POST",
		}, Registry)
		assert.Nil(t, err)

		//404
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"headers":                map[string]string{"Content-Type": "application/json", "Accept": "text/event-stream"},
			"restEndpointUrlPattern": sseServer + "/nothings/",
			"requestMethod":          "POST",
		}, Registry)
		assert.Nil(t, err)

		done := false

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")

		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "chat",
				Data:       "{\"model\": \"chatglm3-6b-32k\", \"messages\": [{\"role\": \"system\", \"content\": \"You are ChatGLM3, a large language model trained by Zhipu.AI. Follow the user's instructions carefully. Respond using markdown.\"}, {\"role\": \"user\", \"content\": \"你好，给我讲一个故事，大概100字\"}], \"stream\": true, \"max_tokens\": 100, \"temperature\": 0.8, \"top_p\": 0.8}",
				AfterSleep: time.Millisecond * 200,
			},
		}
		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
					assert.Equal(t, "200", msg.Metadata.GetValue(statusCodeMetadataKey))
					if msg.GetData() == "[DONE]" {
						done = true
						assert.Equal(t, "data", msg.Metadata.GetValue(eventTypeMetadataKey))
					}
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
					assert.Equal(t, "404", msg.Metadata.GetValue(statusCodeMetadataKey))
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

		time.Sleep(time.Second * 1)
		assert.True(t, done)
	})
}
