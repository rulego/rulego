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
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	_ "github.com/rulego/rulego/components/common"
	_ "github.com/rulego/rulego/components/transform"
	"github.com/rulego/rulego/test/assert"
	"github.com/rulego/rulego/utils/str"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
			assert.Equal(t, fmt.Errorf("SetExecuteNodes node id=%s not found: node not found", "notFound").Error(), err.Error())
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
			assert.Equal(t, fmt.Errorf("SetExecuteNodes node id=%s not found: node not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&count))
	})
	t.Run("WithDebugMode", func(t *testing.T) {
		// ruleChainFile 已设置 debugMode=true，验证 WithDebugMode 的覆盖能力
		ruleEngine, _ := New("TestRuleContext_WithDebugMode", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		var debugCount = int32(0)
		// 链级 debugMode=true，回调正常触发
		ruleEngine.OnMsg(msg, types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&debugCount, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), atomic.LoadInt32(&debugCount))
		atomic.StoreInt32(&debugCount, 0)

		// WithDebugMode(true) 显式启用
		ruleEngine.OnMsg(msg, types.WithDebugMode(true), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&debugCount, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), atomic.LoadInt32(&debugCount))
		atomic.StoreInt32(&debugCount, 0)

		// WithDebugMode(false) 强制关闭 per-message 调试，覆盖链级 debugMode=true
		ruleEngine.OnMsg(msg, types.WithDebugMode(false), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&debugCount, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&debugCount))
	})
	t.Run("WithSkipTellNext", func(t *testing.T) {
		ruleEngine, _ := New("TestRuleContext_WithSkipTellNext", []byte(ruleChainFile), WithConfig(config))
		defer Del(ruleEngine.Id())

		var count = int32(0)
		// WithStartNode("s1") 正常执行 s1 + s2
		ruleEngine.OnMsg(msg, types.WithStartNode("s1"), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(4), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		// WithStartNode("s1") + WithSkipTellNext()：仅执行 s1，不传播到 s2
		ruleEngine.OnMsg(msg, types.WithStartNode("s1"), types.WithSkipTellNext(), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		// 仅 s1 的 IN+OUT = 2 次
		assert.Equal(t, int32(2), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		// WithStartNode("s2") + WithSkipTellNext()：仅执行 s2
		ruleEngine.OnMsg(msg, types.WithStartNode("s2"), types.WithSkipTellNext(), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		// 仅 s2 的 IN+OUT = 2 次
		assert.Equal(t, int32(2), atomic.LoadInt32(&count))
		atomic.StoreInt32(&count, 0)

		// OnMsgAndWait 同样生效
		ruleEngine.OnMsgAndWait(msg, types.WithStartNode("s2"), types.WithSkipTellNext(), types.WithOnNodeDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			atomic.AddInt32(&count, 1)
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(2), atomic.LoadInt32(&count))
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

// waitGroup 等待 WaitGroup 完成，超时则失败，避免测试卡死
func waitGroup(wg *sync.WaitGroup, t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for node execution")
	}
}

// TestTellFailureWritesErrorMsg 验证节点 TellFailure 时把错误信息写入 metadata.errorMsg，
// Failure 分支下游组件可通过 ${metadata.errorMsg} 消费
func TestTellFailureWritesErrorMsg(t *testing.T) {
	var gotErrorMsg string
	var wg sync.WaitGroup
	wg.Add(1)
	action.Functions.Register("testFailErrorMsg26", func(ctx types.RuleContext, msg types.RuleMsg) {
		ctx.TellFailure(msg, errors.New("boom"))
	})
	action.Functions.Register("testCaptureErrorMsg26", func(ctx types.RuleContext, msg types.RuleMsg) {
		gotErrorMsg = msg.Metadata.GetValue(types.KeyErrorMsg)
		wg.Done()
		ctx.TellSuccess(msg)
	})

	chain := `{
		"ruleChain": {
			"id": "test_error_msg_chain_26",
			"name": "errorMsg测试"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{"id": "n1", "type": "functions", "name": "失败节点", "configuration": {"functionName": "testFailErrorMsg26"}},
				{"id": "n2", "type": "functions", "name": "捕获节点", "configuration": {"functionName": "testCaptureErrorMsg26"}}
			],
			"connections": [
				{"fromId": "n1", "toId": "n2", "type": "Failure"}
			]
		}
	}`

	ruleEngine, err := New("test_error_msg_chain_26", []byte(chain))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())

	ruleEngine.OnMsg(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), ""))

	waitGroup(&wg, t)
	assert.Equal(t, "boom", gotErrorMsg)
}

// TestTellSuccessNotWriteErrorMsg 验证 Success 路径不写入 errorMsg
func TestTellSuccessNotWriteErrorMsg(t *testing.T) {
	var gotErrorMsg string
	var wg sync.WaitGroup
	wg.Add(1)
	action.Functions.Register("testSuccessNoErr26", func(ctx types.RuleContext, msg types.RuleMsg) {
		ctx.TellSuccess(msg)
	})
	action.Functions.Register("testCaptureSuccess26", func(ctx types.RuleContext, msg types.RuleMsg) {
		gotErrorMsg = msg.Metadata.GetValue(types.KeyErrorMsg)
		wg.Done()
		ctx.TellSuccess(msg)
	})

	chain := `{
		"ruleChain": {
			"id": "test_no_error_msg_chain_26",
			"name": "无错误测试"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{"id": "n1", "type": "functions", "configuration": {"functionName": "testSuccessNoErr26"}},
				{"id": "n2", "type": "functions", "configuration": {"functionName": "testCaptureSuccess26"}}
			],
			"connections": [
				{"fromId": "n1", "toId": "n2", "type": "Success"}
			]
		}
	}`

	ruleEngine, err := New("test_no_error_msg_chain_26", []byte(chain))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())

	ruleEngine.OnMsg(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), ""))

	waitGroup(&wg, t)
	assert.Equal(t, "", gotErrorMsg)
}

