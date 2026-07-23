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

// Package base provides foundational components and utilities for graceful shutdown
package base

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rulego/rulego/api/types"
)

// DefaultShutdownTimeout Defaults to the elegant shutdown timeout timeout
const DefaultShutdownTimeout = 10 * time.Second

// GracefulShutdown provides a base implementation for graceful shutdown functionality.
// It can be embedded in components, endpoints, or other services that need to handle
// graceful shutdown with timeout and status management.
//
// GracefulShutdown provides the foundational implementation for the graceful shutdown function.
// It can be embedded into components, endpoints, or other services that need to handle elegant downtime, timeouts, and state management.
//
// Key Features:
// Key features:
//   - Context-based shutdown signaling
//   - Configurable shutdown timeout
//   - Atomic shutdown state management
//   - Thread-safe operations
//   - Graceful vs. forced shutdown
//
// Usage Pattern:
// Usage mode:
//  1. Embed GracefulShutdown in your struct
//  2. Call InitGracefulShutdown() during initialization
//  3. Use GetShutdownContext() to check shutdown signals
//  4. Call GracefulStop() to initiate shutdown
//  5. Override doStop() to implement custom cleanup
//
// Thread Safety:
// Thread safety:
//
//	All operations are thread-safe and can be called concurrently
//	from multiple goroutines without additional synchronization.
//
//	All operations are thread-safe and can be called concurrently from multiple goroutines,
//	No additional synchronization is needed.
type GracefulShutdown struct {
	// shutdownCtx is the context for coordinating graceful shutdown
	// shutdownCtx is the context for coordinating elegant shutdowns
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// shutdownTimeout defines the maximum time to wait for graceful shutdown
	// shutdownTimeout defines the maximum waiting time for elegant downtime
	shutdownTimeout time.Duration

	// isShuttingDown indicates whether the component is in shutdown process
	// isShuttingDown indicates whether the component is in a shutdown process
	isShuttingDown int32

	// activeOperations tracks the number of operations currently being processed
	// activeOperations tracks the number of operations currently being processed
	activeOperations int64

	// isReloading indicates whether the component is currently reloading
	// isReloading indicates whether the component is currently being reloaded
	isReloading int32

	// logger provides logging functionality
	// Logger provides logging functionality
	logger types.Logger
}

// InitGracefulShutdown initializes the graceful shutdown functionality.
// This should be called during component initialization.
//
// InitGracefulShutdown initializes the graceful shutdown function.
// It should be called during component initialization.
//
// Parameters:
// Parameters:
//   - logger: Logger instance for shutdown operations
//   - timeout: Maximum time to wait for graceful shutdown, 0 uses default (10s)
//     timeout: Maximum waiting time for elegant downtime, 0 using default value (10 seconds)
func (g *GracefulShutdown) InitGracefulShutdown(logger types.Logger, timeout time.Duration) {
	if timeout == 0 {
		timeout = DefaultShutdownTimeout
	}

	g.shutdownTimeout = timeout
	g.logger = logger
	g.shutdownCtx, g.shutdownCancel = context.WithCancel(context.Background())
	atomic.StoreInt32(&g.isShuttingDown, 0)
}

// GetShutdownContext returns the shutdown context for checking shutdown signals.
// Components can use this context to detect when shutdown has been initiated.
//
// GetShutdownContext returns the shutdown context used to check the shutdown signal.
// Components can use this context to detect when a shutdown has started.
//
// Returns:
// Returns:
//   - context.Context: Context that is canceled when shutdown starts
//
// Usage Example:
// Example:
//
//	select {
//	case <-g.GetShutdownContext().Done():
//	    // Handle shutdown signal
//	    return fmt.Errorf("shutdown requested")
//	default:
//	    // Continue normal operation
//	}
func (g *GracefulShutdown) GetShutdownContext() context.Context {
	return g.shutdownCtx
}

// IsShuttingDown returns whether the component is currently in shutdown process.
// This is a thread-safe way to check shutdown status.
//
// IsShuttingDown returns whether the component is currently in a shutdown process.
// This is a thread-safe method for checking the downtime status.
//
// Returns:
// Returns:
//   - bool: true if shutdown is in progress
func (g *GracefulShutdown) IsShuttingDown() bool {
	return atomic.LoadInt32(&g.isShuttingDown) == 1
}

