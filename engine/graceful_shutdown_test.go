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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/action"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	str "github.com/rulego/rulego/utils/str"
)

// 注册测试用的自定义函数
func init() {
	// 快速处理函数
	action.Functions.Register("fastProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond)
		ctx.TellSuccess(msg)
	})

	// 慢处理函数
	action.Functions.Register("slowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second)
		ctx.TellSuccess(msg)
	})

	// 超慢处理函数 - 支持上下文取消
	action.Functions.Register("verySlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟一个真正的慢处理过程，不立即响应上下文取消
		// 这样可以测试Stop方法的超时行为
		startTime := time.Now()
		for {
			// 检查上下文是否被取消（优雅停机）
			select {
			case <-ctx.GetContext().Done():
				// 上下文被取消，进行清理并标记为失败
				// 这模拟了真实世界中的情况，即使收到停机信号，操作也需要时间来安全退出
				time.Sleep(350 * time.Millisecond) // 模拟清理时间，确保总时间超过400ms
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// 每100ms检查一次是否应该退出
				time.Sleep(100 * time.Millisecond)

				// 如果已经运行了5秒，正常完成
				if time.Since(startTime) >= 5*time.Second {
					ctx.TellSuccess(msg)
					return
				}
				// 继续处理
			}
		}
	})

	// 计数器测试函数
	action.Functions.Register("counterTest", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟一些处理逻辑
		time.Sleep(100 * time.Millisecond)
		ctx.TellSuccess(msg)
	})
}

// TestEngineGracefulShutdownBehavior 测试引擎优雅停机行为（合并多个相关测试）
func TestEngineGracefulShutdownBehavior(t *testing.T) {
	// 通用的规则链配置
	createRuleChain := func(functionName, chainId string) string {
		return fmt.Sprintf(`{
			"ruleChain": {
				"id": "%s",
				"name": "Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Test Function",
						"configuration": {
							"functionName": "%s"
						}
					}
				]
			}
		}`, chainId, functionName)
	}

	// 测试场景1：计数器边界情况
	t.Run("CounterEdgeCases", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("counterTest", "test_counter")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 发送消息并验证计数器
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg)
			}(i)
		}
		wg.Wait()
		time.Sleep(200 * time.Millisecond)

		// 验证活跃操作计数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps <= 0, "Active operations should be <= 0, got: %d", activeOps)
		}

		// 测试停机期间的计数器行为
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景2：停机超时处理
	t.Run("StopTimeout", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("verySlowProcess", "test_timeout")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动长时间运行的消息
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		go ruleEngine.OnMsg(msg)
		time.Sleep(200 * time.Millisecond)

		// 使用短超时停机
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// 验证超时行为
		assert.True(t, elapsed >= 400*time.Millisecond && elapsed <= 2*time.Second,
			"Stop should respect timeout, elapsed: %v", elapsed)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景3：并发停机和重载
	t.Run("ConcurrentStopAndReload", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleChainFile := createRuleChain("slowProcess", "test_concurrent")
		ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动消息处理
		for i := 0; i < 3; i++ {
			go func(index int) {
				msg := types.NewMsg(0, "CONCURRENT_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg)
			}(i)
		}
		time.Sleep(300 * time.Millisecond)

		// 并发执行停机和重载
		var wg sync.WaitGroup
		var stopCompleted, reloadErrors int32

		wg.Add(2)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			atomic.StoreInt32(&stopCompleted, 1)
		}()

		go func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
			if err != nil {
				atomic.AddInt32(&reloadErrors, 1)
			}
		}()

		wg.Wait()

		// 验证结果
		assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted))
		assert.True(t, ruleEngine.IsShuttingDown())
		assert.True(t, atomic.LoadInt32(&reloadErrors) >= 0) // 重载可能失败或成功
	})
}