// traversalCount 包级计数：节点实例经 New() 克隆，计数必须落在共享变量上
var traversalCount int64

type traversalNode struct{}

func (n *traversalNode) Type() string { return "test/traversal" }
func (n *traversalNode) New() types.Node {
	return &traversalNode{}
}
func (n *traversalNode) Init(_ types.Config, _ types.Configuration) error { return nil }
func (n *traversalNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	atomic.AddInt64(&traversalCount, 1)
	ctx.TellSuccess(msg)
}
func (n *traversalNode) Destroy() {}

func traversalChainDsl(hops int) []byte {
	dsl := `{"ruleChain":{"id":"traversal_chain"},"metadata":{"nodes":[`
	for i := 0; i < hops; i++ {
		if i > 0 {
			dsl += ","
		}
		dsl += `{"id":"n` + strconv.Itoa(i) + `","type":"test/traversal","name":"n` + strconv.Itoa(i) + `"}`
	}
	dsl += `],"connections":[`
	for i := 0; i < hops-1; i++ {
		if i > 0 {
			dsl += ","
		}
		dsl += `{"fromId":"n` + strconv.Itoa(i) + `","toId":"n` + strconv.Itoa(i+1) + `","type":"Success"}`
	}
	dsl += `]}}`
	return []byte(dsl)
}

// TestLinearChainTraversal 验证线性链在 wait 与 async 两种模式下每个节点恰好执行一次：
// 内联调度（单关系单子节点跳转在当前 goroutine 续跑）不得漏执行或多执行节点
func TestLinearChainTraversal(t *testing.T) {
	if err := Registry.Register(&traversalNode{}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = Registry.Unregister("test/traversal") }()

	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("traversal_chain", traversalChainDsl(10), WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	// wait 模式：OnMsgAndWait 返回后 10 个节点应各执行一次
	atomic.StoreInt64(&traversalCount, 0)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ruleEngine.OnMsgAndWait(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), ""))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("OnMsgAndWait did not return in 5s")
	}
	if got := atomic.LoadInt64(&traversalCount); got != 10 {
		t.Fatalf("wait mode: expected 10 node executions, got %d", got)
	}

	// async 模式：OnEnd 触发时 10 个节点应各执行一次
	atomic.StoreInt64(&traversalCount, 0)
	asyncDone := make(chan struct{})
	ruleEngine.OnMsg(types.NewMsg(0, "test", types.JSON, types.NewMetadata(), ""),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			close(asyncDone)
		}))
	select {
	case <-asyncDone:
	case <-time.After(5 * time.Second):
		t.Fatal("OnEnd did not fire in 5s")
	}
	if got := atomic.LoadInt64(&traversalCount); got != 10 {
		t.Fatalf("async mode: expected 10 node executions, got %d", got)
	}
}

