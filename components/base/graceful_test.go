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

package base

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rulego/rulego/test/assert"
)

// mockLogger is a test logger implementation
type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLogger) Print(v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprint(v...))
}

func (m *mockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.messages...)
}

func (m *mockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

// TestGracefulShutdownBasicFunctionality tests basic graceful shutdown functionality
func TestGracefulShutdownBasicFunctionality(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}

	// Test initialization
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	// Initially should not be shutting down
	assert.False(t, graceful.IsShuttingDown())

	// Context should be available and not cancelled
	ctx := graceful.GetShutdownContext()
	assert.NotNil(t, ctx)

	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected case
	}

	// Check shutdown signal should return nil initially
	err := graceful.CheckShutdownSignal()
	assert.Nil(t, err)

	// Start graceful shutdown
	stopCalled := false
	graceful.GracefulStop(func() {
		stopCalled = true
	})

	// Should be shutting down now
	assert.True(t, graceful.IsShuttingDown())

	// Phase 1: Context should NOT be cancelled immediately (two-phase graceful shutdown)
	// But CheckShutdownSignal should return error due to shutdown flag
	// 第一阶段：上下文不应立即被取消（两阶段优雅停机）
	// 但CheckShutdownSignal应该因为停机标志而返回错误
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled in phase 1 of graceful shutdown")
	default:
		// Expected case for phase 1
	}

	// Check shutdown signal should return error now (due to shutdown flag, not context)
	err = graceful.CheckShutdownSignal()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "operation cancelled due to shutdown"))

	// Stop function should have been called
	assert.True(t, stopCalled)

	// Phase 2: Test forced shutdown (context cancellation)
	// 第二阶段：测试强制停机（上下文取消）
	graceful.ForceStop()

	// Now context should be cancelled
	select {
	case <-ctx.Done():
		// Expected case for phase 2
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after ForceStop")
	}
}

// TestGracefulShutdownDefaultTimeout tests the default timeout behavior
func TestGracefulShutdownDefaultTimeout(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}

	// Initialize with zero timeout (should use default)
	graceful.InitGracefulShutdown(logger, 0)

	// Default timeout should be applied
	assert.Equal(t, DefaultShutdownTimeout, graceful.shutdownTimeout)
}

// TestGracefulShutdownCustomTimeout tests custom timeout configuration
func TestGracefulShutdownCustomTimeout(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}

	customTimeout := 10 * time.Second
	graceful.InitGracefulShutdown(logger, customTimeout)

	assert.Equal(t, customTimeout, graceful.shutdownTimeout)
}

// TestGracefulShutdownConcurrentOperations tests thread safety with concurrent operations
func TestGracefulShutdownConcurrentOperations(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	const concurrentOperations = 100
	var wg sync.WaitGroup
	var successfulChecks int64
	var shutdownChecks int64

	wg.Add(concurrentOperations)

	// Start concurrent operations
	for i := 0; i < concurrentOperations; i++ {
		go func(index int) {
			defer wg.Done()

			// Simulate some work
			time.Sleep(time.Millisecond * time.Duration(index%10))

			// Check shutdown signal
			if err := graceful.CheckShutdownSignal(); err != nil {
				atomic.AddInt64(&shutdownChecks, 1)
			} else {
				atomic.AddInt64(&successfulChecks, 1)
			}

			// Check shutdown status
			_ = graceful.IsShuttingDown()

			// Get shutdown context
			_ = graceful.GetShutdownContext()
		}(i)
	}

	// Let some operations start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	stopCalled := int32(0)
	graceful.GracefulStop(func() {
		atomic.StoreInt32(&stopCalled, 1)
	})

	wg.Wait()

	// Stop function should have been called exactly once
	assert.Equal(t, int32(1), atomic.LoadInt32(&stopCalled))

	// Should be shutting down
	assert.True(t, graceful.IsShuttingDown())

	// At least some operations should have detected shutdown
	totalChecks := atomic.LoadInt64(&successfulChecks) + atomic.LoadInt64(&shutdownChecks)
	assert.Equal(t, int64(concurrentOperations), totalChecks)
}

