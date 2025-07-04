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
	"runtime"
	"testing"
	"time"

	"github.com/rulego/rulego/api/types"
)

// getGoroutineCount returns the current number of goroutines
func getGoroutineCount() int {
	return runtime.NumGoroutine()
}

// waitForGoroutines waits for a short time to allow goroutines to start/stop
func waitForGoroutines() {
	time.Sleep(50 * time.Millisecond)
	runtime.Gosched() // Give scheduler a chance to run
}

// TestCombineContextsGoroutineLeak tests for goroutine leaks in combineContexts method
func TestCombineContextsGoroutineLeak(t *testing.T) {
	// Create a simple rule chain for testing
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Test Node",
					"configuration": {
						"jsScript": "return true;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		t.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	t.Run("UserContextTimeout_ShouldNotLeakGoroutines", func(t *testing.T) {
		// Record initial goroutine count
		initialCount := getGoroutineCount()
		waitForGoroutines()

		// Create multiple contexts with timeouts
		numTests := 10
		for i := 0; i < numTests; i++ {
			// Create user context with very short timeout
			userCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)

			// Create shutdown context (won't be cancelled in this test)
			shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

			// Call combineContexts to create the monitoring goroutine
			combinedCtx := engine.combineContexts(userCtx, shutdownCtx)

			// Wait for timeout to occur
			<-userCtx.Done()

			// Wait a bit more to ensure the monitoring goroutine exits
			waitForGoroutines()

			// Check that combinedCtx is also cancelled
			select {
			case <-combinedCtx.Done():
				// Expected - combinedCtx should be cancelled when userCtx times out
			default:
				t.Errorf("Combined context should be cancelled when user context times out")
			}

			// Clean up
			cancel()
			shutdownCancel()
		}

		// Wait for all goroutines to finish
		waitForGoroutines()
		time.Sleep(100 * time.Millisecond)

		// Check final goroutine count
		finalCount := getGoroutineCount()

		// Allow some tolerance for test runner goroutines
		leakedGoroutines := finalCount - initialCount
		if leakedGoroutines > 2 { // Allow small tolerance for test infrastructure
			t.Errorf("Potential goroutine leak detected: initial=%d, final=%d, leaked=%d",
				initialCount, finalCount, leakedGoroutines)
		}
	})

	t.Run("UserContextManualCancel_ShouldNotLeakGoroutines", func(t *testing.T) {
		// Record initial goroutine count
		initialCount := getGoroutineCount()
		waitForGoroutines()

		// Create multiple contexts and cancel them manually
		numTests := 10
		for i := 0; i < numTests; i++ {
			// Create user context
			userCtx, userCancel := context.WithCancel(context.Background())

			// Create shutdown context (won't be cancelled in this test)
			shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

			// Call combineContexts to create the monitoring goroutine
			combinedCtx := engine.combineContexts(userCtx, shutdownCtx)

			// Manually cancel user context
			userCancel()

			// Wait for the monitoring goroutine to detect cancellation
			waitForGoroutines()

			// Check that combinedCtx is also cancelled
			select {
			case <-combinedCtx.Done():
				// Expected - combinedCtx should be cancelled when userCtx is cancelled
			default:
				t.Errorf("Combined context should be cancelled when user context is cancelled")
			}

			// Clean up
			shutdownCancel()
		}

		// Wait for all goroutines to finish
		waitForGoroutines()
		time.Sleep(100 * time.Millisecond)

		// Check final goroutine count
		finalCount := getGoroutineCount()

		// Allow some tolerance for test runner goroutines
		leakedGoroutines := finalCount - initialCount
		if leakedGoroutines > 2 { // Allow small tolerance for test infrastructure
			t.Errorf("Potential goroutine leak detected: initial=%d, final=%d, leaked=%d",
				initialCount, finalCount, leakedGoroutines)
		}
	})

	t.Run("ShutdownContext_ShouldNotLeakGoroutines", func(t *testing.T) {
		// Record initial goroutine count
		initialCount := getGoroutineCount()
		waitForGoroutines()

		// Create multiple contexts and cancel shutdown context
		numTests := 10
		for i := 0; i < numTests; i++ {
			// Create user context (won't be cancelled in this test)
			userCtx, userCancel := context.WithCancel(context.Background())

			// Create shutdown context
			shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

			// Call combineContexts to create the monitoring goroutine
			combinedCtx := engine.combineContexts(userCtx, shutdownCtx)

			// Cancel shutdown context (simulating engine shutdown)
			shutdownCancel()

			// Wait for the monitoring goroutine to detect cancellation
			waitForGoroutines()

			// Check that combinedCtx is also cancelled
			select {
			case <-combinedCtx.Done():
				// Expected - combinedCtx should be cancelled when shutdownCtx is cancelled
			default:
				t.Errorf("Combined context should be cancelled when shutdown context is cancelled")
			}

			// Clean up
			userCancel()
		}

		// Wait for all goroutines to finish
		waitForGoroutines()
		time.Sleep(100 * time.Millisecond)

		// Check final goroutine count
		finalCount := getGoroutineCount()

		// Allow some tolerance for test runner goroutines
		leakedGoroutines := finalCount - initialCount
		if leakedGoroutines > 2 { // Allow small tolerance for test infrastructure
			t.Errorf("Potential goroutine leak detected: initial=%d, final=%d, leaked=%d",
				initialCount, finalCount, leakedGoroutines)
		}
	})
}