// TestRestoreFromMultipleBranches verifies that we can restore execution from multiple branches
// (C1 and C2) that are children of a Fork node (A), and successfully merge at a Join node (D).
func TestRestoreFromMultipleBranches(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())

	// Read rule chain from file
	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := New("complex_restore_chain", buf, WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")

	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex

	// Use WithRestoreNodes to restore from C1 and C2, specifying A as the common parent (Fork node).
	// This simulates that A has spawned C1 and C2, and now we are resuming them.
	// The engine will create a parent context for A, and child contexts for C1 and C2.
	// When C1 and C2 complete, A will be marked as executed (via childDone),
	// satisfying D's Join condition (which waits for A's completion).
	ruleEngine.OnMsg(msg,
		types.WithRestoreNodes(types.ExecuteNode("node_c1"), types.ExecuteNode("node_c2")),
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			executedNodes[nodeRunLog.Id] = true
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata() == nil {
			t.Error("metadata is nil")
		} else if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		// Verify execution path
		if !executedNodes["node_c1"] {
			t.Error("node_c1 was not executed")
		}
		if !executedNodes["node_c2"] {
			t.Error("node_c2 was not executed")
		}
		if !executedNodes["node_d"] {
			t.Error("node_d was not executed")
		}
		if !executedNodes["node_e"] {
			t.Error("node_e was not executed")
		}
		if !executedNodes["node_f"] {
			t.Error("node_f was not executed")
		}

		// node_b1 and node_b2 should NOT be executed because we restored directly from C1/C2
		if executedNodes["node_b1"] {
			t.Error("node_b1 should not be executed")
		}
		if executedNodes["node_b2"] {
			t.Error("node_b2 should not be executed")
		}

	case <-time.After(time.Second * 6):
		t.Log("Executed nodes:", executedNodes)
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestRestoreFromMultipleBranchesAutoParent verifies automatic parent detection.
func TestRestoreFromMultipleBranchesAutoParent(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())

	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := New("complex_restore_chain_auto", buf, WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex

	// Omit parentNodeId ("") to test auto detection via WithStartNode
	ruleEngine.OnMsg(msg,
		types.WithStartNode("node_c1", "node_c2"),
		types.WithOnNodeCompleted(func(ctx types.RuleContext, nodeRunLog types.RuleNodeRunLog) {
			lock.Lock()
			defer lock.Unlock()
			executedNodes[nodeRunLog.Id] = true
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
			close(done)
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata() == nil {
			t.Error("metadata is nil")
		} else if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		if !executedNodes["node_d"] {
			t.Error("node_d was not executed")
		}
	case <-time.After(time.Second * 6):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestStartNode verifies the basic functionality of WithStartNode (single node start).
// It tests starting from a middle node in a simple chain.
func TestStartNode(t *testing.T) {
	// Re-use the complex chain but we will just test a linear path segment if possible,
	// or we can test starting from 'node_e' which is the end node.

	config := NewConfig(types.WithDefaultPool())
	buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := New("complex_restore_chain_start", buf, WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lastMsg types.RuleMsg
	var lock sync.Mutex
	// Start from node_e (End Node).
	// node_d should NOT be executed.
	// node_f SHOULD be executed (it's after node_e).
	ruleEngine.OnMsg(msg,
		types.WithStartNode("node_e"),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			lock.Lock()
			defer lock.Unlock()
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
			}
			close(done)
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			lock.Lock()
			defer lock.Unlock()
			lastMsg = msg
		}),
	)

	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if lastMsg.GetMetadata().GetValue("f") != "executed" {
			t.Error("node_f was not executed")
		}
		if lastMsg.GetData() != "{\"from\":\"inputMsg\"}" {
			t.Error("Unexpected message data")
		}
		if !executedNodes["node_e"] {
			t.Error("node_e was not executed")
		}
		if !executedNodes["node_f"] {
			t.Error("node_f was not executed")
		}
		if executedNodes["node_d"] {
			t.Error("node_d should not be executed")
		}
	case <-time.After(time.Second * 2):
		t.Fatal("Timeout waiting for execution to complete")
	}
}

// TestRestoreForkNoJoin verifies restoration from multiple branches without a subsequent join node.
// It ensures that independent branches can be restored and execute to completion.
func TestRestoreForkNoJoin(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())

	buf, err := os.ReadFile("../testdata/rule/test_restore_fork_no_join.json")
	if err != nil {
		t.Fatal(err)
	}

	ruleEngine, err := New("fork_no_join_chain", buf, WithConfig(config))
	if err != nil {
		t.Fatal(err)
	}

	msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{\"from\":\"inputMsg\"}")
	done := make(chan struct{})
	var executedNodes = make(map[string]bool)
	var lock sync.Mutex
	var count int32
	// Restore from B1 and B2 (children of A).
	// They should execute independently and then trigger C1 and C2 respectively.
	ruleEngine.OnMsg(msg,
		types.WithRestoreNodes(types.ExecuteNode("node_b1"), types.ExecuteNode("node_b2")),
		types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
			}
			close(done)
		}),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			atomic.AddInt32(&count, 1)
		}),
	)
	select {
	case <-done:
		lock.Lock()
		defer lock.Unlock()
		if atomic.LoadInt32(&count) != 2 {
			t.Error("Unexpected number of end callbacks")
		}
		if !executedNodes["node_b1"] {
			t.Error("node_b1 was not executed")
		}
		if !executedNodes["node_c1"] {
			t.Error("node_c1 was not executed")
		}
		if !executedNodes["node_b2"] {
			t.Error("node_b2 was not executed")
		}
		if !executedNodes["node_c2"] {
			t.Error("node_c2 was not executed")
		}
	case <-time.After(time.Second * 6):
		t.Fatal("Timeout waiting for execution to complete")
	}

}

