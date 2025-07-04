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

// DefaultShutdownTimeout 默认优雅停机超时时间
const DefaultShutdownTimeout = 10 * time.Second

// GracefulShutdown provides a base implementation for graceful shutdown functionality.
// It can be embedded in components, endpoints, or other services that need to handle
// graceful shutdown with timeout and status management.
//
// GracefulShutdown 为优雅停机功能提供基础实现。
// 它可以嵌入到需要处理优雅停机、超时和状态管理的组件、端点或其他服务中。
//
// Key Features:
// 主要特性：
//   - Context-based shutdown signaling  基于上下文的停机信号
//   - Configurable shutdown timeout  可配置的停机超时
//   - Atomic shutdown state management  原子停机状态管理
//   - Thread-safe operations  线程安全操作
//   - Graceful vs. forced shutdown  优雅停机与强制停机
//
// Usage Pattern:
// 使用模式：
//  1. Embed GracefulShutdown in your struct  在结构体中嵌入 GracefulShutdown
//  2. Call InitGracefulShutdown() during initialization  在初始化期间调用 InitGracefulShutdown()
//  3. Use GetShutdownContext() to check shutdown signals  使用 GetShutdownContext() 检查停机信号
//  4. Call GracefulStop() to initiate shutdown  调用 GracefulStop() 启动停机
//  5. Override doStop() to implement custom cleanup  重写 doStop() 实现自定义清理
//
// Thread Safety:
// 线程安全：
//
//	All operations are thread-safe and can be called concurrently
//	from multiple goroutines without additional synchronization.
//
//	所有操作都是线程安全的，可以从多个 goroutine 并发调用，
//	无需额外的同步。
type GracefulShutdown struct {
	// shutdownCtx is the context for coordinating graceful shutdown
	// shutdownCtx 是协调优雅停机的上下文
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// shutdownTimeout defines the maximum time to wait for graceful shutdown
	// shutdownTimeout 定义优雅停机的最大等待时间
	shutdownTimeout time.Duration

	// isShuttingDown indicates whether the component is in shutdown process
	// isShuttingDown 指示组件是否处于停机过程中
	isShuttingDown int32

	// activeOperations tracks the number of operations currently being processed
	// activeOperations 跟踪当前正在处理的操作数量
	activeOperations int64

	// isReloading indicates whether the component is currently reloading
	// isReloading 指示组件当前是否正在重载
	isReloading int32

	// logger provides logging functionality
	// logger 提供日志功能
	logger types.Logger
}

// InitGracefulShutdown initializes the graceful shutdown functionality.
// This should be called during component initialization.
//
// InitGracefulShutdown 初始化优雅停机功能。
// 应在组件初始化期间调用。
//
// Parameters:
// 参数：
//   - logger: Logger instance for shutdown operations  停机操作的日志记录器实例
//   - timeout: Maximum time to wait for graceful shutdown, 0 uses default (10s)
//     timeout: 优雅停机的最大等待时间，0 使用默认值（10秒）
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
// GetShutdownContext 返回用于检查停机信号的停机上下文。
// 组件可以使用此上下文来检测何时启动了停机。
//
// Returns:
// 返回：
//   - context.Context: Context that is canceled when shutdown starts  停机开始时取消的上下文
//
// Usage Example:
// 使用示例：
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
// IsShuttingDown 返回组件当前是否处于停机过程中。
// 这是检查停机状态的线程安全方式。
//
// Returns:
// 返回：
//   - bool: true if shutdown is in progress  如果正在停机则为 true
func (g *GracefulShutdown) IsShuttingDown() bool {
	return atomic.LoadInt32(&g.isShuttingDown) == 1
}

// GracefulStop initiates graceful shutdown with two-phase design.
// Phase 1: Sets shutdown flag to reject new operations but allows ongoing operations to complete.
// Phase 2: Only cancels context after timeout to force interrupt ongoing operations.
//
// GracefulStop 启动两阶段优雅停机。
// 第一阶段：设置停机标志拒绝新操作，但允许正在进行的操作完成。
// 第二阶段：只有在超时后才取消上下文强制中断正在进行的操作。
//
// Parameters:
// 参数：
//   - stopFunc: Function to call for cleanup, should handle timeout logic (can be nil)
//     stopFunc: 清理函数，应处理超时逻辑（可以为 nil）
//
// The graceful shutdown process:
// 优雅停机过程：
//  1. Sets shutdown flag to prevent new operations  设置停机标志以防止新操作
//  2. Waits for ongoing operations to complete  等待正在进行的操作完成
//  3. Only cancels context if timeout is exceeded  只有超时时才取消上下文
//  4. Calls stopFunc() for cleanup  调用 stopFunc() 进行清理
func (g *GracefulShutdown) GracefulStop(stopFunc func()) {
	// 如果已经在停机，直接返回
	if !atomic.CompareAndSwapInt32(&g.isShuttingDown, 0, 1) {
		return
	}

	// 如果提供了停机函数，同步调用它
	// stopFunc 应该包含等待逻辑和超时处理
	if stopFunc != nil {
		stopFunc()
	}
}