// TestRealWorldScenario_MessageProcessingWithTimeout tests a real-world scenario
func TestRealWorldScenario_MessageProcessingWithTimeout(t *testing.T) {
	// Create a rule chain that takes some time to process
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Slow Node",
					"configuration": {
						"jsScript": "var start = Date.now(); while(Date.now() - start < 100) {} return true;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		t.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	// Record initial goroutine count
	initialCount := getGoroutineCount()
	waitForGoroutines()

	// Simulate multiple message processing with user-defined timeouts
	numMessages := 20
	for i := 0; i < numMessages; i++ {
		// Create message
		msg := types.NewMsgWithJsonData(`{"type": "test_msg", "data": "TestData"}`)

		// Create user context with timeout shorter than processing time
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)

		// Process message with timeout context (will timeout before completion)
		go func() {
			engine.OnMsg(msg, types.WithContext(ctx))
		}()

		// Wait for timeout
		<-ctx.Done()
		cancel()

		// Small delay between messages
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all processing to complete
	time.Sleep(500 * time.Millisecond)
	waitForGoroutines()

	// Check final goroutine count
	finalCount := getGoroutineCount()

	// In a real scenario, we should not have significant goroutine accumulation
	leakedGoroutines := finalCount - initialCount
	if leakedGoroutines > 5 { // Allow some tolerance for async processing
		t.Errorf("Potential goroutine leak in real-world scenario: initial=%d, final=%d, leaked=%d",
			initialCount, finalCount, leakedGoroutines)
	}

	t.Logf("Real-world test completed: initial goroutines=%d, final goroutines=%d, difference=%d",
		initialCount, finalCount, leakedGoroutines)
}

// TestGoroutineLeakStressTest performs a stress test to detect goroutine leaks
func TestGoroutineLeakStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Create a simple rule chain
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Test Node",
					"configuration": {
						"jsScript": "return true;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		t.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	// Record initial goroutine count
	initialCount := getGoroutineCount()
	t.Logf("Initial goroutine count: %d", initialCount)

	// Stress test: create many contexts with timeouts
	numIterations := 1000
	for i := 0; i < numIterations; i++ {
		// Create user context with very short timeout
		userCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)

		// Create shutdown context
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

		// Call combineContexts
		_ = engine.combineContexts(userCtx, shutdownCtx)

		// Wait for timeout
		<-userCtx.Done()

		// Clean up
		cancel()
		shutdownCancel()

		// Periodically check goroutine count
		if i%100 == 0 {
			currentCount := getGoroutineCount()
			t.Logf("Iteration %d: goroutine count = %d", i, currentCount)

			// If goroutines are accumulating rapidly, fail early
			if currentCount > initialCount+50 {
				t.Fatalf("Goroutines accumulating too rapidly at iteration %d: current=%d, initial=%d",
					i, currentCount, initialCount)
			}
		}
	}

	// Final cleanup and check
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	waitForGoroutines()

	finalCount := getGoroutineCount()
	leakedGoroutines := finalCount - initialCount

	t.Logf("Stress test completed: initial=%d, final=%d, leaked=%d",
		initialCount, finalCount, leakedGoroutines)

	// In a stress test, we should not accumulate significant goroutines
	if leakedGoroutines > 10 {
		t.Errorf("Stress test detected goroutine leak: %d goroutines leaked", leakedGoroutines)
	}
}

// BenchmarkCombineContexts benchmarks the combineContexts method
func BenchmarkCombineContexts(b *testing.B) {
	// Create a simple rule chain
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Test Node",
					"configuration": {
						"jsScript": "return true;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		b.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create contexts
		userCtx, userCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

		// Call combineContexts
		combinedCtx := engine.combineContexts(userCtx, shutdownCtx)

		// Cancel user context to trigger cleanup
		userCancel()

		// Wait for context to be cancelled
		<-combinedCtx.Done()

		// Clean up
		shutdownCancel()
	}
}