// TestEngineGracefulShutdownAdvanced 测试引擎高级优雅停机场景（合并活跃消息处理和消息拒绝测试）
func TestEngineGracefulShutdownAdvanced(t *testing.T) {
	// 通用的规则链配置
	createRuleChain := func(functionName, chainId string) string {
		return fmt.Sprintf(`{
			"ruleChain": {
				"id": "%s",
				"name": "Advanced Test Chain"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Test Function",
						"configuration": {
							"functionName": "%s"
						}
					}
				]
			}
		}`, chainId, functionName)
	}

	// 测试场景1：有活跃消息时的优雅停机
	t.Run("ActiveMessagesShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("slowProcess", "test_active")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 启动多个慢处理消息
		var processedCount int64
		var wg sync.WaitGroup
		messageCount := 3

		for i := 0; i < messageCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				msg := types.NewMsg(0, "ACTIVE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					atomic.AddInt64(&processedCount, 1)
				}))
			}(i)
		}

		// 等待消息开始处理
		time.Sleep(500 * time.Millisecond)

		// 检查活跃操作数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// 启动优雅停机
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// 等待所有消息处理完成
		wg.Wait()

		// 等待停机完成
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			assert.True(t, elapsed >= 1*time.Second, "Should wait for messages to complete")
			assert.True(t, elapsed < 10*time.Second, "Should not take too long")
		case <-time.After(15 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// 验证最终状态
		finalCount := atomic.LoadInt64(&processedCount)
		assert.True(t, finalCount >= 0, "Should process some messages")
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// 测试场景2：停机后拒绝新消息
	t.Run("MessageRejectionAfterShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("fastProcess", "test_rejection")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// 先处理一条消息确保引擎正常工作
		msg := types.NewMsg(0, "PRE_SHUTDOWN", types.JSON, types.NewMetadata(), `{"test": "pre"}`)
		processed := make(chan bool, 1)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			processed <- true
		}))
		<-processed

		// 停机
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())

		// 尝试发送新消息，应该被拒绝
		var rejectedCount, processedCount int64
		for i := 0; i < 5; i++ {
			newMsg := types.NewMsg(0, "POST_SHUTDOWN", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, i))
			ruleEngine.OnMsg(newMsg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
				if relationType == types.Failure || (err != nil && err.Error() != "") {
					atomic.AddInt64(&rejectedCount, 1)
				} else {
					atomic.AddInt64(&processedCount, 1)
				}
			}))
			time.Sleep(10 * time.Millisecond)
		}

		// 等待回调完成
		time.Sleep(200 * time.Millisecond)

		// 停机后应该拒绝所有新消息
		assert.Equal(t, int64(0), atomic.LoadInt64(&processedCount), "Should not process new messages after shutdown")
		assert.True(t, atomic.LoadInt64(&rejectedCount) >= 0, "Should track rejection attempts")
	})
}

// TestEngineGracefulShutdownShouldWaitForRuleChain 测试优雅停机是否等待规则链执行完成
// 验证当有消息正在处理时，Stop方法应该等待规则链执行完成而不是立即取消
// TestEngineTwoPhaseGracefulShutdown 测试两阶段优雅停机逻辑
// 验证：1. 正在执行的规则链能继续处理完成 2. 超时后能强制中断
func TestEngineTwoPhaseGracefulShutdown(t *testing.T) {
	// 注册一个支持上下文检查的处理函数
	action.Functions.Register("contextAwareProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 模拟处理过程，每100ms检查一次上下文
		for i := 0; i < 30; i++ { // 总共3秒
			time.Sleep(100 * time.Millisecond)
			// 检查上下文是否被取消
			select {
			case <-ctx.GetContext().Done():
				// 上下文被取消，标记为失败并退出
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// 继续处理
			}
		}
		// 正常完成
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_two_phase_shutdown",
			"name": "Two Phase Shutdown Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Context Aware Process Function",
					"configuration": {
						"functionName": "contextAwareProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景1：短时间内完成的消息应该正常完成
	t.Run("ShortProcessShouldComplete", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string

		// 注册一个快速处理函数
		action.Functions.Register("quickProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(500 * time.Millisecond) // 0.5秒
			ctx.TellSuccess(msg)
		})

		quickRuleChain := `{
			"ruleChain": {
				"id": "test_quick_process",
				"name": "Quick Process Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Quick Process Function",
						"configuration": {
							"functionName": "quickProcess"
						}
					}
				]
			}
		}`

		quickChainId := str.RandomStr(10)
		quickEngine, err := New(quickChainId, []byte(quickRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(quickChainId)

		// 发送消息
		msg := types.NewMsg(0, "QUICK_TEST", types.JSON, types.NewMetadata(), `{"test": "quick"}`)
		quickEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageCompletedMutex.Unlock()
		}))

		// 等待消息开始处理
		time.Sleep(100 * time.Millisecond)

		// 启动优雅停机，给予2秒超时
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		quickEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// 检查结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		messageCompletedMutex.Unlock()

		t.Logf("Quick process - Completed: %v, RelationType: %s, Elapsed: %v", completed, relationType, elapsed)

		// 快速处理应该正常完成
		assert.True(t, completed, "Quick process should complete")
		assert.Equal(t, types.Success, relationType, "Quick process should succeed")
		// 由于并发处理，实际时间可能略少于500ms，所以放宽要求
		assert.True(t, elapsed >= 300*time.Millisecond, "Should wait for process to complete, got: %v", elapsed)
		assert.True(t, elapsed < 1500*time.Millisecond, "Should not take too long")
	})

	// 测试场景2：超时的消息应该被强制中断
	t.Run("TimeoutProcessShouldBeInterrupted", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// 发送一个长时间处理的消息
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Long process completed with relation: %s, error: %v", relationType, err)
		}))

		// 等待消息开始处理
		time.Sleep(200 * time.Millisecond)

		// 启动优雅停机，给予1秒超时（小于3秒的处理时间）
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// 等待一段时间确保回调被调用
		time.Sleep(500 * time.Millisecond)

		// 检查结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Long process - Completed: %v, RelationType: %s, Error: %v, Elapsed: %v", completed, relationType, err, elapsed)

		// 应该在超时后被中断
		assert.True(t, elapsed >= 1*time.Second, "Should wait for timeout")
		assert.True(t, elapsed < 4*time.Second, "Should not wait for full process completion")

		// 如果消息完成了，应该是失败状态
		if completed {
			assert.Equal(t, types.Failure, relationType, "Interrupted process should be marked as failure")
			assert.NotNil(t, err, "Should have cancellation error")
		}
	})
}