func TestRestoreWithOptions(t *testing.T) {
	executeNextFunc := func(t *testing.T, useWithTellNext bool) {
		config := NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := New("complex_restore_chain_next", buf, WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)
		// Start from node_a's children via "True" and "False" relations.
		// node_a connects to node_b1 (Success) and node_b2 (Success).
		// So ExecuteNext("node_a", "Success") should trigger both b1 and b2.
		var opts []types.RuleContextOption
		opts = append(opts, types.WithRestoreNodes(
			types.ExecuteNext("node_a", "Success"),
		))
		if useWithTellNext {
			opts = append(opts, types.WithTellNext("node_a", "Success"))
		} else {
			opts = append(opts, types.WithRestoreNodes(
				types.ExecuteNext("node_a", "Success"),
			))
		}
		opts = append(opts, types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
			for _, log := range snapshot.Logs {
				executedNodes[log.Id] = true
				// node_a 只有输出没有输入
				if log.Id == "node_a" {
					if log.InMsg.Id != "" {
						t.Error("node_a should not have an input message")
					}
					if log.OutMsg.Id == "" {
						t.Error("node_a should have an output message")
					}
				}
			}
			close(done)
		}))
		ruleEngine.OnMsg(msg, opts...)

		select {
		case <-done:
			if !executedNodes["node_b1"] {
				t.Error("node_b1 was not executed")
			}
			if !executedNodes["node_b2"] {
				t.Error("node_b2 was not executed")
			}
			if !executedNodes["node_c1"] {
				t.Error("node_c1 was not executed")
			}
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for ExecuteNext test")
		}
	}
	// Test restore from child nodes (ExecuteNext)
	t.Run("ExecuteNext", func(t *testing.T) {
		executeNextFunc(t, false)
	})
	t.Run("TellNext", func(t *testing.T) {
		executeNextFunc(t, true)
	})
	// Test restore from disjoint branches that don't merge immediately but share common ancestor
	t.Run("DisjointBranches", func(t *testing.T) {
		// Using fork_no_join_chain
		config := NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_fork_no_join.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := New("fork_no_join_chain_disjoint", buf, WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}
		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)

		// Restore B1 and B2. Common ancestor is A.
		ruleEngine.OnMsg(msg,
			types.WithRestoreNodes(types.ExecuteNode("node_b1"), types.ExecuteNode("node_b2")),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
				}
				close(done)
			}),
		)
		select {
		case <-done:
			if !executedNodes["node_b1"] || !executedNodes["node_b2"] {
				t.Error("b1/b2 not executed")
			}
		case <-time.After(time.Second * 2):
			t.Fatal("Timeout disjoint")
		}
	})

	// TestRestoreFromPartialBranches verifies restoration from a single branch (C1)
	// when other branches (C2) are NOT restored.
	// In this case, Join node (D) will trigger but will wait for C2's input.
	// Since C2 is not restored, D will eventually TIMEOUT and fail.
	// This test confirms that to successfully complete a Join, you must restore ALL branches
	// that contribute to it, or the Join node must be configured to accept partial results.
	t.Run("PartialRestore", func(t *testing.T) {
		config := NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := New("complex_restore_chain_partial", buf, WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, nil, "{}")
		done := make(chan struct{})
		var executedNodes = make(map[string]bool)

		// Restore ONLY from C1.
		ruleEngine.OnMsg(msg,
			types.WithRestoreNodes(types.ExecuteNode("node_c1")),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
					if log.Err != "" {
						t.Logf("Node %s error: %s", log.Id, log.Err)
					}
				}
				close(done)
			}),
		)

		select {
		case <-done:
			// C1 should be executed
			if !executedNodes["node_c1"] {
				t.Error("node_c1 was not executed")
			}
			// D, E, F should be executed
			// D will wait for C2 until timeout (5s) because C2 is not provided in restore context.
			// After timeout, D fails with "context deadline exceeded" (standard Join node behavior when missing inputs).
			// So E and F are NOT executed.
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}
			if executedNodes["node_e"] {
				t.Error("node_e should not be executed (D failed)")
			}
			if executedNodes["node_f"] {
				t.Error("node_f should not be executed (D failed)")
			}

			// C2, B1, B2 should NOT be executed
			if executedNodes["node_c2"] {
				t.Error("node_c2 should not be executed")
			}
			if executedNodes["node_b1"] {
				t.Error("node_b1 should not be executed")
			}
			if executedNodes["node_b2"] {
				t.Error("node_b2 should not be executed")
			}
		case <-time.After(time.Second * 6):
			t.Fatal("Timeout waiting for PartialRestore test")
		}
	})

	// TestRestoreWithDifferentMessages verifies that we can restore nodes with different messages.
	t.Run("TestRestoreWithDifferentMessages", func(t *testing.T) {
		config := NewConfig(types.WithDefaultPool())
		buf, err := os.ReadFile("../testdata/rule/test_restore_complex.json")
		if err != nil {
			t.Fatal(err)
		}
		ruleEngine, err := New("complex_restore_chain_diff_msg", buf, WithConfig(config))
		if err != nil {
			t.Fatal(err)
		}

		// Define messages for B1 and B2
		msgB1 := types.NewMsg(0, "MSG_B1", types.JSON, nil, `{"source":"b1"}`)
		msgB2 := types.NewMsg(0, "MSG_B2", types.JSON, nil, `{"source":"b2"}`)
		// Default message for the engine (will be used if node request doesn't have one, but here we provide one for all)
		defaultMsg := types.NewMsg(0, "DEFAULT_MSG", types.JSON, nil, "{}")

		done := make(chan struct{})
		var executedNodes = make(map[string]bool)
		var nodeLogs = make(map[string]types.RuleNodeRunLog)

		// Restore B1 and B2 with different messages
		ruleEngine.OnMsg(defaultMsg,
			types.WithRestoreNodes(
				types.ExecuteNodeWithMsg("node_b1", msgB1),
				types.ExecuteNodeWithMsg("node_b2", msgB2),
			),
			types.WithOnRuleChainCompleted(func(ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) {
				for _, log := range snapshot.Logs {
					executedNodes[log.Id] = true
					nodeLogs[log.Id] = log
				}
				close(done)
			}),
		)

		select {
		case <-done:
			if !executedNodes["node_b1"] || !executedNodes["node_b2"] {
				t.Error("b1/b2 not executed")
			}
			// Check if B1 received the correct message
			if log, ok := nodeLogs["node_b1"]; ok {
				if log.InMsg.Type != "MSG_B1" {
					t.Errorf("node_b1 expected MSG_B1, got %s", log.InMsg.Type)
				}
				if log.InMsg.Data.Get() != `{"source":"b1"}` {
					t.Errorf("node_b1 expected data {\"source\":\"b1\"}, got %s", log.InMsg.Data.Get())
				}
			} else {
				t.Error("node_b1 log not found")
			}

			// Check if B2 received the correct message
			if log, ok := nodeLogs["node_b2"]; ok {
				if log.InMsg.Type != "MSG_B2" {
					t.Errorf("node_b2 expected MSG_B2, got %s", log.InMsg.Type)
				}
				if log.InMsg.Data.Get() != `{"source":"b2"}` {
					t.Errorf("node_b2 expected data {\"source\":\"b2\"}, got %s", log.InMsg.Data.Get())
				}
				if log.RelationType != types.Success {
					t.Errorf("node_b2 expected relation type %s, got %s", types.Success, log.RelationType)
				}
			} else {
				t.Error("node_b2 log not found")
			}

			// Verify that Join node (D) executed successfully
			if !executedNodes["node_d"] {
				t.Error("node_d was not executed")
			}

		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for TestRestoreWithDifferentMessages")
		}
	})
}

