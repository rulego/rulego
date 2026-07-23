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
	// A counter used for tallying debug logs
	var debugCount int64

	// A function that records debugging logs
	debugFunc := func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		atomic.AddInt64(&debugCount, 1)
	}
	// Reset the counter
	atomic.StoreInt64(&debugCount, 0)

	// Create an asynchronous DSL configuration
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

	// Create rule engine configurations
	config := rulego.NewConfig(
		types.WithDefaultPool(),
		types.WithEndpointEnabled(true),
		types.WithOnDebug(debugFunc),
	)

	// Create a rule engine
	ruleEngine, err := rulego.New("http_async_debug_test", []byte(asyncDSL), types.WithConfig(config))
	assert.Nil(t, err)
	if ruleEngine == nil {
		t.Fatal("Failure to create a rule engine")
	}
	// Waiting for the service to start
	time.Sleep(time.Second * 2)
	// Release resources
	defer ruleEngine.Stop(context.Background())
	// Test asynchronous request (wait: false)
	t.Run("AsyncRequest", func(t *testing.T) {

		// Send asynchronous requests
		payload := `{"test": "async_data", "id": 1}`
		resp, err := http.Post("http://localhost:9098/api/v1/async", "application/json", strings.NewReader(payload))
		if err != nil {
			t.Logf("Asynchronous request failed: %v", err)
		} else {
			defer resp.Body.Close()
		}

		// Wait for the asynchronous processing to complete
		time.Sleep(time.Second * 1)

		// Check the debugging log
		finalCount := atomic.LoadInt64(&debugCount)

		// Verify whether there is a debug log; one node generates two (In/Out) entries.
		assert.Equal(t, int64(2), finalCount, "异步请求未产生预期的调试日志数量")

	})

	//Test the decoding of RuleChainRunSnapshot
	t.Run("TestOnRuleChainCompleted", func(t *testing.T) {
		ruleEngine.OnMsg(types.NewMsg(0, "TELEMETRY", types.JSON, types.NewMetadata(), "aaa"), types.WithOnRuleChainCompleted(
			func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				_, err := json.Marshal(snapshot)
				assert.Nil(t, err)
			},
		))
	})
}
