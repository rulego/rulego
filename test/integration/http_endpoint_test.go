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

package integration

import (
	"context"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego"
	"github.com/rulego/rulego/api/types"
)

func TestHttpAsyncDebugLog(t *testing.T) {
	// 用于统计调试日志的计数器
	var debugCount int64

	// 记录调试日志的函数
	debugFunc := func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt64(&debugCount, 1)
	}
	// 重置计数器
	atomic.StoreInt64(&debugCount, 0)

	// 创建异步DSL配置
	asyncDSL := `{
			"ruleChain": {
				"id": "http_async_debug_test",
				"name": "异步调试测试链",
				"root": true,
				"debugMode": true
			},
			"metadata": {
				"endpoints": [
					{
						"id": "fasthttp_async_endpoint",
						"type": "endpoint/http",
						"name": "FastHttp异步服务器",
						"configuration": {
							"server": ":9098",
							"allowCors": true
						},
						"routers": [
							{
								"id": "async_router",
								"params": ["POST"],
								"from": {
									"path": "/api/v1/async"
								},
								"to": {
									"path": "http_async_debug_test:async_processor",
									"wait": false
								}
							}
						]
					}
				],
				"nodes": [
					{
						"id": "async_processor",
						"type": "jsTransform",
						"name": "异步处理器",
						"configuration": {
							"jsScript": "var result = {\n  message: '异步处理完成',\n  timestamp: new Date().toISOString(),\n  inputData: JSON.parse(msg)\n};\nreturn {'msg': result, 'metadata': metadata, 'msgType': msgType};"
						},
						"debugMode": true
					}
				],
				"connections": []
			}
		}`

	// 创建规则引擎配置
	config := rulego.NewConfig(
		types.WithDefaultPool(),
		types.WithEndpointEnabled(true),
		types.WithOnDebug(debugFunc),
	)

	// 创建规则引擎
	ruleEngine, err := rulego.New("http_async_debug_test", []byte(asyncDSL), types.WithConfig(config))
	assert.Nil(t, err)
	if ruleEngine == nil {
		t.Fatal("创建规则引擎失败")
	}
	// 等待服务启动
	time.Sleep(time.Second * 2)
	// 清理资源
	defer ruleEngine.Stop(context.Background())
	// 测试异步请求（wait: false）
	t.Run("AsyncRequest", func(t *testing.T) {

		// 发送异步请求
		payload := `{"test": "async_data", "id": 1}`
		resp, err := http.Post("http://localhost:9098/api/v1/async", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Logf("异步请求失败: %v", err)
		} else {
			defer resp.Body.Close()
		}

		// 等待异步处理完成
		time.Sleep(time.Second * 1)

		// 检查调试日志
		finalCount := atomic.LoadInt64(&debugCount)

		// 验证是否有调试日志，1个node会产生两条（In/Out）
		assert.Equal(t, int64(2), finalCount, "异步请求未产生预期的调试日志数量")

	})

	//测试RuleChainRunSnapshot解码
	t.Run("TestOnRuleChainCompleted", func(t *testing.T) {
		ruleEngine.OnMsg(types.NewMsg(0, "TELEMETRY", types.JSON, types.NewMetadata(), "aaa"), types.WithOnRuleChainCompleted(
			func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				_, err := json.Marshal(snapshot)
				assert.Nil(t, err)
			},
		))
	})
}