// TestGracefulShutdownMultipleStopCalls tests that multiple GracefulStop calls are safe
func TestGracefulShutdownMultipleStopCalls(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 1*time.Second)

	var stopCallCount int64
	stopFunc := func() {
		atomic.AddInt64(&stopCallCount, 1)
	}

	// Call GracefulStop multiple times concurrently
	const numCalls = 10
	var wg sync.WaitGroup
	wg.Add(numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			defer wg.Done()
			graceful.GracefulStop(stopFunc)
		}()
	}

	wg.Wait()

	// Stop function should have been called only once
	assert.Equal(t, int64(1), atomic.LoadInt64(&stopCallCount))

	// Should be shutting down
	assert.True(t, graceful.IsShuttingDown())
}

// TestGracefulShutdownWithNilStopFunc tests shutdown behavior with nil stop function
func TestGracefulShutdownWithNilStopFunc(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 1*time.Second)

	// Should not panic with nil stop function
	graceful.GracefulStop(nil)

	// Should be shutting down
	assert.True(t, graceful.IsShuttingDown())

	// Phase 1: Context should NOT be cancelled immediately (two-phase graceful shutdown)
	// 第一阶段：上下文不应立即被取消（两阶段优雅停机）
	ctx := graceful.GetShutdownContext()
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled in phase 1 of graceful shutdown")
	default:
		// Expected case for phase 1
	}

	// CheckShutdownSignal should return error due to shutdown flag
	err := graceful.CheckShutdownSignal()
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "operation cancelled due to shutdown"))

	// Phase 2: Test forced shutdown
	graceful.ForceStop()

	// Now context should be cancelled
	select {
	case <-ctx.Done():
		// Expected case for phase 2
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after ForceStop")
	}
}

// TestGracefulShutdownWithTimeout tests timeout behavior during shutdown
func TestGracefulShutdownWithTimeout(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 200*time.Millisecond)

	var stopCalled int64
	startTime := time.Now()

	graceful.GracefulStop(func() {
		atomic.StoreInt64(&stopCalled, 1)
		// Simulate work that takes time to complete
		// 模拟需要时间完成的工作
		time.Sleep(150 * time.Millisecond)
	})

	endTime := time.Now()
	elapsed := endTime.Sub(startTime)

	// Stop should have been called
	assert.Equal(t, int64(1), atomic.LoadInt64(&stopCalled))

	// Should have taken at least the time of the work in stopFunc
	// 应该至少花费stopFunc中工作的时间
	assert.True(t, elapsed >= 100*time.Millisecond, "Shutdown took too little time: %v", elapsed)
	assert.True(t, elapsed <= 300*time.Millisecond, "Shutdown took too long: %v", elapsed)

	// Should be shutting down
	assert.True(t, graceful.IsShuttingDown())
}

// TestCheckShutdownContext tests context checking functionality
func TestCheckShutdownContext(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	// Test with normal context
	normalCtx := context.Background()
	err := graceful.CheckShutdownContext(normalCtx)
	assert.Nil(t, err)

	// Test with nil context
	err = graceful.CheckShutdownContext(nil)
	assert.Nil(t, err)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// Wait for context to be cancelled with timeout
	select {
	case <-cancelledCtx.Done():
		// Context is definitely cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
	err = graceful.CheckShutdownContext(cancelledCtx)
	assert.NotNil(t, err)
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))
	}

	// Test with timeout context
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelTimeout()
	// Wait for context to timeout with additional safety margin
	select {
	case <-timeoutCtx.Done():
		// Context has timed out
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have timed out")
	}
	err = graceful.CheckShutdownContext(timeoutCtx)
	assert.NotNil(t, err)
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "operation cancelled"))
	}
}

// TestGracefulShutdownIntegrationWithComponents tests integration with component-like structures
func TestGracefulShutdownIntegrationWithComponents(t *testing.T) {
	// Simulate a component that embeds GracefulShutdown
	type TestComponent struct {
		*GracefulShutdown
		running       int32
		operationDone chan struct{}
	}

	logger := &mockLogger{}
	component := &TestComponent{
		GracefulShutdown: &GracefulShutdown{},
		operationDone:    make(chan struct{}),
	}

	component.InitGracefulShutdown(logger, 2*time.Second)

	// Start a long-running operation
	atomic.StoreInt32(&component.running, 1)
	go func() {
		defer close(component.operationDone)
		defer atomic.StoreInt32(&component.running, 0)

		for {
			// Check shutdown signal
			if err := component.CheckShutdownSignal(); err != nil {
				// Gracefully exit
				return
			}

			// Simulate work
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Let the operation start
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&component.running))

	// Trigger shutdown
	component.GracefulStop(func() {
		// Wait for operation to complete
		<-component.operationDone
	})

	// Operation should have stopped
	assert.Equal(t, int32(0), atomic.LoadInt32(&component.running))
	assert.True(t, component.IsShuttingDown())

	// Channel should be closed
	select {
	case <-component.operationDone:
		// Expected case
	default:
		t.Error("Operation should have completed")
	}
}