// TestEndErrLocation 分支终止错误应补充链与节点定位，且不写回 metadata.errorMsg
func TestEndErrLocation(t *testing.T) {
	var ruleChainFile = `{
          "ruleChain": {
            "id": "errLoc",
            "name": "errLoc"
          },
          "metadata": {
            "nodes": [
              {
                "id": "bad",
                "type": "jsTransform",
                "name": "transform",
                "configuration": {
                  "jsScript": "null.x; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
                }
              }
            ],
            "connections": []
          }
        }`
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, err := New("errLoc", []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())

	errCh := make(chan error, 1)
	metaCh := make(chan string, 1)
	ruleEngine.OnMsgAndWait(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}"),
		types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			errCh <- err
			metaCh <- msg.Metadata.GetValue(types.KeyErrorMsg)
		}))

	select {
	case err := <-errCh:
		assert.True(t, strings.Contains(err.Error(), "chain=errLoc node=bad"), "unexpected err: %v", err)
	case <-time.After(time.Second * 5):
		t.Fatal("timeout waiting for OnEnd")
	}
	select {
	case errorMsg := <-metaCh:
		assert.False(t, strings.Contains(errorMsg, "chain="), "metadata.errorMsg should keep the raw component error, got: %s", errorMsg)
		assert.True(t, strings.Contains(errorMsg, "TypeError"), "metadata.errorMsg should keep the raw component error, got: %s", errorMsg)
	case <-time.After(time.Second * 5):
		t.Fatal("timeout waiting for errorMsg metadata")
	}
}

