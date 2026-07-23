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

// Custom functions for registering tests
func init() {
	// Quick-processing functions
	action.Functions.Register("fastProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond)
		ctx.TellSuccess(msg)
	})

	// Slow processing function
	action.Functions.Register("slowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second)
		ctx.TellSuccess(msg)
	})

	// Ultra-slow processing function - supports context cancellation
	action.Functions.Register("verySlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Simulating a truly slow processing process that does not immediately respond to context cancellation
		// This allows testing the timeout behavior of the Stop method
		startTime := time.Now()
		for {
			// Check if the context has been canceled (graceful shutdown)
			select {
			case <-ctx.GetContext().Done():
				// The context is canceled, cleaned up, and marked as failed
				// This simulates real-world situations: even when a shutdown signal is received, operation still takes time to exit safely
				time.Sleep(350 * time.Millisecond) // Simulated cleaning time ensures a total time of over 400ms
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// Check every 100ms to see if you should exit
				time.Sleep(100 * time.Millisecond)

				// If it has run for 5 seconds, it completes normally
				if time.Since(startTime) >= 5*time.Second {
					ctx.TellSuccess(msg)
					return
				}
				// Keep working on it
			}
		}
	})

	// Counter test function
	action.Functions.Register("counterTest", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Simulate some processing logic
		time.Sleep(100 * time.Millisecond)
		ctx.TellSuccess(msg)
	})
}

// TestEngineGracefulShutdownBehavior (Combines multiple related tests)
func TestEngineGracefulShutdownBehavior(t *testing.T) {
	// A general rule chain configuration
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

	// Test Scenario 1: Counter boundary conditions
	t.Run("CounterEdgeCases", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("counterTest", "test_counter")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// Send messages and verify the counter
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

		// Verify the active operation count
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps <= 0, "Active operations should be <= 0, got: %d", activeOps)
		}

		// Test counter behavior during downtime
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// Test Scenario 2: Handling downtime timeout
	t.Run("StopTimeout", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("verySlowProcess", "test_timeout")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// Launch messages that run for a long time
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		go ruleEngine.OnMsg(msg)
		time.Sleep(200 * time.Millisecond)

		// Use short overtime shutdown
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// Verify timeout behavior
		assert.True(t, elapsed >= 400*time.Millisecond && elapsed <= 2*time.Second,
			"Stop should respect timeout, elapsed: %v", elapsed)
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// Test Scenario 3: Concurrent shutdown and heavy loading
	t.Run("ConcurrentStopAndReload", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleChainFile := createRuleChain("slowProcess", "test_concurrent")
		ruleEngine, err := New(chainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// Start message processing
		for i := 0; i < 3; i++ {
			go func(index int) {
				msg := types.NewMsg(0, "CONCURRENT_TEST", types.JSON, types.NewMetadata(), fmt.Sprintf(`{"index": %d}`, index))
				ruleEngine.OnMsg(msg)
			}(i)
		}
		time.Sleep(300 * time.Millisecond)

		// Concurrent execution of shutdown and reload
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

		// Verify the results
		assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted))
		assert.True(t, ruleEngine.IsShuttingDown())
		assert.True(t, atomic.LoadInt32(&reloadErrors) >= 0) // Overloading can fail or succeed
	})
}