// GracefulStop initiates graceful shutdown with two-phase design.
// Phase 1: Sets shutdown flag to reject new operations but allows ongoing operations to complete.
// Phase 2: Only cancels context after timeout to force interrupt ongoing operations.
//
// GracefulStop initiates a two-stage elegant shutdown.
// Phase One: Set a shutdown sign to reject new operations but allow ongoing operations to be completed.
// Stage Two: Stop the ongoing operation by forcing context only after timeout.
//
// Parameters:
// Parameters:
//   - stopFunc: Function to call for cleanup, should handle timeout logic (can be nil)
//     stopFunc: Cleanup function, should handle timeout logic (can be nil)
//
// The graceful shutdown process:
// Elegant shutdown process:
//  1. Sets shutdown flag to prevent new operations
//  2. Waits for ongoing operations to complete
//  3. Only cancels context if timeout is exceeded
//  4. Calls stopFunc() for cleanup
func (g *GracefulShutdown) GracefulStop(stopFunc func()) {
	// If the machine is already down, just return directly
	if !atomic.CompareAndSwapInt32(&g.isShuttingDown, 0, 1) {
		return
	}

	// If a shutdown function is provided, call it synchronously
	// stopFunc should include waiting logic and timeout handling
	if stopFunc != nil {
		stopFunc()
	}
}

// ForceStop immediately cancels the shutdown context to interrupt all ongoing operations.
// This should only be called after graceful shutdown timeout.
//
// ForceStop immediately removes the downtime context to interrupt all ongoing operations.
// This should only be called after the elegant shutdown timeout.
func (g *GracefulShutdown) ForceStop() {
	// Forced disengagement of context, interrupting all ongoing operations
	if g.shutdownCancel != nil {
		g.shutdownCancel()
	}
}

// CheckShutdownSignal is a convenience method for components to check shutdown signals.
// It returns an error if shutdown has been requested, allowing components to exit gracefully.
//
// CheckShutdownSignal is a convenient way for components to check shutdown signals.
// If a shutdown is requested, it returns an error and allows the component to exit gracefully.
//
// Returns:
// Returns:
//   - error: Error if shutdown is requested, nil otherwise
//
// Usage Example:
// Example:
//
//	if err := g.CheckShutdownSignal(); err != nil {
//	    return err // Exit the operation
//	}
func (g *GracefulShutdown) CheckShutdownSignal() error {
	// First check if shutdown flag is set (phase 1 of graceful shutdown)
	// First, check if the shutdown sign has been set (the first stage of graceful shutdown).
	if atomic.LoadInt32(&g.isShuttingDown) == 1 {
		return fmt.Errorf("operation cancelled due to shutdown")
	}

	// Also check if context has been cancelled (phase 2 of graceful shutdown)
	// At the same time, check whether the context has been canceled (the second stage of graceful shutdown).
	if g.shutdownCtx != nil {
		select {
		case <-g.shutdownCtx.Done():
			return fmt.Errorf("operation cancelled due to shutdown")
		default:
		}
	}
	return nil
}

// CheckShutdownContext checks if the provided context has been cancelled due to shutdown.
// This is useful when components have their own context and want to check for shutdown.
//
// CheckShutdownContext checks whether the provided context has been canceled due to downtime.
// This is useful when components have their own context and want to check for downtime.
//
// Parameters:
// Parameters:
//   - ctx: Context to check for cancellation
//
// Returns:
// Returns:
//   - error: Error if context is cancelled, nil otherwise
//
// Usage Example:
// Example:
//
//	if err := g.CheckShutdownContext(ctx); err != nil {
//	    return err
//	}
func (g *GracefulShutdown) CheckShutdownContext(ctx context.Context) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}
	}
	return nil
}

// IncrementActiveOperations atomically increments the active operations counter.
// This should be called when starting a new operation that needs to complete before shutdown.
//
// IncrementActiveOperations atomically increases the active operations counter.
// This method should be called when starting a new operation that needs to be completed before shutdown.
//
// Returns:
// Returns:
//   - int64: The new count of active operations
//
// Usage Example:
// Example:
//
//	count := g.IncrementActiveOperations()
//	defer g.DecrementActiveOperations()
func (g *GracefulShutdown) IncrementActiveOperations() int64 {
	return atomic.AddInt64(&g.activeOperations, 1)
}

