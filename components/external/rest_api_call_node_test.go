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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
)

const (
	// 代理测试开关
	ENABLE_PROXY_TEST = false
	// 代理配置
	PROXY_HOST        = "127.0.0.1"
	HTTP_PROXY_PORT   = 10809
	SOCKS5_PROXY_PORT = 10808
)

func TestRestApiCallNode(t *testing.T) {
	var targetNodeType = "restApiCall"
	headers := map[string]string{"Content-Type": "application/json"}
	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &RestApiCallNode{}, types.Configuration{
			"requestMethod":            "POST",
			"maxParallelRequestsCount": 200,
			"readTimeoutMs":            2000,
			"insecureSkipVerify":       true,
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
			"insecureSkipVerify":       true,
			"headers":                  headers,
		}, types.Configuration{
			"requestMethod":            "POST",
			"maxParallelRequestsCount": 200,
			"readTimeoutMs":            2000,
			"insecureSkipVerify":       true,
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
					code := msg.Metadata.GetValue(StatusCodeMetadataKey)
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
					assert.Equal(t, "200", msg.Metadata.GetValue(StatusCodeMetadataKey))
					if msg.GetData() == "[DONE]" {
						done = true
						assert.Equal(t, "data", msg.Metadata.GetValue(EventTypeMetadataKey))
					}
				},
			},
			{
				Node:    node2,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Failure, relationType)
					assert.Equal(t, "404", msg.Metadata.GetValue(StatusCodeMetadataKey))
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}

		time.Sleep(time.Second * 1)
		assert.True(t, done)
	})

	// 代理测试
	t.Run("ProxyTest", func(t *testing.T) {
		// 检查是否启用代理测试
		if !ENABLE_PROXY_TEST {
			t.Skip("跳过代理测试，修改常量 ENABLE_PROXY_TEST 为 true 来启用")
			return
		}

		// 创建测试服务器
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message":"success"}`))
		}))
		defer testServer.Close()

		// HTTP代理测试
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc",
			"requestMethod":          "GET",
			"enableProxy":            true,
			"proxyScheme":            "http",
			"proxyHost":              PROXY_HOST,
			"proxyPort":              HTTP_PROXY_PORT,
		}, Registry)
		assert.Nil(t, err)
		defer node1.Destroy()

		// 带认证的HTTP代理测试
		node2, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc",
			"requestMethod":          "GET",
			"enableProxy":            true,
			"proxyScheme":            "http",
			"proxyHost":              PROXY_HOST,
			"proxyPort":              HTTP_PROXY_PORT,
			"proxyUser":              "testuser",
			"proxyPassword":          "testpass",
		}, Registry)
		assert.Nil(t, err)
		defer node2.Destroy()

		// SOCKS5代理测试
		node3, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern": "https://rulego.cc",
			"requestMethod":          "GET",
			"enableProxy":            true,
			"proxyScheme":            "socks5",
			"proxyHost":              PROXY_HOST,
			"proxyPort":              SOCKS5_PROXY_PORT,
		}, Registry)
		assert.Nil(t, err)
		defer node3.Destroy()

		// 系统代理测试
		node4, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"restEndpointUrlPattern":   "https://rulego.cc",
			"requestMethod":            "GET",
			"enableProxy":              true,
			"useSystemProxyProperties": true,
		}, Registry)
		assert.Nil(t, err)
		defer node4.Destroy()

		metaData := types.BuildMetadata(make(map[string]string))
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "TEST",
				Data:       `{"test":"data"}`,
				AfterSleep: time.Millisecond * 100,
			},
		}

		// 注意：这些测试可能会失败，因为代理服务器可能不存在
		// 这里主要测试代理配置是否正确解析和应用
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
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node3,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					assert.Equal(t, types.Success, relationType)
				},
			},
			{
				Node:    node4,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {
					//assert.Equal(t, types.Success, relationType)
				},
			},
		}

		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
		time.Sleep(time.Second * 3)
	})

	// 代理配置验证测试
	t.Run("ProxyConfigurationValidation", func(t *testing.T) {
		// 测试有效的代理配置
		t.Run("ValidProxyConfig", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"restEndpointUrlPattern": "https://httpbin.org/get",
				"requestMethod":          "GET",
				"enableProxy":            true,
				"proxyScheme":            "http",
				"proxyHost":              "proxy.example.com",
				"proxyPort":              8080,
			}, Registry)
			assert.Nil(t, err)
			defer node.Destroy()
		})

		// 测试有效的SOCKS5代理配置
		t.Run("ValidSOCKS5ProxyConfig", func(t *testing.T) {
			node, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
				"restEndpointUrlPattern": "https://httpbin.org/get",
				"requestMethod":          "GET",
				"enableProxy":            true,
				"proxyScheme":            "socks5",
				"proxyHost":              "127.0.0.1",
				"proxyPort":              1080,
				"proxyUser":              "user",
				"proxyPassword":          "pass",
			}, Registry)
			assert.Nil(t, err)
			defer node.Destroy()
		})
	})
}