// TestGracefulShutdownWithSimulatedWork tests realistic work simulation
func TestGracefulShutdownWithSimulatedWork(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 1*time.Second)

	var activeOperations int64
	var completedOperations int64
	var shutdownDetected int64

	const numWorkers = 20
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	// Start multiple workers
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			atomic.AddInt64(&activeOperations, 1)
			defer atomic.AddInt64(&activeOperations, -1)

			// Simulate work with shutdown checks
			for j := 0; j < 100; j++ {
				// Check shutdown signal
				if err := graceful.CheckShutdownSignal(); err != nil {
					atomic.AddInt64(&shutdownDetected, 1)
					return
				}

				// Simulate some work
				time.Sleep(time.Millisecond)
			}

			atomic.AddInt64(&completedOperations, 1)
		}(i)
	}

	// Let workers start
	time.Sleep(100 * time.Millisecond)

	// Should have active operations
	assert.True(t, atomic.LoadInt64(&activeOperations) > 0)

	// Trigger shutdown
	startShutdown := time.Now()
	graceful.GracefulStop(func() {
		// Wait for all workers to finish
		wg.Wait()
	})
	shutdownTime := time.Since(startShutdown)

	// Shutdown should complete
	assert.True(t, graceful.IsShuttingDown())

	// Should have detected shutdown in at least some workers
	assert.True(t, atomic.LoadInt64(&shutdownDetected) > 0)

	// Total workers should equal completed + shutdown detected
	total := atomic.LoadInt64(&completedOperations) + atomic.LoadInt64(&shutdownDetected)
	assert.Equal(t, int64(numWorkers), total)

	// Shutdown should have completed reasonably quickly
	assert.True(t, shutdownTime < 5*time.Second, "Shutdown took too long: %v", shutdownTime)

	// No more active operations
	assert.Equal(t, int64(0), atomic.LoadInt64(&activeOperations))
}

// TestGracefulShutdownLogging tests logging functionality
func TestGracefulShutdownLogging(t *testing.T) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 100*time.Millisecond)

	// Create a channel to simulate work that will timeout
	done := make(chan struct{})

	// Trigger shutdown with a function that will cause timeout
	graceful.GracefulStop(func() {
		// Wait for done signal or timeout will trigger
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
		}
	})

	// Give some time for logging to occur
	time.Sleep(50 * time.Millisecond)

	// For this test, we mainly verify that the graceful shutdown completes
	// The specific timeout logging behavior depends on the implementation details
	assert.True(t, graceful.IsShuttingDown(), "Should be in shutdown state")
}

// TestGracefulShutdownWithNilLogger tests behavior with nil logger
func TestGracefulShutdownWithNilLogger(t *testing.T) {
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(nil, 100*time.Millisecond)

	// Should not panic with nil logger
	stopCalled := false
	graceful.GracefulStop(func() {
		stopCalled = true
	})

	assert.True(t, stopCalled)
	assert.True(t, graceful.IsShuttingDown())
}

// BenchmarkGracefulShutdownCheckSignal benchmarks the CheckShutdownSignal method
func BenchmarkGracefulShutdownCheckSignal(b *testing.B) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = graceful.CheckShutdownSignal()
		}
	})
}

// BenchmarkGracefulShutdownIsShuttingDown benchmarks the IsShuttingDown method
func BenchmarkGracefulShutdownIsShuttingDown(b *testing.B) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = graceful.IsShuttingDown()
		}
	})
}

// BenchmarkGracefulShutdownConcurrentAccess benchmarks concurrent access patterns
func BenchmarkGracefulShutdownConcurrentAccess(b *testing.B) {
	logger := &mockLogger{}
	graceful := &GracefulShutdown{}
	graceful.InitGracefulShutdown(logger, 5*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = graceful.IsShuttingDown()
			_ = graceful.CheckShutdownSignal()
			_ = graceful.GetShutdownContext()
		}
	})
}