// DecrementActiveOperations atomically decrements the active operations counter.
// This should be called when an operation completes, either successfully or with error.
//
// DecrementActiveOperations atomically reduces the active operations counter.
// This method should be called upon completion of the operation, whether successful or incorrect.
//
// Returns:
// Returns:
//   - int64: The new count of active operations
//
// Usage Example:
// Example:
//
//	defer g.DecrementActiveOperations()
func (g *GracefulShutdown) DecrementActiveOperations() int64 {
	return atomic.AddInt64(&g.activeOperations, -1)
}

// GetActiveOperations returns the current number of active operations.
// This is useful for monitoring or debugging purposes.
//
// GetActiveOperations returns the number of currently active operations.
// This is useful for monitoring or debugging purposes.
//
// Returns:
// Returns:
//   - int64: Current count of active operations
func (g *GracefulShutdown) GetActiveOperations() int64 {
	return atomic.LoadInt64(&g.activeOperations)
}

// WaitForActiveOperations waits for all active operations to complete with a timeout.
// This is typically used during graceful shutdown to ensure operations finish cleanly.
//
// WaitForActiveOperations Wait for all active operations to complete within the timeout.
// This is usually used during elegant downtime to ensure the operation is clean.
//
// Parameters:
// Parameters:
//   - timeout: Maximum time to wait for operations to complete
//
// Returns:
// Returns:
//   - bool: true if all operations completed, false if timeout occurred
//
// Usage Example:
// Example:
//
//	if !g.WaitForActiveOperations(30 * time.Second) {
//	    g.logf("Timeout waiting for operations to complete")
//	}
func (g *GracefulShutdown) WaitForActiveOperations(timeout time.Duration) bool {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout reached
			return false
		case <-ticker.C:
			if atomic.LoadInt64(&g.activeOperations) <= 0 {
				// All operations completed
				return true
			}
		}
	}
}

// logf provides internal logging with null-check
// logf provides internal logs with null checks
func (g *GracefulShutdown) logf(format string, args ...interface{}) {
	if g.logger != nil {
		g.logger.Printf(format, args...)
	}
}

// ContextUtils provides utility functions for context checking in components.
// These functions can be used by components that don't embed GracefulShutdown
// but still need to check for shutdown signals.
//
// ContextUtils provides practical functions for contextual checking within components.
// These functions can be used by components that are not embedded in GracefulShutdown but still need to check for shutdown signals.
var ContextUtils = &contextUtils{}

type contextUtils struct{}

// CheckContext checks if the provided context has been cancelled.
// This is a convenience function for components to quickly check for cancellation signals.
//
// CheckContext checks whether the provided context has been canceled.
// This is a convenient function for components to quickly check cancel signals.
//
// Parameters:
// Parameters:
//   - ctx: Context to check for cancellation
//   - operation: Optional operation description for error message
//
// Returns:
// Returns:
//   - error: Error if context is cancelled, nil otherwise
//
// Usage Example:
// Example:
//
//	func (x *NetNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
//	    if err := base.ContextUtils.CheckContext(ctx.GetContext(), "network operation"); err != nil {
//	        ctx.TellFailure(msg, err)
//	        return
//	    }
//	    // Continue with normal processing
//	}
func (u *contextUtils) CheckContext(ctx context.Context, operation string) error {
	if ctx != nil {
		select {
		case <-ctx.Done():
			if operation != "" {
				return fmt.Errorf("%s cancelled: %w", operation, ctx.Err())
			}
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		default:
		}
	}
	return nil
}

// CheckContextWithTimeout checks if the provided context has been cancelled,
// with an additional timeout to avoid blocking indefinitely.
//
// CheckContextWithTimeout checks whether the provided context has been canceled,
// and set additional timeouts to avoid indefinite blocking.
//
// Parameters:
// Parameters:
//   - ctx: Context to check for cancellation
//   - timeout: Maximum time to wait for context state
//   - operation: Optional operation description for error message
//
// Returns:
// Returns:
//   - error: Error if context is cancelled or timeout occurs
//
// Usage Example:
// Example:
//
//	if err := base.ContextUtils.CheckContextWithTimeout(ctx.GetContext(),
//	    time.Second, "database operation"); err != nil {
//	    return err
//	}
func (u *contextUtils) CheckContextWithTimeout(ctx context.Context, timeout time.Duration, operation string) error {
	if ctx == nil {
		return nil
	}

	checkCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		if operation != "" {
			return fmt.Errorf("%s cancelled: %w", operation, ctx.Err())
		}
		return fmt.Errorf("operation cancelled: %w", ctx.Err())
	case <-checkCtx.Done():
		// Timeout reached, assume context is not cancelled
		return nil
	default:
		return nil
	}
}