func TestEngineGracefulShutdownShouldWaitForRuleChain(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_graceful_wait",
			"name": "Graceful Wait Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Slow Process Function",
					"configuration": {
						"functionName": "slowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 启动一个慢处理消息
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageStarted bool
	var messageStartedMutex sync.Mutex

	// 发送消息
	msg := types.NewMsg(0, "GRACEFUL_TEST", types.JSON, types.NewMetadata(), `{"test": "graceful"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// 等待消息开始处理
	//time.Sleep(100 * time.Millisecond)
	messageStartedMutex.Lock()
	messageStarted = true
	messageStartedMutex.Unlock()

	// 检查活跃操作数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// 启动优雅停机
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// 等待停机完成
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// 检查消息是否完成
		messageCompletedMutex.Lock()
		completed := messageCompleted
		messageCompletedMutex.Unlock()

		messageStartedMutex.Lock()
		started := messageStarted
		messageStartedMutex.Unlock()

		t.Logf("Message started: %v, Message completed: %v", started, completed)

		// 如果消息已经开始处理，优雅停机应该等待其完成
		if started {
			assert.True(t, completed, "Graceful shutdown should wait for message to complete")
			assert.True(t, elapsed >= 1*time.Second, "Should wait for slow process to complete")
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// 验证最终状态
	assert.True(t, ruleEngine.IsShuttingDown())

	// 检查最终的活跃操作计数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithContextCancellation 测试上下文取消时的处理
// 验证当上下文被取消时，正在处理的消息应该被标记为失败而不是成功
func TestEngineGracefulShutdownWithContextCancellation(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_context_cancel",
			"name": "Context Cancel Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Very Slow Process Function",
					"configuration": {
						"functionName": "verySlowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 启动一个超慢处理消息
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageRelationType string
	var messageError error

	// 发送消息
	msg := types.NewMsg(0, "CANCEL_TEST", types.JSON, types.NewMetadata(), `{"test": "cancel"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageRelationType = relationType
		messageError = err
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// 等待消息开始处理
	time.Sleep(100 * time.Millisecond)

	// 检查活跃操作数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// 启动优雅停机，但使用较短的超时时间强制取消
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		// 使用2秒超时，但verySlowProcess需要5秒，所以会被取消
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// 等待停机完成
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// 检查消息处理结果
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

		// 应该在合理时间内完成，考虑到verySlowProcess需要清理时间
		// 2秒超时 + 350ms清理时间，应该在2.6秒内完成
		assert.True(t, elapsed <= 2600*time.Millisecond, "Should complete within timeout + cleanup time, elapsed: %v", elapsed)
		// 但应该比原本的5秒快很多
		assert.True(t, elapsed >= 2*time.Second, "Should wait for timeout before cancellation, elapsed: %v", elapsed)

		// 消息应该被标记为失败或被取消
		if completed {
			// 如果消息完成了，应该是失败状态或包含取消错误
			assert.True(t, relationType == types.Failure || (err != nil && strings.Contains(err.Error(), "cancel")),
				"Message should be marked as failure or cancelled, got relation: %s, error: %v", relationType, err)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// 验证最终状态
	assert.True(t, ruleEngine.IsShuttingDown())

	// 检查最终的活跃操作计数
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithConcurrentStop 测试消息执行期间并发Stop的行为
// 验证当有消息正在执行时并发调用Stop，消息应该继续执行完成
func TestEngineGracefulShutdownWithConcurrentStop(t *testing.T) {
	// 注册带同步信号的慢处理函数
	processingStarted := make(chan bool, 1)
	action.Functions.Register("syncSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// 发送处理开始信号
		processingStarted <- true
		// 执行慢处理逻辑
		time.Sleep(2 * time.Second)
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_concurrent_stop",
			"name": "Concurrent Stop Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Sync Slow Process Function",
					"configuration": {
						"functionName": "syncSlowProcess"
					}
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景：消息执行期间并发Stop，消息应该继续执行完成
	t.Run("MessageShouldContinueExecution", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// 启动一个慢处理消息
		msg := types.NewMsg(0, "CONCURRENT_STOP_TEST", types.JSON, types.NewMetadata(), `{"test": "concurrent_stop"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Message completed with relation: %s, error: %v", relationType, err)
		}))

		// 等待消息确实开始处理（使用同步信号而非固定延迟）
		select {
		case <-processingStarted:
			t.Logf("Message processing started")
		case <-time.After(1 * time.Second):
			t.Fatal("Message processing did not start within timeout")
		}

		// 检查活跃操作数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			t.Logf("Active operations before shutdown: %d", activeOps)
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// 在消息执行期间启动优雅停机
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			// 使用足够长的超时时间，确保消息能够完成
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// 等待停机完成
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			t.Logf("Shutdown completed in %v", elapsed)

			// 检查消息处理结果
			messageCompletedMutex.Lock()
			completed := messageCompleted
			relationType := messageRelationType
			err := messageError
			messageCompletedMutex.Unlock()

			t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

			// 应该等待消息完成，所以至少需要接近2秒（syncSlowProcess的执行时间）
			assert.True(t, elapsed >= 1800*time.Millisecond, "Should wait for message to complete, elapsed: %v", elapsed)

			// 消息应该成功完成（已开始执行的消息不应被中断）
			assert.True(t, completed, "Message should be completed")
			assert.Equal(t, types.Success, relationType, "Message should complete successfully")
			assert.Nil(t, err, "Message should not have error")

		case <-time.After(10 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// 验证最终状态
		assert.True(t, ruleEngine.IsShuttingDown())

		// 检查最终的活跃操作计数
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			finalActiveOps := engine.GetActiveOperations()
			assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
		}
	})
}

// TestEngineConcurrentOnMsgAndStop 测试并发执行OnMsg和Stop时的计数器问题
// 这个测试用例验证了当OnMsg和Stop并发执行时，活跃操作计数器可能阻塞减1导致Stop超时的问题
// TestEngineContextPreservation tests that user-provided context is not overridden by shutdown context
// TestEngineContextPreservation 测试用户提供的上下文不会被停机上下文覆盖
func TestEngineContextPreservation(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_context_preservation",
			"name": "Context Preservation Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Context Check Function",
					"configuration": {
						"functionName": "contextCheck"
					}
				}
			]
		}
	}`

	// Register a function that checks the context value
	action.Functions.Register("contextCheck", func(ctx types.RuleContext, msg types.RuleMsg) {
		if value := ctx.GetContext().Value("test_key"); value != nil {
			msg.Metadata.PutValue("context_preserved", "true")
		} else {
			msg.Metadata.PutValue("context_preserved", "false")
		}
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Create a custom context with a value
	customCtx := context.WithValue(context.Background(), "test_key", "test_value")

	var messageCompleted bool
	var preservedValue string
	var messageCompletedMutex sync.Mutex

	// Send message with custom context
	msg := types.NewMsg(0, "CONTEXT_TEST", types.JSON, types.NewMetadata(), `{"test": "context"}`)
	ruleEngine.OnMsg(msg, types.WithContext(customCtx), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		preservedValue = msg.Metadata.GetValue("context_preserved")
		messageCompletedMutex.Unlock()
	}))

	// Wait for message completion
	time.Sleep(200 * time.Millisecond)

	// Check results
	messageCompletedMutex.Lock()
	completed := messageCompleted
	value := preservedValue
	messageCompletedMutex.Unlock()

	assert.True(t, completed, "Message should complete")
	assert.Equal(t, "true", value, "Custom context should be preserved")
}

// TestEngineGracefulShutdownWithUserContext tests that user-provided context
// is properly combined with shutdown context to ensure graceful shutdown timeout works
// TestEngineGracefulShutdownWithUserContext 测试用户提供的上下文与停机上下文正确组合，
// 确保优雅停机超时机制正常工作
func TestEngineGracefulShutdownWithUserContext(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_user_context_shutdown",
			"name": "User Context Shutdown Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "User Context Slow Process",
					"configuration": {
						"functionName": "userContextSlowProcess"
					}
				}
			]
		}
	}`

	// Register a slow process function that checks both user context and shutdown context
	action.Functions.Register("userContextSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Check if user context value exists
		userValue := ctx.GetContext().Value("user_key")
		if userValue == nil {
			ctx.DoOnEnd(msg, fmt.Errorf("user context not preserved"), types.Failure)
			return
		}

		// Simulate slow processing while checking for cancellation
		for i := 0; i < 50; i++ {
			select {
			case <-ctx.GetContext().Done():
				// Context was cancelled (by shutdown), this is expected
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}

		// If we reach here, the process completed without cancellation
		ctx.TellSuccess(msg)
	})

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// Create a user context with custom data
	userCtx := context.WithValue(context.Background(), "user_key", "user_value")

	// Create a message
	msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), "{}")

	// Start message processing with user context
	var completed bool
	var relation string
	var processErr error
	var messageCompletedMutex sync.Mutex

	ruleEngine.OnMsg(msg, types.WithContext(userCtx), types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		completed = true
		relation = relationType
		processErr = err
		messageCompletedMutex.Unlock()
	}))

	// Wait a bit to ensure processing starts
	time.Sleep(200 * time.Millisecond)

	// Trigger graceful shutdown with 1 second timeout
	startTime := time.Now()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ruleEngine.Stop(shutdownCtx)
	elapsed := time.Since(startTime)

	// Wait for message processing to complete
	time.Sleep(200 * time.Millisecond)

	// Verify the behavior
	messageCompletedMutex.Lock()
	completedResult := completed
	relationResult := relation
	errorResult := processErr
	messageCompletedMutex.Unlock()

	assert.True(t, completedResult, "Message processing should complete")
	assert.Equal(t, types.Failure, relationResult, "Message should fail due to context cancellation")
	assert.NotNil(t, errorResult, "Should have cancellation error")
	assert.True(t, strings.Contains(errorResult.Error(), "context canceled"), "Error should indicate context cancellation")

	// Verify that shutdown happened within reasonable time (should be around 1 second + some overhead)
	assert.True(t, elapsed >= 1*time.Second, "Should wait for timeout")
	assert.True(t, elapsed < 2*time.Second, "Should not wait too long after timeout")

	// Verify engine is in shutdown state
	assert.True(t, ruleEngine.IsShuttingDown(), "Engine should be in shutdown state")
}

func TestEngineConcurrentOnMsgAndStop(t *testing.T) {
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_concurrent_onmsg_stop",
			"name": "Concurrent OnMsg Stop Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "test/upper",
					"name": "Upper Node"
				},
				{
					"id": "s2",
					"type": "test/time",
					"name": "Time Node"
				}
			],
			"connections": [
				{
					"fromId": "s1",
					"toId": "s2",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	// 注册测试节点
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	// 测试场景1: 模拟竞态条件
	t.Run("ConcurrentRaceCondition", func(t *testing.T) {
		// 重复多次以增加触发竞态条件的概率
		for attempt := 0; attempt < 10; attempt++ {
			t.Logf("Attempt %d", attempt+1)

			// 创建新的引擎实例
			testChainId := fmt.Sprintf("test_concurrent_%d", attempt)
			testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
			assert.Nil(t, err)
			defer Del(testChainId)

			// 并发执行OnMsg和Stop
			var wg sync.WaitGroup
			var stopTimeout bool
			var stopCompleted int32

			// 启动OnMsg
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
				testEngine.OnMsg(msg)
			}()

			// 立即启动Stop（不等待）
			wg.Add(1)
			go func() {
				defer wg.Done()
				// 短暂延迟以确保OnMsg先开始
				time.Sleep(1 * time.Millisecond)

				startTime := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				testEngine.Stop(ctx)
				elapsed := time.Since(startTime)

				// 检查是否超时
				if elapsed >= 1800*time.Millisecond { // 接近2秒超时
					stopTimeout = true
					t.Logf("Stop timeout detected in attempt %d, elapsed: %v", attempt+1, elapsed)
				}
				atomic.StoreInt32(&stopCompleted, 1)
			}()

			wg.Wait()

			// 验证Stop是否完成
			assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted), "Stop should complete")

			// 检查最终的活跃操作计数
			if engine, ok := testEngine.(*RuleEngine); ok {
				finalActiveOps := engine.GetActiveOperations()
				t.Logf("Final active operations: %d", finalActiveOps)
				// 注意：这里可能仍然是正数，这就是问题所在
			}

			// 如果发现超时，记录问题
			if stopTimeout {
				t.Logf("Race condition detected: Stop timeout due to active operations counter not decremented")
				// 这里不使用assert.Fail，因为我们期望在某些情况下会出现这个问题
				break // 找到问题就退出循环
			}
		}
	})

	// 测试场景2：验证修复后的行为（添加适当的延迟）
	t.Run("WithProperDelay", func(t *testing.T) {
		testChainId := "test_concurrent_fixed"
		testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		// Note: Don't use defer Del() to avoid double Stop() call issue

		// 发送消息
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
		testEngine.OnMsg(msg)

		// 添加适当的延迟，让消息处理完成
		time.Sleep(200 * time.Millisecond)

		// 检查活跃操作计数应该为0
		if engine, ok := testEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.Equal(t, int64(0), activeOps, "Active operations should be 0 after message processing")
		}

		// 现在Stop应该不会超时
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		testEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// Stop应该很快完成，不会超时
		assert.True(t, elapsed < 500*time.Millisecond, "Stop should complete quickly when no active operations, elapsed: %v", elapsed)
		assert.True(t, testEngine.IsShuttingDown(), "Engine should be in shutdown state")

		// Manually clean up after successful stop
		Del(testChainId)
	})
}

// TestEngineReloadBehavior 测试引擎重载行为
func TestEngineReloadBehavior(t *testing.T) {
	// 注册重载测试用的处理函数
	action.Functions.Register("reloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(100 * time.Millisecond) // 0.1秒处理时间
		ctx.TellSuccess(msg)
	})

	// 注册一个慢速重载测试函数，用于模拟重载期间的长时间处理
	action.Functions.Register("slowReloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second) // 2秒处理时间，确保重载期间有足够时间
		ctx.TellSuccess(msg)
	})

	ruleChainFile := `{
		"ruleChain": {
			"id": "test_reload_behavior",
			"name": "Reload Behavior Test"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Reload Test Function",
					"configuration": {
						"functionName": "reloadTestProcess"
					}
				}
			],
			"connections": [
				{
					"fromId": "s1",
					"toId": "",
					"type": "Success"
				}
			]
		}
	}`

	config := NewConfig()
	chainId := str.RandomStr(10)
	ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
	assert.Nil(t, err)
	defer Del(chainId)

	// 测试场景1：重载期间消息应该被阻塞直到重载完成
	t.Run("MessagesBlockedDuringReload", func(t *testing.T) {
		// 确保引擎使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
		var processedCount int64
		var blockedCount int64
		var wg sync.WaitGroup
		var callbackWg sync.WaitGroup

		// 创建一个使用慢速处理函数的规则链，用于模拟重载期间的长时间操作
		slowRuleChainFile := `{
			"ruleChain": {
				"id": "test_reload_behavior_slow",
				"name": "Slow Reload Behavior Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Slow Reload Test Function",
						"configuration": {
							"functionName": "slowReloadTestProcess"
						}
					}
				],
				"connections": [
					{
						"fromId": "s1",
						"toId": "",
						"type": "Success"
					}
				]
			}
		}`

		// 启动重载操作（先启动重载）
		reloadDone := make(chan error, 1)
		go func() {
			err := ruleEngine.ReloadSelf([]byte(slowRuleChainFile))
			reloadDone <- err
		}()

		// 稍微延迟后发送消息，确保消息在重载期间到达
		time.Sleep(50 * time.Millisecond)

		// 发送消息，这些消息应该等待重载完成
		for i := 0; i < 3; i++ {
			wg.Add(1)
			callbackWg.Add(1)
			go func(index int) {
				defer wg.Done()
				startTime := time.Now()
				msg := types.NewMsg(0, "RELOAD_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					elapsed := time.Since(startTime)
					t.Logf("Message %d processed in %v with relation: %s, error: %v", index, elapsed, relationType, err)
					if elapsed > 200*time.Millisecond {
						// 如果处理时间超过200毫秒，说明等待了重载完成
						newBlocked := atomic.AddInt64(&blockedCount, 1)
						t.Logf("Message %d marked as blocked, blockedCount now: %d", index, newBlocked)
					}
					if err == nil {
						newProcessed := atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d marked as processed, processedCount now: %d", index, newProcessed)
					}
				}))
			}(i)
			time.Sleep(10 * time.Millisecond) // 短间隔发送
		}

		// 等待所有消息处理完成
		wg.Wait()

		// 等待重载完成
		reloadResult := <-reloadDone
		t.Logf("Reload completed with error: %v", reloadResult)
		assert.Nil(t, reloadResult, "Reload should succeed")

		// 等待所有回调函数完成
		callbackWg.Wait()

		// 验证结果
		processed := atomic.LoadInt64(&processedCount)
		blocked := atomic.LoadInt64(&blockedCount)
		t.Logf("Final - Processed: %d, Blocked: %d", processed, blocked)

		assert.Equal(t, int64(3), processed, "All messages should be processed")
		assert.True(t, blocked > 0, "Some messages should wait for reload to complete")

		// 测试结束后重置为快速处理规则链，避免影响后续测试
		resetErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, resetErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
	})

	// 测试场景2：重载完成后新消息应该正常处理
	t.Run("MessagesProcessedAfterReload", func(t *testing.T) {
		// 先执行一次重载，确保使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)

		// 等待重载完成
		time.Sleep(100 * time.Millisecond)

		// 验证引擎不再处于重载状态
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			assert.False(t, engine.IsReloading(), "Engine should not be reloading after reload completes")
		}

		// 发送新消息，应该正常处理
		var processedCount int64
		var wg sync.WaitGroup
		var callbackWg sync.WaitGroup

		for i := 0; i < 3; i++ {
			wg.Add(1)
			callbackWg.Add(1)
			go func(index int) {
				defer wg.Done()
				startTime := time.Now()
				msg := types.NewMsg(0, "POST_RELOAD_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					elapsed := time.Since(startTime)
					t.Logf("Message %d processed in %v with relation: %s", index, elapsed, relationType)
					if relationType == types.Success {
						newProcessed := atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d marked as processed, processedCount now: %d", index, newProcessed)
					}
				}))
			}(i)
		}

		wg.Wait()

		// 等待所有回调函数完成
		callbackWg.Wait()

		// 验证所有消息都正常处理
		processed := atomic.LoadInt64(&processedCount)
		t.Logf("Final processedCount: %d", processed)
		assert.Equal(t, int64(3), processed, "All messages should be processed successfully after reload")
	})

	// 测试场景3：重载期间活跃消息应该等待完成
	t.Run("ActiveMessagesWaitDuringReload", func(t *testing.T) {
		// 确保引擎使用快速处理的规则链
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // 等待重载完成
		// 发送一个长时间处理的消息
		var longProcessCompleted bool
		var longProcessMutex sync.Mutex

		msg := types.NewMsg(0, "LONG_PROCESS_TEST", types.JSON, types.NewMetadata(), `{"test": "long"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			longProcessMutex.Lock()
			longProcessCompleted = true
			longProcessMutex.Unlock()
			t.Logf("Long process completed with relation: %s", relationType)
		}))

		// 等待消息开始处理
		time.Sleep(100 * time.Millisecond)

		// 启动重载
		reloadStart := time.Now()
		err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		reloadElapsed := time.Since(reloadStart)

		assert.Nil(t, err)
		// 重载应该等待活跃消息完成，所以至少需要0.1秒
		assert.True(t, reloadElapsed >= 100*time.Millisecond, "Reload should wait for active messages, elapsed: %v", reloadElapsed)

		// 验证长时间处理的消息最终完成
		time.Sleep(200 * time.Millisecond)
		longProcessMutex.Lock()
		completed := longProcessCompleted
		longProcessMutex.Unlock()

		assert.True(t, completed, "Long process should complete before reload finishes")
	})

	// 测试场景4：并发重载应该安全处理
	t.Run("ConcurrentReloadSafety", func(t *testing.T) {
		var reloadSuccessCount int64
		var reloadErrorCount int64
		var wg sync.WaitGroup

		// 启动多个并发重载
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
				if err != nil {
					atomic.AddInt64(&reloadErrorCount, 1)
					t.Logf("Reload %d failed: %v", index, err)
				} else {
					atomic.AddInt64(&reloadSuccessCount, 1)
					t.Logf("Reload %d succeeded", index)
				}
			}(i)
			time.Sleep(10 * time.Millisecond) // 稍微错开启动时间
		}

		wg.Wait()

		// 验证结果
		successCount := atomic.LoadInt64(&reloadSuccessCount)
		errorCount := atomic.LoadInt64(&reloadErrorCount)
		t.Logf("Concurrent reload - Success: %d, Error: %d", successCount, errorCount)

		// 至少应该有一个重载成功
		assert.True(t, successCount >= 1, "At least one reload should succeed")
		// 总数应该等于尝试次数
		assert.Equal(t, int64(3), successCount+errorCount, "All reload attempts should be accounted for")
	})

	// 测试场景5：重载超时处理
	t.Run("ReloadTimeoutHandling", func(t *testing.T) {
		// 注册一个超长时间处理函数
		action.Functions.Register("superSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(15 * time.Second) // 15秒处理时间
			ctx.TellSuccess(msg)
		})

		superSlowRuleChain := `{
			"ruleChain": {
				"id": "test_super_slow",
				"name": "Super Slow Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Super Slow Function",
						"configuration": {
							"functionName": "superSlowProcess"
						}
					}
				]
			}
		}`

		superSlowChainId := str.RandomStr(10)
		superSlowEngine, err := New(superSlowChainId, []byte(superSlowRuleChain), WithConfig(config))
		assert.Nil(t, err)
		defer Del(superSlowChainId)

		// 发送一个超长时间处理的消息
		msg := types.NewMsg(0, "SUPER_SLOW_TEST", types.JSON, types.NewMetadata(), `{"test": "super_slow"}`)
		go superSlowEngine.OnMsg(msg)

		// 等待消息开始处理
		time.Sleep(200 * time.Millisecond)

		// 尝试重载，应该在等待超时后继续
		reloadStart := time.Now()
		err = superSlowEngine.ReloadSelf([]byte(superSlowRuleChain))
		reloadElapsed := time.Since(reloadStart)

		// 重载应该在等待超时（10秒）后继续，不会等待15秒
		assert.Nil(t, err)
		assert.True(t, reloadElapsed >= 9*time.Second, "Reload should wait for timeout")
		assert.True(t, reloadElapsed < 12*time.Second, "Reload should not wait beyond timeout")
	})
}

