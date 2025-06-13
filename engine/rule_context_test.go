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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
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
			assert.Equal(t, "value1", globalCache.Get("key1"))
		})

		t.Run("Delete", func(t *testing.T) {
			globalCache.Set("key2", "value2", "1m")
			assert.Nil(t, globalCache.Delete("key2"))
			assert.Nil(t, globalCache.Get("key2"))
		})
	})

	t.Run("ChainCache", func(t *testing.T) {
		t.Run("Isolation", func(t *testing.T) {
			chainCache.Set("key1", "value1", "1m")
			assert.Equal(t, "value1", chainCache.Get("key1"))
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
		assert.Equal(t, "global_value", globalCache.Get("same_key"))
		assert.Equal(t, "chain_value", chainCache.Get("same_key"))

		// 删除一个缓存的值，另一个不受影响
		globalCache.Delete("same_key")
		assert.Nil(t, globalCache.Get("same_key"))
		assert.Equal(t, "chain_value", chainCache.Get("same_key"))
	})
}

// TestRuleContextPool 测试RuleContext对象池的回收和隔离机制
func TestRuleContextPool(t *testing.T) {
	t.Run("SimpleChain", func(t *testing.T) {
		testRuleContextPoolSimpleChain(t)
	})

	t.Run("ForkChain", func(t *testing.T) {
		testRuleContextPoolForkChain(t)
	})
}

// testRuleContextPoolSimpleChain 测试简单链式场景下的RuleContext池
func testRuleContextPoolSimpleChain(t *testing.T) {
	config := NewConfig()

	// 用于记录上下文隔离错误
	var contextIsolationErrors int32
	var processedMessages int32

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		// 在节点执行时检查上下文隔离
		if flowType == types.In && nodeId == "node1" {
			atomic.AddInt32(&processedMessages, 1)
		}
	}

	// 创建一个简单的规则链用于测试
	testRuleChain := `{
		"ruleChain": {
			"id": "test_pool",
			"name": "testContextPool",
			"debugMode": true,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "node1",
					"type": "jsTransform",
					"name": "transform1",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata.contextID = $ctx.GetSelfId() + '_' + Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "node2",
					"type": "jsTransform",
					"name": "transform2",
					"debugMode": true,
					"configuration": {
						"jsScript": "return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "node1",
					"toId": "node2",
					"type": "Success"
				}
			]
		}
	}`

	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(testRuleChain), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 并发测试，验证上下文池的正确性
	var wg sync.WaitGroup
	maxGoroutines := 100
	messagesPerGoroutine := 10

	wg.Add(maxGoroutines)

	for i := 0; i < maxGoroutines; i++ {
		go func(goroutineIndex int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				metaData := types.NewMetadata()
				metaData.PutValue("goroutineID", fmt.Sprintf("goroutine_%d", goroutineIndex))
				metaData.PutValue("messageIndex", fmt.Sprintf("%d", j))

				msg := types.NewMsg(0, "TEST_MSG_TYPE", types.JSON, metaData, "{\"test\":\"data\"}")

				// 使用独立的上下文值来验证隔离性
				ctxValue := fmt.Sprintf("ctx_value_%d_%d", goroutineIndex, j)
				ctx := context.WithValue(context.Background(), "testKey", ctxValue)

				var msgWg sync.WaitGroup
				msgWg.Add(1)

				ruleEngine.OnMsg(msg, types.WithContext(ctx), types.WithEndFunc(func(ruleCtx types.RuleContext, resultMsg types.RuleMsg, err error) {
					defer msgWg.Done()

					// 验证上下文值是否正确传递
					if ruleCtx.GetContext().Value("testKey") != ctxValue {
						atomic.AddInt32(&contextIsolationErrors, 1)
						t.Errorf("Context isolation failed: expected %s, got %v", ctxValue, ruleCtx.GetContext().Value("testKey"))
					}

					// 验证消息元数据是否正确
					if resultMsg.Metadata.GetValue("goroutineID") != fmt.Sprintf("goroutine_%d", goroutineIndex) {
						atomic.AddInt32(&contextIsolationErrors, 1)
						t.Errorf("Message metadata isolation failed")
					}

					assert.Nil(t, err)
				}))

				msgWg.Wait()

				// 添加小延迟以增加上下文重用的可能性
				time.Sleep(time.Microsecond * 10)
			}
		}(i)
	}

	wg.Wait()

	// 等待所有异步操作完成
	time.Sleep(time.Millisecond * 500)

	// 验证测试结果
	// 验证没有上下文隔离错误
	assert.Equal(t, int32(0), atomic.LoadInt32(&contextIsolationErrors), "No context isolation errors should occur")

	// 验证所有消息都被处理
	totalMessages := int32(maxGoroutines * messagesPerGoroutine)
	assert.Equal(t, totalMessages, atomic.LoadInt32(&processedMessages), "All messages should be processed")

	// 测试对象池的内存回收
	var poolTestContexts []*DefaultRuleContext
	for i := 0; i < 10; i++ {
		ctx := defaultContextPool.Get().(*DefaultRuleContext)
		poolTestContexts = append(poolTestContexts, ctx)
	}

	// 将上下文放回池中
	for _, ctx := range poolTestContexts {
		defaultContextPool.Put(ctx)
	}

	// 再次获取，应该能重用之前的上下文
	for i := 0; i < 5; i++ {
		ctx := defaultContextPool.Get().(*DefaultRuleContext)
		// 验证上下文的关键字段被正确重置
		assert.NotNil(t, ctx, "Context should not be nil")
		defaultContextPool.Put(ctx)
	}
}