// ShouldStop provides a simple boolean check for whether an operation should stop.
// This is useful for components that prefer boolean checks over error handling.
//
// ShouldStop provides a simple Boolean check on whether the operation should be stopped.
// This is useful for components that prefer Boolean checking over error handling.
//
// Parameters:
// Parameters:
//   - ctx: Context to check for cancellation
//
// Returns:
// Returns:
//   - bool: true if operation should stop, false otherwise
//
// Usage Example:
// Example:
//
//	for !base.ContextUtils.ShouldStop(ctx.GetContext()) {
//	    // Continue processing
//	    doWork()
//	}
func (u *contextUtils) ShouldStop(ctx context.Context) bool {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return true
		default:
		}
	}
	return false
}

// WithGracefulShutdown wraps a function with graceful shutdown checking.
// It will call the provided function only if the context is not cancelled.
//
// WithGracefulShutdown uses the Graceful Shutdown check wrapper function.
// The provided function is only called when the context is not canceled.
//
// Parameters:
// Parameters:
//   - ctx: Context to check for cancellation
//   - operation: Function to execute if context is not cancelled
//
// Returns:
// Returns:
//   - error: Error if context is cancelled, otherwise error from operation
//
// Usage Example:
// Example:
//
//	err := base.ContextUtils.WithGracefulShutdown(ctx.GetContext(), func() error {
//	    return performNetworkOperation()
//	})
//	if err != nil {
//	    ctx.TellFailure(msg, err)
//	    return
//	}
func (u *contextUtils) WithGracefulShutdown(ctx context.Context, operation func() error) error {
	if err := u.CheckContext(ctx, ""); err != nil {
		return err
	}
	return operation()
}

// IsContextCancelled provides a simple check if context is cancelled without returning an error.
// This is useful for logging or conditional logic without error propagation.
//
// IsContextCancelled provides a simple context cancel check without returning an error.
// This is useful for logging or conditional logic without the need for false propagation.
//
// Parameters:
// Parameters:
//   - ctx: Context to check
//
// Returns:
// Returns:
//   - bool: true if context is cancelled
//   - error: The cancellation error if any
//
// Usage Example:
// Example:
//
//	if cancelled, err := base.ContextUtils.IsContextCancelled(ctx.GetContext()); cancelled {
//	    logger.Printf("Operation cancelled: %v", err)
//	    return
//	}
func (u *contextUtils) IsContextCancelled(ctx context.Context) (bool, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		default:
		}
	}
	return false, nil
}

// IsReloading returns whether the component is currently in reload process.
// This is a thread-safe way to check reload status.
//
// IsReloading returns whether the component is currently in the overload process.
// This is the thread safety method for checking overload status.
//
// Returns:
// Returns:
//   - bool: true if reload is in progress
func (g *GracefulShutdown) IsReloading() bool {
	return atomic.LoadInt32(&g.isReloading) == 1
}

// SetReloading sets the reload status atomically.
// This should be called when starting or finishing a reload operation.
//
// SetReloading: Atomically sets the overload state.
// This method should be called when starting or completing a reload operation.
//
// Parameters:
// Parameters:
//   - reloading: true to set reloading state, false to clear it true
func (g *GracefulShutdown) SetReloading(reloading bool) {
	if reloading {
		atomic.StoreInt32(&g.isReloading, 1)
	} else {
		atomic.StoreInt32(&g.isReloading, 0)
	}
}

// WaitForReloadComplete waits for reload operation to complete with a timeout.
// This can be used by message processing to wait for reload to finish.
//
// WaitForReloadComplete Wait for the reload operation to complete within the timeout.
// Message processing can be used in this way to wait for the overload to complete.
//
// Parameters:
// Parameters:
//   - timeout: Maximum time to wait for reload to complete
//
// Returns:
// Returns:
//   - bool: true if reload completed, false if timeout occurred
func (g *GracefulShutdown) WaitForReloadComplete(timeout time.Duration) bool {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout reached
			return false
		case <-ticker.C:
			if !g.IsReloading() {
				// Reload completed
				return true
			}
		}
	}
}