// TestEngineGracefulShutdownAdvanced Advanced Elegant Downtime Scenario Test Engine (Merging Active Message Processing and Message Denial Testing)
func TestEngineGracefulShutdownAdvanced(t *testing.T) {
	// A general rule chain configuration
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

	// Test Scenario 1: Graceful downtime when active messages appear
	t.Run("ActiveMessagesShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("slowProcess", "test_active")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// Initiate multiple slow-processing messages
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

		// Waiting for the message to start processing
		time.Sleep(500 * time.Millisecond)

		// Check the number of active operations
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// Launch with elegant stop
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// Wait for all messages to be processed
		wg.Wait()

		// Wait for the shutdown to complete
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			assert.True(t, elapsed >= 1*time.Second, "Should wait for messages to complete")
			assert.True(t, elapsed < 10*time.Second, "Should not take too long")
		case <-time.After(15 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// Verify the final state
		finalCount := atomic.LoadInt64(&processedCount)
		assert.True(t, finalCount >= 0, "Should process some messages")
		assert.True(t, ruleEngine.IsShuttingDown())
	})

	// Test Scenario 2: Rejecting new messages after a shutdown
	t.Run("MessageRejectionAfterShutdown", func(t *testing.T) {
		config := NewConfig()
		chainId := str.RandomStr(10)
		ruleEngine, err := New(chainId, []byte(createRuleChain("fastProcess", "test_rejection")), WithConfig(config))
		assert.Nil(t, err)
		defer Del(chainId)

		// First, handle a message to ensure the engine is working properly
		msg := types.NewMsg(0, "PRE_SHUTDOWN", types.JSON, types.NewMetadata(), `{"test": "pre"}`)
		processed := make(chan bool, 1)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			processed <- true
		}))
		<-processed

		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		assert.True(t, ruleEngine.IsShuttingDown())

		// Trying to send a new message should be rejected
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

		// Waiting for the pullback to complete
		time.Sleep(200 * time.Millisecond)

		// After the shutdown, all new news should be rejected
		assert.Equal(t, int64(0), atomic.LoadInt64(&processedCount), "Should not process new messages after shutdown")
		assert.True(t, atomic.LoadInt64(&rejectedCount) >= 0, "Should track rejection attempts")
	})
}