// TestCrossChainDebugInherit 跨链执行节点应继承发起链的调试模式，子链节点产生 IN/OUT 调试事件
func TestCrossChainDebugInherit(t *testing.T) {
	var subChainFile = `{
          "ruleChain": {
            "id": "ccSub",
            "name": "ccSub"
          },
          "metadata": {
            "nodes": [
              {
                "id": "subNode",
                "type": "log",
                "name": "sub",
                "configuration": {
                  "jsScript": "return 'sub out';"
                }
              }
            ],
            "connections": []
          }
        }`
	var mainChainFile = `{
          "ruleChain": {
            "id": "ccMain",
            "name": "ccMain"
          },
          "metadata": {
            "nodes": [
              {
                "id": "m1",
                "type": "functions",
                "name": "cross",
                "configuration": {
                  "functionName": "ccCross"
                }
              }
            ],
            "connections": []
          }
        }`
	action.Functions.Register("ccCross", func(ctx types.RuleContext, msg types.RuleMsg) {
		ctx.TellChainNode(ctx.GetContext(), "ccSub", "subNode", msg, true, func(c types.RuleContext, m types.RuleMsg, e error, rt string) {
			if e != nil {
				ctx.TellFailure(m, e)
			} else {
				ctx.TellNext(m, rt)
			}
		}, nil)
	})

	var subInEvents int32
	config := NewConfig(types.WithOnDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		if ruleChainId == "ccSub" && flowType == types.In {
			atomic.AddInt32(&subInEvents, 1)
		}
	}))
	subEngine, err := New("ccSub", []byte(subChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(subEngine.Id())
	ruleEngine, err := New("ccMain", []byte(mainChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(ruleEngine.Id())

	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")
	// 负对照：子链 DSL 未开 debugMode，非调试消息不产生子链 IN 事件
	ruleEngine.OnMsg(msg)
	time.Sleep(time.Millisecond * 200)
	assert.Equal(t, int32(0), atomic.LoadInt32(&subInEvents))

	// 发起链按消息开调试后，跨链执行的子链节点产生 IN 事件
	ruleEngine.OnMsg(msg, types.WithDebugMode(true))
	time.Sleep(time.Millisecond * 200)
	assert.True(t, atomic.LoadInt32(&subInEvents) > 0, "sub chain IN debug events not found")
}

// outputToTestLogger 收集 Infof 输出，用于断言 log 节点的服务端日志行为
type outputToTestLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *outputToTestLogger) Printf(format string, v ...interface{}) {}
func (l *outputToTestLogger) Debugf(format string, v ...interface{}) {}
func (l *outputToTestLogger) Infof(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, v...))
}
func (l *outputToTestLogger) Warnf(format string, v ...interface{})  {}
func (l *outputToTestLogger) Errorf(format string, v ...interface{}) {}