// ForceStop immediately cancels the shutdown context to interrupt all ongoing operations.
// This should only be called after graceful shutdown timeout.
//
// ForceStop 立即取消停机上下文以中断所有正在进行的操作。
// 这应该只在优雅停机超时后调用。
func (g *GracefulShutdown) ForceStop() {
	// 强制取消上下文，中断所有正在进行的操作
	if g.shutdownCancel != nil {
		g.shutdownCancel()
	}
}

// CheckShutdownSignal is a convenience method for components to check shutdown signals.
// It returns an error if shutdown has been requested, allowing components to exit gracefully.
//
// CheckShutdownSignal 是组件检查停机信号的便捷方法。
// 如果请求了停机，它返回错误，允许组件优雅退出。
//
// Returns:
// 返回：
//   - error: Error if shutdown is requested, nil otherwise  如果请求停机则返回错误，否则为 nil
//
// Usage Example:
// 使用示例：
//
//	if err := g.CheckShutdownSignal(); err != nil {
//	    return err // Exit the operation
//	}
func (g *GracefulShutdown) CheckShutdownSignal() error {
	// First check if shutdown flag is set (phase 1 of graceful shutdown)
	// 首先检查停机标志是否已设置（优雅停机的第一阶段）
	if atomic.LoadInt32(&g.isShuttingDown) == 1 {
		return fmt.Errorf("operation cancelled due to shutdown")
	}

	// Also check if context has been cancelled (phase 2 of graceful shutdown)
	// 同时检查上下文是否已被取消（优雅停机的第二阶段）
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
// CheckShutdownContext 检查提供的上下文是否因停机而被取消。
// 当组件有自己的上下文并想要检查停机时，这很有用。
//
// Parameters:
// 参数：
//   - ctx: Context to check for cancellation  要检查取消的上下文
//
// Returns:
// 返回：
//   - error: Error if context is cancelled, nil otherwise  如果上下文被取消则返回错误，否则为 nil
//
// Usage Example:
// 使用示例：
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
// IncrementActiveOperations 原子地增加活跃操作计数器。
// 在启动需要在停机前完成的新操作时应调用此方法。
//
// Returns:
// 返回：
//   - int64: The new count of active operations  活跃操作的新计数
//
// Usage Example:
// 使用示例：
//
//	count := g.IncrementActiveOperations()
//	defer g.DecrementActiveOperations()
func (g *GracefulShutdown) IncrementActiveOperations() int64 {
	return atomic.AddInt64(&g.activeOperations, 1)
}

// DecrementActiveOperations atomically decrements the active operations counter.
// This should be called when an operation completes, either successfully or with error.
//
// DecrementActiveOperations 原子地减少活跃操作计数器。
// 在操作完成时应调用此方法，无论成功还是出错。
//
// Returns:
// 返回：
//   - int64: The new count of active operations  活跃操作的新计数
//
// Usage Example:
// 使用示例：
//
//	defer g.DecrementActiveOperations()
func (g *GracefulShutdown) DecrementActiveOperations() int64 {
	return atomic.AddInt64(&g.activeOperations, -1)
}

// GetActiveOperations returns the current number of active operations.
// This is useful for monitoring or debugging purposes.
//
// GetActiveOperations 返回当前活跃操作的数量。
// 这对监控或调试目的很有用。
//
// Returns:
// 返回：
//   - int64: Current count of active operations  当前活跃操作的计数
func (g *GracefulShutdown) GetActiveOperations() int64 {
	return atomic.LoadInt64(&g.activeOperations)
}

// WaitForActiveOperations waits for all active operations to complete with a timeout.
// This is typically used during graceful shutdown to ensure operations finish cleanly.
//
// WaitForActiveOperations 等待所有活跃操作在超时时间内完成。
// 这通常在优雅停机期间使用，以确保操作干净地完成。
//
// Parameters:
// 参数：
//   - timeout: Maximum time to wait for operations to complete  等待操作完成的最大时间
//
// Returns:
// 返回：
//   - bool: true if all operations completed, false if timeout occurred  如果所有操作完成则为 true，如果超时则为 false
//
// Usage Example:
// 使用示例：
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
// logf 提供带空检查的内部日志记录
func (g *GracefulShutdown) logf(format string, args ...interface{}) {
	if g.logger != nil {
		g.logger.Printf(format, args...)
	}
}

// ContextUtils provides utility functions for context checking in components.
// These functions can be used by components that don't embed GracefulShutdown
// but still need to check for shutdown signals.
//
// ContextUtils 为组件中的上下文检查提供实用函数。
// 这些函数可以被不嵌入 GracefulShutdown 但仍需要检查停机信号的组件使用。
var ContextUtils = &contextUtils{}

type contextUtils struct{}

// CheckContext checks if the provided context has been cancelled.
// This is a convenience function for components to quickly check for cancellation signals.
//
// CheckContext 检查提供的上下文是否已被取消。
// 这是组件快速检查取消信号的便捷函数。
//
// Parameters:
// 参数：
//   - ctx: Context to check for cancellation  要检查取消的上下文
//   - operation: Optional operation description for error message  错误消息的可选操作描述
//
// Returns:
// 返回：
//   - error: Error if context is cancelled, nil otherwise  如果上下文被取消则返回错误，否则为 nil
//
// Usage Example:
// 使用示例：
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
// CheckContextWithTimeout 检查提供的上下文是否已被取消，
// 并设置额外的超时以避免无限期阻塞。
//
// Parameters:
// 参数：
//   - ctx: Context to check for cancellation  要检查取消的上下文
//   - timeout: Maximum time to wait for context state  等待上下文状态的最大时间
//   - operation: Optional operation description for error message  错误消息的可选操作描述
//
// Returns:
// 返回：
//   - error: Error if context is cancelled or timeout occurs  如果上下文被取消或超时则返回错误
//
// Usage Example:
// 使用示例：
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
// ShouldStop 提供操作是否应该停止的简单布尔检查。
// 这对于偏好布尔检查而非错误处理的组件很有用。
//
// Parameters:
// 参数：
//   - ctx: Context to check for cancellation  要检查取消的上下文
//
// Returns:
// 返回：
//   - bool: true if operation should stop, false otherwise  如果操作应该停止则为 true，否则为 false
//
// Usage Example:
// 使用示例：
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
// WithGracefulShutdown 使用优雅停机检查包装函数。
// 只有在上下文未被取消时才会调用提供的函数。
//
// Parameters:
// 参数：
//   - ctx: Context to check for cancellation  要检查取消的上下文
//   - operation: Function to execute if context is not cancelled  如果上下文未被取消要执行的函数
//
// Returns:
// 返回：
//   - error: Error if context is cancelled, otherwise error from operation  如果上下文被取消则返回错误，否则返回操作的错误
//
// Usage Example:
// 使用示例：
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
// IsContextCancelled 提供简单的上下文取消检查而不返回错误。
// 这对于日志记录或条件逻辑很有用，而无需错误传播。
//
// Parameters:
// 参数：
//   - ctx: Context to check  要检查的上下文
//
// Returns:
// 返回：
//   - bool: true if context is cancelled  如果上下文被取消则为 true
//   - error: The cancellation error if any  如果有的话，取消错误
//
// Usage Example:
// 使用示例：
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
// IsReloading 返回组件当前是否处于重载过程中。
// 这是检查重载状态的线程安全方式。
//
// Returns:
// 返回：
//   - bool: true if reload is in progress  如果正在重载则为 true
func (g *GracefulShutdown) IsReloading() bool {
	return atomic.LoadInt32(&g.isReloading) == 1
}

// SetReloading sets the reload status atomically.
// This should be called when starting or finishing a reload operation.
//
// SetReloading 原子地设置重载状态。
// 在开始或完成重载操作时应调用此方法。
//
// Parameters:
// 参数：
//   - reloading: true to set reloading state, false to clear it  true 设置重载状态，false 清除状态
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
// WaitForReloadComplete 等待重载操作在超时时间内完成。
// 消息处理可以使用此方法等待重载完成。
//
// Parameters:
// 参数：
//   - timeout: Maximum time to wait for reload to complete  等待重载完成的最大时间
//
// Returns:
// 返回：
//   - bool: true if reload completed, false if timeout occurred  如果重载完成则为 true，如果超时则为 false
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