// testRuleContextPoolForkChain 测试分叉链场景下的RuleContext池和多协程数据隔离
func testRuleContextPoolForkChain(t *testing.T) {
	config := NewConfig()

	// 用于记录上下文隔离错误和节点执行计数
	var contextIsolationErrors int32
	var node1ProcessedMessages int32
	var node2ProcessedMessages int32
	var node3ProcessedMessages int32
	var node4ProcessedMessages int32

	config.OnDebug = func(chainId, flowType string, nodeId string, msg types.RuleMsg, relationType string, err error) {
		// 统计各节点的执行次数
		switch nodeId {
		case "node1":
			if flowType == types.In {
				atomic.AddInt32(&node1ProcessedMessages, 1)
			}
		case "node2":
			if flowType == types.In {
				atomic.AddInt32(&node2ProcessedMessages, 1)
			}
		case "node3":
			if flowType == types.In {
				atomic.AddInt32(&node3ProcessedMessages, 1)
			}
		case "node4":
			if flowType == types.In {
				atomic.AddInt32(&node4ProcessedMessages, 1)
			}
		}
	}

	// 创建一个分叉规则链用于测试 - 一个节点分叉到多个并发节点
	forkRuleChain := `{
		"ruleChain": {
			"id": "test_fork_pool",
			"name": "testForkContextPool",
			"debugMode": true,
			"root": true
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "node1",
					"type": "jsTransform",
					"name": "forkNode",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata.forkContextID = $ctx.GetSelfId() + '_' + Date.now(); metadata.forkTime = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "node2",
					"type": "jsTransform",
					"name": "branch1",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata.branch1ContextID = $ctx.GetSelfId() + '_' + Date.now(); metadata.branch1Time = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "node3",
					"type": "jsTransform",
					"name": "branch2",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata.branch2ContextID = $ctx.GetSelfId() + '_' + Date.now(); metadata.branch2Time = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				},
				{
					"id": "node4",
					"type": "jsTransform",
					"name": "branch3",
					"debugMode": true,
					"configuration": {
						"jsScript": "metadata.branch3ContextID = $ctx.GetSelfId() + '_' + Date.now(); metadata.branch3Time = Date.now(); return {'msg':msg,'metadata':metadata,'msgType':msgType};"
					}
				}
			],
			"connections": [
				{
					"fromId": "node1",
					"toId": "node2",
					"type": "Success"
				},
				{
					"fromId": "node1",
					"toId": "node3",
					"type": "Success"
				},
				{
					"fromId": "node1",
					"toId": "node4",
					"type": "Success"
				}
			]
		}
	}`

	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(forkRuleChain), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 并发测试，验证分叉场景下的上下文池正确性
	var wg sync.WaitGroup
	maxGoroutines := 50
	messagesPerGoroutine := 5

	// 用于收集所有分支的结果
	type branchResult struct {
		goroutineIndex int
		messageIndex   int
		ctxValue       string
		branchResults  []types.RuleMsg
		mu             sync.Mutex
	}

	results := make([]*branchResult, maxGoroutines*messagesPerGoroutine)
	resultIndex := int32(0)

	wg.Add(maxGoroutines)

	for i := 0; i < maxGoroutines; i++ {
		go func(goroutineIndex int) {
			defer wg.Done()

			for j := 0; j < messagesPerGoroutine; j++ {
				metaData := types.NewMetadata()
				metaData.PutValue("goroutineID", fmt.Sprintf("goroutine_%d", goroutineIndex))
				metaData.PutValue("messageIndex", fmt.Sprintf("%d", j))
				metaData.PutValue("originalTime", fmt.Sprintf("%d", time.Now().UnixNano()))

				msg := types.NewMsg(0, "TEST_FORK_MSG_TYPE", types.JSON, metaData, "{\"test\":\"fork_data\"}")

				// 使用独立的上下文值来验证隔离性
				ctxValue := fmt.Sprintf("fork_ctx_value_%d_%d", goroutineIndex, j)
				ctx := context.WithValue(context.Background(), "testKey", ctxValue)

				// 创建结果收集器
				idx := atomic.AddInt32(&resultIndex, 1) - 1
				results[idx] = &branchResult{
					goroutineIndex: goroutineIndex,
					messageIndex:   j,
					ctxValue:       ctxValue,
					branchResults:  make([]types.RuleMsg, 0, 3), // 预期3个分支结果
				}

				var msgWg sync.WaitGroup
				msgWg.Add(3) // 等待3个分支的OnEnd回调

				ruleEngine.OnMsg(msg, types.WithContext(ctx), types.WithEndFunc(func(ruleCtx types.RuleContext, resultMsg types.RuleMsg, err error) {
					defer msgWg.Done()

					// 验证上下文值是否正确传递
					if ruleCtx.GetContext().Value("testKey") != ctxValue {
						atomic.AddInt32(&contextIsolationErrors, 1)
						t.Errorf("Fork context isolation failed: expected %s, got %v", ctxValue, ruleCtx.GetContext().Value("testKey"))
					}

					// 验证消息元数据是否正确
					if resultMsg.Metadata.GetValue("goroutineID") != fmt.Sprintf("goroutine_%d", goroutineIndex) {
						atomic.AddInt32(&contextIsolationErrors, 1)
						t.Errorf("Fork message metadata isolation failed")
					}

					// 收集分支结果
					results[idx].mu.Lock()
					results[idx].branchResults = append(results[idx].branchResults, resultMsg)
					results[idx].mu.Unlock()

					assert.Nil(t, err)
				}))

				msgWg.Wait()

				// 添加小延迟以增加上下文重用的可能性
				time.Sleep(time.Microsecond * 50)
			}
		}(i)
	}

	wg.Wait()

	// 等待所有异步操作完成
	time.Sleep(time.Millisecond * 1000)

	// 验证测试结果
	// 验证没有上下文隔离错误
	assert.Equal(t, int32(0), atomic.LoadInt32(&contextIsolationErrors), "No fork context isolation errors should occur")

	// 验证所有消息都被处理
	totalMessages := int32(maxGoroutines * messagesPerGoroutine)
	assert.Equal(t, totalMessages, atomic.LoadInt32(&node1ProcessedMessages), "All messages should be processed by node1")

	// 由于分叉，每个分支节点应该处理相同数量的消息
	assert.Equal(t, totalMessages, atomic.LoadInt32(&node2ProcessedMessages), "All messages should be processed by node2")
	assert.Equal(t, totalMessages, atomic.LoadInt32(&node3ProcessedMessages), "All messages should be processed by node3")
	assert.Equal(t, totalMessages, atomic.LoadInt32(&node4ProcessedMessages), "All messages should be processed by node4")

	// 验证分叉结果的数据隔离
	for i, result := range results {
		if result == nil {
			continue
		}
		result.mu.Lock()
		// 每个消息应该产生3个分支结果
		assert.Equal(t, 3, len(result.branchResults), fmt.Sprintf("Result %d should have 3 branch results", i))

		// 验证每个分支结果的数据隔离
		for _, branchMsg := range result.branchResults {
			// 验证原始数据保持一致
			assert.Equal(t, fmt.Sprintf("goroutine_%d", result.goroutineIndex), branchMsg.Metadata.GetValue("goroutineID"))
			assert.Equal(t, fmt.Sprintf("%d", result.messageIndex), branchMsg.Metadata.GetValue("messageIndex"))

			// 验证分支特有的上下文ID存在且不为空
			forkContextID := branchMsg.Metadata.GetValue("forkContextID")
			assert.True(t, forkContextID != "", "Fork context ID should not be empty")

			// 验证至少有一个分支上下文ID存在
			branch1ID := branchMsg.Metadata.GetValue("branch1ContextID")
			branch2ID := branchMsg.Metadata.GetValue("branch2ContextID")
			branch3ID := branchMsg.Metadata.GetValue("branch3ContextID")
			assert.True(t, branch1ID != "" || branch2ID != "" || branch3ID != "", "At least one branch context ID should exist")
		}
		result.mu.Unlock()
	}

	// 测试对象池的内存回收（分叉场景下会创建更多上下文）
	var poolTestContexts []*DefaultRuleContext
	for i := 0; i < 20; i++ {
		ctx := defaultContextPool.Get().(*DefaultRuleContext)
		poolTestContexts = append(poolTestContexts, ctx)
	}

	// 将上下文放回池中
	for _, ctx := range poolTestContexts {
		defaultContextPool.Put(ctx)
	}

	// 再次获取，应该能重用之前的上下文
	for i := 0; i < 10; i++ {
		ctx := defaultContextPool.Get().(*DefaultRuleContext)
		// 验证上下文的关键字段被正确重置
		assert.NotNil(t, ctx, "Fork context should not be nil")
		defaultContextPool.Put(ctx)
	}
}