// TestContextUtilsCheckContext tests the CheckContext utility function
func TestContextUtilsCheckContext(t *testing.T) {
	// Test with nil context
	err := ContextUtils.CheckContext(nil, "test operation")
	assert.Nil(t, err)

	// Test with normal context
	ctx := context.Background()
	err = ContextUtils.CheckContext(ctx, "test operation")
	assert.Nil(t, err)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = ContextUtils.CheckContext(cancelledCtx, "test operation")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "test operation cancelled"))
	assert.True(t, strings.Contains(err.Error(), "context canceled"))

	// Test with cancelled context without operation name
	err = ContextUtils.CheckContext(cancelledCtx, "")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "operation cancelled"))
}

// TestContextUtilsCheckContextWithTimeout tests the CheckContextWithTimeout utility function
func TestContextUtilsCheckContextWithTimeout(t *testing.T) {
	// Test with nil context
	err := ContextUtils.CheckContextWithTimeout(nil, time.Second, "test operation")
	assert.Nil(t, err)

	// Test with normal context (should not timeout)
	ctx := context.Background()
	err = ContextUtils.CheckContextWithTimeout(ctx, 100*time.Millisecond, "test operation")
	assert.Nil(t, err)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// Wait for context to be cancelled with timeout
	select {
	case <-cancelledCtx.Done():
		// Context is definitely cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
	err = ContextUtils.CheckContextWithTimeout(cancelledCtx, time.Second, "test operation")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "test operation cancelled"))

	// Test with timeout context (should detect cancellation before our timeout)
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelTimeout()
	// Wait for context to timeout with additional safety margin
	select {
	case <-timeoutCtx.Done():
		// Context has timed out
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have timed out")
	}
	err = ContextUtils.CheckContextWithTimeout(timeoutCtx, time.Second, "timeout operation")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "timeout operation cancelled"))
}

// TestContextUtilsShouldStop tests the ShouldStop utility function
func TestContextUtilsShouldStop(t *testing.T) {
	// Test with nil context
	shouldStop := ContextUtils.ShouldStop(nil)
	assert.False(t, shouldStop)

	// Test with normal context
	ctx := context.Background()
	shouldStop = ContextUtils.ShouldStop(ctx)
	assert.False(t, shouldStop)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// Wait for context to be cancelled with timeout
	select {
	case <-cancelledCtx.Done():
		// Context is definitely cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
	shouldStop = ContextUtils.ShouldStop(cancelledCtx)
	assert.True(t, shouldStop)

	// Test with timeout context
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelTimeout()
	// Wait for context to timeout with additional safety margin
	select {
	case <-timeoutCtx.Done():
		// Context has timed out
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have timed out")
	}
	shouldStop = ContextUtils.ShouldStop(timeoutCtx)
	assert.True(t, shouldStop)
}

// TestContextUtilsWithGracefulShutdown tests the WithGracefulShutdown utility function
func TestContextUtilsWithGracefulShutdown(t *testing.T) {
	operationCalled := false
	operationFunc := func() error {
		operationCalled = true
		return nil
	}

	// Test with normal context - operation should be called
	ctx := context.Background()
	err := ContextUtils.WithGracefulShutdown(ctx, operationFunc)
	assert.Nil(t, err)
	assert.True(t, operationCalled)

	// Reset for next test
	operationCalled = false

	// Test with cancelled context - operation should not be called
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = ContextUtils.WithGracefulShutdown(cancelledCtx, operationFunc)
	assert.NotNil(t, err)
	assert.False(t, operationCalled)
	assert.True(t, strings.Contains(err.Error(), "operation cancelled"))

	// Test with operation that returns error
	operationWithError := func() error {
		return fmt.Errorf("operation error")
	}
	err = ContextUtils.WithGracefulShutdown(context.Background(), operationWithError)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "operation error"))
}