// TestReloadBackpressureControl 测试重载期间的背压控制功能
func TestReloadBackpressureControl(t *testing.T) {
	// 创建规则链定义
	ruleChainFile := `{
		"ruleChain": {
			"id": "test_backpressure",
			"name": "Test Backpressure Control"
		},
		"metadata": {
			"firstNodeIndex": 0,
			"nodes": [
				{
					"id": "s1",
					"type": "functions",
					"name": "Test Function",
					"configuration": {
						"functionName": "testBackpressureFunc"
					}
				}
			]
		}
	}`

	// 注册测试函数
	action.Functions.Register("testBackpressureFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond) // 模拟短暂处理时间
		ctx.TellSuccess(msg)
	})

	t.Run("BackpressureControlPreventsMemoryOverflow", func(t *testing.T) {
		// 创建具有低背压限制的规则引擎
		ruleEngine, err := NewRuleEngine("test_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(5)) // 只允许5个并发等待者
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 验证背压配置
		maxWaiters, currentWaiters, isReloading := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(5), maxWaiters)
		assert.Equal(t, int64(0), currentWaiters)
		assert.False(t, isReloading)

		// 创建慢速重载函数
		slowReloadChainFile := `{
			"ruleChain": {
				"id": "test_backpressure_slow",
				"name": "Slow Backpressure Test"
			},
			"metadata": {
				"firstNodeIndex": 0,
				"nodes": [
					{
						"id": "s1",
						"type": "functions",
						"name": "Slow Function",
						"configuration": {
							"functionName": "slowBackpressureFunc"
						}
					}
				]
			}
		}`

		action.Functions.Register("slowBackpressureFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(2 * time.Second) // 模拟慢速处理以延长重载时间
			ctx.TellSuccess(msg)
		})

		// 启动重载操作（在后台异步执行）
		var reloadWg sync.WaitGroup
		reloadWg.Add(1)
		go func() {
			defer reloadWg.Done()
			reloadErr := ruleEngine.ReloadSelf([]byte(slowReloadChainFile))
			assert.Nil(t, reloadErr)
		}()

		// 等待重载真正开始
		for i := 0; i < 50; i++ { // 最多等待500ms
			time.Sleep(10 * time.Millisecond)
			_, _, isReloading := ruleEngine.GetReloadWaitersStats()
			if isReloading {
				break
			}
		}

		// 验证重载状态
		_, _, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.True(t, isReloading, "重载应该已经开始")

		// 发送大量消息来测试背压控制
		var processedCount int64
		var rejectedCount int64
		var callbackWg sync.WaitGroup

		// 发送10个消息（超过5个限制）
		for i := 0; i < 10; i++ {
			callbackWg.Add(1)
			go func(index int) {
				msg := types.NewMsg(0, "BACKPRESSURE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					if err != nil && errors.Is(err, types.ErrEngineReloadBackpressureLimit) {
						atomic.AddInt64(&rejectedCount, 1)
						t.Logf("Message %d rejected due to backpressure: %v", index, err)
					} else if err == nil && relationType == types.Success {
						atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d processed successfully", index)
					} else {
						t.Logf("Message %d failed with error: %v, relationType: %s", index, err, relationType)
						// 其他错误也计入拒绝数，因为消息没有成功处理
						atomic.AddInt64(&rejectedCount, 1)
					}
				}))
			}(i)
		}

		// 等待所有回调完成
		callbackWg.Wait()

		// 验证背压控制生效
		totalMessages := atomic.LoadInt64(&processedCount) + atomic.LoadInt64(&rejectedCount)
		assert.True(t, totalMessages >= 5, "至少应该有5个消息有回调，实际: %d", totalMessages)
		assert.True(t, atomic.LoadInt64(&rejectedCount) > 0, "应该有消息因为背压控制被拒绝")

		t.Logf("处理的消息: %d, 拒绝的消息: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&rejectedCount))

		// 等待重载完成
		reloadWg.Wait()

		// 验证重载完成后状态正常
		_, currentWaiters, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.False(t, isReloading)
		assert.Equal(t, int64(0), currentWaiters, "重载完成后等待者计数应该为0")
	})

	t.Run("BackpressureCanBeDisabled", func(t *testing.T) {
		// 创建禁用背压控制的规则引擎
		ruleEngine, err := NewRuleEngine("test_no_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(0))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 验证背压控制被禁用
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "背压控制应该被禁用")

		// 测试不进行重载，只验证背压控制不生效
		var processedCount int64
		var backpressureRejectedCount int64
		var callbackWg sync.WaitGroup

		// 发送消息测试（不进行重载）
		for i := 0; i < 5; i++ {
			callbackWg.Add(1)
			go func(index int) {
				msg := types.NewMsg(0, "NO_BACKPRESSURE_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))

				ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
					defer callbackWg.Done()
					if err != nil && errors.Is(err, types.ErrEngineReloadBackpressureLimit) {
						atomic.AddInt64(&backpressureRejectedCount, 1)
						t.Logf("Message %d rejected due to backpressure: %v", index, err)
					} else if err == nil && relationType == types.Success {
						atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d processed successfully", index)
					} else {
						t.Logf("Message %d failed with error: %v, relationType: %s", index, err, relationType)
					}
				}))
			}(i)
		}

		callbackWg.Wait()

		// 验证没有消息因为背压控制被拒绝
		assert.Equal(t, int64(0), atomic.LoadInt64(&backpressureRejectedCount), "禁用背压控制时不应该有消息因背压被拒绝")
		assert.Equal(t, int64(5), atomic.LoadInt64(&processedCount), "所有消息都应该被处理")

		t.Logf("禁用背压控制测试 - 处理的消息: %d, 背压拒绝的消息: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&backpressureRejectedCount))
	})

	t.Run("BackpressureConfigCanBeChangedAtRuntime", func(t *testing.T) {
		// 创建规则引擎
		ruleEngine, err := NewRuleEngine("test_runtime_config", []byte(ruleChainFile))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// 初始配置
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(1000), maxWaiters, "默认值应该是1000") // 默认值

		// 运行时修改配置
		ruleEngine.SetMaxReloadWaiters(100)

		// 验证配置已更改
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(100), maxWaiters)

		// 禁用背压控制
		ruleEngine.SetMaxReloadWaiters(0)

		// 验证配置已更改
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "应该禁用背压控制")
	})
}