// TestFixedImplementationBaseline verifies that the fixed implementation
// serves as a reliable baseline for goroutine leak prevention
func TestFixedImplementationBaseline(t *testing.T) {
	// Create a simple rule chain
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Test Node",
					"configuration": {
						"jsScript": "return true;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		t.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	t.Run("Baseline_ShouldNotLeakGoroutines", func(t *testing.T) {
		// Record initial goroutine count
		initialCount := getGoroutineCount()
		waitForGoroutines()

		numTests := 50

		// Create multiple contexts with the current implementation
		for i := 0; i < numTests; i++ {
			// Create user context with timeout
			userCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)

			// Create shutdown context (never cancelled in this test)
			shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

			// Call the combineContexts method
			combinedCtx := engine.combineContexts(userCtx, shutdownCtx)

			// Wait for timeout to occur
			<-userCtx.Done()

			// Wait for monitoring goroutine to exit
			waitForGoroutines()

			// Verify combined context is cancelled
			select {
			case <-combinedCtx.Done():
				// Expected
			default:
				t.Errorf("Combined context should be cancelled when user context times out")
			}

			// Clean up
			cancel()
			shutdownCancel()
		}

		// Wait for all goroutines to clean up
		waitForGoroutines()
		time.Sleep(200 * time.Millisecond)

		// Check goroutine count - should show no significant leaks
		fixedCount := getGoroutineCount()
		leakedGoroutines := fixedCount - initialCount

		t.Logf("Baseline implementation: initial=%d, after_test=%d, leaked=%d",
			initialCount, fixedCount, leakedGoroutines)

		// The baseline implementation should not accumulate significant goroutines
		if leakedGoroutines > 5 {
			t.Errorf("Baseline implementation still leaking goroutines: %d leaked", leakedGoroutines)
		}
	})
}

// TestDetectLeakInProductionScenario simulates a production-like scenario
func TestDetectLeakInProductionScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping production scenario test in short mode")
	}

	// Create a rule chain
	ruleChainDef := `{
		"ruleChain": {
			"id": "test_chain",
			"name": "Test Chain",
			"root": true
		},
		"metadata": {
			"nodes": [
				{
					"id": "node1",
					"type": "jsFilter",
					"name": "Fast Node",
					"configuration": {
						"jsScript": "return Math.random() > 0.5;"
					}
				}
			],
			"connections": []
		}
	}`

	engine, err := NewRuleEngine("test", []byte(ruleChainDef))
	if err != nil {
		t.Fatalf("Failed to create rule engine: %v", err)
	}
	defer engine.Stop(context.Background())

	// Record initial state
	initialCount := getGoroutineCount()
	t.Logf("Production test - initial goroutines: %d", initialCount)

	// Simulate production traffic with various timeout scenarios
	scenarios := []struct {
		name     string
		timeout  time.Duration
		messages int
	}{
		{"fast_timeout", 5 * time.Millisecond, 100},
		{"medium_timeout", 50 * time.Millisecond, 50},
		{"slow_timeout", 200 * time.Millisecond, 25},
	}

	for _, scenario := range scenarios {
		t.Logf("Running scenario: %s", scenario.name)

		for i := 0; i < scenario.messages; i++ {
			// Create message
			msg := types.NewMsgWithJsonData(fmt.Sprintf(`{"type": "prod_test", "scenario": "%s", "index": %d}`, scenario.name, i))

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), scenario.timeout)

			// Process message
			go func() {
				defer cancel()
				engine.OnMsg(msg, types.WithContext(ctx))
			}()

			// Small delay to simulate real traffic
			time.Sleep(2 * time.Millisecond)
		}

		// Wait for scenario to complete
		time.Sleep(scenario.timeout + 100*time.Millisecond)

		// Check intermediate state
		currentCount := getGoroutineCount()
		t.Logf("After scenario %s: goroutines = %d", scenario.name, currentCount)
	}

	// Final assessment
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	waitForGoroutines()

	finalCount := getGoroutineCount()
	totalLeaked := finalCount - initialCount

	t.Logf("Production test completed:")
	t.Logf("  Initial goroutines: %d", initialCount)
	t.Logf("  Final goroutines: %d", finalCount)
	t.Logf("  Total leaked: %d", totalLeaked)

	// In a production scenario, we should have minimal leakage
	if totalLeaked > 15 {
		t.Errorf("Production scenario detected significant goroutine leak: %d goroutines", totalLeaked)
	} else {
		t.Logf("✅ Production scenario passed with acceptable goroutine usage")
	}
}