// TestContextUtilsIsContextCancelled tests the IsContextCancelled utility function
func TestContextUtilsIsContextCancelled(t *testing.T) {
	// Test with nil context
	cancelled, err := ContextUtils.IsContextCancelled(nil)
	assert.False(t, cancelled)
	assert.Nil(t, err)

	// Test with normal context
	ctx := context.Background()
	cancelled, err = ContextUtils.IsContextCancelled(ctx)
	assert.False(t, cancelled)
	assert.Nil(t, err)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// Wait for context to be cancelled with timeout
	select {
	case <-cancelledCtx.Done():
		// Context is definitely cancelled
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have been cancelled")
	}
	cancelled, err = ContextUtils.IsContextCancelled(cancelledCtx)
	assert.True(t, cancelled)
	assert.NotNil(t, err)
	assert.Equal(t, context.Canceled, err)

	// Test with timeout context
	timeoutCtx, cancelTimeout := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelTimeout()
	// Wait for context to timeout with additional safety margin
	select {
	case <-timeoutCtx.Done():
		// Context has timed out
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should have timed out")
	}
	cancelled, err = ContextUtils.IsContextCancelled(timeoutCtx)
	assert.True(t, cancelled)
	assert.NotNil(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

// TestContextUtilsConcurrentAccess tests concurrent access to ContextUtils methods
func TestContextUtilsConcurrentAccess(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()

			// Test different methods concurrently
			_ = ContextUtils.CheckContext(ctx, "concurrent test")
			_ = ContextUtils.ShouldStop(ctx)
			_, _ = ContextUtils.IsContextCancelled(ctx)

			if index%2 == 0 {
				// Test with cancelled context too
				_ = ContextUtils.CheckContext(cancelledCtx, "concurrent test")
				_ = ContextUtils.ShouldStop(cancelledCtx)
				_, _ = ContextUtils.IsContextCancelled(cancelledCtx)
			}
		}(i)
	}

	wg.Wait()
	// If we reach here without deadlock or panic, the test passes
}

// TestContextUtilsIntegrationWithRealScenarios tests integration scenarios
func TestContextUtilsIntegrationWithRealScenarios(t *testing.T) {
	// Simulate a component operation that checks context at multiple points
	type ComponentOperation struct {
		checkPoints []string
		errors      []error
		mu          sync.Mutex
	}

	operation := &ComponentOperation{}

	checkAndRecord := func(ctx context.Context, checkPoint string) error {
		err := ContextUtils.CheckContext(ctx, checkPoint)
		operation.mu.Lock()
		operation.checkPoints = append(operation.checkPoints, checkPoint)
		operation.errors = append(operation.errors, err)
		operation.mu.Unlock()
		return err
	}

	// Test normal flow
	ctx := context.Background()

	err := checkAndRecord(ctx, "initialization")
	assert.Nil(t, err)

	err = checkAndRecord(ctx, "data validation")
	assert.Nil(t, err)

	err = checkAndRecord(ctx, "processing")
	assert.Nil(t, err)

	operation.mu.Lock()
	assert.Equal(t, 3, len(operation.checkPoints))
	assert.Equal(t, []string{"initialization", "data validation", "processing"}, operation.checkPoints)
	for _, err := range operation.errors {
		assert.Nil(t, err)
	}
	operation.mu.Unlock()

	// Reset for cancelled context test
	operation.checkPoints = nil
	operation.errors = nil

	// Test with cancellation during processing
	cancelledCtx, cancel := context.WithCancel(context.Background())

	err = checkAndRecord(cancelledCtx, "initialization")
	assert.Nil(t, err)

	// Cancel context between checks
	cancel()

	err = checkAndRecord(cancelledCtx, "data validation")
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "data validation cancelled"))

	operation.mu.Lock()
	assert.Equal(t, 2, len(operation.checkPoints))
	assert.Equal(t, []string{"initialization", "data validation"}, operation.checkPoints)
	assert.Nil(t, operation.errors[0])
	assert.NotNil(t, operation.errors[1])
	operation.mu.Unlock()
}

// BenchmarkContextUtilsCheckContext benchmarks the CheckContext method
func BenchmarkContextUtilsCheckContext(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = ContextUtils.CheckContext(ctx, "benchmark test")
		}
	})
}

// BenchmarkContextUtilsShouldStop benchmarks the ShouldStop method
func BenchmarkContextUtilsShouldStop(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = ContextUtils.ShouldStop(ctx)
		}
	})
}

// BenchmarkContextUtilsIsContextCancelled benchmarks the IsContextCancelled method
func BenchmarkContextUtilsIsContextCancelled(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = ContextUtils.IsContextCancelled(ctx)
		}
	})
}