// TestLogNodeOutputTo log 节点输出位置分支：默认双通道、console 免服务端日志、logger 免调试通道
func TestLogNodeOutputTo(t *testing.T) {
	run := func(outputTo string) (*outputToTestLogger, int32) {
		tl := &outputToTestLogger{}
		var logEvents int32
		config := NewConfig(types.WithLogger(tl), types.WithOnDebug(func(ruleChainId string, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
			if flowType == types.Log {
				atomic.AddInt32(&logEvents, 1)
			}
		}))
		chainId := "logOut" + outputTo
		outputCfg := `{"jsScript":"return 'out-to-test';"`
		if outputTo != "" {
			outputCfg += `,"outputTo":"` + outputTo + `"`
		}
		outputCfg += `}`
		chain := `{"ruleChain":{"id":"` + chainId + `","name":"t"},"metadata":{"nodes":[{"id":"n1","type":"log","name":"l","configuration":` + outputCfg + `}],"connections":[]}}`
		ruleEngine, err := New(chainId, []byte(chain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)
		ruleEngine.OnMsgAndWait(types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}"))
		// OnDebug 走异步任务，等待落盘
		time.Sleep(time.Millisecond * 200)
		return tl, atomic.LoadInt32(&logEvents)
	}

	t.Run("DefaultBoth", func(t *testing.T) {
		tl, logEvents := run("")
		assert.Equal(t, 1, len(tl.lines))
		assert.True(t, strings.Contains(tl.lines[0], "[chain=logOut node=n1] out-to-test"), "unexpected: %v", tl.lines)
		assert.True(t, logEvents >= 1)
	})
	t.Run("ConsoleOnly", func(t *testing.T) {
		tl, logEvents := run("console")
		assert.Equal(t, 0, len(tl.lines))
		assert.True(t, logEvents >= 1)
	})
	t.Run("LoggerOnly", func(t *testing.T) {
		tl, logEvents := run("logger")
		assert.Equal(t, 1, len(tl.lines))
		assert.Equal(t, int32(0), logEvents)
	})
	t.Run("UpperCaseNormalized", func(t *testing.T) {
		tl, logEvents := run("Console")
		assert.Equal(t, 0, len(tl.lines))
		assert.True(t, logEvents >= 1)
	})
}