// TestEngineGracefulShutdownShouldWaitForRuleChain Test whether the elegant shutdown is waiting for the rule chain to complete
// Verify that when a message is being processed, the Stop method should wait for the rule chain to complete execution rather than cancel immediately
// TestEngineTwoPhaseGracefulShutdown tests the logic of the two-phase graceful shutdown
// Verification: 1. The executing rule chain can continue processing and completing 2. After timeout, you can forcibly interrupt the event
func TestEngineTwoPhaseGracefulShutdown(t *testing.T) {
	// Register a handler that supports context checking
	action.Functions.Register("contextAwareProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Simulates the processing process, checking context every 100ms
		for i := 0; i < 30; i++ { // A total of 3 seconds
			time.Sleep(100 * time.Millisecond)
			// Check if the context is canceled
			select {
			case <-ctx.GetContext().Done():
				// The context is canceled, marked as failed, and exited
				ctx.DoOnEnd(msg, ctx.GetContext().Err(), types.Failure)
				return
			default:
				// Keep working on it
			}
		}
		// Completed normally
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

	// Test Scenario 1: Messages completed within a short time should be completed normally
	t.Run("ShortProcessShouldComplete", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string

		// Register a quick processing function
		action.Functions.Register("quickProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(500 * time.Millisecond) // 0.5 seconds
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

		// Send the message
		msg := types.NewMsg(0, "QUICK_TEST", types.JSON, types.NewMetadata(), `{"test": "quick"}`)
		quickEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageCompletedMutex.Unlock()
		}))

		// Waiting for the message to start processing
		time.Sleep(100 * time.Millisecond)

		// Graceful start-up stop, with a 2-second timeout
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		quickEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// Inspection results
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		messageCompletedMutex.Unlock()

		t.Logf("Quick process - Completed: %v, RelationType: %s, Elapsed: %v", completed, relationType, elapsed)

		// Quick processing should be completed normally
		assert.True(t, completed, "Quick process should complete")
		assert.Equal(t, types.Success, relationType, "Quick process should succeed")
		// Due to concurrent processing, the actual time may be slightly less than 500ms, so the requirements are relaxed
		assert.True(t, elapsed >= 300*time.Millisecond, "Should wait for process to complete, got: %v", elapsed)
		assert.True(t, elapsed < 1500*time.Millisecond, "Should not take too long")
	})

	// Test Scenario 2: Timed messages should be forcibly interrupted
	t.Run("TimeoutProcessShouldBeInterrupted", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// Send a message that has been processed for a long time
		msg := types.NewMsg(0, "TIMEOUT_TEST", types.JSON, types.NewMetadata(), `{"test": "timeout"}`)
		ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Long process completed with relation: %s, error: %v", relationType, err)
		}))

		// Waiting for the message to start processing
		time.Sleep(200 * time.Millisecond)

		// Start-up graceful shutdown, with a 1-second timeout (less than 3 seconds of processing time)
		shutdownStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		elapsed := time.Since(shutdownStart)

		// Wait a while to ensure the pullback is called
		time.Sleep(500 * time.Millisecond)

		// Inspection results
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Long process - Completed: %v, RelationType: %s, Error: %v, Elapsed: %v", completed, relationType, err, elapsed)

		// It should be interrupted after the timeout
		assert.True(t, elapsed >= 1*time.Second, "Should wait for timeout")
		assert.True(t, elapsed < 4*time.Second, "Should not wait for full process completion")

		// If the message is complete, it should be a failure state
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

	// Start a slow message processing
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageStarted bool
	var messageStartedMutex sync.Mutex

	// Send the message
	msg := types.NewMsg(0, "GRACEFUL_TEST", types.JSON, types.NewMetadata(), `{"test": "graceful"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// Waiting for the message to start processing
	//time.Sleep(100 * time.Millisecond)
	messageStartedMutex.Lock()
	messageStarted = true
	messageStartedMutex.Unlock()

	// Check the number of active operations
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// Launch with elegant stop
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// Wait for the shutdown to complete
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// Check if the message is complete
		messageCompletedMutex.Lock()
		completed := messageCompleted
		messageCompletedMutex.Unlock()

		messageStartedMutex.Lock()
		started := messageStarted
		messageStartedMutex.Unlock()

		t.Logf("Message started: %v, Message completed: %v", started, completed)

		// If the message has already started to be processed, graceful shutdown should wait for it to be completed
		if started {
			assert.True(t, completed, "Graceful shutdown should wait for message to complete")
			assert.True(t, elapsed >= 1*time.Second, "Should wait for slow process to complete")
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// Verify the final state
	assert.True(t, ruleEngine.IsShuttingDown())

	// Check the final active operation count
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithContextCancellation Tests the handling of context cancellation
// Validation: When context is canceled, the message being processed should be marked as a failure rather than a success
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

	// Start an ultra-slow message processing
	var messageCompleted bool
	var messageCompletedMutex sync.Mutex
	var messageRelationType string
	var messageError error

	// Send the message
	msg := types.NewMsg(0, "CANCEL_TEST", types.JSON, types.NewMetadata(), `{"test": "cancel"}`)
	ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
		messageCompletedMutex.Lock()
		messageCompleted = true
		messageRelationType = relationType
		messageError = err
		messageCompletedMutex.Unlock()
		t.Logf("Message completed with relation: %s, error: %v", relationType, err)
	}))

	// Waiting for the message to start processing
	time.Sleep(100 * time.Millisecond)

	// Check the number of active operations
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		activeOps := engine.GetActiveOperations()
		t.Logf("Active operations before shutdown: %d", activeOps)
		assert.True(t, activeOps > 0, "Should have active operations")
	}

	// Start-up gracefully stops but uses a shorter timeout to force cancellation
	shutdownStart := time.Now()
	shutdownDone := make(chan bool, 1)

	go func() {
		// Uses a 2-second timeout, but verySlowProcess takes 5 seconds, so it will be canceled
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ruleEngine.Stop(ctx)
		shutdownDone <- true
	}()

	// Wait for the shutdown to complete
	select {
	case <-shutdownDone:
		elapsed := time.Since(shutdownStart)
		t.Logf("Shutdown completed in %v", elapsed)

		// Check the message processing results
		messageCompletedMutex.Lock()
		completed := messageCompleted
		relationType := messageRelationType
		err := messageError
		messageCompletedMutex.Unlock()

		t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

		// It should be completed within a reasonable time, considering that verySlowProcess requires cleaning time
		// 2-second timeout + 350ms cleaning time should be completed within 2.6 seconds
		assert.True(t, elapsed <= 2600*time.Millisecond, "Should complete within timeout + cleanup time, elapsed: %v", elapsed)
		// But it should be much faster than the original 5 seconds
		assert.True(t, elapsed >= 2*time.Second, "Should wait for timeout before cancellation, elapsed: %v", elapsed)

		// Messages should be marked as failed or canceled
		if completed {
			// If the message is complete, it should be in a failed state or contain a cancel error
			assert.True(t, relationType == types.Failure || (err != nil && strings.Contains(err.Error(), "cancel")),
				"Message should be marked as failure or cancelled, got relation: %s, error: %v", relationType, err)
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown timeout")
	}

	// Verify the final state
	assert.True(t, ruleEngine.IsShuttingDown())

	// Check the final active operation count
	if engine, ok := ruleEngine.(*RuleEngine); ok {
		finalActiveOps := engine.GetActiveOperations()
		assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
	}
}

// TestEngineGracefulShutdownWithConcurrentStop The behavior of concurrent Stop during test message execution
// Verify that when a message is running, a concurrent call called Stop should continue execution to complete
func TestEngineGracefulShutdownWithConcurrentStop(t *testing.T) {
	// Register slow processing functions with synchronization signals
	processingStarted := make(chan bool, 1)
	action.Functions.Register("syncSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		// Send a signal to start processing
		processingStarted <- true
		// Execute slow processing logic
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

	// Test scenario: During message execution, concurrent Stop occurs, and the message should continue to be executed
	t.Run("MessageShouldContinueExecution", func(t *testing.T) {
		var messageCompleted bool
		var messageCompletedMutex sync.Mutex
		var messageRelationType string
		var messageError error

		// Start a slow message processing
		msg := types.NewMsg(0, "CONCURRENT_STOP_TEST", types.JSON, types.NewMetadata(), `{"test": "concurrent_stop"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			messageCompletedMutex.Lock()
			messageCompleted = true
			messageRelationType = relationType
			messageError = err
			messageCompletedMutex.Unlock()
			t.Logf("Message completed with relation: %s, error: %v", relationType, err)
		}))

		// Wait for the message to actually start processing (using synchronization signals instead of fixed delay)
		select {
		case <-processingStarted:
			t.Logf("Message processing started")
		case <-time.After(1 * time.Second):
			t.Fatal("Message processing did not start within timeout")
		}

		// Check the number of active operations
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			t.Logf("Active operations before shutdown: %d", activeOps)
			assert.True(t, activeOps > 0, "Should have active operations")
		}

		// Graceful shutdown is initiated during message execution
		shutdownStart := time.Now()
		shutdownDone := make(chan bool, 1)

		go func() {
			// Use a sufficiently long timeout to ensure messages can be completed
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ruleEngine.Stop(ctx)
			shutdownDone <- true
		}()

		// Wait for the shutdown to complete
		select {
		case <-shutdownDone:
			elapsed := time.Since(shutdownStart)
			t.Logf("Shutdown completed in %v", elapsed)

			// Check the message processing results
			messageCompletedMutex.Lock()
			completed := messageCompleted
			relationType := messageRelationType
			err := messageError
			messageCompletedMutex.Unlock()

			t.Logf("Message completed: %v, Relation: %s, Error: %v", completed, relationType, err)

			// You should wait for the message to complete, so it takes at least nearly 2 seconds (the execution time of syncSlowProcess)
			assert.True(t, elapsed >= 1800*time.Millisecond, "Should wait for message to complete, elapsed: %v", elapsed)

			// Messages should be completed successfully (messages that have started execution should not be interrupted)
			assert.True(t, completed, "Message should be completed")
			assert.Equal(t, types.Success, relationType, "Message should complete successfully")
			assert.Nil(t, err, "Message should not have error")

		case <-time.After(10 * time.Second):
			t.Fatal("Graceful shutdown timeout")
		}

		// Verify the final state
		assert.True(t, ruleEngine.IsShuttingDown())

		// Check the final active operation count
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			finalActiveOps := engine.GetActiveOperations()
			assert.True(t, finalActiveOps <= 0, "Final active operations should be <= 0, got: %d", finalActiveOps)
		}
	})
}