func TestRuleContext(t *testing.T) {
	config := NewConfig(types.WithDefaultPool())
	ruleEngine, _ := New("TestRuleContext", []byte(ruleChainFile), WithConfig(config))
	metaData := types.NewMetadata()
	metaData.PutValue("productType", "test01")
	msg := types.NewMsg(0, "TEST_MSG_TYPE1", types.JSON, metaData, "{\"temperature\":41,\"humidity\":90}")

	t.Run("hasOnEnd", func(t *testing.T) {
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

		ruleEngine.Stop()

		err = ruleEngine.ReloadChild("", []byte("{"))
		assert.Equal(t, "unexpected end of JSON input", err.Error())
		time.Sleep(time.Millisecond * 100)
	})
	t.Run("notEnd", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, nil, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("doOnEnd", func(t *testing.T) {
		ctx := NewRuleContext(context.Background(), config, ruleEngine.RootRuleChainCtx().(*RuleChainCtx), nil, nil, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			assert.Equal(t, types.Success, relationType)
		}, nil)
		ctx.DoOnEnd(msg, nil, types.Success)
	})
	t.Run("notSelf", func(t *testing.T) {
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

		ctx := NewRuleContext(context.Background(), config, ruleEngine2.RootRuleChainCtx().(*RuleChainCtx), nil, nodeCtx, nil, func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			//assert.Equal(t, "", relationType)
		}, nil)

		ctx.TellSelf(msg, 1000)
		ctx.tellSelf(msg, nil, types.Success)
	})
	t.Run("WithStartNode", func(t *testing.T) {
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
			assert.Equal(t, fmt.Errorf("SetExecuteNode node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&count))
	})
	t.Run("WithTellNext", func(t *testing.T) {
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
			assert.Equal(t, fmt.Errorf("SetExecuteNode node id=%s not found", "notFound").Error(), err.Error())
		}))
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, int32(0), atomic.LoadInt32(&count))
	})
}
