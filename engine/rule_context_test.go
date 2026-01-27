/*
 * Copyright 2024 The RuleGo Authors.
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

package engine

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
)

// TestCache 测试缓存
func TestCache(t *testing.T) {
	config := NewConfig()
	ruleEngine, err := New(str.RandomStr(10), []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())
	rootCtx := ruleEngine.RootRuleContext().(*DefaultRuleContext)
	rootCtxCopy := NewRuleContext(rootCtx.GetContext(), rootCtx.config, rootCtx.ruleChainCtx, rootCtx.from, rootCtx.self, rootCtx.pool, rootCtx.onEnd, DefaultPool)

	globalCache := rootCtxCopy.GlobalCache()
	assert.NotNil(t, globalCache)
	chainCache := rootCtxCopy.ChainCache()
	assert.NotNil(t, globalCache)

	t.Run("GlobalCache", func(t *testing.T) {
		t.Run("SetAndGet", func(t *testing.T) {
			err := globalCache.Set("key1", "value1", "1m")
			assert.Nil(t, err)
			val, err := globalCache.Get("key1")
			assert.Nil(t, err)
			assert.Equal(t, "value1", val)
		})

		t.Run("Delete", func(t *testing.T) {
			globalCache.Set("key2", "value2", "1m")
			assert.Nil(t, globalCache.Delete("key2"))
			val, _ := globalCache.Get("key2")
			assert.Nil(t, val)
		})
	})

	t.Run("ChainCache", func(t *testing.T) {
		t.Run("Isolation", func(t *testing.T) {
			chainCache.Set("key1", "value1", "1m")
			val, err := chainCache.Get("key1")
			assert.Nil(t, err)
			assert.Equal(t, "value1", val)
		})

		t.Run("Has", func(t *testing.T) {
			chainCache.Set("key2", "value2", "1m")
			assert.True(t, chainCache.Has("key2"))
			assert.False(t, chainCache.Has("key3"))
		})
	})

	t.Run("SameKeyIsolation", func(t *testing.T) {
		// 设置相同key到两个缓存
		globalCache.Set("same_key", "global_value", "1m")
		chainCache.Set("same_key", "chain_value", "1m")

		// 验证两个缓存的值互不影响
		val, err := globalCache.Get("same_key")
		assert.Nil(t, err)
		assert.Equal(t, "global_value", val)
		val, err = chainCache.Get("same_key")
		assert.Nil(t, err)
		assert.Equal(t, "chain_value", val)

		// 删除一个缓存的值，另一个不受影响
		globalCache.Delete("same_key")
		val, _ = globalCache.Get("same_key")
		assert.Nil(t, val)
		val, err = chainCache.Get("same_key")
		assert.Nil(t, err)
		assert.Equal(t, "chain_value", val)
	})
}

// TestEndNodeBehavior 测试结束节点的行为
func TestEndNodeBehavior(t *testing.T) {
	config := NewConfig()
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	// 测试配置了结束节点的规则链，OnEnd只会调用一次
	t.Run("WithEndNode", func(t *testing.T) {
		// 创建包含结束节点的规则链
		ruleChainWithEndNode := `{
			"ruleChain": {
				"id": "test_with_end_node",
				"name": "testRuleChainWithEndNode",
				"debugMode": true,
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "jsFilter",
						"name": "过滤",
						"configuration": {
							"jsScript": "return msg.temperature>10;"
						}
					},
					{
						"id": "s2",
						"type": "jsTransform",
						"name": "转换1",
						"configuration": {
							"jsScript": "msgType='TRANSFORM1';return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "s3",
						"type": "jsTransform",
						"name": "转换2",
						"configuration": {
							"jsScript": "msgType='TRANSFORM2';return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "end1",
						"type": "end",
						"name": "结束节点"
					}
				],
				"connections": [
					{
						"fromId": "s1",
						"toId": "s2",
						"type": "True"
					},
					{
						"fromId": "s1",
						"toId": "s3",
						"type": "True"
					},
					{
						"fromId": "s2",
						"toId": "end1",
						"type": "Success"
					}
				]
			}
		}`

		ruleEngine, err := New("TestEndNodeBehavior_WithEndNode", []byte(ruleChainWithEndNode), WithConfig(config))
		assert.Nil(t, err)
		defer Del(ruleEngine.Id())

		var onEndCallCount int32
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&onEndCallCount, 1)
			assert.Equal(t, types.Success, relationType)
		}))

		time.Sleep(time.Millisecond * 200)
		// 配置了结束节点，即使有多个分支，OnEnd也只会调用一次
		assert.Equal(t, int32(1), atomic.LoadInt32(&onEndCallCount))
	})

	// 测试没有配置结束节点的规则链，OnEnd会调用多次
	t.Run("WithoutEndNode", func(t *testing.T) {
		// 创建不包含结束节点的规则链
		ruleChainWithoutEndNode := `{
			"ruleChain": {
				"id": "test_without_end_node",
				"name": "testRuleChainWithoutEndNode",
				"debugMode": true,
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "jsFilter",
						"name": "过滤",
						"configuration": {
							"jsScript": "return msg.temperature>10;"
						}
					},
					{
						"id": "s2",
						"type": "jsTransform",
						"name": "转换1",
						"configuration": {
							"jsScript": "msgType='TRANSFORM1';return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					},
					{
						"id": "s3",
						"type": "jsTransform",
						"name": "转换2",
						"configuration": {
							"jsScript": "msgType='TRANSFORM2';return {'msg':msg,'metadata':metadata,'msgType':msgType};"
						}
					}
				],
				"connections": [
					{
						"fromId": "s1",
						"toId": "s2",
						"type": "True"
					},
					{
						"fromId": "s1",
						"toId": "s3",
						"type": "True"
					}
				]
			}
		}`

		ruleEngine, err := New("TestEndNodeBehavior_WithoutEndNode", []byte(ruleChainWithoutEndNode), WithConfig(config))
		assert.Nil(t, err)
		defer Del(ruleEngine.Id())

		var onEndCallCount int32
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&onEndCallCount, 1)
			assert.Equal(t, types.Success, relationType)
		}))

		time.Sleep(time.Millisecond * 200)
		// 没有配置结束节点，每个分支结束时都会调用OnEnd，所以会调用2次
		assert.Equal(t, int32(2), atomic.LoadInt32(&onEndCallCount))
	})
}

func TestOnEndWithFailure(t *testing.T) {
	// 包含结束节点的规则链
	ruleChainWithEndNode := `{
		"ruleChain": {
			"id": "test_on_end_failure",
			"name": "TestOnEndWithFailure",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "s1",
					"type": "jsFilter",
					"name": "Filter",
					"configuration": {
						"jsScript": "return msg.temperature > 50;"
					}
				},
				{
					"id": "end1",
					"type": "end",
					"name": "End Node"
				}
			],
			"connections": [
				{
					"fromId": "s1",
					"toId": "end1",
					"type": "True"
				}
			]
		}
	}`

	msg := types.NewMsg(0, "TELEMETRY", types.JSON, nil, `{"temperature":10}`) // < 50, so filter returns False. Connection is True. So it will be Failure/False?

	// Case 1: OnEndWithFailure = true (Default)
	t.Run("DefaultTrue", func(t *testing.T) {
		config := NewConfig()
		// Make script fail
		ruleChainWithScriptError := strings.Replace(ruleChainWithEndNode, "return msg.temperature > 50;", "throw 'error';", 1)

		ruleEngine, err := New("TestOnEndWithFailure_True", []byte(ruleChainWithScriptError), WithConfig(config))
		assert.Nil(t, err)
		defer Del(ruleEngine.Id())

		var onEndCalled int32
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.StoreInt32(&onEndCalled, 1)
			assert.Equal(t, types.Failure, relationType)
		}))

		time.Sleep(time.Millisecond * 200)
		assert.Equal(t, int32(1), atomic.LoadInt32(&onEndCalled))
	})

	// Case 2: OnEndWithFailure = false
	t.Run("SetFalse", func(t *testing.T) {
		config := NewConfig(types.WithOnEndWithFailure(false))
		// Make script fail
		ruleChainWithScriptError := strings.Replace(ruleChainWithEndNode, "return msg.temperature > 50;", "throw 'error';", 1)

		ruleEngine, err := New("TestOnEndWithFailure_False", []byte(ruleChainWithScriptError), WithConfig(config))
		assert.Nil(t, err)
		defer Del(ruleEngine.Id())

		var onEndCalled int32
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.StoreInt32(&onEndCalled, 1)
		}))

		time.Sleep(time.Millisecond * 200)
		// Should NOT be called because it's Failure, and we have an End node (so normally only End node triggers), and we disabled OnEndWithFailure.
		assert.Equal(t, int32(0), atomic.LoadInt32(&onEndCalled))
	})
}

func TestRuleContext(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	t.Run("hasOnEnd", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_hasOnEnd", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {

		}, nil)
		assert.Nil(t, ctx.From())
		assert.True(t, reflect.DeepEqual(ctx.Config().EndpointEnabled, config.EndpointEnabled))
		ctx.SetRuleChainPool(DefaultPool)
		assert.Equal(t, ctx.ruleChainPool, DefaultPool)

		assert.NotNil(t, ctx.GetEndFunc())

		ruleEngine.OnMsg(msg)
		err := ruleEngine.ReloadChild("s1", []byte(""))
		assert.NotNil(t, err)
		err = ruleEngine.ReloadChild("", []byte("{"))
		assert.NotNil(t, err)

		ruleEngine.Stop(context.Background())

		err = ruleEngine.ReloadChild("", []byte("{"))
		assert.Equal(t, "engine is shutting down", err.Error())
		time.Sleep(time.Millisecond * 100)
	})
	t.Run("notEnd", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_notEnd", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, nil, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("doOnEnd", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_doOnEnd", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("notSelf", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_notSelf", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}, nil)
		ctx.tellSelf(msg, nil, types.Success)
	})
	t.Run("notRuleChainCtx", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, nil, nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, "", relationType)
		}, nil)
		_, ok := ctx.getNextNodes(types.Success)
		assert.False(t, ok)
	})

	t.Run("tellSelf", func(t *testing.T) {
		selfDefinition := types.RuleNode{
			Id:            "s1",
			Type:          "log",
			Configuration: map[string]interface{}{"Add": "add"},
		}
		nodeCtx, _ := InitRuleNodeCtx(NewConfig(), nil, nil, &selfDefinition)
		ruleEngine2, _ := New("TestRuleContextTellSelf", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine2.Id())

		ctx := NewRuleContext(context.Background(), config, ruleEngine2.RootRuleChainCtx().(*RuleChainCtx), nil, nodeCtx, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			//assert.Equal(t, "", relationType)
		}, nil)

		ctx.TellSelf(msg, 1000)
		ctx.tellSelf(msg, nil, types.Success)
	})
	t.Run("WithStartNode", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_WithStartNode", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		var count = int32(0)
		ruleEngine.OnMsg(msg, types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsgAndWait(msg, types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("s2"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(2), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("notFound"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, fmt.Errorf("SetExecuteNodes node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&count))
	})
	t.Run("WithTellNext", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_WithTellNext", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		var count = int32(0)
		ruleEngine.OnMsg(msg, types.WithTellNext("s1", types.True), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(3), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithTellNext("s2", types.Success), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(1), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsgAndWait(msg, types.WithTellNext("s2", types.Success), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(1), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		ruleEngine.OnMsg(msg, types.WithStartNode("notFound"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, fmt.Errorf("SetExecuteNodes node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&count))
	})
	t.Run("ContextCancellation", func(t *testing.T) {
		// 测试 OnMsg context 取消
		ruleChainWithFunctions := `{
			"ruleChain": {
				"id": "test_context_cancellation",
				"name": "testRuleChainContextCancellation",
				"debugMode": true,
				"root": true
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "测试函数节点",
						"configuration": {
							"functionName": "testContextCancellation"
						}
					}
				],
				"connections": []
			}
		}`

		// 注册测试函数
		var contextCancelled int32
		var functionExecuted int32
		action.Functions.Register("testContextCancellation", func(ctx types.RuleContext, msg types.RuleMsg) {
			atomic.StoreInt32(&functionExecuted, 1)
			// 模拟一些处理时间，在处理过程中检查 context 是否被取消
			// 由于新的实现不使用goroutine，我们需要轮询检查Err()方法
			done := make(chan struct{})
			go func() {
				time.Sleep(time.Millisecond * 50)
				close(done)
			}()

			// 检查 context 是否被取消（使用Done() channel监听）
			if ctx.GetContext() != nil {
				select {
				case <-done:
					// 处理完成，context 正常
					ctx.TellSuccess(msg)
					return
				case <-ctx.GetContext().Done():
					// 收到取消信号
					atomic.StoreInt32(&contextCancelled, 1)
					ctx.TellFailure(msg, ctx.GetContext().Err())
					return
				}
			}
			// 如果没有 context，直接成功
			<-done
			ctx.TellSuccess(msg)
		})
		defer action.Functions.UnRegister("testContextCancellation")

		ruleEngine, err := New("TestRuleContext_ContextCancellation", []byte(ruleChainWithFunctions), WithConfig(config))
		assert.Nil(t, err)
		defer Del(ruleEngine.Id())

		// 创建一个可取消的 context
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 启动一个 goroutine，在函数执行过程中取消 context
		go func() {
			time.Sleep(time.Millisecond * 10) // 等待函数开始执行
			cancel()                          // 取消 context
		}()

		var onEndCalled int32
		var endError error
		var endErrorMu sync.Mutex
		ruleEngine.OnMsg(msg, types.WithContext(ctx), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.StoreInt32(&onEndCalled, 1)
			endErrorMu.Lock()
			endError = err
			endErrorMu.Unlock()
		}))

		// 等待处理完成
		time.Sleep(time.Millisecond * 200)

		// 验证函数被执行了
		assert.Equal(t, int32(1), atomic.LoadInt32(&functionExecuted), "函数应该被执行")

		// 验证收到了 context 取消信号
		assert.Equal(t, int32(1), atomic.LoadInt32(&contextCancelled), "应该检测到 context 取消")

		// 验证 OnEnd 被调用
		assert.Equal(t, int32(1), atomic.LoadInt32(&onEndCalled), "OnEnd 应该被调用")

		// 验证错误信息包含取消信息
		endErrorMu.Lock()
		errValue := endError
		endErrorMu.Unlock()
		assert.NotNil(t, errValue, "应该返回错误")
		assert.True(t, strings.Contains(errValue.Error(), "canceled"), "错误信息应该包含 canceled")
	})

}