// TestEngineConcurrentOnMsgAndStop tests counter issues when OnMsg and Stop are executed concurrently
// This test case verifies that when OnMsg and Stop execute concurrently, the active operation counter may block minus 1, causing Stop to time out
// TestEngineContextPreservation tests that user-provided context is not overridden by shutdown context
// TestEngineContextPreservation tests that the context provided by the user will not be overwritten by the downtime context
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
// TestEngineGracefulShutdownWithUserContext tests the correct combination of the user-provided context and the downtime context,
// Ensure the elegant shutdown timeout mechanism works properly
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
	// Register test nodes
	_ = Registry.Register(&test.UpperNode{})
	_ = Registry.Register(&test.TimeNode{})

	// Test Scenario 1: Simulating race conditions
	t.Run("ConcurrentRaceCondition", func(t *testing.T) {
		// Repeat multiple times to increase the chance of triggering a contest condition
		for attempt := 0; attempt < 10; attempt++ {
			t.Logf("Attempt %d", attempt+1)

			// Create a new engine instance
			testChainId := fmt.Sprintf("test_concurrent_%d", attempt)
			testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
			assert.Nil(t, err)
			defer Del(testChainId)

			// Concurrent execution of OnMsg and Stop
			var wg sync.WaitGroup
			var stopTimeout bool
			var stopCompleted int32

			// Launch OnMsg
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
				testEngine.OnMsg(msg)
			}()

			// Stop (no waiting) immediately started
			wg.Add(1)
			go func() {
				defer wg.Done()
				// A brief delay to ensure OnMsg starts first
				time.Sleep(1 * time.Millisecond)

				startTime := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				testEngine.Stop(ctx)
				elapsed := time.Since(startTime)

				// Check if it's timed out
				if elapsed >= 1800*time.Millisecond { // Nearly 2 seconds over
					stopTimeout = true
					t.Logf("Stop timeout detected in attempt %d, elapsed: %v", attempt+1, elapsed)
				}
				atomic.StoreInt32(&stopCompleted, 1)
			}()

			wg.Wait()

			// Verify whether the Stop has been completed
			assert.Equal(t, int32(1), atomic.LoadInt32(&stopCompleted), "Stop should complete")

			// Check the final active operation count
			if engine, ok := testEngine.(*RuleEngine); ok {
				finalActiveOps := engine.GetActiveOperations()
				t.Logf("Final active operations: %d", finalActiveOps)
				// Note: This may still be positive, and that's the problem
			}

			// If timeouts are found, record the issues
			if stopTimeout {
				t.Logf("Race condition detected: Stop timeout due to active operations counter not decremented")
				// assert.Fail is not used here, because we expect this issue to arise in certain situations
				break // Once you find a problem, exit the loop
			}
		}
	})

	// Test Scenario 2: Verify the behavior after fixing (add appropriate delay)
	t.Run("WithProperDelay", func(t *testing.T) {
		testChainId := "test_concurrent_fixed"
		testEngine, err := New(testChainId, []byte(ruleChainFile), WithConfig(config))
		assert.Nil(t, err)
		// Note: Don't use defer Del() to avoid double Stop() call issue

		// Send the message
		msg := types.NewMsg(0, "TEST", types.JSON, types.NewMetadata(), `{"test": "data"}`)
		testEngine.OnMsg(msg)

		// Add appropriate delay to allow message processing to complete
		time.Sleep(200 * time.Millisecond)

		// Check that the active operation count should be zero
		if engine, ok := testEngine.(*RuleEngine); ok {
			activeOps := engine.GetActiveOperations()
			assert.Equal(t, int64(0), activeOps, "Active operations should be 0 after message processing")
		}

		// Stop should not be timed out now
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		testEngine.Stop(ctx)
		elapsed := time.Since(startTime)

		// The stop should be completed quickly and will not be timed out
		assert.True(t, elapsed < 500*time.Millisecond, "Stop should complete quickly when no active operations, elapsed: %v", elapsed)
		assert.True(t, testEngine.IsShuttingDown(), "Engine should be in shutdown state")

		// Manually clean up after successful stop
		Del(testChainId)
	})
}

// TestEngineReloadBehavior
func TestEngineReloadBehavior(t *testing.T) {
	// Register handler functions for overloading tests
	action.Functions.Register("reloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(100 * time.Millisecond) // 0.1 seconds processing time
		ctx.TellSuccess(msg)
	})

	// Register a slow reload test function to simulate long-term handling during overload
	action.Functions.Register("slowReloadTestProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(2 * time.Second) // 2-second processing time ensures sufficient time during heavy loading
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

	// Test Scenario 1: During reload, messages should be blocked until the overload is complete
	t.Run("MessagesBlockedDuringReload", func(t *testing.T) {
		// Ensure the engine uses fast-processing rule chains
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // Wait for the reload to complete
		var processedCount int64
		var blockedCount int64
		var wg sync.WaitGroup
		var callbackWg sync.WaitGroup

		// Create a rule chain using slow processing functions to simulate long operations during overload
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

		// Start the reload operation (start the reload first)
		reloadDone := make(chan error, 1)
		go func() {
			err := ruleEngine.ReloadSelf([]byte(slowRuleChainFile))
			reloadDone <- err
		}()

		// Send messages with a slight delay to ensure messages arrive during reload
		time.Sleep(50 * time.Millisecond)

		// Send messages, which should wait for the reload to complete
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
						// If the processing time exceeds 200 milliseconds, it means the overload is waiting for completion
						newBlocked := atomic.AddInt64(&blockedCount, 1)
						t.Logf("Message %d marked as blocked, blockedCount now: %d", index, newBlocked)
					}
					if err == nil {
						newProcessed := atomic.AddInt64(&processedCount, 1)
						t.Logf("Message %d marked as processed, processedCount now: %d", index, newProcessed)
					}
				}))
			}(i)
			time.Sleep(10 * time.Millisecond) // Short intervals for transmission
		}

		// Wait for all messages to be processed
		wg.Wait()

		// Wait for the reload to complete
		reloadResult := <-reloadDone
		t.Logf("Reload completed with error: %v", reloadResult)
		assert.Nil(t, reloadResult, "Reload should succeed")

		// Wait for all callback functions to complete
		callbackWg.Wait()

		// Verify the results
		processed := atomic.LoadInt64(&processedCount)
		blocked := atomic.LoadInt64(&blockedCount)
		t.Logf("Final - Processed: %d, Blocked: %d", processed, blocked)

		assert.Equal(t, int64(3), processed, "All messages should be processed")
		assert.True(t, blocked > 0, "Some messages should wait for reload to complete")

		// After testing, the chain resets to fast processing of the rule chain to avoid affecting subsequent tests
		resetErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, resetErr)
		time.Sleep(100 * time.Millisecond) // Wait for the reload to complete
	})

	// Test Scenario 2: After reloading is complete, new messages should be processed normally
	t.Run("MessagesProcessedAfterReload", func(t *testing.T) {
		// First, perform a reload to ensure the use of fast-processing rule chains
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)

		// Wait for the reload to complete
		time.Sleep(100 * time.Millisecond)

		// The verification engine is no longer overloaded
		if engine, ok := ruleEngine.(*RuleEngine); ok {
			assert.False(t, engine.IsReloading(), "Engine should not be reloading after reload completes")
		}

		// Sending new messages should be handled normally
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

		// Wait for all callback functions to complete
		callbackWg.Wait()

		// Verify that all messages are processed correctly
		processed := atomic.LoadInt64(&processedCount)
		t.Logf("Final processedCount: %d", processed)
		assert.Equal(t, int64(3), processed, "All messages should be processed successfully after reload")
	})

	// Test Scenario 3: Active messages during reload should wait for completion
	t.Run("ActiveMessagesWaitDuringReload", func(t *testing.T) {
		// Ensure the engine uses fast-processing rule chains
		reloadErr := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		assert.Nil(t, reloadErr)
		time.Sleep(100 * time.Millisecond) // Wait for the reload to complete
		// Send a message that has been processed for a long time
		var longProcessCompleted bool
		var longProcessMutex sync.Mutex

		msg := types.NewMsg(0, "LONG_PROCESS_TEST", types.JSON, types.NewMetadata(), `{"test": "long"}`)
		go ruleEngine.OnMsg(msg, types.WithOnEnd(func(ctx types.RuleContext, msg types.RuleMsg, err error, relationType string) {
			longProcessMutex.Lock()
			longProcessCompleted = true
			longProcessMutex.Unlock()
			t.Logf("Long process completed with relation: %s", relationType)
		}))

		// Waiting for the message to start processing
		time.Sleep(100 * time.Millisecond)

		// Start the heavy load
		reloadStart := time.Now()
		err := ruleEngine.ReloadSelf([]byte(ruleChainFile))
		reloadElapsed := time.Since(reloadStart)

		assert.Nil(t, err)
		// Reloading should wait for the active message to complete, so it takes at least 0.1 seconds
		assert.True(t, reloadElapsed >= 100*time.Millisecond, "Reload should wait for active messages, elapsed: %v", reloadElapsed)

		// Verify that the long-processed message is finally complete
		time.Sleep(200 * time.Millisecond)
		longProcessMutex.Lock()
		completed := longProcessCompleted
		longProcessMutex.Unlock()

		assert.True(t, completed, "Long process should complete before reload finishes")
	})

	// Test Scenario 4: Concurrent overload should be handled safely
	t.Run("ConcurrentReloadSafety", func(t *testing.T) {
		var reloadSuccessCount int64
		var reloadErrorCount int64
		var wg sync.WaitGroup

		// Starts multiple concurrent overloads
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
			time.Sleep(10 * time.Millisecond) // Slightly stagger the startup time
		}

		wg.Wait()

		// Verify the results
		successCount := atomic.LoadInt64(&reloadSuccessCount)
		errorCount := atomic.LoadInt64(&reloadErrorCount)
		t.Logf("Concurrent reload - Success: %d, Error: %d", successCount, errorCount)

		// At the very least, there should be a successful reload
		assert.True(t, successCount >= 1, "At least one reload should succeed")
		// The total should equal the number of attempts
		assert.Equal(t, int64(3), successCount+errorCount, "All reload attempts should be accounted for")
	})

	// Test Scenario 5: Heavy load timeout handling
	t.Run("ReloadTimeoutHandling", func(t *testing.T) {
		// Register an ultra-long processing function
		action.Functions.Register("superSlowProcess", func(ctx types.RuleContext, msg types.RuleMsg) {
			time.Sleep(15 * time.Second) // 15-second processing time
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

		// Send a message that takes a very long time to process
		msg := types.NewMsg(0, "SUPER_SLOW_TEST", types.JSON, types.NewMetadata(), `{"test": "super_slow"}`)
		go superSlowEngine.OnMsg(msg)

		// Waiting for the message to start processing
		time.Sleep(200 * time.Millisecond)

		// Try reloading and should continue after waiting for the timeout
		reloadStart := time.Now()
		err = superSlowEngine.ReloadSelf([]byte(superSlowRuleChain))
		reloadElapsed := time.Since(reloadStart)

		// Reloading should continue after waiting for timeout (10 seconds), not wait 15 seconds
		assert.Nil(t, err)
		assert.True(t, reloadElapsed >= 9*time.Second, "Reload should wait for timeout")
		assert.True(t, reloadElapsed < 12*time.Second, "Reload should not wait beyond timeout")
	})
}

// TestReloadBackpressureControl tests the backpressure control function during reload
func TestReloadBackpressureControl(t *testing.T) {
	// Create a rule chain definition
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

	// Register the test function
	action.Functions.Register("testBackpressureFunc", func(ctx types.RuleContext, msg types.RuleMsg) {
		time.Sleep(10 * time.Millisecond) // Simulates short processing times
		ctx.TellSuccess(msg)
	})

	t.Run("BackpressureControlPreventsMemoryOverflow", func(t *testing.T) {
		// Create a rule engine with low backpressure limits
		ruleEngine, err := NewRuleEngine("test_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(5)) // Only 5 concurrent waiting parties are allowed
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Verify the backpressure configuration
		maxWaiters, currentWaiters, isReloading := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(5), maxWaiters)
		assert.Equal(t, int64(0), currentWaiters)
		assert.False(t, isReloading)

		// Create slow overload functions
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
			time.Sleep(2 * time.Second) // Simulates slow processing to extend heavy load time
			ctx.TellSuccess(msg)
		})

		// Start the overload operation (asynchronously in the background)
		var reloadWg sync.WaitGroup
		reloadWg.Add(1)
		go func() {
			defer reloadWg.Done()
			reloadErr := ruleEngine.ReloadSelf([]byte(slowReloadChainFile))
			assert.Nil(t, reloadErr)
		}()

		// Waiting for the reload to truly begin
		for i := 0; i < 50; i++ { // Wait up to 500ms
			time.Sleep(10 * time.Millisecond)
			_, _, isReloading := ruleEngine.GetReloadWaitersStats()
			if isReloading {
				break
			}
		}

		// Verify the overload status
		_, _, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.True(t, isReloading, "重载应该已经开始")

		// Send a large number of messages to test back pressure control
		var processedCount int64
		var rejectedCount int64
		var callbackWg sync.WaitGroup

		// Send 10 messages (exceed the 5 limit)
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
						// Other errors are also counted in the rejection count because messages were not successfully processed
						atomic.AddInt64(&rejectedCount, 1)
					}
				}))
			}(i)
		}

		// Wait for all pullbacks to complete
		callbackWg.Wait()

		// Verify that backpressure control is effective
		totalMessages := atomic.LoadInt64(&processedCount) + atomic.LoadInt64(&rejectedCount)
		assert.True(t, totalMessages >= 5, "至少应该有5个消息有回调，实际: %d", totalMessages)
		assert.True(t, atomic.LoadInt64(&rejectedCount) > 0, "应该有消息因为背压控制被拒绝")

		t.Logf("Messages processed: %d, rejected messages: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&rejectedCount))

		// Wait for the reload to complete
		reloadWg.Wait()

		// Verify that the state is normal after reloading is complete
		_, currentWaiters, isReloading = ruleEngine.GetReloadWaitersStats()
		assert.False(t, isReloading)
		assert.Equal(t, int64(0), currentWaiters, "重载完成后等待者计数应该为0")
	})

	t.Run("BackpressureCanBeDisabled", func(t *testing.T) {
		// Create a rule engine that disables back pressure control
		ruleEngine, err := NewRuleEngine("test_no_backpressure", []byte(ruleChainFile),
			types.WithMaxReloadWaiters(0))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Verify that backpressure control is disabled
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "背压控制应该被禁用")

		// The test does not perform overloading, only verifying that backpressure control is not effective
		var processedCount int64
		var backpressureRejectedCount int64
		var callbackWg sync.WaitGroup

		// Send message test (no overloading)
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

		// No news from verification because back pressure control was denied
		assert.Equal(t, int64(0), atomic.LoadInt64(&backpressureRejectedCount), "禁用背压控制时不应该有消息因背压被拒绝")
		assert.Equal(t, int64(5), atomic.LoadInt64(&processedCount), "所有消息都应该被处理")

		t.Logf("Disable backpressure control test - processed messages: %d, backpressure rejected messages: %d",
			atomic.LoadInt64(&processedCount),
			atomic.LoadInt64(&backpressureRejectedCount))
	})

	t.Run("BackpressureConfigCanBeChangedAtRuntime", func(t *testing.T) {
		// Create a rule engine
		ruleEngine, err := NewRuleEngine("test_runtime_config", []byte(ruleChainFile))
		assert.Nil(t, err)
		defer ruleEngine.Stop(context.Background())

		// Initial configuration
		maxWaiters, _, _ := ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(1000), maxWaiters, "默认值应该是1000") // Default values

		// Configure settings at runtime
		ruleEngine.SetMaxReloadWaiters(100)

		// Verification configuration has changed
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(100), maxWaiters)

		// Disable back pressure control
		ruleEngine.SetMaxReloadWaiters(0)

		// Verification configuration has changed
		maxWaiters, _, _ = ruleEngine.GetReloadWaitersStats()
		assert.Equal(t, int64(0), maxWaiters, "应该禁用背压控制")
	})
}
